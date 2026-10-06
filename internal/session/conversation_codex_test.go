package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

const (
	codexTestWork  = "/work/proj"
	codexTestIDFmt = "019a0000-0000-7000-8000-%012d"
)

func codexTestID(n int) string { return fmt.Sprintf(codexTestIDFmt, n) }

// codexMetaLine is the first line of a rollout, with pad bytes of
// base_instructions so a test can make it long.
func codexMetaLine(t *testing.T, id, cwd string, pad int) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"type": "session_meta",
		"payload": map[string]any{
			"id":                id,
			"cwd":               cwd,
			"base_instructions": strings.Repeat("x", pad),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(b) + "\n"
}

// writeRollout writes sessions/Y/M/D/rollout-<stamp>-<nameID>.jsonl and returns its path.
func writeRollout(t *testing.T, root, day, stamp, nameID, content string) string {
	t.Helper()
	dir := filepath.Join(root, day)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "rollout-"+stamp+"-"+nameID+".jsonl")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// plantCodexRollout lays a one-line rollout for id under home/sessions, in the
// directory layout Codex writes, and returns its path.
func plantCodexRollout(t *testing.T, home, id, cwd string) string {
	t.Helper()
	return writeRollout(t, filepath.Join(home, "sessions"), "2026/10/06", "2026-10-06T00-11-13", id, codexMetaLine(t, id, cwd, 0))
}

func TestConversationsForDispatch(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	home := t.TempDir()
	f.mgr.SetCodexHome(home)
	id := codexTestID(7)
	plantCodexRollout(t, home, id, f.repo())

	tests := []struct {
		name string
		h    harness.Name
		want int
	}{
		{"codex reads rollouts", harness.Codex, 1},
		{"claude reads its own history, none planted", harness.Claude, 0},
		{"other has no conversations", harness.Other, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := f.mgr.ConversationsFor(tc.h, f.repo())
			if len(got) != tc.want {
				t.Fatalf("ConversationsFor(%q) returned %d conversations, want %d", tc.h, len(got), tc.want)
			}
			if tc.h == harness.Codex && got[0].ID != id {
				t.Errorf("ConversationsFor(codex) ID = %q, want %q", got[0].ID, id)
			}
		})
	}

	t.Run("codex with no home", func(t *testing.T) {
		t.Parallel()
		bare := newManagerFixture(t)
		if got := bare.mgr.ConversationsFor(harness.Codex, bare.repo()); got != nil {
			t.Errorf("ConversationsFor(codex) with no codex home = %v, want nil", got)
		}
	})
}

func TestConversationsForCodexResolvesTheWorkDir(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	home := t.TempDir()
	f.mgr.SetCodexHome(home)
	plantCodexRollout(t, home, codexTestID(1), f.repo())
	plantCodexRollout(t, home, codexTestID(2), f.outside)

	if got := f.mgr.ConversationsFor(harness.Codex, f.outside); len(got) != 0 {
		t.Errorf("ConversationsFor(codex) outside the approved roots returned %d conversations, want 0", len(got))
	}
	link := f.insideLink()
	if got := f.mgr.ConversationsFor(harness.Codex, link); len(got) != 1 || got[0].ID != codexTestID(1) {
		t.Errorf("ConversationsFor(codex) through a symlink spelling = %v, want the one rollout for the real path", got)
	}
}

func TestContinueOtherRefusedEarly(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	f.mgr.SetStartCommands(config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: claudeStartCommand,
		"shell":                        "bash",
	}))
	req := f.request()
	req.StartCommand = "shell"
	s, _ := mustCreate(t, f, req)
	before := len(f.tmux.Calls())

	_, err := f.mgr.Continue(context.Background(), *s, harnessTestConversation)
	if !errors.Is(err, ErrInvalidResume) {
		t.Fatalf("Continue() error = %v, want ErrInvalidResume", err)
	}
	if got := f.tmux.Calls()[before:]; len(got) != 0 {
		t.Errorf("Continue() made %d tmux calls on a session that cannot resume, want 0", len(got))
	}
	stored, getErr := f.store.Get(s.ID, s.Owner)
	if getErr != nil {
		t.Fatalf("Get() unexpected error: %v", getErr)
	}
	if stored.ConversationID != s.ConversationID {
		t.Errorf("store ConversationID = %q, want %q", stored.ConversationID, s.ConversationID)
	}
}

