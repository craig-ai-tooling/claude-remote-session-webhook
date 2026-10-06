package podctl

import (
	"context"
	"fmt"
	"strings"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CodexConversation asks the pod which Codex conversation its pane holds. An
// id that is not a conversation identifier is an error and is never recorded.
//
// A pod that is not yet running its session container has no answer to give, and
// exec'ing into it only logs "container not found". That is "" and no error, which
// the caller reads as not yet known.
func (c *Controller) CodexConversation(ctx context.Context, name string) (string, error) {
	if ready, err := c.sessionRunning(ctx, name); err != nil {
		return "", fmt.Errorf("podctl: codex-conversation %s: %w", name, err)
	} else if !ready {
		return "", nil
	}
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
//
// A pod whose session container is not running yet is ErrNotReady, never a no:
// a resume check that read "pod starting" as "no transcript" would end a session
// that has one.
func (c *Controller) HasTranscript(ctx context.Context, name string, h harness.Name, id, workDir string) (bool, error) {
	if ready, err := c.sessionRunning(ctx, name); err != nil {
		return false, fmt.Errorf("podctl: has-transcript %s: %w", name, err)
	} else if !ready {
		return false, fmt.Errorf("podctl: has-transcript %s: %w", name, ErrNotReady)
	}
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

// sessionRunning reports whether the pod exists, is Running and is not being
// deleted, and whether its session container is running. It is the same pod
// state Has reads, so the two agree about what "up" means.
func (c *Controller) sessionRunning(ctx context.Context, name string) (bool, error) {
	pod, err := c.pods.CoreV1().Pods(c.cfg.Namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("get pod: %w", err)
	}
	if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return false, nil
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Name == Container {
			return cs.State.Running != nil, nil
		}
	}
	return false, nil
}
