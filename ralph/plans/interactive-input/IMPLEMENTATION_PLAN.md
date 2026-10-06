# Implementation Plan — interactive input (spec 018)

**Session page gains a prompt box, a key bar and a scrollback viewer.** Craig
chose this shape on 2026-10-06 over a browser terminal. It works the same for
Claude Code and Codex, because the daemon delivers bytes and knows neither
program.

The full Definition-of-Ready entry for every task is in
`specs/018-interactive-input/tasks.md`, under the same task ID. That file has the
exact signatures, edge cases, file anchors, tests and guardrails. **Read the
task's entry there before you start it.** This file is the checklist the loop
ticks.

## Measured before planning (do not re-measure, do not doubt)

`specs/018-interactive-input/research.md` holds the evidence:

- **R1**: Claude Code submits line one of a plain multi-line paste. With
  `paste-buffer -p`, both Claude and Codex hold the whole message.
- **R2**: `history-limit` set after `new-session` changes nothing. Chaining
  `set-option -g history-limit 5000 ; new-session` in one invocation works, even
  with no server running.
- **R3**: all twelve tmux key names produce the expected bytes on tmux 3.4.

## Conventions

- `- [ ]` open · `- [x]` done · `- [!]` blocked (reason in `PROGRESS.md`)
- The loop takes the topmost open item. Order is dependency order.
- Every task ends green. That means the seven commands under "Every task ends
  green" in `specs/018-interactive-input/tasks.md`.
- `go.sum` must never appear.
- AR-008: no refactoring outside the task.
- A new guard is proven by breaking it, and the result goes in `PROGRESS.md`.
- `-tags tmux` is the only thing that proves a real tmux honoured `-p` or the
  history limit. Run it when the task touches `internal/tmuxctl`.

## Files touched (allowlist)

The loop may create or edit these paths and no others:

- `internal/tmuxctl/controller.go`
- `internal/tmuxctl/fake.go`
- `internal/tmuxctl/exec.go`
- `internal/tmuxctl/argv.go`
- `internal/tmuxctl/argv_test.go`
- `internal/tmuxctl/fake_test.go`
- `internal/tmuxctl/exec_test.go`
- `internal/tmuxctl/exec_tmux_test.go`
- `internal/session/input.go`
- `internal/session/input_test.go`
- `internal/session/manager_test.go`
- `internal/httpapi/input.go`
- `internal/httpapi/input_test.go`
- `internal/httpapi/outcome.go`
- `internal/httpapi/outcome_test.go`
- `internal/httpapi/server.go`
- `internal/httpapi/ratelimit.go`
- `internal/httpapi/dashboard_test.go`
- `internal/httpapi/stylesheet_test.go`
- `internal/audit/audit.go`
- `internal/audit/audit_test.go`
- `internal/audit/leak_test.go`
- `web/templates/partials/pane.html`
- `web/static/crswd.css`
- `web/static/crswd.js`
- `AGENTS.md`
- `docs/security.md`
- `docs/components.md`
- `docs/mobile-open-questions.md`
- `specs/001-crswd-daemon-core/contracts/tmuxctl.md`
- `ralph/plans/interactive-input/IMPLEMENTATION_PLAN.md`
- `ralph/plans/interactive-input/PROGRESS.md`

## Tasks

- [x] **T001** Create every session with `history-limit 5000` (chain in `argvNew`, `HistoryLimit` const, three argv expectations updated). Done when `go test -tags tmux ./internal/tmuxctl -run FiveThousand` and `go test ./internal/tmuxctl ./internal/session` pass.
- [ ] **T002** Add `PasteBracketed` to `Controller`, `Fake`, `Exec`, plus `ArgvPasteBracketed`. Done when `go test -tags tmux ./internal/tmuxctl -run Bracketed` passes and removing `-p` fails it.
- [ ] **T003** Add `CaptureHistory` (`-S -5000 -E -1`, refuse past 5000 lines or 4 MiB with `ErrHistoryTooLarge`) and `Fake.SetHistory`. Done when `go test -tags tmux ./internal/tmuxctl -run History` and `go test ./internal/tmuxctl` pass.
- [ ] **T004** Add `session.Key`, `Keys`, `ParseKey`, `ValidateTyped`, `MaxTypeBytes` and three sentinels in a new input file. Done when `go test ./internal/session -run 'Key|Typed'` passes.
- [ ] **T005** Add `Manager.Type` (Touch, bracketed paste, optional Enter) and `Manager.PressKey`. Done when `go test ./internal/session -run 'Type|PressKey'` passes.
- [ ] **T006** Add `Manager.History` (strip, no Touch, no `unreadable`). Done when `go test ./internal/session -run History` passes.
- [ ] **T007** Add audit actions `dashboard.type`, `dashboard.key`, `dashboard.history` to both action tables. Done when `go test ./internal/audit` passes.
- [ ] **T008** Add seven outcome codes with their exact sentences, and the `inputs` bucket (240/min) built inside `newServer`. Done when `go test ./internal/httpapi -run 'Outcome|InputBudget'` passes.
- [ ] **T009** Add `POST /dashboard/sessions/{id}/type` behind `handleAction`, answering 204. Done when `go test ./internal/httpapi -run Type` passes, including every refusal shape.
- [ ] **T010** Add `POST /dashboard/sessions/{id}/key` behind `handleAction`, sharing the bucket. Done when `go test ./internal/httpapi -run Key` passes.
- [ ] **T011** Add `GET /sessions/{id}/history` behind `handleBrowser`, refusing cross-site, as text/plain no-store. Done when `go test ./internal/httpapi -run History` passes.
- [ ] **T012** Make the leak suite type a canary and prove it reaches no record. Done when `go test ./internal/audit -run 'Leak|Secret|Drives'` passes.
- [ ] **T013** Add the input panel, key bar and Scrollback disclosure to the pane partial, with five classes from existing tokens. Done when `go test ./internal/httpapi` passes, including the stylesheet sweeps.
- [ ] **T014** Add the input client to `crswd.js` (204 handling, Ctrl/Cmd+Enter, scrollback on toggle) and make the shared handler skip it. Done when `go test ./internal/httpapi -run 'Input|Scrollback|CtrlEnter'` passes.
- [ ] **T015** Amend the never-types rule in AGENTS.md and security.md, and update components.md, the mobile Q4/Q5 and the tmuxctl contract. Done when `grep -c 'on its own initiative' AGENTS.md` prints `1` and `go test ./...` passes.
- [ ] **T016** Run the full gate, including `-tags tmux` and `-tags dev`, and break and restore the five guards. Done when `go test -tags tmux ./...` exits 0 and PROGRESS.md lists five failed-then-restored guards.
