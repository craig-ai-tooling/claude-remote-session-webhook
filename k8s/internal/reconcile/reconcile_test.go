package reconcile

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

type rig struct {
	r    *Reconciler
	kube *kubefake.Clientset
	dyn  *dynamicfake.FakeDynamicClient
}

func newRig(t *testing.T, cfg Config, objs []v1alpha1.AgentSession, pods ...*corev1.Pod) *rig {
	t.Helper()
	var robjs []runtime.Object
	for _, o := range objs {
		o.Metadata.Namespace = cfg.SessionNamespace
		u, err := agentsession.FromObject(o)
		if err != nil {
			t.Fatal(err)
		}
		robjs = append(robjs, u)
	}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{agentsession.GVR: v1alpha1.ListKind}, robjs...)
	var kobjs []runtime.Object
	for _, p := range pods {
		kobjs = append(kobjs, p)
	}
	kc := kubefake.NewSimpleClientset(kobjs...)
	r, err := New(kc, dyn, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &rig{r: r, kube: kc, dyn: dyn}
}

func (g *rig) count(verb, resource string) int {
	n := 0
	for _, a := range g.kube.Actions() {
		if a.Matches(verb, resource) {
			n++
		}
	}
	return n
}

func (g *rig) status(t *testing.T, name string) v1alpha1.AgentSessionStatus {
	t.Helper()
	o, ok, err := g.r.sessions.Get(context.Background(), name)
	if err != nil || !ok {
		t.Fatalf("get %s: ok=%v err=%v", name, ok, err)
	}
	return o.Status
}

func ownedPod(t *testing.T, phase corev1.PodPhase) *corev1.Pod {
	t.Helper()
	p, err := PodFor(podObj(), podCfg(), podNow)
	if err != nil {
		t.Fatal(err)
	}
	p.UID = "pu1"
	p.Status.Phase = phase
	return p
}

func TestReconcileCreatesMissingPod(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := g.count("create", "pods"); n != 1 {
		t.Errorf("creates = %d, want 1", n)
	}
	if got := g.status(t, "crswd-abc").Phase; got != v1alpha1.PhasePending {
		t.Errorf("phase = %q, want Pending", got)
	}
}

func TestReconcileTerminatingPodNoCreate(t *testing.T) {
	t.Parallel()
	p := ownedPod(t, corev1.PodRunning)
	now := metav1.NewTime(podNow)
	p.DeletionTimestamp = &now
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, p)
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d, want 0", n)
	}
	if n := g.count("delete", "pods"); n != 0 {
		t.Errorf("deletes = %d, want 0", n)
	}
}

func TestReconcileFailedPodDeletedOnce(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, ownedPod(t, corev1.PodFailed))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	var del *clienttesting.DeleteActionImpl
	for _, a := range g.kube.Actions() {
		if d, ok := a.(clienttesting.DeleteActionImpl); ok {
			del = &d
		}
	}
	if del == nil {
		t.Fatal("no delete action")
	}
	if del.DeleteOptions.GracePeriodSeconds != nil {
		t.Error("delete overrides the grace period")
	}
	pre := del.DeleteOptions.Preconditions
	if pre == nil || pre.UID == nil || *pre.UID != types.UID("pu1") {
		t.Errorf("delete lacks the pod UID precondition: %+v", pre)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d in the same call, want 0", n)
	}
	if got := g.status(t, "crswd-abc").Phase; got != v1alpha1.PhaseReviving {
		t.Errorf("phase = %q, want Reviving", got)
	}
	o, _, err := g.r.sessions.Get(context.Background(), "crswd-abc")
	if err != nil {
		t.Fatal(err)
	}
	if o.Metadata.Annotations[AnnotationRecreates] != "1" {
		t.Errorf("recreates = %q, want 1", o.Metadata.Annotations[AnnotationRecreates])
	}
}

func TestReconcileDeadlineExceeded(t *testing.T) {
	t.Parallel()
	p := ownedPod(t, corev1.PodFailed)
	p.Status.Reason = "DeadlineExceeded"
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, p)
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if g.count("delete", "pods")+g.count("create", "pods") != 0 {
		t.Error("touched a pod")
	}
	if st := g.status(t, "crswd-abc"); st.Phase != v1alpha1.PhaseFailed || st.Reason != "the session reached its lifetime" {
		t.Errorf("status = %+v", st)
	}
}

func TestReconcileOutsideRoot(t *testing.T) {
	t.Parallel()
	o := podObj()
	o.Spec.WorkDir = "/etc"
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if st := g.status(t, "crswd-abc"); st.Phase != v1alpha1.PhaseRejected || st.Reason == "" {
		t.Errorf("status = %+v", st)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d", n)
	}
}

