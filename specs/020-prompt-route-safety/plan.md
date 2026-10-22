# Implementation Plan: Prompt Route Safety

**Branch**: `plan/lm-steer-crswd-018` | **Date**: 2026-10-15 | **Spec**:
[spec.md](spec.md) | **Research**: [research.md](research.md) | **Tasks**:
[tasks.md](tasks.md)

## Summary

`POST /sessions/{id}/prompt` is shipped and has live callers — lawnmower's board
`answer` and its opp-lease handoff. This milestone hardens it **in place**: six
gates and one optional field, no new route, no new package, no change to the
`tmuxctl.Controller` interface.

What changes, in the order the request meets it:

- the handler refuses a `request_id` outside `[a-z0-9-]{1,32}` and a prompt whose
  first non-whitespace character is `!` or `/`, both `400`, before anything
  reaches tmux;
- `Manager.Prompt` refuses a `failed` session, a session whose harness liveness
  reads *stopped*, and a session whose restart claim a revive holds — all three
  with one sentinel, `ErrSessionNotReady`, answered `409`
  `{"error":"session not ready"}`;
- the paste becomes **bracketed for every harness**, so the multi-line payloads
  every live caller sends arrive as one message instead of submitting line one;
- liveness is read **again** after the paste and before the Enter: *stopped* there
  means `C-u`, no Enter, `409`;
- a valid `request_id` is echoed in the `202` and written on the request's one
  audit record.

The liveness signal, the per-session mutex, the fresh-buffer-per-paste and the
non-blocking restart claim all exist already (research.md D2, D6). This plan wires
them into one route; the genuinely new code is a sentinel, a field, a character
check and a second liveness read.

## Technical Context

**Language**: Go, standard library only in the root module — it has no `go.sum`
and gains none (`docs/security.md` §5). **Storage**: none; the session journal is
untouched. **Testing**: table-driven, `t.Parallel()`, `tmuxctl.Fake` for
everything except the one `//go:build tmux` file this plan adds, which drives a
real tmux 3.4 on a private `-L` socket (pattern: `internal/session/mode_test.go`).
`go test -race` for the concurrency case. **Measured inputs**: probe checks 1, 3,
8, 9 and 13; spec 018 research R1 (bracketed paste, Claude Code 2.1.290); spec
019 M3 and M4 (Codex 0.153.4). All are recorded in [research.md](research.md) with
the decision each one settles. **Out of reach from the loop's container**: the two
external source documents and `golangci-lint`; see Risks.

## Constitution Check

*GATE: passed before the first Go change (task G1 re-asserts it from the files).*

| Principle | Assessment | Pass |
|---|---|---|
| **I — Security is a gate** | Every gate here *narrows* the route. Authentication, the per-session credential and the ownership check are untouched: the resolver still settles which session this is, and the handler still reads neither the path nor the body for it. The two new `400`s run before any tmux command. The three not-ready states are indistinguishable in body, status and reason, so a caller learns nothing about a session it may not read. No shell string is built — the text still travels as `load-buffer` on stdin and the Enter and `C-u` are daemon constants. The `409` body and the audit reason are the sentinel's own text, authored in this repo. | ✅ |
| **II — Unknowns surfaced** | Nine decisions, each closed on evidence in research.md; none is reopened here. Requirement 7 is **declined on probe evidence**, not guessed at and not deferred for want of an answer (D8), and spec.md says what would have to be answered to reopen it. The one residual this milestone does not fix — the `ESC[201~` marker inside a bracketed paste — is named in spec.md FR-019 with what is known and what is not, rather than being quietly fixed or quietly ignored. No artifact in this feature leaves an open question behind — G1 asserts that from the files before the first Go change. | ✅ |
| **III — Verifiable** | Every FR lands on a status code, an argv the fake recorded, a key sequence on a real tmux, an audit record a test reads, or a line of `http-api.md`. Each of requirements 1 to 6 gets a test that **fails against the code before its change**, with both results written into the notebook's "Failing-before evidence" table (SC-001), and F1 refuses to close the milestone until there are six rows. | ✅ |
| **IV — Smallest correct change** | No new route, no new package, no new `Controller` method, no new dependency. The gate reads the `List` the supervisor already calls; the lock, the buffer naming and the restart claim are reused as they stand. `Manager.Compact`, the browser type route, the `dead` path and the milestone 1 response shape are left exactly as they are — the last by omitting `request_id` when it is absent. | ✅ |
| **V — Standards enforced** | The audit leak suite gains the new field as a canary rather than being relaxed (SC-005); the six-route assertion and the contract's six headings stay as they are and are counted (SC-006). The fake gains `ExitAfterPaste` so the post-paste gate is provable in the default build instead of only under `-tags tmux`. | ✅ |
| **VI — Blast radius bounded** | This is the principle the milestone exists for. A pane whose harness has exited is a login shell, and today's paste-then-Enter makes a caller's prompt a command line that shell runs. FR-003 to FR-005 close that: refuse before, re-read after, and `C-u` with no Enter if it moved under us. Allowed roots, the session cap, lifetimes, the reaper and the loopback bind are untouched. Nothing here widens any bound; `request_id` is the only new caller-chosen byte anywhere, and it is bounded to 32 characters of `[a-z0-9-]`. | ✅ |
| **VII — Design system** | No template, no stylesheet, no script, no component. The browser door is explicitly unchanged (FR-013). | ✅ |

