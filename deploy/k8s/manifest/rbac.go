package manifest

import "github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"

const rbacAPIVersion = "rbac.authorization.k8s.io/v1"

// ReconcilerRBAC is the reconciler's grant in the session namespace: it is the
// only principal that may create pods, and it writes status on the objects it
// admits or rejects.
func ReconcilerRBAC() map[string]any {
	return roleAndBinding(
		"crswd-reconciler", SessionNamespace,
		[]map[string]any{
			rule([]string{""}, []string{"pods"}, []string{"create", "get", "list", "watch", "delete"}),
			rule([]string{v1alpha1.Group}, []string{v1alpha1.Plural}, []string{"get", "list", "watch", "update"}),
			rule([]string{v1alpha1.Group}, []string{v1alpha1.Plural + "/status"}, []string{"update"}),
		},
		ReconcilerSA, ReconcilerNamespace,
	)
}

// LeaseRBAC lets the reconciler hold its leader-election Lease. It sits in the
// reconciler's own namespace so the session namespace never carries a grant
// that is not about sessions.
func LeaseRBAC() map[string]any {
	return roleAndBinding(
		"crswd-reconciler-lease", ReconcilerNamespace,
		[]map[string]any{
			rule([]string{"coordination.k8s.io"}, []string{"leases"}, []string{"get", "create", "update"}),
		},
		ReconcilerSA, ReconcilerNamespace,
	)
}

// DaemonRBAC is what the browser-facing daemon holds. It can write the
// AgentSession objects the reconciler admits but cannot create a pod, and it
// can exec only into pods in the session namespace.
func DaemonRBAC() map[string]any {
	return roleAndBinding(
		"crswd", SessionNamespace,
		[]map[string]any{
			rule([]string{v1alpha1.Group}, []string{v1alpha1.Plural}, []string{"get", "list", "watch", "create", "update", "patch", "delete"}),
			rule([]string{""}, []string{"pods"}, []string{"get", "list", "watch"}),
			rule([]string{""}, []string{"pods/exec"}, []string{"create", "get"}),
		},
		DaemonSA, SessionNamespace,
	)
}

func rule(groups, resources, verbs []string) map[string]any {
	return map[string]any{"apiGroups": groups, "resources": resources, "verbs": verbs}
}

// roleAndBinding returns a v1 List so kubectl applies the Role and its binding in
// one call. Rules are an ordered slice and the list is ordered Role first, so
// the output does not depend on map iteration.
func roleAndBinding(name, namespace string, rules []map[string]any, saName, saNamespace string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "List",
		"items": []any{
			map[string]any{
				"apiVersion": rbacAPIVersion,
				"kind":       "Role",
				"metadata":   map[string]any{"name": name, "namespace": namespace},
				"rules":      rules,
			},
			map[string]any{
				"apiVersion": rbacAPIVersion,
				"kind":       "RoleBinding",
				"metadata":   map[string]any{"name": name, "namespace": namespace},
				"roleRef":    map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": name},
				"subjects": []any{
					map[string]any{"kind": "ServiceAccount", "name": saName, "namespace": saNamespace},
				},
			},
		},
	}
}
