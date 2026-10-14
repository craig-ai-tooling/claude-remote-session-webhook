# Research: prompt route safety

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · **Tasks**: [tasks.md](tasks.md)

The requirements come from `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`,
section (g) "Phase 3 detail", and the evidence from the probe run recorded in
`~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`. Both live outside this
repository and are read, never edited. Everything cited below as a *probe check* is a
numbered check in that results file; everything cited as *measured here* was read off
this tree on 10/14/26 and is restated in `ralph/plans/lm-steer-crswd-018/PROGRESS.md`
so a later iteration need not re-measure it.

The decisions were taken before the plan was cut. They are recorded here in full so the
spec, the plan and the task list all cite one source. **They are not reopened by this
spec** — a decision that needs changing needs a new one, with its own evidence.

## D1 — Spec number

The spec lives at `specs/020-prompt-route-safety/`.

The design calls this work "crswd spec 018". That number is taken here: `specs/018-interactive-input`
is the session page's input panel and `specs/019-codex-runtime` is the Codex runtime
(measured here: both directories exist and carry a `spec.md`). The next free number is 020.

*Rejected: reusing 018 because the design names it.* A spec directory is addressed by
number from `tasks.md`, from `docs/`, and from the two specs that this one amends; two
specs under one number is a reference that resolves to the wrong document.

## D2 — Liveness gate (requirement 1)

`Manager.Prompt` reads the session's row out of the controller's `List` and consults its
`Claude` liveness before it pastes, and again after the paste:

- `LivenessStopped` returns `ErrSessionNotReady` and nothing is typed.
- `LivenessUnknown` delivers. That is the reading `tmuxctl.Liveness` documents for
  every caller — it is the zero value precisely so that a caller who forgets the field
  reads "alive" — and it is how spec 012's detection requirement (FR-006) is already
  satisfied in `internal/session/supervisor.go`: an unknown liveness is a session
  started before `@crswd-binary` existed, not evidence of a stopped harness.
- A failing `List` is an error, not a pass. The gate fails closed.
- If the *second* read is stopped — Claude exited between the paste and the submit — the
  daemon sends `C-u` and no Enter, so the half-delivered text is cleared off the shell's
  line rather than executed by it, and the call still answers `ErrSessionNotReady`.

Evidence: probe check 13 — the `@crswd-binary` tmux option matched `pane_current_command`
(`claude`) on 6 of 6 live panes, 3 of them under `rcq`. So the option this daemon already
sets is a usable liveness answer on the panes lawnmower actually drives, and the gate
needs no new fact on the host.

Measured here: the fake controller's `List` already derives `Claude` liveness from
`@crswd-binary` against the pane command (`livenessOf`), and the fake has
`SetPaneCommand(name, command)` to drive it.

*Rejected: a per-session liveness method on `tmuxctl.Controller`.* There is none today,
and the interface has a second implementation — `k8s/internal/podctl` — which a seventh
method would force to change. `k8s/` is out of this milestone's reach by construction, so
the gate reads the row it can already get.

*Rejected: trusting the stored session state alone.* A pane whose harness exited is still
`running` in the store until the next supervisor sweep; the whole point of the gate is to
answer for the pane as it is at the moment of the paste.

## D3 — Not-ready answer (requirements 1, 2, 5)

One sentinel, `session.ErrSessionNotReady`, carries every "the session cannot take a
prompt right now" refusal. The prompt handler maps it to `409` with body
`{"error":"session not ready"}` and records an audit Deny whose reason is the sentinel's
text, `session not ready`.

A session in state `failed` is refused with it before any tmux command runs. `dead` keeps
the answer it gives today (`ErrSessionDead`), because that is a frozen client-visible
behaviour and nothing in this spec asks to change it.

Evidence: lawnmower's `deliver()` (`~/code/ai-lawnmower/loop/session_resume.py`) already
treats any response body carrying an `error` key as a refusal, so a 409 of this shape
reads as a refusal to the only caller this route has today without a change on that side.

