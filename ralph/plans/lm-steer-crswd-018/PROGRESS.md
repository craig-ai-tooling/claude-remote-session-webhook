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
