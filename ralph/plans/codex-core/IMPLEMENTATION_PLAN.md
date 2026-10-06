# Implementation Plan — Phase 1 — Codex core parity

Spec: `specs/019-codex-runtime/spec.md`. Tasks (the full contract for each line below):
`specs/019-codex-runtime/tasks.md`. Decisions and measurements: `specs/019-codex-runtime/research.md`.

## Dependency

Spec 018 (interactive input) is merged to `main`: `git show main:ralph/plans/interactive-input/PROGRESS.md | grep -qx RALPH_COMPLETE` exits 0. If it does not, follow PROMPT.md "Blocked work" with the reason `spec 018 not merged`.

## Files touched (allowlist)

A diff outside these paths is out of scope for this notebook. Each task's own Files line in
tasks.md narrows this further.

- internal/harness/
- `internal/config/config.go`
- `internal/config/codexflags.go`
- `internal/config/codexflags_test.go`
- `k8s/internal/podctl/` (T003 only, and only when that directory exists)
- internal/tmuxctl/
- internal/session/
- internal/httpapi/
- `web/templates/partials/create-form.html`
- `web/templates/partials/session-card.html`
- `web/static/crswd.js`
- `web/static/crswd.css`
- `cmd/crswd/quickstart_test.go`
- `docs/harnesses.md`
- `docs/security.md`
- `docs/components.md`
- `README.md`
- `config.example`
- ralph/plans/codex-core/

## Conventions

- `- [ ]` open · `- [x]` done · `- [!]` blocked (reason in `PROGRESS.md`).
- The loop takes the topmost open item.
- Every task ends green: `go build ./... && go vet ./... && go test ./... && golangci-lint run`.
- `go.sum` must never appear in the root module.
- A task is done when something calls it, not when the code exists.
- A new guard is proven by breaking it: the task's test fails before the change.
- No refactoring outside the task (AR-008).

## Tasks

- [x] **T001** Add the harness package. Done when `go test ./internal/harness/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T001.
- [x] **T002** Liveness matches a set of names. Done when `go test -tags tmux ./internal/tmuxctl/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T002.
- [x] **T003** Controller.PanePID. Done when `go test -tags tmux ./internal/tmuxctl/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T003.
- [x] **T004** Session derives the harness and renders per-harness lines. Done when `go test ./internal/session/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T004.
- [x] **T004a** Config refuses a Codex start command that re-enables the update check. Done when `go test ./internal/config/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T004a.
- [x] **T005** Prompt and Compact use bracketed paste on Codex. Done when `go test ./internal/session/... -run 'Prompt|Compact'` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T005.
- [x] **T006** SetMode refuses non-Claude sessions. Done when `go test ./internal/session/... -run Mode` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T006.
- [x] **T007** Codex trust seeding in config.toml. Done when `go test ./internal/session/... -run 'Codex|Trust'` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T007.
- [x] **T008** Seed trust per harness before typing. Done when `go test ./internal/session/... ./internal/httpapi/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T008.
- [x] **T009** Stepped, verified quit for Codex restarts. Done when `go test ./internal/session/... -run Restart` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T009.
- [x] **T010a** Codex conversation listing and transcript check (pure). Done when `go test ./internal/session/... -run Codex` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T010a.
- [x] **T010b** Per-harness conversation dispatch. Done when `go test ./internal/session/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T010b.
- [x] **T011** /proc conversation discovery (pure). Done when `go test ./internal/session/... -run Discover` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T011.
- [x] **T012** Supervisor records discovered conversations. Done when `go test ./internal/session/... -run 'Sweep|Replay'` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T012.
- [ ] **T013** Dialog registry per harness. Done when `go test ./internal/session/... ./internal/httpapi/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T013.
- [ ] **T014** Browser create: the harness field. Done when `go test ./internal/httpapi/... -run BrowserCreate` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T014.
- [ ] **T014a** The canonical radio group component. Done when `go test ./internal/httpapi/... -run RadioGroup` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T014a.
- [ ] **T015** Create form markup, preview script, components doc. Done when `go test ./internal/httpapi/... -run CreateForm` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T015.
- [ ] **T016** Show the harness on cards, the session page and the API. Done when `go test ./internal/httpapi/...` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T016.
- [ ] **T017** Acceptance: a Codex session end to end. Done when `go test -tags quickstart ./cmd/crswd -run Codex` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T017.
- [ ] **T018** Documentation and the full gate. Done when `go test -tags quickstart ./cmd/crswd` exits 0 and the task's acceptance in tasks.md holds. Full task: specs/019-codex-runtime/tasks.md, section T018.
