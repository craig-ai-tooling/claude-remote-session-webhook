# Implementation plan: a refused "not ready" prompt carries no prompt text (lm-steer-crswd-018-fix-7-fix-3)

Fix lane, filed by `loop/fix-features.py` from verify-run 1022-tbfh on
`lm-steer-crswd-018-fix-7` (PR #229). The assertion: "Refused prompt's error/log/audit carries
no prompt text". The evidence: `go test ./internal/audit/ ./internal/httpapi/` exits 0, but the
"not ready" refusal path does not exist, so nothing tests it.

The no-leak tests cannot exist alone. A prompt is only refused "not ready" once `Manager.Prompt`
re-reads liveness after the paste and `refusePrompt` maps the sentinel to 409. The parent built
the fake's `ExitAfterPaste` (on `main`) and then blocked its T2; see
`ralph/plans/lm-steer-crswd-018-fix-7/PROGRESS.md`, "Iteration 2 — T2, blocked". So T1-T4 carry
those prerequisites, and T5-T7 are the tests this assertion asks for.

Sibling branches `plan/lm-steer-crswd-018-fix-7-fix-1` and `plan/lm-steer-crswd-018-fix-7-fix-2`
(both unmerged at planning time) plan T1-T4 with the same decisions. If `main` has gained any of
them before a task runs, that task's first line says how to detect it and reuse it.

## Conventions

- `- [ ]` open, `- [x]` done, `- [!]` blocked (reason in PROGRESS.md).
- The loop takes the topmost open task. The order is the dependency order.
- Each task's Verify command is the check. It must exit 0 before the box is ticked.

## Facts measured at planning time (10/27/26, main `97a0f58`)

- `go test ./internal/audit/ ./internal/httpapi/` exits 0 (audit 2.7 s, httpapi 15.9 s).
- `grep -rn 'ErrSessionNotReady\|session not ready\|"C-u"' internal/ cmd/` prints nothing.
- `Manager.Prompt` is `internal/session/manager.go` near line 1194. It calls `m.paste`, then
  `m.tmux.SendKeys(ctx, name, enterKey)` unconditionally. `const enterKey = "Enter"` is line 39.
- `ErrSessionDead` is in the `var (...)` block of `internal/session/session.go` at line 659.
- `refusePrompt` is `internal/httpapi/sessions.go` line 867. It maps only `session.ErrEmptyPrompt`
  (400); everything else is `s.failInternal(w, r, errPromptUndelivered)`.
- `var bodyTeardownUnverified` is `internal/httpapi/sessions.go` line 32. `failTeardownUnverified`
  (line 752) is the model for a fixed 409: audit Deny, JSON content type, `http.StatusConflict`,
  the body, and a write error reported through `s.report`.
- `tmuxctl.Fake.ExitAfterPaste(name)` exists (`internal/tmuxctl/fake.go:477`). The next successful
  `Paste` or `PasteBracketed` flips the pane command to `"bash"` once. It does not set `@crswd-binary`.
- `tmuxctl.Fake.SetPaneCommand(name, command)` (`fake.go:803`) and
  `tmuxctl.Fake.SetOption(ctx, name, option, value)` (`fake.go:353`) exist. `tmuxctl.OptionBinary`
  is the `@crswd-binary` option name.
- `tmuxctl.ArgvList()` (`internal/tmuxctl/argv.go:74`) is `tmux list-sessions -F <format>`. It names
  no session and has no `-t`.
- The httpapi prompt fixture `newPromptFixture` (`internal/httpapi/sessions_test.go:883`) plants a
  session with no `@crswd-binary`, so the fake reads it `LivenessUnknown`. For a stopped read after
  the paste, call `f.fixture.tmux.SetOption(ctx, f.live.TmuxName(), tmuxctl.OptionBinary, "claude")`
  and `f.fixture.tmux.ExitAfterPaste(f.live.TmuxName())` before `f.post`.
- The audited test server writes the trail into `f.sink` (a `syncSink`; `f.sink.String()`), and every
  error the handler reports through `s.report` lands in `f.failed`. `f.only(t)` returns the one audit
  record; deny is `rec["decision"] == string(audit.Deny)`, the reason is `rec["reason"]`.
- `TestThePromptTextReachesNoAuditRecordOrLog` (`internal/httpapi/sessions_test.go:1111`) is the
  model for T6: a marker inside the prompt, then the sink, the response body plus headers, and
  `f.failed` are each searched for it. Today it covers only the 202 path.
- `TestPromptNamesNoPromptTextInItsError` (`internal/session/manager_test.go:1234`) covers only a
  paste failure. It uses `hostileLabel` (line 28) as the prompt text and `errTmuxBroken` (line 38).
- The whole-daemon leak sweep is `internal/audit/leak_test.go`. `driveEveryOperation` drives
  "a prompt the host refused" (500) at line 1359 with `r.tmux.FailOp(tmuxctl.OpPaste, errHostError)`.
  `r.tmux` is the same `*tmuxctl.Fake` the server uses. `promptText` (line 128) starts with
  `markPrompt`, which `marks()` already sweeps for in every record and log line. The session in
  scope there is `id`; its tmux name is `session.Session{ID: id}.TmuxName()`.
- With the post-paste `List` in place, the parent measured exactly four failures:
  `TestPromptPastesThenSubmits` (`internal/session/manager_test.go:1149`),
  `TestPromptCodexIsBracketed` (`internal/session/harness_test.go:214`),
  `TestAPromptIsDeliveredByteForByte` (`internal/httpapi/sessions_test.go:1052`), and
  `TestNoRequestScopedToOneSessionAddressesAnothersWindow` (`internal/httpapi/isolation_test.go:240`;
  its `bufferOnly` exemption is line 262).
- In `codexFixture` (`internal/session/harness_test.go:192`) `@crswd-binary` is `codex` but the fake's
  pane command is `claude`, so the fake reads it `LivenessStopped`.
- `docs/fixes-log.md` does NOT end in a newline. An appended line must start with one.
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
    no Enter, return `fmt.Errorf("submit the prompt in session %s: %w", s.ID, ErrSessionNotReady)`.
    If that `C-u` send fails, still wrap `ErrSessionNotReady` (`errors.Join`).
  - `LivenessRunning`, `LivenessUnknown`, or no row: send Enter as today.
  - `List` fails: no Enter, no `C-u`, return the wrapped `List` error.
  - Put this in an unexported helper `submitPaste(ctx, s)`; `Prompt` ends `return m.submitPaste(ctx, s)`.
  - No error string carries prompt text. `submitPaste` never receives the text.
- **D5 Key constant.** `clearLineKey = "C-u"`, a daemon-authored constant next to `enterKey`.
- **D6 HTTP answer.** ``var bodySessionNotReady = []byte(`{"error":"session not ready"}`)`` beside
  `bodyTeardownUnverified`. `refusePrompt` checks `errors.Is(err, session.ErrSessionNotReady)` after
  the `ErrEmptyPrompt` branch and calls a new `failSessionNotReady(w, r)` shaped like
  `failTeardownUnverified`: `AuditFrom(r.Context()).Deny("session not ready")`, JSON content type,
  `http.StatusConflict`, `bodySessionNotReady`. The audit reason is that fixed string, never
  `err.Error()`. Update `refusePrompt`'s doc comment to name the 409.
- **D7 What "no prompt text" means.** A marker string placed inside the prompt must be absent from:
  the returned Go error (session layer), the response body and headers, the audit trail, and every
  error passed to `s.report` (httpapi), and every audit record and log line of the leak sweep.
  T5-T7 assert the refusal happened first (sentinel or 409), so none can pass on the delivered path.
- **D8 Out of scope.** A pre-paste liveness gate, a `failed`-state refusal, bracketed paste for
  Claude, the `!`/`/` rule, the restart claim, `request_id`, `Type`'s submit path, a real-tmux
  mid-prompt test, and any `specs/` edit.

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

- [ ] T1 isolation test: if `grep -n addressesNoWindow internal/httpapi/isolation_test.go` prints a line, change nothing. Otherwise add the exemption exactly as D2 says. Test-only change. Verify: `go test ./internal/httpapi/ -run AddressesAnothersWindow` exits 0.
- [ ] T2 Codex fixture: if `codexFixture` in `internal/session/harness_test.go` already calls `SetPaneCommand`, change nothing. Otherwise apply D3. Test-only change. Verify: `go test ./internal/session/ -run Codex` exits 0.
- [ ] T3 session: D1, D4, D5 (skip any part `grep -rn` shows present). New `TestPromptWithdrawsWhenClaudeExitsMidPrompt` in `manager_test.go` via `ExitAfterPaste`: last call send-keys `C-u`, no `Enter`, `errors.Is(err, ErrSessionNotReady)`. Add `OpList` to the 3 exact-call tests in Facts. Verify: `go test ./internal/session/ ./internal/httpapi/` exits 0.
- [ ] T4 httpapi 409: D6 (skip if `grep -n bodySessionNotReady internal/httpapi/sessions.go` prints a line). New `TestPromptAnswersNotReadyWhenClaudeExitsMidPrompt` in `sessions_test.go` per Facts: assert 409, exact body, `C-u` sent, no `Enter`, one deny record, reason `session not ready`. Verify: `go test ./internal/httpapi/ -run Prompt` exits 0.
- [ ] T5 session no-text: make `TestPromptNamesNoPromptTextInItsError` table-driven with a second row where `ExitAfterPaste` stops Claude mid-prompt. That row asserts `errors.Is(err, ErrSessionNotReady)` first, then that `hostileLabel` is absent from `err.Error()`. Verify: `go test ./internal/session/ -run PromptNamesNoPromptText -v` exits 0 and shows both rows PASS.
- [ ] T6 httpapi no-text: new `TestANotReadyPromptCarriesNoPromptText` in `sessions_test.go`, shaped like `TestThePromptTextReachesNoAuditRecordOrLog`: marker in the prompt, stopped read per Facts, assert 409 first, then marker absent from `f.sink`, response body and headers, and `f.failed`; one deny record. Verify: `go test ./internal/httpapi/ -run NotReady -v` exits 0.
- [ ] T7 leak sweep: in `driveEveryOperation` (`internal/audit/leak_test.go`), after "a prompt the host refused", set `OptionBinary` to `claude` on `id`'s tmux name, `ExitAfterPaste`, `r.want` 409 for a prompt of `promptText`, then `SetPaneCommand(name, "claude")` to restore. Add a comment why. Verify: `go test ./internal/audit/` exits 0.
- [ ] T8 docs: append one dated line (starting with a newline) to `docs/fixes-log.md` in its existing format: a prompt refused "session not ready" (Claude gone between paste and submit) is now tested to carry no prompt text in its error, response, audit record or log; end `(lm-steer-crswd-018-fix-7-fix-3)`. Verify: `grep -q 'lm-steer-crswd-018-fix-7-fix-3' docs/fixes-log.md` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

### Task notes

- **T3, T4.** Write the new test first and record its failure against the old code (PROMPT.md,
  "Prove a new test fails against the old code"). If `main` already carries the code change so the
  test passes at once, record that in PROGRESS.md instead of a failure line.
- **T5, T6, T7.** The refusal path exists by now, so these tests pass on first run. Prove each one
  is not vacuous instead: temporarily change the marker check to look for a string that IS present
  (for example `"session not ready"` in the T6 body), confirm the test fails, record that failure
  line in PROGRESS.md, then restore the check. Never commit the temporary change.
- **T7.** `TestTheLeakSuiteReallyDrivesTheDaemon` runs the same drive. Run the whole package, not
  only the sweep. If restoring the pane command is not enough for a later step in the drive to
  pass, record what failed in PROGRESS.md and follow "If the task is unclear or cannot be finished".

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-7-fix-3/` is always in scope):

```allowlist
internal/session/session.go
internal/session/manager.go
internal/session/manager_test.go
internal/session/harness_test.go
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
internal/httpapi/isolation_test.go
internal/audit/leak_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `specs/`, `internal/tmuxctl/`,
`internal/audit/audit.go`, `go.mod`, `go.sum`, any new directory under `internal/`, and other
milestones' notebooks under `ralph/plans/` (read them, never edit them).
