# Feature Specification: Prompt Route Safety

**Feature Branch**: `plan/lm-steer-crswd-018`
**Created**: 2026-10-14
**Status**: Draft
**Input**: `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`, section (g)
"Phase 3 detail", requirements 1 to 7. Evidence:
`~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`, numbered checks.
Both are outside this repository and are read-only; the facts they supply are
transcribed in [research.md](research.md) and in
`ralph/plans/lm-steer-crswd-018/PROGRESS.md`.

**Spec**: this file · **Research**: [research.md](research.md) ·
**Plan**: [plan.md](plan.md) · **Tasks**: [tasks.md](tasks.md)

`POST /sessions/{id}/prompt` is the one API route that delivers free text into a
running session. ai-lawnmower calls it today — a board answer, a requeue resume,
an opp-lease batch handoff, an opp-lease follow-up — so every requirement below
is a change to a live route with a live client. Nothing here adds a route,
a `tmuxctl.Controller` method or a package.

Every decision this specification rests on was taken before it was written and is
recorded with its evidence in [research.md](research.md) as D1 to D9. This file
states what must be true; research.md states why, and what was rejected.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A prompt into a session that is no longer running is refused, not executed (Priority: P1)

A script sends a prompt to a session whose Claude Code has exited while its tmux
window survived. Today the text lands in the login shell underneath and the Enter
after it runs the first line as a command, unsandboxed, as the daemon's user. The
caller is told `202 delivered`. After this feature the daemon refuses, nothing
reaches the pane, and the caller is told so in a form it already understands.

**Why this priority**: it is the one failure in this route that executes caller
text as a host command line. Everything else here is correctness; this is the
security gate.

**Independent Test**: put a session's pane on a shell with `@crswd-binary` still
naming `claude`, POST a prompt, and read the pane back: `409`, and the pane is
empty.

**Acceptance Scenarios**:

1. **Given** a session whose pane command no longer matches `@crswd-binary`,
   **When** a prompt is posted, **Then** the answer is `409`
   `{"error":"session not ready"}`, no tmux command that changes anything has
   run, and the audit record is a Deny whose reason is `session not ready`.
2. **Given** a session whose Claude exits **between** the paste and the submit,
   **When** the prompt is delivered, **Then** the daemon sends `C-u`, sends no
   Enter, and answers `409`. Nothing runs in the shell.
3. **Given** a session whose liveness is unknown — created by a build older than
   `@crswd-binary` — **When** a prompt is posted, **Then** it is delivered, as the
   supervisor already treats that state as healthy.
4. **Given** a controller `List` that fails, **When** a prompt is posted, **Then**
   the answer is the route's existing `500` with no detail, and nothing is
   delivered. A readiness check that cannot run is not a readiness check that
   passed.

---

### User Story 2 - A prompt into a session the supervisor gave up on is refused (Priority: P1)

A session in state `failed` is one the supervisor tried to revive and ran out of
attempts for. Nothing is running in its pane. A prompt into it is the same failure
as User Story 1 reached by a different road, and it gets the same answer.

**Why this priority**: it is the same host exposure, and it is one comparison.

**Independent Test**: POST a prompt to a session recorded `failed`; the answer is
`409` and the fake controller recorded no command at all.

**Acceptance Scenarios**:

1. **Given** a session in state `failed`, **When** a prompt is posted, **Then**
   the answer is `409` `{"error":"session not ready"}` and no tmux command ran.
2. **Given** a session in state `dead`, **When** a prompt is posted, **Then** the
   answer is exactly what it is today — the resolver's uniform `404` — and this
   feature changes nothing about it.

---

### User Story 3 - A multi-line prompt arrives once, whole, as text (Priority: P1)

The opp-lease handoff sends a multi-line item body. Under a plain paste a harness
can submit line one early and treat the rest as a second turn. A bracketed paste
is the form both harnesses read as one block of text.

**Why this priority**: it is the correctness half of the requirement that the
route carries what the caller wrote, and lawnmower's real payloads are multi-line.

**Independent Test**: deliver a two-line prompt and assert one bracketed paste
followed by exactly one Enter — against the fake, and against a real tmux pane
running `cat`.

