// Package manifest holds the Kubernetes objects the next-generation deployment
// applies, as Go values. Files renders them to JSON so one generator writes the
// committed copies and a test proves the copies have not drifted. JSON, not
// YAML, because the host module has no dependencies and encoding/json is
// standard library.
package manifest

import (
	"encoding/json"
	"fmt"
)

// Namespaces and ServiceAccounts. The reconciler lives apart from the sessions
// it creates so that a session pod's namespace never holds the credential that
// can create more pods.
const (
	SessionNamespace    = "crswd-next"
	ReconcilerNamespace = "crswd-next-reconciler"
	DaemonSA            = "crswd"
	ReconcilerSA        = "crswd-reconciler"
)

// Files returns every manifest keyed by file name under deploy/k8s. Marshalling
// maps sorts their keys, which keeps the bytes stable between runs.
func Files() (map[string][]byte, error) {
	objects := map[string]any{
		"crd.json":             CRD(),
		"rbac-reconciler.json": ReconcilerRBAC(),
		"rbac-lease.json":      LeaseRBAC(),
		"rbac-daemon.json":     DaemonRBAC(),
	}
	files, err := render(objects)
	if err != nil {
		return nil, err
	}
	next, err := crswdNextFiles()
	if err != nil {
		return nil, err
	}
	for name, b := range next {
		files[name] = b
	}
	return files, nil
}

func render(objects map[string]any) (map[string][]byte, error) {
	files := make(map[string][]byte, len(objects))
	for name, v := range objects {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal %s: %w", name, err)
		}
		files[name] = append(b, '\n')
	}
	return files, nil
}
