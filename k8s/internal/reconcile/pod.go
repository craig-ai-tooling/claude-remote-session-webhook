package reconcile

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ErrLifetimeOver means the object's deadline has already passed, so no pod
// should be made for it.
var ErrLifetimeOver = errors.New("reconcile: the session is past its lifetime")

const (
	claudeCredsMount = "/var/run/claude-creds" //nolint:gosec // G101: a mount path, not a credential
	codexCredsMount  = "/var/run/codex-creds"  //nolint:gosec // G101: a mount path, not a credential

	// linkScript keeps a credentials link pointing at the directory-mounted
	// Secret. It is the only shell string in the package, it is a constant, and
	// nothing from the object reaches it. The existence guard keeps an absent
	// optional Secret from leaving a dangling link.
	linkScript = `set -u
while :; do
  if [ -e "$TARGET" ] && [ "$(readlink "$LINK" 2>/dev/null)" != "$TARGET" ]; then
    if [ -e "$LINK" ] || [ -L "$LINK" ]; then
      echo "creds-link: $(date -u +%FT%TZ) $LINK was replaced; re-linked to $TARGET"
    fi
    ln -sfn "$TARGET" "$LINK"
  fi
  sleep "$CREDS_LINK_INTERVAL"
done`

	linkReady = `[ ! -e "$TARGET" ] || [ "$(readlink "$LINK")" = "$TARGET" ]`
)

func ptr[T any](v T) *T { return &v }

// PodFor builds the one pod that runs a Claude Code or a Codex session. It is
// pure and reads the object only for its name, UID, workDir and lifetime: the
// spec has no pod-shaping field and nothing here may grow one.
//
// ActiveDeadlineSeconds counts from the pod's start, not its creation, so a pod
// that waits to be scheduled outlives the object's deadline by that wait. It is
// the backstop for a down daemon; the reaper ends the session at its exact
// deadline through podctl.Kill.
func PodFor(obj v1alpha1.AgentSession, cfg Config, now time.Time) (*corev1.Pod, error) {
	lifetime, err := time.ParseDuration(obj.Spec.Lifetime)
	if err != nil {
		return nil, fmt.Errorf("reconcile: spec.lifetime is not a duration: %w", err)
	}
	deadline := obj.Metadata.CreationTimestamp.Add(lifetime)
	secs := int64(math.Ceil(deadline.Sub(now).Seconds()))
	if secs < 1 {
		return nil, ErrLifetimeOver
	}

	n := obj.Metadata.Name
	claudeDir := cfg.Home + "/.claude"
	codexDir := cfg.Home + "/.codex"

	nodeSelector := map[string]string{"kubernetes.io/arch": "amd64"}
	if cfg.Node != "" {
		nodeSelector["kubernetes.io/hostname"] = cfg.Node
	}

	sidecarRes := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("5m"),
			corev1.ResourceMemory: resource.MustParse("16Mi"),
		},
		Limits: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("32Mi")},
	}
	sidecar := func(name, link, target string, mounts []corev1.VolumeMount) corev1.Container {
		return corev1.Container{
			Name:          name,
			Image:         cfg.Image,
			Command:       []string{"bash", "-c", linkScript},
			RestartPolicy: ptr(corev1.ContainerRestartPolicyAlways),
			Env: []corev1.EnvVar{
				{Name: "LINK", Value: link},
				{Name: "TARGET", Value: target},
				{Name: "CREDS_LINK_INTERVAL", Value: "60"},
			},
			VolumeMounts: mounts,
			Resources:    sidecarRes,
			StartupProbe: &corev1.Probe{
				ProbeHandler:     corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"bash", "-c", linkReady}}},
				PeriodSeconds:    1,
				FailureThreshold: 30,
			},
		}
	}

	session := corev1.Container{
		Name:       SessionContainer,
		Image:      cfg.Image,
		Args:       []string{"session-pod", n, obj.Spec.WorkDir},
		WorkingDir: sessionpod.WorkRoot,
		Env: []corev1.EnvVar{
			{Name: "HOME", Value: cfg.Home},
			{Name: "CLAUDE_CONFIG_DIR", Value: claudeDir},
			{Name: "CODEX_HOME", Value: codexDir},
			{Name: "LANG", Value: "C.UTF-8"},
			{Name: "TERM", Value: "xterm-256color"},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "sessions", MountPath: sessionpod.WorkRoot, SubPath: n + "/work"},
			{Name: "claude-config", MountPath: claudeDir},
			{Name: "sessions", MountPath: claudeDir + "/projects", SubPath: n + "/projects"},
			{Name: "sessions", MountPath: codexDir, SubPath: n + "/codex"},
			{Name: "claude-creds", MountPath: claudeCredsMount, ReadOnly: true},
			{Name: "codex-creds", MountPath: codexCredsMount, ReadOnly: true},
		},
		// No limits (FR-008).
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("570Mi"),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		},
	}

	// Both Secrets are directory mounts, never SubPath: a SubPath mount never
	// sees the keeper's rotation (FR-015). Both are optional so a namespace
	// with one runtime signed in still runs.
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n,
			Namespace: cfg.SessionNamespace,
			Labels:    map[string]string{LabelManagedBy: ManagedByValue, LabelSession: n},
			// BlockOwnerDeletion stays nil: setting it needs permission on the
			// owner's finalizers, which the reconciler's Role does not grant.
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: v1alpha1.APIVersion,
				Kind:       v1alpha1.Kind,
				Name:       n,
				UID:        types.UID(obj.Metadata.UID),
				Controller: ptr(true),
			}},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			AutomountServiceAccountToken:  ptr(false),
			EnableServiceLinks:            ptr(false),
			ActiveDeadlineSeconds:         &secs,
			TerminationGracePeriodSeconds: ptr(int64(30)),
			NodeSelector:                  nodeSelector,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    ptr(cfg.UID),
				RunAsGroup:   ptr(cfg.UID),
				FSGroup:      ptr(cfg.UID),
				RunAsNonRoot: ptr(true),
			},
			Volumes: []corev1.Volume{
				{Name: "sessions", VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: cfg.Claim}}},
				{Name: "claude-config", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "claude-creds", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
					SecretName: cfg.ClaudeSecret, Optional: ptr(true), DefaultMode: ptr(int32(0o440))}}},
				{Name: "codex-creds", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
					SecretName: cfg.CodexSecret, Optional: ptr(true), DefaultMode: ptr(int32(0o440))}}},
			},
			// Each link is resolved in the session container's mount namespace,
			// so the Secret is mounted there at the same path as in the sidecar.
			InitContainers: []corev1.Container{
				sidecar("creds-link", claudeDir+"/.credentials.json", claudeCredsMount+"/.credentials.json", []corev1.VolumeMount{
					{Name: "claude-config", MountPath: claudeDir},
					{Name: "claude-creds", MountPath: claudeCredsMount, ReadOnly: true},
				}),
				sidecar("codex-creds-link", codexDir+"/auth.json", codexCredsMount+"/auth.json", []corev1.VolumeMount{
					{Name: "sessions", MountPath: codexDir, SubPath: n + "/codex"},
					{Name: "codex-creds", MountPath: codexCredsMount, ReadOnly: true},
				}),
			},
			Containers: []corev1.Container{session},
		},
	}, nil
}
