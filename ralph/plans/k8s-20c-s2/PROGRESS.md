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

## NEEDS CLARIFICATION

None open.
