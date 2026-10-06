package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

var rbacFiles = []string{"rbac-reconciler.json", "rbac-lease.json", "rbac-daemon.json"}

type policyRule struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
	Verbs     []string `json:"verbs"`
}

type subject struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type rbacObject struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Rules    []policyRule `json:"rules"`
	Subjects []subject    `json:"subjects"`
	RoleRef  struct {
		APIGroup string `json:"apiGroup"`
		Kind     string `json:"kind"`
		Name     string `json:"name"`
	} `json:"roleRef"`
}

type rbacList struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Items      []rbacObject `json:"items"`
}

// decodedRBAC decodes the bytes Files() produces, with unknown fields refused,
// so a misspelled key in a manifest fails here instead of being applied and
// silently ignored by the API server.
func decodedRBAC(t *testing.T, name string) rbacList {
	t.Helper()
	files, err := Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	raw, ok := files[name]
	if !ok {
		t.Fatalf("Files() has no %s", name)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var l rbacList
	if err := dec.Decode(&l); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return l
}

func allRBAC(t *testing.T) []rbacObject {
	t.Helper()
	var all []rbacObject
	for _, name := range rbacFiles {
		all = append(all, decodedRBAC(t, name).Items...)
	}
	return all
}

func has(list []string, want string) bool { return slices.Contains(list, want) }

// secretViolations reports every rule that names secrets or a wildcard in any
// of its three lists. A wildcard is as good as naming secrets.
func secretViolations(objs []rbacObject) []string {
	var out []string
	for _, o := range objs {
		for i, r := range o.Rules {
			for _, field := range []struct {
				what string
				vals []string
			}{{"apiGroups", r.APIGroups}, {"resources", r.Resources}, {"verbs", r.Verbs}} {
				for _, bad := range []string{"secrets", "*"} {
					if has(field.vals, bad) {
						out = append(out, fmt.Sprintf("%s %q rule %d: %s contains %q", o.Kind, o.Metadata.Name, i, field.what, bad))
					}
				}
			}
		}
	}
	return out
}

// rolesBoundTo returns the Roles a RoleBinding in objs grants to the named
// ServiceAccount, matched by namespace and name the way the API server does.
func rolesBoundTo(objs []rbacObject, sa string) []rbacObject {
	var out []rbacObject
	for _, b := range objs {
		if b.Kind != "RoleBinding" {
			continue
		}
		bound := false
		for _, s := range b.Subjects {
			if s.Kind == "ServiceAccount" && s.Name == sa {
				bound = true
			}
		}
		if !bound {
			continue
		}
		for _, r := range objs {
			if r.Kind == "Role" && r.Metadata.Name == b.RoleRef.Name && r.Metadata.Namespace == b.Metadata.Namespace {
				out = append(out, r)
			}
		}
	}
	return out
}

// daemonViolations checks what the browser-facing daemon's ServiceAccount may
// never hold: pod creation, a grant in the reconciler's namespace, and exec
// anywhere but the session namespace.
func daemonViolations(objs []rbacObject) []string {
	var out []string
	for _, role := range rolesBoundTo(objs, DaemonSA) {
		if role.Metadata.Namespace == ReconcilerNamespace {
			out = append(out, fmt.Sprintf("role %q is in the reconciler namespace", role.Metadata.Name))
		}
		for i, r := range role.Rules {
			if has(r.Resources, "pods") && has(r.Verbs, "create") {
				out = append(out, fmt.Sprintf("role %q rule %d: create on pods", role.Metadata.Name, i))
			}
			if has(r.Resources, "pods/exec") && role.Metadata.Namespace != SessionNamespace {
				out = append(out, fmt.Sprintf("role %q rule %d: pods/exec in %q", role.Metadata.Name, i, role.Metadata.Namespace))
			}
		}
	}
	return out
}

func TestRBACFilesAreNamespacedLists(t *testing.T) {
	t.Parallel()
	for _, name := range rbacFiles {
		l := decodedRBAC(t, name)
		if l.APIVersion != "v1" || l.Kind != "List" {
			t.Errorf("%s is %s %s, want v1 List", name, l.APIVersion, l.Kind)
		}
		if len(l.Items) != 2 || l.Items[0].Kind != "Role" || l.Items[1].Kind != "RoleBinding" {
			t.Errorf("%s items are not [Role, RoleBinding]", name)
			continue
		}
		for _, o := range l.Items {
			if o.APIVersion != "rbac.authorization.k8s.io/v1" {
				t.Errorf("%s %s apiVersion = %q", name, o.Kind, o.APIVersion)
			}
			if o.Metadata.Namespace == "" {
				t.Errorf("%s %s %q has no namespace", name, o.Kind, o.Metadata.Name)
			}
		}
		ref := l.Items[1].RoleRef
		if ref.Kind != "Role" || ref.APIGroup != "rbac.authorization.k8s.io" || ref.Name != l.Items[0].Metadata.Name {
			t.Errorf("%s binding roleRef = %+v, does not point at its Role", name, ref)
		}
	}
}

func TestRBACNoSecretsOrWildcards(t *testing.T) {
	t.Parallel()
	for _, v := range secretViolations(allRBAC(t)) {
		t.Error(v)
	}
}

// The walker is only worth something if it can fail.
func TestRBACWalkersCanFail(t *testing.T) {
	t.Parallel()
	var bad rbacObject
	bad.Kind = "Role"
	bad.Metadata.Name = "bad"
	bad.Metadata.Namespace = ReconcilerNamespace
	bad.Rules = []policyRule{
		{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}},
		{APIGroups: []string{"*"}, Resources: []string{"pods"}, Verbs: []string{"get"}},
		{APIGroups: []string{""}, Resources: []string{"pods", "pods/exec"}, Verbs: []string{"create"}},
	}
	var bind rbacObject
	bind.Kind = "RoleBinding"
	bind.Metadata.Name = "bad"
	bind.Metadata.Namespace = ReconcilerNamespace
	bind.RoleRef.Name = "bad"
	bind.Subjects = []subject{{Kind: "ServiceAccount", Name: DaemonSA, Namespace: SessionNamespace}}
	objs := []rbacObject{bad, bind}

	if got := secretViolations(objs); len(got) != 2 {
		t.Errorf("secretViolations = %v, want the secrets rule and the wildcard", got)
	}
	got := strings.Join(daemonViolations(objs), "\n")
	for _, want := range []string{"reconciler namespace", "create on pods", "pods/exec"} {
		if !strings.Contains(got, want) {
			t.Errorf("daemonViolations = %q, missing %q", got, want)
		}
	}
}

