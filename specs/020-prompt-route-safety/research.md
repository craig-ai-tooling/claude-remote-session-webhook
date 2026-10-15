# Research: Prompt route safety (spec 020)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · **Tasks**: [tasks.md](tasks.md)

This milestone hardens `POST /sessions/{id}/prompt`, the one route lawnmower's board
`answer` and its opp-lease handoff already drive, so that a voice `steer` can use it
later without a second route and without a second set of rules.

Every decision below was taken while planning, on 10/14/26, and is **closed**. This
file records each one with the evidence it rests on; it does not reopen any of them.
Two sources are outside this repository and are read-only:

- The design: `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`, section (g)
  "Phase 3 detail", which states requirements 1 to 7.
- The probe: `~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`, whose numbered
  checks are cited below as "probe check N".

Facts about this repository were measured on 10/14/26 against `main` at 1eeb59c, which
is also the revision the daemon on this host was running when the plan was cut. They
are recorded in `ralph/plans/lm-steer-crswd-018/PROGRESS.md` under "Facts measured while
planning" and are not re-measured per task.

## D1 — The spec number is 020

**Question.** The design calls this work "crswd spec 018". Which directory does it get?

**Evidence.** `specs/` already holds `018-interactive-input` and `019-codex-runtime`
(measured 10/14/26). Reusing either number would put two specs behind one identifier,
and the amendments in D5 and D7 need to name the spec they amend unambiguously.

**Decision.** The spec lives at `specs/020-prompt-route-safety/`. The design's "crswd
spec 018" means this work, not spec 018 in this repository, and the two are referred to
by their own numbers everywhere in this spec.

## D2 — The liveness gate reads `List`, before and after the paste (req 1)

**Question.** A pane whose Claude has exited falls back to its login shell. A paste into
it is a line typed at a shell, and the Enter after it runs that line. What tells the
daemon the harness is still there, and when?

**Evidence.**

- Probe check 13: the session option `@crswd-binary` matched `pane_current_command`
  (`claude`) on **6 of 6** live panes, 3 of them `rcq` sessions. So the match the daemon
  already computes is a usable liveness signal on the real host, not just in the fake.
- `tmuxctl.Controller.List` already derives a per-session `Claude` liveness from that
  option against the pane command — `livenessFrom` / `livenessOf`, with three values,
  `LivenessRunning`, `LivenessStopped` and `LivenessUnknown` (`internal/tmuxctl/controller.go`
  around line 312, asserted by `TestLivenessUnknownIsAlive`).
- There is **no** per-session liveness method on `Controller`, and adding one would force
  a matching change in the second module's `k8s/internal/podctl`. The loop's rules forbid
  adding a method to that interface, and `k8s/` is out of scope for this milestone.
- Probe check 1: after the first turn there is no check-then-Enter race worth defending
  against; a lost Enter was observed only in the first ~5 s after launch. That window is
  out of scope (D9), so the gate is about the harness being *gone*, not about it being
  momentarily busy.

**Decision.** The gate reads the session's row from the controller's existing `List` and
looks at its `Claude` liveness. `LivenessStopped` refuses with `ErrSessionNotReady` and
pastes nothing. `LivenessUnknown` **passes**, which is how spec 012 FR-006 already reads
that value — a session started by a build before the option existed is not evidence of a
dead harness, and refusing it would break sessions the daemon adopted. A `List` that
fails is an error, not an implicit pass: this is an auth-adjacent bound and it fails closed.

It runs **twice**: once before the paste, and again after the paste and before the Enter.
If the second read is stopped, the daemon sends `C-u` to clear whatever the shell now has
on its line, sends **no** Enter, and returns `ErrSessionNotReady`. The second read is what
makes the window between the two tmux commands survivable; `C-u` without Enter is what
keeps the pasted text from being a command the next Enter runs.

**Rejected.** A new `Controller` method for one session's liveness. It is a smaller read,
but it widens the interface the pod controller must satisfy for a route that pod
controller does not serve.

## D3 — One sentinel, `ErrSessionNotReady`, answered 409 (reqs 1, 2, 5)

**Question.** Three different refusals — a stopped pane (D2), a `failed` session, and a
held restart claim (D6) — all mean "not now". How many answers does the caller get?

**Evidence.**

- `refusePrompt` in `internal/httpapi/sessions.go` today maps `ErrEmptyPrompt` to 400 and
  everything else to 500. A 500 is this daemon claiming a fault of its own for a session
  state it understood perfectly well.
