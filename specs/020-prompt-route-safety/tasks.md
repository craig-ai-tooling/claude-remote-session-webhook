---
description: "Task list for 020-prompt-route-safety"
---

# Tasks: Prompt Route Safety

**Input**: [spec.md](spec.md) (FR-001 … FR-020, SC-001 … SC-007),
[plan.md](plan.md) (the design and the file list), [research.md](research.md)
(D1 … D9, the decisions and their evidence). The route's contract is
`specs/001-crswd-daemon-core/contracts/http-api.md`.

**Tests**: REQUIRED (`AGENTS.md`, Constitution III). Every T-task below adds at
least one test that **fails against the code before the change**, and the
requirements that must each produce such a test are named in
`ralph/plans/lm-steer-crswd-018/PROGRESS.md` under "Failing-before evidence".

**Read before any task**: `AGENTS.md`, `.specify/memory/constitution.md`,
`docs/conventions.md`, `docs/security.md` §2 (validate at the boundary), §3
(secret hygiene) and §4 (fail closed), plus [spec.md](spec.md) and
[plan.md](plan.md). The tasks are in dependency order: do them top to bottom.

**Scope**: the files a task may touch are the ones it names, and every one of
them is in the allowlist in `ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md`
"Files touched" (reproduced in plan.md "Source code"). Nothing outside it is in
scope — not `AGENTS.md`, not `k8s/`, not `internal/tmuxctl/controller.go` or
`exec.go`, not `go.mod`, and no new directory under `internal/`.

## Task list

The boxes the autonomous loop ticks are the ones in
`ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md`. The boxes here are the
index into the sections below and stay as they are.

- [ ] T001 audit: `Record` gains `RequestID` written as `request_id`, omitted when empty, in `internal/audit/audit.go`
- [ ] T002 httpapi: `RequestAudit.SetRequestID` in `internal/httpapi/middleware.go` carries the value onto the request's one audit record
- [ ] T003 httpapi: prompt body gains optional `request_id` (D7)
- [ ] T004 httpapi: refuse a prompt whose first non-whitespace character is `!` or `/` with 400 and no tmux command (D5)
- [ ] T005 session + httpapi: add `ErrSessionNotReady`; `Prompt` refuses a `failed` session with it before any tmux command; the handler maps it to 409 `{"error":"session not ready"}` (D3)
- [ ] T006 session: `Prompt` pastes with `PasteBracketed` for every harness; `Compact` unchanged (D4)
- [ ] T007 session: liveness gate before the paste (D2)
- [ ] T008 tmuxctl fake: add `ExitAfterPaste(name)`, which sets the pane command to `bash` after the next `Paste` or `PasteBracketed` on that session
- [ ] T009 session: re-check liveness after the paste (D2)
- [ ] T010 session: `Prompt` takes the restart claim after `lockSession`; held means `ErrSessionNotReady` with no tmux command (D6)
- [ ] T011 httpapi: fixtures in `internal/httpapi/testdata/lawnmower/` for answer, resume, opp-lease batch, opp follow-up
- [ ] T012 session tmux test: new file prompt_tmux_test.go in `internal/session/`
- [ ] T013 docs: add a "Prompt route gates (spec 020)" subsection under "Non-negotiables" in `docs/security.md`

## Every task ends green

In this order, from the repository root. These are IMPLEMENTATION_PLAN.md's
"Pre-commit commands" and `AGENTS.md`'s command table, and they are the same
commands CI runs.

1. `gofmt -l .` prints nothing.
2. `go build ./...`
3. `go vet ./...`
4. `go vet -tags tmux ./...`
5. `go vet -tags quickstart ./cmd/crswd`
6. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run`
7. `test ! -e go.sum`
8. `git status --short` lists only allowlisted paths and the notebook directory.

Plus the task's own **Verify** command, and — for every task that changes Go —
`go test ./...`, because a `-run` filter proves the new test and says nothing
about the ones a change broke elsewhere.

`golangci-lint` is **not installed in the autonomous loop's container** (exit
127; see the notebook's Findings). A task that changes Go and cannot run step 6
records that result as unverified in those words rather than claiming it passed.
CI runs the pinned 2.12.2 regardless.

## Conventions every task follows

- Comments explain *why*, never *what*, at the density of the file being edited
  (`docs/conventions.md`).
- Errors wrap with `%w` and context. **No error, log line, audit record or
  response may carry prompt text, its length, a hash of it, or a prefix of it**
  (spec.md FR-002, `docs/security.md` §3). The one caller-chosen value admitted
  onto the trail is `request_id`, and only because of the class and bound T003
  enforces.
- Tests are table-driven, call `t.Parallel()`, and use `tmuxctl.Fake` — except
  `internal/session/prompt_tmux_test.go` (T012), which is `//go:build tmux` and
  drives a real tmux on a private `-L` socket and therefore does **not** call
  `t.Parallel()`.
- No refactoring outside the task (Constitution IV).
- A gate is proven by breaking it: write the test, watch it fail, make the
  change, watch it pass, and record both results in the notebook.

## Failing-before evidence

Each of requirements 1 to 6 needs one row in
`ralph/plans/lm-steer-crswd-018/PROGRESS.md` (SC-001). The task that adds each
row:

