# Feature Specification: Prompt Route Safety

**Feature Branch**: `plan/lm-steer-crswd-018`

**Created**: 2026-10-14

**Status**: Draft (decisions recorded in [research.md](research.md) D1–D9)

**Depends on**: spec 018 (interactive input) and spec 019 (Codex runtime), both merged to `main`.

**Input**: `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`, section (g) "Phase 3
detail", requirements 1 to 7. Evidence from the probe run in
`~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`. Both documents live outside
this repository and are read, never edited.

## What this is

`POST /sessions/{id}/prompt` is the one route that delivers caller-authored text into a
running session, and lawnmower already drives it: `answer`, `resume`, the opp-lease batch
handoff and the opp follow-up all arrive here. Today the route pastes and presses Enter
with no question asked about whether anything is listening, which means the failure mode
is silent — text typed at a dead harness lands on a login shell, and text typed beside a
restart lands in the middle of a command line.

This spec closes those gaps on the route as it stands. It adds no route, no package, no
`Controller` method and no dependency; every change lands on the existing prompt route,
`internal/session`'s `Manager.Prompt`, the audit record and the `tmuxctl` fake.

Requirements 1 to 6 of the design are built. Requirement 7 is deferred with a recorded
reason (Out of scope, below). Every decision this spec rests on was taken before it was
written and is reproduced with its evidence in [research.md](research.md); none is
reopened here, and no open question remains in this document.

## Non-goals

- **Deciding readiness from Claude Code's session registry.** Requirement 7's
  `~/.claude/sessions/<pid>.json` gate is deferred (research D8). See Out of scope.
- **Opening a transcript.** The no-transcript rule holds; spec 019 D11's bounded Codex
  exception is that spec's and is not widened.
- **Holding or calling an LLM.** This daemon delivers text; it does not author it.
- **A seventh API route.** The API door has six and keeps six.
- **Free text on the browser door beyond spec 018.** The session page's input panel keeps
  its behaviour exactly, including its acceptance of a leading `/`.
