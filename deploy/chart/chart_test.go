package chart

import (
	"os"
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
