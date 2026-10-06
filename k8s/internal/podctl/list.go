package podctl

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// replayOrder is every annotation-backed option a repair writes, in the order
// session.markSession writes them, with @crswd-width where one was annotated.
// OptionManaged is not here: it goes last, on its own.
var replayOrder = []string{
	tmuxctl.OptionOwner, tmuxctl.OptionName, tmuxctl.OptionWorkDir, tmuxctl.OptionStart,
	tmuxctl.OptionLifetime, tmuxctl.OptionConversation, tmuxctl.OptionBinary, tmuxctl.OptionWidth,
}

// List reads each session from its AgentSession and, for a Running pod, from
// the pod's own tmux. The object is the record: a pod whose tmux disagrees with
// it (a recreated pod, or a SetOption whose pod half failed) is repaired.
func (c *Controller) List(ctx context.Context) ([]tmuxctl.SessionInfo, error) {
	objects, err := c.sessions.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("podctl: list: %w", err)
	}
	rows := make([]tmuxctl.SessionInfo, 0, len(objects))
	for _, obj := range objects {
		if obj.Status.Phase == v1alpha1.PhaseRejected || obj.Status.Phase == v1alpha1.PhaseFailed {
			continue
		}
		base := rowFromObject(obj)
		rows = append(rows, c.observe(ctx, obj, base))
	}
	return rows, nil
}

func rowFromObject(obj v1alpha1.AgentSession) tmuxctl.SessionInfo {
	a := obj.Metadata.Annotations
	workDir := ""
	if b, err := base64.StdEncoding.DecodeString(a[annotationKey(tmuxctl.OptionWorkDir)]); err == nil {
		workDir = string(b)
	}
	return tmuxctl.SessionInfo{
		Name: obj.Metadata.Name,
		// Always the object's: a revived pod's tmux is younger than the session.
		Created:        obj.Metadata.CreationTimestamp,
		Managed:        a[annotationKey(tmuxctl.OptionManaged)] == tmuxctl.OptionManagedValue,
		Label:          a[annotationKey(tmuxctl.OptionName)],
		WorkDir:        workDir,
		StartCommand:   a[annotationKey(tmuxctl.OptionStart)],
		Lifetime:       a[annotationKey(tmuxctl.OptionLifetime)],
		Width:          a[annotationKey(tmuxctl.OptionWidth)],
		ConversationID: a[annotationKey(tmuxctl.OptionConversation)],
		Claude:         tmuxctl.LivenessUnknown,
	}
}

// observe layers the pod's answer over the base row. Any failure leaves the
// base row, whose Unknown liveness the supervisor reads as alive.
func (c *Controller) observe(ctx context.Context, obj v1alpha1.AgentSession, base tmuxctl.SessionInfo) tmuxctl.SessionInfo {
	pod, err := c.pods.CoreV1().Pods(c.cfg.Namespace).Get(ctx, base.Name, metav1.GetOptions{})
	if err != nil || pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
		return base
	}
	live, ok := c.listRow(ctx, base.Name)
	if !ok {
		return base
	}
	if base.Managed && drifted(base, live) {
		if c.replay(ctx, obj) != nil {
			return base
		}
		again, ok := c.listRow(ctx, base.Name)
		if !ok {
			return base
		}
		live = again
	}
	base.Claude = live.Claude
	if live.ConversationID != "" {
		base.ConversationID = live.ConversationID
	}
	return base
}

// listRow runs list-sessions in the pod and finds the session's own row.
func (c *Controller) listRow(ctx context.Context, name string) (tmuxctl.SessionInfo, bool) {
	r, err := c.run(ctx, name, inPod(tmuxctl.ArgvList()), nil, maxHistoryBytes)
	if err != nil || r.code != 0 || r.over {
		return tmuxctl.SessionInfo{}, false
	}
	parsed, err := tmuxctl.ParseSessions(r.stdout)
	if err != nil {
		return tmuxctl.SessionInfo{}, false
	}
	for _, row := range parsed {
		if row.Name == name {
			return row, true
		}
	}
	return tmuxctl.SessionInfo{}, false
}

// drifted is whether the pod's tmux lacks what the object records: no managed
// mark, no @crswd-binary, or any recorded field different.
func drifted(base, live tmuxctl.SessionInfo) bool {
	return !live.Managed || live.Claude == tmuxctl.LivenessUnknown ||
		live.Label != base.Label || live.WorkDir != base.WorkDir ||
		live.StartCommand != base.StartCommand || live.Lifetime != base.Lifetime ||
		live.Width != base.Width || live.ConversationID != base.ConversationID
}

// replay writes every annotated option into the pod's tmux, @crswd-managed
// last: a repair cut off part-way still reads as unmanaged and is retried whole.
func (c *Controller) replay(ctx context.Context, obj v1alpha1.AgentSession) error {
	name := obj.Metadata.Name
	opts := append(append([]string(nil), replayOrder...), tmuxctl.OptionManaged)
	for _, option := range opts {
		value, ok := obj.Metadata.Annotations[annotationKey(option)]
		if !ok {
			continue
		}
		if _, err := c.must(ctx, "set-option", name, inPod(tmuxctl.ArgvSetOption(name, option, value)), nil); err != nil {
			return err
		}
	}
	return nil
}