**Acceptance Scenarios**:

1. **Given** a Claude session, **When** a two-line prompt is delivered, **Then**
   the controller recorded one `PasteBracketed` and one Enter, in that order, and
   no plain `Paste`.
2. **Given** a Codex session, **When** a prompt is delivered, **Then** it is also
   bracketed. `Prompt` no longer asks which harness it is talking to.
3. **Given** a compact, **When** it is delivered, **Then** it still uses the
   harness-based paste it uses today. Its payload is a daemon constant.
4. **Given** a prompt holding `ESC [ 2 0 1 ~`, **When** it is posted, **Then** it
   is refused with `400` and nothing reaches the pane (FR-007).

---

### User Story 4 - A prompt that is really a command is refused (Priority: P2)

A client that meant to say something and instead sent `/clear` discards the
conversation it was trying to continue; one that sent `!rm -rf …` runs a shell
command in the session's own unsandboxed context. On the API door those two
leading characters are a client bug, and the daemon can see it.

**Why this priority**: it is cheap, it is decided at the boundary, and no measured
caller is refused by it. It ranks below the three above because a client has to
send it deliberately.

**Independent Test**: POST `"/clear"`; the answer is `400` and the fake recorded
nothing. POST the same text through the browser type route; it is delivered.

**Acceptance Scenarios**:

1. **Given** a prompt whose first non-whitespace character is `/` or `!`, **When**
   it is posted to the API door, **Then** the answer is `400` with the uniform
   bad-request body and no tmux command ran.
2. **Given** a prompt whose *later* line begins with `/`, **When** it is posted,
   **Then** it is delivered. The rule reads the first non-whitespace character of
   the whole text and nothing else.
3. **Given** the operator typing `/compact` into the session page's input panel,
   **When** they press Send, **Then** it is delivered unchanged. The browser door
   is not touched by this rule.

---

### User Story 5 - Two writers cannot interleave inside one pane (Priority: P2)

A prompt that arrives while a revive or a continue is typing a new command line
into the pane lands partly in the dying process and partly in the next one. The
session lock does not exclude a restart, because a restart does not hold it for
its whole run.

**Why this priority**: it is a real corruption, and it is rare — it needs a prompt
to arrive inside a restart's window.

**Independent Test**: hold the restart claim, POST a prompt, and see `409` with no
tmux command recorded.

**Acceptance Scenarios**:

1. **Given** a revive or continue holding the session's restart claim, **When** a
   prompt is posted, **Then** the answer is `409` `{"error":"session not ready"}`
   and no tmux command ran. The prompt does not wait for the restart.
2. **Given** two prompts to one session at the same moment, **When** both run,
   **Then** each uses its own tmux buffer name and neither one's Enter lands
   between the other's paste and Enter.

---

### User Story 6 - A caller can find its own delivery in the trail (Priority: P3)

A client that delivered a prompt and then lost its answer — a timeout, a daemon
restart — has no way to tell whether the text landed. The audit trail is the only
durable record of a delivery, and today it carries nothing the caller chose.

**Why this priority**: it is an operability feature, not a safety one, and it is
the only requirement here that adds a field rather than a refusal.

**Independent Test**: POST with `"request_id":"abc-123"`; the `202` echoes it and
the audit record carries it. POST a 33-character or upper-case one; `400`.

**Acceptance Scenarios**:

1. **Given** a body carrying a valid `request_id`, **When** the prompt is
   delivered, **Then** the `202` echoes the value and the request's one audit
   record carries it in a field named `request_id`.
2. **Given** a body carrying an invalid `request_id`, **When** it arrives,
   **Then** the answer is `400` and no tmux command ran.
3. **Given** a body carrying no `request_id`, **When** the prompt is delivered,
   **Then** the field appears nowhere — not in the response, not on the record.
   An existing client sees byte-for-byte the response it sees today.
4. **Given** a refused prompt that carried a valid `request_id`, **When** it is
   refused, **Then** the Deny record carries the same field.

---

### Edge Cases

- **A prompt holding `ESC [ 2 0 1 ~`.** Under today's plain paste the bytes are
  inert; under FR-005's bracketed paste they would end the paste early and the
  remainder would reach the pane as keystrokes. FR-007 refuses the payload, so it
  cannot arrive. This is the one exposure bracketing *creates*, and it is closed
  in the same change that creates it.
