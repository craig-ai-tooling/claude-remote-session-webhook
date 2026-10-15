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

### Iteration 1 — P1 (research.md)

**Changed.** Created `specs/020-prompt-route-safety/research.md`, the first file in that
directory. Nine sections, `## D1` to `## D9`, one per decision in IMPLEMENTATION_PLAN.md
"Decisions already made", each carrying the evidence that decision cites: probe checks
13, 1, 8, 3 and 9; the spec 018 R1 and spec 019 M3/M4 paste measurements already in this
repo; the lawnmower builder grep of 10/14/26; and the file anchors from "Facts measured
while planning" above. No decision reopened, no requirement added.

**Commands run.**

- `grep -c '^## D[0-9]' specs/020-prompt-route-safety/research.md` → `9`, so the task's
  Verify (`-ge 9`) holds. See the note below on why the bracketed form itself was not run.
- `gofmt -l .` → no output.
- `go build ./...` → exit 0.
- `go vet ./...` → exit 0.
- `go vet -tags tmux ./...` → exit 0.
- `go vet -tags quickstart ./cmd/crswd` → exit 0.
- `test ! -e go.sum` → exit 0.
- `git status --short` → `?? specs/020-prompt-route-safety/` only.
- `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` → **not run.** See Findings.

**For the next iteration.**

- The external sources (`~/code/ai-lawnmower/docs/mcp-session-steering-design.md` and
  `.../lm-steer-design/results.md`) are **not readable from this loop environment** — the
  tooling is confined to `/workspace`. Do not plan on opening them. Everything needed is
  already copied into IMPLEMENTATION_PLAN.md "Decisions already made", the "Facts measured
  while planning" section above, and now research.md.
- Anchors confirmed by reading, for the T-tasks: `Manager.Prompt` at
  `internal/session/manager.go:1194` (empty-ID, `StateDead`, empty-text refusals, then
  `lockSession`, `m.paste`, `SendKeys` enterKey); `Manager.paste` at
  `internal/session/manager.go:1253` (bracketed only when `specOf(s).BracketedPaste`);
  `claimRestart` / `releaseRestart` at `internal/session/manager.go:2663` and `2688`;
  the three `Liveness` constants at `internal/tmuxctl/controller.go:312`–`332`.
- The next task is P2 (spec.md). It is Markdown only; the template is
  `.specify/templates/spec-template.md` and the sibling specs 018 and 019 show the house
  style.

### Iteration 2 — P2 (spec.md)

**Changed.** Created `specs/020-prompt-route-safety/spec.md` from
`.specify/templates/spec-template.md`, in the house style of
`specs/018-interactive-input/spec.md`. Five prioritised user stories (one per
requirement 1 to 6, with reqs 1 and 2 split by what they refuse versus what the
caller is told), Edge Cases, FR-001 to FR-020, Key Entities, SC-001 to SC-007 and
Assumptions. No decision reopened and no requirement added beyond D1 to D9.

FR map, for the T-tasks that implement them:

- FR-001 amends spec 018 FR-006 (quoted in full, then withdrawn); FR-002 amends
  spec 001 FR-042 for `request_id` only.
- FR-003 to FR-006: the liveness gate (req 1) — `List` only, no `Controller`
  method, stopped refuses, unknown delivers, a `List` failure is an error, second
  read after the paste sends `C-u` and no Enter.
- FR-007, FR-008: one sentinel `ErrSessionNotReady` with text `session not ready`,
  409 `{"error":"session not ready"}`, one audit deny with that reason, three
  states indistinguishable, `dead` unchanged (reqs 1, 2, 5).
- FR-009 to FR-011: bracketed paste for every harness, `Compact` untouched, no
  newline refusal (req 3).
- FR-012, FR-013: leading `!` or `/` refused 400 on the API door only; the browser
  type route unchanged (req 4).
