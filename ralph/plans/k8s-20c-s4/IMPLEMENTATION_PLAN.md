# Implementation plan: k8s-20c-s4

S4 of k8s-20c: the Kubernetes client foundation, a nested Go module in `k8s/` (spec 017,
`specs/017-k8s-native-execution/k8s-20c-plan.md` section 2 "S4", decision 1 option A). The
definition of done is `ralph/plans/k8s-20c-s4/VALIDATION_CONTRACT.md`.

The root module stays exactly as it is: `go.mod` with no `require`, no root `go.sum`. Only
`k8s/go.mod` and `k8s/go.sum` may carry dependencies (`docs/security.md`, "The rule covers the
host module").

## Dependency

None. This slice does not need S2 or S3 code.

## Tasks

Take the topmost open task. One per iteration.

- [x] T1: Create the nested module and its first package per Design §1 and §2: `k8s/go.mod`, `k8s/go.sum`, `k8s/internal/kube/kube.go`, `k8s/internal/kube/kube_test.go`. Verify: `go -C k8s vet ./... && go -C k8s test ./...` exits 0, and `test ! -e go.sum` exits 0 at the repo root.
- [x] T2: Add the `/k8s` gomod entry to `.github/dependabot.yml` per Design §3. Verify: `python3 -c "import yaml" 2>/dev/null` is not required; `grep -n 'directory: "/k8s"' .github/dependabot.yml` prints one line.
- [ ] T3: Add `k8s/internal/kube/boundary_test.go` per Design §4. Verify: `go -C k8s test ./internal/kube -run Boundary -v` passes.
- [ ] T4: Run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md, then append `RALPH_COMPLETE`. Verify: `go -C k8s test ./... && go test ./...` exits 0.

## Files touched

A diff outside this list is rejected.

- `k8s/go.mod`
- `k8s/go.sum`
- `k8s/internal/kube/kube.go`
- `k8s/internal/kube/kube_test.go`
- `k8s/internal/kube/boundary_test.go`
- `.github/dependabot.yml`
- `ralph/plans/k8s-20c-s4/`

Never touch: the root `go.mod`, any root `go.sum`, `AGENTS.md`, `docs/security.md`,
`.github/workflows/`, `.claude/`, anything outside `k8s/` except `.github/dependabot.yml`.

## Design

Decided in the source plan. Do not reopen these. If one cannot be built as written, log it in
PROGRESS.md under NEEDS CLARIFICATION and stop that task.

### §1 The module

Build it with these commands, from the repo root, in this order:

1. `mkdir -p k8s/internal/kube`
2. Write `k8s/go.mod` by hand with exactly these lines:
   ```
   module github.com/nctiggy/claude-remote-session-webhook/k8s

   go 1.23.0

   require github.com/nctiggy/claude-remote-session-webhook v0.0.0

   replace github.com/nctiggy/claude-remote-session-webhook => ../
   ```
3. Write `kube.go` (§2) first, so the imports exist.
4. `go -C k8s get k8s.io/client-go@v0.32.8 k8s.io/api@v0.32.8 k8s.io/apimachinery@v0.32.8`
5. `go -C k8s get github.com/moby/spdystream@v0.5.1 golang.org/x/oauth2@v0.27.0 github.com/gorilla/websocket@v1.5.3`
6. `go -C k8s mod tidy`
7. Confirm `grep -n '^go ' k8s/go.mod` prints `go 1.23.0` and `grep -c '^toolchain' k8s/go.mod` prints `0`. If `go get` raised the `go` line or added a `toolchain` line, a pin pulled a newer requirement: log the module that did it under NEEDS CLARIFICATION and stop. Do not hand-edit the line back.
8. The root `require` on the root module is needed only once a package under `k8s/` imports one. If `go mod tidy` drops it, that is correct; leave it dropped.

Use no other dependency. In particular no `sigs.k8s.io/controller-runtime` (source plan E5).

### §2 `k8s/internal/kube/kube.go` (package `kube`)

Package doc: the cluster-only helpers shared by the daemon and the reconciler in kubernetes mode
(spec 017). Comments explain why, per `docs/conventions.md`.

- `func InCluster() (*rest.Config, error)`: returns `rest.InClusterConfig()`, wrapping an error
  with `fmt.Errorf("kube: in-cluster config: %w", err)`.
- `type ElectorConfig struct { Namespace, Name, Identity string; LeaseDuration, RenewDeadline, RetryPeriod time.Duration }`.
- `func NewElector(client kubernetes.Interface, c ElectorConfig, onStart func(context.Context), onStop func()) (*leaderelection.LeaderElector, error)`:
  a `resourcelock.LeaseLock` on `c.Namespace`/`c.Name` with `Identity: c.Identity`, and
  `leaderelection.NewLeaderElector` with `ReleaseOnCancel: true` and those durations. Empty
  `Namespace`, `Name` or `Identity` returns an error before touching the client. Zero durations
  take defaults `15s`, `10s`, `2s`.

### §2a Tests (`kube_test.go`)

- `TestNewElectorRefusesEmptyFields`: table over the three empty fields, each returns an error.
- `TestExactlyOneLeads`: one `fake.NewSimpleClientset()`, two electors on the same lease with
  durations `LeaseDuration 2s, RenewDeadline 1s, RetryPeriod 200ms`, each `Run` in its own
  goroutine under one context. `onStart` increments an `atomic.Int32`. After 3 s, assert the
  count is exactly 1 and the Lease object exists in the namespace. Cancel, then wait for both
  `Run` calls to return.
- Write these first. They fail before §2 exists because the package does not compile; record that
  in PROGRESS.md.

### §3 `.github/dependabot.yml`

Append a second `gomod` entry after the existing one, same shape, with `directory: "/k8s"`, a
comment saying the cluster module carries client-go by design (spec 017, decision 1 option A),
and `open-pull-requests-limit: 5`. Do not change the root entry. Dependabot fails on a
directory that does not exist, which is why this lands in the same PR as `k8s/go.mod`.

### §4 `boundary_test.go`

`TestBoundaryRootImportsNothingFromK8s`: walk the repo root (`../../..` from the package dir),
skipping `k8s/`, `.git/`, `.claude/` and any directory named `testdata`. For every `*.go` file,
parse imports with `go/parser` (`parser.ImportsOnly`) and fail if any import path starts with
`github.com/nctiggy/claude-remote-session-webhook/k8s`. Assert at least 50 Go files were read, so
an empty walk cannot pass. Also fail if `../../../go.sum` exists. Prove it can fail: add a
temporary import in a scratch copy, never in the repo. A table case that runs the import check
over a synthetic in-memory source string is enough.