- lawnmower's `deliver()` (`~/code/ai-lawnmower/loop/session_resume.py`, 10/14/26) already
  treats any response body carrying an `error` key as a refusal, so a 409 with
  `{"error":"session not ready"}` reads correctly at the existing caller with no change
  there.
- `docs/security.md` §1: refusals are uniform and leak no detail about which check failed.
  Three distinguishable bodies would tell a caller which of the three states a session it
  may not read is in.

**Decision.** One sentinel, `session.ErrSessionNotReady`. The handler maps it to **409**
with body `{"error":"session not ready"}` and one audit **Deny** whose reason is the
sentinel's own text, `session not ready`. A session in state `failed` gets it. `dead`
keeps today's answer and today's sentinel (`ErrSessionDead`) — that path is shipped, a
client reads it, and changing it is not in this milestone.

The reason on the trail is a sentinel this codebase authored, so the record cannot carry
a byte the caller chose (`docs/security.md`, "Rate limiting & audit").

## D4 — `Prompt` pastes bracketed for every harness; `Compact` does not change (req 3)

**Question.** `Manager.paste` chooses bracketed paste only when the harness spec says
`BracketedPaste`, and Claude's spec does not. So a multi-line prompt to a Claude session
has its first LF typed as a CR, which submits line one and leaves the rest typed.

**Evidence.**

- Spec 018 research R1, measured on tmux 3.4 with Claude Code 2.1.290: without `-p`, a
  two-line paste into Claude **submitted line one** and left line two in the input box.
  With `-p` both lines were held, unsent, as one message.
- Spec 019 M3 and M4 found the same for Codex: `paste-buffer -d` plus Enter does not
  submit, `paste-buffer -p -d` plus Enter does, for one line and for two.
- The opp-lease handoff pastes multi-line item bodies — `opp-lease-batch.txt` and the
  other three fixtures in PROGRESS.md "Payload fixtures" are all multi-line. This is the
  ordinary case on this route, not an edge.
- `Manager.Compact` pastes the constant `/compact\n` and **depends** on that LF becoming a
  CR to submit itself (spec 018 R1's rejected option (a) names exactly this).

**Decision.** `Manager.Prompt` calls `PasteBracketed` for every harness, unconditionally,
rather than going through the harness-dependent `m.paste`. `Manager.Compact` keeps its
harness-based paste and is not touched.

**No newline refusal.** Spec 018 FR-004 refuses C0 controls other than TAB and LF inside
a bracketed paste, because `ESC[201~` would end the paste; LF stays allowed, and that is
what the handoff needs.

## D5 — A leading `!` or `/` is refused on the API door (req 4)

**Question.** Claude Code reads a leading `/` as a slash command and a leading `!` as a
bash escape. A prompt that begins with either is not a message to the assistant; it is a
command to the harness, delivered by whoever can reach this route.

**Evidence.**

- Grep of lawnmower's prompt builders on 10/14/26 — `console.py` `_answer_action` (via
  `session_resume.deliver`), `lawnmower-route` `resume_prompt`, `_batch_listing_prompt`,
  and `opp_followup_prompt` — every one of them starts with a **letter**. No live caller
  is broken by this refusal.
- The fixtures in PROGRESS.md keep the shapes that do occur: a later line beginning `/`
  (`resume.txt`: `/compact only if the context gets long`), double quotes, `$`, `;`, `&`
  and a backtick. So the rule has to be about the **first** non-whitespace character and
  nothing else, or it refuses traffic that works today.
- The browser's type route is spec 018's input panel: an operator pressing a key in a
  panel the daemon drew, who may legitimately want to send `/compact`. Refusing there
  would remove a thing the operator can already do by attaching to the pane.

**Decision.** API door only. Find the first character that is not space, tab, CR or LF;
if it is `!` or `/`, answer **400** with the uniform bad-request body, before any tmux
command runs. The browser type route is unchanged.

**This amends spec 018 FR-006**, which says the API door's prompt route must be
unchanged. That sentence was written to keep 018 from touching a shipped route; it is
superseded here, deliberately and in one named place.

## D6 — The per-session lock stays, and `Prompt` takes the restart claim (req 5)

**Question.** Two prompts arriving at one session at the same moment, and a prompt
arriving while a revive or a continue is retyping the start command. What stops a paste
from landing between another delivery's paste and its Enter?

**Evidence.** Most of this already exists, from the 018 and 019 reviews:

- `lockSession` (`internal/session/input.go`) is the per-session mutex already shared by
  Type, PressKey, Prompt, Compact, SetMode, Continue, Destroy and the supervisor's Codex
  discovery. `Manager.Prompt` already takes it (`internal/session/manager.go`, around
  line 1209).
