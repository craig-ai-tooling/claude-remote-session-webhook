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
