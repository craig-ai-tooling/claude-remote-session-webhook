---
description: "Task list for 020-prompt-route-safety"
---

# Tasks: Prompt Route Safety

**Tests**: REQUIRED per `AGENTS.md`. Every task names the test that must fail without it.

**Read first**: [spec.md](spec.md) (FR-001 to FR-021, SC-001 to SC-011),
[research.md](research.md) (D1 to D9 and their evidence), [plan.md](plan.md) (the gate's step
order, the handler, the audit record, the fake). The loop's own order, allowlist and memory are
`ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` and `PROGRESS.md`; where this file and
the allowlist could drift, **the allowlist wins**.

**Line numbers** were read on `ralph/1014-eks5` at `a4dc0c1` (10/14/26). Find the named
function or constant with `grep -n` and edit that, never a stale line.

**The gate** (every task ends with all of these, in this order):

```
gofmt -l .                                   # prints nothing
go build ./...
go vet ./...
go vet -tags tmux ./...
go vet -tags quickstart ./cmd/crswd
GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run
test ! -e go.sum
git status --short                           # only allowlisted paths and the notebook
```

**Global guardrails** (every task, not restated per entry):

- No new dependency; `go.sum` must not appear (FR-020).
- No seventh API route, no new package under `internal/`, no new method on
  `tmuxctl.Controller` (FR-020). The gate reads `Controller.List`, so `k8s/` stays untouched.
- Prompt text, its length and a hash of it reach no response, error, log line or audit record
  (FR-016). The one new field a record may gain is `request_id`, bounded by FR-014.
- Every new test is table-driven, calls `t.Parallel()` and uses `tmuxctl.Fake` — except
  T012's `//go:build tmux` file, which uses a real tmux on a private `-L` socket and calls no
  `t.Parallel`, as `internal/session/mode_test.go` already does for its own reason (FR-021).
- Never edit `AGENTS.md`, anything under `k8s/`, `internal/tmuxctl/controller.go`,
  `internal/tmuxctl/exec.go` or `go.mod`.
- `Manager.Compact`, `Manager.Type` and `Manager.PressKey` keep today's behaviour exactly.
  Only `Manager.Prompt` and the API door's prompt handler change.

## How to read an entry

Each task below is one loop iteration. `**Failing-before test**` is the test to write *first*,
with what it asserts and how it fails against the unchanged code — that failure, and its line,
goes in `PROGRESS.md`. `**Verify**` is the command that must exit 0 when the task is done, and
is the same command the notebook's task line carries. `**Also update**` names existing tests
that go red for a *correct* reason: extend them, never relax them.

## Phase 0: Spec — notebook P1 to P5 and G1

Not T-tasks and not listed here as tasks: the four documents in this directory (P1 to P4), the
amendment to `specs/001-crswd-daemon-core/contracts/http-api.md` (P5, FR-019), and the G1 gate
that confirms no open question survives into the Go work. They are Markdown only, they come
first, and their order and Verify commands live in the notebook.

**G1 blocks every task below.** No Go changes before it exits 0.

---

## Phase 1: Correlation — User Story 6 (Priority: P3), requirement 6

**Goal**: a caller can find its own delivery on the trail.

**Why first**: it is the only change that adds a field rather than a refusal, and the three
tasks are strictly additive — nothing in Phase 2 onwards reads them, so a problem here is
visible before any gate exists to confuse it with.

**Independent test**: post a prompt with `"request_id":"abc-123"`; the `202` echoes it and the
request's one audit record carries it. Post `"request_id":"AB C"`; it is `400`, no tmux command.

- [ ] T001 audit: `Record` gains `RequestID` written as `request_id`, omitted when empty, in `internal/audit/audit.go`. Test both cases in `internal/audit/audit_test.go`.
  - **Files**: `internal/audit/audit.go`, `internal/audit/audit_test.go`,
    `internal/audit/leak_test.go`.
  - **Implements**: FR-015, FR-018. Asserted by SC-008.
  - **Change**: add `keyRequestID = "request_id"` to the key block (`audit.go:30-36`). Add
    `RequestID string` as the **last** field of `Record` (`audit.go:390`), with a doc comment
    saying what FR-018 says: it is the caller's own correlator, bounded to 1 to 32 characters
    of `[a-z0-9-]` **by the handler that accepts it**, and it is the one caller-chosen value
    the trail may carry — because that bound cannot hold prompt text, a sentence, a path or a
    credential. In `Emit` (`audit.go:468`), after the `Remote` block, add
    `if rec.RequestID != "" { r.AddAttrs(slog.String(keyRequestID, rec.RequestID)) }`, so the
    attribute order stays data-model.md's field order with the new field last.
  - **Failing-before test**: `TestEmitWritesTheRequestID` in `audit_test.go` — a table with a
    present value (the decoded line carries `request_id` with that value) and an empty one
    (the key is absent). Against the unchanged code it fails to **compile**:
    `unknown field RequestID in struct literal of type audit.Record`. Record that compile
    error and its line as the failing-before evidence; a package that does not build is a
    failing test, and this is the only shape this task's evidence can take.
  - **Also update**: `TestEmitWritesNoKeyBeyondTheDocumentedSeven` (`audit_test.go:94`) — its
    `want` gains `"request_id"` in sorted position and it becomes the eighth-key assertion;
    rename it only if the name is now false. `TestRecordCannotCarryFreeFormContent` (`:500`)
    — `want` gains `"RequestID"` as the seventh entry; **do not loosen** its
    every-field-is-a-string check, which is the half of FR-018 that makes the amendment safe.
  - **leak_test.go**: read before editing. `markPrompt` is `CANARY-PROMPT` (`:83`) and the
    suite drives the prompt route at `:1277` with `{"text": promptText}` only, so no canary
    needs to move for this task. Touch the file only if the new field makes a reflective
    assertion there stale — check with `grep -n 'NumField\|keysOf' internal/audit/leak_test.go`
    — and leave the sweep's own forbidden-value list alone.
  - **Verify**: `go test ./internal/audit/` exits 0.
  - **Depends**: G1.
  - **Guardrails**: the field is a `string`, like every other field on `Record`. No map, no
    slice, no `any`. Nothing in `internal/audit` validates the value — the bound belongs to the
    handler at the boundary (T003), and a second validator here would be a second rule.

