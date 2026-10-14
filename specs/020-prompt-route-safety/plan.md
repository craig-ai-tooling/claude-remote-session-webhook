# Implementation Plan: Prompt Route Safety

**Branch**: `plan/lm-steer-crswd-018` | **Date**: 2026-10-14 | **Spec**: [spec.md](spec.md)
| **Research**: [research.md](research.md) | **Tasks**: [tasks.md](tasks.md)

**Input**: Feature specification from `/specs/020-prompt-route-safety/spec.md`

**Notebook**: `ralph/plans/lm-steer-crswd-018/` — `IMPLEMENTATION_PLAN.md` is the loop's task
order and its file allowlist, `PROGRESS.md` its memory between iterations,
`VALIDATION_CONTRACT.md` its definition of done. This document is the design behind that
plan; where the two could drift, the allowlist below is reproduced from it verbatim.

## Summary

`POST /sessions/{id}/prompt` already carries lawnmower's `answer`, `resume`, opp-lease batch
handoff and opp follow-up into running sessions. It pastes and presses Enter without asking
whether anything is listening, so its failure mode is silent: text typed at a dead harness
becomes a command line on a login shell, and text typed beside a revive lands in the middle
of one.

This plan closes that on the route as it stands. Six changes, no new surface:

1. **A liveness gate** in `Manager.Prompt`, read from the controller's `List` before the
   paste and again before the Enter (research D2). No new `Controller` method, so
   `k8s/internal/podctl` is untouched.
2. **One sentinel**, `session.ErrSessionNotReady`, for every "not right now" refusal —
   `failed` state, stopped liveness, a held restart claim — mapped by the handler to one
   `409` with one body (D3).
3. **Bracketed paste for every harness** in `Prompt`; `Compact` keeps its harness-based
   paste (D4).
4. **A leading-character refusal** on the API door: first non-whitespace `!` or `/` is a
   `400` before any tmux command (D5). The browser's input routes are untouched.
5. **The restart claim** taken by `Prompt` after `lockSession`, non-blocking (D6).
6. **An optional `request_id`**, 1–32 of `[a-z0-9-]`, echoed in the `202` and written on the
   request's one audit record (D7).

Requirement 7 of the design — a readiness gate reading Claude Code's
`~/.claude/sessions/<pid>.json` — is deferred with the probe evidence that defers it (D8),
and is listed under the spec's Out of scope. It was a MAY; nothing downstream waits on it.

Every decision this plan implements was taken before the plan was cut and is recorded with
its evidence in [research.md](research.md) D1–D9. None is reopened here, and no question in
this document is left open.

## Technical Context

**Language/Version**: Go, as pinned by the root `go.mod`.

**Primary Dependencies**: none. The root module has no dependencies and no `go.sum`, and this
feature adds neither — stdlib `encoding/json`, `errors`, `fmt`, `strings` and the repo's own
`internal/audit`, `internal/httpapi`, `internal/session`, `internal/tmuxctl` are the whole of
it.

**Storage**: none new. Nothing here writes the session journal, the configuration file, or
any other durable state; the only persistent output is the audit trail, which gains one
bounded field.

**Testing**: `go test ./...` — table-driven, `t.Parallel()`, `tmuxctl.Fake` for every package
test. One new `//go:build tmux` file, `internal/session/prompt_tmux_test.go`, on a private
`-L` socket, following `internal/session/mode_test.go`. `go vet` on the default, `tmux` and
`quickstart` builds; `gofmt -l .`; `golangci-lint run`.

**Target Platform**: Linux host, tmux ≥ 3.4, loopback listener. The Kubernetes module in
`k8s/` is out of reach by construction: the gate reads `List`, which `podctl` already
implements.

**Project Type**: a single Go daemon — HTTP API and server-rendered dashboard in one binary.

**Performance Goals**: the gate adds one `List` before the paste and one after it, on a route
a caller invokes per steering message. No budget, no loop, no poll.

