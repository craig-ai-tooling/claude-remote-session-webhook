# Implementation Plan — Phase 4a — Codex in kubernetes mode

Spec: `specs/019-codex-runtime/spec.md`. Tasks (the full contract for each line below):
`specs/019-codex-runtime/tasks.md`. Decisions and measurements: `specs/019-codex-runtime/research.md`.

## Superseded (10/6/26)

Do not run this notebook. Spec 017 slice S5 (`ralph/plans/k8s-20c-s5`) absorbed T041 to T043,
and S5 T11 with S7 cover T044. It stays as the record of the original Phase 4a plan.

## Dependency

Spec 017 slice S7 is merged (`k8s/go.mod` exists on `main`: `git show main:k8s/go.mod` exits 0) and Phase 1 is merged. If either check fails, follow PROMPT.md "Blocked work" with the reason that failed. Operator-run measurement O-1 is not a prerequisite for any task here.

## Files touched (allowlist)

A diff outside these paths is out of scope for this notebook. Each task's own Files line in
tasks.md narrows this further.

- internal/sessionpod/
- `internal/session/manager.go`
- k8s/
- docs/k8s-mode.md (created by spec 017 S7)
- ralph/plans/codex-k8s/

## Conventions

- `- [ ]` open · `- [x]` done · `- [!]` blocked (reason in `PROGRESS.md`).
- The loop takes the topmost open item.
- Every task ends green: `go build ./... && go vet ./... && go test ./... && golangci-lint run`.
- `go.sum` must never appear in the root module.
- A task is done when something calls it, not when the code exists.
- A new guard is proven by breaking it: the task's test fails before the change.
- No refactoring outside the task (AR-008).

## Tasks

- [!] **T041** CODEX_HOME passes into session pods. Done when `go test ./internal/sessionpod/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T041.
- [!] **T042** In-pod conversation lookup. Done when `go -C k8s test ./...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T042.
- [!] **T043** podctl uses the in-pod lookup. Done when `go -C k8s test ./...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T043.
- [!] **T044** Docs and the full gate. Done when `go -C k8s vet ./...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T044.
