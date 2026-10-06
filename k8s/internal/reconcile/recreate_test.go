package reconcile

import (
	"context"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// Finding 4: reserve the recreate before deleting the failed pod.
func TestRecreateUpdateFailureMeansNoDelete(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, ownedPod(t, corev1.PodFailed))
	g.dyn.PrependReactor("update", "agentsessions", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewConflict(agentsession.GVR.GroupResource(), "crswd-abc", nil)
	})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err == nil {
		t.Error("err = nil, want the update failure")
	}
	if n := len(g.podDeletes()); n != 0 {
		t.Errorf("pod deletes = %d, want 0", n)
	}
}

func TestRecreateSamePodUIDNotCountedTwice(t *testing.T) {
	t.Parallel()
	// A crash after the reservation and before the delete leaves exactly this.
	o := podObj()
	o.Status.PodRecreates, o.Status.RecreateOf = 1, "pu1"
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o}, ownedPod(t, corev1.PodFailed))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if got := g.status(t, "crswd-abc").PodRecreates; got != 1 {
		t.Errorf("recreates = %d, want 1", got)
	}
	if n := len(g.podDeletes()); n != 1 {
		t.Errorf("pod deletes = %d, want 1", n)
	}
}

func TestRecreateReservesInStatusBeforeDelete(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, ownedPod(t, corev1.PodFailed))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	st := g.status(t, "crswd-abc")
	if st.PodRecreates != 1 || st.RecreateOf != "pu1" || st.Phase != v1alpha1.PhaseReviving {
		t.Errorf("status = %+v", st)
	}
}

// The count and the reservation are status only: an annotation anyone with
// update on the object can write must neither raise nor lower them.
func TestRecreateForgedAnnotationsHaveNoEffect(t *testing.T) {
	t.Parallel()
	forged := map[string]string{
		v1alpha1.Group + "/pod-recreates":   "0",
		v1alpha1.Group + "/pod-recreate-of": "pu1",
	}
	o := podObj()
	o.Metadata.Annotations = forged
	o.Status.PodRecreates = MaxRecreates
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o}, ownedPod(t, corev1.PodFailed))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := len(g.podDeletes()); n != 0 {
		t.Errorf("pod deletes = %d, want 0: a forged reservation bypassed the cap", n)
	}
	if st := g.status(t, "crswd-abc"); st.Phase != v1alpha1.PhaseFailed {
		t.Errorf("status = %+v, want Failed", st)
	}

	o2 := podObj()
	o2.Metadata.Annotations = forged
	g2 := newRig(t, podCfg(), []v1alpha1.AgentSession{o2}, ownedPod(t, corev1.PodFailed))
	if err := g2.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if st := g2.status(t, "crswd-abc"); st.PodRecreates != 1 || st.RecreateOf != "pu1" {
		t.Errorf("status = %+v, want a count of 1 that ignores the annotations", st)
	}
	if n := len(g2.podDeletes()); n != 1 {
		t.Errorf("pod deletes = %d, want 1", n)
	}
}