**Constraints**: no seventh API route; no new package under `internal/`; no new method on
`tmuxctl.Controller`; no new dependency and no `go.sum`; `AGENTS.md` unchanged and under 150
lines; nothing under `k8s/` touched. Prompt text, its length and a hash of it stay out of
every response, error, log line and audit record.

**Scale/Scope**: six behaviour changes on one route, thirteen implementation tasks across
four existing packages and `docs/security.md`.

## Constitution Check

*GATE: passed before the first Go change (notebook task G1). Re-checked when tasks.md is
written and again at F1.*

Assessed against `.specify/memory/constitution.md` v2.0.0.

| Principle | Assessment | Pass |
|---|---|---|
| **I — Security is a gate (NON-NEGOTIABLE)** | Authorization is unchanged and untouched: layer 3 still resolves `{id}` to a record the caller owns before the handler runs, and this feature adds no route and no exemption. Every new check *narrows* what the route will do. The one widening is `request_id` on the trail (FR-018, amending 001 FR-042), and its justification is the bound, not the use: 32 characters of `[a-z0-9-]` cannot carry prompt text, a sentence, a path or a credential, and FR-016 keeps text, length and hash forbidden, asserted by `internal/audit`'s leak test. No shell string is built — delivery still goes through `load-buffer` on stdin then `paste-buffer`, and the one new keystroke (`C-u`) is a daemon constant through the closed table. The `409` body is a constant; every refusal reason on the trail is a sentinel this repo authored. | ✅ |
| **II — Unknowns surfaced, never invented (NON-NEGOTIABLE)** | D1–D9 each carry the evidence they rest on, and research.md's "Not verified" section names the three things that were not measured — the registry file's reachability from a session pod, the exact width of the startup lost-Enter window, and whether a `LivenessUnknown` pane could be running a harness that swallows a paste. None of the three is depended on by a requirement here: the first is deferred with requirement 7, the second is Out of scope, and the third follows spec 012 FR-006 as the `tmuxctl.Liveness` type itself documents it and as `internal/session/supervisor.go` already reads it. Requirement 7's deferral is a recorded reason for a MAY, so it leaves nothing open. | ✅ |
| **III — Every change is verifiable** | Each FR maps to a success criterion (SC-001 to SC-011) and to a task that writes its test *before* the code, proving the test fails against the old build and recording the failure in `PROGRESS.md`'s "Failing-before evidence" table — one row per design requirement 1 to 6, counted by F1. The commands are the `AGENTS.md` commands, run in full before every commit. Two of them are environment-dependent on the loop's host and are named under Risks rather than assumed. | ✅ |
| **IV — Smallest correct change** | Nothing is refactored in passing. The gate is a read of an existing `List`; the sentinel joins the sentinels already in `internal/session`; the claim is the existing `claimRestart`; the buffer-per-paste and the per-session mutex are *asserted*, not rebuilt (FR-013), because both already exist from the 018 and 019 reviews. `Compact` is explicitly left alone. The two amendments are scoped to the two sentences they change and quote the FRs they amend. | ✅ |
| **V — Standards are enforced, not documented** | The new rules land as code and tests, not as prose: the gate and the refusals are in `Manager.Prompt`, the field validation is at the decode boundary, and `docs/security.md` gains a subsection (T013) that *describes* what the tests enforce. The fake gains `ExitAfterPaste` so the between-paste-and-Enter case is exercised by a test rather than reasoned about — the same shape as the existing `QuitAfterInterrupts`. No hook is changed, disabled, or routed around. | ✅ |
| **VI — Blast radius is bounded by construction (NON-NEGOTIABLE)** | Allowed roots, the concurrency cap, the absolute lifetime, verified teardown and the loopback bind are all untouched — this feature creates no session and destroys none. It is a net *reduction* in reach: refusing a prompt at a dead harness removes the path by which caller text became a command line on an unsandboxed login shell, and the post-paste `C-u` clears a half-delivered line instead of leaving it for a stray Enter. The restart claim removes the window in which a prompt could corrupt a revive's command line. Nothing here widens any of the six bounds, so the plan has no widening to name. | ✅ |
| **VII — Design system is binding** | No template, stylesheet, script or component changes. The session page's input panel keeps spec 018's behaviour exactly, including its acceptance of a leading `/` (FR-011). Pane output rendering is not on this route's path and is unchanged; the `409` body is a JSON constant on the API door, which gains no headers. | ✅ |

