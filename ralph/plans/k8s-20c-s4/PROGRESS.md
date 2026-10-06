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

## Iteration 2 (T1)

BLOCKED again, same cause. `go -C k8s test ./...` and even `go -C k8s version` return "This command requires approval" in this non-interactive session, and compound commands are refused outright. `k8s/go.mod` and `k8s/internal/kube/kube_test.go` are already committed (9521216, sweep). No code written this iteration: without running Go I cannot do the failing-first run, `go get` or the gate, and committing unverified code would break the rule against a broken tree. T1 stays unticked.

Unblock: add `Bash(go -C k8s:*)` (and `go -C k8s get`, `go -C k8s mod tidy`) to the runner's allowlist, then rerun. Next iteration, in order: write `kube.go` per Design §2, run the two tests (expect pass now that the package compiles; the pre-change failure is the compile error from kube_test.go alone), `go -C k8s get` the pins, `go -C k8s mod tidy`, check the `go 1.23.0` line, run the gate.

## Operator (10/6/26)

Unblocked: `.claude/settings.json` now allows `go -C k8s:*`, `go get:*` and `go list:*`. Skip linting inside `k8s/` (PROMPT step 4 of the cluster gate); the operator runs it. Start T1 from the committed `k8s/go.mod` and `kube_test.go`.

## Iteration 3 (T1)

Wrote `k8s/internal/kube/kube.go` (InCluster, ElectorConfig, NewElector), ran the `go get` pins and `go mod tidy`. `go 1.23.0` held, no `toolchain` line. Tidy dropped the root `require` from `k8s/go.mod` (nothing under `k8s/` imports the root yet), as Design §1 step 8 allows; the `replace` line stays.

Failing first: `go -C k8s test ./internal/kube` before the change: `no required module provides package k8s.io/apimachinery/pkg/apis/meta/v1 ... [setup failed]`. After: `TestNewElectorRefusesEmptyFields` and `TestExactlyOneLeads` pass (3.0s).

Gate: `go -C k8s vet/test/build` pass. Root `go build`, `go vet`, `go test`, `-tags tmux`, `-tags quickstart ./cmd/crswd` (127.0.0.1:8765 was free) and `golangci-lint run` (0 issues) pass. `test ! -e go.sum` ok, `grep -c require go.mod` prints 0. `k8s/` was not linted, per the operator note.

Rediscovery traps: root `golangci-lint run` refuses with "parallel golangci-lint is running" while another session lints; wait with `until ! pgrep -x golangci-lint; do sleep 3; done` rather than skipping. A `cd` in a compound command is refused by the sandbox even for a sed edit; use Edit.

Noticed, not fixed: none.

## Iteration 4 (T2)

Appended the `/k8s` gomod entry to `.github/dependabot.yml` (same shape as the root entry, limit 5, comment cites spec 017 decision 1 option A). Root entry untouched.

Failing first: `grep -n 'directory: "/k8s"' .github/dependabot.yml` printed nothing and exited 1. After: one line, 38. `yaml.safe_load` sees 3 update entries.

Gate: root `go build`, `go vet`, `go test`, `-tags tmux` and `golangci-lint run` (0 issues) pass; `go -C k8s vet/test/build` pass; `test ! -e go.sum` ok, `grep -c require go.mod` prints 0. **`go test -tags quickstart ./cmd/crswd` FAILS**: `TestDashboardQuickstartStory2Cap`, `quickstart_dashboard_test.go:844: a stream opened after one closed = 429, want 200`, 3 of 3 reruns plus the full suite. This diff touches no Go, so the YAML cannot cause it. The cause I found: the deployed `crswd` (pid 1537404) is now listening on 127.0.0.1:8765, which iteration 3 found free. I did not confirm that is the cause, and I could not run the test at an unmodified HEAD because the sandbox refuses `git worktree add`. Committed anyway because the change is YAML only; the operator should rerun quickstart with 8765 free before the PR.

Rediscovery traps: a `... | tail` after a Go command hides its exit status, so read the output rather than the `$?`. `git worktree add` and `mktemp -d` command substitution are refused by the sandbox.

Noticed, not fixed: the quickstart failure above.

## Iteration 5 (T3)

Added `k8s/internal/kube/boundary_test.go`: `TestBoundaryRootImportsNothingFromK8s` (walks the repo root, skips `k8s/`, `.git/`, `.claude/` and `testdata`, parses imports with `parser.ImportsOnly`, asserts at least 50 files read and no root `go.sum`) and `TestBoundaryDetectsK8sImport` (table over synthetic source: module root, subpackage, grouped import, stdlib only, root-module package), which proves the check can fail.

Failing first: before the `importsK8s` helper existed, `go -C k8s test ./internal/kube -run Boundary` printed `boundary_test.go:42:16: undefined: importsK8s ... [build failed]`. After: both tests pass.

Gate: `go -C k8s vet/test/build` pass. Root `go build`, `go vet`, `go test`, `-tags tmux`, `-tags quickstart ./cmd/crswd` (passed, 51s) and `golangci-lint run` (0 issues) pass. `test ! -e go.sum` ok, `grep -c require go.mod` prints 0. `k8s/` not linted, per the operator note.

Rediscovery traps: the sandbox refuses `;`, `|` and `until` in a Bash call, so run each gate command as its own call. The format hook rewrote the import block after an Edit; read before a second Edit on that region.

Noticed, not fixed: the iteration 4 quickstart failure (`TestDashboardQuickstartStory2Cap` 429) did not recur with 8765 free, so it was most likely port contention with the deployed daemon, as suspected.
