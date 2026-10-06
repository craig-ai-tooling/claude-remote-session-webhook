package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

// transcriptRig is a root with one working directory in it, a home holding a
// Claude transcript for it and a Codex home holding a rollout for it.
type transcriptRig struct {
	roots     []config.ApprovedRoot
	work      string
	home      string
	codexHome string
}

const (
	transcriptClaudeID = "7f3a1b2c-4d5e-4f60-8a71-b2c3d4e5f607"
	transcriptCodexID  = "019a0000-0000-7000-8000-000000000042"
)

func newTranscriptRig(t *testing.T) transcriptRig {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "proj")
	if err := os.MkdirAll(work, 0o700); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	project := filepath.Join(home, ".claude", "projects", projectDirFor(work))
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, transcriptClaudeID+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	codexHome := t.TempDir()
	writeRollout(t, filepath.Join(codexHome, "sessions"), "2026/10/06", "2026-10-06T10-00-00",
		transcriptCodexID, codexMetaLine(t, transcriptCodexID, work, 0))
	return transcriptRig{roots: []config.ApprovedRoot{{Path: root}}, work: work, home: home, codexHome: codexHome}
}

func TestTranscriptExistsClaude(t *testing.T) {
	t.Parallel()
	r := newTranscriptRig(t)
	if !TranscriptExists(harness.Claude, transcriptClaudeID, r.work, r.home, "", r.roots) {
		t.Fatal("a transcript that is there was not found")
	}
	if TranscriptExists(harness.Claude, "00000000-0000-4000-8000-000000000000", r.work, r.home, "", r.roots) {
		t.Fatal("a transcript that is not there was found")
	}
	if TranscriptExists(harness.Claude, transcriptClaudeID, r.work, "", "", r.roots) {
		t.Fatal("found with no home")
	}
}

func TestTranscriptExistsCodex(t *testing.T) {
	t.Parallel()
	r := newTranscriptRig(t)
	if !TranscriptExists(harness.Codex, transcriptCodexID, r.work, "", r.codexHome, r.roots) {
		t.Fatal("a rollout that is there was not found")
	}
	if TranscriptExists(harness.Codex, transcriptCodexID, r.work, "", "", r.roots) {
		t.Fatal("found with no codex home")
	}
	if TranscriptExists(harness.Codex, codexTestID(9), r.work, "", r.codexHome, r.roots) {
		t.Fatal("found a rollout with another id")
	}
}

func TestTranscriptExistsOther(t *testing.T) {
	t.Parallel()
	r := newTranscriptRig(t)
	if TranscriptExists(harness.Other, transcriptClaudeID, r.work, r.home, r.codexHome, r.roots) {
		t.Fatal("true for a harness with no transcripts")
	}
}

func TestTranscriptExistsOutsideRoots(t *testing.T) {
	t.Parallel()
	r := newTranscriptRig(t)
	outside := t.TempDir()
	if TranscriptExists(harness.Claude, transcriptClaudeID, outside, r.home, "", r.roots) {
		t.Fatal("claude: a workdir outside the roots was accepted")
	}
	if TranscriptExists(harness.Codex, transcriptCodexID, outside, "", r.codexHome, r.roots) {
		t.Fatal("codex: a workdir outside the roots was accepted")
	}
}
