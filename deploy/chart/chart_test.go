package chart

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readTemplate(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("templates", name)) //nolint:gosec // G304: name comes from a listing of this chart's own templates directory.
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}
	return string(b)
}

// The RBAC rules are generated Go values written to files/rules.json. A rule
// typed into a template would be a second copy that no drift test compares.
func TestChartRBACReadsGeneratedRules(t *testing.T) {
	t.Parallel()
	rbac := readTemplate(t, "rbac.yaml")

	for _, key := range []string{"daemon", "lease", "reconciler"} {
		re := regexp.MustCompile(`\.Files\.Get "files/rules\.json"[^}]*\)\.` + key + `\b`)
		if !re.MatchString(rbac) {
			t.Errorf("rbac.yaml has no .Files.Get \"files/rules.json\" expression for %q", key)
		}
	}
	if strings.Contains(rbac, "verbs:") {
		t.Error("rbac.yaml contains a literal verbs: a rule was written by hand")
	}

	entries, err := os.ReadDir("templates")
	if err != nil {
		t.Fatalf("read templates: %v", err)
	}
	for _, e := range entries {
		text := readTemplate(t, e.Name())
		for _, banned := range []string{"ClusterRole", "secrets"} {
			if strings.Contains(text, banned) {
				t.Errorf("templates/%s contains %q", e.Name(), banned)
			}
		}
	}
}

// Both Deployments run as the manifests S7 tested: no root, no writable image
// layer, no privilege, and nothing shared with the node.
func TestChartWorkloadsAreLockedDown(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"daemon.yaml", "reconciler.yaml"} {
		text := readTemplate(t, name)
		for _, want := range []string{
			"readOnlyRootFilesystem: true",
			"runAsNonRoot: true",
			"allowPrivilegeEscalation: false",
			"serviceAccountName:",
			"crswd.selectorLabels",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not contain %q", name, want)
			}
		}
		for _, banned := range []string{"hostPath", "privileged: true", "hostNetwork"} {
			if strings.Contains(text, banned) {
				t.Errorf("%s contains %q", name, banned)
			}
		}
	}
}

// FR-021: a fresh install offers Claude Code and Codex without an override.
func TestChartOffersBothRuntimes(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("values.yaml")
	if err != nil {
		t.Fatalf("read values.yaml: %v", err)
	}
	var line string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "startCommands:") {
			line = l
		}
	}
	if line == "" {
		t.Fatal("values.yaml has no startCommands line")
	}
	for _, want := range []string{"default=claude", "codex="} {
		if !strings.Contains(line, want) {
			t.Errorf("startCommands does not contain %q", want)
		}
	}
}

// helmTemplate renders the chart from this directory in namespace crswd with
// the values that make it renderable, plus extra. It skips when helm is not
// installed; CI's chart step renders the same chart with the helm it sets up.
func helmTemplate(t *testing.T, extra ...string) (string, error) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm is not installed")
	}
	args := []string{"template", "t", ".", "-n", "crswd", "--set", "sharedSecret.existingSecret=s", "--set", "sessionNode=node-a"}
	args = append(args, extra...)
	out, err := exec.Command("helm", args...).CombinedOutput() //nolint:gosec // G204: fixed binary, arguments written in this file.
	return string(out), err
}

func mustFail(t *testing.T, want string, extra ...string) {
	t.Helper()
	out, err := helmTemplate(t, extra...)
	if err == nil {
		t.Fatalf("render succeeded, want a failure mentioning %q", want)
	}
	if !strings.Contains(out, want) {
		t.Errorf("failure does not mention %q:\n%s", want, out)
	}
}

// FR-009 and FR-013: the daemon holds pods/exec in the release namespace, the
// reconciler creates pods in its own. Equal namespaces give the daemon exec on
// the reconciler's pods.
func TestChartRefusesReconcilerInReleaseNamespace(t *testing.T) {
	t.Parallel()
	mustFail(t, "reconciler.namespace", "--set", "reconciler.namespace=crswd")
	if out, err := helmTemplate(t); err != nil {
		t.Fatalf("default render failed: %v\n%s", err, out)
	}
}

// FR-010: every session pod runs on the claim's node, so an empty sessionNode
// must not render.
func TestChartRequiresSessionNode(t *testing.T) {
	t.Parallel()
	mustFail(t, "sessionNode", "--set", "sessionNode=")
}

// config.IsSecret classifies shared_secret, access_allowed_emails and
// dashboard_password. None of them may be a literal in a pod spec.
func TestChartSecretsComeFromSecrets(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"daemon.yaml", "reconciler.yaml"} {
		text := readTemplate(t, name)
		for _, v := range []string{"CRSW_SHARED_SECRET", "CRSW_DASHBOARD_PASSWORD", "CRSW_ACCESS_ALLOWED_EMAILS"} {
			i := strings.Index(text, "name: "+v)
			if i < 0 {
				continue
			}
			rest := text[i:]
			if j := strings.Index(rest[1:], "- name:"); j >= 0 {
				rest = rest[:j+1]
			}
			if !strings.Contains(rest, "secretKeyRef") {
				t.Errorf("%s renders %s without a secretKeyRef", name, v)
			}
		}
	}
	b, err := os.ReadFile("values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?m)^\s+allowedEmails:`).Match(b) {
		t.Error("values.yaml still has a literal access.allowedEmails")
	}

	out, err := helmTemplate(t, "--set", "access.teamDomain=x.cloudflareaccess.com", "--set", "access.aud=a",
		"--set", "access.allowedEmailsSecret.name=emails", "--set", "access.allowedEmailsSecret.key=list")
	if err != nil {
		t.Fatalf("render with the emails Secret failed: %v\n%s", err, out)
	}
	re := regexp.MustCompile(`(?s)name: CRSW_ACCESS_ALLOWED_EMAILS\s+valueFrom:\s+secretKeyRef:\s+name: "emails"\s+key: "list"`)
	if !re.MatchString(out) {
		t.Errorf("CRSW_ACCESS_ALLOWED_EMAILS is not a secretKeyRef to emails/list:\n%s", out)
	}
	mustFail(t, "access.allowedEmailsSecret", "--set", "access.teamDomain=x.cloudflareaccess.com", "--set", "access.aud=a")
}

// The sidecar image is pinned by digest; the tag stays in a comment.
func TestChartCloudflaredIsDigestPinned(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("values.yaml")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^\s+image: \S+@sha256:[0-9a-f]{64} #.*\d+\.\d+\.\d+`)
	if !re.Match(b) {
		t.Error("cloudflared.image is not image@sha256:<digest> with the tag in a comment")
	}
}
