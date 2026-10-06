package podctl

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
)

func newSessions(t *testing.T) *agentsession.Client {
	t.Helper()
	s, err := agentsession.New(dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), nil), "ns")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestNewControllerRefuses(t *testing.T) {
	sessions := newSessions(t)
	pods := kfake.NewSimpleClientset()
	ok := Config{Namespace: "ns", PaneBound: 10}
	if _, err := New(sessions, pods, newRecorder(), ok); err != nil {
		t.Fatalf("valid config refused: %v", err)
	}
	neg := -time.Second
	cases := map[string]struct {
		ex  Executor
		cfg Config
	}{
		"empty namespace": {newRecorder(), Config{PaneBound: 10}},
		"zero panebound":  {newRecorder(), Config{Namespace: "ns"}},
		"nil executor":    {nil, ok},
		"neg ready":       {newRecorder(), Config{Namespace: "ns", PaneBound: 10, ReadyTimeout: neg}},
		"neg kill":        {newRecorder(), Config{Namespace: "ns", PaneBound: 10, KillTimeout: neg}},
		"neg poll":        {newRecorder(), Config{Namespace: "ns", PaneBound: 10, PollInterval: neg}},
		"neg idle":        {newRecorder(), Config{Namespace: "ns", PaneBound: 10, StreamIdle: neg}},
	}
	for name, tc := range cases {
		if _, err := New(sessions, pods, tc.ex, tc.cfg); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := New(nil, pods, newRecorder(), ok); err == nil {
		t.Error("nil sessions: want error")
	}
	if _, err := New(sessions, nil, newRecorder(), ok); err == nil {
		t.Error("nil pods: want error")
	}
}

func TestNewControllerDefaults(t *testing.T) {
	c, err := New(newSessions(t), kfake.NewSimpleClientset(), newRecorder(), Config{Namespace: "ns", PaneBound: 10})
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.ReadyTimeout != 90*time.Second || c.cfg.KillTimeout != 60*time.Second ||
		c.cfg.PollInterval != time.Second || c.cfg.StreamIdle != 30*time.Second || c.cfg.Now == nil {
		t.Errorf("defaults not applied: %+v", c.cfg)
	}
}

func TestExecutorRecorder(t *testing.T) {
	r := newRecorder()
	r.on("tmux ls", reply{stdout: "x", code: 3})
	var out strings.Builder
	code, err := r.Exec(context.Background(), "p", []string{"tmux", "ls"}, strings.NewReader("in"), &out, nil)
	if err != nil || code != 3 || out.String() != "x" {
		t.Fatalf("got %d %v %q", code, err, out.String())
	}
	if len(r.calls) != 1 || r.calls[0].pod != "p" || string(r.calls[0].stdin) != "in" {
		t.Errorf("not recorded: %+v", r.calls)
	}
}

func TestExecutorAnnotationKeyAndInPod(t *testing.T) {
	if got := annotationKey("@crswd-owner"); got != "crswd.craigcloud.io/owner" {
		t.Errorf("annotationKey = %q", got)
	}
	got := strings.Join(inPod([]string{"tmux", "has-session", "-t", "x"}), " ")
	if got != "tmux -L crswd-session has-session -t x" {
		t.Errorf("inPod = %q", got)
	}
}