- **A prompt holding a bare `\r`.** Refused by FR-007 as a C0 control. This door
  does not CRLF-normalise the way the session page's route does: the API door's
  text is delivered byte-for-byte, and rewriting it here would deliver something
  the caller did not send. No measured caller sends one.
- **A prompt between 16 KiB and the body bound.** Delivered. FR-007 adopts the
  character rule and deliberately **not** the session page's 16384-byte bound; the
  prompt route keeps the body bound it has today.
- **A later line beginning `/` or `!`.** Delivered (FR-008). The `resume.txt`
  fixture holds a line beginning `/compact` for exactly this reason.
- **A pane that falls back to a shell between the second liveness read and the
  Enter.** Not covered, and not claimed to be: the window is two tmux commands
  wide and nothing measured it (research.md "Not verified").
- **A Codex composer meeting the `C-u` of FR-003.** `C-u` clears a Claude composer
  and a shell line; Codex's was not measured. The worst case is a composer left
  holding text that was never submitted, which is the state a refused prompt is
  supposed to leave behind.
- **A session parked on a dialog.** Still accepts a prompt. `parked_on` and
  `suspicious_dialog` keep reporting it in the `202` and keep not refusing — the
  readiness signal that would refuse it is out of scope (D8).
- **A `request_id` reused by the caller.** The daemon does not deduplicate. It is a
  correlation value the caller chose, not a key the daemon owns, and a route that
  remembered one would be keeping state about callers it deliberately keeps none
  about.

## Requirements *(mandatory)*

### Readiness

- **FR-001**: `Manager.Prompt` MUST read the session's row from the controller's
  `List` before any tmux command that changes anything, and branch on its `Claude`
  liveness: `LivenessStopped` refuses, `LivenessUnknown` and `LivenessRunning`
  deliver, and a `List` that fails is an error and never a delivery. It MUST NOT
  add a method to `tmuxctl.Controller`, so the `k8s/` module's `podctl` is
  untouched.
- **FR-002**: Readiness MUST NOT be decided by reading what the pane shows. Spec
  012 FR-007 forbids it for detection and pane content is secret under
  `docs/security.md` §3; `paneDialogState` stays what it is, additive information
  and never a gate.
- **FR-003**: `Manager.Prompt` MUST read liveness a second time after the paste and
  before the Enter. If that read is `LivenessStopped` it MUST send `C-u`, MUST NOT
  send Enter, and MUST refuse. No third read is required: the lost-Enter window
  measured by the probe is the first seconds after launch, which is out of scope.
- **FR-004**: `Manager.Prompt` MUST refuse a session in state `failed` before any
  tmux command. A session in state `dead` keeps today's answer unchanged.

### The refusal

- **FR-005**: `internal/session` MUST gain exactly one sentinel,
  `ErrSessionNotReady`, and every refusal FR-001, FR-003, FR-004 and FR-010 make
  MUST return it. `refusePrompt` MUST map it to `409` with body
  `{"error":"session not ready"}`, and the request's one audit record MUST be a
  Deny whose reason is the constant `session not ready`. That reason names no
  state, no pane content and no prompt. No other status, body or reason is added
  to this route.

### Delivery

- **FR-006**: `Manager.Prompt` MUST deliver with `PasteBracketed` for every
  harness — one paste, then at most one Enter. `Manager.Compact` MUST keep the
  harness-based paste it uses today; its payload is a daemon constant, not caller
  text. No rule refusing newlines may be added: the opp-lease handoff pastes
  multi-line bodies, and bracketing is what makes a newline safe.
