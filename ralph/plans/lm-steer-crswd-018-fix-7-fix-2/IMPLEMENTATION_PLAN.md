# Implementation plan: Claude gone mid-prompt answers 409 over HTTP (lm-steer-crswd-018-fix-7-fix-2)

Fix lane, filed by `loop/fix-features.py` from verify-run 1022-tbfh on
`lm-steer-crswd-018-fix-7` (PR #229). The assertion: "Same case over HTTP: 409
`{"error":"session not ready"}`, deny". The evidence: `go test ./internal/httpapi/ -run Prompt`
exits 0, but `grep -rn 'session not ready' internal/` finds nothing and no prompt test covers a 409.

The HTTP 409 cannot exist alone. `refusePrompt` can only map `ErrSessionNotReady` if
`Manager.Prompt` returns it, and today it never does. The parent milestone finished its T1
(the fake's `ExitAfterPaste`, on `main`) and blocked its T2; see
`ralph/plans/lm-steer-crswd-018-fix-7/PROGRESS.md`, "Iteration 2 — T2, blocked". So this plan
carries the session-side prerequisites (T1-T3) and then the HTTP answer (T4).

Sibling branch `plan/lm-steer-crswd-018-fix-7-fix-1` (unmerged at planning time) plans the
same prerequisites with the same decisions. If `main` has gained any of them before a task
runs, that task's first line says how to detect it and reuse it.

## Facts measured at planning time (10/27/26, main `97a0f58`)

- `go test ./internal/httpapi/ ./internal/session/ -run 'Prompt|AddressesAnothersWindow|Codex'` exits 0.
- `grep -rn 'session not ready' internal/` prints nothing.
- `grep -rn ErrSessionNotReady internal/ cmd/` prints nothing.
- `grep -rn '"C-u"' internal/` prints nothing.
- `Manager.Prompt` is `internal/session/manager.go` line 1194. It calls `m.paste`, then
  `m.tmux.SendKeys(ctx, name, enterKey)` unconditionally. `const enterKey = "Enter"` is just below it.
- `ErrSessionDead` is in the `var (...)` block of `internal/session/session.go` at line 659.
- `refusePrompt` is `internal/httpapi/sessions.go` line 867. It maps only `session.ErrEmptyPrompt`
  (400, via `rejectBadRequest`); everything else is `s.failInternal(w, r, errPromptUndelivered)`.
- `var bodyTeardownUnverified` is `internal/httpapi/sessions.go` line 32.
  `failTeardownUnverified` (near line 752) is the model for a fixed 409: it calls
  `AuditFrom(r.Context()).Deny(...)`, sets `headerContentType` to `contentTypeJSON`, writes
  `http.StatusConflict`, writes the body, and reports a write error through `s.report`.
- `tmuxctl.Fake.ExitAfterPaste(name)` exists (`internal/tmuxctl/fake.go:477`). The next successful
  `Paste` or `PasteBracketed` flips the pane command to `"bash"` once. It does not set `@crswd-binary`.
- `tmuxctl.Fake.SetPaneCommand(name, command)` exists (`internal/tmuxctl/fake.go:803`).
- `tmuxctl.ArgvList()` is exported (`internal/tmuxctl/argv.go:74`): `tmux list-sessions -F <format>`.
  It names no session and has no `-t`.
- `newPromptFixture` (`internal/httpapi/sessions_test.go:883`) plants a session with no
  `@crswd-binary`, so the fake reads it `LivenessUnknown`. For a stopped read after the paste, a test
  calls `f.fixture.tmux.SetOption(ctx, f.live.TmuxName(), tmuxctl.OptionBinary, "claude")` and
  `f.fixture.tmux.ExitAfterPaste(f.live.TmuxName())` before `f.post`.
- The prompt handler runs `OpCapturePane` (the dialog check) before the paste. The exact-call list in
  `TestAPromptIsDeliveredByteForByte` (`internal/httpapi/sessions_test.go:1052`) is capture-pane,
  load-buffer, paste-buffer, send-keys `Enter`.
- The audited server records one audit record per request; tests read it with `s.only(t)`
  (for example `internal/httpapi/sessions_test.go:449`). Deny is `rec["decision"] == string(audit.Deny)`;
  the reason is `rec["reason"]`.
- With the post-paste `List` in place, the parent measured exactly four failures:
  `TestPromptPastesThenSubmits` (`internal/session/manager_test.go:1149`),
  `TestPromptCodexIsBracketed` (`internal/session/harness_test.go:214`),
  `TestAPromptIsDeliveredByteForByte`, and
  `TestNoRequestScopedToOneSessionAddressesAnothersWindow` (`internal/httpapi/isolation_test.go:240`).
- In `codexFixture` (`internal/session/harness_test.go:192`) `@crswd-binary` is `codex` but the fake's
  pane command is `claude`, so the fake reads it `LivenessStopped`.
- `docs/fixes-log.md` does NOT end in a newline. An appended line must start with one.
- `specs/001-crswd-daemon-core/contracts/http-api.md` already documents the 409. No spec edit.
- `go test ./...` has been flaky in the loop container in `internal/sessionpod` only (5 s timeouts
  under load). `go test -count=1 -p 1 ./internal/sessionpod/` passes. Nothing there touches `Paste`.
- `golangci-lint` is at `/home/nctiggy/bin/golangci-lint` on the planning host. Earlier loop
  containers did not have it, and refused `safe-commit.sh` and `git checkout -- <path>`.

## Decisions already made (do not reopen them)

- **D1 Sentinel.** `ErrSessionNotReady = errors.New("session not ready")` in the `var (...)` block
  of `internal/session/session.go`, after `ErrSessionDead`, with a why-comment.
- **D2 Isolation exemption.** In `TestNoRequestScopedToOneSessionAddressesAnothersWindow`, beside
  `bufferOnly`, add `addressesNoWindow`: true only when `call.Argv[1] == "list-sessions"` and no
  element of `call.Argv` equals `"-t"`. Skip the name check when `bufferOnly || addressesNoWindow`.
  Keep both `absentFrom` checks for every call. Never skip by `call.Op == tmuxctl.OpList`.
  Comment: a server-wide listing reads, targets no window, and so cannot be the wrong one.
- **D3 Codex fixture.** In `codexFixture`, after the session is created, call
  `f.tmux.SetPaneCommand(s.TmuxName(), "codex")` so the fake holds what the host holds.
- **D4 Re-check.** After `m.paste` succeeds and before `SendKeys(Enter)`, `Prompt` calls
  `m.tmux.List(ctx)` once and finds the row whose `Name` is `s.TmuxName()`.
  - Row found and `Claude == tmuxctl.LivenessStopped`: `m.tmux.SendKeys(ctx, name, clearLineKey)`,
    no Enter, return an error wrapping `ErrSessionNotReady`. If that `C-u` send fails, still wrap
    `ErrSessionNotReady` (`errors.Join`).
  - `LivenessRunning`, `LivenessUnknown`, or no row: send Enter as today.
  - `List` fails: no Enter, no `C-u`, return the wrapped `List` error.
  - Put this in an unexported helper `submitPaste(ctx, s)`; `Prompt` ends `return m.submitPaste(ctx, s)`.
  - No error string carries prompt text.
- **D5 Key constant.** `clearLineKey = "C-u"`, a daemon-authored constant next to `enterKey`.
- **D6 HTTP answer.** ``var bodySessionNotReady = []byte(`{"error":"session not ready"}`)`` beside
  `bodyTeardownUnverified`. `refusePrompt` checks `errors.Is(err, session.ErrSessionNotReady)` after
  the `ErrEmptyPrompt` branch and calls a new `failSessionNotReady(w, r)` shaped like
  `failTeardownUnverified`: `AuditFrom(r.Context()).Deny("session not ready")`, JSON content type,
  `http.StatusConflict`, `bodySessionNotReady`. Update `refusePrompt`'s doc comment to name the 409.
- **D7 Out of scope.** A pre-paste liveness gate, a `failed`-state refusal, bracketed paste for
  Claude, the `!`/`/` rule, the restart claim, `request_id`, `Type`'s submit path, and a real-tmux
  mid-prompt test.

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

- [ ] T1 isolation test: if `grep -n addressesNoWindow internal/httpapi/isolation_test.go` already prints a line, change nothing. Otherwise add the exemption exactly as D2 says. Test-only change. Verify: `go test ./internal/httpapi/ -run AddressesAnothersWindow` exits 0.
- [ ] T2 Codex fixture: if `codexFixture` already calls `SetPaneCommand`, change nothing. Otherwise apply D3 in `internal/session/harness_test.go`. Test-only change. Verify: `go test ./internal/session/ -run Codex` exits 0.
- [ ] T3 session: D1, D4, D5 (skip any `grep -rn` shows present). New `TestPromptWithdrawsWhenClaudeExitsMidPrompt` in `manager_test.go` via `ExitAfterPaste`: last call send-keys `C-u`, no `Enter`, `errors.Is(err, ErrSessionNotReady)`. Add `OpList` (`tmuxctl.ArgvList()`) to the 3 exact-call tests in Facts. Verify: `go test ./internal/session/ ./internal/httpapi/` exits 0.
- [ ] T4 httpapi 409: D6. New `TestPromptAnswersNotReadyWhenClaudeExitsMidPrompt` in `sessions_test.go`: set `@crswd-binary` to `claude`, `ExitAfterPaste`, post; assert 409, exact body, `C-u` sent, no `Enter`, one deny record with reason `session not ready`, no prompt text in body or record. Verify: `go test ./internal/httpapi/ -run Prompt` exits 0.
- [ ] T5 docs: append one dated line (starting with a newline) to `docs/fixes-log.md` in its existing format: Claude exiting between paste and submit now gets `C-u`, no Enter, and a 409 `{"error":"session not ready"}` with an audit deny; end `(lm-steer-crswd-018-fix-7-fix-2)`. Verify: `grep -q 'session not ready' docs/fixes-log.md` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-7-fix-2/` is always in scope):

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
`go.mod`, `go.sum`, any new directory under `internal/`, and other milestones' notebooks under
`ralph/plans/` (read them, never edit them).
