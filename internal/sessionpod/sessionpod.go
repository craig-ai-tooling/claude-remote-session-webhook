// Package sessionpod is the in-pod half of kubernetes mode (spec 017, FR-002,
// FR-011, FR-015): what runs inside a session pod, next to the tmux server that
// holds the one session the pod exists for.
//
// Two entrypoints, and neither is dispatched from cmd/crswd. The kubernetes
// module wires them as `session-pod` and `pane-loop` (k8s-20c slice S7), so the
// binary that ships as the host daemon gains no code path from this package
// (FR-004).
//
//   - Run resolves the working directory INSIDE the pod, refuses one that
//     escapes the approved root, starts the tmux session with a login shell
//     only, and returns when that session is gone.
//   - PaneLoop is the read loop FR-011 describes: one capture a second, each
//     frame followed by the byte 0x1E, written to a stream the daemon holds open
//     through the Kubernetes exec API.
//
// **Nothing here builds a byte that reaches a pane.** Run starts a shell and
// stops there, so the Claude command line still has exactly one author, the
// daemon's supervisor (FR-002), and PaneLoop only reads. Every tmux command is
// run through tmuxctl, which is exec of an argv slice with no shell in it.
//
// Standard library and this repository only. docs/security.md §5 holds go.sum
// absent, and TestNoDependenciesOutsideTheStandardLibrary fails the day this
// package imports something that would need one.
package sessionpod

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

const (
	// Socket is the tmux server a session pod runs, named with -L. There is one
	// server and one session per pod, so a fixed name is enough, and it must be
	// fixed: Run and PaneLoop are separate processes and both have to reach the
	// server the first one started.
	//
	// **The exported tmuxctl.Argv* wrappers carry no -L.** A caller that runs
	// them in a pod through the exec API (the daemon's second Controller) must
	// prepend "-L", Socket after argv[0], or it addresses tmux's default server
	// and finds no session.
	Socket = "crswd-session"

	// FrameSeparator ends every frame PaneLoop writes. tmux never stores a C0
	// control byte in a cell, so a screen cannot legitimately contain it, which
	// is what makes it usable as a delimiter; encodeFrame still removes it,
	// because that is a fact about tmux and not something this stream may lean
	// on.
	FrameSeparator byte = 0x1e

	// DefaultInterval is the period of both loops: the frame rate FR-011 states,
	// and how often Run asks whether the session is still there.
	DefaultInterval = time.Second

	// callTimeout bounds one tmux command. A tmux that hangs would otherwise
	// hang the loop with it, and a loop that never returns from a capture never
	// writes, so it can never find out that the stream it writes to is cut.
	callTimeout = 5 * time.Second

	// maxAskFailures is how many polls in a row may fail to establish whether the
	// session exists before Run gives up with an error. It is a number of polls
	// rather than a duration so it does not change with the interval.
	maxAskFailures = 5
)

// passThrough is what the session's environment carries beyond
// config.SessionEnvironment's own base set. CLAUDE_CONFIG_DIR is not in that
// set, and it is where FR-015's credential symlink and the session's transcripts
// live: without it Claude in the pane would look in $HOME and find nothing.
var passThrough = []string{"CLAUDE_CONFIG_DIR"}

// Pod runs the two entrypoints against any tmuxctl.Controller. Run and PaneLoop
// at package level build the real one; the type exists so a test can hand it the
// fake and a short interval.
type Pod struct {
	Tmux tmuxctl.Controller

	// Interval is the period of the frame loop and of the liveness poll. Zero
	// means DefaultInterval, so a Pod built as a literal behaves as production
	// does.
	Interval time.Duration
}

func (p *Pod) interval() time.Duration {
	if p.Interval > 0 {
		return p.Interval
	}
	return DefaultInterval
}

// Run is the session pod's entrypoint. It returns nil when the tmux session it
// started is gone, and an error for everything else, so the container's exit
// status says which.
//
// The working directory is resolved here, with every symlink followed, and only
// the result is tested against roots: that is constitution VI's "resolved and
// verified", and it has to happen in the pod because the daemon cannot see this
// filesystem. It runs before any tmux command, so a refused directory leaves no
// session behind. roots are consumed as they are and never re-resolved, exactly
// as session.ResolveWorkDir treats the host's, so each must already be an
// absolute, cleaned, symlink-free path.
//
// It starts nothing but the login shell. The daemon's supervisor sends the
// Claude command, as it does on the host, so a pod deleted while the daemon is
// down sits at a shell until the daemon returns.
//
// A cancelled ctx is the pod being stopped. Run kills the session, confirms it
// is gone, and returns ctx.Err(); a session it cannot confirm gone is an error,
// because an orphaned session is a live shell with nobody watching it.
func (p *Pod) Run(ctx context.Context, name, workdir string, roots []config.ApprovedRoot) error {
	if err := session.ValidateName(name); err != nil {
		return fmt.Errorf("session pod: %w", err)
	}
	resolved, err := session.ResolveWorkDir(workdir, roots)
	if err != nil {
		return fmt.Errorf("session pod %s: %w", name, err)
	}
	if err := p.Tmux.New(ctx, name, resolved); err != nil {
		return fmt.Errorf("session pod %s: start: %w", name, err)
	}

	ticker := time.NewTicker(p.interval())
	defer ticker.Stop()

	failures := 0
	for {
		select {
		case <-ctx.Done():
			return p.stop(ctx, name)
		case <-ticker.C:
		}

		gone, err := p.gone(ctx, name)
		if err != nil {
			failures++
			if failures >= maxAskFailures {
				return fmt.Errorf("session pod %s: cannot tell whether the session is still there: %w", name, err)
			}
			continue
		}
		if gone {
			return nil
		}
		failures = 0
	}
}

