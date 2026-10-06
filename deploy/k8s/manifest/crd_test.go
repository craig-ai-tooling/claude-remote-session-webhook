package manifest

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
)

// jsonNames lists the JSON field names of a struct, so the CRD schema is
// compared with the Go type rather than with a second hand-typed list.
func jsonNames(t *testing.T, v any) []string {
	t.Helper()
	rt := reflect.TypeOf(v)
	var names []string
	for i := 0; i < rt.NumField(); i++ {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("json"), ",")
		if name == "" || name == "-" {
			t.Fatalf("field %s has no json name", rt.Field(i).Name)
		}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// decodedCRD round-trips the CRD through JSON so the test sees the bytes kubectl
// would apply, not the Go value that produced them.
func decodedCRD(t *testing.T) map[string]any {
	t.Helper()
	files, err := Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	raw, ok := files["crd.json"]
	if !ok {
		t.Fatal("Files() has no crd.json")
	}
	var crd map[string]any
	if err := json.Unmarshal(raw, &crd); err != nil {
		t.Fatalf("decode crd.json: %v", err)
	}
	return crd
}

// dig walks nested maps and slices by key or index.
func dig(t *testing.T, v any, path ...any) any {
	t.Helper()
	for _, p := range path {
		switch k := p.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("at %q: not an object", k)
			}
			if v, ok = m[k]; !ok {
				t.Fatalf("missing key %q", k)
			}
		case int:
			s, ok := v.([]any)
			if !ok || k >= len(s) {
				t.Fatalf("at [%d]: not a list of that length", k)
			}
			v = s[k]
		}
	}
	return v
}

func asList(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("%v is not a list", v)
	}
	return s
}

func asObject(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%v is not an object", v)
	}
	return m
}

func asStrings(t *testing.T, v any) []string {
	t.Helper()
	var out []string
	for _, e := range asList(t, v) {
		s, ok := e.(string)
		if !ok {
			t.Fatalf("%v is not a string", e)
		}
		out = append(out, s)
	}
	return out
}

