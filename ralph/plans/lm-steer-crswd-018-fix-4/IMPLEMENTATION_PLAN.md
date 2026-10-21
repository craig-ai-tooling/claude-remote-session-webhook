# Implementation plan: shell-fallback prompt refusal (lm-steer-crswd-018-fix-4)

A verifier (run 1014-la1i) failed this assertion of milestone lm-steer-crswd-018:
"shell-fallback pane gives 409 `session not ready`, no paste". On `main` today the
string `session not ready` is in no `.go` file, `Manager.Prompt` pastes into whatever
the pane runs, and no test covers a pane that fell back to a shell.

This milestone closes that one assertion: requirement 1 of
`~/code/ai-lawnmower/docs/mcp-session-steering-design.md` section "Phase 3 detail"
(the gate before the paste), and the 409 answer. It is a fix-lane change: no new
spec directory. It is logged in `docs/fixes-log.md`.

## Conventions

- `- [ ]` open, `- [x]` done, `- [!]` blocked (reason in PROGRESS.md).
- The loop takes the topmost open task. The order is the dependency order.
- Each task's Verify command is the check. It must exit 0 before the box is ticked.

## Facts measured while planning (10/21/26)

- **Liveness is already on the List row.** `tmuxctl.SessionInfo.Claude` is a `tmuxctl.Liveness`:
  `LivenessRunning`, `LivenessStopped` (pane runs something other than `@crswd-binary`, in
  practice the login shell) or `LivenessUnknown` (no `@crswd-binary` recorded; spec 012 FR-006
  says read it as alive). `internal/session/supervisor.go` `quitStepped` already finds one
  session's row by looping `m.tmux.List(ctx)` and matching `info.Name == s.TmuxName()`. Copy that.
- **The k8s controller reports `LivenessUnknown`** (`k8s/internal/podctl/list.go:58`), so the gate
  passes there. No `k8s/` change and no new `Controller` method.
- **`List` is a recorded call on the fake** (`tmuxctl.OpList`). Every test that asserts the exact
  tmux calls a prompt makes gains one `OpList` before the paste. Known ones:
  `TestPromptPastesThenSubmits` (`internal/session/manager_test.go`), `TestPromptCodexIsBracketed`
  and `TestPromptClaudeUnchanged` (`internal/session/harness_test.go`). Compare the List call by
  `Op` only: its argv comes from the unexported `argvList`. Spec 018's old contract said
  `TestPromptPastesThenSubmits` passes unmodified; this milestone supersedes that line.
- **Fake helpers that model a shell fallback.** In `internal/session` tests, after `mustCreate`,
  `f.tmux.SetPaneCommand(s.TmuxName(), "bash")` makes the row `LivenessStopped` (the create sets
  `@crswd-binary`). In `internal/httpapi` tests, `plant` seeds the tmux session with no
  `@crswd-binary` (reads `LivenessUnknown`); re-seed it with
  `f.fixture.tmux.Seed(tmuxctl.SessionInfo{Name: f.live.TmuxName(), Created: f.live.CreatedAt, Managed: true, Claude: tmuxctl.LivenessStopped})`.
- **The handler reads the pane before Prompt** (`paneDialogState` calls `Output`, a
  `CapturePane`). That read pastes nothing. "No paste" means no `OpPaste`,
  `OpPasteBracketed` or `OpSendKeys` call.
- **The quickstart stand-in reads as `sh`, not `claude`.** `writeShim` in
  `cmd/crswd/quickstart_test.go` writes a `#!/bin/sh` script named `claude`. On tmux 3.4 a pane
  running it reports `pane_current_command` = `sh`, and `#{m/r:^(claude)$,...}` = `0`. With the
  gate in, every quickstart prompt (Story2Prompt, Story4Restart, Story6Audit and two in
  `quickstart_dashboard_test.go`) would get 409. Measured fix: the shim re-execs itself through
  bash with `exec -a claude`, guarded by an environment variable, and tmux then reports `claude`.
  The measured script head:
  ```sh
  #!/bin/sh
  if [ -z "$CRSWD_SHIM_NAMED" ]; then CRSWD_SHIM_NAMED=1 exec bash -c "exec -a claude bash \"\$0\"" "$0"; fi
  ```
  The rest of the shim (the `shimReady`, `shimEnv` and `shimEcho` lines) follows unchanged.
- **Port 8765 is held on the planning host** by the deployed daemon, so the full quickstart
  suite could not be run there.

## Decisions (do not reopen them)

- **D1 Sentinel.** `ErrSessionNotReady = errors.New("session not ready")`, declared in the sentinel
  block of `internal/session/session.go`, with a why-comment.
- **D2 Gate placement.** In `Manager.Prompt`, after the empty-ID, dead and empty-text guards and
  after `lockSession`, before `m.paste`. `LivenessStopped` returns a wrapped `ErrSessionNotReady`.
  `LivenessRunning` and `LivenessUnknown` deliver. A `List` error returns a wrapped error with no
  paste. A missing row returns a wrapped `ErrSessionDead` with no paste, as `quitStepped` does.
- **D3 HTTP answer.** `refusePrompt` maps `ErrSessionNotReady` to `409` with the exact body
  `{"error":"session not ready"}` (a `bodySessionNotReady` byte slice beside
  `bodyTeardownUnverified`), and an audit `Deny` whose reason is `session.ErrSessionNotReady.Error()`.
  Every other failure keeps today's answer.