- **FR-007**: Because FR-006 brackets this route's paste, the prompt text MUST be
  refused when it is not valid UTF-8, or holds any byte in `0x00–0x1F` other than
  `0x09` and `0x0A`, or holds `0x7F`. This is the rule `docs/security.md` §2
  already states for bracketed paste and `session.ValidateTyped` already applies
  for the session page (spec 018 FR-004), and it MUST be **the same predicate
  rather than a second copy** — two spellings of one rule are free to disagree,
  and the one that disagrees is the one nobody reads. The refusal MUST be `400`
  with the uniform bad-request body, before any tmux command.
  - The **length** half of `ValidateTyped` is deliberately not adopted. The prompt
    route keeps the bound it has today (`CRSW_MAX_BODY_BYTES`, default 65536);
    taking `MaxTypeBytes` (16384) with it would tighten a live route's bound for
    no requirement, which is a client break wearing a safety check's clothes.
  - This door MUST NOT CRLF-normalise. The session page's route rewrites `\r\n`
    before validating because a textarea produces it; the API door delivers caller
    bytes unaltered, so a `\r` is refused rather than rewritten.
- **FR-008**: The API door MUST refuse a prompt whose first character that is not
  space, tab, CR or LF is `!` or `/`, with `400` and the uniform bad-request body,
  before any tmux command. The rule reads the first non-whitespace character of
  the whole text and no other character. The session page's type route MUST be
  unchanged, and a test MUST assert that it still delivers `/compact`.

### Concurrency

- **FR-009**: `Manager.Prompt` MUST take the restart claim (`claimRestart`,
  non-blocking) **after** `lockSession`, and MUST release it on return. The order
  matches `Continue`'s, so no new lock ordering enters the package.
- **FR-010**: A prompt that finds the restart claim held MUST refuse with
  `ErrSessionNotReady` and MUST run no tmux command. It MUST NOT wait: a prompt
  that blocked across a process restart would hold the request open, and a caller
  cannot tell a slow delivery from a lost one.
- **FR-011**: The two guarantees already true MUST be asserted rather than assumed
  — the per-session mutex is held across the paste and the Enter together, and
  each paste draws its own tmux buffer name. A test MUST show two concurrent
  prompts using distinct buffer names and never interleaving a paste and a submit.

### Correlation

- **FR-012**: `promptRequest` MUST gain one optional field, `request_id`, of 1 to
  32 characters drawn from `[a-z0-9-]`. A value outside that is a `400` decided at
  the boundary, before any tmux command. The body stays decoded with
  `DisallowUnknownFields`, so no other new field is accepted.
- **FR-013**: A valid `request_id` MUST be echoed in the `202` body and written to
  the request's one audit record under the key `request_id`. An absent one MUST
  appear nowhere: the field is omitted from the response and from the record, so a
  client that sends none sees today's bytes.
- **FR-014**: The record MUST gain the `request_id` and **nothing about the
  prompt** — not its text, not its length, not a hash of it. `Record.Reason` stays
  server-authored sentinels and MUST NOT carry the value; `request_id` is the one
  field on the record whose bytes a caller chose.

### Bounds on the change

- **FR-015**: The API door MUST still have exactly six routes, and this feature
  MUST NOT add a package under `internal/`, a dependency, a `go.sum`, or a
  `tmuxctl.Controller` method.
- **FR-016**: Every payload shape ai-lawnmower sends today — board answer, requeue
  resume, opp-lease batch handoff, opp-lease follow-up — MUST still answer `202`
  with `"delivered":true`. The four are fixed byte-for-byte as test fixtures so
  that a later change to any rule here is measured against the traffic rather than
  argued about.
- **FR-017**: `docs/security.md` MUST gain a "Prompt route gates (spec 020)"
  subsection under "Non-negotiables", naming the liveness gate, the `failed`
  refusal, the bracketed paste and the control-character rule it brings, the
  leading-character rule, the restart claim and `request_id`. `AGENTS.md` MUST NOT
  be edited, and MUST stay under the 150-line CI limit.

### Amendments to shipped specifications

