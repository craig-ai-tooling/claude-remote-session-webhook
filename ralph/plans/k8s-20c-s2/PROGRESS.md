# Progress: k8s-20c-s2

Notebook for the S2 slice of k8s-20c. Newest entry at the bottom.

## Iteration 0 (planning, 9/30/26)

Plan written from a read of the repo at `8c76e56` on branch `plan/k8s-20c-s2`. Nothing
implemented. Facts checked on disk, so no iteration has to rediscover them:

- Module path `github.com/nctiggy/claude-remote-session-webhook`, `go 1.23.0`, no `require`,
  no `go.sum`. `internal/config/docs_test.go`, `internal/release/readme_test.go`,
  `cmd/crswd/quickstart_test.go` and `internal/sessionpod/deps_test.go` all fail if that changes.
- `api/`, `internal/admit/` and `deploy/k8s/` do not exist yet. `k8s/` (the nested module) does
  not exist either and belongs to S4.
- The group is `crswd.craigcloud.io` (FR-005, commit 3053cec). `crswd.dev` was the earlier answer
  and is wrong.
- `internal/session/workdir.go`: `ResolveWorkDir` at line 66 calls `underAnyRoot` (line 106),
  which calls `underRoot` (line 120). `workdir_test.go:314` calls `underRoot` directly, so keep
  that one unexported.
- `ResolveWorkDir` callers in `manager.go`: Create (~675), ReplayJournal (~2303), Continue
  (~2447). `conversation.go` (159, 303) and `internal/sessionpod/sessionpod.go:125` also call it
  and stay as they are.
- Setter pattern to copy: `Manager.SetStartCommands` at `manager.go:228`.
- `config.ApprovedRoot{Path string; IsDefault bool}` at `internal/config/config.go:471`.
- `.golangci.yml` enables gosec with no preset exclusions, so G304/G306 fire on the generator's
  file writes. A `//nolint:gosec` needs a reason.
- Tools on the planning VM: `golangci-lint`, `tmux`, `jq` all on PATH.

## Re-plan (operator, 10/6/26)

- The kind is `AgentSession` (`agentsessions.crswd.craigcloud.io`, short name `as`), renamed from
  ClaudeSession because Codex is a peer runtime (spec 017 amendment, FR-021). Every name in this
  notebook was renamed with it.
- Spec 019 (Codex) merged since 9/30, so `manager.go` line numbers above are stale: find the three
  `ResolveWorkDir` calls with `grep -n ResolveWorkDir internal/session/manager.go`.

## Iteration 1 (T1, 10/6/26)

Created `api/v1alpha1/types.go` and `types_test.go` per Design §1.

- Failing first: `go test ./api/...` printed `types_test.go:30:27: undefined: AgentSessionSpec`
  (package did not compile, no types yet). Passes after.
- Gate green: build, vet, test, `-tags tmux`, `-tags quickstart ./cmd/crswd` (port 8765 was free),
  golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0.
- Next iteration: `CreationTimestamp time.Time` with `omitempty` still marshals the zero time
  (`0001-01-01T00:00:00Z`); `encoding/json` ignores omitempty on structs. Harmless for T4 (admit takes
  the struct) but T5 and T7 should not rely on it being omitted from JSON.
- The full quickstart suite takes about 95 seconds.

## Iteration 2 (T2, 10/6/26)

Exported `UnderAnyRoot` and added `LexicalWorkDir` in `internal/session/workdir.go`; new
`workdir_lexical_test.go`.

- Failing first: `go test ./internal/session/ -run 'LexicalWorkDir|UnderAnyRootExported'` printed
  `workdir_lexical_test.go:41:16: undefined: LexicalWorkDir` (package did not compile). Passes after.
- Gate green: build, vet, test, `-tags tmux`, `-tags quickstart ./cmd/crswd` (port 8765 free, 58s),
  golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0.
- Next iteration: `underAnyRoot` had no caller outside `workdir.go`, so the rename touched only
  that file and `manager.go` was not needed for T2. T3 edits `manager.go`: find the three
  `ResolveWorkDir` calls with `grep -n ResolveWorkDir internal/session/manager.go`.
- Piping the gate through `grep -v '^ok'` makes `$?` 1 when everything passed; read the output, not the code.

## Iteration 3 (T3, 10/6/26)

Added `Manager.SetWorkDirResolver` and the private `m.workDir` helper in `internal/session/manager.go`;
Create, journal replay and Continue go through it. New `resolver_test.go`.

- Failing first: `go test -run WorkDirResolver ./internal/session` printed
  `resolver_test.go:18:8: f.mgr.SetWorkDirResolver undefined (type *Manager has no field or method SetWorkDirResolver)`
  (package did not compile). 4 tests pass after.
