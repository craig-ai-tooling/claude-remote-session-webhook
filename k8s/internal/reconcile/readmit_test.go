package reconcile

import (
	"context"
	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clienttesting "k8s.io/client-go/testing"
	"testing"
	"time"
)

func (g *rig) podDeletes() []clienttesting.DeleteActionImpl {
	var out []clienttesting.DeleteActionImpl
	for _, a := range g.kube.Actions() {
		if d, ok := a.(clienttesting.DeleteActionImpl); ok && d.Matches("delete", "pods") {
			out = append(out, d)
		}
	}
	return out
}

func (g *rig) statusWrites() int {
	n := 0
	for _, a := range g.dyn.Actions() {
		if a.Matches("update", "agentsessions") && a.GetSubresource() == "status" {
			n++
		}
	}
	return n
}

// Finding 1: an edited spec must not leave an inadmissible session running.
func TestReconcileLivePodReadmitsWorkDirAndCeiling(t *testing.T) {
	t.Parallel()
	for _, phase := range []corev1.PodPhase{corev1.PodRunning, corev1.PodPending} {
		for name, mut := range map[string]func(*v1alpha1.AgentSession, *Config){
			"workdir outside root": func(o *v1alpha1.AgentSession, _ *Config) { o.Spec.WorkDir = "/etc" },
			"lifetime over max":    func(_ *v1alpha1.AgentSession, c *Config) { c.LifetimeMax = time.Hour },
		} {
			t.Run(string(phase)+" "+name, func(t *testing.T) {
				t.Parallel()
				cfg := podCfg()
				o := podObj()
				mut(&o, &cfg)
				g := newRig(t, cfg, []v1alpha1.AgentSession{o}, ownedPod(t, phase))
				if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
					t.Fatal(err)
				}
				if st := g.status(t, "crswd-abc"); st.Phase != v1alpha1.PhaseRejected || st.Reason == "" {
					t.Errorf("status = %+v, want Rejected with a reason", st)
				}
				dels := g.podDeletes()
				if len(dels) != 1 {
					t.Fatalf("pod deletes = %d, want 1", len(dels))
				}
				if dels[0].DeleteOptions.GracePeriodSeconds != nil {
					t.Error("delete overrides the grace period")
				}
				if pre := dels[0].DeleteOptions.Preconditions; pre == nil || pre.UID == nil || *pre.UID != types.UID("pu1") {
					t.Errorf("delete lacks the pod UID precondition: %+v", pre)
				}
			})
		}
	}
}

func TestReconcileLivePodNotEvictedByCap(t *testing.T) {
	t.Parallel()
	cfg := podCfg()
	cfg.Cap = 1
	older := podObj()
	older.Metadata.Name, older.Metadata.UID = "crswd-old", "u0"
	older.Metadata.CreationTimestamp = podNow.Add(-2 * time.Hour)
	older.Status.Phase = v1alpha1.PhaseRunning
	g := newRig(t, cfg, []v1alpha1.AgentSession{older, podObj()}, ownedPod(t, corev1.PodRunning))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := len(g.podDeletes()); n != 0 {
		t.Errorf("pod deletes = %d, want 0", n)
	}
	if got := g.status(t, "crswd-abc").Phase; got != v1alpha1.PhaseRunning {
		t.Errorf("phase = %q, want Running", got)
	}
}

// The Rejected status is written before the delete, so a failed delete must be
// retried from a Rejected object.
func TestReconcileRejectedObjectEndsItsPod(t *testing.T) {
	t.Parallel()
	o := podObj()
	o.Status.Phase, o.Status.Reason = v1alpha1.PhaseRejected, "working directory is not under an approved root"
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o}, ownedPod(t, corev1.PodRunning))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := len(g.podDeletes()); n != 1 {
		t.Errorf("pod deletes = %d, want 1", n)
	}
}
