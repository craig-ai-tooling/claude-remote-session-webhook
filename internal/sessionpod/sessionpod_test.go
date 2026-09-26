package sessionpod

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

const (
	podName = "crswd-9f2c4a1b8e6d3f7a0c5b2e9d4f1a7c3b"

	// Generous against a real scheduler and irrelevant to a passing test: every
	// wait below returns as soon as its condition holds, so this is only how long
	// a regression takes to be reported instead of hanging the package.
	deadline = 5 * time.Second
)

var errBoom = errors.New("boom")

// newPod is a Pod on the fake with a period short enough to run many of them.
func newPod() (*Pod, *tmuxctl.Fake) {
	f := tmuxctl.NewFake()
	return &Pod{Tmux: f, Interval: time.Millisecond}, f
}

// approvedRoot makes a directory and the one-entry allowlist that names it, the
// way config resolves a root: absolute, and with every symlink already followed.
// t.TempDir may itself sit behind a symlink, and a root that did would fail every
// containment check for a reason that has nothing to do with the test.
func approvedRoot(t *testing.T) (string, []config.ApprovedRoot) {
	t.Helper()

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve the temporary root: %v", err)
	}
	return root, []config.ApprovedRoot{{Path: root}}
}

func mkdir(t *testing.T, path string) string {
	t.Helper()

	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	return path
}

// runAsync starts Run and hands back what it returns. The channel is buffered so
// a test that gave up waiting leaves no goroutine blocked on the send.
func runAsync(ctx context.Context, p *Pod, name, workdir string, roots []config.ApprovedRoot) <-chan error {
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx, name, workdir, roots) }()
	return done
}

func paneLoopAsync(ctx context.Context, p *Pod, name string, w io.Writer) <-chan error {
	done := make(chan error, 1)
	go func() { done <- p.PaneLoop(ctx, name, w) }()
	return done
}

// result waits for a run to finish, and fails instead of hanging when it does not.
func result(t *testing.T, done <-chan error, what string) error {
	t.Helper()

	select {
	case err := <-done:
		return err
	case <-time.After(deadline):
		t.Fatalf("%s did not return within %s", what, deadline)
		return nil
	}
}

