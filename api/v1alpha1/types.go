// Package v1alpha1 is the wire shape of the AgentSession object. It is plain
// structs with no Kubernetes client dependency, so the daemon, the reconciler
// and the manifest generator share one definition without a second module.
package v1alpha1

import "time"

// The group is fixed by spec 017 FR-005. Changing it orphans every stored object.
const (
	Group      = "crswd.craigcloud.io"
	Version    = "v1alpha1"
	Kind       = "AgentSession"
	ListKind   = "AgentSessionList"
	Plural     = "agentsessions"
	Singular   = "agentsession"
	APIVersion = Group + "/" + Version
)

// ObjectMeta carries only the metadata the daemon and reconciler read.
type ObjectMeta struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace,omitempty"`
	UID               string            `json:"uid,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Finalizers        []string          `json:"finalizers,omitempty"`
}

// AgentSession asks the reconciler for one agent session pod.
type AgentSession struct {
	APIVersion string             `json:"apiVersion"`
	Kind       string             `json:"kind"`
	Metadata   ObjectMeta         `json:"metadata"`
	Spec       AgentSessionSpec   `json:"spec"`
	Status     AgentSessionStatus `json:"status"`
}

// AgentSessionSpec is deliberately small. Anyone who can write this object can
// cause a pod to run, so no field shapes the pod (image, service account, env,
// volumes, secrets, command, node, resources) and none carries a token. The
// reconciler owns the pod template; the spec only names what to run and where.
type AgentSessionSpec struct {
	SessionName string `json:"sessionName"`
	Owner       string `json:"owner"`
	WorkDir     string `json:"workDir"`
	// StartCommand is the configured start command key, never a command line.
	StartCommand string `json:"startCommand"`
	Conversation string `json:"conversation,omitempty"`
	// Lifetime is a Go duration such as "8h". A pod needs a finite
	// activeDeadlineSeconds, so "never" is not a valid value.
	Lifetime string `json:"lifetime"`
}

// AgentSessionStatus is written by the reconciler through the status subresource.
type AgentSessionStatus struct {
	Phase        Phase  `json:"phase,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Conversation string `json:"conversation,omitempty"`
}

// Phase is the reconciler's verdict on one AgentSession.
type Phase string

const (
	PhasePending  Phase = "Pending"
	PhaseRunning  Phase = "Running"
	PhaseRejected Phase = "Rejected"
	PhaseReviving Phase = "Reviving"
	PhaseFailed   Phase = "Failed"
)
