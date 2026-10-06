package manifest

import (
	"encoding/json"
	"strings"
	"testing"
)

type nextContainer struct {
	Image string   `json:"image"`
	Args  []string `json:"args"`
	Env   []struct {
		Name      string `json:"name"`
		Value     string `json:"value"`
		ValueFrom *struct {
			SecretKeyRef struct {
				Name string `json:"name"`
				Key  string `json:"key"`
			} `json:"secretKeyRef"`
		} `json:"valueFrom"`
	} `json:"env"`
	SecurityContext struct {
		RunAsNonRoot             bool  `json:"runAsNonRoot"`
		ReadOnlyRootFilesystem   bool  `json:"readOnlyRootFilesystem"`
		AllowPrivilegeEscalation *bool `json:"allowPrivilegeEscalation"`
		Capabilities             struct {
			Drop []string `json:"drop"`
		} `json:"capabilities"`
	} `json:"securityContext"`
	VolumeMounts []struct {
		Name      string `json:"name"`
		MountPath string `json:"mountPath"`
	} `json:"volumeMounts"`
}

type nextDeployment struct {
	Metadata struct {
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Labels    map[string]string `json:"labels"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
		Strategy struct {
			Type string `json:"type"`
		} `json:"strategy"`
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				ServiceAccountName string          `json:"serviceAccountName"`
				Containers         []nextContainer `json:"containers"`
				Volumes            []struct {
					Name     string          `json:"name"`
					EmptyDir *map[string]any `json:"emptyDir"`
				} `json:"volumes"`
				Tolerations []struct {
					Key      string `json:"key"`
					Operator string `json:"operator"`
					Effect   string `json:"effect"`
				} `json:"tolerations"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

func nextRaw(t *testing.T, name string) []byte {
	t.Helper()
	files, err := Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	b, ok := files["crswd-next/"+name]
	if !ok {
		t.Fatalf("Files() has no crswd-next/%s", name)
	}
	return b
}

func nextDeploy(t *testing.T, name string) nextDeployment {
	t.Helper()
	var d nextDeployment
	if err := json.Unmarshal(nextRaw(t, name), &d); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return d
}

func envOf(c nextContainer) map[string]string {
	m := map[string]string{}
	for _, e := range c.Env {
		m[e.Name] = e.Value
	}
	return m
}

func TestCrswdNextDeployments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		file, name, ns, sa, component string
	}{
		{"daemon.json", "crswd", SessionNamespace, DaemonSA, "daemon"},
		{"reconciler.json", "crswd-reconciler", ReconcilerNamespace, ReconcilerSA, "reconciler"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			raw := nextRaw(t, tc.file)
			d := nextDeploy(t, tc.file)
			if d.Metadata.Name != tc.name || d.Metadata.Namespace != tc.ns {
				t.Errorf("metadata = %+v, want %s in %s", d.Metadata, tc.name, tc.ns)
			}
			if d.Spec.Replicas != 1 || d.Spec.Strategy.Type != "Recreate" {
				t.Errorf("replicas %d strategy %q, want 1 Recreate", d.Spec.Replicas, d.Spec.Strategy.Type)
			}
			if d.Spec.Template.Spec.ServiceAccountName != tc.sa {
				t.Errorf("serviceAccountName = %q, want %q", d.Spec.Template.Spec.ServiceAccountName, tc.sa)
			}
			want := map[string]string{
				"app.kubernetes.io/name":      "crswd",
				"app.kubernetes.io/instance":  "crswd-next",
				"app.kubernetes.io/component": tc.component,
			}
			for k, v := range want {
				if d.Spec.Selector.MatchLabels[k] != v || d.Spec.Template.Metadata.Labels[k] != v {
					t.Errorf("label %s: selector %q, template %q, want %q", k, d.Spec.Selector.MatchLabels[k], d.Spec.Template.Metadata.Labels[k], v)
				}
			}
			if len(d.Spec.Selector.MatchLabels) != len(want) {
				t.Errorf("selector.matchLabels = %v, want exactly %v", d.Spec.Selector.MatchLabels, want)
			}
			if d.Metadata.Labels["app.kubernetes.io/part-of"] != "crswd" {
				t.Errorf("part-of label missing: %v", d.Metadata.Labels)
			}
			if len(d.Spec.Template.Spec.Containers) != 1 {
				t.Fatalf("%d containers, want 1", len(d.Spec.Template.Spec.Containers))
			}
			c := d.Spec.Template.Spec.Containers[0]
			if c.Image != DaemonImage {
				t.Errorf("image = %q, want DaemonImage", c.Image)
			}
			sc := c.SecurityContext
			if !sc.RunAsNonRoot || !sc.ReadOnlyRootFilesystem || sc.AllowPrivilegeEscalation != nil && *sc.AllowPrivilegeEscalation {
				t.Errorf("securityContext = %+v", sc)
			}
			if !strings.Contains(string(raw), `"allowPrivilegeEscalation": false`) {
				t.Error("allowPrivilegeEscalation is not set to false")
			}
			if len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
				t.Errorf("capabilities.drop = %v, want [ALL]", sc.Capabilities.Drop)
			}
			tol := d.Spec.Template.Spec.Tolerations
			if len(tol) != 1 || tol[0].Key != "drbd.linbit.com/lost-quorum" || tol[0].Operator != "Exists" || tol[0].Effect != "NoSchedule" {
				t.Errorf("tolerations = %+v", tol)
			}
			for _, banned := range []string{"hostPath", "privileged\": true", "hostNetwork", "." + "claude", "lawn" + "mower"} {
				if strings.Contains(string(raw), banned) {
					t.Errorf("%s contains %q", tc.file, banned)
				}
			}
		})
	}
}

