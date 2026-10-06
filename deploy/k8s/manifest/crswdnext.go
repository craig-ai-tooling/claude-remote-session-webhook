package manifest

// The images carry REPLACE where the built sha7 goes. The operator substitutes it
// and regenerates before applying; a test fails if a real tag is committed.
const (
	DaemonImage  = "docker.io/nctiggy/crswd:2.0.0-dev.REPLACE"
	SessionImage = "docker.io/nctiggy/crswd-session:2.1.246-codex0.153.4-REPLACE"
)

const (
	claimName        = "crswd-sessions"
	sharedSecretName = "crswd-shared-secret" //nolint:gosec // G101: a Secret's name, not its value
	workRoot         = "/work"
)

// crswdNextFiles renders the crswd-next objects keyed by file name under
// deploy/k8s. The S2 RBAC files are applied alongside, unchanged.
func crswdNextFiles() (map[string][]byte, error) {
	return render(crswdNextObjects())
}

func crswdNextObjects() map[string]any {
	return map[string]any{
		"crswd-next/namespaces.json": list(
			namespace(SessionNamespace),
			namespace(ReconcilerNamespace),
		),
		"crswd-next/serviceaccounts.json": list(
			serviceAccount(DaemonSA, SessionNamespace),
			serviceAccount(ReconcilerSA, ReconcilerNamespace),
		),
		"crswd-next/claim.json":                   sessionClaim(),
		"crswd-next/reconciler.json":              reconcilerDeployment(),
		"crswd-next/daemon.json":                  daemonDeployment(),
		"crswd-next/placeholder-credentials.json": placeholderCredentials(),
	}
}

func list(items ...any) map[string]any {
	return map[string]any{"apiVersion": "v1", "kind": "List", "items": items}
}

func objectLabels() map[string]any {
	return map[string]any{"app.kubernetes.io/part-of": "crswd"}
}

func namespace(name string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata":   map[string]any{"name": name, "labels": objectLabels()},
	}
}

// Both accounts call the API, so both automount their token.
func serviceAccount(name, ns string) map[string]any {
	return map[string]any{
		"apiVersion":                   "v1",
		"kind":                         "ServiceAccount",
		"metadata":                     map[string]any{"name": name, "namespace": ns, "labels": objectLabels()},
		"automountServiceAccountToken": true,
	}
}

func sessionClaim() map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata":   map[string]any{"name": claimName, "namespace": SessionNamespace, "labels": objectLabels()},
		"spec": map[string]any{
			"storageClassName": "linstor-replicated",
			"accessModes":      []string{"ReadWriteOnce"},
			"resources":        map[string]any{"requests": map[string]any{"storage": "50Gi"}},
		},
	}
}

// placeholderCredentials is empty so no pod is logged in by accident; the
// keeper fills it.
func placeholderCredentials() map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   map[string]any{"name": "claude-credentials", "namespace": SessionNamespace, "labels": objectLabels()},
		"type":       "Opaque",
	}
}

func envValue(name, value string) map[string]any {
	return map[string]any{"name": name, "value": value}
}

func deployment(name, ns, sa, component string, container map[string]any, volumes []any) map[string]any {
	selector := map[string]any{
		"app.kubernetes.io/name":      "crswd",
		"app.kubernetes.io/instance":  "crswd-next",
		"app.kubernetes.io/component": component,
	}
	podSpec := map[string]any{
		"serviceAccountName": sa,
		"containers":         []any{container},
		"tolerations": []any{
			map[string]any{"key": "drbd.linbit.com/lost-quorum", "operator": "Exists", "effect": "NoSchedule"},
		},
	}
	if volumes != nil {
		podSpec["volumes"] = volumes
	}
	return map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": name, "namespace": ns, "labels": mergeLabels(selector)},
		"spec": map[string]any{
			"replicas": 1,
			// Recreate: a second daemon or reconciler beside the first is never wanted.
			"strategy": map[string]any{"type": "Recreate"},
			"selector": map[string]any{"matchLabels": selector},
			"template": map[string]any{
				"metadata": map[string]any{"labels": selector},
				"spec":     podSpec,
			},
		},
	}
}

func mergeLabels(sel map[string]any) map[string]any {
	out := objectLabels()
	for k, v := range sel {
		out[k] = v
	}
	return out
}

func hardenedContext() map[string]any {
	return map[string]any{
		"runAsNonRoot":             true,
		"readOnlyRootFilesystem":   true,
		"allowPrivilegeEscalation": false,
		"capabilities":             map[string]any{"drop": []string{"ALL"}},
	}
}

func reconcilerDeployment() map[string]any {
	c := map[string]any{
		"name":            "reconciler",
		"image":           DaemonImage,
		"args":            []string{"reconcile"},
		"securityContext": hardenedContext(),
		"env": []any{
			envValue("CRSW_SESSION_NAMESPACE", SessionNamespace),
			envValue("CRSW_RECONCILER_NAMESPACE", ReconcilerNamespace),
			envValue("CRSW_SESSION_IMAGE", SessionImage),
			// The session image is amd64 only and the claim is RWO on one node (FR-008).
			envValue("CRSW_SESSION_NODE", "lm-amd64-1"),
			envValue("CRSW_SESSION_CLAIM", claimName),
			envValue("CRSW_MAX_SESSIONS", "10"),
			envValue("CRSW_CLAUDE_SECRET", "claude-credentials"),
			envValue("CRSW_CODEX_SECRET", "codex-auth"),
		},
	}
	return deployment("crswd-reconciler", ReconcilerNamespace, ReconcilerSA, "reconciler", c, nil)
}

func daemonDeployment() map[string]any {
	c := map[string]any{
		"name":            "daemon",
		"image":           DaemonImage,
		"securityContext": hardenedContext(),
		"volumeMounts":    []any{map[string]any{"name": "work", "mountPath": workRoot}},
		"env": []any{
			envValue("CRSW_EXECUTION_MODE", "kubernetes"),
			envValue("CRSW_SESSION_NAMESPACE", SessionNamespace),
			envValue("CRSW_LISTEN", "127.0.0.1:8765"),
			// config.Load needs every root to exist on the daemon's own filesystem;
			// the sessions' real /work is in their pods.
			envValue("CRSW_ALLOWED_ROOTS", workRoot),
			envValue("CRSW_MAX_SESSIONS", "10"),
			envValue("CRSW_START_COMMANDS", "default=claude --dangerously-skip-permissions,codex=/usr/local/bin/codex --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust"),
			map[string]any{
				"name": "CRSW_SHARED_SECRET",
				"valueFrom": map[string]any{
					"secretKeyRef": map[string]any{"name": sharedSecretName, "key": "secret"},
				},
			},
		},
	}
	volumes := []any{map[string]any{"name": "work", "emptyDir": map[string]any{}}}
	return deployment("crswd", SessionNamespace, DaemonSA, "daemon", c, volumes)
}
