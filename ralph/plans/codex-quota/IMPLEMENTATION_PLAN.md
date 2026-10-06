# Implementation Plan — Phase 3 — Codex usage meter

Spec: `specs/019-codex-runtime/spec.md`. Tasks (the full contract for each line below):
`specs/019-codex-runtime/tasks.md`. Decisions and measurements: `specs/019-codex-runtime/research.md`.

## Dependency

Phase 2 (`ralph/plans/codex-auth`) is merged to `main`. Check with `grep -q RALPH_COMPLETE <(git show main:ralph/plans/codex-auth/PROGRESS.md)`; if it exits non-zero, follow PROMPT.md "Blocked work" with the reason `phase 2 not merged`. T034 has no dependency, is listed first, and may run before T030: when T034 is the topmost open task, skip the Dependency check for it.

## Files touched (allowlist)

A diff outside these paths is out of scope for this notebook. Each task's own Files line in
tasks.md narrows this further.

- internal/quota/
- internal/httpapi/
- `web/templates/partials/header.html`
- `web/static/crswd.js`
- `web/static/crswd.css`
- `docs/components.md`
- `specs/016-weekly-quota-bar/spec.md`
- `specs/019-codex-runtime/research.md`
- ralph/plans/codex-quota/

## Conventions

- `- [ ]` open · `- [x]` done · `- [!]` blocked (reason in `PROGRESS.md`).
- The loop takes the topmost open item.
- Every task ends green: `go build ./... && go vet ./... && go test ./... && golangci-lint run`.
- `go.sum` must never appear in the root module.
- A task is done when something calls it, not when the code exists.
- A new guard is proven by breaking it: the task's test fails before the change.
- No refactoring outside the task (AR-008).

## Tasks

- [x] **T034** Spike: why quota-axi says auth_required for Codex. Done when `grep -n '^| M19 ' specs/019-codex-runtime/research.md` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T034.
- [x] **T030** quota.ReadProvider. Done when `go test ./internal/quota/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T030.
- [x] **T031** Quota route harness parameter. Done when `go test ./internal/httpapi/... -run Quota` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T031.
- [ ] **T032** Codex meter in the header. Done when `go test ./internal/httpapi/... -run 'Header|Quota'` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T032.
- [ ] **T033** Docs and the full gate. Done when `go test -tags quickstart ./cmd/crswd` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T033.
