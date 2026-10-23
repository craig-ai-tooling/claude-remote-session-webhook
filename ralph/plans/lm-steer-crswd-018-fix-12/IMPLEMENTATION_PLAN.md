# Implementation plan: `request_id` on the prompt route (lm-steer-crswd-018-fix-12)

Fix lane, filed from a verify-run FAIL on parent milestone `lm-steer-crswd-018`
(run 1014-la1i). The parent's decision D7 says: optional JSON field `request_id`,
1 to 32 characters of `[a-z0-9-]`; invalid is `400`; valid is echoed in the 202
body and written as `request_id` on the audit record; absent means no field
anywhere. The parent's T001 and T002 landed the audit half. Its T003, the handler
half, never did.

## Facts measured at planning time (10/23/26, main `ebefd36`)

- The verifier's evidence ("`request_id` appears in no .go file") is stale. `internal/audit/audit.go`
  has `Record.RequestID`, written as `request_id` and omitted when empty. `internal/httpapi/middleware.go`
  has `func (a *RequestAudit) SetRequestID(id string)`. Both are tested. Do not touch either file.
- `go test ./internal/httpapi/ -run 'RequestID|Prompt'` exits 0 on the baseline. The only `RequestID`
  match is `TestAHandlerCanAmendTheOneRecordWithARequestID` in `middleware_test.go`, which tests the
  setter alone.
- The gap: `promptRequest` in `internal/httpapi/sessions.go` (near line 264) has one field, `Text`.
  `decodeBody` (`internal/httpapi/decode.go`) calls `DisallowUnknownFields`, so today a body with
  `request_id` answers 400 with reason `errBodyUnknownField`. `promptResponse` (near line 283) has
  no `request_id` field.
- The handler is `func (s *Server) promptSession` in `internal/httpapi/sessions.go` (near line 779).
  Order today: `SessionFrom`, `decode[promptRequest]`, the leading `!`/`/` refusal
  (`errPromptLeadingCommand`), `paneDialogState` (a `capture-pane`, recorded by the fake as a tmux
  call), `s.sessions.Prompt`, then `s.writeJSON(... http.StatusAccepted, promptResponse{...})`.
- `s.rejectBadRequest(w, r, reason)` (`internal/httpapi/decode.go`) writes `400` `bodyBadRequest`
  and records `AuditFrom(r.Context()).Deny(reason.Error())`. `AuditFrom` is in `middleware.go`.
- The prompt route's sentinels live in a `var (...)` block in `sessions.go` (`errPromptNoSession`,
  `errPromptUndelivered`, `errPromptLeadingCommand`, near lines 387 to 405), each with a why-comment.
- Existing tests build bodies with `json.Marshal(promptRequest{Text: text})`. A new field without
  `omitempty` would add `"request_id":null` to every one of those bodies.
- Test fixtures, all in package `httpapi`, `internal/httpapi/sessions_test.go`:
  `newPromptFixture(t)`; `f.post(t, body, testTime)` returns `(recorder, map[string]any)`;
  `f.fixture.tmux.Calls()`; `f.only(t)` returns the one audit record as `map[string]any`;
  `promptBody()` is `{"text":"run the tests"}`. `TestPromptRefusesALeadingCommandCharacter`
  (near line 1231) is the model for a refusal test; `TestPromptAnswersTheContractResponse` for a 202.
- `specs/001-crswd-daemon-core/contracts/http-api.md` and `specs/020-prompt-route-safety/spec.md`
  already document this rule. No spec or contract edit is needed.
- `golangci-lint` is at `/home/nctiggy/bin/golangci-lint` and `~/code/ai-lawnmower/loop/safe-commit.sh`
  exists on the planning host. Earlier loops found both missing inside the loop container.
- Sibling branches `plan/lm-steer-crswd-018-fix-4`, `-7`, `-8`, `-10`, `-11` hold plans only and
  none implements `request_id`. Several of them plan edits to `promptSession` too; keep this change
  small and local so a later merge is easy.

## Decisions already made (do not reopen them)

- **H1 Field.** `promptRequest` gains `RequestID *string `json:"request_id,omitempty"``. A pointer,
  so a present empty string is distinguishable from absent and is refused (1 is the minimum).
- **H2 Rule.** A package-level `func validRequestID(id string) bool` in `sessions.go`: true when
  `1 <= len(id) <= 32` and every byte is `a`-`z`, `0`-`9` or `-`. Bytes, not runes: any non-ASCII
  byte fails.
