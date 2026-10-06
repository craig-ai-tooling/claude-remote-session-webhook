package podctl

import (
	"context"
	"fmt"
	"strings"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
)

// CodexConversation asks the pod which Codex conversation its pane holds. An
// id that is not a conversation identifier is an error and is never recorded.
func (c *Controller) CodexConversation(ctx context.Context, name string) (string, error) {
	r, err := c.must(ctx, "codex-conversation", name, []string{Binary, "codex-conversation", name}, nil)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(r.stdout)
	if id == "" {
		return "", nil
	}
	// The value stays out of the error: it came from the pod.
	if _, err := session.ValidateResume(id); err != nil {
		return "", fmt.Errorf("podctl: codex-conversation %s: %w", name, err)
	}
	return id, nil
}

// HasTranscript reports whether the pod holds the conversation's transcript.
// Exit 1 is a definite no; every other failure is an error, because only a
// definite answer may end a session.
func (c *Controller) HasTranscript(ctx context.Context, name string, h harness.Name, id, workDir string) (bool, error) {
	r, err := c.run(ctx, name, []string{Binary, "has-transcript", name, string(h), id, workDir}, nil, maxStderrBytes)
	if err != nil {
		return false, fmt.Errorf("podctl: has-transcript %s: %w", name, err)
	}
	switch r.code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	}
	return false, exitError("has-transcript", name, r)
}