- Gate: build, vet, test, `-tags tmux`, golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0 all green.
  **`go test -tags quickstart ./cmd/crswd` fails on `TestDashboardQuickstartStory2Cap`**
  (`quickstart_dashboard_test.go:844: a stream opened after one closed = 429, want 200`).
  It fails the same way on a clean detached checkout of HEAD `ea6a61a` (6 of 10 runs there, 8 of 10 on this
  tree with the change), so it is not caused by T3. It passed in iterations 1 and 2 on this VM. Cause not
  investigated (looks timing-dependent: a stream slot not yet released). Committed anyway because reverting
  would not change the result; the operator should decide whether to chase it.
- Next iteration: the resolver test covers Create only. Replay and Continue use the same helper but have
  no test of their own (plan asked for Create-level tests). A manager's `roots` are passed to the resolver
  as-is, so a resolver never sees a copy.
- A scratch worktree for a baseline check must not live under an unset `$TMPDIR` (it resolves to `/`);
  `go test -C <dir>` avoids the `cd` prompt.

## Iteration 4 (T4, 10/6/26)

Created `internal/admit/admit.go` (pure `Admit`) and `admit_test.go` (25 table rows plus a no-mutation test).

- Failing first: `go test ./internal/admit` printed `admit_test.go:178:19: undefined: Admit`
  (package did not compile, no `Admit` yet). Passes after.
- Gate green: build, vet, test, `-tags tmux`, `-tags quickstart ./cmd/crswd` (port 8765 free, 58s),
  golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0. `TestDashboardQuickstartStory2Cap`
  passed this time, so the iteration 3 flake is intermittent.
- Next iteration: reasons are fixed sentences: "working directory is not under an approved root",
  "lifetime must be a positive duration such as 8h", "session is past its lifetime", "session limit reached".
  `CreationTimestamp + lifetime == now` counts as past. A zero `CreationTimestamp` with a short lifetime
  is rejected as past; the reconciler (S-later) reads real API-server timestamps, so it is not a concern here.
- Do not run the gate with `cd` or `&&` chains that mix `-C` and `golangci-lint`: the harness stalls for
  approval. Run each command separately from the worktree root.

## Iteration 5 (T5, 10/6/26)

Created `deploy/k8s/manifest/` with `manifest.go` (constants, `Files()`), `crd.go` (`CRD()`) and `crd_test.go`.

- Failing first: `go test ./deploy/k8s/manifest` printed `crd_test.go:34:16: undefined: Files`
  (package had no non-test source, so it did not compile). Passes after.
- Gate green: build, vet, test, `-tags tmux`, `-tags quickstart ./cmd/crswd` (port 8765 free, 53s),
  golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0.
- Next iteration: `Files()` holds a `map[string]any` of file name to value and currently has only `crd.json`.
  T6 adds `rbac-reconciler.json`, `rbac-lease.json`, `rbac-daemon.json` by adding entries there and
  `rbac.go` with the three Lists. `crd_test.go` already has `dig`, `asList`, `asObject`, `asStrings`,
  `keysOf` helpers that `rbac_test.go` can reuse (same package). errcheck flags bare `.([]any)` type
  assertions in tests, so use those helpers rather than asserting inline.
- `CRD()` returns `map[string]any`, so rules/enums in it are `[]string` until marshalled; tests decode
  through `Files()` to see the real JSON types.

## Iteration 6 (T6, 10/6/26)

Added `deploy/k8s/manifest/rbac.go` (`ReconcilerRBAC`, `LeaseRBAC`, `DaemonRBAC`, registered in `Files()`) and `rbac_test.go`.

- Failing first: `go test ./deploy/k8s/manifest` printed `rbac_test.go:216: Files() has no rbac-reconciler.json`
  (and the same for the daemon file; 6 RBAC tests failed). 12 tests pass after.
- Gate green: build, vet, test, `-tags tmux`, golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0.
  `-tags quickstart ./cmd/crswd` failed once on `TestDashboardQuickstartStory2Cap` (the iteration 3 flake,
  `quickstart_dashboard_test.go:844`, 429 want 200) and passed on the immediate rerun (75s, port 8765 free).
- Next iteration (T7): `Files()` now has four entries, so the generator writes `crd.json`, `rbac-reconciler.json`,
  `rbac-lease.json`, `rbac-daemon.json`. `rbac_test.go` declares types `rbacList`, `rbacObject`, `policyRule`,
  `subject` and helper `has`; do not reuse those names in `drift_test.go` or `fixtures_test.go`. The production
  helper is `roleAndBinding`. `deploy/k8s/*.json` does not exist yet, so run the generator before the drift test.
- Not fixed: the walker's `secrets`/`*` check ignores `resourceNames` and `nonResourceURLs`; neither is used.