func TestCrswdNextDaemonEnvAndVolume(t *testing.T) {
	t.Parallel()
	d := nextDeploy(t, "daemon.json")
	c := d.Spec.Template.Spec.Containers[0]
	if len(c.Args) != 0 {
		t.Errorf("daemon args = %v, want none", c.Args)
	}
	env := envOf(c)
	for k, v := range map[string]string{
		"CRSW_EXECUTION_MODE":    "kubernetes",
		"CRSW_SESSION_NAMESPACE": SessionNamespace,
		"CRSW_LISTEN":            "127.0.0.1:8765",
		"CRSW_ALLOWED_ROOTS":     "/work",
		"CRSW_MAX_SESSIONS":      "10",
	} {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
	if !strings.Contains(env["CRSW_START_COMMANDS"], "codex=") || !strings.Contains(env["CRSW_START_COMMANDS"], "default=") {
		t.Errorf("CRSW_START_COMMANDS = %q, want default= and codex= entries", env["CRSW_START_COMMANDS"])
	}
	for _, e := range c.Env {
		if e.ValueFrom == nil {
			continue
		}
		ref := e.ValueFrom.SecretKeyRef
		if e.Name != "CRSW_SHARED_SECRET" || ref.Name != "crswd-shared-secret" || ref.Key != "secret" {
			t.Errorf("env %s reads Secret %s/%s; only CRSW_SHARED_SECRET from crswd-shared-secret/secret is allowed", e.Name, ref.Name, ref.Key)
		}
	}
	if env["CRSW_SHARED_SECRET"] != "" {
		t.Error("CRSW_SHARED_SECRET is inlined")
	}
	var found bool
	for _, e := range c.Env {
		if e.Name == "CRSW_SHARED_SECRET" && e.ValueFrom != nil {
			found = true
		}
	}
	if !found {
		t.Error("CRSW_SHARED_SECRET is not read from a secretKeyRef")
	}
	var mount string
	for _, m := range c.VolumeMounts {
		if m.MountPath == "/work" {
			mount = m.Name
		}
	}
	if mount == "" {
		t.Fatal("no volume mounted at /work")
	}
	var ok bool
	for _, v := range d.Spec.Template.Spec.Volumes {
		if v.Name == mount && v.EmptyDir != nil {
			ok = true
		}
	}
	if !ok {
		t.Errorf("volume %q at /work is not an emptyDir", mount)
	}
}

func TestCrswdNextReconcilerEnv(t *testing.T) {
	t.Parallel()
	d := nextDeploy(t, "reconciler.json")
	c := d.Spec.Template.Spec.Containers[0]
	if len(c.Args) != 1 || c.Args[0] != "reconcile" {
		t.Errorf("args = %v, want [reconcile]", c.Args)
	}
	env := envOf(c)
	daemon := envOf(nextDeploy(t, "daemon.json").Spec.Template.Spec.Containers[0])
	if env["CRSW_SESSION_NAMESPACE"] != daemon["CRSW_SESSION_NAMESPACE"] {
		t.Errorf("reconciler namespace %q != daemon %q", env["CRSW_SESSION_NAMESPACE"], daemon["CRSW_SESSION_NAMESPACE"])
	}
	for k, v := range map[string]string{ //nolint:gosec // G101: Secret names, not credentials
		"CRSW_RECONCILER_NAMESPACE": ReconcilerNamespace,
		"CRSW_SESSION_IMAGE":        SessionImage,
		"CRSW_SESSION_NODE":         "lm-amd64-1",
		"CRSW_SESSION_CLAIM":        "crswd-sessions",
		"CRSW_MAX_SESSIONS":         "10",
		"CRSW_CLAUDE_SECRET":        "claude-credentials",
		"CRSW_CODEX_SECRET":         "codex-auth",
	} {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
}

func TestCrswdNextImagesArePlaceholders(t *testing.T) {
	t.Parallel()
	for name, img := range map[string]string{"DaemonImage": DaemonImage, "SessionImage": SessionImage} {
		if !strings.Contains(img, "REPLACE") {
			t.Errorf("%s = %q: a real tag must never be committed", name, img)
		}
	}
}

func TestCrswdNextClaimAndNamespaces(t *testing.T) {
	t.Parallel()
	var claim struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			StorageClassName string   `json:"storageClassName"`
			AccessModes      []string `json:"accessModes"`
			Resources        struct {
				Requests map[string]string `json:"requests"`
			} `json:"resources"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(nextRaw(t, "claim.json"), &claim); err != nil {
		t.Fatal(err)
	}
	if claim.Kind != "PersistentVolumeClaim" || claim.Metadata.Name != "crswd-sessions" || claim.Metadata.Namespace != SessionNamespace {
		t.Errorf("claim = %+v", claim)
	}
	if claim.Spec.StorageClassName != "linstor-replicated" || len(claim.Spec.AccessModes) != 1 || claim.Spec.AccessModes[0] != "ReadWriteOnce" || claim.Spec.Resources.Requests["storage"] != "50Gi" {
		t.Errorf("claim spec = %+v", claim.Spec)
	}

	for file, want := range map[string][]string{
		"namespaces.json":      {SessionNamespace, ReconcilerNamespace},
		"serviceaccounts.json": {DaemonSA, ReconcilerSA},
	} {
		var l struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Automount *bool `json:"automountServiceAccountToken"`
			} `json:"items"`
		}
		if err := json.Unmarshal(nextRaw(t, file), &l); err != nil {
			t.Fatal(err)
		}
		if len(l.Items) != len(want) {
			t.Fatalf("%s has %d items, want %d", file, len(l.Items), len(want))
		}
		for i, it := range l.Items {
			if it.Metadata.Name != want[i] {
				t.Errorf("%s item %d = %q, want %q", file, i, it.Metadata.Name, want[i])
			}
			if file == "serviceaccounts.json" && (it.Automount == nil || !*it.Automount) {
				t.Errorf("%s does not automount its token", it.Metadata.Name)
			}
		}
	}
}

func TestCrswdNextPlaceholderCredentialsHoldNoData(t *testing.T) {
	t.Parallel()
	var s struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Data       map[string]string `json:"data"`
		StringData map[string]string `json:"stringData"`
	}
	if err := json.Unmarshal(nextRaw(t, "placeholder-credentials.json"), &s); err != nil {
		t.Fatal(err)
	}
	if s.Kind != "Secret" || s.Metadata.Name != "claude-credentials" || s.Metadata.Namespace != SessionNamespace {
		t.Errorf("secret = %+v", s)
	}
	if len(s.Data) != 0 || len(s.StringData) != 0 {
		t.Error("the placeholder Secret holds data")
	}
}
