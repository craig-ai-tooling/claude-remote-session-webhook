package reconcile

import (
	"context"
	"errors"
	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
	"testing"
)

// Finding 3: a status write is bound to the object it was computed from.
func TestStatusWriteRefusesOtherUID(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, ownedPod(t, corev1.PodRunning))
	gets := 0
	g.dyn.PrependReactor("get", "agentsessions", func(clienttesting.Action) (bool, runtime.Object, error) {
		gets++
		if gets < 2 {
			return false, nil, nil
		}
		o := podObj()
		o.Metadata.Namespace, o.Metadata.UID = "sessions", "u-other"
		u, err := agentsession.FromObject(o)
		return true, u, err
	})
	err := g.r.ReconcileOne(context.Background(), "crswd-abc")
	if !apierrors.IsConflict(err) {
		t.Errorf("err = %v, want a conflict", err)
	}
	if n := g.statusWrites(); n != 0 {
		t.Errorf("status writes = %d, want 0", n)
	}
}

func TestStatusWriteRefusesChangedResourceVersion(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, ownedPod(t, corev1.PodRunning))
	gets := 0
	g.dyn.PrependReactor("get", "agentsessions", func(clienttesting.Action) (bool, runtime.Object, error) {
		gets++
		o := podObj()
		o.Metadata.Namespace = "sessions"
		u, err := agentsession.FromObject(o)
		if err != nil {
			return true, nil, err
		}
		u.SetResourceVersion("rv" + string(rune('0'+gets)))
		return true, u, nil
	})
	err := g.r.ReconcileOne(context.Background(), "crswd-abc")
	if !apierrors.IsConflict(err) {
		t.Errorf("err = %v, want a conflict", err)
	}
	if n := g.statusWrites(); n != 0 {
		t.Errorf("status writes = %d, want 0", n)
	}
}

func TestRequeueOnConflictIsImmediate(t *testing.T) {
	t.Parallel()
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	defer q.ShutDown()
	conflict := apierrors.NewConflict(schema.GroupResource{Resource: "agentsessions"}, "x", nil)
	for i := 0; i < 3; i++ {
		finish(q, "x", conflict)
	}
	if n := q.NumRequeues("x"); n != 0 {
		t.Errorf("requeues = %d, a conflict must not count as a failure", n)
	}
	finish(q, "y", errors.New("boom"))
	if n := q.NumRequeues("y"); n != 1 {
		t.Errorf("requeues = %d, want 1 for a real failure", n)
	}
}
