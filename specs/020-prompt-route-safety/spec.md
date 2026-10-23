# Feature Specification: Prompt Route Safety

**Feature Branch**: `plan/lm-steer-crswd-018`

**Created**: 2026-10-15

**Status**: Draft

**Input**: `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`, section (g)
"Phase 3 detail", requirements 1 to 7, with the probe evidence in
`~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`. Both are outside
this repository and read-only. Every decision this spec rests on was taken on
2026-10-14 and is recorded with its evidence in [research.md](research.md) as
`D1` to `D9`; none of them is reopened here.

**Spec**: this file · **Research**: [research.md](research.md) · **Plan**:
[plan.md](plan.md) · **Tasks**: [tasks.md](tasks.md)

`POST /sessions/{id}/prompt` is the one route lawnmower's board `answer` and its
opp-lease handoff already drive. It is shipped and it has live callers. This
milestone hardens it in place — six gates, one new optional field, no new route —
so that a voice `steer` can use it later without a second route and without a
second set of rules. The spec number is 020 because 018 and 019 are taken in this
repository (research.md D1); the design's own "crswd spec 018" means this work.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A prompt into a session whose harness has exited is refused, not run (Priority: P1)

A session's pane is a login shell. Claude Code started in it and has since
exited, so the pane is sitting at a `$` prompt with the shell's own job of
reading a line and running it. A caller delivers a prompt. Today the text is
pasted and an Enter follows, so the caller's prompt becomes a command line that
the shell executes, unsandboxed, as the daemon's user.

**Why this priority**: it is the one failure on this route that turns delivered
text into executed text. Every other gate here bounds a nuisance; this one bounds
arbitrary code execution reached by a request that was otherwise entitled to
deliver a prompt.

**Independent Test**: start a pane on a shell, set `@crswd-binary` to `claude` so
the daemon reads the harness as stopped, and post a prompt. The route answers
`409` and the pane is byte-for-byte unchanged.

**Acceptance Scenarios**:

1. **Given** a session whose pane command does not match its `@crswd-binary`,
   **When** a prompt is posted, **Then** the answer is `409` with body
   `{"error":"session not ready"}`, no tmux buffer is loaded, no key is sent, and
   the pane is unchanged.
2. **Given** a session whose harness liveness reads *unknown* — a session this
   daemon adopted, or one started by a build before the option existed — **When**
   a prompt is posted, **Then** it is delivered normally.
3. **Given** a session whose harness exits **after** the paste has landed and
   before the Enter, **When** the delivery continues, **Then** the daemon sends
   `C-u`, sends no Enter, and answers `409`. The pasted text is cleared off the
   shell's line rather than left there for the next Enter to run.
4. **Given** the controller cannot be listed at all, **When** a prompt is posted,
   **Then** the delivery fails rather than proceeding on an unknown answer.

---

### User Story 2 - A caller can tell "not now" from "this daemon broke" (Priority: P1)

lawnmower posts a prompt into a session it believes is working. The session is in
state `failed`, or a revive is retyping its start command. Today the route
answers `500`, which is this daemon claiming a fault of its own for a state it
understood perfectly well — so the caller's only safe reading is "something is
wrong with crswd".

**Why this priority**: the existing caller already treats a body carrying an
`error` key as a refusal, so this is the difference between a handoff that
retries sensibly and one that reports the daemon as broken.

**Independent Test**: put a session in state `failed` and post a prompt. The
answer is `409 {"error":"session not ready"}` and the audit record is a deny
whose reason is `session not ready`.

**Acceptance Scenarios**:

1. **Given** a session in state `failed`, **When** a prompt is posted, **Then**
   the answer is `409 {"error":"session not ready"}` and no tmux command runs.
2. **Given** a revive or a continue holds the session's restart claim, **When** a
   prompt is posted, **Then** the same `409` is answered and no tmux command runs.
3. **Given** any of the three not-ready states, **When** the trail is read,
   **Then** there is exactly one audit record for the request, a deny, with
   reason `session not ready` — the same reason for all three, so the body and
   the trail agree and neither distinguishes which state the session was in.