func TestRBACDaemonIsBounded(t *testing.T) {
	t.Parallel()
	objs := allRBAC(t)
	if len(rolesBoundTo(objs, DaemonSA)) == 0 {
		t.Fatal("no Role is bound to the daemon ServiceAccount, so the checks below prove nothing")
	}
	for _, v := range daemonViolations(objs) {
		t.Error(v)
	}
}

func TestRBACDaemonRules(t *testing.T) {
	t.Parallel()
	l := decodedRBAC(t, "rbac-daemon.json")
	want := []policyRule{
		{[]string{"crswd.craigcloud.io"}, []string{"agentsessions"}, []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{[]string{""}, []string{"pods"}, []string{"get", "list", "watch"}},
		{[]string{""}, []string{"pods/exec"}, []string{"create", "get"}},
	}
	if !reflect.DeepEqual(l.Items[0].Rules, want) {
		t.Errorf("daemon rules = %+v, want %+v", l.Items[0].Rules, want)
	}
	if ns := l.Items[0].Metadata.Namespace; ns != SessionNamespace {
		t.Errorf("daemon Role namespace = %q, want %q", ns, SessionNamespace)
	}
	if got := l.Items[1].Subjects; !reflect.DeepEqual(got, []subject{{"ServiceAccount", DaemonSA, SessionNamespace}}) {
		t.Errorf("daemon subjects = %+v", got)
	}
}

func TestRBACReconcilerRulesExact(t *testing.T) {
	t.Parallel()
	group := []string{"crswd.craigcloud.io"}
	cases := []struct {
		file      string
		role      string
		namespace string
		rules     []policyRule
	}{
		{
			"rbac-reconciler.json", "crswd-reconciler", SessionNamespace,
			[]policyRule{
				{[]string{""}, []string{"pods"}, []string{"create", "get", "list", "watch", "delete"}},
				{group, []string{"agentsessions"}, []string{"get", "list", "watch", "update"}},
				{group, []string{"agentsessions/status"}, []string{"update"}},
			},
		},
		{
			"rbac-lease.json", "crswd-reconciler-lease", ReconcilerNamespace,
			[]policyRule{
				{[]string{"coordination.k8s.io"}, []string{"leases"}, []string{"get", "create", "update"}},
			},
		},
	}
	for _, tc := range cases {
		l := decodedRBAC(t, tc.file)
		role, binding := l.Items[0], l.Items[1]
		if role.Metadata.Name != tc.role || role.Metadata.Namespace != tc.namespace {
			t.Errorf("%s Role = %s/%s, want %s/%s", tc.file, role.Metadata.Namespace, role.Metadata.Name, tc.namespace, tc.role)
		}
		if !reflect.DeepEqual(role.Rules, tc.rules) {
			t.Errorf("%s rules = %+v, want %+v", tc.file, role.Rules, tc.rules)
		}
		if binding.Metadata.Namespace != tc.namespace {
			t.Errorf("%s binding namespace = %q, want %q", tc.file, binding.Metadata.Namespace, tc.namespace)
		}
		if want := []subject{{"ServiceAccount", ReconcilerSA, ReconcilerNamespace}}; !reflect.DeepEqual(binding.Subjects, want) {
			t.Errorf("%s subjects = %+v, want %+v", tc.file, binding.Subjects, want)
		}
	}
}

// The reconciler may create pods and the daemon may not; a swap of the two
// ServiceAccounts in a binding would pass every exact-rules check on its own.
func TestRBACOnlyReconcilerCreatesPods(t *testing.T) {
	t.Parallel()
	objs := allRBAC(t)
	if len(rolesBoundTo(objs, ReconcilerSA)) != 2 {
		t.Errorf("reconciler ServiceAccount is bound to %d Roles, want 2", len(rolesBoundTo(objs, ReconcilerSA)))
	}
	for _, role := range rolesBoundTo(objs, DaemonSA) {
		for _, r := range role.Rules {
			if has(r.Resources, "pods") && has(r.Verbs, "delete") {
				t.Errorf("daemon role %q can delete pods", role.Metadata.Name)
			}
		}
	}
}