- **The startup window in which an Enter can be lost** (probe check 1), and **the socket
  transport** (the probe's own verdict, which binds the lm-steer-write milestone).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A prompt into a session whose harness has exited is refused (Priority: P1)

Lawnmower delivers an answer to a session whose Claude process exited some minutes ago.
The pane is a login shell. Today the text is pasted into that shell and Enter runs it as a
command line; the caller is told `delivered: true` and nothing anywhere records that the
conversation never saw it. Instead the daemon reads the pane's liveness, refuses, and says
so, having typed nothing.

**Why this priority**: it is the one failure in which a refused-looking success causes
execution the operator never asked for. Requirement 1 of the design.

**Independent Test**: with the fake controller's pane command set to something other than
the session's `@crswd-binary`, `Manager.Prompt` returns `ErrSessionNotReady`, and the fake
records no `load-buffer`, no `paste-buffer` and no `send-keys`.

**Acceptance Scenarios**:

1. **Given** a running session whose pane command no longer matches `@crswd-binary`,
   **When** a prompt is delivered, **Then** it is refused with `ErrSessionNotReady` and
   nothing is typed into the pane.
2. **Given** a session whose liveness reads unknown — one started before `@crswd-binary`
   existed — **When** a prompt is delivered, **Then** it is delivered, exactly as spec 012
   FR-006 reads that value everywhere else.
3. **Given** a controller whose `List` fails, **When** a prompt is delivered, **Then** the
   call is an error and nothing is typed. The gate fails closed.
4. **Given** a session whose harness exits *between* the paste and the submit, **When** the
   second liveness read runs, **Then** the daemon sends `C-u`, sends no Enter, and answers
   `ErrSessionNotReady`, so the half-delivered text is cleared off the shell's line rather
   than executed by it.

---

### User Story 2 - A session that cannot take a prompt says so (Priority: P1)

Lawnmower delivers into a session the supervisor has marked `failed`. It gets a `409`
carrying `{"error":"session not ready"}`, which its own `deliver()` already reads as a
refusal, so it retries or escalates instead of believing the text arrived.

**Why this priority**: the caller's only existing signal is `delivered`, and a `500` is
indistinguishable from a host fault it should not retry. Requirement 2 of the design.

**Independent Test**: post a prompt for a `failed` session through the API door; the
response is `409` with that exact body, and the fake records no tmux command.

**Acceptance Scenarios**:

1. **Given** a session in state `failed`, **When** a prompt is delivered, **Then** the
   answer is `409` with body `{"error":"session not ready"}` and no tmux command ran.
2. **Given** any refusal that `ErrSessionNotReady` carries — failed state, stopped
   liveness, a held restart claim — **When** the caller reads the answer, **Then** it is the
   same `409` and the same body, and which gate refused is distinguishable only on the
   audit trail.
3. **Given** a session in state `dead`, **When** a prompt is delivered, **Then** the answer
   is the one it is today, unchanged: the resolver's uniform `404`.
4. **Given** a refused prompt, **When** the trail is read, **Then** the request's one audit
   record is a Deny whose reason is `session not ready` and which carries no prompt text.

---

### User Story 3 - A multi-line prompt arrives once, as one prompt (Priority: P1)

The opp-lease batch handoff is a dozen lines with blank lines in it. It must reach the
harness as one paste that one Enter submits, not as a dozen submissions of which the first
eleven are half-formed questions.

**Why this priority**: it is the shape of the traffic this route carries. Requirement 3 of
the design.

**Independent Test**: `Manager.Prompt` on a Claude session records `paste-buffer -p -d`
(bracketed) and then exactly one `send-keys Enter`; a two-line text records the same two
calls and no more.

**Acceptance Scenarios**:

1. **Given** a Claude session, **When** a prompt is delivered, **Then** it is pasted
   bracketed and submitted by one Enter.
2. **Given** a Codex session, **When** a prompt is delivered, **Then** it is pasted
   bracketed and submitted by one Enter, as spec 019 FR-006 already requires.
3. **Given** a prompt holding a newline, **When** it is delivered, **Then** it is not
   refused for holding one.
4. **Given** a Compact, **When** it is delivered, **Then** its paste is the harness-based
   one it is today and this spec changes nothing about it.

---

### User Story 4 - A prompt that would not be a prompt is refused (Priority: P2)

A caller's text begins with `/` or `!`. Delivered as-is it is a Claude Code slash command
or its shell escape, so a steering message silently becomes `/clear` and the conversation
it was steering is gone, with no error anywhere. The API door refuses it instead, before
anything is typed.

**Why this priority**: the damage is unbounded and invisible, but no current caller sends
such a text, so this is a guard rather than a fix. Requirement 4 of the design.

**Independent Test**: post `"/clear"` and `"  \n!rm -rf /"` to the API prompt route; each
is `400` with the uniform bad-request body and the fake records no tmux command. Post
`/compact` through the browser type route; it is delivered.

**Acceptance Scenarios**:

1. **Given** a text whose first character that is not a space, tab, CR or LF is `/` or `!`,
   **When** it is posted to the API prompt route, **Then** the answer is `400` with the
   uniform bad-request body and no tmux command ran.
2. **Given** a text whose first non-whitespace character is a letter but a **later line**
   starts with `/` — the `resume` fixture is one — **When** it is posted, **Then** it is
   delivered. The rule reads the first non-whitespace character of the whole text and never
   a line.
3. **Given** the session page's input panel, **When** the operator types `/compact` and
   submits, **Then** it is delivered. The operator is a human at their own keyboard and this
   refusal is not theirs.

---

### User Story 5 - A prompt never interleaves with a restart (Priority: P2)

A revive or a Continue is typing a command line into the pane. A prompt pasted into the
middle of that line is neither a prompt nor a command. The prompt is refused while the
restart holds its claim, and the caller may retry.

**Why this priority**: the window is narrow and the outcome is a corrupted command line on
a session running unsandboxed. Requirement 5 of the design.

**Independent Test**: hold `claimRestart` for a session, call `Manager.Prompt`, and get
`ErrSessionNotReady` with no tmux command recorded; release it and the same call delivers.

**Acceptance Scenarios**:

1. **Given** a revive or a Continue holding the session's restart claim, **When** a prompt
   is delivered, **Then** it is refused with `ErrSessionNotReady` and no tmux command ran.
2. **Given** a prompt that has taken the claim, **When** it returns — delivered or refused
   — **Then** the claim is released.
3. **Given** two prompts to one session at once, **When** both run, **Then** each paste
   uses a buffer name of its own and no paste is followed by the other call's Enter.

---

### User Story 6 - A caller can find its own delivery on the trail (Priority: P3)

Lawnmower delivers a prompt and then reads the audit trail to find out what happened to it.
It sends a correlator it chose; the `202` echoes it and the trail carries it, so two
deliveries to one session in one second are told apart by something other than the clock.

**Why this priority**: it is an observability affordance, not a safety gate, and the route
is correct without it. Requirement 6 of the design.

**Independent Test**: post a prompt with `"request_id":"abc-123"`; the `202` body carries
`"request_id":"abc-123"` and the request's one audit record carries the same value under
`request_id`. Post one with `"request_id":"AB C"`; it is `400` and no tmux command ran.

**Acceptance Scenarios**:

1. **Given** a body with a valid `request_id`, **When** the prompt is delivered, **Then**
   the `202` echoes it and the request's one audit record carries it.
2. **Given** a body with a `request_id` that is empty, longer than 32 characters, or holds a
   character outside `[a-z0-9-]`, **When** it is posted, **Then** the answer is `400` and no
   tmux command ran.
3. **Given** a body with no `request_id`, **When** the prompt is delivered, **Then** the
   field appears nowhere: not in the response, not on the audit record. A caller that sends
   none sees byte-for-byte the response it sees today.
4. **Given** any prompt at all, **When** the response, the trail and the logs are read,
   **Then** none of them carries the prompt text, its length, or a hash of it.

### Edge Cases

- A prompt whose text is empty: `400` with `ErrEmptyPrompt`, as today. Unchanged.
- A prompt that is nothing but whitespace: the leading-character rule finds no first
  non-whitespace character, so it does not refuse; emptiness and the length bound are the
  checks that apply, exactly as today.
- A text whose first non-whitespace character is `!` or `/` **and** whose session is not
  ready: the `400` wins, because it is decided in the handler before `Manager.Prompt` runs.
- A session whose harness exits after the Enter: outside this spec. Delivery is confirmed to
  the pane, never to the harness, and `202` has always said so.
- `LivenessUnknown` on both reads: delivered on both, per User Story 1 scenario 2.
- A `List` that succeeds but does not hold the session's row: it cannot be read as alive, so
  it is refused with `ErrSessionNotReady`.
- A restart claim taken by this prompt and a *second* prompt arriving: the second waits on
  `lockSession` as it does today, then finds the claim free again.
- `request_id` sent on the browser routes: there is no such field there, and an unknown
  field is already refused by `DisallowUnknownFields`.

## Requirements *(mandatory)*

### Liveness (requirement 1)

- **FR-001**: Before any paste, `Manager.Prompt` MUST read the session's row from the
  controller's `List` and consult its `Claude` liveness. `LivenessStopped` MUST return
  `ErrSessionNotReady` with no tmux command issued. `LivenessUnknown` MUST deliver. A row
  that is absent MUST be refused. A failing `List` MUST be returned as an error, not read as
  a pass.
- **FR-002**: After the paste and before the Enter, `Manager.Prompt` MUST read that liveness
  again. If it reads stopped, the daemon MUST send `C-u`, MUST NOT send Enter, and MUST
  return `ErrSessionNotReady`.
- **FR-003**: The gate MUST NOT add a method to `tmuxctl.Controller`. `List` is the row it
  reads, so `k8s/internal/podctl` is untouched by this spec.

### Readiness and its one answer (requirements 1, 2, 5)

- **FR-004**: `internal/session` MUST define one sentinel, `ErrSessionNotReady`, carrying
  every "this session cannot take a prompt right now" refusal in this spec.
- **FR-005**: `Manager.Prompt` MUST refuse a session in state `failed` with
  `ErrSessionNotReady` before any tmux command. A session in state `dead` MUST keep the
  answer it gives today (`ErrSessionDead`).
- **FR-006**: The prompt handler MUST map `ErrSessionNotReady` to `409` with body
  `{"error":"session not ready"}` and MUST record an audit Deny whose reason is
  `session not ready`. The answer MUST be identical for every gate that raises the sentinel;
  which one refused is distinguished only server-side, on the trail.

### Delivery (requirement 3)

- **FR-007**: `Manager.Prompt` MUST paste with `Controller.PasteBracketed` for every
  harness, then submit with one Enter. `Manager.Compact` MUST keep its harness-based paste
  unchanged.
- **FR-008**: Prompt text MUST NOT be refused for holding a newline. The byte checks that
  apply are the ones already in force (spec 018 FR-004's C0 rule, the body bound, UTF-8).

### The first character (requirement 4)

- **FR-009**: On the API door's prompt route only, a prompt whose first character that is not
  a space (`0x20`), tab (`0x09`), CR (`0x0D`) or LF (`0x0A`) is `!` or `/` MUST be refused
  with `400` and the uniform bad-request body, before any tmux command is issued.
- **FR-010**: The rule MUST read the first non-whitespace character of the whole text and
  MUST NOT read the start of any later line.
- **FR-011**: The session page's input routes MUST be unchanged by FR-009, and a test MUST
  prove the type route still delivers `/compact`.

### The lock and the claim (requirement 5)

- **FR-012**: `Manager.Prompt` MUST take the restart claim (`claimRestart`, non-blocking)
  immediately after `lockSession`, MUST refuse with `ErrSessionNotReady` and no tmux command
  when a revive or a continue already holds it, and MUST release it on every return path.
- **FR-013**: Two concurrent prompts to one session MUST each paste under a buffer name of
  its own, and a paste MUST NOT be separated from its own Enter by the other call's commands.
  Both properties exist today (`lockSession`, `newBufferName`); this spec MUST assert them
  rather than assume them.

### Correlation (requirement 6)

- **FR-014**: The prompt body MUST accept one optional field, `request_id`, of 1 to 32
  characters drawn from `[a-z0-9-]`. A value outside that MUST be `400` with no tmux command
  issued.
- **FR-015**: A valid `request_id` MUST be echoed in the `202` body and MUST be written as
  `request_id` on the request's one audit record. Absent, the field MUST appear nowhere — not
  in the response, not on the record.
- **FR-016**: No response, error, log line or audit record may carry the prompt text, its
  length, or a hash of it. `docs/security.md` §3 stands unchanged.

### Amendments to specs already merged

- **FR-017**: This spec **amends spec 018 FR-006** ("The API door's `POST
  /sessions/{id}/prompt` MUST be unchanged: plain paste, then Enter. Bracketed paste is for
  the interactive route only") in exactly two places: FR-007 moves the API route to bracketed
  paste for every harness, and FR-009 adds the leading-character refusal to it. Everything
  else 018 froze about that route — one `text` field, no session field, `202` meaning
  delivery to the pane — is unchanged, and spec 018's own interactive routes are untouched.
- **FR-018**: This spec **amends spec 001 FR-042** ("Audit records MUST NOT contain prompt
  text, session output, session credentials, or the shared secret") for the one field in
  FR-014 and no other. The bound is what makes it safe: 32 characters of `[a-z0-9-]` cannot
  carry prompt text, a sentence, a path or a credential. Prompt text, session output,
  credentials and the secret remain forbidden, asserted by test.
- **FR-019**: `specs/001-crswd-daemon-core/contracts/http-api.md` MUST record `request_id`,
  the `409` not-ready answer and the FR-009 refusal on the prompt route, and MUST still carry
  exactly six route headings.

### Bounds this spec keeps (restated so a change cannot drift past them)

- **FR-020**: No seventh API route. No new package under `internal/`. No new method on
  `tmuxctl.Controller`. No new dependency, and the root `go.sum` MUST stay absent.
- **FR-021**: Every new test MUST be table-driven, MUST call `t.Parallel()`, and MUST use the
  fake controller except in the one `//go:build tmux` file this spec adds.

### Out of scope

- **Requirement 7 — the session registry gate.** Consulting Claude Code's
  `~/.claude/sessions/<pid>.json` to decide whether a session is ready is **not built**. It
  was a MAY in the design, and the probe gives two reasons to defer it: check 8 left open
  whether crswd can read that file at all under spec 017's v2 option B, so a gate reading it
  would work on the VM and answer nothing in Kubernetes; and checks 3 and 9 found the registry
  misses both the trust dialog and a half-typed draft, so it does not answer the question
  FR-001's liveness gate answers and would pass a session that cannot take a prompt. A
  deferred MAY with a recorded reason leaves no open question and nothing downstream waits on
  it (research D8).
- Everything under Non-goals above (research D9).

## Success Criteria *(mandatory)*

- **SC-001**: A test proves that a prompt into a session whose pane command does not match
  `@crswd-binary` is refused and that the fake recorded no `load-buffer`, no `paste-buffer`
  and no `send-keys`. It fails against the code before FR-001.
- **SC-002**: A test using the fake's `ExitAfterPaste` proves that a harness exiting between
  the paste and the submit yields a recorded `C-u`, no recorded Enter, and
  `ErrSessionNotReady`.
- **SC-003**: A test proves a `failed` session answers `409` with
  `{"error":"session not ready"}`, and that the request's audit record is a Deny reading
  `session not ready`.
- **SC-004**: A test proves a Claude session's prompt is pasted bracketed, for a one-line and
  a two-line text, and that `Compact`'s paste is unchanged.
- **SC-005**: A table test proves the FR-009 refusal for `/…`, `!…`, and the same two behind
  leading whitespace; proves delivery for a text whose first non-whitespace character is a
  letter and a later line starts with `/`; and proves the browser type route still delivers
  `/compact`.
- **SC-006**: A `-race` test with two goroutines prompting one session proves distinct buffer
  names and that no paste is separated from its own Enter.
- **SC-007**: A test proves a held restart claim refuses a prompt with no tmux command, and
  that the claim is free after a prompt returns.
- **SC-008**: Tests prove a valid `request_id` is echoed and audited, that each invalid shape
  is `400` with no tmux command, and that an absent one appears in neither the response nor
  the record. `internal/audit`'s leak test still passes unchanged.
- **SC-009**: A test posts each of the four lawnmower payload fixtures
  (`internal/httpapi/testdata/lawnmower/`) through the API prompt route and gets `202` with
  `delivered: true` — the traffic this route exists to carry still passes every gate added
  here.
- **SC-010**: A `//go:build tmux` test on a private `-L` socket proves that a pane sitting on
  `sh` with `@crswd-binary` of `claude` is refused and the pane stays empty, and that a
  two-line prompt into `cat` arrives once.
- **SC-011**: `go test ./...`, `go test -tags tmux ./...`, `go vet` on the default, `tmux` and
  `quickstart` builds, `gofmt -l .` and `golangci-lint run` are all green, and `go.sum` does
  not exist.

## Assumptions

- `@crswd-binary` is a usable liveness answer on the panes lawnmower drives. Probe check 13
  matched it against `pane_current_command` on 6 of 6 live panes, 3 of them under `rcq`.
- Lawnmower's `deliver()` treats any body carrying an `error` key as a refusal, so the `409`
  in FR-006 needs no change on that side.
- Every lawnmower prompt builder's text begins with a letter (grep of 10/14/26), so FR-009
  refuses none of today's traffic.
- `lockSession`, `claimRestart`/`releaseRestart` and `newBufferName` already exist and behave
  as research D6 records; this spec adds the claim to `Prompt` and asserts the other two.
- Spec 012 FR-006's reading of `LivenessUnknown` as alive — written on the
  `tmuxctl.Liveness` type itself and already relied on by `internal/session/supervisor.go` —
  continues to hold.
