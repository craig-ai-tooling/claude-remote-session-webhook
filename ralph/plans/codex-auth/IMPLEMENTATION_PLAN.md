# Implementation Plan — Phase 2 — Codex sign-in

Spec: `specs/019-codex-runtime/spec.md`. Tasks (the full contract for each line below):
`specs/019-codex-runtime/tasks.md`. Decisions and measurements: `specs/019-codex-runtime/research.md`.

## Dependency

Phase 1 (`ralph/plans/codex-core`) is merged to `main`. Check with `grep -q RALPH_COMPLETE <(git show main:ralph/plans/codex-core/PROGRESS.md)`; if it exits non-zero, follow PROMPT.md "Blocked work" with the reason `phase 1 not merged`.

## Files touched (allowlist)

A diff outside these paths is out of scope for this notebook. Each task's own Files line in
tasks.md narrows this further.

- internal/codexauth/
- internal/loginrelay/
- internal/httpapi/
- `internal/audit/audit.go`
- `web/templates/partials/header.html`
- `web/templates/dashboard.html`
- `web/templates/session.html`
- `web/templates/settings.html`
- `web/templates/not-found.html`
- `web/templates/partials/signin-panel.html`
- `web/static/crswd.js`
- `web/static/crswd.css`
- `docs/auth-and-sessions.md`
- `docs/components.md`
- ralph/plans/codex-auth/

## Conventions

- `- [ ]` open · `- [x]` done · `- [!]` blocked (reason in `PROGRESS.md`).
- The loop takes the topmost open item.
- Every task ends green: `go build ./... && go vet ./... && go test ./... && golangci-lint run`.
- `go.sum` must never appear in the root module.
- A task is done when something calls it, not when the code exists.
- A new guard is proven by breaking it: the task's test fails before the change.
- No refactoring outside the task (AR-008).

## Tasks

- [ ] **T020** Add the codexauth package. Done when `go test ./internal/codexauth/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T020.
- [ ] **T021** Codex sign-in flow in loginrelay. Done when `go test ./internal/loginrelay/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T021.
- [ ] **T022a** Relays and auth cache become per-harness maps. Done when `go test ./internal/httpapi/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T022a.
- [ ] **T022b** Codex relay wiring and the auth status route. Done when `go test ./internal/httpapi/... -run DashboardAuth` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T022b.
- [ ] **T022c** Per-harness create gate. Done when `go test ./internal/httpapi/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T022c.
- [ ] **T023** needs-auth for Codex panes. Done when `go test ./internal/httpapi/... -run NeedsAuth` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T023.
- [ ] **T024** Sign-in routes and panel for Codex. Done when `go test ./internal/httpapi/... -run SignIn` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T024.
- [ ] **T025** Header view, Codex pill and script. Done when `go test ./internal/httpapi/... -run Header` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T025.
- [ ] **T026** Docs and the full gate. Done when `go test -tags quickstart ./cmd/crswd` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T026.
