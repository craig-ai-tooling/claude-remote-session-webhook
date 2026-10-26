# Implementation plan: pasted prompt, Claude gone (lm-steer-crswd-018-fix-7-fix-1)

Fix lane, filed by `loop/fix-features.py` from verify-run 1022-tbfh on
`lm-steer-crswd-018-fix-7` (PR #229). The verifier's evidence:
`go test ./internal/session/ -run Prompt` exits 0, but `grep -rn '"C-u"' internal/session/`
prints nothing, and `Prompt` never re-checks the pane after the paste.

The parent milestone finished its T1 (the fake's `ExitAfterPaste`, now on `main`) and then
blocked its T2 with a NEEDS CLARIFICATION. That note is in
`ralph/plans/lm-steer-crswd-018-fix-7/PROGRESS.md`, "Iteration 2 — T2, blocked". This plan
answers both questions it raised (decisions D2 and D3 below) and finishes the work.

## Facts measured at planning time (10/26/26, main `97a0f58`)

- `Manager.Prompt` is `internal/session/manager.go` at line 1194. It pastes with `m.paste`,
  then calls `m.tmux.SendKeys(ctx, name, enterKey)` unconditionally. `enterKey = "Enter"` is
  at line 39.
- `grep -rn ErrSessionNotReady internal/ cmd/` prints nothing.
- `grep -rn '"C-u"' internal/` prints nothing.
- `grep -n 'session not ready' internal/httpapi/sessions.go` prints nothing.
- `tmuxctl.Fake.ExitAfterPaste(name)` exists (`internal/tmuxctl/fake.go:477`). It seeds the
  session if absent, and the next successful `Paste` or `PasteBracketed` flips the pane
  command to `"bash"` once. It does not set `@crswd-binary`.
- `tmuxctl.Fake.SetPaneCommand(name, command)` exists (`internal/tmuxctl/fake.go:803`).
- `tmuxctl.ArgvList()` is exported (`internal/tmuxctl/argv.go:74`). The fake's `List` records
  `OpList` with that argv: `tmux list-sessions -F <format>`. It names no session and has no `-t`.
- `refusePrompt` (`internal/httpapi/sessions.go`, near line 866) maps only `ErrEmptyPrompt`
  (400). Everything else is a 500 via `errPromptUndelivered`.
- `bodyTeardownUnverified` (`internal/httpapi/sessions.go:32`) is the model for a fixed 409 body.
- `specs/001-crswd-daemon-core/contracts/http-api.md` already documents the 409
  `{"error":"session not ready"}`. No contract edit is needed.
- The parent's Iteration 2 built the fix and ran `go test ./internal/session/ ./internal/httpapi/`.
  Exactly four tests failed, all expected:
  - `TestPromptPastesThenSubmits` (`internal/session/manager_test.go:1149`): exact-call list lacks `OpList`.
  - `TestAPromptIsDeliveredByteForByte` (`internal/httpapi/sessions_test.go:1052`): exact-call list lacks `OpList`.
  - `TestPromptCodexIsBracketed` (`internal/session/harness_test.go`): `codexFixture` reads `LivenessStopped`, see D3.
  - `TestNoRequestScopedToOneSessionAddressesAnothersWindow` (`internal/httpapi/isolation_test.go:240`): see D2.
- In `codexFixture` (`internal/session/harness_test.go:192`), `@crswd-binary` is `codex` but the
  fake's pane command is `claude`, so the fake's `livenessOf("codex", "claude")` is `LivenessStopped`.
- The httpapi prompt fixture (`newPromptFixture`, `internal/httpapi/sessions_test.go:883`) plants a
  session with no `@crswd-binary`, so it reads `LivenessUnknown`. A test that needs a stopped read
  first calls `f.fixture.tmux.SetOption(ctx, f.live.TmuxName(), tmuxctl.OptionBinary, "claude")`.
  The prompt handler's dialog check runs `OpCapturePane` before the paste.
- `go test ./...` has been flaky in the loop container in `internal/sessionpod` only (5 s timeouts
  under load). `go test -count=1 -p 1 ./internal/sessionpod/` passes. Nothing there touches `Paste`.
- `golangci-lint` is at `/home/nctiggy/bin/golangci-lint` on the planning host. The parent's loop
  container did not have it.
- In the parent's loop container, `safe-commit.sh` and `git checkout -- <path>` were both refused by
  the Bash permission layer. The fallback was `git add <path>` per path, then `git commit`.

## Decisions already made (do not reopen them)

- **D1 Sentinel.** `ErrSessionNotReady = errors.New("session not ready")` in the `var (...)` block
  with the other session errors in `internal/session/session.go`, after `ErrSessionDead`.
- **D2 Isolation exemption (answers the parent's hard stop).** `internal/httpapi/isolation_test.go`
  is in scope. In `TestNoRequestScopedToOneSessionAddressesAnothersWindow`, beside `bufferOnly`,
  add `addressesNoWindow`: true only when `call.Argv[1] == "list-sessions"` and no element of
  `call.Argv` equals `"-t"`. Skip the name check when `bufferOnly || addressesNoWindow`. Keep the
  `absentFrom` checks for every call. Never skip by `call.Op == tmuxctl.OpList`. The comment
  says why: a server-wide listing reads, targets no window, and so cannot be the wrong one.
- **D3 Codex fixture.** In `codexFixture`, after `mustCreate`, call
  `f.tmux.SetPaneCommand(s.TmuxName(), "codex")` so the fake holds what the host holds.
- **D4 Re-check.** After `m.paste` succeeds and before `SendKeys(Enter)`, `Prompt` calls
  `m.tmux.List(ctx)` once and finds the row whose `Name` is `s.TmuxName()`.
  - Row found and `Claude == tmuxctl.LivenessStopped`: `m.tmux.SendKeys(ctx, name, clearLineKey)`,
    no Enter, return an error wrapping `ErrSessionNotReady`. If that `C-u` send fails, still wrap
    `ErrSessionNotReady` (`errors.Join`).
  - `LivenessRunning`, `LivenessUnknown`, or no row: send Enter as today.
  - `List` fails: no Enter, no `C-u`, return the wrapped `List` error.
  - Put this in an unexported helper `submitPaste(ctx, s)`; `Prompt` ends `return m.submitPaste(ctx, s)`.
- **D5 Key constant.** `clearLineKey = "C-u"` is a daemon-authored constant next to `enterKey`.
- **D6 HTTP answer.** `refusePrompt` maps `ErrSessionNotReady` to 409 with the body
  `{"error":"session not ready"}`, a package-level `[]byte` beside `bodyTeardownUnverified`, and
  records `AuditFrom(r.Context()).Deny("session not ready")`.
- **D7 Out of scope.** A pre-paste liveness gate, a `failed`-state refusal, bracketed paste for
  Claude, the `!`/`/` rule, `Type`'s submit path and a real-tmux mid-prompt test.

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

- [ ] T1 isolation test: add the `addressesNoWindow` exemption exactly as D2 says, with a why-comment. No production code changes. Verify: `go test ./internal/httpapi/ -run AddressesAnothersWindow` exits 0.
- [ ] T2 Codex fixture: apply D3 in `internal/session/harness_test.go`. No production code changes. Verify: `go test ./internal/session/ -run Codex` exits 0.
- [ ] T3 session: D1, D4, D5. New `TestPromptWithdrawsWhenClaudeExitsMidPrompt` in `manager_test.go` via `ExitAfterPaste`: ops paste, paste, list, send-keys; last argv ends `C-u`; no `Enter`; `errors.Is(err, ErrSessionNotReady)`. Add the `OpList` call (argv `tmuxctl.ArgvList()`) to the three exact-call tests named in Facts. Verify: `go test ./internal/session/ ./internal/httpapi/` exits 0.
- [ ] T4 httpapi: D6. New `TestPromptAnswersNotReadyWhenClaudeExitsMidPrompt` in `sessions_test.go`: set `@crswd-binary` to `claude`, `ExitAfterPaste`, post; assert 409, exact body, `C-u` sent, no `Enter`, one deny record with reason `session not ready`. Verify: `go test ./internal/httpapi/ -run Prompt` exits 0.
- [ ] T5 docs: append one dated line to `docs/fixes-log.md` in its existing format: Claude exiting between paste and submit now gets `C-u`, no Enter, and a 409 `session not ready`; end with `(lm-steer-crswd-018-fix-7-fix-1)`. Verify: `grep -q 'C-u' docs/fixes-log.md` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-7-fix-1/` is always in scope):

```allowlist
internal/session/session.go
internal/session/manager.go
internal/session/manager_test.go
internal/session/harness_test.go
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
internal/httpapi/isolation_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `specs/`, `internal/tmuxctl/`,
`go.mod`, `go.sum`, any new directory under `internal/`, and the parent's notebook
`ralph/plans/lm-steer-crswd-018-fix-7/`.
