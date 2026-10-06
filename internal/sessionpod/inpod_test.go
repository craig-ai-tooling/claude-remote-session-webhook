package sessionpod

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

const (
	inPodCodexID  = "019a0000-0000-7000-8000-000000000042"
	inPodClaudeID = "7f3a1b2c-4d5e-4f60-8a71-b2c3d4e5f607"
)

// claudeProjectDir is how the Claude CLI names a project directory under
// ~/.claude/projects: separators and dots become dashes.
func claudeProjectDir(workDir string) string {
	return strings.Map(func(r rune) rune {
		if r == filepath.Separator || r == '.' {
			return '-'
		}
		return r
	}, workDir)
}

func TestCodexConversationHomeResolution(t *testing.T) {
	t.Parallel()
	fake := tmuxctl.NewFake()
	got, err := CodexConversation(context.Background(), fake, "crswd-x", t.TempDir(), "")
	if err != nil || got != "" {
		t.Fatalf("got %q, %v; want empty, nil", got, err)
	}
	for _, c := range fake.Calls() {
		if c.Op == tmuxctl.OpPanePID {
			t.Fatal("asked tmux for a pid with no codex home")
		}
	}
}

func TestCodexConversationFromProc(t *testing.T) {
	t.Parallel()
	codexHome := t.TempDir()
	day := filepath.Join(codexHome, "sessions", "2026", "10", "06")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(day, "rollout-2026-10-06T00-11-13-"+inPodCodexID+".jsonl")
	if err := os.WriteFile(rollout, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	proc := t.TempDir()
	fdDir := filepath.Join(proc, "100", "fd")
	if err := os.MkdirAll(fdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rollout, filepath.Join(fdDir, "3")); err != nil {
		t.Fatal(err)
	}
	fake := tmuxctl.NewFake()
	fake.SetPanePID("crswd-x", 100)
	got, err := CodexConversation(context.Background(), fake, "crswd-x", proc, codexHome)
	if err != nil {
		t.Fatal(err)
	}
	if got != inPodCodexID {
		t.Fatalf("id = %q, want %q", got, inPodCodexID)
	}
}

func TestPodHasTranscript(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "proj")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	home, codexHome := t.TempDir(), t.TempDir()

	project := filepath.Join(home, ".claude", "projects", claudeProjectDir(work))
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, inPodClaudeID+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	day := filepath.Join(codexHome, "sessions", "2026", "10", "06")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(map[string]any{
		"type":    "session_meta",
		"payload": map[string]any{"id": inPodCodexID, "cwd": work},
	})
	if err != nil {
		t.Fatal(err)
	}
	rollout := filepath.Join(day, "rollout-2026-10-06T10-00-00-"+inPodCodexID+".jsonl")
	if err := os.WriteFile(rollout, append(meta, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	env := []string{"HOME=" + home, "CODEX_HOME=" + codexHome}
	outside := t.TempDir()

	tests := []struct {
		name string
		h    harness.Name
		id   string
		dir  string
		want bool
	}{
		{"codex rollout", harness.Codex, inPodCodexID, work, true},
		{"claude transcript", harness.Claude, inPodClaudeID, work, true},
		{"other harness", harness.Other, inPodClaudeID, work, false},
		{"claude workdir outside the root", harness.Claude, inPodClaudeID, outside, false},
		{"codex workdir outside the root", harness.Codex, inPodCodexID, outside, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasTranscriptIn(root, tt.h, tt.id, tt.dir, env); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