### Quality Gates, from the constitution's own list

- Build, test and lint green by the `AGENTS.md` commands — every iteration, before its commit.
- New behaviour has a test that fails without the change — the failing-before procedure, with
  its evidence table.
- `docs/security.md` satisfied for input, authz and secrets — this is an input-validation and
  refusal feature; T013 writes the subsection, and the checklist items that bite here are
  "input decoded into a typed struct with unknown fields rejected" (already true, extended by
  one field), "no shell string built", "auth failures are uniform and leak no detail", and "no
  session output, prompt, or token in any log line".
- `docs/auth-and-sessions.md` — not touched. No credential, door, cookie or lifecycle change.
- Cross-session isolation and verified teardown — unchanged; no session is created or killed.
- `docs/design-system.md` + `docs/components.md` — nothing visual changes.
- No question left open — G1 is the gate that asserts it before any Go change.
- Reviewed by a code owner — the PR after the loop, per the notebook's "After the loop".

## Project Structure

### Documentation (this feature)

```text
specs/020-prompt-route-safety/
├── spec.md          # FR-001 to FR-021, SC-001 to SC-011, Out of scope  (notebook P2, done)
├── research.md      # D1 to D9 with evidence, plus "Not verified"       (notebook P1, done)
├── plan.md          # this file                                         (notebook P3)
└── tasks.md         # T001 to T013, one entry per implementation task   (notebook P4)
```

No `data-model.md`: the feature adds one optional string field to one request body and one
optional string field to one audit record, both described where they are validated. No
`quickstart.md`: there is no new operator-facing workflow — the route a caller already uses
answers more precisely. No `contracts/` directory of its own: the API contract for this route
lives in `specs/001-crswd-daemon-core/contracts/http-api.md` and is amended there (notebook
P5, FR-019) rather than copied, so the six-route contract stays the one document a client
reads.

### Source code (repository root)

This is the notebook's allowlist, reproduced from
`ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` "Files touched". The loop may create
or edit these paths and no others, plus its own notebook directory.

```text
specs/020-prompt-route-safety/research.md             MOD   P1 wrote it; G1 may reword D8
specs/020-prompt-route-safety/spec.md                 MOD   P2 wrote it
specs/020-prompt-route-safety/plan.md                 NEW   P3, this file
specs/020-prompt-route-safety/tasks.md                NEW   P4
specs/001-crswd-daemon-core/contracts/http-api.md     MOD   P5: request_id, 409, the 400 rule
internal/audit/audit.go                               MOD   T001: Record.RequestID
internal/audit/audit_test.go                          MOD   T001: present and omitted
internal/audit/leak_test.go                           MOD   T001: the bounded field stays bounded
internal/httpapi/middleware.go                        MOD   T002: RequestAudit.SetRequestID
internal/httpapi/middleware_test.go                   MOD   T002
internal/httpapi/sessions.go                          MOD   T003–T005: field, 400 rule, 409 map
internal/httpapi/sessions_test.go                     MOD   T003–T005
internal/httpapi/lawnmower_payload_test.go            NEW   T011: the four fixtures, end to end
internal/httpapi/testdata/lawnmower/answer.txt        NEW   T011
internal/httpapi/testdata/lawnmower/resume.txt        NEW   T011
internal/httpapi/testdata/lawnmower/opp-lease-batch.txt NEW T011
internal/httpapi/testdata/lawnmower/opp-followup.txt  NEW   T011
internal/session/manager.go                           MOD   T005–T007, T009, T010
internal/session/manager_test.go                      MOD   T005–T007, T009, T010
internal/session/prompt_tmux_test.go                  NEW   T012, //go:build tmux
internal/tmuxctl/fake.go                              MOD   T008: ExitAfterPaste
internal/tmuxctl/fake_test.go                         MOD   T008
docs/security.md                                      MOD   T013: "Prompt route gates (spec 020)"
```