- **H3 Sentinel.** Add `errPromptBadRequestID = errors.New("request_id is not 1 to 32 characters of [a-z0-9-]")`
  to the prompt sentinels' `var (...)` block, with a why-comment. The reason never contains the value.
- **H4 Where.** In `promptSession`, right after the successful `decode` and before the leading
  `!`/`/` check: if `req.RequestID != nil && !validRequestID(*req.RequestID)`, call
  `s.rejectBadRequest(w, r, errPromptBadRequestID)` and return. The bad value is never set on the record.
- **H5 Record.** Immediately after H4, if `req.RequestID != nil`, call
  `AuditFrom(r.Context()).SetRequestID(*req.RequestID)`. So a valid id is on the record of every
  outcome past that point, including the leading-character 400 and a 500.
- **H6 Echo.** `promptResponse` gains `RequestID string `json:"request_id,omitempty"``, placed last.
  The 202 sets it from `*req.RequestID` when non-nil. Absent stays absent.
- **H7 Out of scope.** `internal/audit/`, `internal/httpapi/middleware.go`, `internal/session/`,
  `internal/tmuxctl/`, the `deploy/crswd-api` client, every file under `specs/`, the parent plan's
  T003 box, and the parent's `specs/020-prompt-route-safety/tasks.md` boxes.

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

- [ ] T1 httpapi: H1-H4. First add the sentinel, then table test `TestPromptRefusesAMalformedRequestID` in `sessions_test.go` with the rows and assertions under "T1 test" below, then the code. Verify: `go test ./internal/httpapi/ -run 'RequestID|Prompt'` exits 0.
- [ ] T2 httpapi: H5+H6. First add the three tests under "T2 tests" below to `sessions_test.go`, then the code. Verify: `go test ./internal/httpapi/ -run 'RequestID|Prompt'` exits 0.
- [ ] T3 docs: append one line dated 2026-10-23 to `docs/fixes-log.md` in its existing format: the API prompt route now accepts optional `request_id` (1 to 32 of `[a-z0-9-]`), echoes it in the 202 and on the audit record, and answers 400 to a malformed one. End with `(lm-steer-crswd-018-fix-12)`. Verify: `grep -n 'request_id' docs/fixes-log.md | grep -q 2026-10-23` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

## Test details

### T1 test

`TestPromptRefusesAMalformedRequestID`, table-driven, `t.Parallel()`, a fresh `newPromptFixture(t)` per row.
Write each body as a raw JSON byte string with `"text":"run the tests"`.

| Row name | `request_id` in the body | Reason on the record |
|---|---|---|
| `33 characters` | `strings.Repeat("a", 33)` | `errPromptBadRequestID` |
| `an uppercase letter` | `Opp-1` | `errPromptBadRequestID` |
| `an underscore` | `opp_1` | `errPromptBadRequestID` |
| `a space` | `a b` | `errPromptBadRequestID` |
| `empty` | `""` | `errPromptBadRequestID` |
| `another unknown field` | `opp-1`, plus `"x":1` in the same body | `errBodyUnknownField` |

Every row asserts: status 400; body exactly `bodyBadRequest`; `f.fixture.tmux.Calls()` is empty;
`f.only(t)` has `decision` deny and the reason above; the record has no `request_id` key.

### T2 tests

- `TestPromptEchoesAValidRequestID`: rows `opp-lease-7`, `strings.Repeat("a", 32)`, `a`. Each answers
  202, the decoded body's `request_id` equals the value, and `f.only(t)["request_id"]` equals it.
- `TestPromptWithoutARequestIDAnswersAsBefore`: post `promptBody()`. 202; the decoded body has exactly
  the keys `id` and `delivered`; the record has no `request_id` key.
- `TestALeadingCommandRefusalKeepsTheRequestID`: text `/status`, `request_id` `opp-1`. 400; zero tmux
  calls; the deny record's `request_id` is `opp-1`.

Before T2's code change, the first and third fail and the second passes. The second is a regression
guard; record it as "passes before, by design" in PROGRESS.md.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-12/` is always in scope):

```allowlist
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `specs/`, `deploy/`, `internal/audit/`,
`internal/session/`, `internal/tmuxctl/`, `internal/httpapi/middleware.go`, `internal/httpapi/decode.go`,
`go.mod`, `go.sum`, any new directory under `internal/`.
