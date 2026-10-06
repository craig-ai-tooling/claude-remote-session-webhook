# Progress: interactive input (spec 018)

Append one entry per iteration, newest last. Do not rewrite earlier entries.

## Iteration 0: planning (2026-10-06)

- Plan written from measured behaviour. See `specs/018-interactive-input/research.md` R1–R3.
- The probes ran on private `tmux -L` sockets and left no server behind. The
  Codex probe ran in `~/code/dispatch`, which was already trusted, and wrote
  nothing to `~/.codex/config.toml`.
- Not yet dispatched. `ralph/loop.sh` resolves this notebook with
  `RALPH_PLAN=interactive-input`, or on a branch named `plan/interactive-input`.

## Iteration 1: T001 (2026-10-06) — NEEDS CLARIFICATION

T001 was implemented and its own checks passed, then reverted because the full gate
cannot go green in this environment.

- Passed: `go test ./internal/tmuxctl ./internal/session`, `go test -tags tmux
  -v ./internal/tmuxctl -run FiveThousand` (ran, not skipped), the full
  `go test -tags tmux ./internal/tmuxctl/...`, `gofmt -l .`, `go vet` on all three
  tag sets, `golangci-lint --version` (2.12.2).
- Guard proven: with the `set-option ... ";"` elements removed from `argvNew`,
  `TestTmuxNewSessionKeepsFiveThousandLinesOfHistory` failed ("timed out waiting for
  history to fill"). Restored, it passed.
- Blocked: `go build ./...` and `go test ./...` fail with `error obtaining VCS status:
  exit status 128`. `go build -buildvcs=false ./...` succeeds. The failing test is
  `internal/release` `TestBinaryIsStaticallyLinked`, which shells out to `go build
  ./cmd/crswd` without that flag. `git status` works from the shell; go's own git
  call exits 128 here. This is a worktree/sandbox environment problem, not T001.
  `golangci-lint run` and `test ! -e go.sum` were not run.
- Needed from Craig: either set `GOFLAGS=-buildvcs=false` for the loop, or fix go's
  git access in this worktree (the `.git` dir is under the main checkout,
  `/home/nctiggy/code/claude-remote-session-webhook/.git/worktrees/...`). Then flip
  T001 back to `- [ ]`.
- The reverted T001 change, for the next run: chain `set-option -g history-limit 5000
  ;` into `argvNew` (`fake.go`), add `const HistoryLimit = 5000` after the
  `Controller` interface, update the three argv expectations (`fake_test.go`,
  `exec_test.go`, `session/manager_test.go`), add the tmux test.
- Edits through a python heredoc and `go -C` were refused by the sandbox; plain
  `go ...` from the worktree and the Edit tool work.

## Findings

- `GOFLAGS`-less `go build` fails on VCS stamping in this worktree (see Iteration 1).

- Operator (CS, 2026-10-06): loop now runs with GOFLAGS=-buildvcs=false; T001 unblocked.

## Iteration 2: T001 (2026-10-06) — NEEDS CLARIFICATION

T001 was implemented a second time and reverted. The GOFLAGS fix worked: `go build`,
`go vet` (default, `-tags tmux`, `-tags quickstart`) and `go test ./...` all passed.
The one step that could not run is the lint gate.

- Passed: `gofmt -l .` (empty), `go build ./...`, the three `go vet` runs,
  `go test ./...`, `go test -tags tmux ./internal/tmuxctl/...`,
  `go test -tags tmux -v ./internal/tmuxctl -run FiveThousand` (ran, 0.04s),
  `golangci-lint --version` (2.12.2), `test ! -e go.sum`.
- Guard proven: with the `set-option ... ";"` elements removed from `argvNew`,
  `TestTmuxNewSessionKeepsFiveThousandLinesOfHistory` FAILED ("timed out waiting for
  history to fill", 15s). Restored, it PASSED.
- Blocked: `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` is refused by the
  sandbox. `$(...)` is rejected ("A variable in this command can't be checked"), and
  `GOLANGCI_LINT_CACHE=<literal path> golangci-lint run` is refused as needing approval.
  Plain `golangci-lint run` uses the shared default cache, which holds paths from other
  sessions' worktrees under `/tmp/claude-1000/.../scratchpad/crswd-wt/`. It exits 1 with
  63 issues (errcheck 11, gosec 52), every one in those stale `/tmp` paths and none in
  this worktree. That is not a clean pass, so the gate is not met.
- Needed from Craig: either run the loop with `GOLANGCI_LINT_CACHE` already exported to a
  fresh directory, or allow the env-prefixed `golangci-lint run` form. Then flip T001
  back to `- [ ]`. Do not `golangci-lint cache clean`: the cache is shared with other
  sessions.
- The reverted T001 change is the same as Iteration 1's note. `argvNew` in `fake.go`
  returns `{"tmux","set-option","-g","history-limit",strconv.Itoa(HistoryLimit),";","new-session",...}`;
  `const HistoryLimit = 5000` goes after the `Controller` interface in `controller.go`;
  the three argv expectations are `fake_test.go:65`, `exec_test.go:240`,
  `session/manager_test.go:219`; the tmux test goes before `TestTmuxCreateHasKill` and reads
  `#{history_size}` and `#{history_limit}` via `tmux -L <socket> display -p`.
- Do not use `$?` in a Bash command here: the sandbox rejects it the same way.

- Operator (CS, 2026-10-06): loop now exports GOLANGCI_LINT_CACHE to a fresh per-worktree dir; plain `golangci-lint run` is clean. T001 unblocked.

## Iteration 3: T001 (2026-10-06)

Chained `set-option -g history-limit 5000 ;` into `argvNew` (`fake.go`), added
`const HistoryLimit = 5000` to `controller.go`, updated the three argv expectations,
added `TestTmuxNewSessionKeepsFiveThousandLinesOfHistory`.

- Guard proven: with the `set-option ... ";"` elements removed from `argvNew`,
  `TestTmuxNewSessionKeepsFiveThousandLinesOfHistory` FAILED ("timed out waiting for
  history to fill", 15s). Restored, it PASSED (0.05s).
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint
  2.12.2 (0 issues), `go test -tags tmux ./internal/tmuxctl/...`, no `go.sum`.
- Next iteration: `GOFLAGS=-buildvcs=false` and `GOLANGCI_LINT_CACHE` are set by the loop.
  `$(...)` and `$?` in a Bash command are refused by the sandbox, so reading env vars in
  the shell is not possible; run the tools directly.

## Iteration 4: T002 (2026-10-06)

Added `PasteBracketed` to `Controller`, `Fake` (`OpPasteBracketed`, `argvPasteBufferBracketed`),
`Exec` and `ArgvPasteBracketed`, with the argv, fake, exec and real-tmux tests the task names.

- Guard proven: with `"-p"` removed from `argvPasteBufferBracketed`,
  `TestFakeRecordsExactArgv`, `TestExecPasteBracketedKeepsCallerTextOffTheCommandLine` and
  `TestTmuxPasteBracketedWrapsTheText` all FAILED. Restored, all PASSED.
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint
  2.12.2 (0 issues), `go test -tags tmux ./internal/tmuxctl/...`, no `go.sum`.
- Next iteration: the task's tmux test as written ("wait until CapturePane contains
  `cat -v`") is racy. The typed command line contains `cat -v` before the shell has run
  `printf` and `stty`, so the paste arrives unbracketed and the test times out. The test
  now types `...; echo READY; cat -v` and waits for a pane line equal to `READY`. Stable
  over `-count=5`.
- The sandbox refuses ad-hoc multi-statement tmux commands in Bash. To debug a tmux test,
  add a temporary `t.Logf` of the pane and remove it.
