# Implementation plan: leading `!` or `/` on the prompt route (lm-steer-crswd-018-fix-9)

Fix lane, filed from a verify-run FAIL on parent milestone `lm-steer-crswd-018`
(run 1014-la1i). The parent's decision D5 says: on the API door only, refuse a
prompt whose first character that is not space, tab, CR or LF is `!` or `/`, with
`400` and the uniform bad-request body, before any tmux command. The browser type
route is unchanged. On `main` at `9a938b4` none of that exists: `promptSession`
in `internal/httpapi/sessions.go` decodes the body and goes straight to the
dialog check and `Manager.Prompt`.

## Facts measured at planning time (10/22/26, main `9a938b4`)

- `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0 on the baseline. It passes only on old tests.
- The handler is `func (s *Server) promptSession` in `internal/httpapi/sessions.go` (near line 772).
  After `decode[promptRequest]` it runs `s.paneDialogState` when `req.Text != ""`. That is a
  `capture-pane`, which the fake records as an `OpCapturePane` call. So the new check must come
  after the decode and before that block, or "no tmux command" fails.
- `s.rejectBadRequest(w, r, reason)` (`internal/httpapi/decode.go`) writes `400`
  `{"error":"bad request"}` (`bodyBadRequest`) and records `AuditFrom(r.Context()).Deny(reason.Error())`.
- The prompt route's sentinels live in a `var (...)` block in `internal/httpapi/sessions.go`
  (`errPromptNoSession` near line 392, `errPromptUndelivered` near line 398), each with a why-comment.
- The browser type route is `typeFromBrowser` in `internal/httpapi/input.go`. It calls
  `internal/session`'s `Type`, never `promptSession`. Putting the check in the handler leaves it untouched.
  `Manager.Prompt` (`internal/session/manager.go`) is not where the check goes: D5 says API door only.
- Test fixtures, all in package `httpapi`:
  - Prompt: `newPromptFixture(t)`, `f.post(t, body, testTime)` returns `(recorder, decoded)`,
    `f.fixture.tmux.Calls()`, `f.only(t)` returns the one audit record as `map[string]any`
    (`rec["decision"]`, `rec["reason"]`, `rec["action"]`). `TestARefusedPromptCostsNoTmuxCommand`
    in `internal/httpapi/sessions_test.go` is the model for a refusal test.
  - `hostilePrompts` (map, `internal/httpapi/sessions_test.go` near line 1029) feeds
    `TestAPromptIsDeliveredByteForByte`. No current entry starts with `!` or `/`.
  - Type: `newTyper(t)`, `ty.live(t)`, `ty.typed(t, live.ID, text, confirmYes)`; success is `204`.
    `TestTypeNormalisesCRLF` in `internal/httpapi/input_test.go` shows how to read pasted bytes
    from `call.Stdin`.
- No test anywhere posts a prompt beginning with `!` or `/`. Checked `cmd/crswd/quickstart_test.go`
  and `cmd/crswd/quickstart_dashboard_test.go` payload lists: all start with a letter, `;`, `"` or `$`.
- Siblings `plan/lm-steer-crswd-018-fix-4`, `-fix-7`, `-fix-8` hold plans only. None adds this check.
- `golangci-lint` is at `/home/nctiggy/bin/golangci-lint` on the planning host. Earlier loops
  recorded it missing inside the loop container.

## Decisions already made (do not reopen them)

- **G1 Sentinel.** Add `errPromptLeadingCommand = errors.New("prompt text begins with a command character")`
  to the prompt sentinels' `var (...)` block in `internal/httpapi/sessions.go`, with a why-comment:
  Claude Code reads a leading `/` as a slash command and a leading `!` as a bash escape.
- **G2 Where.** In `promptSession`, right after the successful `decode`, and before the
  `req.Text != ""` dialog-check block: if the text's first rune not in `" \t\r\n"` is `!` or `/`,
  call `s.rejectBadRequest(w, r, errPromptLeadingCommand)` and return. Use
  `strings.TrimLeft(req.Text, " \t\r\n")` and check its first byte. No other whitespace counts.
- **G3 Whitespace-only and empty text.** Unchanged. They have no such character, so they fall
  through to the existing path (empty still gets `ErrEmptyPrompt` from `Manager.Prompt`).
- **G4 Type route.** No code change in `internal/httpapi/input.go` or `internal/session/`. Only a
  regression test.
- **G5 Out of scope.** `ErrSessionNotReady`, the liveness gates, bracketed paste for Claude,
  the restart claim, `request_id`, the parent's `specs/020-prompt-route-safety/tasks.md` boxes,
  and the parent plan's T004 box. The contract in `specs/001-crswd-daemon-core/contracts/http-api.md`
  already documents this rule. No spec or contract edit.

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

- [x] T1 httpapi: G1+G2. First add table test `TestPromptRefusesALeadingCommandCharacter` in `sessions_test.go`, rows `/status`, `!ls`, `"  /x"`, `"\n\t!x"`, `"\r/x"`: 400, body `bodyBadRequest`, zero tmux calls, one deny with reason `errPromptLeadingCommand.Error()`. Verify: `go test ./internal/httpapi/ -run Prompt` exits 0.
- [x] T2 httpapi tests: add `hostilePrompts` keys `"a later slash": "fix a/b\n/compact"` and `"a later bang": "say hi!"`; add `TestTypeStillDeliversASlashCommand` in `input_test.go`: `/compact` with `confirmYes` answers 204 and the one stdin payload is `/compact`. Verify: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0.
- [ ] T3 docs: append one line dated 2026-10-22 to `docs/fixes-log.md` in its existing format: the API prompt route now answers 400 to text whose first non-whitespace character is `!` or `/`; the type route is unchanged. Use the word `leading`. Verify: `grep -n 'leading' docs/fixes-log.md | grep -q 2026-10-22` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-9/` is always in scope):

```allowlist
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
internal/httpapi/input_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `specs/`, `internal/session/`,
`internal/tmuxctl/`, `internal/httpapi/input.go`, `go.mod`, `go.sum`, any new directory under
`internal/`.