- **D4 Out of scope here.** The second check between paste and Enter with `C-u`, refusing `failed`
  sessions, bracketed paste for every harness, the leading `!` or `/` rule, the restart claim and
  `request_id`. They belong to the parent milestone lm-steer-crswd-018 (PR #225 branch
  `ralph/1015-yo8i`) and its other fix tasks. `Compact` and the browser type route are unchanged.

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

- [ ] T1 quickstart: make `writeShim` in `cmd/crswd/quickstart_test.go` re-exec under argv[0] `claude` (script head in "Facts measured"). Verify: `go vet -tags quickstart ./cmd/crswd` exits 0, and `go test -tags quickstart ./cmd/crswd -run 'Story2Prompt'` exits 0 where 127.0.0.1:8765 is free; if held, record that in PROGRESS.md.
- [ ] T2 session: add `ErrSessionNotReady` (D1) and the pre-paste liveness gate in `Manager.Prompt` (D2). New table test `TestPromptRefusesAShellFallbackPane`: stopped refuses with no paste or Enter; running and unknown deliver; List failure pastes nothing. Add `OpList` to the exact-call tests. Verify: `go test ./internal/session/` exits 0.
- [ ] T3 httpapi: map `ErrSessionNotReady` to 409 `{"error":"session not ready"}` with an audit Deny (D3). New test `TestPromptToAShellFallbackPaneAnswersNotReady` in `internal/httpapi/sessions_test.go`: exact body, no paste or Enter call, one deny record. Verify: `go test ./internal/httpapi/ ./internal/audit/` exits 0.
- [ ] T4 tmux: new `internal/session/prompt_tmux_test.go` (`//go:build tmux`), `TestPromptRefusesARealShellFallbackPane`: `newModeFixture`, create with `modeLocalName`, wait for `lastLocalLine`, Prompt a marker, expect `ErrSessionNotReady`, marker absent from the pane. Verify: `go test -tags tmux ./internal/session/ -run Prompt` exits 0.
- [ ] T5 docs: in `specs/001-crswd-daemon-core/contracts/http-api.md` add the 409 `{"error":"session not ready"}` to the prompt section and the status table's 409 row; append one dated line to `docs/fixes-log.md`. Verify: `grep -q 'session not ready' specs/001-crswd-daemon-core/contracts/http-api.md` exits 0 and the route-count grep below prints 6.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go test -tags tmux ./...` exits 0.

### Task notes

- **T1.** Change only the script string in `writeShim`. Keep `#!/bin/sh` as the first line and keep
  every existing `printf` line after the re-exec line. Do not touch `writeCodexShim`. Before
  editing, run the tmux probe below; after editing, run it again with the new head and confirm it
  prints `claude`:
  `S=$(mktemp -d); printf '%s\n' '#!/bin/sh' 'while IFS= read -r l; do echo "$l"; done' > $S/claude; chmod +x $S/claude; tmux -L probe$$ new-session -d "PATH=$S:\$PATH; exec bash --norc"; sleep 0.5; tmux -L probe$$ send-keys claude Enter; sleep 1; tmux -L probe$$ display -p '#{pane_current_command}'; tmux -L probe$$ kill-server`
- **T2.** The test is table-driven with `t.Parallel()` and uses `newManagerFixture`, `mustCreate`
  and `f.tmux.SetPaneCommand`. For the List-failure row use `f.tmux.FailOp(tmuxctl.OpList, errTmuxBroken)`
  on a fixture of its own. Count `OpPaste`, `OpPasteBracketed` and `OpSendKeys` calls after the
  `before` index; the refusal rows must have zero. The error must not contain the prompt text.
  Write the test first and record its failure line against the old code in PROGRESS.md.
- **T3.** Use `newPromptFixture`, re-seed as in "Facts measured", and `f.post`. Assert status 409,
  `answer.Body.String()` equals `{"error":"session not ready"}` exactly, and `f.only(t)` gives
  decision `deny` and reason `session not ready`. Also run the whole `internal/httpapi` package:
  other tests may count prompt calls and need the `OpList` added. Write the test first.
- **T4.** Follow the header comment of `internal/session/mode_test.go`: private `-L` socket, no
  `t.Parallel()`. `seq` exits after printing, so the pane is back at the shell and
  `@crswd-binary` is `seq`. Read the pane with the fixture's `onScreen`. If tmux is not installed,
  follow "If the task is unclear or cannot be finished" in PROMPT.md.
- **T5.** Do not add a route heading. The route-count check is
  `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md`,
  which must print 6. The fixes-log line uses the format at the top of `docs/fixes-log.md`
  (`- YYYY-MM-DD — <what broke>; <what changed>.`).

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-4/` is always in scope):

```allowlist
cmd/crswd/quickstart_test.go
internal/session/session.go
internal/session/manager.go
internal/session/manager_test.go
internal/session/harness_test.go
internal/session/prompt_tmux_test.go
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
internal/httpapi/server_test.go
internal/httpapi/middleware_test.go
internal/httpapi/actions_test.go
internal/audit/leak_test.go
specs/001-crswd-daemon-core/contracts/http-api.md
docs/fixes-log.md
```

The four test files after `sessions_test.go` are listed only so an `OpList` count can be added if
a test there asserts the exact tmux calls of a prompt. Change nothing else in them.

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `internal/tmuxctl/`, `web/`,
`go.mod`, any new directory under `internal/`, any new `specs/` directory.
