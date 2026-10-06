package reconcile

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var podNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func podCfg() Config {
	return Config{ //nolint:gosec // G101: Secret names in a test fixture, not credentials
		SessionNamespace: "sessions",
		LeaseNamespace:   "crswd",
		Image:            "registry.example/crswd-session:1.2.3",
		Claim:            "crswd-sessions",
		Cap:              3,
		ClaudeSecret:     "claude-credentials",
		CodexSecret:      "codex-auth",
		Home:             "/home/ralph",
		UID:              10001,
		Now:              func() time.Time { return podNow },
	}
}

func podObj() v1alpha1.AgentSession {
	return v1alpha1.AgentSession{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata: v1alpha1.ObjectMeta{
			Name:              "crswd-abc",
			UID:               "u1",
			CreationTimestamp: podNow.Add(-time.Hour),
		},
		Spec: v1alpha1.AgentSessionSpec{
			SessionName:  "abc",
			Owner:        "me",
			WorkDir:      "/work/repo",
			StartCommand: "claude",
			Lifetime:     "8h",
		},
	}
}

func allContainers(p *corev1.Pod) []corev1.Container {
	return append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...)
}

func mountOf(c corev1.Container, path string) (corev1.VolumeMount, bool) {
	for _, m := range c.VolumeMounts {
		if m.MountPath == path {
			return m, true
		}
	}
	return corev1.VolumeMount{}, false
}