// gone reports whether the session has ended, and an error when it cannot tell.
//
// **has-session alone never says "gone" for the last session on a server.** tmux
// exits with its final session, and has-session on a server that is not running
// answers "no server running", which tmuxctl.Exec.Has reports as an error and
// never as false: "we could not ask" must not be read as "it is gone". This pod
// holds exactly one session, so its end is always that case. list-sessions maps
// the same answer to an empty list, and an empty list that does not name the
// session is an answer. Only when both commands fail is the question open.
func (p *Pod) gone(ctx context.Context, name string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	has, hasErr := p.Tmux.Has(ctx, name)
	if hasErr == nil {
		return !has, nil
	}
	sessions, listErr := p.Tmux.List(ctx)
	if listErr != nil {
		return false, fmt.Errorf("has-session: %w; list-sessions: %w", hasErr, listErr)
	}
	for _, s := range sessions {
		if s.Name == name {
			return false, nil
		}
	}
	return true, nil
}

// stop is what a cancelled Run does: kill the session and check that it went,
// on a context of its own because the caller's is already cancelled.
func (p *Pod) stop(ctx context.Context, name string) error {
	kill, cancel := context.WithTimeout(context.WithoutCancel(ctx), callTimeout)
	defer cancel()

	killErr := p.Tmux.Kill(kill, name)
	// Whatever kill said, the session's presence is what matters: the last
	// session on a server takes the server with it, and a kill racing that exit
	// reports an error for a session that is, by then, gone.
	if gone, err := p.gone(kill, name); err == nil && gone {
		return ctx.Err()
	}
	if killErr != nil {
		return fmt.Errorf("session pod %s: stop: %w", name, killErr)
	}
	return fmt.Errorf("session pod %s: stop: the session is still there after kill-session", name)
}

// PaneLoop writes one frame every interval to w: the pane as tmux renders it,
// then FrameSeparator. It is the loop FR-011 puts inside the pod, and it is
// meant to be held open by an exec stream whose far end is the daemon.
//
// **It returns the moment a write fails.** That is the only way this process
// learns the stream was cut, and a loop that outlived its stream would be an
// orphan in the pod, one more per reconnect. Nothing here swallows an error for
// the same reason: a capture that fails ends the loop as well, because a loop
// that wrote nothing could not notice a cut stream at all. The daemon reopens
// the stream when this one ends.
//
// The first frame is written at once, so a reader that has just connected does
// not wait a period for a screen. Each frame goes out in a single Write, so a
// reader never sees a frame's text separated from its terminator by another
// writer's bytes.
func (p *Pod) PaneLoop(ctx context.Context, name string, w io.Writer) error {
	if err := session.ValidateName(name); err != nil {
		return fmt.Errorf("pane loop: %w", err)
	}

	ticker := time.NewTicker(p.interval())
	defer ticker.Stop()

	for {
		screen, err := p.capture(ctx, name)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("pane loop %s: %w", name, err)
		}
		if _, err := w.Write(encodeFrame(screen)); err != nil {
			return fmt.Errorf("pane loop %s: write: %w", name, err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Pod) capture(ctx context.Context, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return p.Tmux.CapturePane(ctx, name)
}

// encodeFrame is the screen followed by FrameSeparator, with any separator byte
// already in the screen removed so it cannot be read as a boundary.
func encodeFrame(screen string) []byte {
	clean := strings.ReplaceAll(screen, string(rune(FrameSeparator)), "")
	return append([]byte(clean), FrameSeparator)
}

// environment is the whole environment tmux, and so the session, receives in a
// pod. It is composed by the function the host daemon uses, and never the
// process's own: a pod's environment is fixed by the reconciler and holds no
// daemon secret, but "a session never receives a configuration variable" is a
// property of config.SessionEnvironment's result, and this keeps it one.
func environment(parent []string) []string {
	return config.SessionEnvironment(parent, passThrough)
}

// newExec is the real controller: the pod's own tmux server, the pane bound the
// host daemon defaults to, and the composed environment.
func newExec() (*tmuxctl.Exec, error) {
	return tmuxctl.NewExec(Socket, config.DefaultPaneBound, environment(os.Environ()))
}

// stopSignals are what the kubelet and a terminal send to end a process: pod
// deletion is SIGTERM to the container's first process, and SIGINT is a person
// at a terminal.
func stopSignals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
}

// Run is the `session-pod` entrypoint: it builds the real controller and runs a
// Pod on it until the session ends or the process is told to stop. A stop
// signal is the pod being ended in order, so it is a clean exit; anything else
// that returns an error is a non-zero one.
func Run(name, workdir string, roots []config.ApprovedRoot) error {
	tmux, err := newExec()
	if err != nil {
		return fmt.Errorf("session pod %s: %w", name, err)
	}
	ctx, stop := stopSignals()
	defer stop()

	return orderly(ctx, (&Pod{Tmux: tmux}).Run(ctx, name, workdir, roots))
}

// PaneLoop is the `pane-loop` entrypoint, with the same controller and the same
// treatment of a stop signal as Run.
func PaneLoop(name string, w io.Writer) error {
	tmux, err := newExec()
	if err != nil {
		return fmt.Errorf("pane loop %s: %w", name, err)
	}
	ctx, stop := stopSignals()
	defer stop()

	return orderly(ctx, (&Pod{Tmux: tmux}).PaneLoop(ctx, name, w))
}

// orderly turns "we were told to stop, and did" into success. It looks at ctx as
// well as at the error, so a failure that merely wraps context.Canceled from
// somewhere else is not mistaken for one.
func orderly(ctx context.Context, err error) error {
	if err != nil && ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
