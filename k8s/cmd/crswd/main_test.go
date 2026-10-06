package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/podctl"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/reconcile"
)

func TestDispatchUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch(context.Background(), []string{"bogus"}, &out, &errb); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errb.String(), `unknown command "bogus"`) {
		t.Errorf("stderr = %q, want it to name the command", errb.String())
	}
}

func TestDispatchVersion(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch(context.Background(), []string{"--version"}, &out, &errb); code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "cluster build") {
		t.Errorf("stdout = %q, want it to say cluster build", out.String())
	}
}

func TestSessionPodArgs(t *testing.T) {
	name, workdir, roots, err := parseSessionPodArgs([]string{"crswd-abc", "/work/x"})
	if err != nil {
		t.Fatalf("parseSessionPodArgs: %v", err)
	}
	if name != "crswd-abc" || workdir != "/work/x" {
		t.Errorf("got %q %q, want crswd-abc /work/x", name, workdir)
	}
	if len(roots) != 1 || roots[0].Path != sessionpod.WorkRoot {
		t.Errorf("roots = %v, want the one WorkRoot", roots)
	}
	for _, bad := range [][]string{nil, {"only-one"}, {"a", "b", "c"}} {
		if _, _, _, err := parseSessionPodArgs(bad); err == nil {
			t.Errorf("parseSessionPodArgs(%v) = nil error, want one", bad)
		}
	}
}

func TestSessionPodWrongArgCountExitsTwo(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch(context.Background(), []string{"session-pod", "only-one"}, &out, &errb); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestPodContainerNameAgrees(t *testing.T) {
	if podctl.Container != reconcile.SessionContainer {
		t.Errorf("podctl.Container = %q, reconcile.SessionContainer = %q; they must be equal", podctl.Container, reconcile.SessionContainer)
	}
}