Out of scope, ever, in this milestone: `AGENTS.md`, anything under `k8s/`,
`internal/tmuxctl/controller.go`, `internal/tmuxctl/exec.go`, `go.mod`, and any new directory
under `internal/`.

**Structure Decision**: the existing layout, unchanged. Four packages already own the four
halves of this feature — `internal/session` owns everything that can cause execution on the
host, `internal/httpapi` owns the HTTP boundary and the one audit record, `internal/audit`
owns the record's shape, `internal/tmuxctl` owns the fake that makes the between-paste-and-
Enter case testable. A new package would be a fifth owner of a route that already has four,
which is why the notebook forbids one (FR-020). Tests stay colocated with the package they
cover, and the one test needing a real tmux is behind the `tmux` tag so `go test ./...` stays
hermetic.

## Design

### The gate, in `Manager.Prompt`

Order matters, and each step is placed where it is so the step after it is safe:

```text
ID empty            → ErrSessionNotFound        (today, unchanged)
State dead          → ErrSessionDead            (today, unchanged — frozen answer, D3)
text empty          → ErrEmptyPrompt            (today, unchanged)
State failed        → ErrSessionNotReady        T005
lockSession                                     (today)
claimRestart, non-blocking; held → ErrSessionNotReady   T010; released on every return
List → this session's Claude liveness
  stopped           → ErrSessionNotReady        T007, nothing typed
  unknown / running → continue                  T007
PasteBracketed                                  T006
List again
  stopped           → SendKeys C-u, no Enter, ErrSessionNotReady   T009
SendKeys Enter                                  (today)
```

The `failed` check sits before `lockSession` because it reads the record the caller was handed
and needs no serialisation; the two liveness reads sit inside both the lock and the claim
because what they assert — "nothing else is about to type here" — is only true while both are
held. A `List` that fails is wrapped and returned, never read as a pass.

`ErrSessionNotReady` joins the package's existing sentinels and carries all three refusals, so
`errors.Is` is the whole of the handler's branch. The three are told apart only server-side,
on the trail.

### The handler, in `internal/httpapi/sessions.go`

- `promptRequest` gains ``RequestID string `json:"request_id"` ``. `decode` already uses
  `DisallowUnknownFields`, so the field is accepted on this route and still refused on every
  other; validation is a length check and a character-class walk, and a failure is
  `rejectBadRequest` with the uniform body — before `paneDialogState`, so an invalid body
  costs the host not even a pane read.
- The leading-character refusal reads the first byte that is not `0x20`, `0x09`, `0x0D` or
  `0x0A` and refuses `!` or `/`, in the same place and the same way: `400`, uniform body, no
  tmux command. It is in the handler rather than in `Manager.Prompt` because that is what
  confines it to the API door — the session page's routes call `Type` and `PressKey`, not this
  handler, so FR-011 holds by construction rather than by a flag.
- `refusePrompt` gains one branch: `ErrSessionNotReady` → `409` with
  `{"error":"session not ready"}` and `AuditFrom(r.Context()).Deny("session not ready")`.
  `ErrEmptyPrompt` keeps its `400`; everything else keeps its `500` with no detail.
- `promptResponse` gains ``RequestID string `json:"request_id,omitempty"` ``, so a caller that
  sends none sees byte-for-byte the body it sees today. The response still carries no echo of
  the text.

### The audit record

`audit.Record` gains `RequestID`, written as `request_id` and omitted when empty.
`RequestAudit.SetRequestID` carries it onto the request's one record, which the middleware
already emits exactly once. The ordering rule the stream route lives under applies: the value
is set before anything emits. `internal/audit/leak_test.go` is extended rather than relaxed —
the field is bounded, and the test says so.

### The fake

`ExitAfterPaste(name)` sets the session's pane command to `bash` after the next `Paste` or
`PasteBracketed` on it, which makes "the harness exited between the paste and the Enter" a
state a table test can produce. It is the same shape as the existing `QuitAfterInterrupts`,
and it drives the liveness the fake already derives (`livenessOf` against
`OptionBinary`), so no second notion of liveness is introduced.

