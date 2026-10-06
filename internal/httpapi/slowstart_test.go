package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

// deadlineRecorder is a ResponseRecorder that can be given a write deadline,
// which http.NewResponseController finds by method and a plain recorder lacks.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	mu        sync.Mutex
	deadlines []time.Time
}

func (d *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deadlines = append(d.deadlines, t)
	return nil
}

func (d *deadlineRecorder) set() []time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]time.Time(nil), d.deadlines...)
}

// wantDeadlineIn fails unless exactly one deadline was set and it is about want
// from now.
func wantDeadlineIn(t *testing.T, got []time.Time, want time.Duration) {
	t.Helper()
	if len(got) != 1 {
		t.Fatalf("write deadlines set = %v; want exactly one", got)
	}
	if d := time.Until(got[0]) - want; d > 5*time.Second || d < -5*time.Second {
		t.Fatalf("deadline is %v from now; want about %v", time.Until(got[0]), want)
	}
}

func TestAPICreateLiftsItsWriteDeadlineOnlyWhenSlowStartIsSet(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		deadline time.Duration
	}{
		"cluster": {deadline: 2 * time.Minute},
		"host":    {deadline: 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newAuditedServer(t)
			s.slowStartDeadline = tc.deadline
			body := createBody(s.fixture)
			req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
			signRequest(t, req, body, testTime)
			w := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			s.ServeHTTP(w, req)
			if w.Code != http.StatusCreated {
				t.Fatalf("POST /sessions = %d (%s); want 201", w.Code, w.Body.String())
			}
			if tc.deadline == 0 {
				if got := w.set(); len(got) != 0 {
					t.Fatalf("host mode set a write deadline %v; want none", got)
				}
				return
			}
			wantDeadlineIn(t, w.set(), tc.deadline)
		})
	}
}

func TestBrowserCreateLiftsItsWriteDeadlineOnlyWhenSlowStartIsSet(t *testing.T) {
	t.Parallel()
	for name, deadline := range map[string]time.Duration{"cluster": 2 * time.Minute, "host": 0} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := newCreator(t)
			c.slowStartDeadline = deadline
			form := c.wellFormed(t)
			req := httptest.NewRequest(http.MethodPost, createPath, strings.NewReader(form.Encode()))
			req.Header.Set(headerContentType, contentTypeForm)
			req.Header.Set(headerAccessAssertion, c.keys.mint(t, c.keys.claims()))
			req.Header.Set(headerSecFetchSite, secFetchSiteSameOrigin)
			w := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			c.ServeHTTP(w, req)
			if w.Code != http.StatusSeeOther && w.Code != http.StatusFound {
				t.Fatalf("POST %s = %d (%s); want a redirect", createPath, w.Code, w.Body.String())
			}
			if deadline == 0 {
				if got := w.set(); len(got) != 0 {
					t.Fatalf("host mode set a write deadline %v; want none", got)
				}
				return
			}
			wantDeadlineIn(t, w.set(), deadline)
		})
	}
}

func TestCreateRefusesWhenTheWriteDeadlineCannotBeSet(t *testing.T) {
	t.Parallel()
	s := newAuditedServer(t)
	s.slowStartDeadline = time.Minute
	body := createBody(s.fixture)
	req := httptest.NewRequest(http.MethodPost, "/sessions", bytes.NewReader(body))
	signRequest(t, req, body, testTime)
	w := httptest.NewRecorder() // no SetWriteDeadline: the response would be cut off
	s.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("POST /sessions = %d; want 500", w.Code)
	}
	if calls := s.fixture.tmux.Calls(); len(calls) != 0 {
		t.Fatalf("a refused create reached tmux: %v", calls)
	}
}

func TestNewForClusterStartDeadline(t *testing.T) {
	t.Parallel()
	cfg, _ := clusterConfig(t)
	for name, tc := range map[string]struct{ hook, want time.Duration }{
		"default":  {0, 3 * time.Minute},
		"explicit": {150 * time.Second, 150 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv, err := NewForCluster(cfg, tmuxctl.NewFake(), ClusterHooks{StartDeadline: tc.hook})
			if err != nil {
				t.Fatal(err)
			}
			if srv.slowStartDeadline != tc.want {
				t.Fatalf("slowStartDeadline = %v; want %v", srv.slowStartDeadline, tc.want)
			}
		})
	}
	host, err := NewWith(testConfig("127.0.0.1:0"), tmuxctl.NewFake(), audit.New())
	if err != nil {
		t.Fatal(err)
	}
	if host.slowStartDeadline != 0 {
		t.Fatalf("host slowStartDeadline = %v; want 0", host.slowStartDeadline)
	}
}