// waitUntil polls a condition, for the moments a test must not act before the
// goroutine under test has.
func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()

	end := time.Now().Add(deadline)
	for !cond() {
		if time.Now().After(end) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// hasSession asks the fake without going through Has, so the check neither shows
// up among the calls a test counts nor trips a failure knob the test has set.
func hasSession(f *tmuxctl.Fake, name string) bool {
	_, ok := f.WorkDir(name)
	return ok
}

// callsOf lists the recorded calls of the given kinds, in order, leaving out the
// polling that runs until the session ends.
func callsOf(f *tmuxctl.Fake, ops ...tmuxctl.Op) []tmuxctl.Call {
	var out []tmuxctl.Call
	for _, c := range f.Calls() {
		if slices.Contains(ops, c.Op) {
			out = append(out, c)
		}
	}
	return out
}

// Run returns when the session is gone, and the fake is where "gone" is made to
// happen: Vanish is a session whose shell exited.
func TestRunReturnsWhenTheSessionIsGone(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	root, roots := approvedRoot(t)
	workdir := mkdir(t, filepath.Join(root, "repo"))

	done := runAsync(context.Background(), p, podName, workdir, roots)
	waitUntil(t, "the session to exist", func() bool { return hasSession(f, podName) })

	select {
	case err := <-done:
		t.Fatalf("Run returned %v while the session was alive", err)
	case <-time.After(20 * time.Millisecond):
	}

	f.Vanish(podName)
	if err := result(t, done, "Run after the session vanished"); err != nil {
		t.Errorf("Run = %v, want nil once the session is gone", err)
	}
}

// The property that matters: a directory that is a symlink out of the root is
// refused, and refused before tmux is asked to do anything.
func TestRunRefusesAWorkDirThatIsASymlinkOutOfTheRoot(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	root, roots := approvedRoot(t)
	outside := mkdir(t, filepath.Join(t.TempDir(), "elsewhere"))
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	err := result(t, runAsync(context.Background(), p, podName, link, roots), "Run on an escaping symlink")
	if !errors.Is(err, session.ErrWorkDirOutsideRoots) {
		t.Fatalf("Run = %v, want an error wrapping ErrWorkDirOutsideRoots", err)
	}
	if calls := f.Calls(); len(calls) != 0 {
		t.Errorf("tmux was called %d times for a refused directory: %v", len(calls), calls)
	}
}

// Every other way a directory can be unacceptable, and each of them a refusal
// with no tmux call behind it.
func TestRunRefusesAWorkDirItCannotVouchFor(t *testing.T) {
	t.Parallel()

	root, roots := approvedRoot(t)
	file := filepath.Join(root, "afile")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write %s: %v", file, err)
	}
	sibling := mkdir(t, root+"-sibling")

	cases := []struct {
		name    string
		workdir string
		roots   []config.ApprovedRoot
		want    error
	}{
		{"a dot-dot escape", filepath.Join(root, "..", filepath.Base(sibling)), roots, session.ErrWorkDirOutsideRoots},
		{"a string-prefix lookalike of the root", sibling, roots, session.ErrWorkDirOutsideRoots},
		{"no approved root at all", root, nil, session.ErrWorkDirOutsideRoots},
		{"a relative path", "repo", roots, session.ErrWorkDirNotAbsolute},
		{"a path that does not exist", filepath.Join(root, "missing"), roots, session.ErrWorkDirUnresolvable},
		{"a file", file, roots, session.ErrWorkDirNotDirectory},
		{"an empty path", "", roots, session.ErrInvalidWorkDir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p, f := newPod()
			err := result(t, runAsync(context.Background(), p, podName, tc.workdir, tc.roots), "Run")
			if !errors.Is(err, tc.want) {
				t.Fatalf("Run = %v, want an error wrapping %v", err, tc.want)
			}
			if calls := f.Calls(); len(calls) != 0 {
				t.Errorf("tmux was called for a refused directory: %v", calls)
			}
		})
	}
}

// A link that stays inside the root is fine, and the session is created where the
// link points, not where it was spelled: the resolved path is the one that was
// checked, so it is the one that must be used.
func TestRunStartsTheSessionInTheResolvedDirectory(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	root, roots := approvedRoot(t)
	real := mkdir(t, filepath.Join(root, "real"))
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runAsync(ctx, p, podName, link, roots)
	waitUntil(t, "the session to exist", func() bool { return hasSession(f, podName) })

	if got, ok := f.WorkDir(podName); !ok || got != real {
		t.Errorf("session started in %q, want the resolved %q", got, real)
	}
	news := callsOf(f, tmuxctl.OpNew)
	if len(news) != 1 || !slices.Equal(news[0].Argv, tmuxctl.ArgvNew(podName, real)) {
		t.Errorf("new-session calls = %v, want exactly %v", news, tmuxctl.ArgvNew(podName, real))
	}

	cancel()
	if err := result(t, done, "Run after cancel"); !errors.Is(err, context.Canceled) {
		t.Errorf("Run = %v, want context.Canceled", err)
	}
}