## Design

### `internal/audit` — one new field

`Record` gains `RequestID`, written as `request_id` and **omitted when empty**, so
every record the daemon writes today is byte-identical. It is the only
caller-chosen value on the trail besides the session ID, and what makes that
admissible is the character class and the bound the handler enforces before it
gets here (spec.md FR-002, FR-017).

### `internal/httpapi` — the handler half

- `RequestAudit.SetRequestID(string)`, shaped exactly like `SetSessionID`: carries
  the value onto the request's one record, nil-safe, no second record.
- `promptRequest` gains `RequestID string \`json:"request_id"\``. The decoder still
  runs with `DisallowUnknownFields`, so no other field becomes acceptable.
- `promptResponse` gains `RequestID string \`json:"request_id,omitempty"\`` —
  `omitempty` is the contract-compatibility property, not a style choice.
- Two validations in `promptSession`, both before `paneDialogState` and therefore
  before anything reads or touches the pane: the `request_id` class and bound, and
  the first-non-whitespace-character rule. Each refuses through
  `rejectBadRequest`, which already writes the uniform `{"error":"bad request"}`
  body and an audit deny whose reason is the sentinel passed to it — so both gates
  get their trail entry from existing code, and both reasons are errors authored
  here.
- `refusePrompt` gains one branch: `errors.Is(err, session.ErrSessionNotReady)` →
  `409` with `{"error":"session not ready"}` and that reason on the record.
  Everything else still falls to the detail-free `500`.

### `internal/session` — the delivery half

`Manager.Prompt` becomes, in this order:

1. today's refusals: empty ID, `StateDead`, empty text — unchanged, so `dead`
   keeps its shipped answer;
2. `StateFailed` → `ErrSessionNotReady`, before any tmux command;
3. `lockSession(s.ID)`, as today;
4. `claimRestart(s.ID)` — non-blocking; held → `ErrSessionNotReady`, with
   `releaseRestart` deferred on every path out (FR-014);
5. the liveness read: one `m.tmux.List(ctx)`, find the row whose `Name` is
   `s.TmuxName()`, and refuse on `tmuxctl.LivenessStopped`. `LivenessUnknown` and
   a row that is absent both pass, which is how `Supervisor.judge` already reads
   that value; a `List` that errors is returned wrapped, never an implicit pass;
6. `m.tmux.PasteBracketed(ctx, name, []byte(text))` — unconditionally, not through
   `m.paste`, whose harness-dependent choice stays for `Compact` alone;
7. the second liveness read. *Stopped* → `SendKeys(ctx, name, "C-u")`, no Enter,
   `ErrSessionNotReady`;