| Req | What it proves | Task that adds the row |
|---|---|---|
| 1 liveness gate | stopped refuses; the post-paste read sends `C-u` and no Enter | T009 (covering T007's first read) |
| 2 not-ready answer | a `failed` session answers 409, not 500 | T005 |
| 3 bracketed paste | a two-line prompt arrives as one message | T006 |
| 4 first character | `/status` answers 400 with no tmux command | T004 |
| 5 one at a time | a held restart claim refuses; no interleaving under `-race` | T010 |
| 6 `request_id` | echoed in the 202 and on the record; absent nowhere | T003 |

---

### T001 — audit: `Record` gains `RequestID`

- **Files**: `internal/audit/audit.go`, `internal/audit/audit_test.go`,
  `internal/audit/leak_test.go`.
- **Implements**: spec.md FR-002, FR-018 (the record half).
- **Interface**:
  - In the key block at `audit.go:31`, after `keyRemote`, add
    `keyRequestID = "request_id"`.
  - In `Record` (`audit.go:390`), add `RequestID string` **after `Remote`**, with
    a comment saying why a caller-chosen value is admissible here at all: it is a
    correlation handle bounded to 1–32 characters of `[a-z0-9-]` by the handler
    (spec 020 FR-002, FR-017), and prompt text is still forbidden. Last rather
    than beside `SessionID` because spec 001's `data-model.md` documents the six
    fields in order and that file is not in this milestone's scope.
  - In `Emit` (`audit.go:467`), after the `Remote` block, add
    `if rec.RequestID != "" { r.AddAttrs(slog.String(keyRequestID, rec.RequestID)) }`.
- **Approach**: one field, one key, one conditional attribute. Omitted when
  empty is the contract-compatibility property, not a style choice — every
  record the daemon writes today stays byte-identical.
- **Edge cases**: an empty value is absent, not `""`. Nothing here validates the
  value; T003 is the boundary (`docs/security.md` §2), and a value reaching here
  has already passed it.
- **Mirror**: the `SessionID` field and its conditional attribute, which are the
  same shape for the same reason.
- **Failing-before test**: `TestEmitWritesTheRequestIDWhenTheCallerGaveOne` in
  `audit_test.go` — emit a record with `RequestID: "opp-lease-7"` and assert the
  decoded line carries `request_id` with that value, alongside the seven keys
  `TestEmitWritesTheDocumentedFields` names. It does not compile against the old
  `Record`, which is the failure; note that as the before-result.
- **Other tests**:
  1. `TestEmitOmitsTheRequestIDWhenTheCallerGaveNone` — a record with every
     other field set and no `RequestID`: the key list is exactly the documented
     seven, so a milestone 1 record is unchanged.
  2. `TestRecordCannotCarryFreeFormContent` (`audit_test.go:500`) **must be
     updated**: its `want` list is restated rather than derived, so a new field
     is a deliberate decision. Append `"RequestID"` to the list, matching the
     struct order. This is not relaxing the guard — the type stays a string and
     the count assertion stays exact.
  3. `TestEmitWritesNoKeyBeyondTheDocumentedSeven` (`audit_test.go:94`) needs no
     change: its record carries no `RequestID`. Leave it alone.
  4. `leak_test.go`: in the allowed prompt at `leak_test.go:1277`, send a
     `request_id` alongside the text — a value that is valid under FR-017 and
     carries no mark, because this field is *permitted* on the trail. The sweep's
     claim is unchanged and is the point: with `request_id` present, the run
     still leaks no prompt text, no length and no hash of it (SC-005). The
     refused prompt at `:1308` keeps no `request_id`, so both halves of SC-005
     are driven.
- **Verify**: `go test ./internal/audit/` exits 0.
- **Depends on**: nothing.
- **Guardrails**: do not add a map, a slice, an `any`, or a variadic entry point
  to this package — that is what `TestRecordCannotCarryFreeFormContent` and
  `TestLoggerOffersNoFreeFormEntryPoint` exist to refuse. Do not edit
  `specs/001-crswd-daemon-core/data-model.md`; it is out of scope, and FR-002 is
  where this amendment is recorded.

### T002 — httpapi: `RequestAudit.SetRequestID`

- **Files**: `internal/httpapi/middleware.go`,
  `internal/httpapi/middleware_test.go`.
- **Implements**: spec.md FR-018 (the carrier).
- **Interface**: directly after `SetSessionID` (`middleware.go:120`):

  ```go
  func (a *RequestAudit) SetRequestID(id string) {
      if a == nil {
          return
      }
      a.rec.RequestID = id
  }
  ```

- **Approach**: shaped exactly like `SetSessionID` — nil-safe, no second record,
  amends the one record the middleware opened and will emit on the way out. The
  comment says what makes this one different from `SetSessionID`: the value *is*
  caller-supplied, and what makes that admissible is the class and bound the
  handler checked first (FR-002, FR-017).
- **Edge cases**: called on a nil `*RequestAudit` (a handler under direct unit
  test) is a no-op, like every other method here. An empty value sets an empty
  field, which `audit.Emit` omits.
- **Mirror**: `SetSessionID` (`middleware.go:120`) and `Deny` (`:133`).
- **Failing-before test**: `TestAHandlerCanAmendTheOneRecordWithARequestID` in
  `middleware_test.go`, copied from `TestAHandlerCanAmendTheOneRecord` (`:753`):
  a probe handler calls `SetSessionID` and `SetRequestID`, and the one record
  read back carries `request_id` with the value and keeps its own action. It does
  not compile against the old `RequestAudit`; that is the before-result.
- **Other tests**: extend `TestAuditFromIsSafeOutsideTheMiddleware` (`:790`)
  with a `SetRequestID` call on the nil receiver, so the nil-safety is asserted
  rather than assumed.
- **Verify**: `go test ./internal/httpapi/ -run RequestID` exits 0.
- **Depends on**: T001 (the field must exist).
- **Guardrails**: do not emit a second record, and do not take the value from
  the path or from anything the resolver settled — this method is given a value
  a handler has already validated.

### T003 — httpapi: the prompt body gains optional `request_id`

- **Files**: `internal/httpapi/sessions.go`,
  `internal/httpapi/sessions_test.go`.
- **Implements**: spec.md FR-017, FR-018 (requirement 6, research.md D7).
- **Interface**:
  - `promptRequest` (`sessions.go:264`) gains
    `RequestID string \`json:"request_id"\`` with a comment: the decoder still
    runs with `DisallowUnknownFields`, so this does not make any *other* field
    acceptable.
  - `promptResponse` (`sessions.go:283`) gains
    `RequestID string \`json:"request_id,omitempty"\`` **last**, so a body with
    no `request_id` is byte-for-byte the response milestone 1 clients read.
    `omitempty` is the compatibility property, exactly as it is for `ParkedOn`.
  - In the error block at `sessions.go:387`, add
    `errPromptRequestID = errors.New("request_id is not 1 to 32 characters of [a-z0-9-]")`
    — authored here, carrying no byte the caller sent, which is what makes it
    safe for the trail (FR-042, FR-043).
  - A package-level validator in `sessions.go`, `validRequestID(string) bool`:
    length 1 to 32, every byte in `a-z`, `0-9` or `-`. A byte loop, not a
    regexp — the alphabet is three ranges and the package compiles no pattern
    for anything else.
- **Approach**: in `promptSession` (`sessions.go:772`), immediately after the
  `decode` call and **before** `paneDialogState`, refuse an invalid value with
  `s.rejectBadRequest(w, r, errPromptRequestID)`, then
  `AuditFrom(r.Context()).SetRequestID(req.RequestID)` for a valid non-empty one,
  and echo it in the 202 (`sessions.go:807`). Before `paneDialogState` is the
  property that matters: a refusal costs not even a pane read, let alone a tmux
  command.
- **Edge cases**:
  - Absent: the field is empty, nothing is set, nothing is echoed. This is the
    case `TestPromptAnswersTheContractResponse` (`sessions_test.go:931`) pins
    with `len(body) != 2` — it must still pass untouched.
  - `"request_id":""` present and empty is **0 characters, so outside the bound**
    and a 400 (FR-017). It is not treated as absent.
  - 33 characters, an uppercase letter, an underscore, a space, a `/`, a `.`: all
    400, all with the uniform body.
  - Reused across two deliveries: accepted. The field is a handle the caller
    chose, not an identity this daemon issues (spec.md Edge Cases).
- **Mirror**: `rejectBadRequest` (`decode.go:160`), which already writes
  `bodyBadRequest` *and* the audit deny, so this gate gets its trail entry from
  existing code.
- **Failing-before test**: `TestPromptEchoesAValidRequestIDToTheCallerAndTheTrail`
  in `sessions_test.go` — post `{"text":"run the tests","request_id":"opp-lease-7"}`
  through `promptFixture.post`, assert 202, `body["request_id"] == "opp-lease-7"`,
  and `f.only(t)["request_id"] == "opp-lease-7"`. Against the old code the field
  is an unknown field, so the answer is 400: that is the before-result, and it is
  the row this task adds to the notebook for **req 6**.
- **Other tests**:
  1. `TestPromptWithNoRequestIDMentionsItNowhere` — 202, the key absent from the
     response body and from the record.
  2. `TestARefusedRequestIDCostsNoTmuxCommand` — the table of invalid values
     above: 400, `bodyBadRequest` byte for byte, `errPromptRequestID.Error()` as
     the record's reason, and **zero** tmux calls (count the fake's calls, as
     `TestARefusedPromptCostsNoTmuxCommand` at `:1172` does).
  3. `TestARequestIDReachesNoResponseWhenThePromptIsRefused` — a valid
     `request_id` with an empty `text`: still the 400 the contract gives an empty
     prompt, with no echo.
