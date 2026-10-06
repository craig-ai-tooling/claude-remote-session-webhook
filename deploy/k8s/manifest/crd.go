package manifest

import "github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"

// CRD is the AgentSession CustomResourceDefinition. The schema is structural and
// lists exactly the fields in api/v1alpha1, so the API server prunes anything
// else: a client cannot smuggle a pod-shaping field into the stored object.
func CRD() map[string]any {
	str := map[string]any{"type": "string"}
	return map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]any{
			"name": v1alpha1.Plural + "." + v1alpha1.Group,
		},
		"spec": map[string]any{
			"group": v1alpha1.Group,
			"scope": "Namespaced",
			"names": map[string]any{
				"kind":       v1alpha1.Kind,
				"listKind":   v1alpha1.ListKind,
				"plural":     v1alpha1.Plural,
				"singular":   v1alpha1.Singular,
				"shortNames": []string{"as"},
			},
			"versions": []any{
				map[string]any{
					"name":    v1alpha1.Version,
					"served":  true,
					"storage": true,
					// The reconciler writes status through its own verb, so a
					// client with only update on the object cannot forge a phase.
					"subresources": map[string]any{"status": map[string]any{}},
					// Start is the configured start command key, which is how
					// kubectl shows the runtime (spec 017 FR-021) without a
					// runtime field on the object.
					"additionalPrinterColumns": []any{
						map[string]any{"name": "Start", "type": "string", "jsonPath": ".spec.startCommand"},
						map[string]any{"name": "Phase", "type": "string", "jsonPath": ".status.phase"},
						map[string]any{"name": "Age", "type": "date", "jsonPath": ".metadata.creationTimestamp"},
					},
					"schema": map[string]any{
						"openAPIV3Schema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"spec": map[string]any{
									"type": "object",
									// Admission runs only while no pod exists, so what it
									// read must not change under a running session.
									// Conversation stays mutable: the daemon records it.
									"x-kubernetes-validations": immutable(
										"sessionName", "owner", "workDir", "startCommand", "lifetime"),
									"required": []string{
										"sessionName", "owner", "workDir", "startCommand", "lifetime",
									},
									"properties": map[string]any{
										"sessionName":  str,
										"owner":        str,
										"workDir":      str,
										"startCommand": str,
										"conversation": str,
										"lifetime":     str,
									},
								},
								"status": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"phase": map[string]any{
											"type": "string",
											"enum": []string{
												string(v1alpha1.PhasePending),
												string(v1alpha1.PhaseRunning),
												string(v1alpha1.PhaseRejected),
												string(v1alpha1.PhaseReviving),
												string(v1alpha1.PhaseFailed),
											},
										},
										"reason":       str,
										"conversation": str,
										"podRecreates": map[string]any{"type": "integer", "minimum": 0},
										"recreateOf":   str,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// immutable is one transition rule per field, set on the spec object. A rule
// that mentions oldSelf is skipped on create, so it only bites on update.
func immutable(fields ...string) []any {
	rules := make([]any, 0, len(fields))
	for _, f := range fields {
		rules = append(rules, map[string]any{
			"rule":    "self." + f + " == oldSelf." + f,
			"message": f + " is immutable",
		})
	}
	return rules
}
