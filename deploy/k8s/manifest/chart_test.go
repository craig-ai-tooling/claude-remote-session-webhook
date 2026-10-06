package manifest

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

const (
	chartCRDFile   = "crds/agentsessions.crswd.craigcloud.io.json"
	chartRulesFile = "files/rules.json"
)

func chartRules(t *testing.T) map[string][]policyRule {
	t.Helper()
	files, err := ChartFiles()
	if err != nil {
		t.Fatalf("ChartFiles: %v", err)
	}
	raw, ok := files[chartRulesFile]
	if !ok {
		t.Fatalf("ChartFiles() has no %s", chartRulesFile)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var rules map[string][]policyRule
	if err := dec.Decode(&rules); err != nil {
		t.Fatalf("decode %s: %v", chartRulesFile, err)
	}
	return rules
}

// asRoles wraps each rule set in a Role so the S2 walkers, which read Roles,
// judge the chart's rules by the same standard as the hand-applied ones.
func asRoles(rules map[string][]policyRule) []rbacObject {
	var objs []rbacObject
	for name, rs := range rules {
		o := rbacObject{Kind: "Role", Rules: rs}
		o.Metadata.Name = name
		objs = append(objs, o)
	}
	return objs
}

func TestChartFilesCarryCRDAndRules(t *testing.T) {
	t.Parallel()
	chart, err := ChartFiles()
	if err != nil {
		t.Fatalf("ChartFiles: %v", err)
	}
	files, err := Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if !bytes.Equal(chart[chartCRDFile], files["crd.json"]) {
		t.Errorf("%s is not byte-identical to crd.json", chartCRDFile)
	}
	if len(chart) != 2 {
		t.Errorf("ChartFiles() has %d entries, want the CRD and the rules only", len(chart))
	}
	rules := chartRules(t)
	for _, key := range []string{"daemon", "lease", "reconciler"} {
		if len(rules[key]) == 0 {
			t.Errorf("rules.json has no %q rules", key)
		}
	}
	if len(rules) != 3 {
		t.Errorf("rules.json has %d keys, want daemon, lease and reconciler", len(rules))
	}
}

// The chart's rules must be the rules the hand-applied RBAC carries, since both
// come from one set of Go values.
func TestChartRulesMatchTheRBACFiles(t *testing.T) {
	t.Parallel()
	rules := chartRules(t)
	for key, file := range map[string]string{
		"daemon":     "rbac-daemon.json",
		"lease":      "rbac-lease.json",
		"reconciler": "rbac-reconciler.json",
	} {
		role := decodedRBAC(t, file).Items[0]
		if role.Kind != "Role" {
			t.Fatalf("%s item 0 is %s, want Role", file, role.Kind)
		}
		a, err := json.Marshal(role.Rules)
		if err != nil {
			t.Fatalf("marshal %s rules: %v", file, err)
		}
		b, err := json.Marshal(rules[key])
		if err != nil {
			t.Fatalf("marshal chart %q rules: %v", key, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("chart %q rules differ from %s", key, file)
		}
	}
}

func TestChartRulesNoSecretsOrWildcards(t *testing.T) {
	t.Parallel()
	if v := secretViolations(asRoles(chartRules(t))); len(v) != 0 {
		t.Errorf("chart rules grant secrets or a wildcard: %v", v)
	}
}

func TestChartDaemonCannotCreatePods(t *testing.T) {
	t.Parallel()
	for _, r := range chartRules(t)["daemon"] {
		if has(r.Resources, "pods") && has(r.Verbs, "create") {
			t.Errorf("daemon rules create pods: %+v", r)
		}
	}
	creators := 0
	for _, r := range chartRules(t)["reconciler"] {
		if has(r.Resources, "pods") && slices.Contains(r.Verbs, "create") {
			creators++
		}
	}
	if creators != 1 {
		t.Errorf("reconciler rules grant pod create %d times, want 1", creators)
	}
}

func TestChartWalkersCanFail(t *testing.T) {
	t.Parallel()
	bad := map[string][]policyRule{"daemon": {{
		APIGroups: []string{""}, Resources: []string{"secrets", "pods"}, Verbs: []string{"get", "create"},
	}}}
	if len(secretViolations(asRoles(bad))) == 0 {
		t.Error("secretViolations accepted a rule that names secrets")
	}
}
