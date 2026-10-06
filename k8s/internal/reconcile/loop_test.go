package reconcile

import (
	"context"
	"errors"
	"sync/atomic"
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

// The reconciler never releases its Lease, so failover waits LeaseDuration and
// that, not an ordering of release against the worker, bounds any overlap. The
// worker is still joined before RunWithLease returns.
func TestRunWithLeaseJoinsWorkerAndKeepsLease(t *testing.T) {
	setLeaseTimings(t)
	var exited atomic.Bool
	started := make(chan struct{})
	g := newRig(t, podCfg(), nil)

	old := runLoop
	runLoop = func(_ *Reconciler, ctx context.Context) error {
		close(started)
		<-ctx.Done()
		time.Sleep(500 * time.Millisecond)
		exited.Store(true)
		return ctx.Err()
	}
	t.Cleanup(func() { runLoop = old })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunWithLease(ctx, g.kube, "one", g.r) }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("worker never started")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("RunWithLease did not return")
	}
	if !exited.Load() {
		t.Error("RunWithLease returned before the worker exited")
	}
	l, err := g.kube.CoordinationV1().Leases("crswd").Get(context.Background(), LeaseName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if l.Spec.HolderIdentity == nil || *l.Spec.HolderIdentity != "one" {
		t.Errorf("holder = %v, want the Lease kept by one", l.Spec.HolderIdentity)
	}
}

func TestElectorConfigKeepsTheLease(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), nil)
	if c := electorConfig(g.r, "one"); !c.KeepLeaseOnCancel || c.Identity != "one" || c.Name != LeaseName {
		t.Errorf("electorConfig = %+v, want KeepLeaseOnCancel and the reconciler's Lease", c)
	}
}