func TestContinueCodexChecksCodexTranscript(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	f.mgr.SetCodexHome(t.TempDir())
	before := len(f.tmux.Calls())

	// A Claude transcript for the same identifier must not satisfy a Codex session.
	_, err := f.mgr.Continue(context.Background(), *s, harnessTestConversation)
	if !errors.Is(err, ErrInvalidResume) {
		t.Fatalf("Continue() error = %v, want ErrInvalidResume with no rollout on disk", err)
	}
	if got := f.tmux.Calls()[before:]; len(got) != 0 {
		t.Errorf("Continue() made %d tmux calls with no transcript, want 0", len(got))
	}
	if f.mgr.hasTranscriptFor(*s, harnessTestConversation) {
		t.Error("hasTranscriptFor() true with no rollout")
	}
	plantCodexRollout(t, f.mgr.codexHome, harnessTestConversation, s.WorkDir)
	if !f.mgr.hasTranscriptFor(*s, harnessTestConversation) {
		t.Error("hasTranscriptFor() false with a rollout planted")
	}
}

func TestCodexConversations(t *testing.T) {
	t.Parallel()

	t.Run("newest first across days, other cwd skipped", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		days := []string{"2026/09/30", "2026/10/01", "2026/10/02"}
		for i, d := range days {
			id := codexTestID(i + 1)
			p := writeRollout(t, root, d, "2026-10-0"+fmt.Sprint(i+1)+"T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
			mt := time.Unix(1_700_000_000+int64(i)*3600, 0)
			if err := os.Chtimes(p, mt, mt); err != nil {
				t.Fatal(err)
			}
		}
		other := codexTestID(99)
		writeRollout(t, root, "2026/10/02", "2026-10-02T11-00-00", other, codexMetaLine(t, other, "/elsewhere", 0))

		got := codexConversations(root, codexTestWork)
		if len(got) != 3 {
			t.Fatalf("got %d conversations, want 3: %v", len(got), got)
		}
		for i, wantN := range []int{3, 2, 1} {
			if got[i].ID != codexTestID(wantN) {
				t.Errorf("row %d = %s, want %s", i, got[i].ID, codexTestID(wantN))
			}
		}
	})

	t.Run("limit enforced with 60 matches", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		for i := 0; i < 60; i++ {
			id := codexTestID(i)
			writeRollout(t, root, "2026/10/02", fmt.Sprintf("2026-10-02T10-%02d-00", i), id, codexMetaLine(t, id, codexTestWork, 0))
		}
		if got := codexConversations(root, codexTestWork); len(got) != codexListLimit {
			t.Fatalf("got %d, want %d", len(got), codexListLimit)
		}
	})

	t.Run("missing root is empty", func(t *testing.T) {
		t.Parallel()
		if got := codexConversations(filepath.Join(t.TempDir(), "none"), codexTestWork); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("symlinked day directory skipped", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		real := t.TempDir()
		id := codexTestID(1)
		writeRollout(t, real, "", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		if err := os.MkdirAll(filepath.Join(root, "2026", "10"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, filepath.Join(root, "2026", "10", "02")); err != nil {
			t.Fatal(err)
		}
		if got := codexConversations(root, codexTestWork); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("unreadable day directory is not fatal", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		id := codexTestID(1)
		writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		id2 := codexTestID(2)
		writeRollout(t, root, "2026/10/01", "2026-10-01T10-00-00", id2, codexMetaLine(t, id2, codexTestWork, 0))
		bad := filepath.Join(root, "2026", "10", "02")
		if err := os.Chmod(bad, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(bad, 0o700) }) //nolint:gosec,errcheck // G302: a directory needs the execute bit to be removed.
		got := codexConversations(root, codexTestWork)
		if len(got) != 1 && os.Geteuid() != 0 {
			t.Fatalf("got %v, want the readable day only", got)
		}
	})

	t.Run("filename id differing from meta id skipped", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", codexTestID(1), codexMetaLine(t, codexTestID(2), codexTestWork, 0))
		if got := codexConversations(root, codexTestWork); len(got) != 0 {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("symlinked root resolves on both sides", func(t *testing.T) {
		t.Parallel()
		real := t.TempDir()
		id := codexTestID(1)
		writeRollout(t, real, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		link := filepath.Join(t.TempDir(), "sessions")
		if err := os.Symlink(real, link); err != nil {
			t.Fatal(err)
		}
		if got := codexConversations(link, codexTestWork); len(got) != 1 {
			t.Fatalf("got %v", got)
		}
	})
}

// readCodexMetaAt opens a path plainly and reads its metadata, for the tests
// that exercise the parser and not the containment.
func readCodexMetaAt(path string) (id, cwd string, ok bool) {
	f, err := os.Open(path) //nolint:gosec // G304: a test's own temp file.
	if err != nil {
		return "", "", false
	}
	defer f.Close() //nolint:errcheck,gosec // read-only
	return readCodexMeta(f)
}

// codexRollout is whether openCodexRollout accepts path.
func codexRollout(root, path string) bool {
	f, ok := openCodexRollout(root, path)
	if ok {
		f.Close() //nolint:errcheck,gosec // read-only
	}
	return ok
}

func TestReadCodexMeta(t *testing.T) {
	t.Parallel()
	id := codexTestID(1)
	tests := []struct {
		name    string
		content string
		wantOK  bool
	}{
		{"short first line", codexMetaLine(t, id, codexTestWork, 0), true},
		{"30 KB first line", codexMetaLine(t, id, codexTestWork, 30<<10), true},
		{"first line over the limit", codexMetaLine(t, id, codexTestWork, codexMetaReadLimit+10), false},
		{"no newline", strings.TrimSuffix(codexMetaLine(t, id, codexTestWork, 0), "\n"), false},
		{"wrong type", `{"type":"turn_context","payload":{"id":"` + id + `","cwd":"/x"}}` + "\n", false},
		{"not json", "garbage\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := filepath.Join(t.TempDir(), "r.jsonl")
			if err := os.WriteFile(p, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			gotID, gotCwd, ok := readCodexMetaAt(p)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && (gotID != id || gotCwd != codexTestWork) {
				t.Fatalf("got %q %q", gotID, gotCwd)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		if _, _, ok := readCodexMetaAt(filepath.Join(t.TempDir(), "none")); ok {
			t.Fatal("ok for a missing file")
		}
	})
}

func TestCodexHasTranscript(t *testing.T) {
	t.Parallel()
	id := codexTestID(1)

	t.Run("one match in the right cwd", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		if !codexHasTranscript(root, id, codexTestWork) {
			t.Fatal("want true")
		}
	})
	t.Run("other cwd", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, "/elsewhere", 0))
		if codexHasTranscript(root, id, codexTestWork) {
			t.Fatal("want false")
		}
	})
	t.Run("metadata id differs from the filename", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, codexTestID(2), codexTestWork, 0))
		if codexHasTranscript(root, id, codexTestWork) {
			t.Fatal("want false when line 1 names another conversation")
		}
	})
	t.Run("not an id", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if codexHasTranscript(root, "../../etc/passwd", codexTestWork) {
			t.Fatal("want false")
		}
	})
	t.Run("two matches", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		writeRollout(t, root, "2026/10/03", "2026-10-03T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		if codexHasTranscript(root, id, codexTestWork) {
			t.Fatal("want false for an ambiguous id")
		}
	})
	t.Run("none", func(t *testing.T) {
		t.Parallel()
		if codexHasTranscript(t.TempDir(), id, codexTestWork) {
			t.Fatal("want false")
		}
	})
	t.Run("symlinked rollout", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "o.jsonl")
		if err := os.WriteFile(outside, []byte(codexMetaLine(t, id, codexTestWork, 0)), 0o600); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(root, "2026", "10", "02")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(dir, "rollout-2026-10-02T10-00-00-"+id+".jsonl")); err != nil {
			t.Fatal(err)
		}
		if codexHasTranscript(root, id, codexTestWork) {
			t.Fatal("want false")
		}
	})
}