- [ ] T002 httpapi: `RequestAudit.SetRequestID` in `internal/httpapi/middleware.go` carries the value onto the request's one audit record. Test in `internal/httpapi/middleware_test.go`, name containing `RequestID`.
  - **Files**: `internal/httpapi/middleware.go`, `internal/httpapi/middleware_test.go`.
  - **Implements**: FR-015. Asserted by SC-008.
  - **Change**: add `func (a *RequestAudit) SetRequestID(id string) { … }` beside
    `SetSessionID` (`middleware.go:120`), nil-safe exactly as every other method on that type
    is, setting `a.rec.RequestID = id`. Its comment carries the one rule a caller must not
    get wrong: this is the only method on `RequestAudit` that is *given* caller-supplied
    bytes, and it may be called only with a value the handler has already validated against
    FR-014. Note the ordering rule the `emitted` field (`:111`) states: a handler that emits
    early must say everything first.
  - **Failing-before test**: `TestAHandlerCanAmendTheOneRecordWithARequestID` in
    `middleware_test.go`, modelled on `TestAHandlerCanAmendTheOneRecord` (`:753`) — a handler
    that calls `SetRequestID("abc-123")` produces exactly one record carrying
    `request_id: "abc-123"`, and a handler that calls it with `""` produces one record with no
    such key. Against the unchanged code it fails to compile:
    `ra.SetRequestID undefined (type *httpapi.RequestAudit has no field or method SetRequestID)`.
  - **Verify**: `go test ./internal/httpapi/ -run RequestID` exits 0.
  - **Depends**: T001 — the setter writes the field T001 added.
  - **Guardrails**: no route calls it yet. Do not touch `promptSession` in this task; T003 is
    where the value comes from, and keeping them apart is what keeps each failing-before test
    honest.

- [ ] T003 httpapi: prompt body gains optional `request_id` (D7). Invalid is 400 with no tmux command; valid is echoed in the 202 and set on the audit record. Tests in `internal/httpapi/sessions_test.go`, names containing `RequestID`.
  - **Files**: `internal/httpapi/sessions.go`, `internal/httpapi/sessions_test.go`.
  - **Implements**: FR-014, FR-015. Asserted by SC-008.
  - **Change**:
    - `promptRequest` (`sessions.go:264`) gains ``RequestID string `json:"request_id"` ``. Its
      comment says what the field is for and what it may not become: a correlator the caller
      chose, bounded so it cannot be a second `text`.
    - A package-level validator — a length check (1 to 32 when present) and a byte walk over
      `[a-z0-9-]`, no regexp — and a sentinel for the refusal, spelled the way
      `errBodyUnknownField` and its siblings are so `rejectBadRequest` carries no caller bytes.
      An **absent** field is valid; an **empty** one is not (FR-014 says 1 to 32).
    - The check runs in `promptSession` (`:772`) immediately after `decode` and **before**
      `paneDialogState` (`:828`), because that helper reads the pane whenever `req.Text != ""`:
      an invalid body must cost the host not even a pane read.
    - On success, `AuditFrom(r.Context()).SetRequestID(req.RequestID)` before the delivery, and
      `promptResponse` (`:283`) gains ``RequestID string `json:"request_id,omitempty"` `` so a
      caller that sends none sees byte-for-byte the body it sees today.
  - **Failing-before test**: `TestAPromptRequestIDIsEchoedAndAudited` and
    `TestARefusedRequestIDCostsNoTmuxCommand` in `sessions_test.go`, on `newPromptFixture`
    (`:883`). The first: a valid `"abc-123"` answers `202` with `"request_id":"abc-123"` and
    the request's one record carries `request_id`. The second: a table of
    `""`, 33 characters, `"AB C"`, `"abc_123"`, `"ABC"`, `"a b"` — each `400` with the uniform
    bad-request body and **zero** recorded tmux calls, including no `capture-pane`. A third
    row of the first test covers the absent case: no `request_id` key in the response and none
    on the record. Against the unchanged code the body field is unknown, so
    `DisallowUnknownFields` answers `400` for the *valid* case too and the first test fails on
    its status assertion; note that line.
  - **Also update**: nothing. `refusedPrompts` (`:1152`) keeps its rows; the new refusals are
    their own table so the two reasons stay distinguishable.
  - **Verify**: `go test ./internal/httpapi/ -run 'RequestID|Prompt'` exits 0.
  - **Depends**: T002.
  - **Guardrails**: `request_id` exists on this one route. Do not add it to the browser's
    input bodies, to `typeRequest`, or to any other `*Request` struct — an unknown field is
    already refused everywhere else and that is the correct answer there (spec.md Edge Cases).
    The response still carries no echo of the text.

