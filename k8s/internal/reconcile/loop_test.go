package reconcile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createObject(t *testing.T, g *rig, ns string) {
	t.Helper()
	o := podObj()
	o.Metadata.Namespace = ns
	u, err := agentsession.FromObject(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.dyn.Resource(agentsession.GVR).Namespace(ns).Create(context.Background(), u, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestLoopCreatesPodForNewObject(t *testing.T) {
	cfg := podCfg()
	g := newRig(t, cfg, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.r.Run(ctx) }()

	createObject(t, g, cfg.SessionNamespace)

	deadline := time.Now().Add(5 * time.Second)
	for g.count("create", "pods") < 1 {
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("no pod created within 5s")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v, want context.Canceled", err)
	}
	if n := g.count("create", "pods"); n != 1 {
		t.Fatalf("create pods = %d, want 1", n)
	}
}

func setLeaseTimings(t *testing.T) {
	t.Helper()
	old := leaseTimings
	leaseTimings = kube.ElectorConfig{LeaseDuration: 2 * time.Second, RenewDeadline: time.Second, RetryPeriod: 200 * time.Millisecond}
	t.Cleanup(func() { leaseTimings = old })
}

func TestLeaseOneActs(t *testing.T) {
	setLeaseTimings(t)
	cfg := podCfg()
	g := newRig(t, cfg, nil)
	r2, err := New(g.kube, g.dyn, cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 2)
	go func() { done <- RunWithLease(ctx, g.kube, "one", g.r) }()
	go func() { done <- RunWithLease(ctx, g.kube, "two", r2) }()

	createObject(t, g, cfg.SessionNamespace)
	time.Sleep(4 * time.Second)
	if n := g.count("create", "pods"); n != 1 {
		t.Fatalf("create pods across both reconcilers = %d, want 1", n)
	}
	cancel()
	<-done
	<-done
}

func TestRunWithLeaseReturnsLoopError(t *testing.T) {
	setLeaseTimings(t)
	old := runLoop
	runLoop = func(*Reconciler, context.Context) error { return errors.New("boom") }
	t.Cleanup(func() { runLoop = old })

	g := newRig(t, podCfg(), nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := time.Now()
	err := RunWithLease(ctx, g.kube, "one", g.r)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("RunWithLease = %v, want boom", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("took %v, want well before the context ends", time.Since(start))
	}
}
