package podctl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// maxHistoryBytes is tmuxctl's own bound on a captured history.
	maxHistoryBytes = 4 << 20
	// maxStderrBytes keeps a chatty remote command from filling memory; only
	// the first line of stderr is ever reported.
	maxStderrBytes = 4 << 10
	// maxStderrLine bounds the one line that reaches an error string.
	maxStderrLine = 200
)

// capBuffer keeps at most max bytes and records that more arrived. Write
// reports the full length so a remote stream is drained rather than failed.
type capBuffer struct {
	buf  bytes.Buffer
	max  int
	over bool
}

func (b *capBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if room := b.max - b.buf.Len(); n > room {
		b.over = true
		p = p[:max(room, 0)]
	}
	b.buf.Write(p)
	return n, nil
}

type result struct {
	stdout, stderr string
	code           int
	over           bool
}

// run is one exec with both streams bounded. err is a transport failure; a
// non-zero exit is in result.code.
func (c *Controller) run(ctx context.Context, pod string, argv []string, stdin io.Reader, maxOut int) (result, error) {
	out, errBuf := &capBuffer{max: maxOut}, &capBuffer{max: maxStderrBytes}
	code, err := c.exec.Exec(ctx, pod, argv, stdin, out, errBuf)
	return result{stdout: out.buf.String(), stderr: errBuf.buf.String(), code: code, over: out.over}, err
}

// must runs argv and turns a transport failure or a non-zero exit into an
// error. The argv and the stdin never appear in it, because both can carry the
// session's text (docs/security.md §3).
func (c *Controller) must(ctx context.Context, op, pod string, argv []string, stdin io.Reader) (result, error) {
	r, err := c.run(ctx, pod, argv, stdin, maxHistoryBytes)
	if err != nil {
		return r, fmt.Errorf("podctl: %s %s: %w", op, pod, err)
	}
	if r.code != 0 {
		return r, exitError(op, pod, r)
	}
	return r, nil
}

func exitError(op, pod string, r result) error {
	line, _, _ := strings.Cut(strings.TrimSpace(r.stderr), "\n")
	if len(line) > maxStderrLine {
		line = line[:maxStderrLine]
	}
	return fmt.Errorf("podctl: %s %s: exit %d: %s", op, pod, r.code, line)
}

// New asks the reconciler for the session's pod by creating its AgentSession,
// then waits until tmux answers inside it.
func (c *Controller) New(ctx context.Context, name, workDir string) error {
	if c.describe == nil {
		return fmt.Errorf("podctl: no session record for %s", name)
	}
	rec, ok := c.describe(name)
	if !ok {
		return fmt.Errorf("podctl: no session record for %s", name)
	}
	if rec.LifetimeDisabled {
		return errors.New("podctl: a session in kubernetes mode needs a finite lifetime")
	}
	_, found, err := c.sessions.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("podctl: new %s: %w", name, err)
	}
	if !found {
		// Truncated down: the server stamps creationTimestamp after now, and
		// the pod's deadline must not land past the session's.
		d := rec.Deadline.Sub(c.cfg.Now()).Truncate(time.Second)
		if d <= 0 {
			return fmt.Errorf("podctl: new %s: the session has no lifetime left", name)
		}
		_, err := c.sessions.Create(ctx, v1alpha1.AgentSession{
			Metadata: v1alpha1.ObjectMeta{Name: name},
			Spec: v1alpha1.AgentSessionSpec{
				SessionName: rec.Name, Owner: rec.Owner, WorkDir: workDir,
				StartCommand: rec.StartCommand, Conversation: rec.ConversationID, Lifetime: d.String(),
			},
		})
		if err != nil {
			return fmt.Errorf("podctl: new %s: %w", name, err)
		}
	}
	return c.awaitReady(ctx, name)
}

func (c *Controller) awaitReady(parent context.Context, name string) error {
	ctx, cancel := context.WithTimeout(parent, c.cfg.ReadyTimeout)
	defer cancel()
	tick := time.NewTicker(c.cfg.PollInterval)
	defer tick.Stop()
	var last string
	for {
		ready, seen, err := c.readyOnce(ctx, name)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		last = seen
		select {
		case <-ctx.Done():
			if parent.Err() != nil {
				return fmt.Errorf("podctl: new %s: %w", name, parent.Err())
			}
			return fmt.Errorf("podctl: new %s: not ready after %s, last seen: %s", name, c.cfg.ReadyTimeout, last)
		case <-tick.C:
		}
	}
}