### The documentation

`docs/security.md` gains one subsection under "Non-negotiables", "Prompt route gates
(spec 020)", naming the liveness gate, the `failed` refusal, bracketed paste, the
leading-character rule, the restart claim and `request_id` — each with the reason it exists,
in that file's voice. `AGENTS.md` is not edited; T013's Verify asserts it stays under its
150-line CI bound.

## Task order and dependencies

The notebook's order is the dependency order, and the loop takes the topmost open task.

| Task | Depends on | Why that order |
|---|---|---|
| P1–P5, G1 | — | Markdown only. G1 gates every Go change on the spec having no open question. |
| T001 | G1 | The record's field has to exist before anything can set it. |
| T002 | T001 | `SetRequestID` writes the field T001 added. |
| T003 | T002 | The handler needs the setter before it can carry a value to the trail. |
| T004 | T003 | Both are decode-boundary refusals in the same handler; one after the other keeps each one's failing-before test honest. |
| T005 | T004 | Adds the sentinel and the first refusal that raises it, plus the handler's `409`. |
| T006 | T005 | Changes delivery. Separate from T005 so the bracketed-paste change has its own failing-before test. |
| T007 | T006 | The pre-paste gate, asserted against the paste shape T006 settled. |
| T008 | T007 | The fake hook T009 needs. |
| T009 | T008 | The post-paste gate, which cannot be tested without T008. |
| T010 | T009 | The claim, and the `-race` assertion of the lock and the buffer names. |
| T011 | T010 | The four real payloads, run through every gate once they all exist. |
| T012 | T011 | Real tmux, last, because it is the slowest and the most environment-dependent. |
| T013 | T012 | Documentation describes behaviour that is finished. |
| F1 | T013 | The final gate: both suites, the pre-commit commands, and the six evidence rows. |

## Risks

| Risk | Where it is handled |
|---|---|
| `golangci-lint` is absent on the loop's host, so a T-task cannot honestly tick that step | Recorded in `PROGRESS.md` "Findings" from iteration P1 and unchanged since. The binary gets installed outside the loop, or each affected iteration records the gap; `VALIDATION_CONTRACT.md` still requires `golangci-lint run` to exit 0 before the PR. It is an environment gap, not a code finding. |
| The loop cannot execute `safe-commit.sh`, so an iteration's paths stay uncommitted | Recorded in `PROGRESS.md` "Findings" for P1 and P2; P1's were picked up by a sweep commit (`0366fa0`). A T-task's failing-before evidence is only meaningful against a tree whose previous task is committed, so this is the one to fix before T001. |
| Bracketed paste changes Claude's delivery shape, which spec 018 froze | FR-017 amends 018 FR-006 in writing, quoting it. T006 carries the failing-before test that proves the change, and T011 proves the four real payloads still arrive after it. |
| A `List` on every prompt reads the whole server, not one session | Accepted deliberately (research D2): the alternative is a seventh `Controller` method, which would force a change in `k8s/internal/podctl`, a tree this milestone may not touch. |
| `LivenessUnknown` delivers, so a pane whose harness is unknowable is still typed into | The reading is spec 012 FR-006's, written on the `tmuxctl.Liveness` type and already relied on by `internal/session/supervisor.go`. research.md's "Not verified" names it as unmeasured rather than claiming otherwise. Reversing it would refuse every session started before `@crswd-binary` existed. |
| A future route reuses `request_id` as a general free-text field | FR-018's amendment is scoped to this one field with its bound stated, `internal/audit/leak_test.go` asserts the class, and T013 writes the bound into `docs/security.md` where the next author will read it. |

## Complexity Tracking

> Filled only when the Constitution Check has violations to justify.

No violations. Every principle passes above, and the three bounds that would otherwise need a
justification here — a new package, a seventh route, a new `Controller` method — are the
bounds FR-020 restates so a change cannot drift past them. The feature's whole cost is six
refusals and one bounded field on a route that already exists.
