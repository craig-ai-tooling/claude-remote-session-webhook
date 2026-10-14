# Implementation plan: prompt route safety (lm-steer-crswd-018)

Hardens `POST /sessions/{id}/prompt`, the route lawnmower's `answer` and opp-lease
handoff already use, so a voice `steer` could use it later. Source of the
requirements: `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`, section
(g) "Phase 3 detail". Probe evidence: `~/code/ai-lawnmower/ralph/plans/lm-steer-design/results.md`.
Both are outside this repo. Read them, never edit them.

Spec Kit lane: research, spec, plan and tasks are written and committed before any Go
changes. The spec lives at `specs/020-prompt-route-safety/` because 018 and 019 are
already taken here.

## Conventions

- `- [ ]` open, `- [x]` done, `- [!]` blocked (reason in PROGRESS.md).
- The loop takes the topmost open task. The order is the dependency order.
- Every T-task adds a test that fails against the code before the change. PROMPT.md
  says how to prove it.

## Decisions already made (copy these into research.md; do not reopen them)

- **D1 Spec number.** `specs/020-prompt-route-safety`. 018 is interactive-input, 019 is codex-runtime.
- **D2 Liveness gate (req 1).** Probe check 13: `@crswd-binary` matched `pane_current_command`
  (`claude`) on 6 of 6 live panes, 3 of them `rcq`. The gate reads the session's row from the
  controller's `List` and its `Claude` liveness. `LivenessStopped` refuses. `LivenessUnknown`
  passes, as spec 012 FR-006 reads it. It runs before the paste and again after the paste; if
  the second read is stopped, the daemon sends `C-u` and no Enter. No new `Controller` method,
  so the k8s module's `podctl` is untouched.
- **D3 Not-ready answer (reqs 1, 2, 5).** One sentinel `session.ErrSessionNotReady`. The prompt
  handler maps it to `409` with body `{"error":"session not ready"}` and an audit Deny with reason
  `session not ready`. A `failed` session gets it. `dead` stays as it is today.
- **D4 Bracketed paste (req 3).** `Manager.Prompt` uses `PasteBracketed` for every harness.
  `Manager.Compact` keeps its harness-based paste. No newline refusal: the opp-lease handoff
  pastes multi-line item bodies.
- **D5 Leading `!` or `/` (req 4).** API door only. Refuse when the first character that is not
  space, tab, CR or LF is `!` or `/`: `400` with the uniform bad-request body, before any tmux
  command. Grep of 10/14/26: lawnmower's callers (`console.py` `_answer_action` via
  `session_resume.deliver`, `lawnmower-route` `resume_prompt`, `_batch_listing_prompt`,
  `opp_followup_prompt`) all start with a letter. The browser type route is unchanged. This
  amends spec 018 FR-006 ("the API door's prompt route MUST be unchanged").
- **D6 Lock and buffer (req 5).** The per-session mutex (`lockSession`) and a fresh buffer name
  per paste (`newBufferName`) already exist from the 018 and 019 reviews. New here: `Prompt`
  takes the restart claim (`claimRestart`, non-blocking) after `lockSession`, refuses with
  `ErrSessionNotReady` if a revive or continue holds it, and releases it on return.
- **D7 request_id (req 6).** Optional JSON field `request_id`, 1 to 32 characters of `[a-z0-9-]`.
  Invalid is `400`. Valid is echoed in the 202 body and written as `request_id` on the audit
  record. Absent means no field anywhere. Amends 001 FR-042 for this one field. No length, no hash.
- **D8 Registry gate (req 7) deferred.** It was a MAY. Probe check 8 left open whether crswd can
  read `sessions/<pid>.json` under v2 option B (no FR in spec 017 places it). Checks 3 and 9 show
  the registry misses the trust dialog and a half-typed draft. The spec lists it under Out of
  scope with that reason. It is not a NEEDS CLARIFICATION.
- **D9 Out of scope.** Opening a transcript, holding or calling an LLM, a seventh API route, free
  text on the browser door beyond spec 018, the startup lost-Enter window (probe check 1b), the
  socket transport (probe verdict, owned by lm-steer-write), a new `internal/` package.