**Checkpoint**: the trail can carry a correlator, and nothing about delivery has changed yet.

---

## Phase 2: The first character — User Story 4 (Priority: P2), requirement 4

**Goal**: a text that would be a slash command or a shell escape is refused on the API door
before anything is typed.

**Independent test**: post `"/clear"` and `"  \n!rm -rf /"` to the API prompt route; each is
`400` and the fake records nothing. Post `/compact` through the browser type route; delivered.

- [ ] T004 httpapi: refuse a prompt whose first non-whitespace character is `!` or `/` with 400 and no tmux command (D5). Add a test that the browser type route still delivers `/compact`.
  - **Files**: `internal/httpapi/sessions.go`, `internal/httpapi/sessions_test.go`.
  - **Implements**: FR-009, FR-010, FR-011, and half of FR-017. Asserted by SC-005.
  - **Change**: a package-level helper that walks the text for the first byte that is not
    `0x20`, `0x09`, `0x0D` or `0x0A` and reports whether it is `!` or `/`, plus a sentinel for
    the refusal. A text with no such byte — all whitespace — does **not** refuse here; it
    meets the emptiness and length checks that already apply (spec.md Edge Cases). The call
    goes next to T003's validation in `promptSession`, before `paneDialogState`, and answers
    `rejectBadRequest` with the uniform body.
  - **Why it lives in the handler, not in `Manager.Prompt`**: that is what confines it to the
    API door. The session page's panel calls `Manager.Type` and `Manager.PressKey`, which do
    not pass through this handler, so FR-011 holds by construction rather than by a flag. A
    comment must say so, because moving this check "down where the other prompt rules live" is
    exactly the refactor that would silently take `/compact` away from the operator.
  - **Failing-before test**: `TestAPromptStartingWithASlashOrBangIsRefused` in
    `sessions_test.go` — a table over `"/clear"`, `"/compact"`, `"!id"`, `"  /clear"`,
    `"\n\t!id"`, `"\r\n/clear"`, each `400` with the uniform body and zero recorded tmux calls;
    plus delivered rows for `"please run /compact later"`, the `resume` shape
    (`"Craig has …\n/compact only if the context gets long"`, FR-010's case) and `"   "`
    (whitespace-only, which keeps today's `ErrEmptyPrompt`-shaped answer). Against the
    unchanged code the refusal rows answer `202`; note that line.
  - **Also**: `TestTheTypeRouteStillDeliversASlashCommand` in the same file — the browser type
    route with `/compact` delivers (FR-011). It goes in `sessions_test.go` and **not** in
    `internal/httpapi/input_test.go`, which is not on the notebook's allowlist; read
    `TestTypeDeliversAndAnswersNoContent` (`input_test.go:87`) for the fixture shape and write
    the new test next to the prompt tests.
  - **Verify**: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0.
  - **Depends**: T003 — same handler, same decode boundary; one after the other keeps each
    failing-before test honest.
  - **Guardrails**: no existing payload in the tree starts with `!` or `/` — `hostilePrompts`
    (`sessions_test.go:1029`) starts with `;`, `f` and `a`; the leak suite's `promptText`
    starts with `CANARY-PROMPT` (`internal/audit/leak_test.go:83`); the quickstart payloads
    start with `echo` (`cmd/crswd/quickstart_dashboard_test.go:489`). So this task breaks no
    existing test, and if one goes red the payload changed, not the rule. The rule reads the
    first non-whitespace character of the **whole text** and never the start of a line.

**Checkpoint**: the API door refuses the two first characters that would not be a prompt, and
the operator's own panel still accepts them.

---

## Phase 3: Readiness and its one answer — User Story 2 (Priority: P1), requirement 2

**Goal**: a session that cannot take a prompt says so, in one answer, to a caller that already
reads it as a refusal.

**Independent test**: post a prompt for a `failed` session through the API door; `409` with
`{"error":"session not ready"}`, and the fake records no tmux command.

- [ ] T005 session + httpapi: add `ErrSessionNotReady`; `Prompt` refuses a `failed` session with it before any tmux command; the handler maps it to 409 `{"error":"session not ready"}` (D3).
  - **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`,
    `internal/httpapi/sessions.go`, `internal/httpapi/sessions_test.go`. The sentinel joins the
    `var (…)` block at `internal/session/session.go:633`; that file is **not** on the
    allowlist, so declare it in `manager.go` beside the method that raises it and say in its
    comment why it is not in the block with its siblings.
  - **Implements**: FR-004, FR-005, FR-006. Asserted by SC-003.
  - **Change**:
    - `ErrSessionNotReady = errors.New("session is not ready for a prompt")`, with the comment
      D3 asks for: one sentinel for every "not right now" refusal this spec adds — `failed`
      state, stopped liveness, a held restart claim — because they are one fact to the caller
      and are told apart only server-side, on the trail. It is a refusal about **timing**, like
      `ErrRestartInFlight` (`session.go:667`) and unlike `ErrSessionDead`.
    - In `Manager.Prompt` (`manager.go:1194`), after the `text == ""` check and **before**
      `lockSession` (`:1209`): `if s.State == StateFailed { return fmt.Errorf("prompt session
      %s: %w", s.ID, ErrSessionNotReady) }`. It sits before the lock because it reads the
      record the caller was handed and needs no serialisation. `StateDead` keeps
      `ErrSessionDead` on the line above, unchanged and deliberately so.
    - In `refusePrompt` (`sessions.go:849`), one new branch before the `500`:
      `errors.Is(err, session.ErrSessionNotReady)` → `409` with the constant body
      `{"error":"session not ready"}` and `AuditFrom(r.Context()).Deny("session not ready")`.
      The body is a Go constant and the reason is a literal this repo authored; neither is
      built from the request. Write the `409` with `s.writeJSON` the way the other answers on
      this route are written, so the door's existing header and encoding rules apply
      unchanged.
  - **Failing-before test**: `TestPromptRefusesAFailedSession` in `manager_test.go` — a session
    moved to `StateFailed` is refused with `errors.Is(err, ErrSessionNotReady)` and the fake
    records **zero** calls after the state change; and `TestAPromptForANotReadySessionAnswers
    FourOhNine` in `sessions_test.go` — the response is `409`, the body is exactly
    `{"error":"session not ready"}`, the request's one audit record is a Deny whose reason is
    `session not ready` and which carries no prompt text, and no tmux command ran. Against the
    unchanged code the first fails to compile (`undefined: ErrSessionNotReady`) and, once the
    sentinel exists but the check does not, on its "want ErrSessionNotReady" assertion; note
    whichever line the Verify command prints. **This is req 2's row** in PROGRESS.md's
    failing-before table.
  - **Verify**: `go test ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0.
  - **Depends**: T004.
  - **Guardrails**: `dead` keeps today's answer — the resolver's uniform `404` in practice, and
    `ErrSessionDead` where this method is reached directly. Do not fold the two sentinels
    together, and do not give the `409` a second body spelling anywhere: one gate, one answer
    (FR-006). Nothing about which gate refused reaches the caller.

**Checkpoint**: one sentinel, one `409`, one body — with one gate behind it so far.

---

## Phase 4: Delivery — User Story 3 (Priority: P1), requirement 3

**Goal**: a multi-line prompt arrives once, as one prompt, on every harness.

**Independent test**: `Manager.Prompt` on a Claude session records `paste-buffer -p -d`
(bracketed) and then exactly one `send-keys Enter`; a two-line text records the same and no more.

- [ ] T006 session: `Prompt` pastes with `PasteBracketed` for every harness; `Compact` unchanged (D4). Update `TestPromptPastesThenSubmits` and add a two-line case.
  - **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`,
    `internal/audit/leak_test.go`, `internal/httpapi/sessions_test.go`.
  - **Implements**: FR-007, FR-008, and half of FR-017. Asserted by SC-004.
  - **Change**: in `Manager.Prompt`, replace `m.paste(ctx, s, []byte(text))` (`manager.go:1217`)
    with a direct `m.tmux.PasteBracketed(ctx, name, []byte(text))`, keeping the error wrap
    verbatim — it names the session and nothing else, for FR-016's reason. Leave `m.paste`
    (`:1253`) in place for `Compact`, and leave its comment's claim about `Type` alone: `Type`
    is bracketed for every harness and still must not route through `paste`. Add one sentence
    to `Prompt`'s doc comment recording that this amends spec 018 FR-006 (FR-017) and why:
    inside a bracketed paste the harness receives the text as pasted input rather than as
    typed keys, so a blank line in the opp-lease batch handoff cannot submit eleven
    half-formed questions.
  - **Failing-before test**: extend `TestPromptPastesThenSubmits` (`manager_test.go:1149`) to
    assert `OpPasteBracketed` and a `paste-buffer` argv carrying `-p` (its current `want` at
    `:1165-1166` spells `OpPaste` and no `-p`), and add a **two-line** row —
    `"first line\nsecond line"` — asserting the same two paste calls plus exactly one
    `send-keys Enter`, and no second Enter. Against the unchanged code the `-p` assertion
    fails; note that line. **This is req 3's row** in PROGRESS.md's failing-before table.
  - **Also update** (each goes red for a correct reason — the prompt's op changed):
    - `internal/httpapi/sessions_test.go:1071-1073` — the expected call pair in
      `TestAPromptIsDeliveredByteForByte` becomes `OpPasteBracketed` with `-p` in the second
      argv. The stdin assertion and the "the bytes appear in no argv" assertion are unchanged
      and are the point of that test.
    - `internal/httpapi/sessions_test.go:1222` — `"the paste failed": tmuxctl.OpPaste` in
      `TestATmuxFailureDuringAPromptAnswersFiveHundredWithNoDetail` becomes
      `OpPasteBracketed`, or that row stops failing anything.
    - `internal/audit/leak_test.go:955` and `:1358` — `FailOp(tmuxctl.OpPaste, errHostError)`
      and the matching reset no longer fail a prompt. Both become `OpPasteBracketed`. The
      sweep at `:1509` already reads both ops and needs no change.
    - `internal/session/manager_test.go:1239` — `FailOp(OpPaste, errTmuxBroken)` in
      `TestPromptRefusesWhatItCannotDeliver` becomes `OpPasteBracketed`.
    - `TestCompactUsesBufferPath` (`manager_test.go:1258`) must stay green **unedited** — it
      asserts `OpPaste` and `/compact\n` for Claude, which is exactly what D4 keeps.
  - **Allowlist gap, read before starting**: `internal/session/harness_test.go` holds
    `TestPromptClaudeUnchanged` (`:237`), which asserts that a Claude prompt uses **no**
    bracketed paste and no `-p` — the direct negation of FR-007 — and that file is **not** on
    the notebook's allowlist. Confirm with
    `grep -n 'TestPromptClaudeUnchanged' internal/session/harness_test.go`. The task cannot be
    finished without that assertion changing, so this is a plan decision and not a code one:
    follow the loop's "task cannot be finished" procedure, which puts the question in
    `PROGRESS.md` and the `- [!]` on the task, rather than editing a file the allowlist does
    not name or deleting a test to make a suite pass. `PROGRESS.md`'s P4 entry records the
    same finding and the options.
  - **Verify**: `go test ./internal/session/ -run 'Prompt|Compact'` exits 0.
  - **Depends**: T005. Separate from it so the bracketed-paste change carries its own
    failing-before test.
  - **Guardrails**: `Compact` keeps `m.paste` and its harness-based choice. No newline refusal
    is added anywhere (FR-008) — the opp-lease handoff pastes multi-line item bodies, and the
    byte checks that apply are the ones already in force. Exactly one `send-keys Enter` per
    prompt, as today.

**Checkpoint**: delivery is one bracketed paste and one Enter on every harness.

---

## Phase 5: Liveness — User Story 1 (Priority: P1), requirement 1

**Goal**: a prompt into a session whose harness has exited is refused, having typed nothing —
and a harness that exits mid-delivery leaves no half-line for a stray Enter to run.

**Independent test**: with the fake's pane command set to something other than the session's
`@crswd-binary`, `Manager.Prompt` returns `ErrSessionNotReady` and the fake records no
`load-buffer`, no `paste-buffer` and no `send-keys`.

- [ ] T007 session: liveness gate before the paste (D2). `LivenessStopped` returns `ErrSessionNotReady` with no paste; `LivenessUnknown` delivers; a `List` failure is an error. Use the fake's `SetPaneCommand`.
  - **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`.
  - **Implements**: FR-001, FR-003. Asserted by SC-001.
  - **Change**: a small unexported helper on `Manager` that calls `m.tmux.List(ctx)`, finds the
    row whose `Name` equals `s.TmuxName()` and reports its `Claude` liveness together with
    whether the row was present. In `Prompt`, after the lock (and, once T010 lands, after the
    claim) and before the paste:
    - `tmuxctl.LivenessStopped` → `ErrSessionNotReady`, nothing typed.
    - `tmuxctl.LivenessUnknown` or `LivenessRunning` → continue. Unknown **delivers**, and the
      comment must say why rather than leaving it to look like an oversight: the rule is
      written on the `tmuxctl.Liveness` type itself
      (`internal/tmuxctl/controller.go:312-332`), it is the zero value deliberately, and
      `internal/session/supervisor.go:153-156` is the existing caller that reads it this way.
    - A row that is **absent** → `ErrSessionNotReady`. It cannot be read as alive.
    - A `List` that fails → wrapped and returned (`fmt.Errorf("… %w", err)`), never read as a
      pass. The gate fails closed.
    - Read the liveness **once per check** and pass it, rather than calling `List` twice inside
      one check: the second read is T009's, and it is a different question at a different time.
  - **Failing-before test**: `TestPromptRefusesAStoppedHarness` in `manager_test.go` — a table
    over the fake's `SetPaneCommand` (`internal/tmuxctl/fake.go:757`): `"bash"` (stopped) is
    refused with `ErrSessionNotReady` and the fake records **zero** calls after the setup;
    `"claude"` (running) delivers; a session whose `@crswd-binary` option is empty is
    `LivenessUnknown` and delivers; `FailOp(tmuxctl.OpList, …)` is an error that is neither
    `ErrSessionNotReady` nor a delivery; and a session the fake does not hold at all is
    refused. Against the unchanged code the stopped row delivers `202`-shaped success and the
    call-count assertion fails; note that line.
  - **Verify**: `go test ./internal/session/ -run Prompt` exits 0.
  - **Depends**: T006 — the gate is asserted against the paste shape T006 settled.
  - **Allowlist gap, read before starting**: `TestPromptCodexIsBracketed`
    (`internal/session/harness_test.go:214`) creates a Codex session on the fake, whose pane
    command is the fake's `fakeAliveCommand` (`"claude"`, set by `Fake.New` at `fake.go:340`)
    while its `@crswd-binary` is Codex's process set — so `livenessOf` (`fake.go:249`) reads
    **stopped** and this gate refuses the prompt that test expects to deliver. The one-line
    fix is a `SetPaneCommand(s.TmuxName(), "codex")` in that fixture, and that file is not on
    the allowlist. Same procedure as T006's gap, and the same `PROGRESS.md` entry covers both.
  - **Guardrails**: no new `Controller` method (FR-003) — `List` is the row, which is why
    `k8s/internal/podctl` is untouched. Do not add a liveness field to `Session`, do not cache
    the answer, and do not poll: one read, now, under the lock.

- [ ] T008 tmuxctl fake: add `ExitAfterPaste(name)`, which sets the pane command to `bash` after the next `Paste` or `PasteBracketed` on that session. Test in `internal/tmuxctl/fake_test.go`.
  - **Files**: `internal/tmuxctl/fake.go`, `internal/tmuxctl/fake_test.go`.
  - **Implements**: the hook SC-002 rests on.
  - **Change**: copy the shape of `QuitAfterInterrupts` (`fake.go:386`), which is the existing
    hook of exactly this kind: a `bool` on `fakeSession` set by the exported helper, consumed
    in `Paste` (`:405`) and `PasteBracketed` (`:427`) after the two calls are recorded, setting
    `s.paneCommand = "bash"` and clearing itself so it fires once. It seeds the session if it
    does not exist, as `SetPane` and `QuitAfterInterrupts` do. Its comment says what state it
    reproduces — the harness exited between the paste and the submit — and that it drives the
    liveness the fake already derives (`livenessOf` against `OptionBinary`) rather than
    introducing a second notion of liveness.
  - **Failing-before test**: `TestExitAfterPaste` in `fake_test.go` — a table: after
    `ExitAfterPaste(name)`, a `Paste` records its two calls and a following `List` reports
    `LivenessStopped`; the same for `PasteBracketed`; a second paste does **not** re-arm it;
    and `List` before any paste still reports `LivenessRunning`. Against the unchanged code it
    fails to compile (`f.ExitAfterPaste undefined`).
  - **Verify**: `go test ./internal/tmuxctl/ -run ExitAfterPaste` exits 0.
  - **Depends**: T007.
  - **Guardrails**: `internal/tmuxctl/controller.go` and `exec.go` are not touched — this is a
    test hook on the fake and nothing about the real controller changes. The recorded calls
    and their argv are unchanged: `TestLivenessOfSet`, the argv tables in `fake_test.go` and
    `TestListFormatFieldCount` must all stay green unedited.

- [ ] T009 session: re-check liveness after the paste. If stopped, send `C-u`, send no Enter, return `ErrSessionNotReady` (D2). Test with `ExitAfterPaste` and assert the recorded keys.
  - **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`.
  - **Implements**: FR-002. Asserted by SC-002.
  - **Change**: after the paste and before `SendKeys(ctx, name, enterKey)` (`manager.go:1220`),
    read the same liveness again with T007's helper. If it is `LivenessStopped` — or the row
    has gone — send one `C-u` through a new daemon constant beside `enterKey`
    (`manager.go:39`) and `interruptKey` (`:80`), send **no** Enter, and return
    `ErrSessionNotReady`. A failure of the `C-u` itself is wrapped and returned; the refusal is
    still a refusal. `LivenessUnknown` submits, as before. The comment says why `C-u` and not
    nothing: the pasted bytes are sitting on a login shell's line, and a later stray Enter —
    the operator's, or a key the panel sends — would run them. `C-u` clears that line.
  - **Failing-before test**: `TestPromptClearsTheLineWhenTheHarnessExitsMidDelivery` in
    `manager_test.go` — with `ExitAfterPaste` armed, `Prompt` returns `ErrSessionNotReady`; the
    recorded calls are the two paste calls, then one `send-keys` whose last argument is `C-u`,
    and **no** `send-keys` with `Enter`; plus a row proving the ordinary path still ends in
    exactly one `Enter` and no `C-u`. Against the unchanged code the `Enter` is recorded and
    the error is nil; note that line. **This is req 1's row** in PROGRESS.md's failing-before
    table, and it covers T007's first check too.
  - **Verify**: `go test ./internal/session/ -run Prompt` exits 0.
  - **Depends**: T008 — the case cannot be produced without the hook.
  - **Guardrails**: `C-u` is a **daemon constant** typed only here. Do not add it to
    `internal/session/input.go`'s key table (`:35`, `:43`) — that table is the browser's closed
    set of keys, and widening it would add a key to the session page's bar that nobody asked
    for. One `C-u`, no retry, no loop.

**Checkpoint**: User Story 1 is whole. A dead harness is never typed into, and a harness that
dies mid-delivery leaves nothing on the line.

---

## Phase 6: The lock and the claim — User Story 5 (Priority: P2), requirement 5

**Goal**: a prompt never interleaves with a restart, and two prompts never interleave with each
other.

**Independent test**: hold `claimRestart` for a session, call `Manager.Prompt`, get
`ErrSessionNotReady` with no tmux command recorded; release it and the same call delivers.

- [ ] T010 session: `Prompt` takes the restart claim after `lockSession`; held means `ErrSessionNotReady` with no tmux command (D6). Add a two-goroutine test: distinct buffer names, paste then Enter never interleaved.
  - **Files**: `internal/session/manager.go`, `internal/session/manager_test.go`.
  - **Implements**: FR-012, FR-013. Asserted by SC-006 and SC-007.
  - **Change**: in `Prompt`, immediately after `unlock := m.lockSession(s.ID)` and its
    `defer unlock()` (`manager.go:1209-1210`):
    `if !m.claimRestart(s.ID) { return fmt.Errorf("prompt session %s: %w", s.ID,
    ErrSessionNotReady) }` followed by `defer m.releaseRestart(s.ID)`, so it is released on
    every return path including the two liveness refusals and a tmux failure. `claimRestart`
    (`:2674`) is non-blocking and already lazily creates its map, so a `Manager` built by a
    struct literal still works. The claim goes **after** the lock and **before** both liveness
    reads, because what those reads assert — nothing else is about to type here — is only true
    while both are held.
  - **Failing-before test**: `TestPromptRefusesWhileARestartHoldsTheClaim` in `manager_test.go`
    — the test takes `claimRestart(s.ID)` itself, calls `Prompt`, gets `ErrSessionNotReady`
    with zero recorded calls, then releases it and the same call delivers; plus a row proving
    the claim is free after a prompt returns, and after a prompt that was refused by the
    liveness gate. And `TestTwoConcurrentPromptsDoNotInterleave` — two goroutines prompting one
    session, asserting every `load-buffer` carries a distinct buffer name and that each paste
    pair is followed by its own `Enter` with no call from the other prompt in between. Against
    the unchanged code the first test's `Prompt` delivers while the claim is held; note that
    line. **This is req 5's row** in PROGRESS.md's failing-before table.
  - **Verify**: `go test -race ./internal/session/ -run Prompt` exits 0.
  - **Depends**: T009.
  - **Guardrails**: `lockSession` (`internal/session/input.go`) and `newBufferName`
    (`internal/tmuxctl/exec.go`, mirrored by the fake's `nextBuffer`) already exist — FR-013
    says **assert** them, not rebuild them, and `exec.go` is not on the allowlist. Do not make
    the claim blocking, and do not add a retry or a backoff: a refusal the caller can retry is
    the answer D6 chose. Check `TestSessionOperationsWaitForTheLifecycleLock`
    (`internal/session/harness_test.go:674`) still passes — it holds the lifecycle lock, so
    `Prompt` never reaches the claim and it should need no edit; if it goes red, say so in
    `PROGRESS.md` rather than editing a file the allowlist does not name.

**Checkpoint**: every gate requirements 1 to 6 ask for is in place.

---

## Phase 7: Proof and documentation

**Goal**: the traffic this route exists to carry still passes every gate; the one property only
a real tmux can show is shown; the rules are written where the next author reads them.

- [ ] T011 httpapi: fixtures in `internal/httpapi/testdata/lawnmower/` for answer, resume, opp-lease batch, opp follow-up (texts per PROGRESS.md "Payload fixtures"). A test named with `Lawnmower` posts each and gets 202 delivered.
  - **Files**: `internal/httpapi/lawnmower_payload_test.go` (new),
    `internal/httpapi/testdata/lawnmower/answer.txt`,
    `internal/httpapi/testdata/lawnmower/resume.txt`,
    `internal/httpapi/testdata/lawnmower/opp-lease-batch.txt`,
    `internal/httpapi/testdata/lawnmower/opp-followup.txt` (all new).
  - **Implements**: SC-009, and the regression half of FR-008 and FR-010.
  - **Change**: write each of the four blocks in `PROGRESS.md`'s "Payload fixtures" section
    byte for byte, without the fence, to the named file. They follow lawnmower's own builders
    as of 10/14/26 and keep the shapes that matter: multi-line, a first character that is a
    letter, double quotes, `$`, `;`, `&`, a backtick, and — in `resume.txt` — a **later** line
    that starts with `/`, which is FR-010's case as real traffic rather than as a contrived
    row. A fixture is data: read it with `os.ReadFile` and post it unaltered. Do not trim, do
    not normalise newlines, do not re-wrap.
  - **Failing-before test**: `TestLawnmowerPayloadsAreDelivered` in
    `lawnmower_payload_test.go` — a table over the four files (read from `testdata/lawnmower/`,
    a subtest per file): each posts to the API prompt route on `newPromptFixture` and asserts
    `202` with `delivered: true`, that the bytes reached tmux on **stdin** byte for byte, that
    they appear in no argv, and that the trail carries none of them. Against the tree before
    this task the fixtures do not exist, so it fails on `os.ReadFile`; that is the failing-
    before evidence, and it is the honest one for a test whose subject is the data.
  - **Verify**: `go test ./internal/httpapi/ -run Lawnmower` exits 0.
  - **Depends**: T010 — the four real payloads are run through every gate once they all exist.
  - **Guardrails**: the fixtures hold no credential, no host path outside the examples already
    in `PROGRESS.md`, and no real inbox id — the ids in them are the `01M3TEST…` placeholders.
    Nothing in this task changes production code.

- [ ] T012 session tmux test: new file prompt_tmux_test.go in `internal/session/` (`//go:build tmux`, private `-L` socket, pattern from `mode_test.go`). Pane on `sh` with `@crswd-binary` claude is refused and stays empty; a two-line prompt into `cat` arrives once.
  - **Files**: `internal/session/prompt_tmux_test.go` (new).
  - **Implements**: SC-010 — the half of FR-001 and FR-007 only a real tmux can answer, because
    the fake interpolates the same constants it reads.
  - **Change**: new file, `//go:build tmux`, in `package session`, following
    `internal/session/mode_test.go` exactly for its setup: a private `-L` socket so the
    cleanup's `kill-server` can only ever reach sessions these tests made, no `t.Parallel`, and
    a start command that is **not** Claude — a suite that typed the daemon's own default into a
    real shell would start an unsandboxed assistant on whatever host ran it.
  - **The two cases**:
    - A pane sitting on `sh` whose `@crswd-binary` is set to `claude`: `Manager.Prompt` is
      refused with `ErrSessionNotReady`, and a `capture-pane` afterwards shows the pane holds
      none of the prompt's bytes. This is the gate against real tmux rather than against
      `livenessOf`.
    - A pane running `cat` whose `@crswd-binary` is set to `cat`: a two-line prompt is
      delivered, and the pane shows both lines **once** — one bracketed paste, one Enter, no
      duplicate and no truncation at the newline.
  - **Failing-before test**: the file itself. Run the Verify command before T006, T007 and T009
    are in and the refusal case delivers; by the time this task is taken they are in, so the
    honest evidence is the first run of this file against the committed tree, recorded with
    whichever assertion fails if any does. If both cases pass on the first run, say so in
    `PROGRESS.md` — this task's value is the real-tmux proof, and its requirement rows are
    already carried by T009 and T006.
  - **Verify**: `go test -tags tmux ./internal/session/ -run Prompt` exits 0.
  - **Depends**: T011. Last of the code tasks because it is the slowest and the most
    environment-dependent — it needs `tmux` on `PATH`. If tmux is absent, that is an outside
    cause: follow the loop's "task cannot be finished" procedure rather than weakening the
    test.
  - **Guardrails**: no `t.Parallel` in this file, and it is the only file in this spec exempt
    from the fake (FR-021). It touches no production code.

- [ ] T013 docs: add a "Prompt route gates (spec 020)" subsection under "Non-negotiables" in `docs/security.md` naming the liveness gate, failed refusal, bracketed paste, leading-character rule, restart claim and `request_id`.
  - **Files**: `docs/security.md`.
  - **Implements**: the documentation half of principle V — the rules land as code and tests,
    and this subsection *describes* what those tests enforce.
  - **Change**: one new subsection under "Non-negotiables" (the section opens at
    `docs/security.md:216`; place it after §2 "Validate every input at the boundary", whose
    bullet on `load-buffer`/`paste-buffer` and on the daemon never typing on its own initiative
    is what this extends). Six paragraphs or bullets, each naming the gate **and the reason it
    exists**, in that file's voice:
    - the liveness gate, before the paste and again before the Enter, and the `C-u` that
      clears a half-delivered line rather than leaving it for a stray Enter;
    - the `failed` refusal, and that `dead` keeps the uniform `404` it has always had;
    - bracketed paste for every harness on this route, amending spec 018 FR-006, with
      `Compact` unchanged;
    - the leading `!`/`/` rule, that it is the **API door only**, and that the session page's
      panel still accepts `/compact` because the operator is a human at their own keyboard;
    - the restart claim, non-blocking, and that a refusal the caller can retry is the answer;
    - `request_id`: optional, 1 to 32 characters of `[a-z0-9-]`, echoed and audited, **and the
      bound is the justification** — it amends spec 001 FR-042 for this one field and nothing
      else, so prompt text, its length and a hash of it stay forbidden everywhere.
  - **Failing-before test**: there is none, and that is correct for a documentation task. The
    Verify command is the check: `grep -q 'spec 020' docs/security.md` must find the
    subsection, and `test "$(wc -l < AGENTS.md)" -lt 150` must hold — `AGENTS.md` is 147 lines
    and CI fails at 150, which is why this content goes in `docs/security.md` and why that file
    is not edited here.
  - **Verify**: `grep -q 'spec 020' docs/security.md && test "$(wc -l < AGENTS.md)" -lt 150`
    exits 0.
  - **Depends**: T012 — documentation describes behaviour that is finished.
  - **Guardrails**: `AGENTS.md` is not edited, by this task or any other. Add no new
    checklist item that the tests do not already assert, and do not restate the whole feature:
    six gates, each with its reason.

**Checkpoint**: F1 is the notebook's final gate — both suites, the pre-commit commands, and one
failing-before row for each of requirements 1 to 6.

---

## Dependencies & Execution Order

The notebook's order is the dependency order, and the loop takes the topmost open task. Nothing
here runs in parallel: one iteration, one task, one commit.

| Task | Depends on | Why that order |
|---|---|---|
| P1–P5, G1 | — | Markdown only. G1 gates every Go change on the spec having no open question. |
| T001 | G1 | The record's field has to exist before anything can set it. |
| T002 | T001 | `SetRequestID` writes the field T001 added. |
| T003 | T002 | The handler needs the setter before it can carry a value to the trail. |
| T004 | T003 | Both are decode-boundary refusals in the same handler. |
| T005 | T004 | The sentinel, the first refusal that raises it, and the handler's `409`. |
| T006 | T005 | Changes delivery, with its own failing-before test. |
| T007 | T006 | The pre-paste gate, asserted against the paste shape T006 settled. |
| T008 | T007 | The fake hook T009 needs. |
| T009 | T008 | The post-paste gate, untestable without T008. |
| T010 | T009 | The claim, and the `-race` assertion of the lock and the buffer names. |
| T011 | T010 | The four real payloads, through every gate once they all exist. |
| T012 | T011 | Real tmux, last: slowest and most environment-dependent. |
| T013 | T012 | Documentation describes behaviour that is finished. |
| F1 | T013 | Both suites, the pre-commit commands, the six evidence rows. |

### Requirement coverage

| Design requirement | Tasks | FRs | Failing-before row |
|---|---|---|---|
| 1 — liveness gate | T007, T008, T009 | FR-001 to FR-003 | T009 (covers T007's first check) |
| 2 — not-ready answer | T005 | FR-004 to FR-006 | T005 |
| 3 — bracketed paste | T006 | FR-007, FR-008, FR-017 | T006 |
| 4 — leading `!` or `/` | T004 | FR-009 to FR-011, FR-017 | T004 |
| 5 — lock and claim | T010 | FR-012, FR-013 | T010 |
| 6 — `request_id` | T001, T002, T003 | FR-014, FR-015, FR-018 | T003 |
| 7 — registry gate | none, deferred | Out of scope (research D8) | — |
| — the contract | P5 | FR-019 | — |
| — the bounds | every task | FR-016, FR-020, FR-021 | — |

## Notes

- One task per iteration, each ending in its own commit with `IMPLEMENTATION_PLAN.md` and
  `PROGRESS.md` in the path list.
- Write the test first, run the Verify command, confirm it fails and note the line, then change
  the code and run it again. A test that cannot fail is not a test.
- Two tasks (T006, T007) need an assertion in `internal/session/harness_test.go` to change, and
  that path is not on the allowlist. Both entries say so, and neither may be worked around by
  editing an unlisted file or by deleting a test.
- `golangci-lint` is absent on the loop's host (`PROGRESS.md` "Findings"). An iteration that
  cannot run it records the gap; the milestone's `VALIDATION_CONTRACT.md` still requires it
  green before the PR.