One sentinel rather than three keeps the refusal uniform: the caller learns that the
session is not ready and not *which* of the gates said so. The three are distinguished
only server-side, on the trail.

## D4 — Bracketed paste (requirement 3)

`Manager.Prompt` pastes with `PasteBracketed` for every harness. `Manager.Compact` keeps
its harness-based paste and is untouched.

Evidence: spec 019 M3/M4 measured that a plain `paste-buffer -d` plus Enter does not
submit on Codex while `paste-buffer -p -d` plus Enter does, for one line and for two;
spec 018 added `Controller.PasteBracketed` and spec 019 D9 left "whether Claude moves to
bracketed paste too" to spec 018's successor. This is that decision, and it is taken for
both harnesses so that one route has one delivery shape. Spec 018 FR-006 reads "plain
paste, then Enter. Bracketed paste is for the interactive route only", so **this decision
amends that FR** as well as D5 does; the amendment is recorded in spec.md. Bracketed paste is also what makes the C0 refusal
in `docs/security.md` §2 load-bearing rather than decorative: inside a bracketed paste a
literal `ESC[201~` would end the paste, and that text is already refused.

**No newline refusal.** The opp-lease handoff pastes multi-line item bodies (the fixtures
in `ralph/plans/lm-steer-crswd-018/PROGRESS.md` are multi-line as lawnmower builds them
today), so refusing `\n` would refuse the traffic this route exists to carry.

Measured here: `m.paste` is bracketed only when the harness spec says `BracketedPaste`,
which Claude's does not — so this is a behaviour change for Claude sessions and needs the
failing-before test that proves it.

## D5 — A leading `!` or `/` is refused (requirement 4)

On the **API door only**, a prompt is refused when the first character that is not a
space, a tab, a CR or an LF is `!` or `/`: `400` with the uniform bad-request body,
before any tmux command runs.

`/` opens a Claude Code slash command and `!` opens its shell escape. A caller that can
reach this route can already run code on the host through a session, so this is not a
containment boundary; it is a refusal of the two prefixes that make a *delivered prompt*
mean something other than a prompt — a steering message that silently became `/clear` is
a lost conversation with no error anywhere.

Evidence: a grep of lawnmower's callers on 10/14/26 — `console.py` `_answer_action` (via
`session_resume.deliver`), `lawnmower-route` `resume_prompt`, `_batch_listing_prompt` and
`opp_followup_prompt` — found that every builder's text starts with a letter. A *later*
line starting with `/` does occur (the `resume.txt` fixture has one), which is why the
rule reads only the first non-whitespace character of the whole text and never a line.

**The browser type route is unchanged.** The operator typing into the session page's input
panel is a human at their own keyboard who may legitimately want `/compact`, and a test in
this spec asserts that route still delivers it. Spec 018 FR-006 says the API door's prompt
route is unchanged, so this decision **amends that FR** too, for this one refusal.

## D6 — The lock and the buffer (requirement 5)

`Prompt` takes the restart claim (`claimRestart`, non-blocking) immediately after
`lockSession`, refuses with `ErrSessionNotReady` when a revive or a continue already holds
it, and releases it on return.

Measured here, and therefore *not* new work in this spec:

- `lockSession` (`internal/session/input.go`) is already the per-session mutex shared by
  `Type`, `PressKey`, `Prompt`, `Compact`, `SetMode`, `Continue`, `Destroy` and the
  supervisor's Codex discovery. Two concurrent prompts to one session already serialise.
- `Exec.paste` (`internal/tmuxctl/exec.go`) already makes a fresh buffer name per paste
  via `newBufferName`, so two pastes cannot clobber one buffer. The fake records the
  names it was given (`nextBuffer`), so a test can assert they differ.