func TestCodexRolloutRejectsSymlinks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	id := codexTestID(1)
	real := writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))

	if !codexRollout(root, real) {
		t.Error("a regular file inside the root was rejected")
	}

	outside := filepath.Join(t.TempDir(), "o.jsonl")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	toOutside := filepath.Join(root, "2026", "10", "02", "out.jsonl")
	if err := os.Symlink(outside, toOutside); err != nil {
		t.Fatal(err)
	}
	if codexRollout(root, toOutside) {
		t.Error("a symlink to a file outside the root was accepted")
	}

	toInside := filepath.Join(root, "2026", "10", "02", "in.jsonl")
	if err := os.Symlink(real, toInside); err != nil {
		t.Fatal(err)
	}
	if codexRollout(root, toInside) {
		t.Error("a symlink to a file inside the root was accepted")
	}

	if codexRollout(root, filepath.Join(root, "2026", "10", "02", "absent.jsonl")) {
		t.Error("a missing file was accepted")
	}

	// A file whose parent is outside the root.
	if codexRollout(root, outside) {
		t.Error("a file outside the root was accepted")
	}
}

func TestCodexWalkBoundsEntries(t *testing.T) {
	t.Parallel()
	id := codexTestID(1)

	t.Run("a day holding more than the cap is refused", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		path := writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		day := filepath.Dir(path)
		for i := 0; i <= codexEntryCap; i++ {
			if err := os.WriteFile(filepath.Join(day, fmt.Sprintf("junk-%d", i)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if codexHasTranscript(root, id, codexTestWork) {
			t.Error("codexHasTranscript read past the entry cap")
		}
		err := walkCodexRollouts(root, func(string, os.DirEntry, string) bool { return false })
		if !errors.Is(err, ErrDiscoveryBounds) {
			t.Errorf("walkCodexRollouts() = %v, want ErrDiscoveryBounds", err)
		}
	})

	t.Run("a listing keeps what it found before the cap", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeRollout(t, root, "2026/10/03", "2026-10-03T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
		old := filepath.Join(root, "2026", "10", "01")
		if err := os.MkdirAll(old, 0o700); err != nil {
			t.Fatal(err)
		}
		for i := 0; i <= codexEntryCap; i++ {
			if err := os.WriteFile(filepath.Join(old, fmt.Sprintf("junk-%d", i)), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if got := codexConversations(root, codexTestWork); len(got) != 1 {
			t.Errorf("codexConversations() returned %d, want the 1 found before the cap", len(got))
		}
	})
}

func TestCodexWalkSkipsNonNumericDateDirs(t *testing.T) {
	t.Parallel()
	id := codexTestID(1)

	for _, day := range []string{
		"abcd/10/02",
		"2026/ab/02",
		"2026/10/xx",
		"20260/10/02",
		"2026/1/02",
		"2026/10/2",
		"\u0662\u0660\u0662\u0666/10/02",
	} {
		t.Run(day, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeRollout(t, root, day, "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
			if got := codexConversations(root, codexTestWork); len(got) != 0 {
				t.Errorf("a rollout under %q was listed", day)
			}
			if codexHasTranscript(root, id, codexTestWork) {
				t.Errorf("a rollout under %q satisfied the transcript check", day)
			}
		})
	}
}

func TestOpenCodexRolloutSymlinkedParent(t *testing.T) {
	t.Parallel()
	id := codexTestID(1)
	root := t.TempDir()
	outsideRoot := t.TempDir()
	out := writeRollout(t, outsideRoot, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))

	// root/2026 is a symlink to a tree outside the sessions dir.
	if err := os.Symlink(filepath.Join(outsideRoot, "2026"), filepath.Join(root, "2026")); err != nil {
		t.Fatal(err)
	}
	via := filepath.Join(root, "2026", "10", "02", filepath.Base(out))
	if codexRollout(root, via) {
		t.Error("a rollout reached through a symlinked parent outside the root was accepted")
	}
}

func TestOpenCodexRolloutLeafSwappedAfterOpen(t *testing.T) {
	t.Parallel()
	id := codexTestID(1)
	root := t.TempDir()
	real := writeRollout(t, root, "2026/10/02", "2026-10-02T10-00-00", id, codexMetaLine(t, id, codexTestWork, 0))
	secret := filepath.Join(t.TempDir(), "secret.jsonl")
	if err := os.WriteFile(secret, []byte(codexMetaLine(t, codexTestID(9), "/elsewhere", 0)), 0o600); err != nil {
		t.Fatal(err)
	}

	// The window between the open and the check: the leaf becomes a symlink to a
	// file outside the tree. What comes back must not be that file.
	f, ok := openCodexRolloutHook(root, real, func() {
		if err := os.Remove(real); err != nil {
			t.Error(err)
		}
		if err := os.Symlink(secret, real); err != nil {
			t.Error(err)
		}
	})
	if ok {
		defer f.Close() //nolint:errcheck,gosec // read-only
		if gotID, _, _ := readCodexMeta(f); gotID == codexTestID(9) {
			t.Fatal("the rollout read was the swapped-in outside file")
		}
		t.Fatal("a rollout whose path no longer names the opened file was accepted")
	}
}

func TestOpenCodexRolloutRefusesNonRegular(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "2026", "10", "02")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if codexRollout(root, dir) {
		t.Error("a directory was accepted as a rollout")
	}
}
