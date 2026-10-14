# Research: prompt route safety

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · **Tasks**: [tasks.md](tasks.md)

This feature hardens `POST /sessions/{id}/prompt`, the one API route that already
delivers free text into a running session. It is the route ai-lawnmower's `answer`
and opp-lease handoff call today, so every change here is a change to a live client.

## Where the evidence comes from

Three sources, and each decision below says which one it leans on.

| Tag | Source | Status |
|---|---|---|
| **Probe** | `~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`, numbered checks | Outside this repo, read-only. The checks this feature turns on are transcribed into `ralph/plans/lm-steer-crswd-018/PROGRESS.md` ("Probe results that decide this plan") because the loop that implements this spec cannot reach that path |
| **Design** | `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`, section (g) "Phase 3 detail" | Outside this repo, read-only. It is where requirements 1 to 7 are stated |
| **Code** | This repository at `30941f3` | Read and re-read in tree on 10/14/26; the line references below were checked, not remembered |

The decisions were taken during planning, before any task ran, and are recorded in
`ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` under "Decisions already
made". They are reproduced here with their evidence. **They are not reopened by the
implementation**; a task that finds one of them wrong stops and says so rather than
choosing differently.

## The route as it stands (Code, 10/14/26)

What the hardening is measured against:

- `internal/httpapi/sessions.go` `promptSession` (line 772) decodes `promptRequest`,
  best-effort-reads the pane for a known dialog (`paneDialogState`, which is
  additive information and never a gate), calls `Manager.Prompt`, and answers `202`
  with `{"id","delivered","parked_on","suspicious_dialog"}`.
- `refusePrompt` (line 849) maps `session.ErrEmptyPrompt` to `400` and **everything
  else to a `500` with no detail**. There is no 409 on this route today.
- `internal/session/manager.go` `Manager.Prompt` (line 1194) refuses an empty ID
  (`ErrSessionNotFound`), `StateDead` (`ErrSessionDead`) and empty text
  (`ErrEmptyPrompt`); takes `lockSession`; calls `m.paste`; then `SendKeys` Enter.
- `m.paste` (line 1253) is bracketed **only** when the harness spec says
  `BracketedPaste`, which Claude's does not (spec 019 D9 left that to spec 018, and
  spec 018 FR-006 declined it for the API door).
- `Manager.Prompt` does **not** read liveness, does **not** consult `StateFailed`,
  and does **not** take the restart claim.

So a prompt into a session whose Claude has exited lands in a login shell as a
command line, and a prompt racing a revival lands half in one process and half in
the next. Those are the two failures this feature closes.

---

## D1 Spec number

**Decision.** This feature is `specs/020-prompt-route-safety/`.

