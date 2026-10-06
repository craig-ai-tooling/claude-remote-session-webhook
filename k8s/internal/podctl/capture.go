package podctl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

// firstFrameWait is how long CapturePane waits for a stream that has said
// nothing yet.
const firstFrameWait = 3 * time.Second

// stream is the held pane-loop of one session (FR-011). Every field is read and
// written under Controller.mu.
type stream struct {
	latest  string
	have    bool
	err     error
	lastUse time.Time
	cancel  context.CancelFunc
	ready   chan struct{}
	once    sync.Once
}

// markReady wakes the callers waiting on the first frame or the first error.
func (s *stream) markReady() { s.once.Do(func() { close(s.ready) }) }

// CapturePane returns the latest frame of the session's held pane-loop. The
// stream outlives the caller: it is started on a context of its own, so one
// request's cancellation cannot tear down a pane other requests are reading.
func (c *Controller) CapturePane(ctx context.Context, name string) (string, error) {
	c.mu.Lock()
	s := c.streams[name]
	if s == nil {
		s = c.startStream(name)
	}
	s.lastUse = c.cfg.Now()
	c.mu.Unlock()

	wait := time.NewTimer(firstFrameWait)
	defer wait.Stop()
	select {
	case <-s.ready:
	case <-wait.C:
	case <-ctx.Done():
		return "", fmt.Errorf("podctl: capture pane %s: %w", name, ctx.Err())
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !s.have && s.err == nil {
		return "", fmt.Errorf("podctl: no frame from %s yet", name)
	}
	return s.latest, s.err
}

// startStream registers a stream and starts its goroutine. The caller holds c.mu.
func (c *Controller) startStream(name string) *stream {
	ctx, cancel := context.WithCancel(context.Background())
	s := &stream{cancel: cancel, ready: make(chan struct{})}
	c.streams[name] = s
	go c.runStream(ctx, name, s)
	return s
}

// runStream holds one exec open and reopens it when it is cut.
func (c *Controller) runStream(ctx context.Context, name string, s *stream) {
	argv := []string{Binary, "pane-loop", name}
	sp := &splitter{c: c, name: name, s: s}
	for {
		code, err := c.exec.Exec(ctx, name, argv, nil, sp, io.Discard)
		sp.reset()
		if ctx.Err() != nil {
			return
		}
		c.mu.Lock()
		// A later reopen may still deliver a frame, so a cut stream that already
		// delivered one is not an error.
		if !s.have {
			switch {
			case err != nil:
				s.err = fmt.Errorf("podctl: pane stream %s: %w", name, err)
			case code != 0:
				s.err = fmt.Errorf("podctl: pane stream %s: exit %d", name, code)
			}
			if s.err != nil {
				s.markReady()
			}
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-time.After(c.cfg.PollInterval):
		}
	}
}

// splitter cuts a pane-loop's stdout into frames on sessionpod.FrameSeparator,
// keeping a partial frame across writes.
type splitter struct {
	c    *Controller
	name string
	s    *stream

	partial  []byte // only touched by the exec's writer
	overflow bool
}

func (sp *splitter) reset() {
	sp.partial = nil
	sp.overflow = false
}

func (sp *splitter) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, sessionpod.FrameSeparator)
		if i < 0 {
			if !sp.overflow {
				sp.partial = append(sp.partial, p...)
				// A stream that never ends a frame must not grow without bound.
				if len(sp.partial) > maxHistoryBytes {
					sp.partial, sp.overflow = nil, true
				}
			}
			break
		}
		if !sp.overflow {
			sp.partial = append(sp.partial, p[:i]...)
		}
		sp.frame(string(sp.partial), sp.overflow)
		sp.partial, sp.overflow = nil, false
		p = p[i+1:]
	}
	return n, nil
}

// frame stores one complete frame. Idleness is checked here, on every frame,
// because a healthy stream never returns from Exec.
func (sp *splitter) frame(raw string, tooBig bool) {
	screen := tmuxctl.Strip(raw)
	c := sp.c
	c.mu.Lock()
	defer c.mu.Unlock()
	if tooBig || countLines(screen) > c.cfg.PaneBound {
		sp.s.err = tmuxctl.ErrPaneTooLarge
	} else {
		sp.s.latest, sp.s.have, sp.s.err = screen, true, nil
	}
	sp.s.markReady()
	if c.cfg.Now().Sub(sp.s.lastUse) > c.cfg.StreamIdle {
		sp.s.cancel()
		if c.streams[sp.name] == sp.s {
			delete(c.streams, sp.name)
		}
	}
}
