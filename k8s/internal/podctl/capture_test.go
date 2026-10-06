package podctl

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

var sep = string(rune(sessionpod.FrameSeparator))

// fnExec is an Executor written as a function, so a test can hold a stream open
// and write to it on its own schedule.
type fnExec func(ctx context.Context, pod string, argv []string, stdout io.Writer) (int, error)

func (f fnExec) Exec(ctx context.Context, pod string, argv []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
	return f(ctx, pod, argv, stdout)
}

// clock is a Now the test can move.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// captureRig is a rig whose Executor is f, with the stream stopped on cleanup.
func captureRig(t *testing.T, f fnExec) *rig {
	t.Helper()
	r := newRig(t, func(*recorder) Executor { return f })
	t.Cleanup(func() { r.c.stopStream(testName) })
	return r
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestCapturePaneReassemblesFrames(t *testing.T) {
	t.Parallel()
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, out io.Writer) (int, error) {
		for _, piece := range []string{"hel", "lo wor", "ld" + sep + "par"} {
			if _, err := io.WriteString(out, piece); err != nil {
				return 0, err
			}
		}
		<-ctx.Done()
		return 0, ctx.Err()
	})
	got, err := r.c.CapturePane(context.Background(), testName)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello world" {
		t.Fatalf("screen = %q, want the first whole frame", got)
	}
}

func TestCapturePaneStripsANSI(t *testing.T) {
	t.Parallel()
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, out io.Writer) (int, error) {
		if _, err := io.WriteString(out, "\x1b[31mred\x1b[0m text"+sep); err != nil {
			return 0, err
		}
		<-ctx.Done()
		return 0, ctx.Err()
	})
	got, err := r.c.CapturePane(context.Background(), testName)
	if err != nil {
		t.Fatal(err)
	}
	if got != "red text" {
		t.Fatalf("screen = %q", got)
	}
}

func TestCapturePaneArgv(t *testing.T) {
	t.Parallel()
	var argv atomic.Value
	r := captureRig(t, func(ctx context.Context, pod string, a []string, out io.Writer) (int, error) {
		argv.Store(append([]string{pod}, a...))
		if _, err := io.WriteString(out, "x"+sep); err != nil {
			return 0, err
		}
		<-ctx.Done()
		return 0, ctx.Err()
	})
	if _, err := r.c.CapturePane(context.Background(), testName); err != nil {
		t.Fatal(err)
	}
	a, ok := argv.Load().([]string)
	if !ok {
		t.Fatal("the exec never ran")
	}
	got := strings.Join(a, " ")
	if want := testName + " " + Binary + " pane-loop " + testName; got != want {
		t.Fatalf("exec = %q, want %q", got, want)
	}
}

func TestCapturePaneOversizeFrame(t *testing.T) {
	t.Parallel()
	gate := make(chan struct{})
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, out io.Writer) (int, error) {
		if _, err := io.WriteString(out, "ok"+sep); err != nil {
			return 0, err
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
		if _, err := io.WriteString(out, strings.Repeat("row\n", 11)+sep); err != nil { // PaneBound is 10
			return 0, err
		}
		<-ctx.Done()
		return 0, ctx.Err()
	})
	got, err := r.c.CapturePane(context.Background(), testName)
	if err != nil || got != "ok" {
		t.Fatalf("first frame = %q, %v", got, err)
	}
	close(gate)
	eventually(t, "oversize refusal", func() bool {
		_, err := r.c.CapturePane(context.Background(), testName)
		return errors.Is(err, tmuxctl.ErrPaneTooLarge)
	})
	got, err = r.c.CapturePane(context.Background(), testName)
	if got != "ok" || !errors.Is(err, tmuxctl.ErrPaneTooLarge) {
		t.Fatalf("after oversize = %q, %v; want the old screen and ErrPaneTooLarge", got, err)
	}
}

func TestCapturePaneOversizeFirstFrame(t *testing.T) {
	t.Parallel()
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, out io.Writer) (int, error) {
		if _, err := io.WriteString(out, strings.Repeat("row\n", 11)+sep); err != nil {
			return 0, err
		}
		<-ctx.Done()
		return 0, ctx.Err()
	})
	got, err := r.c.CapturePane(context.Background(), testName)
	if got != "" || !errors.Is(err, tmuxctl.ErrPaneTooLarge) {
		t.Fatalf("= %q, %v; want empty and ErrPaneTooLarge", got, err)
	}
}

func TestCapturePaneReopensCutStream(t *testing.T) {
	t.Parallel()
	var execs atomic.Int32
	r := captureRig(t, func(_ context.Context, _ string, _ []string, out io.Writer) (int, error) {
		execs.Add(1)
		if _, err := io.WriteString(out, "frame"+sep); err != nil {
			return 0, err
		}
		return 0, nil
	})
	if _, err := r.c.CapturePane(context.Background(), testName); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a second exec", func() bool { return execs.Load() >= 2 })
}