**Evidence.** The design calls it "crswd spec 018", but `specs/018-interactive-input`
(the session page's input panel) and `specs/019-codex-runtime` already exist in this
repository (Code: `ls specs/`). 020 is the first free number.

The milestone directory keeps the design's name — `ralph/plans/lm-steer-crswd-018/` —
because the lawnmower side refers to it that way. The mismatch is deliberate and is
stated in `VALIDATION_CONTRACT.md` so that a later reader does not "fix" one to match
the other.

---

## D2 Liveness gate (requirement 1)

**Decision.** `Manager.Prompt` reads the session's row from the controller's `List`
and branches on its `Claude` liveness:

- `LivenessStopped` → refuse with `ErrSessionNotReady`, before any tmux command that
  changes anything.
- `LivenessUnknown` → deliver.
- `LivenessRunning` → deliver.
- A `List` that fails → an error, not a delivery.

The gate runs **twice**: once before the paste, and again after the paste and before
the Enter. If the second read is stopped, the daemon sends `C-u` and no Enter.

**Evidence.**

- Probe check 13: `@crswd-binary` matched `pane_current_command` on 6 of 6 live panes,
  three of them `rcq` panes. So the option the daemon already sets is a true liveness
  signal for the sessions lawnmower actually drives, not only for the plain case.
- Code: `tmuxctl.SessionInfo.Claude` is the three-valued `Liveness`
  (`internal/tmuxctl/controller.go:312-341`), derived from `@crswd-binary` against the
  pane's current command — by one tmux format in `Exec` (`exec.go:548`) and by
  `livenessOf` in the fake (`fake.go:251`). Both exist already; nothing new is
  measured here.
- Spec 012 FR-006 ("detect a session whose Claude process has stopped while its shell
  survives") is the requirement this reads, and `supervisor.go:153-156` already treats
  `LivenessUnknown` as healthy with a comment saying it must. This gate reads it the
  same way, so a session created before `@crswd-binary` existed keeps working.
- Probe check 1: there is no check-then-Enter race after the first turn; the lost-Enter
  window is the first ~5 s after launch, which is out of scope (D9). That is why two
  reads around the paste are enough and a third is not.

**Why `List` and not a new method.** A per-session liveness method on
`tmuxctl.Controller` would force a matching change in the k8s module's `podctl`
(Code: `k8s/` is a second module reaching the root's `internal/` through `replace ../`).
This milestone adds no `Controller` method, so `podctl` is untouched and the cluster
build does not move.

**Why the second read, and why `C-u`.** Between the paste and the Enter the pane can
fall back to a shell — Claude exits, the composer's text is gone, and the Enter would
submit whatever the paste left on the shell's line as a command. `C-u` clears that
line; sending no Enter means nothing runs. Refusing *after* a paste is not as good as
refusing before one, and the `C-u` is what makes the imperfect case harmless rather
than merely reported.

*Rejected: refusing `LivenessUnknown`.* It is the state of every session started by a
build older than the option, and refusing it would make this route stop working for
sessions the supervisor itself calls healthy — two answers to one question.

*Rejected: reading the pane's text to decide readiness.* Spec 012 FR-007 forbids it
for detection, pane content is secret under `docs/security.md` §3, and `paneDialogState`
already shows what a text read is good for: additive information, never a gate.

---

## D3 Not-ready answer (requirements 1, 2 and 5)

**Decision.** One new sentinel, `session.ErrSessionNotReady`. Every refusal this
feature adds inside `internal/session` returns it, and `refusePrompt` maps it to
`409` with body `{"error":"session not ready"}` and an audit **Deny** whose reason is
the constant `session not ready`.

A session in `StateFailed` gets it. `StateDead` keeps today's answer unchanged.

**Evidence.**

- Code: `StateFailed` (`internal/session/session.go:86`) means the supervisor tried to
  bring the session back and ran out of attempts (`supervisor.go:311-314`). Nothing is
  running in that pane, so a prompt into it is the same failure as a stopped pane;
  `Manager.Prompt` refuses `StateDead` today and says nothing about `StateFailed`.
- Code: a dead session is already refused as a `404` by the resolver before
  `promptSession` runs, and `refusePrompt`'s comment says reaching its `ErrSessionDead`
  branch would mean a record went terminal between two layers of one request. Leaving
  `dead` alone keeps that path exactly as it is and keeps the enumeration rule
  (`docs/security.md`: a session the caller may not have is a 404, never a 403/409).
- lawnmower's `deliver()` (`~/code/ai-lawnmower/loop/session_resume.py`, as recorded in
  PROGRESS.md on 10/14/26) treats **any** body carrying an `error` key as a refusal. So
  `409 {"error":"session not ready"}` reads as a refusal to the existing client with no
  client change, which is why the body carries `error` and not a new key.
- `docs/security.md` "Rate limiting & audit": every reason on the trail is a sentinel
  this codebase authored. `session not ready` is one. It names no state, no pane
  content and no prompt — an operator reading the trail learns that the route refused
  and why in the daemon's own words, and a caller learns nothing about the host it
  could not already see.

*Rejected: a sentinel per reason* (stopped pane, failed state, restart held). Three
sentinels reaching one status code with one body is three things to keep in step for
a distinction the caller cannot observe. The trail distinguishes nothing more here
because the three are the same event to the client: not now, try later.

*Rejected: `503`.* The condition is about one named session, not about this daemon's
capacity, and `409` is already the contract's "the host's state refuses this" code
(the unverified-teardown answer, `sessions.go:749`).

---

## D4 Bracketed paste (requirement 3)

**Decision.** `Manager.Prompt` pastes with `PasteBracketed` for **every** harness.
`Manager.Compact` keeps its harness-based paste (`m.paste`). No newline refusal.

**Evidence.**

- Code: `Controller.PasteBracketed` already exists (spec 018 added it; `paste-buffer -p`,
  `internal/tmuxctl/exec.go:220-230`), and the session page's type route already uses it
  for every harness. This decision reuses that method; it adds none.
- `docs/security.md` §2: inside a bracketed paste the terminal does not interpret the
  pasted bytes, which is what makes multi-line text one paste instead of a line of text
  plus a submit plus another line. The same section already requires that caller text
  reach tmux only through `load-buffer` on stdin and then `paste-buffer`, bracketed on
  the session page's route; this extends the bracketing to the API door's route.
- Spec 019 D9 / research M3–M5 measured that Codex swallows the Enter after a *plain*
  paste and submits correctly after a bracketed one. Claude accepts both. So bracketed
  is the one form that is correct for both harnesses, and `Prompt` stops needing to ask
  which harness it is talking to at all.
**A consequence this decision does not close, named rather than smoothed over.**
`docs/security.md` §2 says text holding a C0 control other than TAB or LF is refused,
"because inside a bracketed paste `ESC[201~` would end the paste" — and that sentence
is about **the session page's route**. The refusal is `session.ValidateTyped`
(`internal/session/input.go:75-92`, spec 018 FR-004), and Code confirms it is called by
`Manager.Type` (`input.go:105`) and **not** by `Manager.Prompt`: the API door's
`promptRequest.Text` is validated only for emptiness and the body-size bound
(`internal/httpapi/sessions.go:264-270`, "that is the whole of its validation
(FR-030)"). So bracketing this route's paste makes a byte sequence relevant that was
inert under a plain paste: a caller that sent `\x1b[201~` mid-text would end the paste
early and the remainder would reach the pane as keystrokes rather than as pasted text.

D4 as decided adds no refusal, so this feature does not close it, and this document
does not invent one. What is recorded here is that the mechanism already exists
(`ValidateTyped`, one call away) and that spec.md has to say which way it goes —
either the route gains that check or the exposure is accepted in writing. It is logged
as a finding in `ralph/plans/lm-steer-crswd-018/PROGRESS.md` so the task that writes
spec.md cannot miss it. No measured client sends a control byte: every fixture in
PROGRESS.md "Payload fixtures" is printable text plus newlines.

**Why no newline refusal.** The opp-lease handoff pastes multi-line item bodies
(Probe/Design: the builders in `console.py` and `lawnmower-route`, whose shapes are
fixed byte-for-byte as T011's fixtures in PROGRESS.md "Payload fixtures"). A rule
refusing `\n` would refuse the traffic this route exists to carry. Bracketing is what
makes a newline safe; refusing newlines would be the same protection obtained by
breaking the client.

**Why `Compact` is untouched.** Its payload is a daemon constant, not caller text, and
spec 018 FR-006's "unchanged" promise about this route is amended here only for what
requirements 1 to 6 name. Changing `Compact` as well would be an unrequested change to
a path with no requirement behind it (constitution IV).

This amends spec 018 **FR-006**, which says the API door's prompt route must be
unchanged — plain paste, then Enter, with bracketed paste reserved for the interactive
route. The amendment is recorded in spec.md, not left to this file.

---

## D5 Leading `!` or `/` (requirement 4)

**Decision.** On the **API door only**: refuse a prompt whose first character that is
not space, tab, CR or LF is `!` or `/`. The answer is `400` with the uniform
bad-request body, decided before any tmux command runs. The session page's type route
is unchanged.

**Evidence.**

- Design, requirement 4: those two characters are Claude Code's slash-command and
  shell-escape prefixes. A prompt that begins with one is not a prompt — `/clear`
  discards the conversation the caller was trying to continue, and `!` runs a shell
  command in the session's own unsandboxed context. A client that meant to say
  something and instead silently reset the session is the failure being closed.
- Code grep of 10/14/26 over every lawnmower caller of this route: `console.py`
  `_answer_action` (via `session_resume.deliver`), `lawnmower-route` `resume_prompt`,
  `_batch_listing_prompt`, and `opp_followup_prompt` all build text whose first
  character is a letter. **No existing caller is refused by this rule.** That is the
  whole of why it can be added to a live route at all.
- The fixtures in PROGRESS.md keep the shape that matters: `resume.txt` holds a *later*
  line beginning `/compact`. The rule is about the first non-whitespace character of
  the whole text and nothing else, so that fixture is accepted. A rule scanning every
  line would refuse real traffic.
- The browser door is excluded because its route is the operator typing at their own
  session: `docs/security.md` records the session page's input panel as text or a key
  the operator sent, delivered once, when they pressed it. An operator typing
  `/compact` into their own session is the feature, not a mistake, and T004 asserts
  that it still arrives.

*Rejected: stripping or escaping the leading character.* It would deliver something
the caller did not write, into a session that executes code. A refusal the client can
see and fix is strictly better than a silent rewrite.

*Rejected: an allowlist of safe slash commands.* The daemon would then own a list that
tracks Claude Code's command set, and a stale list is a command delivered because
nobody updated a table. The route carries prompts; a caller that wants a command has
the browser door and the operator behind it.

This amends spec 018 **FR-006** a second time (see D4): the route gains one refusal
the interactive route does not have.

---

## D6 Lock and buffer (requirement 5)

**Decision.** `Manager.Prompt` takes the restart claim (`claimRestart`, non-blocking)
**after** `lockSession`, refuses with `ErrSessionNotReady` when a revive or a continue
already holds it, and releases it on return.

**Evidence.**

- Code: `lockSession` (`internal/session/input.go`) is already the per-session mutex
  shared by Type, PressKey, Prompt, Compact, SetMode, Continue, Destroy and the
  supervisor's Codex discovery. `Prompt` already takes it (`manager.go:1209`), so two
  concurrent prompts already cannot interleave paste and Enter.
- Code: `Exec.paste` already draws a fresh buffer name per paste (`newBufferName`,
  `exec.go:186-195`, 16 random hex digits behind `BufferPrefix`, "per call rather than
  per session, so a second paste into the same session cannot overwrite the first
  one's text before it lands"). The fake records the names through `nextBuffer`, so a
  test can assert they differ. Both halves of requirement 5's "distinct buffers, no
  interleaving" are therefore **already true**, from the spec 018 and 019 review
  passes, and this feature only has to prove it (T010) rather than build it.
- Code: `claimRestart` / `releaseRestart` (`manager.go:2674-2688`) are non-blocking and
  are what revival and continue hold while they type a new command line into the pane.
  That is the one concurrent writer `lockSession` does **not** exclude, because a
  restart does not hold the session lock for its whole run. A prompt pasted into a pane
  mid-restart lands partly in the dying process and partly on the shell the new command
  line is about to be typed at.

**Why non-blocking.** A prompt that waited for a revival would hold an HTTP request
open across a process restart, and the caller cannot tell a slow delivery from a lost
one. `409 session not ready` is a fact the client can act on now; lawnmower's
`deliver()` already reads it as a refusal (D3).

**Why after `lockSession` and not before.** The two are taken in one order everywhere
or they deadlock. `lockSession` first matches `Continue`'s order, so no new ordering
rule enters the package.

---

## D7 `request_id` (requirement 6)

**Decision.** The prompt body gains one optional field, `request_id`: 1 to 32
characters of `[a-z0-9-]`. A value outside that is a `400`. A valid value is echoed in
the `202` body and written as `request_id` on the request's one audit record. Absent
means the field appears nowhere — not in the response, not on the record.

**Evidence.**

- Design, requirement 6: the caller needs to correlate a delivery it made with the
  record of that delivery, across a daemon restart and without a transcript. The audit
  trail is the only durable record of a prompt, and today it carries nothing the caller
  chose.
- Code: `audit.Record` (`internal/audit/audit.go:390-414`) has Action, Caller,
  SessionID, Decision, Reason and Remote. `RequestAudit`
  (`internal/httpapi/middleware.go`) has `SetSessionID` and `Deny`, so the
  carry-it-onto-the-one-record mechanism exists and gains one setter (T002) rather
  than a second record.
- The value goes in a **field of its own** and never into `Reason`. `Record.Reason`'s
  own documentation forbids caller-supplied text in it ("must never be built from
  caller-supplied text, a prompt, pane output, a token, or the shared secret"), and
  that rule is unamended: `Reason` stays server-authored sentinels, and `request_id`
  is the one field on the record whose bytes the caller chose.
- `docs/security.md` §2 and the decoder it shows: the body is decoded into a typed
  struct with `DisallowUnknownFields`, so the field has to be declared to be accepted
  at all, and anything else in the body is still refused.

**Why that alphabet and that length.** The field is written to an audit record that an
operator reads and a tool greps. Lower-case, digits and `-` is the same shape this
repository already holds session names to (`^[a-zA-Z0-9-]{1,64}$`, `docs/security.md`
§2), minus the upper case; 32 characters fits a ULID or a UUID and leaves nothing for
a caller to smuggle. Validation is at the boundary and the refusal is a `400`, so no
tmux command runs for a body that was never going to be accepted.

**Why this is not a hole in FR-042.** Spec 001 FR-042 forbids prompt text, session
output, credentials and the shared secret on an audit record, and nothing in it
contemplated a caller-chosen correlation value. This amends FR-042 for **this one
field**, and the bound is exact: the record gains the `request_id` the caller sent and
**nothing about the prompt** — not its text, not its length, not a hash of it. The
alphabet is what keeps the field from becoming a channel for the text itself: 32 bytes
of `[a-z0-9-]` cannot carry a prompt, and a caller that tried would be writing a value
an operator can see is not an identifier. The amendment is recorded in spec.md.

---

## D8 Registry gate (requirement 7) — deferred, not a clarification

**Decision.** Requirement 7, a readiness gate reading Claude Code's session registry,
is **out of scope** for this feature. It is listed under Out of scope in spec.md with
this reason. It is **not** an open question in constitution II's sense — a deferred
MAY is a decision, and the marker that principle calls for is deliberately absent
from this directory. (Spelling that marker out here would also trip the gate that
greps this directory for it, which is why no file under `specs/020-prompt-route-safety/`
contains the phrase.)

**Evidence.**

- The design states requirement 7 as a **MAY**, not a MUST. Deferring a MAY is a scope
  decision, not an unanswered question, and constitution II is about requirements that
  are *ambiguous* — this one is clear and declined.
- Probe check 8 left open whether crswd can read `sessions/<pid>.json` at all under
  spec 017's v2 option B. No FR in spec 017 places that file in a pod, so a gate built
  on it would work on the VM and silently do nothing in the cluster — a readiness check
  that is right about one deployment and blind in the other.
- Probe checks 3 and 9: the registry misses the trust dialog and misses a half-typed
  draft in the composer. So it answers "a session exists" and not "this session will
  read what I send next", which is the question requirement 7 was asked to answer.
- The liveness gate (D2) is the part of requirement 1 that is measurable with what the
  daemon already holds, and it is what ships.

**What this costs, stated rather than smoothed over.** A session sitting on a dialog
still accepts a prompt; `paneDialogState` already reports that in the `202`
(`parked_on`, `suspicious_dialog`) and still does not refuse. That limit is the same
one spec 018 recorded, and closing it needs a readiness signal this daemon does not
have yet.

Also out of this feature by the same reasoning: **the Go code never opens a Claude
transcript or a `~/.claude/sessions/*.json` registry file.** Spec 013 FR-025 is
unchanged by this feature, and spec 019 D11's bounded Codex rollout read stays the
single exception it already is.

---

## D9 Out of scope

**Decision.** This feature does not do any of the following, and a task that finds
itself needing one stops rather than widening:

| Not done here | Why |
|---|---|
| Opening a transcript | Spec 013 FR-025 (D8) |
| Holding or calling an LLM | The daemon routes keystrokes; it does not reason about them |
| A seventh API route | The API door has exactly six (Code: `internal/httpapi/server.go`'s route switch; six `## <METHOD> <path>` headings in `specs/001-crswd-daemon-core/contracts/http-api.md`). Requirements 1 to 6 are all changes to one existing route |
| Free text on the browser door beyond spec 018 | Spec 018 owns that panel; D5 touches its route only to assert it is unchanged |
| The startup lost-Enter window | Probe check 1b: an Enter can be lost in the first ~5 s after launch. Two liveness reads around the paste do not see it, and closing it needs a start-up readiness signal (D8) |
| The socket transport | The probe's verdict ("flip to socket") binds the lm-steer-write milestone, not this one |
| A new package under `internal/` | Every change lands in `audit`, `httpapi`, `session` or `tmuxctl` |

The last two rows are also hard rules of the loop that implements this spec, and the
validation contract asserts the `internal/` one by diffing the tree against `main`.

---

## Not verified

Stated so that a later reader does not mistake any of these for a settled fact:

- Whether a pane can fall back to a shell *between* the second liveness read and the
  Enter. The window is two tmux commands wide and nothing measured it; the `C-u` of D2
  covers the measured case (an exit during the paste) and not this one.
- Whether `LivenessUnknown` ever appears for a session this daemon created. It is
  reachable only for a session created before `@crswd-binary` existed, and no such
  session was on the probe host.
- What a Codex session does with a prompt refused after its paste. `C-u` clears a
  Claude composer and a shell line; Codex's composer was not measured against it.
- Whether any non-lawnmower client of this route exists. The grep covered ai-lawnmower
  only, so D5's "no existing caller is refused" is a statement about that client.
