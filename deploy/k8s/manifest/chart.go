package manifest

import (
	"errors"
	"fmt"
)

// ChartFiles returns the Helm chart's generated files, keyed by path relative to
// deploy/chart. The CRD is the same bytes as deploy/k8s/crd.json. rules.json
// holds the three rule sets with no namespace, so the chart's templates place
// them in whatever namespaces the release uses.
func ChartFiles() (map[string][]byte, error) {
	crd, err := render(map[string]any{"crds/agentsessions.crswd.craigcloud.io.json": CRD()})
	if err != nil {
		return nil, fmt.Errorf("render chart CRD: %w", err)
	}
	sets := map[string]any{}
	for key, list := range map[string]map[string]any{
		"daemon":     DaemonRBAC(),
		"lease":      LeaseRBAC(),
		"reconciler": ReconcilerRBAC(),
	} {
		rules, err := rulesOf(list)
		if err != nil {
			return nil, fmt.Errorf("rules for %s: %w", key, err)
		}
		sets[key] = rules
	}
	rules, err := render(map[string]any{"files/rules.json": sets})
	if err != nil {
		return nil, fmt.Errorf("render chart rules: %w", err)
	}
	for name, b := range rules {
		crd[name] = b
	}
	return crd, nil
}

// rulesOf takes the rules out of the Role a builder in rbac.go returns, so the
// chart and the hand-applied manifests cannot name different verbs. The Role is
// always item 0 of the List roleAndBinding builds.
func rulesOf(list map[string]any) ([]map[string]any, error) {
	items, ok := list["items"].([]any)
	if !ok || len(items) == 0 {
		return nil, errors.New("no items in the RBAC list")
	}
	role, ok := items[0].(map[string]any)
	if !ok {
		return nil, errors.New("item 0 is not an object")
	}
	rules, ok := role["rules"].([]map[string]any)
	if !ok {
		return nil, errors.New("item 0 has no rules")
	}
	return rules, nil
}