- **Verify**: `go test ./internal/httpapi/ -run 'RequestID|Prompt'` exits 0.
- **Depends on**: T001, T002.
- **Guardrails**: no second new field on either struct. Do not log, wrap or echo
  the prompt `text` anywhere in this task. Do not loosen
  `DisallowUnknownFields`.

### T004 — httpapi: refuse a leading `!` or `/`

- **Files**: `internal/httpapi/sessions.go`,
  `internal/httpapi/sessions_test.go`.
- **Implements**: spec.md FR-012, FR-013 (requirement 4, research.md D5).
- **Interface**: in the error block at `sessions.go:387`, add
  `errPromptHarnessCommand = errors.New("a prompt may not begin with ! or /, which the harness reads as a command rather than a message")`.
  A helper `beginsWithHarnessCommand(text string) bool` finds the first byte that
  is not space, tab, CR or LF and reports whether it is `!` or `/`.
- **Approach**: the check runs in `promptSession` beside T003's, before
  `paneDialogState`. Refuse through `rejectBadRequest`, so the body is the
  uniform `{"error":"bad request"}` and the reason reaches the trail and nowhere
  else. The rule is about **that one character**: everything later in the text is
  delivered byte-for-byte, which FR-030 of spec 001 requires and which
  `TestAPromptIsDeliveredByteForByte` (`sessions_test.go:1046`) already asserts
  for `;`, `$(id)` and a backtick.
- **Edge cases**:
  - Leading whitespace then `/`: refused. The first *non-whitespace* character is
    the one the rule is about, because the harness skips the whitespace too.
  - A text that is only whitespace: it has no first non-whitespace character, so
    this gate does not refuse it, and `ErrEmptyPrompt` does not either. It is
    delivered, exactly as today (spec.md Edge Cases).
  - `/` or `!` on the second or a later line: delivered. `resume.txt` (T011)
    holds exactly that shape, which is why this is a case and not a nicety.
  - The browser door is untouched: `POST /dashboard/sessions/{id}/type` keeps
    `ValidateTyped` and its C0 refusal, and an operator may still send
    `/compact` from the panel this daemon drew (FR-013).
- **Mirror**: T003's gate, two lines above it.
- **Failing-before test**: `TestAPromptBeginningWithAHarnessCommandIsRefused` in
  `sessions_test.go` — a table of `"/status"`, `"!ls"`, `"  \t/compact"`,
  `"\n\r!"`: each 400 with `bodyBadRequest`, `errPromptHarnessCommand.Error()` on
  the record, and zero tmux calls. Against the old code every one answers 202,
  which is the before-result and the notebook row for **req 4**.
- **Other tests**:
  1. `TestAPromptWithASlashAfterItsFirstCharacterIsDelivered` — `"run /compact
     when you are done"` and `"first line\n/compact"`: 202, and the exact bytes
     on the load-buffer call's stdin.
  2. `TestTheTypeRouteStillDeliversASlashCommand` — in `sessions_test.go`, using
     `newTyper(t)` and `typed(t, live.ID, "/compact", confirmYes)` from
     `input_test.go` (same package, read-only use — that file is **not** in
     scope): 204, and the host operations are `PasteBracketed`, `PasteBracketed`,
     `SendKeys`, as `TestTypeDeliversAndAnswersNoContent` asserts. This is
     FR-013 as a test rather than as a sentence.