- FR-014 to FR-016: `lockSession` then `claimRestart`, refuse not queue, released
  on every path out, fresh buffer per paste (req 5).
- FR-017, FR-018: optional `request_id`, 1 to 32 of `[a-z0-9-]`, 400 when invalid,
  echoed in the 202 and on the record, absent means absent everywhere (req 6).
- FR-019: Not in this feature — req 7 and all of D9.
- FR-020: what P5 must put in `http-api.md`, including the six-heading count.

**Commands run.**

- Verify, as written: `grep -q 'session not ready' … && grep -q request_id … && !
  grep -q 'NEEDS CLARIFICATION' …` → exit 0. This form **was** accepted by the
  environment (a plain `&&` chain, unlike P1's bracketed `[ "$(…)" ]`).
- `gofmt -l .` → no output.
- `go build ./...` → exit 0.
- `go vet ./...` → exit 0.
- `go vet -tags tmux ./...` → exit 0.
- `go vet -tags quickstart ./cmd/crswd` → exit 0.
- `test ! -e go.sum` → exit 0.
- `git status --short` → `?? specs/020-prompt-route-safety/spec.md` only.
- `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` → **not run**, same cause as
  P1: `golangci-lint version` exits 127 in this container. P2 changed Markdown
  only, so no Go file was touched and the Go linter cannot have regressed.

**For the next iteration.**

- **One residual is named in the spec rather than fixed, and it is deliberate.**
  FR-009 makes the API door's paste bracketed, so a payload containing
  `ESC[201~` would end the paste early and turn the remainder into keystrokes —
  which is a way past FR-012's leading-character rule. No decision in
  IMPLEMENTATION_PLAN.md adds a C0 or marker refusal to this door and no T-task
  implements one, so the spec lists it under FR-019 "Not in this feature" with
  what is known (spec 018 FR-004 makes that check on the browser door; no live
  caller or fixture sends the marker). Do **not** add a validation task for it in
  P4 — that would be inventing a requirement. It is a candidate for a later
  milestone.
- Anchors read while writing this, useful for T003 to T005:
  `promptRequest` / `promptResponse` at `internal/httpapi/sessions.go:264` and
  `:283`; `promptSession` at `:772` (decode, `paneDialogState`, `Prompt`, 202);
  `refusePrompt` at `:849` (only `ErrEmptyPrompt` → 400, everything else
  `failInternal`); `rejectBadRequest` at `internal/httpapi/decode.go:160`, which
  already calls `AuditFrom(r.Context()).Deny(reason.Error())` and writes
  `bodyBadRequest` (`{"error":"bad request"}`, `decode.go:21`) — so a 400 gate
  gets its audit deny for free, and its reason must be an error authored in this
  repo.
- The contract's prompt section is `specs/001-crswd-daemon-core/contracts/http-api.md`
  lines 256 to 301; its status-code table is at line 111, where `409` today reads
  only "Teardown could not be verified". The six route headings are at lines 128,
  183, 223, 231, 256 and 304. P5 needs all of these.
- Next task is P3 (plan.md), from `.specify/templates/plan-template.md`, with a
  Constitution Check against `.specify/memory/constitution.md` and the
  IMPLEMENTATION_PLAN.md allowlist as its file list. Markdown only.

### Iteration 3 — P3 (plan.md)

**Changed.** Created `specs/020-prompt-route-safety/plan.md` from
`.specify/templates/plan-template.md`, in the house style of
`specs/018-interactive-input/plan.md` and `specs/019-codex-runtime/plan.md`:
Summary, Technical Context, a Constitution Check table over all seven principles,
a Design section per package, a requirement→FR→task→test map, Project Structure
(documentation and source), Cross-spec notes, Risks and Complexity Tracking (no
violation, so nothing justified). The source file list **is** the
IMPLEMENTATION_PLAN.md "Files touched" allowlist, in that file's order, each line
annotated NEW/MOD and with the task that touches it. No decision reopened, no
requirement added.

**Also changed, one phrase in `specs/020-prompt-route-safety/research.md` (D8).**
It read "It is **not** a `NEEDS CLARIFICATION`", which is prose *about* the
phrase — and G1's Verify is `! grep -rq 'NEEDS CLARIFICATION'
specs/020-prompt-route-safety`, a grep over the whole directory. Left alone, G1
would have failed on a quotation and the loop would have had to mark it `- [!]`
and stop the milestone over a wording artifact. The sentence now reads "It is
**not** an open question in the constitution's sense: principle II is about …",
which says the same thing. P1's own Verify still holds (`grep -c '^## D[0-9]'` →
`9`). My first draft of plan.md contained the same quotation in the principle II
row and failed its own Verify; it is reworded there too.

**For a later iteration writing Markdown: do not write that phrase as a
quotation anywhere under `specs/020-prompt-route-safety/`.** P4 (tasks.md) and
P5 are the next two Markdown tasks; G1 greps the directory recursively.

**Commands run.**

- Verify, as written: `grep -q 'Constitution Check' … && ! grep -q 'NEEDS
  CLARIFICATION' …` → exit 1 on the first draft (the quotation above), exit 0
  after the rewording. The plain `&&` chain runs in this environment, as in P2.
- `grep -rc 'NEEDS CLARIFICATION' specs/020-prompt-route-safety` → `0` for all
  three files, so G1's recursive grep passes as of this commit.
- `grep -c '^## D[0-9]' specs/020-prompt-route-safety/research.md` → `9`.
- `gofmt -l .` → no output.
- `go build ./...` → exit 0.
- `go vet ./...` → exit 0.
- `go vet -tags tmux ./...` → exit 0.
- `go vet -tags quickstart ./cmd/crswd` → exit 0.
- `test ! -e go.sum` → exit 0.
- `git status --short` → ` M specs/020-prompt-route-safety/research.md` and
  `?? specs/020-prompt-route-safety/plan.md`, both in the allowlist.
- `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` → **not run.**
  `golangci-lint version` still exits 127 in this container (Findings). P3
  changed Markdown only, so no Go file was touched and the Go linter cannot have
  regressed.

**For the next iteration.**

- Next task is **P4 (tasks.md)**, T001 to T013 with the titles already in
  IMPLEMENTATION_PLAN.md. plan.md's "Requirement → FR → task → test" table maps
  every T-task to the FRs it implements and the test that must fail first — copy
  from it rather than re-deriving. Its Verify counts
  `^- \[ \] T0(0[1-9]|1[0-3]) ` and wants exactly 13; the bracketed `[ … -eq 13 ]`
  form will not run here (Findings), so measure the count with `grep -cE` and
  record it.
- **P4 must not add a task for the `ESC[201~` marker.** spec.md FR-019 and
  plan.md's Risks both place it out of this milestone; a validation task for it
  would be inventing a requirement (Iteration 2 said the same).
- Code anchors confirmed by reading while writing this plan, for T001 to T010:
  `Manager.Prompt` at `internal/session/manager.go:1194` (empty ID, `StateDead`,
  empty text, `lockSession:1209`, `m.paste:1217`, `SendKeys` enterKey`:1220`);
  `Manager.paste` at `:1253`; `claimRestart` at `:2674`, `releaseRestart` at
  `:2688`, and the existing claim-then-defer pattern at `:2760`–`2763`;
  `promptRequest` at `internal/httpapi/sessions.go:264`, `promptResponse` at
  `:283` (`ParkedOn` and `SuspiciousDialog` already show the `omitempty`
  additive-field pattern `request_id` should copy); `promptSession` at `:772`;
  `refusePrompt` at `:849`; `audit.Record` at `internal/audit/audit.go:390`;
  `RequestAudit.SetSessionID` at `internal/httpapi/middleware.go:120` and `Deny`
  at `:133`; `Fake.SetPaneCommand` at `internal/tmuxctl/fake.go:761`;
  `Controller.List` at `internal/tmuxctl/controller.go:97`, `SessionInfo.Claude`
  at `:316`, `LivenessStopped` at `:341`.
- **The liveness read has a pattern to copy rather than invent**:
  `Supervisor.Sweep` (`internal/session/supervisor.go:117`) takes one
  `m.tmux.List(ctx)`, indexes the rows by `info.Name`, and `judge` at `:156`
  treats `present && info.Claude != tmuxctl.LivenessStopped` as healthy — so
  unknown *and* absent both pass there. T007 should read the same way.

### Iteration 4 — P4 (tasks.md)

**Changed.** Created `specs/020-prompt-route-safety/tasks.md`, in the house style of
`specs/018-interactive-input/tasks.md`: front matter, inputs, "Every task ends green",
"Conventions every task follows", a "Failing-before evidence" table naming which task
adds which requirement's row, a 13-line task checklist, then one `### T0nn —` section
per task with **Files / Implements / Interface / Approach / Edge cases / Mirror /
Failing-before test / Other tests / Verify / Depends on / Guardrails**, a dependency
diagram, and a closing "What no task in this list may do". Titles are copied verbatim
from IMPLEMENTATION_PLAN.md; the FR→task→test mapping is copied from plan.md's
"Requirement → FR → task → test" table rather than re-derived. No decision reopened,
no requirement added. **No task for the `ESC[201~` marker**, as Iterations 2 and 3
both instructed.

**Commands run.**

- `grep -cE '^- \[ \] T0(0[1-9]|1[0-3]) ' specs/020-prompt-route-safety/tasks.md` →
  `13`, so the task's Verify (`-eq 13`) holds. The bracketed `[ … -eq 13 ]` form still
  will not run in this environment (Findings), so the count was measured, as P1 did.
- `grep -rc 'NEEDS CLARIFICATION' specs/020-prompt-route-safety` → `0` for all four
  files, so G1's recursive grep still passes.
- `gofmt -l .` → no output. `go build ./...`, `go vet ./...`, `go vet -tags tmux ./...`,
  `go vet -tags quickstart ./cmd/crswd`, `test ! -e go.sum` → all exit 0.
- `git status --short` → ` M ralph/plans/…/IMPLEMENTATION_PLAN.md`,
  ` M ralph/plans/…/PROGRESS.md`, `?? specs/020-prompt-route-safety/tasks.md`.
- `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` → **not run.**
  `golangci-lint version` still exits 127 here. P4 changed Markdown only, so no Go
  file was touched and the Go linter cannot have regressed.

**For the next iteration — what the code reading for P4 established, so T001 to T013
do not have to rediscover it.** Every anchor below was read, not guessed.

- **Existing tests a T-task must *update*, each of which would otherwise pass against
  the old behaviour and silently stop testing anything:**
  - T001: `TestRecordCannotCarryFreeFormContent` (`internal/audit/audit_test.go:500`)
    restates `Record`'s field list and asserts the count — append `"RequestID"`.
    `TestEmitWritesNoKeyBeyondTheDocumentedSeven` (`:94`) needs **no** change; its
    record carries no `RequestID`.
  - T006: `TestPromptNamesNoPromptTextInItsError` (`internal/session/manager_test.go:1234`)
    and `internal/audit/leak_test.go:1358` both arm `FailOp(tmuxctl.OpPaste, …)`. After
    the switch to `PasteBracketed` the paste no longer fails: the first test asserts
    nothing and the leak suite's "a prompt the host refused" case stops being a 500.
    Both must become `OpPasteBracketed`. `leakRun.pasted` (`leak_test.go:1504`) already
    reads both ops.
  - T006: the exact-argv expectations are in three places —
    `TestPromptPastesThenSubmits` (`manager_test.go:1149`),
    `TestAPromptIsDeliveredByteForByte` (`internal/httpapi/sessions_test.go:1046`),
    and `TestCompactUsesBufferPath` (`manager_test.go:1258`, which must stay as it is).
    The bracketed argv is `tmux paste-buffer -p -d -b <buffer> -t <pane>`.
  - T007 and T009: each liveness read records one `OpList`, so those same exact-call
    lists grow by two in total. Expect `capture-pane, List, load-buffer, paste-buffer,
    List, send-keys` on the httpapi path.
- **`guardDelivery` (`internal/session/input.go:215`) is shared by `Type` and
  `PressKey`.** T005's `StateFailed` refusal goes inside `Prompt`, not there; FR-013
  leaves the browser door alone.
- **`Continue` claims the restart *before* it locks (`manager.go:2760`), `Prompt` will
  lock before it claims.** That inversion cannot deadlock because `claimRestart` is
  non-blocking — a `Prompt` holding the lock is refused and returns. Keep the comment
  T010 asks for; this is the kind of correct thing a later reader would "fix".
- **The fake already models the gate's input.** `fakeAliveCommand = "claude"`
  (`internal/tmuxctl/fake.go:240`) is the default pane command for a created or seeded
  session, `livenessOf` (`:249`) is the comparison, and `Manager.Create` writes
  `@crswd-binary` as `paneProcesses(template)` (`manager.go:2274`), which for the
  Claude harness is just `claude` (only Codex sets `PaneProcesses`). So the default
  fixtures read *running* and existing Prompt tests keep passing; a fake session with
  no `@crswd-binary` reads *unknown* and also passes.
- **`ExitAfterPaste` has a mirror to copy**: `QuitAfterInterrupts`
  (`internal/tmuxctl/fake.go:390`), including the "seed if absent" behaviour and the
  `"bash"` it leaves as the pane command.
- **tmux inserts the paste brackets only when the pane's application has requested
  bracketed-paste mode (DECSET 2004).** This is why T006 does **not** break the
  `quickstart` shim suite (`cmd/crswd/quickstart_test.go:1465` onwards drives a
  line-oriented shim and is *not* in the allowlist) and why T012's `cat` case receives
  the bytes unwrapped. Spec 018's own tmux test had to `printf '\033[?2004h'` first.
- **`internal/httpapi/input_test.go` is NOT in the allowlist.** T004's "the type route
  still delivers `/compact`" test therefore goes in `sessions_test.go` and *uses*
  `newTyper` / `typed` / `ops` from that file (same package, read-only).
- **The 409 has a complete three-part mirror**: `ErrOrphanedSession`
  (`manager.go:139`) → `bodyTeardownUnverified` (`internal/httpapi/sessions.go:32`) →
  `failTeardownUnverified` (`:745`). T005 copies its shape exactly.
- **A 400 gate gets its audit deny for free** from `rejectBadRequest`
  (`internal/httpapi/decode.go:160`), whose reason must be an error authored in this
  repo; the uniform body is `bodyBadRequest` (`decode.go:21`).
- **`promptFixture.post` signs the timestamp and body**, so T011's four posts need four
  distinct instants or the second is refused as a replay
  (`internal/httpapi/sessions_test.go:899`).
- **T012 must not redefine** `newModeFixture`, `modeSocketFor`, `waitForPane` or
  `containsLine` — `internal/session/mode_test.go` already declares them in the
  `tmux`-tagged build of this package, and `capNotUnderTest` comes from
  `manager_test.go`.
- **T013's insertion point**: `docs/security.md` has `### 5. Dependencies` at line 395
  and `## Configuration, and the page that shows it` at line 410. The new subsection
  goes between them, numbered 6 so §1–§5 references elsewhere keep their meaning.
- Next task is **P5** (amend `specs/001-crswd-daemon-core/contracts/http-api.md`).
  Markdown only. Its anchors are already recorded in Iteration 2: the prompt section is
  lines 256–301, the status table is at line 111 (where `409` reads only "Teardown
  could not be verified"), and the six route headings are at lines 128, 183, 223, 231,
  256 and 304. P5's second Verify counts `^## \`(GET|POST|DELETE|PATCH|PUT) ` and must
  print **6** — a seventh heading is the failure it exists to catch.

### Iteration 5 — P5 (amend http-api.md)

**Changed.** `specs/001-crswd-daemon-core/contracts/http-api.md` only, Markdown.
Everything FR-020 names is now in it:

- **Status table (line 121).** The `409` row reads "Teardown could not be verified —
  the session may still be alive; or the session is not ready to receive a prompt
  (spec 020)". The teardown half is untouched; a destroy's `409` and its body are
  unchanged.
- **Field table on the prompt route.** A `request_id` row: optional, 1 to 32 of
  `[a-z0-9-]`, neither issued nor deduplicated by the daemon, echoed in the 202 and
  written on the one audit record, and absent means absent in both. The `text` row now
  reads "Not otherwise validated or escaped, **past its first character**" and points
  at the leading-character refusal — left as it was, the contract would have asserted
  "not otherwise validated" on the same route whose first character FR-012 refuses.
- **A `request_id` echo example**, next to the existing `parked_on` one, so the
  additive-field shape is shown rather than described.
- **A "Failures (spec 020)" block**: the one-line `·`-separated summary every other
  route section uses, then one bullet each for the leading-`!`/`/` 400, the malformed
  `request_id` 400 and the not-ready 409 — stating that all three decide before any
  tmux command runs, that the 409's three states are indistinguishable by status, body
  and reason, that `dead` is unchanged, and that the type route still sends `/compact`.
- **The post-paste case is stated as a caller-facing fact** (FR-005): a 409 can also
  arrive after the bytes reached the pane, in which case the daemon sends `C-u` and no
  Enter. A contract that said only "refused prompts deliver nothing" would be wrong
  about exactly that case, and a caller reading a 409 needs to know nothing was
  submitted.
- **FR-042's line amended in place**: prompt text still never appears anywhere, and
  `request_id` is named as the one caller-chosen value admitted onto a record, per
  spec 020 FR-002.
- **Four rows in "Contract tests this implies"**, one per new behaviour, keyed
  `020 FR-012`, `020 FR-008`, `020 FR-017`, `020 FR-018` — the `Covers` column held
  only spec 001 identifiers before, so the new rows name their spec. These four are
  the same assertions T003, T004 and T005 already own in tasks.md; no new requirement.
- **Also amended, one sentence of the route's opening paragraph.** It said the route is
  "Implemented with `load-buffer` from stdin plus `paste-buffer -d`"; FR-009 makes that
  `paste-buffer -p -d`. Leaving it would have left the contract naming the wrong tmux
  invocation for the route this milestone changes. The old form is named, so a reader
  of a milestone-1 client knows what moved.

**Not changed, deliberately.** The header's "Six operations, no others (FR-006)" and
the six route headings — a seventh is what P5's second Verify exists to catch. Nothing
about the `ESC[201~` marker (spec.md FR-019, and Iterations 2 and 3 both said so).

**Commands run.**

- Verify part 1: `grep -c 'session not ready' specs/001-crswd-daemon-core/contracts/http-api.md`
  → `2` (the 409 bullet and its JSON body), so the `grep -q` form holds.
- Verify part 2: the heading count must be 6. The pattern as written in
  IMPLEMENTATION_PLAN.md contains a backtick and **the environment refused it**
  (`grep -cE '^## \`(GET|POST|…) '` → "requires approval"). Measured with the backtick
  replaced by `.`: `grep -cE '^## .(GET|POST|DELETE|PATCH|PUT) ' …` → **6**, and
  `grep -nE '^## ' …` lists exactly those six route headings plus Authentication,
  Common rules and Contract tests. Same limit as P1/P4's bracketed forms (Findings).
- `grep -rc 'NEEDS CLARIFICATION' specs/020-prompt-route-safety specs/001-crswd-daemon-core/contracts/http-api.md`
  → `0` for all five files, so **G1's recursive grep passes as of this commit**.
- `gofmt -l .` → no output. `go build ./...`, `go vet ./...`, `go vet -tags tmux ./...`,
  `go vet -tags quickstart ./cmd/crswd`, `test ! -e go.sum` → all exit 0.
- `git status --short` → ` M ralph/plans/…/IMPLEMENTATION_PLAN.md`,
  ` M ralph/plans/…/PROGRESS.md`, ` M specs/001-crswd-daemon-core/contracts/http-api.md`.
- `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` → **not run.**
  `golangci-lint version` still exits 127 here. P5 changed Markdown only, so no Go file
  was touched and the Go linter cannot have regressed.

**For the next iteration.**

- Next task is **G1**, the implement gate: `! grep -rq 'NEEDS CLARIFICATION'
  specs/020-prompt-route-safety`. Measured `0` above, so it passes; run it as written
  (a plain `!`-prefixed `grep -rq` has not been refused here, unlike the bracketed
  forms) and tick it. **Do not start T001 in the same iteration** — one task per
  iteration.
- **The contract is now ahead of the code, and that is the expected state** for the
  rest of this milestone: http-api.md describes the 409, the two 400s and the bracketed
  paste before T003 to T009 implement them. Nothing asserts the contract against the
  code, so no test turns red from this commit; T003's and T005's own tests are what
  close the gap.
- Every code anchor T001 to T013 needs is already in Iteration 4's notes. P5 read no
  Go, so nothing is added to them.

## Findings

- **`golangci-lint` is not installed in the loop's environment and cannot be installed
  there.** `golangci-lint version` exits 127, and the SessionStart hook says so too; the
  `go install …@v2.12.2` it suggests needs an approval this headless run cannot give.
  PROGRESS.md's planning note ("golangci-lint 2.12.2" on this host) was measured on the
  planning host, which is not this container. Pre-commit step 6 was therefore skipped for
  P1, which changes **Markdown only** — no Go file was touched, so the Go linter cannot
  have regressed. A T-task that changes Go will need this resolved, or its lint result
  recorded as unverified in the same words.
- **Compound shell forms are refused by this environment's command analyzer.** The
  bracketed Verify (`[ "$(grep -c …)" -ge 9 ]`) could not be executed as written — command
  substitution inside a test, `sh -c`, and `cmd; echo $?` were each declined. The
  underlying fact was measured instead: the count is exactly 9. Later tasks whose Verify
  is a plain `go test …` are unaffected; P4's and P5's `grep -c` forms will meet the same
  limit, so measure the count and record it.
- **`~/code/ai-lawnmower/loop/safe-commit.sh` is not reachable from this environment**
  (measured on P2). The tooling is confined to `/workspace`, so the script cannot be
  listed or run: `"$PWD"` is refused as unanalyzable shell, the literal-path form needs an
  approval this headless run cannot give, and `ls` on the path is blocked outright. This
  is the same confinement that keeps the two external source documents unreadable
  (Iteration 1). Iteration 1 did not run it either — its work reached the branch as the
  harness's own `ralph: iteration 1 (sweep uncommitted changes)` commit.
  **What P2 did instead**, and what a later iteration should do: `git add <each path by
  name>` — never `-A`, never `.` — then `git commit -m …`, which is what safe-commit
  wraps. `core.hooksPath` is **unset** in this container, so the `.githooks/pre-commit`
  gitleaks run does not fire locally; CI enforces it regardless (`AGENTS.md`,
  Enforcement). Setting it is a repo-config change outside this milestone's allowlist and
  was not made.