8. `SendKeys(ctx, name, enterKey)`, as today.

`ErrSessionNotReady` is one new sentinel with the text `session not ready` — the
same text the `409` body and the audit reason carry, so the three cannot drift
apart.

`Manager.paste`, `Manager.Compact`, `specOf` and the harness specs are untouched.

### `internal/tmuxctl` — the fake only

`Fake.ExitAfterPaste(name)` arms a session so that the next `Paste` or
`PasteBracketed` on it leaves the pane command as `bash`. That is what makes the
post-paste gate (FR-005) provable in the default build: without it the only way to
observe a harness exiting *between* two tmux commands is a real tmux. `exec.go`
and `controller.go` are not edited, so the interface and the k8s module's
`podctl` are untouched.

### Requirement → FR → task → test

| Req | FRs | Task | The test that fails first |
|---|---|---|---|
| 1 liveness | FR-003 … FR-006 | T007, T008, T009 | stopped refuses with no paste; `ExitAfterPaste` proves `C-u` and no Enter |
| 2 not-ready answer | FR-007, FR-008 | T005 | a `failed` session answers `409`, not `500` |
| 3 bracketed paste | FR-009 … FR-011 | T006 | a two-line prompt records one bracketed paste and one Enter |
| 4 first character | FR-012, FR-013 | T004 | `/status` answers `400` with no tmux command; the type route still sends `/compact` |
| 5 one at a time | FR-014 … FR-016 | T010 | a held claim refuses; two goroutines never interleave paste and Enter (`-race`) |
| 6 `request_id` | FR-017, FR-018 | T001, T002, T003 | the value is echoed and on the record; absent appears nowhere |
| — amendments | FR-001, FR-002 | P2, P5 | — |
| — contract | FR-020 | P5 | six headings still counted |
| — docs | — | T013 | `docs/security.md` names the six gates |
| — live callers | SC-003 | T011, T012 | the four recorded payloads each answer `202` |

## Project Structure

### Documentation (this feature)

```text
specs/020-prompt-route-safety/
├── research.md          written (P1) — D1 to D9, each with its evidence
├── spec.md              written (P2) — FR-001 to FR-020, SC-001 to SC-007
├── plan.md              this file (P3)
└── tasks.md             P4 — T001 to T013
```

No `data-model.md`: this feature adds no entity and no stored state. No
`quickstart.md`: the route is shipped and `http-api.md` is its quickstart. No
`contracts/` of its own — the amendment lands in spec 001's contract, which is
where a client already reads this route (P5, FR-020).

### Source code (repository root)

This is the implementation plan's file list, and it is **the allowlist in
`ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` "Files touched"**, in that
file's order. The loop may create or edit these and no others, plus its own
notebook directory:

```text
specs/020-prompt-route-safety/research.md                MOD  (written, P1)
specs/020-prompt-route-safety/spec.md                    MOD  (written, P2)
specs/020-prompt-route-safety/plan.md                    NEW  P3 — this file
specs/020-prompt-route-safety/tasks.md                   NEW  P4
specs/001-crswd-daemon-core/contracts/http-api.md        MOD  P5 — request_id, 409, 400
internal/audit/audit.go                                  MOD  T001 — Record.RequestID
internal/audit/audit_test.go                             MOD  T001 — present and absent
internal/audit/leak_test.go                              MOD  T001 — the field as a canary
internal/httpapi/middleware.go                           MOD  T002 — SetRequestID
internal/httpapi/middleware_test.go                      MOD  T002
internal/httpapi/sessions.go                             MOD  T003, T004, T005 — fields, two gates, 409
internal/httpapi/sessions_test.go                        MOD  T003, T004, T005
internal/httpapi/lawnmower_payload_test.go               NEW  T011 — the four payloads
internal/httpapi/testdata/lawnmower/answer.txt           NEW  T011
internal/httpapi/testdata/lawnmower/resume.txt           NEW  T011
internal/httpapi/testdata/lawnmower/opp-lease-batch.txt  NEW  T011
internal/httpapi/testdata/lawnmower/opp-followup.txt     NEW  T011
internal/session/manager.go                              MOD  T005 … T010 — sentinel, gates, claim, paste
internal/session/manager_test.go                         MOD  T005 … T010
internal/session/prompt_tmux_test.go                     NEW  T012 — //go:build tmux
internal/tmuxctl/fake.go                                 MOD  T008 — ExitAfterPaste
internal/tmuxctl/fake_test.go                            MOD  T008
docs/security.md                                         MOD  T013 — "Prompt route gates (spec 020)"
```