- **Verify**: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0.
- **Depends on**: T003 (same two lines of `promptSession`; doing them in order
  keeps the diff readable).
- **Guardrails**: do not edit, strip, trim or normalise the text — this gate
  refuses a prompt, it never rewrites one. Do not extend the rule to any other
  byte: a refusal of `ESC [ 2 0 1 ~` or of C0 controls on this door is named in
  spec.md FR-019 as **not in this feature**, and adding one here would be
  inventing a requirement.

### T005 — session + httpapi: `ErrSessionNotReady` and the 409

- **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`,
  `internal/httpapi/sessions.go`, `internal/httpapi/sessions_test.go`.
- **Implements**: spec.md FR-007, FR-008 (requirement 2, research.md D3).
- **Interface**:
  - In the sentinel block at `manager.go:169`, after `ErrEmptyPrompt`:
    `ErrSessionNotReady = errors.New("session not ready")`. The text is the body
    and the audit reason too, so the three cannot drift apart. The comment names
    the three states that return it (a `failed` record, a *stopped* harness,
    a held restart claim) and says why they are indistinguishable: a caller that
    may not read a session must not learn which state it is in.
  - In `sessions.go`, beside `bodyTeardownUnverified` (`:32`):
    `var bodySessionNotReady = []byte(`{"error":"session not ready"}`)`, and a
    `failSessionNotReady(w, r)` shaped like `failTeardownUnverified` (`:745`) —
    audit deny with `session.ErrSessionNotReady.Error()`, the JSON content type,
    409, the body.
- **Approach**:
  - `Manager.Prompt` (`manager.go:1194`): after the `StateDead` refusal and
    before `lockSession`, refuse `s.State == StateFailed` with
    `fmt.Errorf("prompt session %s: %w", s.ID, ErrSessionNotReady)`. Before any
    tmux command, which is what FR-007 requires.
  - `refusePrompt` (`sessions.go:849`) gains one branch above the `500`:
    `errors.Is(err, session.ErrSessionNotReady)` → `failSessionNotReady`.
    Everything else still falls to the detail-free `failInternal`.
- **Edge cases**:
  - `dead` keeps `ErrSessionDead` and today's answer. That path is shipped and a
    client reads it (FR-008).
  - A `failed` session reached through the resolver: the resolver's own refusals
    are unchanged; this is the state that reaches the handler.
  - Exactly one audit record for the request, a deny, reason `session not ready`.
    `failSessionNotReady` amends the record the middleware opened; it does not
    write a second one.
- **Mirror**: `ErrOrphanedSession` → `failTeardownUnverified` → 409, which is the
  same three-part shape this task adds for a different fact.
- **Failing-before test**: `TestAPromptIntoAFailedSessionAnswersNotReady` in
  `sessions_test.go` — plant a session, set its record's state to `failed`, post
  an ordinary prompt: 409, `bodySessionNotReady` byte for byte,
  `session.ErrSessionNotReady.Error()` as the record's reason, and zero tmux
  calls. Against the old code it is a 500 with `errPromptUndelivered` on the
  record; that is the before-result and the notebook row for **req 2**.
- **Other tests**:
  1. `TestPromptRefusesWhatItCannotDeliver` (`manager_test.go:1191`): add a
     `"a failed session"` row with `want: ErrSessionNotReady`. The existing
     assertion that a refusal costs no tmux command covers the ordering.
  2. `TestPromptNotReadyAnswersTheSameWhicheverStateItIs` — the `failed` case
     here and (after T007 and T010) the stopped-harness and held-claim cases
     answer with identical status, body and reason. Written in this task with the
     one state that exists, and extended by T007 and T010 rather than duplicated.
- **Verify**: `go test ./internal/session/ ./internal/httpapi/ -run Prompt`
  exits 0.
- **Depends on**: nothing in this list; independent of T001–T004.
- **Guardrails**: **do not put the `StateFailed` check in `guardDelivery`**
  (`internal/session/input.go:215`). That guard is shared with `Type` and
  `PressKey`, and FR-013 leaves the browser door exactly as it is. Do not change
  the `dead` path. Do not add a second sentinel: FR-007 says one.

### T006 — session: `Prompt` pastes bracketed for every harness

- **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`,
  `internal/httpapi/sessions_test.go`, `internal/audit/leak_test.go`.
- **Implements**: spec.md FR-009, FR-010, FR-011 (requirement 3, research.md D4).
- **Interface**: none. One call site changes.
- **Approach**: in `Manager.Prompt`, replace `m.paste(ctx, s, []byte(text))` with
  `m.tmux.PasteBracketed(ctx, name, []byte(text))`. `Manager.paste`
  (`manager.go:1253`), `specOf`, `Compact` and the harness specs are **untouched**
  — `paste` now serves `Compact` alone, and its comment is updated to say so and
  to name FR-009 as the reason `Prompt` no longer routes through it.
- **Edge cases**:
  - `Compact`'s constant text depends on its trailing newline becoming a carriage
    return in order to submit itself (FR-010). Changing its paste would stop
    compaction submitting; leave it.
  - A newline is never a reason to refuse (FR-011). Every live payload is
    multi-line.
  - **tmux inserts the paste brackets only when the application in the pane has
    requested bracketed-paste mode** (DECSET 2004). A line-oriented reader that
    never requests it receives a plain paste, which is why the `quickstart`
    shim suite and T012's `cat` case are unaffected by this change. Measured for
    Claude Code 2.1.290 (spec 018 R1) and Codex 0.153.4 (spec 019 M3, M4).
- **Mirror**: `Manager.Type` (`internal/session/input.go`), which is already
  bracketed for every harness and deliberately does not route through `paste`.
