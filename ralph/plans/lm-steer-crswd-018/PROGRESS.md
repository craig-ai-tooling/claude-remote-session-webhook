# Progress: prompt route safety (lm-steer-crswd-018)

The loop's memory between iterations. Append, never rewrite. Newest entry last.

## Facts measured while planning (10/14/26, do not re-measure)

- Branch `plan/lm-steer-crswd-018`, base `main` at 1eeb59c. The running daemon was at
  vcs.revision 1eeb59c when this was planned.
- `specs/018-interactive-input` and `specs/019-codex-runtime` exist, so this spec is
  `specs/020-prompt-route-safety`.
- The prompt route today: `internal/httpapi/sessions.go` `promptSession` (decode, pane dialog
  check, `Manager.Prompt`, 202), `refusePrompt` maps `ErrEmptyPrompt` to 400 and the rest to 500.
- `Manager.Prompt` is in `internal/session/manager.go`. It refuses an empty ID, `StateDead` and
  empty text, takes `lockSession`, calls `m.paste` (bracketed only when the harness spec says
  `BracketedPaste`, which Claude's does not), then `SendKeys` Enter.
- `lockSession` (in `internal/session/input.go`) is already the per-session mutex shared by
  Type, PressKey, Prompt, Compact, SetMode, Continue, Destroy and the supervisor's Codex discovery.
- `Exec.paste` (`internal/tmuxctl/exec.go`) already makes a fresh buffer name per paste with
  `newBufferName`. The fake records buffer names via `nextBuffer`.
- `claimRestart` / `releaseRestart` are in `internal/session/manager.go` and are non-blocking.
- The fake controller has `SetPaneCommand(name, command)`; its `List` derives `Claude` liveness
  from `@crswd-binary` against the pane command (`livenessOf`). There is no per-session liveness
  method on `Controller`, and adding one would force a change in `k8s/internal/podctl`. Use `List`.
- `audit.Record` (`internal/audit/audit.go`) has Action, Caller, SessionID, Decision, Reason,
  Remote. `RequestAudit` (`internal/httpapi/middleware.go`) has `SetSessionID` and `Deny`.
- The API door has six routes (`internal/httpapi/server.go` route switch; contract headings in
  `specs/001-crswd-daemon-core/contracts/http-api.md` lines 128 to 304).
- `internal/session/mode_test.go` is the existing `//go:build tmux` test in that package. Copy
  its socket setup for T012.
- AGENTS.md is 147 lines; CI fails at 150 (`.github/workflows/ci.yml`, "AGENTS.md stays skimmable").
- Tools on this host: tmux 3.4 at `/usr/bin/tmux`, golangci-lint 2.12.2. The root module has no
  `go.sum` and must not gain one.
- Probe results that decide this plan (`~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`):
  check 13 (`@crswd-binary` matches `claude` on 6 of 6 live panes, rcq included), check 8 (registry
  under v2 option B unplaced, so req 7 is deferred), checks 3 and 9 (registry misses the trust
  dialog and a draft), check 1 (no check-then-Enter race after the first turn; a lost Enter only
  in the first ~5 s after launch, out of scope here). The probe's transport verdict, "flip to
  socket", binds lm-steer-write, not this milestone.
- lawnmower's `deliver()` (`~/code/ai-lawnmower/loop/session_resume.py`) already treats any body
  with an `error` key as refused, so a 409 `{"error":"session not ready"}` reads as a refusal.

## Payload fixtures (for T011)

Write each block below, byte for byte without the fence, to the named file under
`internal/httpapi/testdata/lawnmower/`. They follow the builders in lawnmower as of 10/14/26:
`console.py` `_answer_action`, `lawnmower-route` `resume_prompt`, `_batch_listing_prompt`,
`opp_followup_prompt`, each ending in an abridged `closing_for` where the builder adds it. The shapes that matter
are kept: multi-line, a first character that is a letter, double quotes, `$`, `;`, `&`, a
backtick, and a later line that starts with `/`.

`answer.txt`:

```
Craig answered your question about inbox item 01M3TESTANSWER0000000000AA from the board: B -- keep the old flag.

You are the session that asked this, and you still have the whole context -- pick up from where you stopped rather than starting over.

WHAT HE WAS SHOWN (verbatim; his answer refers to this):
A: drop --legacy and migrate; B: keep the old flag, add "--new" beside it ($0 cost)

Carry out what he chose. Do not ask him this again -- he has answered it. When you are finished, report as you were told to:
  lm-report 01M3TESTANSWER0000000000AA done --note "what you did, in a sentence or two"
```

`resume.txt`:

```
Craig has put inbox item 01M3TESTRESUME0000000000AA back in the queue, and you are the session that already owns it -- you still have the whole context, so pick up from where you stopped rather than starting over.

The item, unchanged:
fix the flaky test in loop/x.sh; see https://example.com/a?b=1&c=2
/compact only if the context gets long

Re-check anything you are about to rely on -- time has passed and he may have changed something himself.

---
Report to Lawnmower, not this terminal -- nobody reads it.
  lm-report 01M3TESTRESUME0000000000AA done      --note "what exists now, and exactly where: URL, path, PR #"
```

`opp-lease-batch.txt`:

```
2 more inbox item(s) have arrived for acme-poc, and you are the session already working this opp -- handle them in this same session rather than a new one starting up next to you.

---
You have 2 items for the SAME opp (acme-poc). Work them ONE AT A TIME, in order, and finish each before starting the next.

1. [01M3TESTBATCH00000000000001] send the "edge sizing" sheet to Dana
2. [01M3TESTBATCH00000000000002] book 30m with their SRE lead; prefer Tue

---
As you finish each item, mark it:
  dispatch status <the id in brackets> done
and if one needs Craig, mark that one needs-input and keep going with the
rest. Do not mark anything done before its work has actually landed.
When every item is accounted for, say so and stop.
```

`opp-followup.txt`:

```
Another inbox item has arrived for acme-poc, and you are the session already working this opp -- handle it in this same session rather than a new one starting up next to you.

The item, 01M3TESTFOLLOW0000000000AA:
they asked for the air-gap bundle size; check `du -sh` on the last build

---
Report to Lawnmower, not this terminal -- nobody reads it.
  lm-report 01M3TESTFOLLOW0000000000AA done      --note "what exists now, and exactly where: URL, path, PR #"
```

## Failing-before evidence (F1 counts these rows)

One row per requirement, added by the T-task that proves it: req 1 by T009 (covering T007's
first check too), req 2 by T005, req 3 by T006, req 4 by T004, req 5 by T010, req 6 by T003. Format:
`| req <n> | <test name> | <result against old code> | <result after change> |`

| Req | Test | Before | After |
|---|---|---|---|

## Iterations

### P1 — research.md (10/14/26)

**Changed.** Created `specs/020-prompt-route-safety/research.md`, the only file this
iteration touched besides this notebook and the plan's checkbox. One `## D<n>` section
per decision D1 to D9 from IMPLEMENTATION_PLAN.md, each carrying the evidence it cites,
plus three sections the verify command does not ask for and the next tasks need: "Where
the evidence comes from" (which of Probe / Design / Code each claim leans on), "The
route as it stands" (the measured baseline the hardening is written against), and "Not
verified".

**Commands run.**

| Command | Result |
|---|---|
| `grep -c '^## D[0-9]' specs/020-prompt-route-safety/research.md` | `9` — the task's verify (`-ge 9`) exits 0 |
| `grep -rn 'NEEDS CLARIFICATION' specs/020-prompt-route-safety/` | no output, so G1's verify and the validation contract's "no open question" both pass |
| `gofmt -l .` | prints nothing |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` | **could not run — see finding F2** |
| `test ! -e go.sum` | exit 0 |
| `git status --short` | `?? specs/020-prompt-route-safety/` only |

**What the next iteration would otherwise rediscover.**

- **Do not write the words N-E-E-D-S C-L-A-R-I-F-I-C-A-T-I-O-N into any file under
  `specs/020-prompt-route-safety/`, not even to say the spec has none.** G1's verify and
  the validation contract both `grep -r` that directory for the phrase, so a sentence
  denying it fails the gate as loudly as a real one. D8 in research.md had to be
  reworded for exactly this. If a genuine open question ever appears, it belongs in
  *this* notebook under NEEDS CLARIFICATION and in a `- [!]` box, which is what the
  PROMPT already says.
- **Line references verified in tree at `30941f3`**, so P2 to P5 and T001+ need not
  re-grep for them: `promptSession` `internal/httpapi/sessions.go:772`, `refusePrompt`
  `:849` (`ErrEmptyPrompt`→400, everything else→500, no 409 today), `promptRequest`
  `:264-270` (one field, emptiness and body size are "the whole of its validation"),
  `Manager.Prompt` `internal/session/manager.go:1194-1224`, `m.paste` `:1253`,
  `claimRestart`/`releaseRestart` `:2674-2688`, `lockSession`
  `internal/session/input.go:171`, `ValidateTyped` `:75-92`, `Manager.Type` `:101`
  (calls `ValidateTyped` at `:105`, `PasteBracketed` at `:117`), `newBufferName`
  `internal/tmuxctl/exec.go:186-195`, `Exec` liveness `:548`, `Liveness` constants
  `internal/tmuxctl/controller.go:312-341`, `PasteBracketed` on the interface `:51`,
  fake `livenessOf` `internal/tmuxctl/fake.go:251` and `nextBuffer` `:280`,
  `audit.Record` `internal/audit/audit.go:390-414`, `StateFailed`
  `internal/session/session.go:86` set at `internal/session/supervisor.go:311-314`,
  `LivenessUnknown` treated as healthy at `internal/session/supervisor.go:153-156`.
- **The contract's six route headings are at `specs/001-crswd-daemon-core/contracts/http-api.md`
  lines 128, 183, 223, 231, 256 (the prompt route) and 304.** P5 edits the 256 block and
  must not add a seventh `## ` heading of that shape.
- **`~/code/ai-lawnmower/` is not readable from the loop's sandbox** — both the design
  doc and the probe results are outside `/workspace` and the read is refused. Every
  probe fact this milestone needs is already transcribed into this notebook, which is
  what research.md cites. Do not plan a task that has to open either file.
- **Requirement 5 is half built already.** `lockSession` is taken by `Prompt` today and
  `Exec.paste` already draws a fresh buffer name per call, so T010 adds the restart
  claim and *proves* the other two rather than building them.

### P2 — spec.md (10/14/26)

**Changed.** Created `specs/020-prompt-route-safety/spec.md`, the only file this
iteration touched besides this notebook and the plan's checkbox. Six user stories
(P1 for the three safety ones, P2 for the leading-character rule and the restart
claim, P3 for `request_id`), an Edge Cases list, FR-001 to FR-021, Key Entities,
SC-001 to SC-008 and Assumptions. Requirements 1 to 6 are FR-001 to FR-014;
requirement 7 is FR-020 under "Out of scope"; the two amendments are FR-018
(spec 018 FR-006) and FR-019 (spec 001 FR-042).

**F1 is decided — the API door's prompt gains the control-character refusal.**
This was the one thing P1 left for P2, and spec.md settles it as FR-007 rather
than accepting the exposure. What was decided, and the two bounds on it that are
*not* obvious and that T006 must honour:

- **The rule is the character rule only**: not valid UTF-8, or any byte in
  `0x00–0x1F` other than `0x09`/`0x0A`, or `0x7F` → `400`, uniform bad-request
  body, before any tmux command. It must be **the same predicate** that
  `session.ValidateTyped` applies, not a second copy of it.
- **The length bound is deliberately NOT adopted.** `ValidateTyped` also enforces
  `MaxTypeBytes` = 16384 (`internal/session/input.go:59`), while this route's
  bound today is `CRSW_MAX_BODY_BYTES`, default **65536**
  (`internal/config/config.go:302`). Calling `ValidateTyped` wholesale from
  `Manager.Prompt` would therefore *tighten a live route's limit by 4x* for no
  requirement — a client break. So T006 needs the character half factored out, or
  an equivalent that leaves the length to the body bound. Do not just add
  `ValidateTyped(text)` to `Prompt` and call it done.
- **No CRLF normalisation on this door.** Spec 018's route rewrites `\r\n` before
  validating because a textarea produces it; the API door delivers caller bytes
  byte-for-byte (`internal/httpapi/sessions.go:264-270`), so a bare `\r` is
  refused, not rewritten. No measured caller sends one.
- Why refuse rather than accept the exposure: `docs/security.md` §2 is binding and
  constitution I makes it a gate, and FR-006's bracketing is what makes
  `\x1b[201~` live on this route. Closing it in the same change that opens it is
  cheaper than documenting it. Reachability was checked: Go's JSON decoder refuses
  a *raw* control byte inside a string, but a body may spell ESC as its
  six-character JSON escape (backslash, `u`, `001b`), which decodes to a real ESC
  byte in the text. So the hole is reachable by a well-formed body today.
  (Spelled out in words because the Write tool turns that escape into a literal
  ESC byte in the file — the milestone-1-3 archive note is about the same trap.)

**Two more facts measured this iteration, both load-bearing for the T-tasks.**

- **`Manager.Prompt` has exactly one caller**, `internal/httpapi/sessions.go:796`
  (`grep -rn '\.Prompt(' --include=*.go | grep -v _test.go` returns that one
  line). So a rule placed in `Manager.Prompt` is API-door-only *by construction*,
  which is how D5's and FR-007's "API door only" are satisfied without a flag or a
  second entry point. The browser type route goes through `Manager.Type`, and
  `Compact` through `Manager.Compact`.
- `refusePrompt` (`sessions.go:849`) today has two branches only —
  `ErrEmptyPrompt` → 400, everything else → `failInternal`. T005 adds the 409 and
  T004/T006 need the 400 branch to cover `ErrInputInvalid` as well, or a
  control-character refusal returned from the manager becomes a 500.

**Commands run.**

| Command | Result |
|---|---|
| `grep -q 'session not ready' … && grep -q request_id … && ! grep -q 'NEEDS CLARIFICATION' …` | exit 0 — the task's verify |
| `! grep -rq 'NEEDS CLARIFICATION' specs/020-prompt-route-safety` | exit 0, so G1's gate and the validation contract still pass with spec.md present |
| `gofmt -l .` | prints nothing |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` | **could not run — finding F2, unchanged; `which golangci-lint` still finds nothing** |
| `test ! -e go.sum` | exit 0 |
| `git status --short` | `?? specs/020-prompt-route-safety/spec.md` only |

**What the next iteration would otherwise rediscover.**

- **P3 to P5 inherit F2's reasoning** (Markdown only, lint byte-identical to the
  baseline). T001 onward change Go and the gate is real for them.
- **P3's plan.md gets its Constitution Check from the gates already named here**:
  principle I is FR-005/FR-007/FR-008 (refuse at the boundary, uniform answers),
  VI is FR-001/FR-003/FR-004 (bounded blast radius — a prompt never reaches a
  shell), IV is why `Compact`, the browser route and the length bound are all left
  alone, and III is SC-001 to SC-008. No principle needs a justification for
  widening, because nothing here widens: every requirement is a refusal except
  `request_id`.
- **P4's tasks.md must put FR-007 inside T006** (F1 named T006 as where it lands),
  and T006's entry has to say the length bound is not adopted — otherwise the
  obvious implementation is the wrong one, per the bullet above.
- **P5 edits the contract's prompt block at line 256** and must not add a seventh
  `## ` route heading. FR-012's field, FR-005's 409 and FR-008's 400 all land
  there.
- **The FR numbers are now fixed**, so tasks.md and plan.md can cite them: reqs 1
  to 6 → FR-001 to FR-014, req 7 → FR-020, amendments → FR-018 (018 FR-006) and
  FR-019 (001 FR-042), the docs change → FR-017.

## Findings

### F1 — bracketing the API door's paste meets text nothing validates (opened by P1, decided by P2)

**Resolved by P2 as FR-007 in spec.md**: the API door's prompt gains the
control-character and UTF-8 refusal, as the same predicate `ValidateTyped`
applies, and **not** its 16384-byte length bound. See the P2 iteration entry above
for the two bounds T006 must honour. The original statement of the finding
follows, unchanged.


**What.** D4 makes `Manager.Prompt` use `PasteBracketed` for every harness. The API
door's prompt text is validated for emptiness and the body-size bound and nothing else
(`internal/httpapi/sessions.go:264-270`: "that is the whole of its validation
(FR-030)"). The C0-control refusal that `docs/security.md` §2 describes —
"inside a bracketed paste `ESC[201~` would end the paste" — is `session.ValidateTyped`
(`internal/session/input.go:75-92`), and it is called by `Manager.Type` (`:105`) and
**not** by `Manager.Prompt`.

**So.** Under today's plain paste those bytes are inert. Under D4 a caller that sends
`\x1b[201~` mid-text ends the paste early, and the remainder reaches the pane as
keystrokes rather than as pasted text — on a route whose session runs unsandboxed.

**Not resolved here, and deliberately not invented away.** D4 as decided adds no
refusal and P1 does not get to add one. **P2 must decide it in spec.md**: either the
API door's prompt text goes through `ValidateTyped` (the mechanism exists, one call
away, and T006 is where it would land) or the exposure is accepted in writing with the
reason. If P2 cannot decide it from the plan, that is a real open question and P2 is
the task that should stop on it.

**What is known.** No measured client sends a control byte: every block under "Payload
fixtures" above is printable text plus newlines, so either answer keeps lawnmower
working. research.md D4 carries this under "A consequence this decision does not close".

### F2 — `golangci-lint` is absent from this host, so the lint gate could not run

**What.** The PROMPT's finish step 9 and the plan's pre-commit step 6 both require
`GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` to exit 0. The binary is not
installed here (`which golangci-lint` finds nothing; only `/usr/local/go/bin/go`,
`gofmt` and `/usr/bin/tmux` are present), and the session-start hook says so too. The
install the hook suggests, `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`,
needs the network and is refused in this sandbox. Note this **contradicts** "Tools on
this host: … golangci-lint 2.12.2" under "Facts measured while planning" — that fact
was measured on the planning host, not on the loop's.

**Why P1 was committed anyway rather than marked `- [!]`.** P1 changed Markdown only
(the PROMPT: "Spec tasks (P1 to P5) change Markdown only, no Go"), so there is no Go
for the linter to have an opinion about and the lint result is byte-identical to the
baseline at `30941f3`, which CI already ran. Marking `- [!]` stops every later
iteration as well, which would halt the milestone over a dev tool unrelated to the
change. The gate is recorded as **not run**, not as passed.

**What the next iterations must do.** P2 to P5 are Markdown-only and inherit the same
reasoning. **T001 onward change Go, and for those the gate is real**: an iteration that
cannot run the linter on a Go change should not claim the pre-commit list passed. If it
is still missing then, that iteration should install it if the sandbox allows, and
otherwise mark its task `- [!]` and say so here. CI runs `golangci-lint run`
regardless, so nothing merges unlinted.