Both came out of the spec 018 and 019 reviews. What is new here is only the claim: a
revive or a continue is *typing a command line into the pane*, and a prompt pasted into
the middle of that line is neither a prompt nor a command. The claim is non-blocking on
purpose — a prompt that waited for a restart would be delivered into a session whose
conversation has since been replaced, which is worse than a refusal the caller can retry.

## D7 — `request_id` (requirement 6)

The prompt body gains one optional field, `request_id`: 1 to 32 characters of `[a-z0-9-]`.
A value outside that is `400`. A valid value is echoed in the `202` body and written as
`request_id` on the audit record. Absent means the field appears nowhere — not in the
response, not on the trail.

This **amends spec 001 FR-042** ("audit records carry no caller-supplied free text") for
this one field, and the bound is what makes the amendment safe to take: 32 characters of
`[a-z0-9-]` cannot carry prompt text, a sentence, a path, or a credential, and the
character class keeps it out of trouble in a JSONL record and in a log line. The
amendment is recorded in spec.md and in `specs/001-crswd-daemon-core/contracts/http-api.md`.

What it buys: lawnmower delivers a prompt and then reads the trail to find out what
happened to it. Without a correlator it matches on session ID and time, which is ambiguous
for exactly the case that matters — two deliveries to one session in one second.

**No length and no hash of the prompt**, here or anywhere else. The rule in
`docs/security.md` §3 stands unchanged: a length is a disclosure and a hash is a
disclosure of a guessable value.

## D8 — The registry gate (requirement 7) is deferred

Requirement 7 — consult Claude Code's `~/.claude/sessions/<pid>.json` registry to decide
whether a session is ready — is **not built**, and the spec lists it under Out of scope
with this reason. It was a MAY in the design, not a MUST.

Evidence, all from the probe:

- Check 8 left open whether crswd can read `sessions/<pid>.json` at all under v2
  option B. No FR in spec 017 places that file in the cluster deployment, so a gate
  reading it would work on the VM and silently answer nothing in Kubernetes.
- Checks 3 and 9: the registry misses the trust dialog and misses a half-typed draft.
  So it does not answer the question the liveness gate (D2) answers, and a gate built on
  it would pass a session that cannot take a prompt.

A deferred MAY with a recorded reason is **not an open question** under the
constitution's principle II, and nothing downstream waits on it. The requirement is
answered — the answer is "not built, for these two reasons" — so there is nothing here
for an iteration to stop and ask about.

## D9 — Out of scope

Named here so that a later iteration does not read one of these as missing work:

- **Opening a transcript.** The no-transcript rule holds (FR-025, recorded in
  `docs/security.md` as spec 013 FR-025); the bounded Codex exception (spec 019 D11) is
  that spec's and is not widened here.
- **Holding or calling an LLM.** This daemon delivers text; it does not author it.
- **A seventh API route.** The API door has six (measured here: the route switch in
  `internal/httpapi/server.go`, and six `## <VERB> <path>` headings in the contract).
  Every change in this spec lands on the existing prompt route.
- **Free text on the browser door beyond spec 018.** The session page's input panel is
  that spec's and keeps its behaviour, D5 included.
- **The startup lost-Enter window.** Probe check 1 found no check-then-Enter race after
  the first turn, and a lost Enter only inside roughly the first 5 s after a launch. That
  window is real and is not closed here.
- **The socket transport.** The probe's verdict was "flip to socket"; it binds the
  lm-steer-write milestone, not this one.
- **A new package under `internal/`.** Every change lands in `audit`, `httpapi`,
  `session` or `tmuxctl`.

## Not verified

- Whether Claude Code's registry file is reachable from a session pod (probe check 8,
  deferred with requirement 7 under D8).
- The exact width of the startup window in which an Enter can be lost; probe check 1
  bounds it at roughly 5 s and this spec does not depend on the number.
- Whether a `LivenessUnknown` pane can be running a harness that would swallow a paste.
  D2 delivers on unknown by spec 012 FR-006; no probe check contradicts it.