- **Failing-before test**: `TestPromptPastesTwoLinesAsOneMessage` in
  `manager_test.go` — deliver `"first line\nsecond line"` and assert exactly
  three calls: `OpPasteBracketed` load-buffer with the payload on stdin,
  `OpPasteBracketed` paste-buffer **carrying `-p`**, and one `OpSendKeys` Enter —
  with no Enter between the two lines. Against the old code the ops are `OpPaste`
  and the argv has no `-p`; that is the before-result and the notebook row for
  **req 3**.
- **Other tests** (all four are updates the change forces, and each one is a test
  that would otherwise have passed against the old behaviour):
  1. `TestPromptPastesThenSubmits` (`manager_test.go:1149`): `OpPaste` →
     `OpPasteBracketed` on both calls, and the paste-buffer argv gains `-p`
     (`argvPasteBufferBracketed`: `tmux paste-buffer -p -d -b <buffer> -t <pane>`).
  2. `TestPromptNamesNoPromptTextInItsError` (`manager_test.go:1234`):
     `FailOp(tmuxctl.OpPaste, …)` → `FailOp(tmuxctl.OpPasteBracketed, …)`.
     Left as it is, the paste no longer fails and the test asserts nothing.
  3. `TestAPromptIsDeliveredByteForByte` (`internal/httpapi/sessions_test.go:1046`):
     the same two ops and the `-p`.
  4. `internal/audit/leak_test.go:1358`: `FailOp(tmuxctl.OpPaste, errHostError)`
     → `OpPasteBracketed`, or the "a prompt the host refused" case stops being a
     500. `leakRun.pasted` (`:1504`) already reads both ops and needs no change.
  5. `TestCompactUsesBufferPath` (`manager_test.go:1258`) must still pass
     **unchanged**, which is FR-010 as a test.
- **Verify**: `go test ./internal/session/ -run 'Prompt|Compact'` exits 0.
  Then `go test ./...`, because this task changes expectations in three packages.
- **Depends on**: T005 (same method; sequential keeps the diff readable).
- **Guardrails**: do not touch `Manager.paste`'s body, `specOf`, or
  `internal/harness`. Do not add a `BracketedPaste` field to any spec. Do not
  refuse a newline.

### T007 — session: the liveness gate before the paste