// FR-002: no byte to a pane is built by new code. The pod starts a login shell
// and sets nothing and types nothing, so the daemon's supervisor stays the one
// author of the Claude command line.
func TestRunStartsALoginShellAndSendsNothing(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	root, roots := approvedRoot(t)
	workdir := mkdir(t, filepath.Join(root, "repo"))

	done := runAsync(context.Background(), p, podName, workdir, roots)
	waitUntil(t, "the session to exist", func() bool { return hasSession(f, podName) })
	waitUntil(t, "a poll after the start", func() bool { return len(callsOf(f, tmuxctl.OpHas)) > 1 })
	f.Vanish(podName)
	if err := result(t, done, "Run"); err != nil {
		t.Fatalf("Run = %v", err)
	}

	typed := callsOf(f, tmuxctl.OpSendKeys, tmuxctl.OpPaste, tmuxctl.OpSetOption, tmuxctl.OpResize, tmuxctl.OpKill)
	if len(typed) != 0 {
		t.Errorf("Run sent %d commands beyond the start: %v", len(typed), typed)
	}
	for _, c := range f.Calls() {
		if c.Argv[0] != "tmux" {
			t.Errorf("Run executed %q, which is not tmux; no shell may be involved", c.Argv)
		}
	}
}

func TestRunRefusesAnInvalidName(t *testing.T) {
	t.Parallel()

	root, roots := approvedRoot(t)
	for _, name := range []string{"", "crswd-a:b", "crswd-a.b", "-t x", strings.Repeat("a", session.MaxNameLen+1)} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p, f := newPod()
			err := result(t, runAsync(context.Background(), p, name, root, roots), "Run")
			if !errors.Is(err, session.ErrInvalidName) {
				t.Errorf("Run(%q) = %v, want ErrInvalidName", name, err)
			}
			if calls := f.Calls(); len(calls) != 0 {
				t.Errorf("tmux was called for an invalid name: %v", calls)
			}
		})
	}
}

// A real tmux answers has-session on a server that is gone with an error, not
// with false (see TestRunSeesTheLastSessionEndOnARealServer), so the fake is made
// to do the same and list-sessions has to carry the answer.
func TestRunTakesAnEmptyListAsTheSessionBeingGone(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.FailOp(tmuxctl.OpHas, errBoom)
	root, roots := approvedRoot(t)
	workdir := mkdir(t, filepath.Join(root, "repo"))

	done := runAsync(context.Background(), p, podName, workdir, roots)
	waitUntil(t, "a poll that failed to ask", func() bool { return len(callsOf(f, tmuxctl.OpHas)) > 0 })
	f.Vanish(podName)

	if err := result(t, done, "Run"); err != nil {
		t.Errorf("Run = %v, want nil: has-session failed, but list-sessions says the session is not there", err)
	}
}

