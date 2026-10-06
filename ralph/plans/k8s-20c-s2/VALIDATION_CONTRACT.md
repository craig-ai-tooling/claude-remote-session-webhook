# Validation contract: k8s-20c-s2

Slice S2 of k8s-20c (spec 017, `kubernetes` mode, option B): the AgentSession API, admission,
the JSON manifests and the RBAC tests. FR-005, FR-007 (logic), FR-013, SC-004, SC-005.

Written before the plan. Each line below is what "done" means, checked against the built tree as
a black box. Run every command from the repository root on the milestone branch.

## Assertions

- The whole gate is green: `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./... && go test -tags quickstart ./cmd/crswd && golangci-lint run` exits 0.
- The host module still has no dependency: `test ! -e go.sum` exits 0 and `grep -c require go.mod` prints `0`.
- Host mode is untouched by the tests: `git diff --diff-filter=M --name-only origin/main...HEAD -- '*_test.go'` prints nothing, so no existing test was edited to make the slice pass.
- The committed manifests are exactly what the generator produces: `go run ./deploy/k8s/gen && git status --porcelain deploy/k8s` prints nothing.
- The resource is registered under the decided group: `jq -r '.metadata.name, .spec.group, .spec.scope, .spec.names.kind' deploy/k8s/crd.json` prints `agentsessions.crswd.craigcloud.io`, `crswd.craigcloud.io`, `Namespaced`, `AgentSession`, and `jq -r '.spec.versions[] | select(.name=="v1alpha1") | [.served, .storage, (.subresources.status != null)] | @csv' deploy/k8s/crd.json` prints `true,true,true`.
- The object cannot shape a pod or carry a token: `jq -c '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | keys' deploy/k8s/crd.json` prints `["conversation","lifetime","owner","sessionName","startCommand","workDir"]`, and `jq -c '.spec.versions[0].schema.openAPIV3Schema.properties.status.properties | keys' deploy/k8s/crd.json` prints `["conversation","phase","reason"]`.
- No RBAC rule anywhere under `deploy/k8s` grants anything on `secrets` or uses a wildcard: `jq -s '[.[] | .. | objects | select(has("rules")) | .rules[] | (.apiGroups[]?, .resources[]?, .verbs[]?) | select(. == "secrets" or . == "*")] | length' deploy/k8s/*.json` prints `0`.
- The daemon cannot create pods and holds `pods/exec` only in the session namespace: `jq -s '[.[] | .. | objects | select(.kind? == "Role") | select(.metadata.name == "crswd") | {ns: .metadata.namespace, pods: [.rules[] | select(.resources | index("pods")) | .verbs[]], exec: [.rules[] | select(.resources | index("pods/exec")) | .verbs[]]}]' deploy/k8s/*.json` shows exactly one Role, in namespace `crswd-next`, whose `pods` verbs do not include `create` and whose `exec` verbs are `create` and `get`; no Role named `crswd` exists in `crswd-next-reconciler`.
- The reconciler can create and delete pods in the session namespace and hold a Lease in its own, and nothing more: the same `jq -s` walk shows Role `crswd-reconciler` in `crswd-next` granting pods `create,get,list,watch,delete`, agentsessions `get,list,watch,update` and `agentsessions/status` `update`, and a Role in `crswd-next-reconciler` granting only `leases` in group `coordination.k8s.io` with `get,create,update`. Each Role has a RoleBinding naming its ServiceAccount (`crswd` in `crswd-next`, `crswd-reconciler` in `crswd-next-reconciler`).
- No manifest names the lawnmower namespace's Secrets or the VM's Claude home (SC-004): `grep -rniE 'lawnmower|\.claude' deploy/k8s/` prints nothing and exits 1.
- Admission rejects what spec 001's containment forbids, with a reason, and admits the rest: `go test -v -run Admit ./internal/admit` passes and its output names a case for each of outside the allowlist, a `..` escape, over the concurrency cap, past the lifetime, and one admitted object. On `main` the same command fails, because the package is absent there.
- A daemon told to check working directories lexically starts a session in a directory that does not exist on its own filesystem but is under an approved root, and still refuses one outside it; a daemon told nothing still refuses the non-existent directory: `go test -run 'WorkDirResolver|LexicalWorkDir' ./internal/session -v` passes and lists those three cases.
- The spec records the permissions the manifests grant: `grep -n 'leases' specs/017-k8s-native-execution/spec.md` and `grep -n 'agentsessions/status' specs/017-k8s-native-execution/spec.md` each print at least one line inside the FR-013 paragraph.
- `kubectl get` shows which runtime and phase: `jq -c '[.spec.versions[0].additionalPrinterColumns[] | .name]' deploy/k8s/crd.json` prints `["Start","Phase","Age"]` and `jq -c .spec.names.shortNames deploy/k8s/crd.json` prints `["as"]`.