// slowStart is a controller whose New takes as long as a pod that is pulling its
// image.
type slowStart struct {
	*tmuxctl.Fake
	delay time.Duration
}

func (s slowStart) New(ctx context.Context, name, workDir string) error {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return ctx.Err()
	}
	return s.Fake.New(ctx, name, workDir)
}

// postOverASocket signs and sends one create to a served daemon. The write
// timeout is shortened for the reason stream_test.go's is: the question is the
// same at 200ms as at 30s.
func postOverASocket(t *testing.T, srv *Server, workDir string) (*http.Response, error) {
	t.Helper()
	srv.http.WriteTimeout = writeDeadlineUnderTest
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve() }()
	t.Cleanup(func() {
		if err := srv.Close(); err != nil {
			t.Errorf("Close() = %v", err)
		}
		if err := <-served; err != nil {
			t.Errorf("Serve() = %v", err)
		}
	})

	body := []byte(`{"name":"slow-start","work_dir":"` + workDir + `"}`)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+srv.Addr().String()+"/sessions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	signRequest(t, req, body, time.Now())
	req.Header.Set(headerContentType, contentTypeJSON)
	return (&http.Client{Timeout: 10 * time.Second}).Do(req) //nolint:bodyclose // the callers close it, or have an error and no body.
}

func TestClusterCreateOutlivesTheServersWriteTimeout(t *testing.T) {
	t.Parallel()
	cfg, root := clusterConfig(t)
	ctl := slowStart{Fake: tmuxctl.NewFake(), delay: 3 * writeDeadlineUnderTest}
	srv, err := NewForCluster(cfg, ctl, ClusterHooks{StartDeadline: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := postOverASocket(t, srv, root)
	if err != nil {
		t.Fatalf("POST /sessions = %v; the connection was dropped while the session was being created", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close the response: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /sessions = %d; want 201", resp.StatusCode)
	}
}

func TestHostCreateKeepsTheServersWriteTimeout(t *testing.T) {
	t.Parallel()
	cfg := testConfig("127.0.0.1:0")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg.Roots = []config.ApprovedRoot{{Path: root}}
	ctl := slowStart{Fake: tmuxctl.NewFake(), delay: 3 * writeDeadlineUnderTest}
	srv, err := NewWith(cfg, ctl, audit.New())
	if err != nil {
		t.Fatal(err)
	}
	resp, err := postOverASocket(t, srv, root)
	if err == nil {
		if cerr := resp.Body.Close(); cerr != nil {
			t.Errorf("close the response: %v", cerr)
		}
		t.Fatalf("host-mode POST /sessions = %d; want the connection cut at the write timeout", resp.StatusCode)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("the client timed out, not the server: %v", err)
	}
}

// A pod-backed destroy waits for the pod to be gone (podctl's KillTimeout, 60 s
// by default, past a 30 s grace), so it needs the same lifted deadline a create
// does. Found by the live install of 10/6/26: DELETE answered nothing although
// the pod went away.
func TestAPIDestroyLiftsItsWriteDeadlineOnlyWhenSlowStartIsSet(t *testing.T) {
	t.Parallel()
	for name, deadline := range map[string]time.Duration{"cluster": 2 * time.Minute, "host": 0} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newDestroyFixture(t)
			f.slowStartDeadline = deadline
			req := httptest.NewRequest(http.MethodDelete, "/sessions/"+f.live.ID, nil)
			signRequest(t, req, nil, testTime)
			req.Header.Set(headerAuthorization, bearerScheme+f.token)
			w := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
			f.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("DELETE = %d (%s); want 200", w.Code, w.Body.String())
			}
			if deadline == 0 {
				if got := w.set(); len(got) != 0 {
					t.Fatalf("host mode set a write deadline %v; want none", got)
				}
				return
			}
			wantDeadlineIn(t, w.set(), deadline)
		})
	}
}

func TestDestroyRefusesWhenTheWriteDeadlineCannotBeSet(t *testing.T) {
	t.Parallel()
	f := newDestroyFixture(t)
	f.slowStartDeadline = time.Minute
	req := httptest.NewRequest(http.MethodDelete, "/sessions/"+f.live.ID, nil)
	signRequest(t, req, nil, testTime)
	req.Header.Set(headerAuthorization, bearerScheme+f.token)
	w := httptest.NewRecorder() // no SetWriteDeadline: the response would be cut off
	f.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("DELETE = %d; want 500", w.Code)
	}
	if calls := f.fixture.tmux.Calls(); len(calls) != 0 {
		t.Fatalf("a refused destroy reached tmux: %v", calls)
	}
}