// The other half: an error from has-session with the session still listed is not
// the end of anything. Run must keep waiting, or a transient tmux failure would
// take the pod's only session down with it.
func TestRunKeepsWaitingWhenHasFailsButTheSessionIsListed(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.FailOp(tmuxctl.OpHas, errBoom)
	root, roots := approvedRoot(t)
	workdir := mkdir(t, filepath.Join(root, "repo"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := runAsync(ctx, p, podName, workdir, roots)
	waitUntil(t, "many failed polls", func() bool { return len(callsOf(f, tmuxctl.OpHas)) > 3*maxAskFailures })

	select {
	case err := <-done:
		t.Fatalf("Run returned %v with the session still listed", err)
	default:
	}
	cancel()
	if err := result(t, done, "Run after cancel"); !errors.Is(err, context.Canceled) {
		t.Errorf("Run = %v, want context.Canceled", err)
	}
}

// "We could not ask" must never be recorded as "it is gone", and must not be
// recorded as success either: with has-session and list-sessions both failing,
// Run ends in an error, so the exit status is non-zero.
func TestRunFailsWhenTmuxCannotBeAsked(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.FailOp(tmuxctl.OpHas, errBoom)
	f.FailOp(tmuxctl.OpList, errBoom)
	root, roots := approvedRoot(t)
	workdir := mkdir(t, filepath.Join(root, "repo"))

	err := result(t, runAsync(context.Background(), p, podName, workdir, roots), "Run")
	if !errors.Is(err, errBoom) {
		t.Fatalf("Run = %v, want an error wrapping the tmux failure", err)
	}
	if !hasSession(f, podName) {
		t.Error("the session was removed; a failure to ask must not touch it")
	}
}

// A stop signal ends the session in order and confirms it, and a session that
// survives the kill is an error, never a clean exit.
func TestRunKillsAndConfirmsTheSessionOnCancel(t *testing.T) {
	t.Parallel()

	t.Run("the session goes", func(t *testing.T) {
		t.Parallel()

		p, f := newPod()
		root, roots := approvedRoot(t)
		workdir := mkdir(t, filepath.Join(root, "repo"))

		ctx, cancel := context.WithCancel(context.Background())
		done := runAsync(ctx, p, podName, workdir, roots)
		waitUntil(t, "the session to exist", func() bool { return hasSession(f, podName) })
		cancel()

		if err := result(t, done, "Run after cancel"); !errors.Is(err, context.Canceled) {
			t.Errorf("Run = %v, want context.Canceled", err)
		}
		if hasSession(f, podName) {
			t.Error("the session outlived a cancelled Run")
		}
		if kills := callsOf(f, tmuxctl.OpKill); len(kills) != 1 || !slices.Equal(kills[0].Argv, tmuxctl.ArgvKill(podName)) {
			t.Errorf("kill calls = %v, want exactly one %v", kills, tmuxctl.ArgvKill(podName))
		}
	})

	t.Run("the session survives the kill", func(t *testing.T) {
		t.Parallel()

		p, f := newPod()
		f.SurviveKill(podName)
		root, roots := approvedRoot(t)
		workdir := mkdir(t, filepath.Join(root, "repo"))

		ctx, cancel := context.WithCancel(context.Background())
		done := runAsync(ctx, p, podName, workdir, roots)
		waitUntil(t, "the session to exist", func() bool { return hasSession(f, podName) })
		cancel()

		err := result(t, done, "Run after cancel")
		if err == nil {
			t.Fatal("Run returned nil with the session still alive")
		}
		if errors.Is(err, context.Canceled) {
			t.Errorf("Run = %v, which reads as an orderly stop; the session is still there", err)
		}
	})
}

// frames splits what PaneLoop wrote back into screens, and fails if the stream
// does not end on a separator: every frame is terminated, including the last.
func frames(t *testing.T, stream []byte) []string {
	t.Helper()

	if len(stream) == 0 {
		return nil
	}
	if stream[len(stream)-1] != FrameSeparator {
		t.Fatalf("the stream ends in %q, not the separator", stream[len(stream)-1])
	}
	parts := strings.Split(string(stream[:len(stream)-1]), string(rune(FrameSeparator)))
	return parts
}

// writerFunc is an io.Writer for a test to script: what was written, and when to
// fail.
type writerFunc func(p []byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

// TestPaneLoopWritesSeparatedFrames is FR-011's loop: successive captures, each
// followed by 0x1E, the pane content untouched between them.
func TestPaneLoopWritesSeparatedFrames(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	screens := []string{"first\nscreen  \n", "second\n", "third\n\n\n"}
	f.SetPane(podName, screens[0])

	var (
		mu     sync.Mutex
		stream bytes.Buffer
		writes int
	)
	w := writerFunc(func(b []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		stream.Write(b)
		writes++
		if writes < len(screens) {
			f.SetPane(podName, screens[writes])
			return len(b), nil
		}
		return 0, errBoom
	})

	err := result(t, paneLoopAsync(context.Background(), p, podName, w), "PaneLoop")
	if !errors.Is(err, errBoom) {
		t.Fatalf("PaneLoop = %v, want the writer's error", err)
	}

	// The last write failed after being made, so what it carried is in the
	// buffer too.
	got := frames(t, stream.Bytes())
	if !slices.Equal(got, screens) {
		t.Errorf("frames = %q, want %q", got, screens)
	}
}

// Every frame leaves in one Write, terminator included.
func TestPaneLoopWritesEachFrameInOneWrite(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.SetPane(podName, "screen\n")

	var sizes []int
	var last byte
	w := writerFunc(func(b []byte) (int, error) {
		sizes = append(sizes, len(b))
		last = b[len(b)-1]
		if len(sizes) == 3 {
			return 0, errBoom
		}
		return len(b), nil
	})
	if err := result(t, paneLoopAsync(context.Background(), p, podName, w), "PaneLoop"); !errors.Is(err, errBoom) {
		t.Fatalf("PaneLoop = %v, want the writer's error", err)
	}

	if want := []int{len("screen\n") + 1, len("screen\n") + 1, len("screen\n") + 1}; !slices.Equal(sizes, want) {
		t.Errorf("write sizes = %v, want one write of screen+separator each: %v", sizes, want)
	}
	if last != FrameSeparator {
		t.Errorf("a write ended in %q, not the separator", last)
	}
}

// A separator byte inside a screen would be read by the daemon as the end of a
// frame. tmux does not produce one, and the stream must not depend on that.
func TestPaneLoopRemovesSeparatorBytesFromAScreen(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.SetPane(podName, "before\x1eafter\n")

	var out bytes.Buffer
	w := writerFunc(func(b []byte) (int, error) {
		out.Write(b)
		return 0, errBoom
	})
	if err := result(t, paneLoopAsync(context.Background(), p, podName, w), "PaneLoop"); !errors.Is(err, errBoom) {
		t.Fatalf("PaneLoop = %v, want the writer's error", err)
	}

	if got, want := out.String(), "beforeafter\n\x1e"; got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
}

// The orphan case. A stream that is cut fails the next write, and the loop must
// be gone by then: one capture, one failed write, no second capture.
func TestPaneLoopExitsWhenTheWriterFails(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.SetPane(podName, "screen\n")

	err := result(t, paneLoopAsync(context.Background(), p, podName, writerFunc(func([]byte) (int, error) { return 0, errBoom })), "PaneLoop")
	if !errors.Is(err, errBoom) {
		t.Fatalf("PaneLoop = %v, want an error wrapping the write failure", err)
	}
	if n := len(callsOf(f, tmuxctl.OpCapturePane)); n != 1 {
		t.Errorf("%d captures ran; the loop must stop at the first failed write", n)
	}
}

// The same with a real broken stream: a pipe whose reader is closed, which is
// what a cut exec connection is to the process writing into it.
func TestPaneLoopExitsWhenThePipeIsClosed(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.SetPane(podName, "screen\n")

	r, w := io.Pipe()
	done := paneLoopAsync(context.Background(), p, podName, w)

	buf := make([]byte, 64)
	n, err := r.Read(buf)
	if err != nil || buf[n-1] != FrameSeparator {
		t.Fatalf("first frame: %q, %v", buf[:n], err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close the reader: %v", err)
	}

	if err := result(t, done, "PaneLoop after the reader closed"); !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("PaneLoop = %v, want io.ErrClosedPipe", err)
	}
}

// A capture that fails ends the loop. Swallowing it would leave a loop that
// writes nothing and so can never learn its stream is gone.
func TestPaneLoopExitsWhenACaptureFails(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.FailOp(tmuxctl.OpCapturePane, errBoom)

	var wrote bool
	w := writerFunc(func(b []byte) (int, error) { wrote = true; return len(b), nil })
	err := result(t, paneLoopAsync(context.Background(), p, podName, w), "PaneLoop")

	if !errors.Is(err, errBoom) {
		t.Fatalf("PaneLoop = %v, want an error wrapping the capture failure", err)
	}
	if wrote {
		t.Error("a frame was written for a capture that failed")
	}
}

// A stop signal ends the loop and is reported as such, not as a failure to write.
func TestPaneLoopStopsOnCancel(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.SetPane(podName, "screen\n")

	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	w := writerFunc(func(b []byte) (int, error) {
		once.Do(cancel)
		return len(b), nil
	})

	done := paneLoopAsync(ctx, p, podName, w)
	if err := result(t, done, "PaneLoop after cancel"); !errors.Is(err, context.Canceled) {
		t.Errorf("PaneLoop = %v, want context.Canceled", err)
	}
}

// No shell string: what the loop runs is exactly the exported capture argv, on
// tmux itself.
func TestPaneLoopRunsTheCaptureArgvWithNoShell(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	f.SetPane(podName, "screen\n")

	n := 0
	w := writerFunc(func(b []byte) (int, error) {
		n++
		if n == 3 {
			return 0, errBoom
		}
		return len(b), nil
	})
	if err := result(t, paneLoopAsync(context.Background(), p, podName, w), "PaneLoop"); !errors.Is(err, errBoom) {
		t.Fatalf("PaneLoop = %v, want the writer's error", err)
	}

	calls := f.Calls()
	if len(calls) != 3 {
		t.Fatalf("%d tmux calls for 3 frames: %v", len(calls), calls)
	}
	want := tmuxctl.ArgvCapturePane(podName)
	for _, c := range calls {
		if c.Op != tmuxctl.OpCapturePane || !slices.Equal(c.Argv, want) {
			t.Errorf("call %v, want %v", c, want)
		}
		if len(c.Stdin) != 0 {
			t.Errorf("the capture carried stdin %q", c.Stdin)
		}
	}
}

func TestPaneLoopRefusesAnInvalidName(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	err := result(t, paneLoopAsync(context.Background(), p, "crswd-a:b", writerFunc(func(b []byte) (int, error) { return len(b), nil })), "PaneLoop")
	if !errors.Is(err, session.ErrInvalidName) {
		t.Errorf("PaneLoop = %v, want ErrInvalidName", err)
	}
	if calls := f.Calls(); len(calls) != 0 {
		t.Errorf("tmux was called for an invalid name: %v", calls)
	}
}

// The environment the session gets is the host's composed one plus
// CLAUDE_CONFIG_DIR, never the process's own: a pod's environment holds no daemon
// secret today, and this is what keeps that true when it does.
func TestEnvironmentIsComposedNotInherited(t *testing.T) {
	t.Parallel()

	parent := []string{
		"HOME=/home/ralph",
		"PATH=/usr/bin",
		"CLAUDE_CONFIG_DIR=/tmp/claude",
		"CRSW_SHARED_SECRET=hunter2",
		"CRSW_LISTEN=127.0.0.1:8765",
		"KUBERNETES_SERVICE_HOST=10.0.0.1",
		"SOME_TOKEN=abc",
	}
	got := environment(parent)

	for _, want := range []string{"HOME=/home/ralph", "PATH=/usr/bin", "CLAUDE_CONFIG_DIR=/tmp/claude"} {
		if !slices.Contains(got, want) {
			t.Errorf("environment lacks %q: %v", want, got)
		}
	}
	for _, kv := range got {
		name, _, _ := strings.Cut(kv, "=")
		switch {
		case strings.HasPrefix(name, "CRSW_"), name == "KUBERNETES_SERVICE_HOST", name == "SOME_TOKEN":
			t.Errorf("environment carries %s", name)
		}
	}
}

// Run and PaneLoop share one socket, and the socket is one NewExec accepts:
// an empty one is refused, and there is no fallback to tmux's default server.
func TestPackageEntrypointsBuildARealController(t *testing.T) {
	t.Parallel()

	if Socket == "" {
		t.Fatal("Socket is empty; tmuxctl.NewExec would refuse it")
	}
	// PATH is present in every environment the test binary runs in; without it
	// the composed environment is empty and NewExec refuses that too.
	if len(environment(os.Environ())) == 0 {
		t.Fatal("the composed environment is empty; tmuxctl.NewExec would refuse it")
	}
	ctl, err := newExec()
	if err != nil {
		t.Fatalf("newExec: %v", err)
	}
	var _ tmuxctl.Controller = ctl
}