// readyOnce is one poll. A tmux that does not answer yet is not an error: a
// pod reports Running before its entrypoint has started the server.
func (c *Controller) readyOnce(ctx context.Context, name string) (bool, string, error) {
	obj, found, err := c.sessions.Get(ctx, name)
	if err != nil && ctx.Err() == nil {
		return false, "", fmt.Errorf("podctl: new %s: %w", name, err)
	}
	if found && (obj.Status.Phase == v1alpha1.PhaseRejected || obj.Status.Phase == v1alpha1.PhaseFailed) {
		return false, "", fmt.Errorf("podctl: new %s: %s: %s", name, obj.Status.Phase, obj.Status.Reason)
	}
	pod, err := c.pods.CoreV1().Pods(c.cfg.Namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		return false, "no pod yet", nil
	case err != nil && ctx.Err() == nil:
		return false, "", fmt.Errorf("podctl: new %s: get pod: %w", name, err)
	case err != nil:
		return false, "pod lookup cut short", nil
	case pod.Status.Phase != corev1.PodRunning:
		return false, "pod phase " + string(pod.Status.Phase), nil
	}
	r, err := c.run(ctx, name, inPod(tmuxctl.ArgvHas(name)), nil, maxStderrBytes)
	if err != nil || r.code != 0 {
		return false, "pod phase Running, tmux not answering", nil
	}
	return true, "", nil
}

// SetOption writes the AgentSession first: the object is the record, and a
// failure between the two halves leaves the pod behind it, which List repairs.
func (c *Controller) SetOption(ctx context.Context, name, option, value string) error {
	if err := c.sessions.SetAnnotation(ctx, name, annotationKey(option), value); err != nil {
		return fmt.Errorf("podctl: set option %s: %w", name, err)
	}
	_, err := c.must(ctx, "set-option", name, inPod(tmuxctl.ArgvSetOption(name, option, value)), nil)
	return err
}

// SendKeys sends daemon-authored key constants.
func (c *Controller) SendKeys(ctx context.Context, name string, keys ...string) error {
	_, err := c.must(ctx, "send-keys", name, inPod(tmuxctl.ArgvSendKeys(name, keys...)), nil)
	return err
}

// Paste delivers payload through a tmux buffer loaded over stdin.
func (c *Controller) Paste(ctx context.Context, name string, payload []byte) error {
	return c.paste(ctx, name, payload, tmuxctl.ArgvPaste)
}

// PasteBracketed is Paste with paste-buffer -p.
func (c *Controller) PasteBracketed(ctx context.Context, name string, payload []byte) error {
	return c.paste(ctx, name, payload, tmuxctl.ArgvPasteBracketed)
}

func (c *Controller) paste(ctx context.Context, name string, payload []byte, build func(buffer, name string) (load, paste []string)) error {
	buf, err := tmuxctl.NewBufferName()
	if err != nil {
		return err
	}
	load, paste := build(buf, name)
	if _, err := c.must(ctx, "load-buffer", name, inPod(load), bytes.NewReader(payload)); err != nil {
		return err
	}
	if _, err := c.must(ctx, "paste-buffer", name, inPod(paste), nil); err != nil {
		// A cancelled ctx would refuse the cleanup too, and the cleanup is the
		// one command that must still run.
		if _, delErr := c.must(context.WithoutCancel(ctx), "delete-buffer", name, inPod(tmuxctl.ArgvDeleteBuffer(buf)), nil); delErr != nil {
			return errors.Join(err, delErr)
		}
		return err
	}
	return nil
}

// Resize sets the pod window to cols by rows.
func (c *Controller) Resize(ctx context.Context, name string, cols, rows int) error {
	_, err := c.must(ctx, "resize-window", name, inPod(tmuxctl.ArgvResize(name, cols, rows)), nil)
	return err
}

