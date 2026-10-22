# Implementation plan: Claude exits mid-prompt (lm-steer-crswd-018-fix-7)

Fix lane, filed from a verify-run FAIL on parent milestone `lm-steer-crswd-018`
(run 1014-la1i). The parent's decision D2 says: after the paste, read liveness
again; if Claude has stopped, send `C-u`, send no Enter, answer 409. On `main` at
`fc58093` none of that exists. `Manager.Prompt` (`internal/session/manager.go`,
the `func (m *Manager) Prompt` near line 1194) pastes and then sends `Enter`
unconditionally.

## Facts measured at planning time (10/22/26, main `fc58093`)

- `go test ./internal/session/ ./internal/tmuxctl/ ./internal/httpapi/` exits 0.
- `grep -rn ErrSessionNotReady internal/ cmd/` prints nothing. Sibling branch
  `plan/lm-steer-crswd-018-fix-4` (unmerged) also plans to add it. Check before adding.
- `refusePrompt` in `internal/httpapi/sessions.go` maps only `ErrEmptyPrompt` (400);
  everything else is 500 `errPromptUndelivered`.
- `specs/001-crswd-daemon-core/contracts/http-api.md` already documents the 409
  `{"error":"session not ready"}` (parent task P5). No contract edit is needed.
- Fake liveness: `tmuxctl.Fake.List` reports `Claude: livenessOf(options[@crswd-binary], paneCommand)`.
  A session made by `Manager.Create` carries `@crswd-binary` = `claude` and pane command
  `claude`, so it reads `LivenessRunning`. `QuitAfterInterrupts` in `internal/tmuxctl/fake.go`
  is the model for a knob that flips `paneCommand` to `"bash"`.
- The httpapi prompt fixture (`newPromptFixture`, `plant` in `internal/httpapi/sessions_test.go`)
  seeds the tmux session with no `@crswd-binary`, so it reads `LivenessUnknown`. A test that
  needs a stopped read must first call `f.fixture.tmux.SetOption(ctx, f.live.TmuxName(), tmuxctl.OptionBinary, "claude")`
  and only then count calls.
- Exact-call tests that will see the new post-paste `List`: `TestPromptPastesThenSubmits`
  (`internal/session/manager_test.go`), `TestPromptCodexIsBracketed`
  (`internal/session/harness_test.go`), `TestAPromptIsDeliveredByteForByte`
  (`internal/httpapi/sessions_test.go`). `argvList` is unexported, so compare the `OpList`
  call by `Op` only.
- Audit records in httpapi tests: `f.only(t)` returns the one record; deny is
  `rec["decision"] == string(audit.Deny)`, reason in `rec["reason"]`.
- `golangci-lint` is at `/home/nctiggy/bin/golangci-lint` on the planning host. The parent's
  PROGRESS.md records it missing inside the loop container.

## Decisions already made (do not reopen them)

- **F1 Sentinel.** `ErrSessionNotReady = errors.New("session not ready")` in the `var (...)`
  block with the other session errors in `internal/session/session.go`. Add it only if
  `grep -rn ErrSessionNotReady internal/session/` prints nothing.
- **F2 Re-check.** After `m.paste` succeeds and before `SendKeys(Enter)`, `Prompt` calls
  `m.tmux.List(ctx)` once and finds the row whose `Name` is `s.TmuxName()`.
  - Row found and `Claude == tmuxctl.LivenessStopped`: send `m.tmux.SendKeys(ctx, name, "C-u")`,
    send no Enter, return an error wrapping `ErrSessionNotReady`. If the `C-u` send fails,
    still return an error wrapping `ErrSessionNotReady` (use `errors.Join` or `%w` twice).
  - `LivenessRunning`, `LivenessUnknown`, or no row: send Enter as today.
  - `List` fails: no Enter, no `C-u`, return the wrapped `List` error (the handler's 500).
- **F3 Key constant.** `C-u` is a daemon-authored constant next to `enterKey`, never caller text.
- **F4 HTTP answer.** `refusePrompt` maps `ErrSessionNotReady` to 409 with body
  `{"error":"session not ready"}` (a package-level `[]byte` beside `bodyTeardownUnverified`),
  and records `AuditFrom(r.Context()).Deny("session not ready")`. If that mapping already
  exists (`grep -n 'session not ready' internal/httpapi/sessions.go`), reuse it.
- **F5 Out of scope.** The pre-paste liveness gate, the `failed`-state refusal, bracketed paste
  for Claude, the `!`/`/` rule, the restart claim and `request_id` belong to the parent plan or
  fix-4. A real-tmux test of a mid-prompt exit is out of scope: it cannot be timed reliably.

## Pre-commit commands (every task, in this order)

1. `gofmt -l .` prints nothing.
2. `go build ./...` exits 0.
3. `go vet ./...` exits 0.
4. `go vet -tags tmux ./...` exits 0.
5. `go vet -tags quickstart ./cmd/crswd` exits 0.
6. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` exits 0. If golangci-lint is not installed, record that in PROGRESS.md.
7. `test ! -e go.sum` exits 0.
8. `git status --short` lists only paths in "Files touched" or this notebook directory.

## Tasks

- [ ] T1 tmuxctl fake: add `ExitAfterPaste(name)`, which seeds the session if absent and makes the next successful `Paste` or `PasteBracketed` on it set the pane command to `"bash"`, once. Test in `internal/tmuxctl/fake_test.go`: `List` reads running before, stopped after. Verify: `go test ./internal/tmuxctl/ -run ExitAfterPaste` exits 0.
- [ ] T2 session: add `ErrSessionNotReady` if absent (F1) and the post-paste re-check (F2, F3). New `TestPromptWithdrawsWhenClaudeExitsMidPrompt` in `manager_test.go` via `ExitAfterPaste`: calls end paste, paste, `OpList`, send-keys `C-u`; no Enter. Add `OpList` to the three exact-call tests. Verify: `go test ./...` exits 0.
- [ ] T3 httpapi: map `ErrSessionNotReady` to 409 `{"error":"session not ready"}` plus audit Deny (F4). New `TestPromptAnswersNotReadyWhenClaudeExitsMidPrompt` in `sessions_test.go`: set `@crswd-binary`, `ExitAfterPaste`, post; assert exact body, `C-u` sent, no Enter, one deny record. Verify: `go test ./internal/httpapi/ -run Prompt` exits 0.
- [ ] T4 docs: append one dated line to `docs/fixes-log.md` in its existing format: Claude exiting between paste and submit now gets `C-u`, no Enter, and a 409 `session not ready`. Verify: `grep -q 'C-u' docs/fixes-log.md` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-7/` is always in scope):

```allowlist
internal/tmuxctl/fake.go
internal/tmuxctl/fake_test.go
internal/session/session.go
internal/session/manager.go
internal/session/manager_test.go
internal/session/harness_test.go
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `internal/tmuxctl/controller.go`,
`internal/tmuxctl/exec.go`, `go.mod`, `go.sum`, any new directory under `internal/`,
`specs/`.