**Structure decision**: no new directory anywhere. Every change lands in a package
that already owns the thing being changed — the record in `audit`, the request
shape and the status mapping in `httpapi`, the delivery in `session`, the test
double in `tmuxctl`.

Out of scope, ever, in this milestone: `AGENTS.md`, `k8s/`,
`internal/tmuxctl/controller.go`, `internal/tmuxctl/exec.go`, `go.mod`, and any
new directory under `internal/`.

## Cross-spec notes

- **Spec 018 (interactive input)**: FR-006 of that spec — "the API door's prompt
  route MUST be unchanged" — is superseded by spec.md FR-001, in one named place.
  Its own routes, including `POST /dashboard/sessions/{id}/type`, are untouched
  (FR-013), and `ValidateTyped` keeps the C0 refusal on the browser door's text.
- **Spec 001 (daemon core)**: FR-042 is amended for the single field `request_id`
  (spec.md FR-002). FR-030's byte-for-byte delivery is unchanged — FR-012 refuses
  a prompt, it never edits one. P5 is the contract edit, and the six-heading count
  is what keeps it from becoming a seventh route.
- **Spec 017 (kubernetes)**: nothing here touches it. The gate reads `List`
  precisely so that `k8s/internal/podctl` does not have to grow a method for a
  route it does not serve (research.md D2). Probe check 8's open question about the
  session registry under that execution model is why requirement 7 is declined
  rather than built (D8).
- **Spec 012 (liveness)**: FR-006's reading of *unknown* as alive is relied on, not
  changed. FR-007's "never read pane content for liveness" holds; the gate reads
  the option-versus-pane-command match the controller already computes.
- **Spec 013 (conversations)**: FR-025 — no transcript is opened — is unchanged.

## Risks

| Risk | Where it is handled |
|---|---|
| A harness exits between the second liveness read and the Enter | Not closed, and said so in spec.md "Edge Cases": the residual window is two tmux round trips wide rather than a whole request wide. Probe check 1 found no such race worth defending after a session's first turn. |
| A prompt carrying `ESC[201~` ends the bracketed paste early, turning the remainder into keystrokes and bypassing FR-012 | Named in spec.md FR-019 as not in this feature, with the evidence that no live caller or fixture sends it. A byte-level refusal on a route with shipped callers is its own decision; P4 must not invent a task for it. |
| A harness release stops enabling bracketed paste | Degrades to today's behaviour — line one submits — which an operator sees immediately. Measured for Claude Code 2.1.290 and Codex 0.153.4 on tmux 3.4. |
| A pane running a harness under an unexpected process name reads as *stopped* and is refused | The direction a wrong answer should fail in. Probe check 13 matched 6 of 6 live panes, `rcq` included; `@crswd-binary` already accepts a set of names (spec 019). |
| `golangci-lint` cannot run in the loop's container (exit 127) | A known limit, recorded in the notebook's Findings. Markdown-only tasks cannot regress it; a task that changes Go records its lint result as unverified in those words, and CI runs the pinned version regardless. |
| The two external source documents are unreadable from the loop | Everything they decide is already copied into research.md, spec.md and the notebook's "Facts measured while planning". No task may depend on opening them. |

## Complexity Tracking

No Constitution Check violation, so nothing to justify here. The one place this
feature could have added complexity — a per-session liveness method on
`tmuxctl.Controller` — was rejected for the simpler read the supervisor already
makes (research.md D2), and the registry gate that would have added a second
readiness signal was declined on evidence (D8).