// PanePID returns the pane process's id inside the pod's own pid namespace.
func (c *Controller) PanePID(ctx context.Context, name string) (int, error) {
	r, err := c.must(ctx, "pane-pid", name, inPod(tmuxctl.ArgvPanePID(name)), nil)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(r.stdout))
	if err != nil || pid < 1 {
		return 0, fmt.Errorf("podctl: pane pid of %s: %w", name, tmuxctl.ErrUnexpectedOutput)
	}
	return pid, nil
}

// CaptureHistory refuses rather than shortens: a truncated history reads as a
// complete one.
func (c *Controller) CaptureHistory(ctx context.Context, name string) (string, error) {
	r, err := c.run(ctx, name, inPod(tmuxctl.ArgvCaptureHistory(name)), nil, maxHistoryBytes)
	if err != nil {
		return "", fmt.Errorf("podctl: capture history %s: %w", name, err)
	}
	if r.over {
		return "", fmt.Errorf("podctl: capture history %s: %w: more than %d bytes", name, tmuxctl.ErrHistoryTooLarge, maxHistoryBytes)
	}
	if r.code != 0 {
		return "", exitError("capture history", name, r)
	}
	if lines := countLines(r.stdout); lines > tmuxctl.HistoryLimit {
		return "", fmt.Errorf("podctl: capture history %s: %w: %d lines", name, tmuxctl.ErrHistoryTooLarge, lines)
	}
	return tmuxctl.Strip(r.stdout), nil
}

// countLines is copied from internal/tmuxctl/exec.go: every pane line followed
// by a newline, and a final line with no newline still counts.
func countLines(screen string) int {
	if screen == "" {
		return 0
	}
	n := strings.Count(screen, "\n")
	if !strings.HasSuffix(screen, "\n") {
		n++
	}
	return n
}

// Has reports whether anything of the session is still alive, because
// session.Manager uses it to confirm a teardown: a pod on its way out, or an
// object whose pod the reconciler has yet to bring, both still count.
func (c *Controller) Has(ctx context.Context, name string) (bool, error) {
	pod, err := c.pods.CoreV1().Pods(c.cfg.Namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		if pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil {
			return true, nil
		}
		r, err := c.run(ctx, name, inPod(tmuxctl.ArgvHas(name)), nil, maxStderrBytes)
		if err != nil {
			return false, fmt.Errorf("podctl: has %s: %w", name, err)
		}
		switch {
		case r.code == 0:
			return true, nil
		case tmuxctl.Absent(r.code, r.stderr):
			return false, nil
		}
		return false, exitError("has-session", name, r)
	case !apierrors.IsNotFound(err):
		return false, fmt.Errorf("podctl: has %s: get pod: %w", name, err)
	}
	_, found, err := c.sessions.Get(ctx, name)
	if err != nil {
		return false, fmt.Errorf("podctl: has %s: %w", name, err)
	}
	return found, nil
}

// Kill deletes the AgentSession and waits for its pod to go. The pod's own
// termination grace applies; nothing here shortens it (spec 017 FR-010).
func (c *Controller) Kill(ctx context.Context, name string) error {
	c.stopStream(name)
	if err := c.sessions.Delete(ctx, name); err != nil {
		return fmt.Errorf("podctl: kill %s: %w", name, err)
	}
	wait, cancel := context.WithTimeout(ctx, c.cfg.KillTimeout)
	defer cancel()
	tick := time.NewTicker(c.cfg.PollInterval)
	defer tick.Stop()
	for {
		_, err := c.pods.CoreV1().Pods(c.cfg.Namespace).Get(wait, name, metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			return nil
		case err != nil && wait.Err() == nil:
			return fmt.Errorf("podctl: kill %s: get pod: %w", name, err)
		}
		select {
		case <-wait.Done():
			if ctx.Err() != nil {
				return fmt.Errorf("podctl: kill %s: %w", name, ctx.Err())
			}
			return fmt.Errorf("podctl: pod %s is still there after the session was deleted", name)
		case <-tick.C:
		}
	}
}

func (c *Controller) stopStream(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s := c.streams[name]; s != nil {
		s.cancel()
		delete(c.streams, name)
	}
}

// ReconcileServerEnvironment has nothing to do: each pod's environment is fixed
// by the reconciler's pod template, so there is no shared server table.
func (c *Controller) ReconcileServerEnvironment(context.Context) (tmuxctl.Reconciliation, error) {
	return tmuxctl.Reconciliation{}, nil
}