func keysOf(t *testing.T, v any) []string {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatal("not an object")
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func TestCRDSchemaMatchesGoTypes(t *testing.T) {
	t.Parallel()
	crd := decodedCRD(t)
	schema := dig(t, crd, "spec", "versions", 0, "schema", "openAPIV3Schema", "properties")

	for _, tc := range []struct {
		name string
		prop string
		typ  any
	}{
		{"spec", "spec", v1alpha1.AgentSessionSpec{}},
		{"status", "status", v1alpha1.AgentSessionStatus{}},
	} {
		got := keysOf(t, dig(t, schema, tc.prop, "properties"))
		want := jsonNames(t, tc.typ)
		if !slices.Equal(got, want) {
			t.Errorf("%s schema properties = %v, Go type has %v", tc.name, got, want)
		}
	}
}

func TestCRDSpecRequiredIsEverythingButConversation(t *testing.T) {
	t.Parallel()
	crd := decodedCRD(t)
	raw := dig(t, crd, "spec", "versions", 0, "schema", "openAPIV3Schema", "properties", "spec", "required")
	got := asStrings(t, raw)
	slices.Sort(got)
	var want []string
	for _, n := range jsonNames(t, v1alpha1.AgentSessionSpec{}) {
		if n != "conversation" {
			want = append(want, n)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("required = %v, want %v", got, want)
	}
}

func TestCRDIdentity(t *testing.T) {
	t.Parallel()
	crd := decodedCRD(t)

	for _, tc := range []struct {
		path []any
		want any
	}{
		{[]any{"apiVersion"}, "apiextensions.k8s.io/v1"},
		{[]any{"kind"}, "CustomResourceDefinition"},
		{[]any{"metadata", "name"}, v1alpha1.Plural + "." + v1alpha1.Group},
		{[]any{"spec", "group"}, v1alpha1.Group},
		{[]any{"spec", "scope"}, "Namespaced"},
		{[]any{"spec", "names", "kind"}, v1alpha1.Kind},
		{[]any{"spec", "names", "listKind"}, v1alpha1.ListKind},
		{[]any{"spec", "names", "plural"}, v1alpha1.Plural},
		{[]any{"spec", "names", "singular"}, v1alpha1.Singular},
		{[]any{"spec", "versions", 0, "name"}, v1alpha1.Version},
		{[]any{"spec", "versions", 0, "served"}, true},
		{[]any{"spec", "versions", 0, "storage"}, true},
	} {
		if got := dig(t, crd, tc.path...); got != tc.want {
			t.Errorf("%v = %v, want %v", tc.path, got, tc.want)
		}
	}

	if n := len(asList(t, dig(t, crd, "spec", "versions"))); n != 1 {
		t.Errorf("versions = %d, want 1", n)
	}
	if got := dig(t, crd, "spec", "names", "shortNames"); !reflect.DeepEqual(got, []any{"as"}) {
		t.Errorf("shortNames = %v, want [as]", got)
	}
	if _, ok := asObject(t, dig(t, crd, "spec", "versions", 0, "subresources"))["status"]; !ok {
		t.Error("status subresource missing")
	}
}

func TestCRDPhaseEnumIsEveryPhase(t *testing.T) {
	t.Parallel()
	crd := decodedCRD(t)
	raw := dig(t, crd, "spec", "versions", 0, "schema", "openAPIV3Schema", "properties", "status", "properties", "phase", "enum")
	got := asStrings(t, raw)
	want := []string{
		string(v1alpha1.PhasePending), string(v1alpha1.PhaseRunning), string(v1alpha1.PhaseRejected),
		string(v1alpha1.PhaseReviving), string(v1alpha1.PhaseFailed),
	}
	if !slices.Equal(got, want) {
		t.Errorf("phase enum = %v, want %v", got, want)
	}
}

// The start column is how an operator sees which runtime a session runs; the
// object itself carries no runtime field (spec 017 FR-021).
func TestCRDPrinterColumns(t *testing.T) {
	t.Parallel()
	crd := decodedCRD(t)
	cols := asList(t, dig(t, crd, "spec", "versions", 0, "additionalPrinterColumns"))
	want := []struct{ name, path, typ string }{
		{"Start", ".spec.startCommand", "string"},
		{"Phase", ".status.phase", "string"},
		{"Age", ".metadata.creationTimestamp", "date"},
	}
	if len(cols) != len(want) {
		t.Fatalf("printer columns = %d, want %d", len(cols), len(want))
	}
	for i, w := range want {
		c := asObject(t, cols[i])
		if c["name"] != w.name || c["jsonPath"] != w.path || c["type"] != w.typ {
			t.Errorf("column %d = %v, want %+v", i, c, w)
		}
	}
}

// Admission runs only before a pod exists, so the fields it reads must not
// change afterwards. Conversation stays mutable: the daemon records it.
func TestCRDImmutableSpecFields(t *testing.T) {
	t.Parallel()
	crd := decodedCRD(t)
	rules := asList(t, dig(t, crd, "spec", "versions", 0, "schema", "openAPIV3Schema", "properties", "spec", "x-kubernetes-validations"))
	got := map[string]string{}
	for _, r := range rules {
		m := asObject(t, r)
		rule, okRule := m["rule"].(string)
		msg, okMsg := m["message"].(string)
		if !okRule || !okMsg {
			t.Fatalf("validation %v needs a string rule and message", m)
		}
		got[rule] = msg
	}
	for _, f := range []string{"sessionName", "owner", "workDir", "startCommand", "lifetime"} {
		rule := "self." + f + " == oldSelf." + f
		if msg, ok := got[rule]; !ok || msg != f+" is immutable" {
			t.Errorf("missing rule %q with message %q; have %v", rule, f+" is immutable", got)
		}
	}
	if len(got) != 5 {
		t.Errorf("rules = %d, want 5 (conversation must stay mutable): %v", len(got), got)
	}
}
