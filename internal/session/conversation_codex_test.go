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
			gotID, gotCwd, ok := readCodexMeta(p)
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
		if _, _, ok := readCodexMeta(filepath.Join(t.TempDir(), "none")); ok {
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
