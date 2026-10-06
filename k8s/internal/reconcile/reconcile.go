package reconcile

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/admit"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	// AnnotationRecreates counts consecutive pod recreates for one object.
	AnnotationRecreates = v1alpha1.Group + "/pod-recreates"
	// MaxRecreates bounds a pod that fails at every start.
	MaxRecreates = 5
	// AnnotationRecreateOf records the UID of the failed pod whose recreate has
	// been counted, which makes the count-then-delete step idempotent.
	AnnotationRecreateOf = v1alpha1.Group + "/pod-recreate-of"
	// AnnotationConversation is the key podctl writes for @crswd-conversation.
	AnnotationConversation = v1alpha1.Group + "/conversation"

	reasonLifetime = "the session reached its lifetime"
)

// Reconciler keeps one pod per AgentSession.
type Reconciler struct {
	kube     kubernetes.Interface
	sessions *agentsession.Client
	dyn      dynamic.Interface
	cfg      Config
}

// New builds a Reconciler over the session namespace.
func New(kube kubernetes.Interface, dyn dynamic.Interface, cfg Config) (*Reconciler, error) {
	if kube == nil {
		return nil, errors.New("reconcile: nil kubernetes client")
	}
	sessions, err := agentsession.New(dyn, cfg.SessionNamespace)
	if err != nil {
		return nil, fmt.Errorf("reconcile: %w", err)
	}
	return &Reconciler{kube: kube, sessions: sessions, dyn: dyn, cfg: cfg}, nil
}

// setStatus writes phase and reason, and copies the conversation the daemon
// recorded on the object. It is the only writer of status.conversation (spec
// 017 FR-005). A write that would change nothing is skipped.
func (r *Reconciler) setStatus(ctx context.Context, obj v1alpha1.AgentSession, phase v1alpha1.Phase, reason string) error {
	want := v1alpha1.AgentSessionStatus{Phase: phase, Reason: reason, Conversation: obj.Status.Conversation}
	if c := obj.Metadata.Annotations[AnnotationConversation]; c != "" {
		want.Conversation = c
	}
	if want == obj.Status {
		return nil
	}
	return r.sessions.UpdateStatus(ctx, obj, want)
}

// ReconcileOne drives one object toward one pod. The first matching rule wins.
func (r *Reconciler) ReconcileOne(ctx context.Context, name string) error {
	obj, found, err := r.sessions.Get(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		// The owner reference lets the garbage collector delete the pod.
		return nil
	}
	if obj.Status.Phase == v1alpha1.PhaseRejected || obj.Status.Phase == v1alpha1.PhaseFailed {
		return r.endRejectedPod(ctx, obj)
	}

	pods := r.kube.CoreV1().Pods(r.cfg.SessionNamespace)
	pod, err := pods.Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return r.reconcileWithPod(ctx, obj, pod)
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("reconcile: get pod %s: %w", name, err)
	}

	// A deleting object gets no pod; the owner reference lets the garbage
	// collector clear any that exists.
	if obj.Metadata.DeletionTimestamp != nil {
		return nil
	}

	others, err := r.sessions.List(ctx)
	if err != nil {
		return err
	}
	if ok, reason := admit.Admit(obj, r.cfg.Roots(), r.cfg.Cap, r.cfg.LifetimeMax, others, r.cfg.now()); !ok {
		return r.setStatus(ctx, obj, v1alpha1.PhaseRejected, reason)
	}
	want, err := PodFor(obj, r.cfg, r.cfg.now())
	if errors.Is(err, ErrLifetimeOver) {
		return r.setStatus(ctx, obj, v1alpha1.PhaseFailed, reasonLifetime)
	}
	if err != nil {
		return err
	}

	// Read the object live, never from the informer cache, so a delete that
	// podctl.Kill has already confirmed is seen before a pod is made.
	live, found, err := r.sessions.Get(ctx, name)
	if err != nil {
		return err
	}
	if !found || live.Metadata.UID != obj.Metadata.UID || live.Metadata.DeletionTimestamp != nil {
		return nil
	}

	if _, err := pods.Create(ctx, want, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("reconcile: create pod %s: %w", name, err)
	}
	phase := v1alpha1.PhasePending
	if obj.Status.Phase == v1alpha1.PhaseRunning || obj.Status.Phase == v1alpha1.PhaseReviving {
		phase = v1alpha1.PhaseReviving
	}
	return r.setStatus(ctx, obj, phase, "")
}

// endRejectedPod finishes a rejection whose pod delete failed: the status is
// written first, so without this the retry would see a terminal object and
// leave the pod running.
func (r *Reconciler) endRejectedPod(ctx context.Context, obj v1alpha1.AgentSession) error {
	if obj.Status.Phase != v1alpha1.PhaseRejected {
		return nil
	}
	pod, err := r.kube.CoreV1().Pods(r.cfg.SessionNamespace).Get(ctx, obj.Metadata.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reconcile: get pod %s: %w", obj.Metadata.Name, err)
	}
	if !ownedBy(pod, obj) || pod.DeletionTimestamp != nil {
		return nil
	}
	return r.deletePod(ctx, pod)
}