func TestReconcileOverCap(t *testing.T) {
	t.Parallel()
	cfg := podCfg()
	cfg.Cap = 1
	older := podObj()
	older.Metadata.Name = "crswd-old"
	older.Metadata.UID = "u0"
	older.Metadata.CreationTimestamp = podNow.Add(-2 * time.Hour)
	older.Status.Phase = v1alpha1.PhaseRunning
	g := newRig(t, cfg, []v1alpha1.AgentSession{older, podObj()})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if got := g.status(t, "crswd-abc").Phase; got != v1alpha1.PhaseRejected {
		t.Errorf("phase = %q, want Rejected", got)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d", n)
	}
}

func TestReconcilePastLifetime(t *testing.T) {
	t.Parallel()
	o := podObj()
	o.Metadata.CreationTimestamp = podNow.Add(-9 * time.Hour)
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if p := g.status(t, "crswd-abc").Phase; p != v1alpha1.PhaseRejected && p != v1alpha1.PhaseFailed {
		t.Errorf("phase = %q", p)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d", n)
	}
}

func TestReconcileObjectAbsent(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), nil)
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if a := g.kube.Actions(); len(a) != 0 {
		t.Errorf("kube actions = %d, want 0", len(a))
	}
}

func TestReconcileRunningObjectPodGoneRevives(t *testing.T) {
	t.Parallel()
	o := podObj()
	o.Status.Phase = v1alpha1.PhaseRunning
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := g.count("create", "pods"); n != 1 {
		t.Errorf("creates = %d, want 1", n)
	}
	if got := g.status(t, "crswd-abc").Phase; got != v1alpha1.PhaseReviving {
		t.Errorf("phase = %q, want Reviving", got)
	}
}

func TestReconcileAlreadyExistsIsNotAnError(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()})
	g.kube.PrependReactor("create", "pods", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{Resource: "pods"}, "crswd-abc")
	})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Errorf("err = %v, want nil", err)
	}
}

func TestReconcileRecreateCapReached(t *testing.T) {
	t.Parallel()
	for name, val := range map[string]string{
		"at the cap":                       strconv.Itoa(MaxRecreates),
		"garbage recreates annotation":     "three",
		"empty recreates annotation":       "",
		"negative recreates annotation":    "-1",
		"overflowing recreates annotation": "99999999999999999999",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			o := podObj()
			o.Metadata.Annotations = map[string]string{AnnotationRecreates: val}
			g := newRig(t, podCfg(), []v1alpha1.AgentSession{o}, ownedPod(t, corev1.PodFailed))
			if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
				t.Fatal(err)
			}
			if n := g.count("delete", "pods"); n != 0 {
				t.Errorf("deletes = %d, want 0", n)
			}
			if st := g.status(t, "crswd-abc"); st.Phase != v1alpha1.PhaseFailed {
				t.Errorf("status = %+v", st)
			}
		})
	}
}

func TestReconcileRunningPodClearsRecreateCount(t *testing.T) {
	t.Parallel()
	o := podObj()
	o.Metadata.Annotations = map[string]string{AnnotationRecreates: "2"}
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o}, ownedPod(t, corev1.PodRunning))
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	got, _, err := g.r.sessions.Get(context.Background(), "crswd-abc")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Metadata.Annotations[AnnotationRecreates]; ok {
		t.Error("recreate count not cleared")
	}
	if got.Status.Phase != v1alpha1.PhaseRunning {
		t.Errorf("phase = %q, want Running", got.Status.Phase)
	}
}

func TestReconcileForeignPodUntouched(t *testing.T) {
	t.Parallel()
	p := ownedPod(t, corev1.PodFailed)
	p.OwnerReferences[0].UID = "someone-else"
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()}, p)
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if g.count("delete", "pods")+g.count("create", "pods") != 0 {
		t.Error("touched a pod it does not own")
	}
	if got := g.status(t, "crswd-abc").Phase; got != v1alpha1.PhaseFailed {
		t.Errorf("phase = %q, want Failed", got)
	}
}

func TestReconcileCopiesConversation(t *testing.T) {
	t.Parallel()
	o := podObj()
	o.Metadata.Annotations = map[string]string{AnnotationConversation: "conv-1"}
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{o})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if got := g.status(t, "crswd-abc").Conversation; got != "conv-1" {
		t.Errorf("conversation = %q", got)
	}
}

func TestReconcileLiveReadNotFoundCreatesNothing(t *testing.T) {
	t.Parallel()
	g := newRig(t, podCfg(), []v1alpha1.AgentSession{podObj()})
	gets := 0
	g.dyn.PrependReactor("get", "agentsessions", func(clienttesting.Action) (bool, runtime.Object, error) {
		gets++
		if gets >= 2 {
			return true, nil, apierrors.NewNotFound(agentsession.GVR.GroupResource(), "crswd-abc")
		}
		return false, nil, nil
	})
	if err := g.r.ReconcileOne(context.Background(), "crswd-abc"); err != nil {
		t.Fatal(err)
	}
	if n := g.count("create", "pods"); n != 0 {
		t.Errorf("creates = %d, want 0", n)
	}
}