4. **Given** a session in state `dead`, **When** a prompt is posted, **Then** the
   answer is today's answer, unchanged.

---

### User Story 3 - A multi-line prompt arrives as one message (Priority: P1)

The opp-lease handoff pastes a multi-line item body. Today `Manager.Prompt`
chooses bracketed paste only when the harness spec asks for it, and Claude's does
not — so the first newline is typed as a carriage return, line one submits on its
own, and the rest sits in the input box as a half-typed draft nobody sends.

**Why this priority**: every one of the four live payload shapes is multi-line
(research.md D4). This is the ordinary case on this route, not an edge, and it is
wrong today for every Claude session.

**Independent Test**: post a two-line prompt and assert one bracketed paste
followed by exactly one Enter, with no Enter between the lines.

**Acceptance Scenarios**:

1. **Given** a Claude session, **When** a two-line prompt is delivered, **Then**
   the harness receives one two-line message and does not submit after line one.
2. **Given** any harness at all, **When** a prompt is delivered, **Then** the
   paste is bracketed. The choice does not depend on the harness.
3. **Given** a compact is requested, **When** it is delivered, **Then** it is
   delivered exactly as it is today — its constant text depends on that newline
   becoming a carriage return in order to submit itself.

---

### User Story 4 - A prompt cannot be a slash command or a bash escape (Priority: P2)

Claude Code reads a leading `/` as a slash command and a leading `!` as a bash
escape. A prompt that begins with either is not a message to the assistant; it is
a command to the harness, chosen by whoever could reach this route.

**Why this priority**: it narrows what an API caller can reach without removing
anything a live caller does. Every lawnmower builder starts its prompt with a
letter (research.md D5).

**Independent Test**: post `/status` and get `400`; post a prompt whose *second*
line starts with `/` and have it delivered.

**Acceptance Scenarios**:

1. **Given** a prompt whose first non-whitespace character is `/` or `!`,
   **When** it is posted to the API door, **Then** the answer is `400` with the
   uniform bad-request body and no tmux command runs.
2. **Given** a prompt whose first non-whitespace character is a letter and whose
   third line begins `/compact`, **When** it is posted, **Then** it is delivered
   whole.
3. **Given** the session page's type route, **When** the operator sends
   `/compact`, **Then** it is delivered. That door is unchanged.

---

### User Story 5 - A caller can tie a delivery to its record in the trail (Priority: P3)

lawnmower sends several prompts into one session. Reading the audit trail
afterwards, the session ID is not enough to say which record belongs to which
delivery.

**Why this priority**: it is a correlation handle, not a gate. It is last because
nothing is unsafe without it.

**Independent Test**: post a prompt with `"request_id":"opp-lease-7"`, read the
`202` body and the audit record, and find the value in both.

**Acceptance Scenarios**:

1. **Given** a body carrying `request_id` of 1 to 32 characters of `[a-z0-9-]`,
   **When** the prompt is delivered, **Then** the value is echoed in the `202`
   body and written as `request_id` on the request's one audit record.
2. **Given** a body carrying a `request_id` outside that character class or that
   bound, **When** it is posted, **Then** the answer is `400` with the uniform
   bad-request body and no tmux command runs.
3. **Given** a body carrying no `request_id`, **When** the prompt is delivered,
   **Then** the field appears nowhere — not in the response body, not on the
   audit record. A milestone 1 client sees byte-for-byte the response it always
   has.

### Edge Cases

- **A `List` that fails while the gate is reading it.** It is an error and not an
  implicit pass. This is an auth-adjacent bound and it fails closed
  (`docs/security.md` §4); the caller gets this route's existing `500` with no
  detail, because which of the host's failures was hit is an oracle about a
  machine the caller cannot otherwise see.
- **Liveness *unknown* on an adopted session.** It passes. Spec 012 FR-006 reads
  that value as alive, and refusing it would break every session the daemon
  adopted from a build that predates the option.
- **The window between the two tmux commands.** A harness that exits between the
  paste and the Enter is exactly what the second liveness read is for. There is no
  window it closes completely — the pane could exit after the second read and
  before the Enter — and what bounds that residue is that the window is two tmux
  round trips wide rather than a whole request wide. Probe check 1 found no
  check-then-Enter race worth defending against after a session's first turn.