func (r *Reconciler) reconcileWithPod(ctx context.Context, obj v1alpha1.AgentSession, pod *corev1.Pod) error {
	if !ownedBy(pod, obj) {
		return r.setStatus(ctx, obj, v1alpha1.PhaseFailed, "a pod with this session's name exists and is not owned by it")
	}
	// No replacement while the old pod object exists, terminating included
	// (FR-010: two writers fork a conversation).
	if pod.DeletionTimestamp != nil {
		return nil
	}
	// The CRD makes the spec immutable, but an object that predates the rule, or
	// a cluster without it, must not leave an inadmissible session running. The
	// cap is left out: it counts creation order and would evict a running one.
	if live := pod.Status.Phase; live == corev1.PodRunning || live == corev1.PodPending || live == "" {
		if ok, reason := admit.CheckSpec(obj, r.cfg.Roots(), r.cfg.LifetimeMax); !ok {
			return r.rejectLive(ctx, obj, pod, reason)
		}
	}
	_, counted := obj.Metadata.Annotations[AnnotationRecreates]
	_, reserved := obj.Metadata.Annotations[AnnotationRecreateOf]

	switch pod.Status.Phase {
	case corev1.PodFailed, corev1.PodSucceeded:
		if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "DeadlineExceeded" {
			return r.setStatus(ctx, obj, v1alpha1.PhaseFailed, reasonLifetime)
		}
		n := recreateCount(obj.Metadata.Annotations)
		reservedHere := obj.Metadata.Annotations[AnnotationRecreateOf] == string(pod.UID)
		// An attempt already reserved for this pod passed the cap when it was made.
		if n >= MaxRecreates && !reservedHere {
			return r.setStatus(ctx, obj, v1alpha1.PhaseFailed, fmt.Sprintf("the session pod failed %d times in a row", MaxRecreates))
		}
		// Reserve the attempt before the pod goes: a crash between the two
		// writes then costs a retry of the delete, not an uncounted recreate.
		// The pod's UID marks the reservation, so a second pass over the same
		// failed pod deletes without counting again.
		if !reservedHere {
			var err error
			obj, err = r.sessions.Update(ctx, obj, func(o *v1alpha1.AgentSession) {
				if o.Metadata.Annotations == nil {
					o.Metadata.Annotations = map[string]string{}
				}
				o.Metadata.Annotations[AnnotationRecreates] = strconv.Itoa(n + 1)
				o.Metadata.Annotations[AnnotationRecreateOf] = string(pod.UID)
			})
			if err != nil {
				return err
			}
		}
		if err := r.deletePod(ctx, pod); err != nil {
			return err
		}
		return r.setStatus(ctx, obj, v1alpha1.PhaseReviving, "")
	case corev1.PodRunning:
		if counted || reserved {
			var err error
			obj, err = r.sessions.Update(ctx, obj, func(o *v1alpha1.AgentSession) {
				delete(o.Metadata.Annotations, AnnotationRecreates)
				delete(o.Metadata.Annotations, AnnotationRecreateOf)
			})
			if err != nil {
				return err
			}
		}
		return r.setStatus(ctx, obj, v1alpha1.PhaseRunning, "")
	default:
		// A Pending pod is left alone, never recreated in a loop.
		if obj.Status.Phase == "" {
			return r.setStatus(ctx, obj, v1alpha1.PhasePending, "")
		}
		return nil
	}
}

// rejectLive records the verdict before it ends the pod, so a crash between the
// two leaves the object Rejected rather than a running pod nobody judged. The
// delete keeps the pod's own grace period and is bound to its UID.
func (r *Reconciler) rejectLive(ctx context.Context, obj v1alpha1.AgentSession, pod *corev1.Pod, reason string) error {
	if err := r.setStatus(ctx, obj, v1alpha1.PhaseRejected, reason); err != nil {
		return err
	}
	return r.deletePod(ctx, pod)
}

func (r *Reconciler) deletePod(ctx context.Context, pod *corev1.Pod) error {
	uid := pod.UID
	err := r.kube.CoreV1().Pods(r.cfg.SessionNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("reconcile: delete pod %s: %w", pod.Name, err)
	}
	return nil
}

// ownedBy is true when the pod is this object's controlled, labelled pod.
func ownedBy(pod *corev1.Pod, obj v1alpha1.AgentSession) bool {
	ref := metav1.GetControllerOf(pod)
	return ref != nil && string(ref.UID) == obj.Metadata.UID &&
		pod.Labels[LabelManagedBy] == ManagedByValue
}

// recreateCount reads the pod-recreates annotation. Absent means no recreate
// yet. Present but not a non-negative integer is corrupt state, and for a
// counter that bounds recreation the safe reading is "already at the cap", so
// the session fails instead of being recreated without limit.
func recreateCount(annotations map[string]string) int {
	raw, ok := annotations[AnnotationRecreates]
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return MaxRecreates
	}
	return n
}
