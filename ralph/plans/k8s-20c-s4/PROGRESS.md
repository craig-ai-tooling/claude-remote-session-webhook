# Progress: k8s-20c-s4

Notebook for the S4 slice of k8s-20c. Newest entry at the bottom.

## Iteration 0 (planning, 10/6/26)

Plan written by the operator session against main `a0b1e5f`. Facts checked on disk:

- Root module `github.com/nctiggy/claude-remote-session-webhook`, `go 1.23.0`, no `require`, no
  `go.sum`. `cmd/crswd/quickstart_test.go:2368` and `internal/release/readme_test.go:143` fail if a
  root `go.sum` appears. A `go.sum` under `k8s/` is allowed (`docs/security.md` ~line 403).
- CI already runs `go -C k8s mod download`, golangci-lint with `working-directory: k8s`, vet, test
  and build when `k8s/go.mod` exists (`.github/workflows/ci.yml` ~line 231). Nothing in
  `.github/workflows/` needs to change.
- `k8s.io/client-go v0.32.8` exists on the proxy. Local Go is 1.24.0; the module's `go` line
  must stay `1.23.0`.
- If `go build` fails on VCS stamping in the sandbox, the runner exports
  `GOFLAGS=-buildvcs=false`. A private `GOLANGCI_LINT_CACHE` is also exported; lint findings in
  paths outside this repo are cache noise, not yours.

## NEEDS CLARIFICATION

None open.

## Iteration 1 (T1)

BLOCKED: the sandbox refuses every way of running Go inside the nested module. `go -C k8s test ./...`, `(cd k8s && go test ./...)` and the piped form each stalled for approval, and this session is non-interactive. Written so far, uncommitted: `k8s/go.mod` and `k8s/internal/kube/kube_test.go`. Not done: `kube.go`, `go get`, `go mod tidy`, the failing-first run, the gate. Nothing was committed and T1 stays unticked. Needs `go -C k8s ...` and `go -C k8s get` allowed for the runner (or a pre-approved wrapper).