- **Two prompts into one session at once.** Both land, one after the other. The
  per-session mutex is held across a whole delivery, so a paste can never land
  between another delivery's paste and its Enter, and each paste loads a buffer
  name of its own, so two deliveries cannot read each other's bytes.
- **A prompt holding the end-of-paste marker `ESC [ 2 0 1 ~`.** Under a bracketed
  paste that marker ends the paste early and turns the remainder into keystrokes,
  which is a way around the leading-character rule in FR-012. No gate in this
  milestone refuses it; it is named under "Not in this feature" (FR-019) with what
  is known and what is not.
- **A prompt that is only whitespace.** It is non-empty, so `ErrEmptyPrompt` does
  not catch it, and it has no first non-whitespace character, so FR-012 does not
  refuse it. It is delivered, exactly as it is today.
- **A `request_id` reused across two deliveries.** Nothing rejects it. The field
  is a handle the caller chose, not an identity this daemon issues or deduplicates.

## Requirements *(mandatory)*

### The rules this feature amends

- **FR-001**: Spec 018 FR-006 — "The API door's `POST /sessions/{id}/prompt` MUST
  be unchanged: plain paste, then Enter. Bracketed paste is for the interactive
  route only." — is **superseded by this spec**. That sentence was written to keep
  spec 018 from touching a shipped route, and it is withdrawn here deliberately
  and in one named place. What replaces it is FR-009: the API door's prompt route
  pastes bracketed for every harness.
- **FR-002**: Spec 001 FR-042 — audit records MUST NOT contain prompt text,
  session output, session credentials, or the shared secret — is amended **for the
  one field `request_id` and for no other**. A value of 1 to 32 characters of
  `[a-z0-9-]` is a correlation handle rather than free text: the character class
  and the bound are what make it one. Prompt text itself is unchanged by this —
  no prompt text, no length of it, no hash of it, and no prefix of it may reach an
  error, a log line or an audit record.

### Readiness — the harness is still there (requirement 1)

- **FR-003**: Before any tmux command, `Manager.Prompt` MUST read the session's
  row from the controller's existing `List` and consult its harness liveness. It
  MUST NOT add a method to the `Controller` interface: the second module's pod
  controller must satisfy that interface and does not serve this route.
- **FR-004**: A liveness of *stopped* MUST refuse with `ErrSessionNotReady` and
  MUST deliver nothing — no buffer loaded, no key sent. A liveness of *unknown*
  MUST deliver. A `List` that fails MUST return that failure, never an implicit
  pass.
- **FR-005**: After the paste has landed and before the Enter, `Manager.Prompt`
  MUST read liveness a second time. If the second read is *stopped*, the daemon
  MUST send `C-u`, MUST send no Enter, and MUST return `ErrSessionNotReady`.
- **FR-006**: Detection MUST NOT read the content of a session's pane, a
  transcript, or any `~/.claude/sessions/*.json` registry file. Spec 013 FR-025
  and spec 012 FR-007 are unchanged.

### Readiness — the answer a caller gets (requirements 1, 2 and 5)

- **FR-007**: There MUST be exactly one sentinel for "not now",
  `session.ErrSessionNotReady`, whose text is `session not ready`. Three states
  return it: a *stopped* harness (FR-004, FR-005), a session in state `failed`,
  and a held restart claim (FR-014). A session in state `failed` MUST be refused
  with it **before any tmux command runs**.
- **FR-008**: The prompt handler MUST map `ErrSessionNotReady` to **`409`** with
  body `{"error":"session not ready"}`, and MUST record the request's one audit
  record as a deny whose reason is that sentinel's own text. The three states MUST
  NOT be distinguishable by the body, the status or the reason — a caller that may
  not read a session must not learn which state it is in. A session in state
  `dead` keeps today's sentinel and today's answer; that path is shipped and a
  client reads it.

### Delivery (requirement 3)

- **FR-009**: `Manager.Prompt` MUST paste with a **bracketed** paste for every
  harness, unconditionally, rather than through the harness-dependent paste. One
  Enter follows, as it does today.
