package sessionpod

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

// CodexConversation is the in-pod half of Codex conversation discovery (spec
// 019 FR-023). The pane's process tree lives in this pod, not in the daemon's,
// so the daemon asks here instead of reading its own /proc.
//
// An empty codexHome means Codex keeps nothing in this pod: the answer is "no
// conversation" and tmux is not asked.
func CodexConversation(ctx context.Context, tmux tmuxctl.Controller, name, procRoot, codexHome string) (string, error) {
	if codexHome == "" {
		return "", nil
	}
	pid, err := tmux.PanePID(ctx, name)
	if err != nil {
		return "", err
	}
	return session.DiscoverCodexConversation(procRoot, pid, filepath.Join(codexHome, "sessions"))
}

// HasTranscript answers whether this pod holds the conversation's transcript.
// The one approved root is WorkRoot, the same one Run resolves against.
func HasTranscript(h harness.Name, id, workDir string, env []string) bool {
	return hasTranscriptIn(WorkRoot, h, id, workDir, env)
}

// hasTranscriptIn takes the root as a parameter only so a test can use a
// temporary directory; production always passes WorkRoot.
func hasTranscriptIn(root string, h harness.Name, id, workDir string, env []string) bool {
	home := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "HOME="); ok {
			home = v
		}
	}
	return session.TranscriptExists(h, id, workDir, home, session.CodexHome(env), []config.ApprovedRoot{{Path: root}})
}

// NewInPodExec is the pod's own tmux server as a controller, for the
// `codex-conversation` subcommand the kubernetes module dispatches.
func NewInPodExec() (*tmuxctl.Exec, error) { return newExec() }