- `Exec.paste` (`internal/tmuxctl/exec.go`) already makes a fresh buffer name per paste
  via `newBufferName`, and the fake records those names through `nextBuffer`. So two
  concurrent deliveries cannot read each other's buffer.
- `claimRestart` / `releaseRestart` (`internal/session/manager.go`, around line 2663) are
  already non-blocking and already guard the single restart.

**Decision.** New here, and only this: `Prompt` takes the restart claim with
`claimRestart` **after** `lockSession`, refuses with `ErrSessionNotReady` if a revive or
a continue holds it, and releases it on return. Nothing else about the lock or the buffer
naming changes.

The order matters and is not arbitrary: the lock is held first, so the claim is attempted
by one delivery at a time, and a refusal is about a restart rather than about another
prompt. A prompt is refused rather than queued because a revive retypes the start command
— text pasted across that is text typed at whatever the pane is mid-restart.

## D7 — An optional `request_id`, echoed and audited (req 6)

**Question.** A caller that delivers a prompt and later reads the trail has no way to tie
the two together. The session ID is not enough: lawnmower sends several prompts to one
session.

**Evidence.**

- `audit.Record` (`internal/audit/audit.go`) carries Action, Caller, SessionID, Decision,
  Reason and Remote, and nothing caller-chosen beyond the session ID. `RequestAudit`
  (`internal/httpapi/middleware.go`) exposes `SetSessionID` and `Deny`, so there is a
  shape to copy for a per-request value.
- Spec 001 FR-042 forbids caller-supplied free text on the trail, for the reason
  `docs/security.md` gives: a record must never carry a byte the caller chose. A 32-character
  `[a-z0-9-]` field is not free text — the character class and the bound are what make it
  a correlation handle rather than a channel.

**Decision.** An **optional** JSON field `request_id`, 1 to 32 characters of `[a-z0-9-]`.
A value outside that is **400**. A valid value is echoed in the 202 body and written as
`request_id` on the audit record. Absent means the field appears nowhere — not in the
body, not on the record.

**This amends spec 001 FR-042 for this one field**, and for no other. The prompt text
itself is unchanged by this: no length, no hash, no prefix of it reaches an error, a log
line or a record, which is `docs/security.md` §3 and the loop's own standing rule.

## D8 — The registry gate (req 7) is deferred, and that is not a clarification

**Question.** Requirement 7 asks whether the daemon should consult Claude's session
registry (`~/.claude/sessions/<pid>.json`) before delivering, to see whether the session
is idle.

**Evidence.**

- It is a **MAY** in the design, not a MUST.
- Probe check 8 left open whether crswd can read `sessions/<pid>.json` at all under v2
  option B, and no functional requirement in spec 017 places that file in the k8s
  execution model. So the gate has no defined behaviour for a pod-backed session.
- Probe checks 3 and 9: the registry **misses** the trust dialog and a half-typed draft.
  A gate built on it would report ready for two states in which a delivery is wrong,
  which is worse than no gate — it is a check that lies, and the liveness gate in D2
  already covers the state that matters here.

**Decision.** Out of scope, listed as such in spec.md with that reason. It is **not** a
`NEEDS CLARIFICATION`: constitution principle II is about an ambiguity that would be
guessed at, and this is a MAY that was evaluated against probe evidence and declined.
Reopening it needs the spec 017 question in probe check 8 answered first.

## D9 — What this milestone does not do

Named here so that a later iteration does not read silence as permission:

- **Opening a transcript**, or any `~/.claude/sessions/*.json` registry file, from Go.
  Spec 013 FR-025 is unchanged, and D8 is why the registry stays unread here.
- **Holding or calling an LLM.** This route delivers bytes an operator or a caller chose.
- **A seventh API route.** The door has six (`internal/httpapi/server.go`'s route switch;
  contract headings at `specs/001-crswd-daemon-core/contracts/http-api.md` lines 128 to
  304), and the count is asserted.
- **Free text on the browser door beyond spec 018.** The input panel is the whole of it.
- **The startup lost-Enter window** (probe check 1b): the first ~5 s after a launch, where
  an Enter can be swallowed. Real, measured, and a different fix.
- **The socket transport.** The probe's verdict "flip to socket" binds lm-steer-write, not
  this milestone.
- **A new package under `internal/`**, and no change to `k8s/`, `AGENTS.md`,
  `internal/tmuxctl/controller.go`, `internal/tmuxctl/exec.go` or `go.mod`. The root
  module gains no `go.sum` (`docs/security.md` §5).