- **FR-010**: `Manager.Compact` MUST be unchanged. Its text is a constant whose
  trailing newline becoming a carriage return is what submits it.
- **FR-011**: A prompt MUST NOT be refused for containing a newline. The handoff's
  payloads are multi-line and that is the ordinary case on this route.

### The first character (requirement 4)

- **FR-012**: On the **API door only**, the handler MUST find the first character
  of `text` that is not space, tab, carriage return or line feed. If that
  character is `!` or `/`, it MUST answer **`400`** with the uniform bad-request
  body, before any tmux command runs. The rule is about that one character and
  nothing else: a `/` or `!` anywhere later in the text, and every other
  metacharacter, is delivered byte-for-byte as FR-030 of spec 001 requires.
- **FR-013**: The session page's type route (`POST /dashboard/sessions/{id}/type`,
  spec 018 FR-002) MUST be unchanged and MUST still deliver `/compact`. An
  operator pressing a key in a panel this daemon drew may legitimately send a
  slash command; they can already do it by attaching to the pane.

### One delivery at a time (requirement 5)

- **FR-014**: `Manager.Prompt` MUST take the per-session mutex first and then the
  restart claim, non-blocking. A claim held by a revive or a continue MUST refuse
  with `ErrSessionNotReady` and MUST run no tmux command. The claim MUST be
  released on return, by every path out. The order is the property: the lock is
  held first, so the claim is attempted by one delivery at a time and a refusal is
  about a restart rather than about another prompt.
- **FR-015**: A prompt is refused rather than queued when a restart holds the
  claim. A revive retypes the start command, and text pasted across that is text
  typed at whatever the pane is mid-restart.
- **FR-016**: Each paste MUST load a buffer name of its own, as it does today, so
  that two concurrent deliveries cannot read each other's bytes. Two concurrent
  prompts MUST NOT interleave a paste with another delivery's Enter.

### Correlation (requirement 6)

- **FR-017**: The prompt body MUST accept an **optional** field `request_id` of 1
  to 32 characters drawn from `[a-z0-9-]`. A value outside that class or that
  bound MUST answer **`400`** with the uniform bad-request body and MUST run no
  tmux command. The body is still decoded into a typed struct with unknown fields
  rejected, so no other new field is accepted.
- **FR-018**: A valid `request_id` MUST be echoed in the `202` body and written as
  `request_id` on the request's one audit record. An absent `request_id` MUST
  appear nowhere: the field is omitted from the response body and from the audit
  record, so a client written against the milestone 1 contract sees byte-for-byte
  the response it always has.

### Not in this feature

- **FR-019**: None of the following is in this milestone, and silence about them
  is not permission (research.md D8, D9):
  - **The session-registry readiness gate** (requirement 7 of the design). It is a
    MAY, and it is declined on evidence rather than deferred for want of a
    decision: probe check 8 left open whether this daemon can read
    `sessions/<pid>.json` at all under the k8s execution model, where no functional
    requirement in spec 017 places that file, and probe checks 3 and 9 found the
    registry blind to both the workspace-trust dialog and a half-typed draft. A
    gate that reports ready in two states where a delivery is wrong is a check
    that lies, and the liveness gate in FR-003 already covers the state that
    matters here. Reopening it needs probe check 8's spec 017 question answered
    first.
  - **A byte-level refusal of `ESC [ 2 0 1 ~`, or of C0 controls, on the API
    door.** Spec 018 FR-004 makes that check on the browser door's text, and
    FR-009 now makes this door's paste bracketed too, so the marker's effect here
    is the same: it would end the paste early and turn the remainder into
    keystrokes, including a leading `/` or `!` that FR-012 would otherwise have
    refused. No live caller sends it and no payload fixture holds it. Adding a
    byte-level refusal to a route with shipped callers is its own decision with its
    own evidence, and it is not taken here.
  - **Opening a transcript** or any `~/.claude/sessions/*.json` file from Go.
  - **Holding or calling a model.** This route delivers bytes a caller chose.
  - **A seventh API route.** The door has six and the count is asserted.
  - **Free text on the browser door beyond spec 018's input panel.**
  - **The startup lost-Enter window** — the first few seconds after a launch,
    where an Enter can be swallowed (probe check 1b). Real, measured, a different fix.
  - **The socket transport.** The probe's "flip to socket" verdict binds a
    different milestone.
  - **A new package under `internal/`**, any change to `k8s/`, to `AGENTS.md`, to
    the `Controller` interface, or to `go.mod`. The root module gains no `go.sum`
    (`docs/security.md` §5).