## Pre-commit commands (every task, in this order)

1. `gofmt -l .` prints nothing.
2. `go build ./...`
3. `go vet ./...`
4. `go vet -tags tmux ./...`
5. `go vet -tags quickstart ./cmd/crswd`
6. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run`
7. `test ! -e go.sum`
8. `git status --short` lists only paths in "Files touched" or the notebook directory.

## Tasks

### Spec (Spec Kit lane; no Go changes)

- [ ] P1 Write research.md in `specs/020-prompt-route-safety/`: one `## D<n>` section per decision D1 to D9 above, each with the evidence it cites. Verify: `[ "$(grep -c '^## D[0-9]' specs/020-prompt-route-safety/research.md)" -ge 9 ]` exits 0.
- [ ] P2 Write spec.md in `specs/020-prompt-route-safety/` from the spec template: FRs for reqs 1 to 6, req 7 under Out of scope, amendments to 001 FR-042 and 018 FR-006. Verify: `grep -q 'session not ready' specs/020-prompt-route-safety/spec.md && grep -q request_id specs/020-prompt-route-safety/spec.md && ! grep -q 'NEEDS CLARIFICATION' specs/020-prompt-route-safety/spec.md`.
- [ ] P3 Write plan.md in `specs/020-prompt-route-safety/` from `.specify/templates/plan-template.md`, with a Constitution Check against `.specify/memory/constitution.md` and this file's allowlist as its file list. Verify: `grep -q 'Constitution Check' specs/020-prompt-route-safety/plan.md && ! grep -q 'NEEDS CLARIFICATION' specs/020-prompt-route-safety/plan.md`.
- [ ] P4 Write tasks.md in `specs/020-prompt-route-safety/`: entries T001 to T013 with the same titles as below, each naming files, the failing-before test and its Verify command. Verify: `[ "$(grep -cE '^- \[ \] T0(0[1-9]|1[0-3]) ' specs/020-prompt-route-safety/tasks.md)" -eq 13 ]` exits 0.
- [ ] P5 Amend `specs/001-crswd-daemon-core/contracts/http-api.md`: prompt section gains `request_id`, the 409 not-ready and the 400 leading-`!`/`/` rule; status table 409 row covers not-ready. Verify: `grep -q 'session not ready' specs/001-crswd-daemon-core/contracts/http-api.md` and `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints 6.
- [ ] G1 Implement gate: confirm the spec has no open question before any Go change. Verify: `! grep -rq 'NEEDS CLARIFICATION' specs/020-prompt-route-safety` exits 0. If it fails, mark this task `- [!]` and stop.

### Implementation (each T-task: failing-before test first)

- [ ] T001 audit: `Record` gains `RequestID` written as `request_id`, omitted when empty, in `internal/audit/audit.go`. Test both cases in `internal/audit/audit_test.go`. Verify: `go test ./internal/audit/` exits 0.
- [ ] T002 httpapi: `RequestAudit.SetRequestID` in `internal/httpapi/middleware.go` carries the value onto the request's one audit record. Test in `internal/httpapi/middleware_test.go`, name containing `RequestID`. Verify: `go test ./internal/httpapi/ -run RequestID` exits 0.
- [ ] T003 httpapi: prompt body gains optional `request_id` (D7). Invalid is 400 with no tmux command; valid is echoed in the 202 and set on the audit record. Tests in `internal/httpapi/sessions_test.go`, names containing `RequestID`. Verify: `go test ./internal/httpapi/ -run 'RequestID|Prompt'` exits 0.
- [ ] T004 httpapi: refuse a prompt whose first non-whitespace character is `!` or `/` with 400 and no tmux command (D5). Add a test that the browser type route still delivers `/compact`. Verify: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0.
- [ ] T005 session + httpapi: add `ErrSessionNotReady`; `Prompt` refuses a `failed` session with it before any tmux command; the handler maps it to 409 `{"error":"session not ready"}` (D3). Verify: `go test ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0.
- [ ] T006 session: `Prompt` pastes with `PasteBracketed` for every harness; `Compact` unchanged (D4). Update `TestPromptPastesThenSubmits` and add a two-line case. Verify: `go test ./internal/session/ -run 'Prompt|Compact'` exits 0.
- [ ] T007 session: liveness gate before the paste (D2). `LivenessStopped` returns `ErrSessionNotReady` with no paste; `LivenessUnknown` delivers; a `List` failure is an error. Use the fake's `SetPaneCommand`. Verify: `go test ./internal/session/ -run Prompt` exits 0.
- [ ] T008 tmuxctl fake: add `ExitAfterPaste(name)`, which sets the pane command to `bash` after the next `Paste` or `PasteBracketed` on that session. Test in `internal/tmuxctl/fake_test.go`. Verify: `go test ./internal/tmuxctl/ -run ExitAfterPaste` exits 0.
- [ ] T009 session: re-check liveness after the paste. If stopped, send `C-u`, send no Enter, return `ErrSessionNotReady` (D2). Test with `ExitAfterPaste` and assert the recorded keys. Verify: `go test ./internal/session/ -run Prompt` exits 0.
- [ ] T010 session: `Prompt` takes the restart claim after `lockSession`; held means `ErrSessionNotReady` with no tmux command (D6). Add a two-goroutine test: distinct buffer names, paste then Enter never interleaved. Verify: `go test -race ./internal/session/ -run Prompt` exits 0.
- [ ] T011 httpapi: fixtures in `internal/httpapi/testdata/lawnmower/` for answer, resume, opp-lease batch, opp follow-up (texts per PROGRESS.md "Payload fixtures"). A test named with `Lawnmower` posts each and gets 202 delivered. Verify: `go test ./internal/httpapi/ -run Lawnmower` exits 0.
- [ ] T012 session tmux test: new file prompt_tmux_test.go in `internal/session/` (`//go:build tmux`, private `-L` socket, pattern from `mode_test.go`). Pane on `sh` with `@crswd-binary` claude is refused and stays empty; a two-line prompt into `cat` arrives once. Verify: `go test -tags tmux ./internal/session/ -run Prompt` exits 0.
- [ ] T013 docs: add a "Prompt route gates (spec 020)" subsection under "Non-negotiables" in `docs/security.md` naming the liveness gate, failed refusal, bracketed paste, leading-character rule, restart claim and `request_id`. Verify: `grep -q 'spec 020' docs/security.md && test "$(wc -l < AGENTS.md)" -lt 150`.
- [ ] F1 Final gate: run `go test ./...`, `go test -tags tmux ./...` and the pre-commit commands; confirm PROGRESS.md has a failing-before row for each of reqs 1 to 6. Verify: `go test ./... && go test -tags tmux ./...` exits 0 and `grep -oE '^\| req [1-6] ' ralph/plans/lm-steer-crswd-018/PROGRESS.md | sort -u | wc -l` prints 6.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018/` is always in scope):

```allowlist
specs/020-prompt-route-safety/research.md
specs/020-prompt-route-safety/spec.md
specs/020-prompt-route-safety/plan.md
specs/020-prompt-route-safety/tasks.md
specs/001-crswd-daemon-core/contracts/http-api.md
internal/audit/audit.go
internal/audit/audit_test.go
internal/audit/leak_test.go
internal/httpapi/middleware.go
internal/httpapi/middleware_test.go
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
internal/httpapi/lawnmower_payload_test.go
internal/httpapi/testdata/lawnmower/answer.txt
internal/httpapi/testdata/lawnmower/resume.txt
internal/httpapi/testdata/lawnmower/opp-lease-batch.txt
internal/httpapi/testdata/lawnmower/opp-followup.txt
internal/session/manager.go
internal/session/manager_test.go
internal/session/prompt_tmux_test.go
internal/tmuxctl/fake.go
internal/tmuxctl/fake_test.go
docs/security.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `internal/tmuxctl/controller.go`,
`internal/tmuxctl/exec.go`, `go.mod`, any new directory under `internal/`.

## After the loop (a session does these, not Ralph)

1. Open the PR from this branch; merge through the no-mistakes path.
2. Tag, release and let the daemon self-update.
3. Run the "After merge" check in VALIDATION_CONTRACT.md.
