//go:build tmux

package sessionpod

// The tests that need a real tmux binary, run with:
//
//	go test -tags tmux ./internal/sessionpod
//
// Each one gets a private -L server, never tmux's default, so the kill-server in
// its cleanup can only reach sessions the test made. They exist because the fake
// cannot show the one thing Run's end-of-session logic rests on: what a real
// tmux answers when the last session on a server has just ended.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

// realPod is a Pod on a real tmux server of the test's own.
func realPod(t *testing.T) *Pod {
	t.Helper()

	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skipf("tmux is not installed: %v", err)
	}

	socket := "crswd-podtest-" + strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '-'
		}
	}, t.Name())

	// Enough environment for a real shell, and nothing more. A pane with no PATH
	// fails in ways that look like tmux misbehaving.
	env := []string{"TERM=xterm"}
	for _, name := range []string{"HOME", "PATH", "SHELL", "USER", "LOGNAME"} {
		if v := os.Getenv(name); v != "" {
			env = append(env, name+"="+v)
		}
	}
	ctl, err := tmuxctl.NewExec(socket, 1000, env)
	if err != nil {
		t.Fatalf("NewExec: %v", err)
	}

	t.Cleanup(func() {
		out, err := exec.Command("tmux", "-L", socket, "kill-server").CombinedOutput() //nolint:gosec // socket is derived from t.Name()
		if err != nil && !strings.Contains(string(out), "no server running") && !strings.Contains(string(out), "error connecting to") {
			t.Logf("cleanup kill-server: %v: %s", err, out)
		}
	})
	return &Pod{Tmux: ctl, Interval: 50 * time.Millisecond}
}

// TestRunSeesTheLastSessionEndOnARealServer is the reason Run asks list-sessions
// when has-session errors. The pod's only session ending takes the tmux server
// with it, and has-session on a server that is gone is an error, not false, so a
// Run that waited for Has to say false would never return in production while
// every fake-backed test passed.
func TestRunSeesTheLastSessionEndOnARealServer(t *testing.T) {
	p := realPod(t)
	root, roots := approvedRoot(t)
	workdir := mkdir(t, root+"/repo")

	// Establish the premise, so this test fails for the right reason if a later
	// tmux starts saying false: the ending of the last session really is an error
	// from Has, and not a clean "no".
	done := runAsync(context.Background(), p, podName, workdir, roots)
	waitUntil(t, "the session to exist", func() bool {
		ok, err := p.Tmux.Has(context.Background(), podName)
		return err == nil && ok
	})
	if err := p.Tmux.Kill(context.Background(), podName); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitUntil(t, "the server to be gone", func() bool {
		_, err := p.Tmux.Has(context.Background(), podName)
		return err != nil
	})

	if err := result(t, done, "Run after the last session ended"); err != nil {
		t.Errorf("Run = %v, want nil once the session is gone", err)
	}
}

// A real pane and a real capture: the frame carries what the shell printed, is
// terminated, and the loop ends when the stream does.
func TestPaneLoopReadsARealPane(t *testing.T) {
	p := realPod(t)
	ctx := context.Background()
	root, _ := approvedRoot(t)

	if err := p.Tmux.New(ctx, podName, root); err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := p.Tmux.SendKeys(ctx, podName, "echo pane-loop-marker", "Enter"); err != nil {
		t.Fatalf("SendKeys: %v", err)
	}

	var (
		mu     sync.Mutex
		stream bytes.Buffer
		cut    bool
	)
	w := writerFunc(func(b []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()

		if cut {
			return 0, errors.New("stream cut")
		}
		stream.Write(b)
		return len(b), nil
	})
	done := paneLoopAsync(ctx, p, podName, w)

	waitUntil(t, "a frame carrying the marker", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return strings.Contains(stream.String(), "pane-loop-marker")
	})

	mu.Lock()
	cut = true
	got := stream.Bytes()
	mu.Unlock()
	if got[len(got)-1] != FrameSeparator {
		t.Errorf("the stream ends in %q, not the separator", got[len(got)-1])
	}

	if err := result(t, done, "PaneLoop after the stream was cut"); err == nil || !strings.Contains(err.Error(), "stream cut") {
		t.Errorf("PaneLoop = %v, want the write failure", err)
	}
}