### The contract

- **FR-020**: `specs/001-crswd-daemon-core/contracts/http-api.md` MUST state all
  of this on the prompt route: the optional `request_id` field with its rules, the
  `409` not-ready answer with its body, and the leading-`!`-or-`/` refusal. Its
  status-code table's `409` row MUST cover not-ready alongside the unverified
  teardown it covers today. The file MUST still carry exactly six route headings.

**Key Entities**: **Harness liveness**, one of *running*, *stopped* or *unknown*,
derived by the controller from a session's `@crswd-binary` option against its
pane's current command — the signal the gate reads, and never pane content.
**`ErrSessionNotReady`**, the one sentinel for all three not-now states.
**`request_id`**, a caller-chosen correlation handle bounded to 1–32 characters of
`[a-z0-9-]`; the only caller-chosen bytes this spec admits onto the audit trail.
**Prompt text**, caller bytes that are secret under `docs/security.md` §3 — never
in an error, a log line, a record, or a response.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Each of requirements 1 to 6 has a test that **fails against the code
  before its change** and passes after, recorded with both results in
  `ralph/plans/lm-steer-crswd-018/PROGRESS.md` under "Failing-before evidence" —
  one row per requirement, six rows.
- **SC-002**: `go test -tags tmux ./internal/session/ -run Prompt` proves on a
  real tmux that a pane sitting on a shell with `@crswd-binary` set to `claude` is
  refused and left byte-for-byte empty, and that a two-line prompt into a reader
  arrives once, whole.
- **SC-003**: A test posts each of the four recorded lawnmower payloads — board
  answer, resume, opp-lease batch, opp follow-up — and every one answers `202`
  with `delivered: true`. No live caller is broken by any gate in this spec.
- **SC-004**: `go test -race ./internal/session/ -run Prompt` passes with two
  concurrent deliveries into one session, with distinct buffer names and no paste
  interleaved with the other delivery's Enter.
- **SC-005**: The audit leak suite still finds no prompt text, no length of it and
  no hash of it in any record or log line, with `request_id` present and absent.
- **SC-006**: The API door still has exactly six routes, asserted, and
  `http-api.md` still carries exactly six route headings.
- **SC-007**: `gofmt -l .` prints nothing; `go build ./...`, `go vet ./...`,
  `go vet -tags tmux ./...`, `go vet -tags quickstart ./cmd/crswd` and the lint
  command in `AGENTS.md` all exit 0; the root `go.sum` does not exist.

## Assumptions

- **The liveness signal the daemon already computes is good enough for this gate.**
  Probe check 13 matched `@crswd-binary` against `pane_current_command` on 6 of 6
  live panes, 3 of them `rcq` sessions. A pane running a harness under a different
  process name would read as *stopped* and be refused; that is the direction a
  wrong answer should fail in.
- **The existing caller reads a `409` carrying an `error` key as a refusal.**
  lawnmower's `deliver()` already treats any body with an `error` key that way, so
  no change is needed on that side for the new answer to be understood.
- **Bracketed paste is enabled by the harnesses in use.** Measured for Claude Code
  2.1.290 (spec 018 R1) and Codex 0.153.4 (spec 019 M3, M4) on tmux 3.4. A later
  release that stops enabling it degrades to "line one submits", which is today's
  behaviour and which an operator sees immediately.
- **No live caller begins a prompt with `!` or `/`.** Grepped across lawnmower's
  four builders on 2026-10-14. A future caller that wants to send a slash command
  uses the session page, or asks for a decision of its own.
- **tmux 3.4 key names are stable across 3.x**, so `C-u` means what it was
  measured to mean.
