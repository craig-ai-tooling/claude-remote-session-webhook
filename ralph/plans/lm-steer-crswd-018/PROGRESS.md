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
