package reconcile

import (
	"context"
	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"testing"
	"time"
)

// Finding 2: deletionTimestamp.
func TestReconcileDeletingObjectCreatesNoPod(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()})
	u, err := g.dyn.Resource(agentsession.GVR).Namespace("sessions").Get(context.Background(), "crswd-abc", getOpts)
	if err != nil {
		t.Fatal(err)
	}
	u.SetDeletionTimestamp(&metaNow)
	if _, err := g.dyn.Resource(agentsession.GVR).Namespace("sessions").Update(context.Background(), u, updOpts); err != nil {
		t.Fatal(err)
	}
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d, want 0", n)
	}
}

func TestReconcileLiveReadDeletingCreatesNothing(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()})
	gets := 0
	g.dyn.PrependReactor("get", "agentsessions", func(a clienttesting.Action) (bool, runtime.Object, error) {
		gets++
		if gets < 2 {
			return false, nil, nil
		}
		o := podObj()
		o.Metadata.Namespace = "sessions"
		u, err := agentsession.FromObject(o)
		if err != nil {
			return true, nil, err
		}
		u.SetDeletionTimestamp(&metaNow)
		return true, u, nil
	})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d, want 0", n)
	}
}

var (
	metaNow = metaTime(podNow)
	getOpts = metaGet()
	updOpts = metaUpd()
)

func metaTime(t time.Time) metav1.Time { return metav1.NewTime(t) }
func metaGet() metav1.GetOptions       { return metav1.GetOptions{} }
func metaUpd() metav1.UpdateOptions    { return metav1.UpdateOptions{} }