- **FR-018**: Spec 018 **FR-006** ("the API door's `POST /sessions/{id}/prompt`
  MUST be unchanged: plain paste, then Enter. Bracketed paste is for the
  interactive route only") is amended by FR-006, FR-007 and FR-008 of this
  specification: the route's paste becomes bracketed, it gains the
  control-character rule that bracketing makes load-bearing, and it gains one
  refusal the interactive route does not have. Nothing else about the interactive
  route changes, and spec 018's own FR-004 is untouched.
- **FR-019**: Spec 001 **FR-042** (an audit record carries no prompt text, no
  session output, no credential and no shared secret) is amended for exactly one
  field by FR-013 and FR-014: the record may carry a caller-supplied `request_id`
  of at most 32 characters from `[a-z0-9-]`. Everything else FR-042 forbids stays
  forbidden, and the alphabet is what keeps the field from becoming a channel for
  the text — 32 bytes of that set cannot carry a prompt.

### Out of scope

- **FR-020**: A readiness gate reading Claude Code's session registry
  (`sessions/<pid>.json`) is **not** built here. It was stated as a MAY; probe
  check 8 left open whether the daemon can read that file at all under spec 017's
  v2 option B, so a gate built on it would work on the VM and be blind in the
  cluster; and probe checks 3 and 9 show the registry misses the trust dialog and
  a half-typed draft, so it answers "a session exists" rather than "this session
  will read what I send next". Deferring a MAY is a decision, and research.md D8
  records it as one. The cost is stated there: a session parked on a dialog still
  accepts a prompt.
- **FR-021**: Also out of scope, each with its reason in research.md D9: opening a
  Claude transcript or registry file from Go (spec 013 FR-025 is unchanged);
  holding or calling an LLM; a seventh API route; free text on the browser door
  beyond spec 018; the startup lost-Enter window (probe check 1b); and the socket
  transport, which belongs to the lm-steer-write milestone.

**Key Entities**: **Liveness**, the existing three-valued `tmuxctl` answer derived
from `@crswd-binary` against the pane's current command — running, stopped,
unknown. **Not-ready**, one condition with one sentinel and one answer, covering a
stopped pane, a `failed` session and a held restart claim; the caller cannot tell
the three apart because to a caller they are one event, *not now, try later*.
**`request_id`**, a caller-chosen correlation value, bounded in length and
alphabet, and the only caller bytes on an audit record.

## Success Criteria *(mandatory)*

- **SC-001**: `go test ./internal/... -run Prompt` proves, against the fake
  controller, each of: a stopped pane answers `409` `{"error":"session not ready"}`
  with no paste; a `failed` session the same with no tmux command at all; an exit
  between the paste and the Enter yields `C-u`, no Enter and `409`; a held restart
  claim yields `409` with no tmux command.
- **SC-002**: `go test -tags tmux ./internal/session/ -run Prompt` proves on a real
  tmux that a pane on `sh` carrying `@crswd-binary` `claude` is refused and reads
  back empty, and that a two-line prompt into `cat` arrives once.
- **SC-003**: `go test ./internal/httpapi/ -run 'Prompt|Type'` proves that a
  leading `/` or `!` answers `400` with nothing delivered, and that the session
  page's type route still delivers `/compact`.
- **SC-004**: `go test ./internal/httpapi/ -run RequestID` proves that a valid
  `request_id` is echoed and audited, that a 33-character or upper-case one answers
  `400` with nothing delivered, and that an absent one appears nowhere.
- **SC-005**: `go test ./internal/httpapi/ -run Lawnmower` posts all four fixtures
  and gets `202` `"delivered":true` for each.
- **SC-006**: The leak suite finds no prompt text, no length and no hash of it in
  any audit record or log line, with the new field present.
- **SC-007**: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) '
  specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`, and a diff of
  `internal/`'s directories against `main` is empty.
- **SC-008**: `go test ./...`, `go test -tags tmux ./...`,
  `go vet -tags quickstart ./cmd/crswd` and `golangci-lint run` all exit 0, and
  `go.sum` still does not exist.

## Assumptions

- `@crswd-binary` against the pane's current command is a true liveness signal for
  the sessions this route serves. Probe check 13 measured 6 of 6 live panes,
  three of them `rcq`.
- ai-lawnmower is the only client of this route. The grep that found no caller
  refused by FR-008 covered that client; an unknown client sending a leading `/`,
  a control byte or a bare `\r` is refused with a `400` it can see and fix.
- `409 {"error":"session not ready"}` needs no client change: lawnmower's
  `deliver()` already treats any body carrying an `error` key as a refusal.
- A refusal is cheaper than a wrong delivery on this route, because a wrong
  delivery is a command line in an unsandboxed shell and a refusal is a retry.