- **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`.
- **Implements**: spec.md FR-003, FR-004, FR-006 (requirement 1, research.md D2).
- **Interface**: a private helper on `Manager`, e.g.
  `harnessStopped(ctx context.Context, name string) (bool, error)`: one
  `m.tmux.List(ctx)`, find the row whose `Name` is `name`, report whether its
  `Claude` is `tmuxctl.LivenessStopped`. **No method is added to
  `tmuxctl.Controller`** — the k8s module's `podctl` must satisfy that interface
  and does not serve this route (FR-003).
- **Approach**: in `Manager.Prompt`, after the lock and the claim (T010) and
  before the paste, call the helper and refuse `ErrSessionNotReady` when it
  reports stopped. A `List` that errors is returned wrapped
  (`fmt.Errorf("read the liveness of session %s: %w", s.ID, err)`), never an
  implicit pass — this is an auth-adjacent bound and it fails closed
  (`docs/security.md` §4).
- **Edge cases**:
  - `LivenessUnknown` delivers. Spec 012 FR-006 reads that value as alive, and
    refusing it would break every session adopted from a build that predates
    `@crswd-binary`.
  - A row that is **absent** delivers, for the same reason: this is how
    `Supervisor.judge` (`internal/session/supervisor.go:156`) already reads the
    same value, and it is the pattern to copy rather than invent — `Sweep`
    (`:117`) takes one `List` and indexes it by `info.Name`.
  - Pane *content* is never read for this (FR-006). The signal is the
    `@crswd-binary`-versus-pane-command match the controller already computes.
  - The gate adds one `OpList` call to every delivery, so the exact-call
    expectations in `TestPromptPastesThenSubmits`,
    `TestPromptPastesTwoLinesAsOneMessage` and
    `TestAPromptIsDeliveredByteForByte` grow by one (and by two after T009 adds
    the second read). Update them; do not relax them to a count comparison.
  - A fixture whose configured start command is not the fake's default
    `claude` reads as *stopped* once this lands. The default fixture is safe —
    `Manager.Create` writes `@crswd-binary` as `paneProcesses(template)`, which
    for the default harness is `claude`, and `tmuxctl.Fake` gives a created
    session the pane command `claude` (`fake.go:240`). Any test that fails for
    this reason says so with `SetPaneCommand`.
- **Mirror**: `Supervisor.Sweep` and `judge`, named above.
- **Failing-before test**: `TestPromptRefusesASessionWhoseHarnessHasStopped` in
  `manager_test.go` — create a session, `f.tmux.SetPaneCommand(s.TmuxName(),
  "bash")`, then `Prompt`: `errors.Is(err, ErrSessionNotReady)`, and **no paste
  and no send-keys** after the `List` (count the fake's calls from `before`).
  Against the old code the prompt is delivered into the login shell, which is the
  failure requirement 1 exists for.
- **Other tests**:
  1. `TestPromptDeliversWhenLivenessIsUnknown` — a session with no
     `@crswd-binary` (seed one with `Seed`, or clear the option): delivered.
  2. `TestPromptReportsAListFailureRatherThanDelivering` —
     `FailOp(tmuxctl.OpList, errTmuxBroken)`: the error comes back, it is **not**
     `ErrSessionNotReady` (a host failure is not a refusal a caller can act on),
     and no paste ran.
  3. Extend `TestPromptNotReadyAnswersTheSameWhicheverStateItIs` (T005) with the
     stopped-harness case through the route: the same 409, body and reason.
- **Verify**: `go test ./internal/session/ -run Prompt` exits 0. Then
  `go test ./...`.
- **Depends on**: T006.
- **Guardrails**: do not add a method to `tmuxctl.Controller`; do not edit
  `internal/tmuxctl/controller.go` or `exec.go`; do not read a pane, a
  transcript, or any `~/.claude/sessions/*.json` file. Do not gate `Type`,
  `PressKey` or `Compact` — requirement 1 is about this route.

### T008 — tmuxctl fake: `ExitAfterPaste`

- **Files**: `internal/tmuxctl/fake.go`, `internal/tmuxctl/fake_test.go`.
- **Implements**: the seam FR-005 needs to be provable in the default build.
- **Interface**:

  ```go
  // ExitAfterPaste makes the next Paste or PasteBracketed on name end its
  // process: the pane's command becomes the login shell, as it does when a
  // harness exits while text is being delivered into it. It seeds the session
  // if it does not exist yet, as SetPane does.
  func (f *Fake) ExitAfterPaste(name string)
  ```

- **Approach**: a `exitAfterPaste bool` field on `fakeSession` (`fake.go:285`).
  Both `Paste` (`:405`) and `PasteBracketed` (`:427`) clear the flag and set
  `s.paneCommand = "bash"` **after** recording their two calls and after the
  `fail`/unknown-session returns, so an armed session that was never pasted into
  is unchanged. One-shot, because what it models is one exit.
- **Edge cases**: arming a session twice, arming one that never gets a paste,
  and arming an unknown name (seeded, like `SetPane`). A paste that fails
  (`FailOp`) does **not** trip it: nothing was delivered.
- **Mirror**: `QuitAfterInterrupts` (`fake.go:390`), which is the same idea for
  `SendKeys` and `C-c`, including the seeding behaviour and the `"bash"` it
  leaves behind.
- **Failing-before test**: `TestFakeExitAfterPasteLeavesTheLoginShell` in
  `fake_test.go` — `New` a session, `SetOption(OptionBinary, "claude")`,
  `ExitAfterPaste(name)`, assert `List` reports `LivenessRunning`,
  `PasteBracketed`, then assert `List` reports `LivenessStopped`. A second
  subtest does the same through `Paste`; a third arms and never pastes and
  asserts the liveness is unchanged. It does not compile against the old fake,
  which is the before-result.
- **Verify**: `go test ./internal/tmuxctl/ -run ExitAfterPaste` exits 0.
- **Depends on**: nothing.
- **Guardrails**: the fake only. Do not touch `controller.go`, `exec.go` or
  `argv.go`, and do not add an `Op`: nothing new is executed, a session's state
  moves. The fake must hold what the host holds (`docs/conventions.md` Tests) —
  the pane command it leaves is the same `"bash"` `QuitAfterInterrupts` leaves.

### T009 — session: the second liveness read, `C-u`, no Enter

- **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`.
- **Implements**: spec.md FR-005 (requirement 1's second half, research.md D2).
- **Interface**: none. A `C-u` key constant beside `enterKey` if one does not
  already exist, named for what it does (clear the line a shell is reading).
- **Approach**: in `Manager.Prompt`, after the paste and **before** the
  `SendKeys` Enter, call T007's helper again. Stopped means: `SendKeys(ctx,
  name, "C-u")`, no Enter, return `ErrSessionNotReady`. A `List` failure here is
  returned wrapped, as in T007. The comment says what the `C-u` is for — the
  pasted text is cleared off the shell's line rather than left there for the next
  Enter to run — and what the residual window is: the pane can still exit after
  the second read and before the Enter, which is two tmux round trips wide rather
  than a whole request wide (spec.md Edge Cases, plan.md Risks).
- **Edge cases**:
  - The `C-u` itself failing: the error is reported, and `ErrSessionNotReady` is
    what the caller is told if the clear succeeded. Wrap, never swallow.
  - A harness that is *unknown* on the second read delivers, as on the first.
  - The second `OpList` lands between the paste and the send-keys, so the
    exact-call expectations updated in T007 grow by one more. Update them.
- **Mirror**: T007's first read, which this reuses verbatim.
- **Failing-before test**: `TestPromptClearsTheLineWhenTheHarnessExitsMidDelivery`
  in `manager_test.go` — create a session, `SetOption`/`SetPaneCommand` so the
  first read is *running*, `f.tmux.ExitAfterPaste(s.TmuxName())`, then `Prompt`:
  `errors.Is(err, ErrSessionNotReady)`, the recorded calls end
  `… PasteBracketed, List, SendKeys("C-u")`, and **no `Enter` was ever sent**
  (scan every `OpSendKeys` argv). Against the old code the Enter is sent into a
  login shell holding the caller's text; that is the before-result and the
  notebook row for **req 1** (which also covers T007's first read).
- **Verify**: `go test ./internal/session/ -run Prompt` exits 0. Then
  `go test ./...`.
- **Depends on**: T007, T008.
- **Guardrails**: send `C-u` and nothing else — no `C-c`, no clear, no second
  paste. Do not try to close the residual window with a third read: spec.md says
  what bounds it, and a loop here would be inventing a requirement.

### T010 — session: `Prompt` takes the restart claim

- **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`.
- **Implements**: spec.md FR-014, FR-015, FR-016 (requirement 5, research.md D6).
- **Interface**: none. `claimRestart` (`manager.go:2674`) and `releaseRestart`
  (`:2688`) already exist and are non-blocking.
- **Approach**: in `Manager.Prompt`, immediately after `unlock := m.lockSession(s.ID)`
  and its `defer`:

  ```go
  if !m.claimRestart(s.ID) {
      return fmt.Errorf("prompt session %s: %w", s.ID, ErrSessionNotReady)
  }
  defer m.releaseRestart(s.ID)
  ```

  The order is the property (FR-014): the lock is taken first, so the claim is
  attempted by one delivery at a time and a refusal is about a *restart* rather
  than about another prompt.
- **Edge cases**:
  - A revive or a `Continue` holding the claim: refused, not queued (FR-015).
    Text pasted across a revive is text typed at whatever the pane is
    mid-restart.
  - **`Continue` claims before it locks and `Prompt` locks before it claims.**
    That inversion cannot deadlock because `claimRestart` never blocks: a
    `Prompt` holding the lock is refused the claim and returns, releasing it.
    Say so in the comment — this is exactly the kind of correct thing a later
    reader would "fix".
  - Released on **every** path out, including the T007 and T009 refusals and the
    paste failure. The `defer` is what guarantees it.
  - Each paste already loads a buffer name of its own (`Exec.paste`'s
    `newBufferName`; the fake's `nextBuffer`), which is FR-016 and needs no code
    change — only the assertion below.
- **Mirror**: `Continue` (`manager.go:2760`–`2763`), the existing
  claim-then-defer pair.
- **Failing-before test**: `TestPromptRefusesWhileARestartHoldsTheClaim` in
  `manager_test.go` — take the claim from the test (through `Continue`'s own
  path, or by calling the package-private `claimRestart` from this in-package
  test), then `Prompt`: `errors.Is(err, ErrSessionNotReady)` and **no tmux
  command at all**. Against the old code the prompt is delivered into a pane
  being restarted; that is the before-result and the notebook row for **req 5**.
- **Other tests**:
  1. `TestTwoConcurrentPromptsNeverInterleave` — two goroutines, one session,
     distinct payloads, a `sync.WaitGroup`: both succeed, every paste uses a
     buffer name no other paste used, and in the recorded call order no paste
     appears between another delivery's paste and its Enter. Run under `-race`.
  2. Extend `TestPromptNotReadyAnswersTheSameWhicheverStateItIs` (T005) with the
     held-claim case: the third state answers with the same 409, body and reason,
     completing FR-008's "indistinguishable".
- **Verify**: `go test -race ./internal/session/ -run Prompt` exits 0. Then
  `go test ./...`.
- **Depends on**: T005 (the sentinel), T009 (the method's final shape).
- **Guardrails**: do not make the claim blocking, do not queue, and do not take
  it before the lock. Do not touch `Continue`, the supervisor's revival, or
  `lockSession`.

### T011 — httpapi: the four recorded lawnmower payloads

- **Files** (all new): `internal/httpapi/lawnmower_payload_test.go`,
  `internal/httpapi/testdata/lawnmower/answer.txt`,
  `internal/httpapi/testdata/lawnmower/resume.txt`,
  `internal/httpapi/testdata/lawnmower/opp-lease-batch.txt`,
  `internal/httpapi/testdata/lawnmower/opp-followup.txt`.
- **Implements**: spec.md SC-003 — no live caller is broken by any gate in this
  spec.
- **Approach**:
  - Write each fixture **byte for byte** from
    `ralph/plans/lm-steer-crswd-018/PROGRESS.md` "Payload fixtures", without the
    code fence, with a single trailing newline. They are the shapes the four
    builders produce as of 2026-10-14: multi-line, a first character that is a
    letter, double quotes, `$`, `;`, `&`, a backtick, and — in `resume.txt` — a
    later line that begins `/compact`.
  - One table-driven test, `TestEveryLawnmowerPayloadIsStillDelivered`, over the
    four files: `os.ReadFile`, post the bytes verbatim as `text` through
    `promptFixture.post`, assert `202` and `delivered: true`, and assert the
    payload reached tmux on the load-buffer call's stdin byte for byte.
  - Post each with a `request_id` on one case, so the handle rides the real
    shapes too.
- **Edge cases**:
  - `promptFixture.post` signs the timestamp and the body, so four posts to one
    server need **four distinct instants** or the second is refused as a replay.
    Pass `testTime.Add(time.Duration(i) * time.Second)`, or give each case its
    own fixture.
  - No fixture may begin with `!` or `/` — that is the grep D5 rests on, and a
    fixture that did would mean the payload changed, not that the gate is wrong.
  - No fixture holds `ESC [ 2 0 1 ~`. Asserting that is cheap and is the
    evidence spec.md FR-019 cites for leaving that residual open.
- **Mirror**: `promptFixture` and `post` (`internal/httpapi/sessions_test.go:876`,
  `:899`).
- **Failing-before test**: the test is new, so run it before the fixtures exist
  to see it fail on the missing files; the substantive before-result is that
  `opp-lease-batch.txt` and the rest are **multi-line**, and against the code
  before T006 the payload arrives as `OpPaste` with line one submitting on its
  own. Assert the bracketed op, so this test also fails against pre-T006 code.
- **Verify**: `go test ./internal/httpapi/ -run Lawnmower` exits 0.
- **Depends on**: T003 … T010 (it is the regression net over all of them).
- **Guardrails**: the fixture text is data recorded from another repository —
  do not edit it to suit a gate. If a gate refuses one of these payloads, the
  gate is wrong, and that is a finding for the notebook rather than an edit to
  the fixture. No prompt text from these files may reach an assertion message
  that a failure prints into a log: assert on bytes, report positions and
  lengths of the *fixture file*, never the text itself.

### T012 — session: the real-tmux prompt test

- **Files**: `internal/session/prompt_tmux_test.go` (new).
- **Implements**: spec.md SC-002.
- **Approach**:
  - `//go:build tmux` as the first line, package `session`, the header comment
    explaining why a real tmux is needed here: the liveness signal is a round
    trip through tmux's own format string, and the fake is where a format
    mistake would be invisible.
  - Reuse what the tagged build already provides rather than redefining it:
    `newModeFixture(t)` (`mode_test.go:80`) for the private `-L` socket and the
    `kill-server` cleanup, `modeSocketFor`, `waitForPane`, `containsLine`, and
    `capNotUnderTest`. **No `t.Parallel()`** in this file.
  - Start commands of its own through `mgr.SetStartCommands`, neither of which
    runs Claude: one `sh`, one `cat`. A suite that typed the daemon's default
    into a real shell would start an unsandboxed assistant on whatever host ran
    it (`mode_test.go`'s own reason).
  - **The refusal**: create the `sh` session, then set its `@crswd-binary` to
    `claude` with `f.tmux.SetOption(ctx, name, tmuxctl.OptionBinary, "claude")`,
    so the daemon reads a live login shell as a harness that has stopped —
    which is the production state requirement 1 exists for. `Prompt` must return
    `ErrSessionNotReady`, and `capture-pane` must show the pane **byte-for-byte
    unchanged**: the text is not on the line and nothing was run.
  - **The delivery**: create the `cat` session and deliver a two-line prompt.
    `cat` never requests bracketed-paste mode, so tmux inserts no brackets and
    the bytes arrive as they were sent — wait for both lines, each exactly once
    (`containsLine`, not `strings.Contains`: `"first"` is inside `"first-line"`).
- **Edge cases**: `tmux` absent is a `t.Skipf`, as `newModeFixture` already
  does. The deadline for waiting on a real pane is generous, as `waitForPane`'s
  is. Nothing in this file may touch the operator's own tmux server — every
  command goes through the fixture's socket.
- **Mirror**: `internal/session/mode_test.go` end to end; it is the only other
  `//go:build tmux` file in this package.
- **Failing-before test**: the refusal case is the failing-before one — against
  code without T007 the pasted text reaches the shell's line and the Enter runs
  it, so the "pane unchanged" assertion fails loudly.
- **Verify**: `go test -tags tmux ./internal/session/ -run Prompt` exits 0.
  Needs tmux installed (3.4 is what the probes measured).
- **Depends on**: T006, T007, T009.
- **Guardrails**: never the default tmux socket; never a start command that runs
  a real harness; no `t.Parallel()`; no helper redefined that `mode_test.go`
  already declares in this package (it would not compile).

### T013 — docs: the gates in `docs/security.md`

- **Files**: `docs/security.md`.
- **Implements**: Constitution I and V — the security docs are binding, and a
  gate that is not written down is a gate the next reader removes.
- **Approach**: one new `###` subsection, **"6. Prompt route gates (spec 020)"**,
  at the end of `## Non-negotiables` — after `### 5. Dependencies` and before
  `## Configuration, and the page that shows it`. Numbered 6 so the section's
  numbering stays contiguous and existing references to §1 … §5 keep their
  meaning. It names, in the order a request meets them:
  1. the `request_id` class and bound, and that it is the only caller-chosen
     value admitted onto the audit trail (FR-002, FR-017);
  2. the leading-`!`-or-`/` refusal on the API door, and that the browser door is
     unchanged (FR-012, FR-013);
  3. the refusal of a `failed` session, answered `409 {"error":"session not
     ready"}` with that reason on the trail, indistinguishable from the other two
     not-ready states (FR-007, FR-008);
  4. the restart claim, taken after the per-session lock, refused and not queued
     (FR-014, FR-015);
  5. the liveness gate before the paste, and the second read after it that sends
     `C-u` and no Enter — with the sentence that matters: **a pane whose harness
     has exited is a login shell, and a paste-then-Enter into it is arbitrary
     code execution by a caller who was only entitled to deliver a prompt**
     (FR-003 … FR-005);
  6. the bracketed paste for every harness (FR-009), and that `Compact` is
     unchanged (FR-010).

  Each bullet points at the file that enforces it, so a reader can check the
  prose against the code. Prose only — no new rule that no FR states, and
  nothing about the `ESC [ 2 0 1 ~` residual beyond what spec.md FR-019 already
  says, if it is mentioned at all.
- **Edge cases**: `AGENTS.md` must not be edited and must stay under 150 lines
  (CI's "AGENTS.md stays skimmable" check; it is at 147). The Verify command
  asserts both halves.
- **Mirror**: `### 2. Validate every input at the boundary` and
  `### 4. Fail closed`, whose shape and density this follows; and the spec-019
  amendment subsections, which show how this repo records a per-spec amendment
  in these docs.
- **Verify**: `grep -q 'spec 020' docs/security.md && test "$(wc -l < AGENTS.md)" -lt 150`
  exits 0.
- **Depends on**: T001 … T012 — it describes shipped behaviour, so it is written
  once the behaviour is there.
- **Guardrails**: do not edit `AGENTS.md`. Do not restate the whole spec here —
  this is the binding summary an operator and a reviewer read, and
  `specs/020-prompt-route-safety/` is where the detail lives.

---

## Dependencies & execution order

```text
T001 ─→ T002 ─→ T003 ─→ T004 ─┐
                              ├─→ T011 ─→ T013
T005 ─→ T006 ─→ T007 ─→ T009 ─┤          ↑
              T008 ─────────↗  │          │
                          T010 ┘          │
                          T012 ───────────┘
```

- **T001 → T002 → T003**: the field must exist before the carrier, and the
  carrier before the handler that calls it.
- **T003 → T004**: both add a gate to the same two lines of `promptSession`.
- **T005 → T006 → T007 → T009 → T010**: all five change `Manager.Prompt`, and
  the order above is the order the finished method reads in (plan.md "Design").
- **T008 → T009**: the fake's seam is what makes the post-paste gate provable in
  the default build.
- **T011** is the regression net over every gate, so it comes after them.
- **T012** needs the delivery half finished (T006, T007, T009) and a real tmux.
- **T013** describes what the twelve before it built.

**No parallelism is claimed.** The autonomous loop takes one task per iteration
with a fresh context, in this order, and commits after each
(`.specify/memory/constitution.md`, Development Workflow). Five of the thirteen
edit the same method and three edit the same handler, so `[P]` would be wrong
here rather than merely unused.

## What no task in this list may do

Restated from spec.md FR-019 and plan.md, because this is the file an
implementer has open:

- Add a seventh API route, or change the six-route assertion.
- Add a method to `tmuxctl.Controller`, or edit `internal/tmuxctl/controller.go`
  or `internal/tmuxctl/exec.go`.
- Add a package or a directory under `internal/`, add a dependency, or create a
  root `go.sum`.
- Edit `AGENTS.md`, anything under `k8s/`, or `go.mod`.
- Open a Claude transcript or a `~/.claude/sessions/*.json` registry file from
  Go — including the session-registry readiness gate, which is **declined on
  evidence** (research.md D8), not pending.
- Add a byte-level refusal of `ESC [ 2 0 1 ~` or of C0 controls to the API door.
  It is named in spec.md FR-019 as not in this feature, with the evidence that
  no live caller or fixture sends it.
- Put prompt text, its length, a hash of it, or a prefix of it in an error, a log
  line, an audit record or a response.
- Change the browser door's input panel, the `dead` path, `Manager.Compact`, or
  the milestone 1 response shape for a body that carries no `request_id`.