func TestCapturePaneIdleStreamIsCancelled(t *testing.T) {
	t.Parallel()
	var execs atomic.Int32
	cancelled := make(chan struct{})
	gate := make(chan struct{})
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, out io.Writer) (int, error) {
		if execs.Add(1) > 1 {
			if _, err := io.WriteString(out, "again"+sep); err != nil {
				return 0, err
			}
			<-ctx.Done()
			return 0, ctx.Err()
		}
		if _, err := io.WriteString(out, "one"+sep); err != nil {
			return 0, err
		}
		<-gate
		// A healthy stream never returns, so idleness has to be found here.
		if _, err := io.WriteString(out, "two"+sep); err != nil {
			return 0, err
		}
		<-ctx.Done()
		close(cancelled)
		return 0, ctx.Err()
	})
	clk := &clock{t: testNow}
	r.c.cfg.Now = clk.now
	if _, err := r.c.CapturePane(context.Background(), testName); err != nil {
		t.Fatal(err)
	}
	clk.advance(r.c.cfg.StreamIdle + time.Second)
	close(gate)
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("an idle stream that kept delivering frames was not cancelled")
	}
	got, err := r.c.CapturePane(context.Background(), testName)
	if err != nil || got != "again" {
		t.Fatalf("new stream = %q, %v", got, err)
	}
	if n := execs.Load(); n != 2 {
		t.Fatalf("execs = %d, want 2", n)
	}
}

func TestCapturePaneErrorBeforeFirstFrame(t *testing.T) {
	t.Parallel()
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, _ io.Writer) (int, error) {
		return 1, nil
	})
	if _, err := r.c.CapturePane(context.Background(), testName); err == nil {
		t.Fatal("a stream that exited non-zero before any frame returned no error")
	}
}

func TestCapturePaneTransportErrorBeforeFirstFrame(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	r := captureRig(t, func(context.Context, string, []string, io.Writer) (int, error) {
		return 0, boom
	})
	if _, err := r.c.CapturePane(context.Background(), testName); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want it to wrap the transport error", err)
	}
}

func TestCapturePaneKillStopsStream(t *testing.T) {
	t.Parallel()
	stopped := make(chan struct{})
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, out io.Writer) (int, error) {
		if _, err := io.WriteString(out, "x"+sep); err != nil {
			return 0, err
		}
		<-ctx.Done()
		close(stopped)
		return 0, ctx.Err()
	})
	if _, err := r.c.CapturePane(context.Background(), testName); err != nil {
		t.Fatal(err)
	}
	r.object(t, testName, nil, v1alpha1.AgentSessionStatus{})
	if err := r.c.Kill(context.Background(), testName); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Kill left the pane stream running")
	}
}

func TestCapturePaneCallerContext(t *testing.T) {
	t.Parallel()
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, _ io.Writer) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.c.CapturePane(ctx, testName); err == nil {
		t.Fatal("a cancelled caller got no error")
	}
	// The stream belongs to the session, not to the caller that started it.
	r.c.mu.Lock()
	s := r.c.streams[testName]
	r.c.mu.Unlock()
	if s == nil {
		t.Fatal("the caller's cancellation tore the stream down")
	}
}

func TestCapturePaneIdleReapsFailingStream(t *testing.T) {
	t.Parallel()
	var execs atomic.Int32
	r := captureRig(t, func(context.Context, string, []string, io.Writer) (int, error) {
		execs.Add(1)
		return 0, errors.New("exec refused")
	})
	r.c.cfg.StreamIdle = 40 * time.Millisecond
	if _, err := r.c.CapturePane(context.Background(), testName); err == nil {
		t.Fatal("a failing stream returned no error")
	}
	r.c.mu.Lock()
	s := r.c.streams[testName]
	r.c.mu.Unlock()
	if s == nil {
		t.Fatal("no stream registered")
	}
	eventually(t, "the failing stream to be reaped", func() bool {
		r.c.mu.Lock()
		defer r.c.mu.Unlock()
		return len(r.c.streams) == 0
	})
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the reaped stream's goroutine did not exit")
	}
	n := execs.Load()
	time.Sleep(30 * time.Millisecond)
	if execs.Load() != n {
		t.Fatal("a reaped stream kept retrying exec")
	}
}

func TestCapturePaneRacingKillLeavesNoStream(t *testing.T) {
	t.Parallel()
	r := captureRig(t, func(ctx context.Context, _ string, _ []string, _ io.Writer) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	r.pods.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		once.Do(func() { close(entered) })
		<-release
		return false, nil, nil
	})
	r.object(t, testName, nil, v1alpha1.AgentSessionStatus{})
	killed := make(chan error, 1)
	go func() { killed <- r.c.Kill(context.Background(), testName) }()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := r.c.CapturePane(ctx, testName); err == nil {
		t.Fatal("CapturePane on a name being killed returned no error")
	}
	close(release)
	if err := <-killed; err != nil {
		t.Fatal(err)
	}
	r.c.mu.Lock()
	n := len(r.c.streams)
	r.c.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d stream(s) survived Kill", n)
	}
}
