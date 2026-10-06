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
	return r.sessions.UpdateStatus(ctx, obj.Metadata.Name, want)
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
		return nil
	}

	pods := r.kube.CoreV1().Pods(r.cfg.SessionNamespace)
	pod, err := pods.Get(ctx, name, metav1.GetOptions{})
	switch {
	case err == nil:
		return r.reconcileWithPod(ctx, obj, pod)
	case !apierrors.IsNotFound(err):
		return fmt.Errorf("reconcile: get pod %s: %w", name, err)
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
	if !found || live.Metadata.UID != obj.Metadata.UID {
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

func (r *Reconciler) reconcileWithPod(ctx context.Context, obj v1alpha1.AgentSession, pod *corev1.Pod) error {
	if !ownedBy(pod, obj) {
		return r.setStatus(ctx, obj, v1alpha1.PhaseFailed, "a pod with this session's name exists and is not owned by it")
	}
	// No replacement while the old pod object exists, terminating included
	// (FR-010: two writers fork a conversation).
	if pod.DeletionTimestamp != nil {
		return nil
	}
	name := obj.Metadata.Name
	_, counted := obj.Metadata.Annotations[AnnotationRecreates]

	switch pod.Status.Phase {
	case corev1.PodFailed, corev1.PodSucceeded:
		if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "DeadlineExceeded" {
			return r.setStatus(ctx, obj, v1alpha1.PhaseFailed, reasonLifetime)
		}
		n, _ := strconv.Atoi(obj.Metadata.Annotations[AnnotationRecreates])
		if n >= MaxRecreates {
			return r.setStatus(ctx, obj, v1alpha1.PhaseFailed, fmt.Sprintf("the session pod failed %d times in a row", MaxRecreates))
		}
		uid := pod.UID
		err := r.kube.CoreV1().Pods(r.cfg.SessionNamespace).Delete(ctx, name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("reconcile: delete pod %s: %w", name, err)
		}
		err = r.sessions.Update(ctx, name, func(o *v1alpha1.AgentSession) {
			if o.Metadata.Annotations == nil {
				o.Metadata.Annotations = map[string]string{}
			}
			o.Metadata.Annotations[AnnotationRecreates] = strconv.Itoa(n + 1)
		})
		if err != nil {
			return err
		}
		return r.setStatus(ctx, obj, v1alpha1.PhaseReviving, "")
	case corev1.PodRunning:
		if counted {
			err := r.sessions.Update(ctx, name, func(o *v1alpha1.AgentSession) {
				delete(o.Metadata.Annotations, AnnotationRecreates)
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

// ownedBy is true when the pod is this object's controlled, labelled pod.
func ownedBy(pod *corev1.Pod, obj v1alpha1.AgentSession) bool {
	ref := metav1.GetControllerOf(pod)
	return ref != nil && string(ref.UID) == obj.Metadata.UID &&
		pod.Labels[LabelManagedBy] == ManagedByValue
}
