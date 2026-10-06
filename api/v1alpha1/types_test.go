package v1alpha1

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// jsonNames returns the JSON property name of every field of t, so a test
// can compare the whole set against an allowlist.
func jsonNames(t reflect.Type) []string {
	var names []string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestSpecAndStatusFieldAllowlist(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		typ  reflect.Type
		want []string
	}{
		{"spec", reflect.TypeOf(AgentSessionSpec{}),
			[]string{"conversation", "lifetime", "owner", "sessionName", "startCommand", "workDir"}},
		{"status", reflect.TypeOf(AgentSessionStatus{}),
			[]string{"conversation", "phase", "reason"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := jsonNames(tc.typ)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s fields = %v, want exactly %v; a field on the spec is pod creation for anyone who can write the object", tc.name, got, tc.want)
			}
		})
	}
}

func TestNoTokenField(t *testing.T) {
	t.Parallel()
	for _, typ := range []reflect.Type{
		reflect.TypeOf(AgentSession{}),
		reflect.TypeOf(ObjectMeta{}),
		reflect.TypeOf(AgentSessionSpec{}),
		reflect.TypeOf(AgentSessionStatus{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if strings.Contains(strings.ToLower(f.Name), "token") ||
				strings.Contains(strings.ToLower(f.Tag.Get("json")), "token") {
				t.Errorf("%s.%s names a token; the object must never carry one", typ.Name(), f.Name)
			}
		}
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, got, want string }{
		{"group", Group, "crswd.craigcloud.io"},
		{"version", Version, "v1alpha1"},
		{"kind", Kind, "AgentSession"},
		{"listkind", ListKind, "AgentSessionList"},
		{"plural", Plural, "agentsessions"},
		{"singular", Singular, "agentsession"},
		{"apiversion", APIVersion, "crswd.craigcloud.io/v1alpha1"},
		{"pending", string(PhasePending), "Pending"},
		{"running", string(PhaseRunning), "Running"},
		{"rejected", string(PhaseRejected), "Rejected"},
		{"reviving", string(PhaseReviving), "Reviving"},
		{"failed", string(PhaseFailed), "Failed"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
