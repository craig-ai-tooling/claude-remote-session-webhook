package reconcile

import (
	"context"
	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"testing"
)

const recreateOfKey = AnnotationRecreateOf

// Finding 4: reserve the recreate before the delete.
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
	o.Metadata.Annotations = map[string]string{AnnotationRecreates: "1", recreateOfKey: "pu1"}
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o}, ownedPod(t, corev1.PodFailed))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	got, _, err := g.r.sessions.Get(context.Background(), "crswd-abc")
	if err != nil {
		t.Fatal(err)
	}
	if v := got.Metadata.Annotations[AnnotationRecreates]; v != "1" {
		t.Errorf("recreates = %q, want 1", v)
	}
	if n := len(g.podDeletes()); n != 1 {
		t.Errorf("pod deletes = %d, want 1", n)
	}
}

func TestRecreateReservesBeforeDelete(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, ownedPod(t, corev1.PodFailed))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	got, _, err := g.r.sessions.Get(context.Background(), "crswd-abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata.Annotations[AnnotationRecreates] != "1" || got.Metadata.Annotations[recreateOfKey] != "pu1" {
		t.Errorf("annotations = %v", got.Metadata.Annotations)
	}
}