func envOf2(c corev1.Container, name string) string {
	for _, e := range c.Env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

func TestPodForDeadline(t *testing.T) {
	t.Parallel()
	p, err := PodFor(podObj(), podCfg(), podNow)
	if err != nil {
		t.Fatalf("PodFor: %v", err)
	}
	if p.Spec.ActiveDeadlineSeconds == nil || *p.Spec.ActiveDeadlineSeconds != 25200 {
		t.Errorf("ActiveDeadlineSeconds = %v, want 25200", p.Spec.ActiveDeadlineSeconds)
	}
	o := podObj()
	o.Metadata.CreationTimestamp = podNow.Add(-8 * time.Hour)
	if _, err := PodFor(o, podCfg(), podNow); !errors.Is(err, ErrLifetimeOver) {
		t.Errorf("err = %v, want ErrLifetimeOver", err)
	}
	o.Spec.Lifetime = "never"
	if _, err := PodFor(o, podCfg(), podNow); err == nil || errors.Is(err, ErrLifetimeOver) {
		t.Errorf("err = %v, want a parse error", err)
	}
}

func TestPodForNoSecretReadNoHostAccess(t *testing.T) {
	t.Parallel()
	p, err := PodFor(podObj(), podCfg(), podNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range p.Spec.Volumes {
		if v.HostPath != nil {
			t.Errorf("volume %s is a HostPath", v.Name)
		}
	}
	if p.Spec.HostNetwork {
		t.Error("HostNetwork set")
	}
	if p.Spec.AutomountServiceAccountToken == nil || *p.Spec.AutomountServiceAccountToken {
		t.Error("AutomountServiceAccountToken is not false")
	}
	if p.Spec.ServiceAccountName != "" {
		t.Errorf("ServiceAccountName = %q", p.Spec.ServiceAccountName)
	}
	secretVols := map[string]bool{}
	for _, v := range p.Spec.Volumes {
		if v.Secret != nil {
			secretVols[v.Name] = true
		}
	}
	for _, c := range allContainers(p) {
		if len(c.Ports) != 0 {
			t.Errorf("%s has a port", c.Name)
		}
		for _, e := range c.Env {
			if e.ValueFrom != nil {
				t.Errorf("%s env %s uses ValueFrom", c.Name, e.Name)
			}
		}
		for _, m := range c.VolumeMounts {
			if secretVols[m.Name] && m.SubPath != "" {
				t.Errorf("%s mounts Secret volume %s with a SubPath", c.Name, m.Name)
			}
		}
	}
}

func TestPodForBothRuntimes(t *testing.T) {
	t.Parallel()
	p, err := PodFor(podObj(), podCfg(), podNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Spec.Containers) != 1 || p.Spec.Containers[0].Name != SessionContainer {
		t.Fatalf("containers = %+v", p.Spec.Containers)
	}
	s := p.Spec.Containers[0]
	if envOf2(s, "CLAUDE_CONFIG_DIR") != "/home/ralph/.claude" || envOf2(s, "CODEX_HOME") != "/home/ralph/.codex" {
		t.Errorf("env = %+v", s.Env)
	}
	if m, ok := mountOf(s, "/home/ralph/.codex"); !ok || m.Name != "sessions" || m.SubPath != "crswd-abc/codex" {
		t.Errorf("codex mount = %+v", m)
	}
	if m, ok := mountOf(s, "/home/ralph/.claude/projects"); !ok || m.Name != "sessions" || m.SubPath != "crswd-abc/projects" {
		t.Errorf("projects mount = %+v", m)
	}
	if m, ok := mountOf(s, "/work"); !ok || m.SubPath != "crswd-abc/work" {
		t.Errorf("work mount = %+v", m)
	}
	if got := strings.Join(s.Args, " "); got != "session-pod crswd-abc /work/repo" {
		t.Errorf("Args = %q", got)
	}
	if len(p.Spec.InitContainers) != 2 {
		t.Fatalf("init containers = %d, want 2", len(p.Spec.InitContainers))
	}
	for _, sc := range p.Spec.InitContainers {
		if sc.RestartPolicy == nil || *sc.RestartPolicy != corev1.ContainerRestartPolicyAlways {
			t.Errorf("%s is not a native sidecar", sc.Name)
		}
		if sc.StartupProbe == nil {
			t.Errorf("%s has no startup probe", sc.Name)
		}
		target := envOf2(sc, "TARGET")
		dir := target[:strings.LastIndex(target, "/")]
		if _, ok := mountOf(s, dir); !ok {
			t.Errorf("%s links to %s, which the session container does not mount", sc.Name, target)
		}
	}
}

func TestPodForOwnerAndLabels(t *testing.T) {
	t.Parallel()
	p, err := PodFor(podObj(), podCfg(), podNow)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "crswd-abc" || p.Namespace != "sessions" {
		t.Errorf("name, ns = %q, %q", p.Name, p.Namespace)
	}
	if p.Labels[LabelManagedBy] != ManagedByValue || p.Labels[LabelSession] != "crswd-abc" {
		t.Errorf("labels = %v", p.Labels)
	}
	if len(p.OwnerReferences) != 1 {
		t.Fatalf("owner refs = %d", len(p.OwnerReferences))
	}
	o := p.OwnerReferences[0]
	if string(o.UID) != "u1" || o.Controller == nil || !*o.Controller || o.BlockOwnerDeletion != nil {
		t.Errorf("owner ref = %+v", o)
	}
}

func TestPodForIgnoresObjectExtras(t *testing.T) {
	t.Parallel()
	o := podObj()
	o.Metadata.Annotations = map[string]string{"x/y": "z"}
	o.Metadata.Labels = map[string]string{"evil": "1"}
	p, err := PodFor(o, podCfg(), podNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Labels["evil"]; ok || len(p.Annotations) != 0 {
		t.Errorf("object extras leaked: labels=%v annotations=%v", p.Labels, p.Annotations)
	}
}

func TestPodForNoTolerations(t *testing.T) {
	t.Parallel()
	p, err := PodFor(podObj(), podCfg(), podNow)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Spec.Tolerations) != 0 {
		t.Errorf("Tolerations = %v", p.Spec.Tolerations)
	}
}

func TestPodForNodeSelector(t *testing.T) {
	t.Parallel()
	p, err := PodFor(podObj(), podCfg(), podNow)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.NodeSelector["kubernetes.io/arch"] != "amd64" || len(p.Spec.NodeSelector) != 1 {
		t.Errorf("NodeSelector = %v", p.Spec.NodeSelector)
	}
	cfg := podCfg()
	cfg.Node = "node-a"
	p, err = PodFor(podObj(), cfg, podNow)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.NodeSelector["kubernetes.io/hostname"] != "node-a" || p.Spec.NodeSelector["kubernetes.io/arch"] != "amd64" {
		t.Errorf("NodeSelector = %v", p.Spec.NodeSelector)
	}
}
