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

## Iteration 5: T003 (2026-10-06)

Added `CaptureHistory` to `Controller`, `Fake` (`OpCaptureHistory`, `argvCaptureHistory`, `SetHistory`),
`Exec` (`ErrHistoryTooLarge`, `maxHistoryBytes`) and `ArgvCaptureHistory`, with the argv, fake, exec
and real-tmux tests the task names.

- Guards proven: with the line check changed to `HistoryLimit+1000000`, the "one line past the bound"
  and "unterminated" rows of `TestExecCaptureHistoryRefusesPastTheBound` FAILED; restored, PASSED.
  With `"-E", "-1"` removed from `argvCaptureHistory`, `TestTmuxCaptureHistoryReturnsOnlyHistory`
  FAILED; restored, PASSED. The 4 MiB row was not broken separately.
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint 2.12.2
  (0 issues), `go test -tags tmux ./internal/tmuxctl/...`, no `go.sum`.
- Next iteration: the exec stub passes stdout through one env var, which Linux caps near 128 KiB,
  so a 4 MiB stdout cannot go through `stub.stdout`. `stub` gained a `fill` byte count
  (`CRSWD_TEST_STUB_FILL`, also listed in `testSessionEnv`'s forwarded names). Use it for any
  other large-output row.
- The task's tmux test waits for the pane to contain `6000`, which the typed `seq 1 6000` also
  contains. It now waits for a line equal to `6000`, as T002's READY wait does.
- A bare `cd` in Bash persists across calls; an earlier `cd internal/tmuxctl` made `./...` resolve
  wrongly until a bare `cd` back to the worktree root.

### Findings

- `argv_test.go` `TestArgvWrappersEqualBuilders` has no `ArgvPasteBracketed` row, which T002's entry
  asked for. Not fixed here (T002 is closed and the task did not name it).

## Iteration 6: T004 (2026-10-06)

Added `internal/session/input.go` (`Key`, `Keys`, `ParseKey`, `ValidateTyped`, `MaxTypeBytes`, three
sentinels, unexported `tmuxKeys`) and `input_test.go` with the three tests the task names.

- Guard proven: with `\r` and `0x1b` allowed through the control-byte check, `TestValidateTyped`
  rows `paste_terminator` and `carriage_return` FAILED. Restored, all PASSED.
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint 2.12.2
  (0 issues), no `go.sum`. The `-tags tmux` run was skipped: the task did not touch `internal/tmuxctl`.
- Next iteration: `ValidateTyped` checks length before UTF-8, so an oversize invalid string reports
  `ErrInputTooLong`; a test row pins that order. T005 should call `ValidateTyped` first and leave
  `ErrEmptyPrompt` to it.

## Iteration 7: T005 (2026-10-06)

Added `Manager.Type` and `Manager.PressKey` to `internal/session/input.go`, with two unexported helpers
(`guardDelivery`, `recordDriving`) that carry Compact's guards and Touch/FleetChanged steps so the two
methods do not copy them twice. `Compact` itself is untouched.

- Guards proven: with the unknown-key check disabled, `TestPressKeyRefusesAnUnknownKey` FAILED; with
  `recordDriving` skipped in `Type`, `TestTypeRecordsTheDriving` FAILED. Restored, all PASSED.
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint 2.12.2 (0 issues),
  no `go.sum`. `-tags tmux` skipped: the task did not touch `internal/tmuxctl`.
- Next iteration: the format hook runs goimports on every Edit, so it strips an import block you add
  before the code that uses it. Add the code first, and the hook restores the imports itself.
  Python heredocs are refused by the sandbox (brace-with-quote); use Edit.
- T006 `History` must not Touch and must not return `unreadable`; the fake's `SetHistory` feeds it.

## Iteration 8: T006 (2026-10-06)

Added `Manager.History` to `internal/session/input.go` and the four tests the task names. It reuses
`guardDelivery` for the two guards, calls `CaptureHistory`, strips, and neither Touches nor calls `unreadable`.

- Guards proven: with `recordDriving` added and `Strip` removed, `TestHistoryStripsEscapes` and
  `TestHistoryDoesNotRecordTheDriving` FAILED. Restored, all four PASSED.
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint 2.12.2 (0 issues),
  no `go.sum`. `-tags tmux` skipped: the task did not touch `internal/tmuxctl`.
- Next iteration: the sandbox refuses a `cat >> file <<'EOF'` heredoc when the body holds braces next to
  quotes (Go code). Append Go with Edit; a plain-prose heredoc like this one works.
- `TestHistoryPassesTheBoundThrough` checks the record survives with `mustStored`, which fails the test if
  it is gone; the store has no `Get` to call directly.

## Iteration 9: T007 (2026-10-06)

Added `ActionDashboardType`, `ActionDashboardKey` and `ActionDashboardHistory` to `internal/audit/audit.go`
and to both tables in `audit_test.go`.

- Guard proven: with `ActionDashboardKey` spelled `dashboard.keys`, `TestEmitAcceptsEveryDocumentedAction`
  and `TestDashboardActionsAreDistinctFromAPI` FAILED on the `dashboard.key` row. Restored, all PASSED.
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint 2.12.2 (0 issues),
  no `go.sum`. `-tags tmux` skipped: the task did not touch `internal/tmuxctl`.
- Next iteration: `leak_test.go:1542` lists the dashboard action names; T012 owns it, not T007.

## Iteration 10: T008 (2026-10-06)

Added the seven input outcome codes and sentences to `outcome.go`, `inputRatePerMin = 240` to
`ratelimit.go`, the `inputs` limiter field and its construction in `newServer`, and
`TestTheInputBudgetIsBuilt` in the new `input_test.go`.

- Guards proven: with the `Server` literal's `inputs` forced to nil, `TestTheInputBudgetIsBuilt` FAILED;
  with `key-unknown`'s sentence emptied, `TestEveryOutcomeThisPackageSpellsHasASentence` FAILED.
  Restored, all PASSED.
- Full gate green: gofmt, build, three `go vet` runs, `go test ./...`, golangci-lint 2.12.2 (0 issues),
  no `go.sum`. `-tags tmux` skipped: the task did not touch `internal/tmuxctl`.
- Next iteration: `outcome_test.go`'s `spelledOutcomes` table did need the seven codes (the task said
  "only if"); the count check fails without them. The Read tool shows an extra leading tab on that
  file's lines, so match with one tab fewer than displayed.
- Removing the `inputs:` literal line breaks the build (declared and not used), so it proves nothing;
  break a guard in a way that still compiles.