## Iteration 7 (T7, 10/6/26)

Added `deploy/k8s/gen/main.go`, ran it to write the four JSON files, and added `drift_test.go` and `fixtures_test.go`.

- Failing first: `go test ./deploy/k8s/manifest` printed
  `drift_test.go:23: read crd.json: open ../crd.json: no such file or directory; run ...` (and the same for the
  three RBAC files). Passes after the generator ran.
- The SC-004 walk test passes on a clean tree, so it cannot fail first against the old code. `TestForbiddenHitsCanFail`
  plants an upper-case match in a temp dir and proves the scanner reports it.
- Gate green: build, vet, test, `-tags tmux`, golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0.
  `-tags quickstart ./cmd/crswd` failed once on `TestDashboardQuickstartStory2Cap` (the iteration 3 flake) and
  passed on the immediate rerun (87s, port 8765 free).
- Next iteration (T8): gosec G304 fires on `os.ReadFile` with a variable path in tests too, so each carries a
  `//nolint:gosec` with a reason. The generator writes at 0600; git stores the files as 0644 regardless.
  `deploy/` (not `deploy/k8s`) holds `crswd.example.service`, which names the forbidden strings, so the SC-004 walk
  must stay scoped to `deploy/k8s`. T8 edits only `specs/017-k8s-native-execution/spec.md` FR-013, then runs
  every command in `VALIDATION_CONTRACT.md` and appends `RALPH_COMPLETE`.

## Iteration 8 (T8, 10/6/26)

Amended spec 017 FR-013 (reconciler: pods get, `agentsessions/status` update, Lease in its own namespace; daemon `pods/exec` verbs `create`, `get`).

- Failing first: `sed -n '223,229p'` of the spec at HEAD piped to `grep -c -E 'leases|agentsessions/status'` printed `0`. After the edit both strings are in the FR-013 paragraph (spec.md lines 225-226).
- Gate: build, vet, test, `-tags tmux`, golangci-lint 0 issues, no go.sum, `grep -c require go.mod` = 0. `-tags quickstart ./cmd/crswd` failed twice on `TestDashboardQuickstartStory2Cap` (the iteration 3 flake, `quickstart_dashboard_test.go:844`) and passed on the third run (47s, port 8765 free). Spec-only change, so it cannot cause that failure.

Validation contract results (each command run from the repo root):

- Gate green: yes, with the flake rerun above.
- `test ! -e go.sum` ok; `grep -c require go.mod` printed 0.
- `git diff --diff-filter=M --name-only origin/main...HEAD -- '*_test.go'` printed nothing.
- `go run ./deploy/k8s/gen && git status --porcelain deploy/k8s` printed nothing (run before this commit, tree clean for deploy/k8s).
- CRD jq: `agentsessions.crswd.craigcloud.io`, `crswd.craigcloud.io`, `Namespaced`, `AgentSession`; `true,true,true`.
- Spec keys `["conversation","lifetime","owner","sessionName","startCommand","workDir"]`; status keys `["conversation","phase","reason"]`.
- Secrets/wildcard walk printed `0`.
- Daemon walk printed `[{"ns":"crswd-next","pods":["get","list","watch"],"exec":["create","get"]}]`: one Role, no `create` on pods, no `crswd` Role in `crswd-next-reconciler`.
- Reconciler: Role `crswd-reconciler` in `crswd-next` has pods `create,get,list,watch,delete`, agentsessions `get,list,watch,update`, agentsessions/status `update`; Role `crswd-reconciler-lease` in `crswd-next-reconciler` has only coordination.k8s.io leases `get,create,update`. RoleBindings name `crswd`/`crswd-next` and `crswd-reconciler`/`crswd-next-reconciler`.
- `grep -rniE 'lawnmower|\.claude' deploy/k8s/` printed nothing, exit 1.
- `go test -v -run Admit ./internal/admit` passes with cases for outside, dotdot, over cap, past lifetime, admitted. On `main` the package is absent, so it fails there.
- `go test -run 'WorkDirResolver|LexicalWorkDir' ./internal/session -v` passes (6 tests: missing dir under root admitted, outside refused, unset still refuses).
- Spec greps for `leases` and `agentsessions/status` each hit lines inside FR-013.
- Printer columns `["Start","Phase","Age"]`; short names `["as"]`.

Not fixed: `TestDashboardQuickstartStory2Cap` flakes on clean HEAD too (iteration 3); operator should decide whether to chase it. The contract's daemon `jq` walk only works because it filters on `.metadata.name`; a bare `select(.kind == "Role")` also matches each RoleBinding's `roleRef`.

## NEEDS CLARIFICATION

None open.

RALPH_COMPLETE
