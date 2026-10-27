# Implementation plan: a lint check that gives the same answer on every host (lm-steer-crswd-018-fix-7-fix-5)

Filed by `loop/fix-features.py` from a `verify-run.sh` FAIL (run `1022-tbfh`, parent
task `lm-steer-crswd-018-fix-7`, PR #229):

> Assertion: `golangci-lint run` exits 0 where installed
> Evidence: Installed at ~/bin. Exit 1 with 71 issues (errcheck 11, gosec 59, staticcheck 1).
> The issue paths point at another worktree (`...lm-steer-crswd-018-fix-3`), but the command
> as specified still exits non-zero.

## Why it failed

No Go code is wrong. The tree lints clean. Two host problems make a bare
`golangci-lint run` give a wrong answer:

1. **Shared cache.** golangci-lint's default cache (`~/.cache/golangci-lint`) is shared by
   every checkout on the host. It replays findings recorded against files in other
   worktrees that no longer exist. golangci-lint then cannot open those files to read their
   `//nolint` directives, so findings the source suppresses come back as live issues.
2. **Toolchain mismatch.** `~/bin/golangci-lint` is v2.12.2 built with go1.25.12. The host's
   `go` is go1.26.8. With an empty cache, golangci-lint type-checks go1.26 stdlib files and
   panics: `file requires newer Go version go1.26 (application built with go1.25)`.

## Facts measured at planning time (10/27/26, main `97a0f58`)

- `GOLANGCI_LINT_CACHE=$(mktemp -d) ~/bin/golangci-lint run` exits 2 with the panic above.
- `GOTOOLCHAIN=go1.25.12 GOLANGCI_LINT_CACHE=$(mktemp -d) ~/bin/golangci-lint run` exits 0 and
  prints `0 issues.`, both in the worktree and in a `tar --exclude=.git` copy of it.
- In that no-`.git` copy, `~/bin/golangci-lint run` with the default shared cache and default
  toolchain exits 1 with 157 lines naming files under
  `~/.local/state/lawnmower/worktrees/claude-remote-session-webhook-lm-steer-crswd-018-fix-3/`
  and `.../worktrees/pr227/`, each with `no such file or directory`.
- `~/bin/golangci-lint version` contains `built with go1.25.12`. `go env GOVERSION` prints
  `go1.26.8`. `go env GOTOOLCHAIN` prints `auto`.
- The go1.25.12 toolchain is already in the module cache
  (`~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.12.linux-amd64`), so pinning it needs no
  network on this host.
- Root `go.mod` says `go 1.23.0`. `k8s/go.mod` is a separate module and says `go 1.26.0`, so a
  go1.25 linter cannot load it. Root `golangci-lint run` does not descend into `k8s/`.
- `.golangci.yml` is v2 schema. It has no cache setting; only the `GOLANGCI_LINT_CACHE`
  environment variable moves the cache.
- `.github/scripts/` holds `open-pr.sh` and `open-pr-check/`. CI's shellcheck step
  (`.github/workflows/ci.yml` line 85) already runs `shellcheck -S warning ... .github/scripts/*.sh`,
  so a new script there is linted in CI with no workflow edit.
- `shellcheck` is at `~/.local/bin/shellcheck` on the planning host.
- The lint bullets that fail the same way: `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md`
  line 21 (`` - `golangci-lint run` exits 0. ``), `ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md`
  line 27 and `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` line 34 (both
  `` - `golangci-lint run` exits 0 where golangci-lint is installed. ``).
- `ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` asserts the parent contract has
  `23` bullets (`grep -c '^- '`). A one-line text edit keeps that count.
- Earlier loop containers (fix-7, fix-16 PROGRESS.md) had no golangci-lint, and refused
  `safe-commit.sh`, `git checkout`, `bash -c '...'`, `$(...)` and pipes at the Bash guard.

## Decisions already made (do not reopen them)

- **D1 Location.** The wrapper is `.github/scripts/lint.sh`. Its test is
  `.github/scripts/lint_test.sh`. Both are bash, start with `#!/usr/bin/env bash` and
  `set -euo pipefail`.
- **D2 Absent linter.** If `command -v golangci-lint` fails, print
  `golangci-lint not found on PATH` to stderr and exit 127. Check this before anything else.
- **D3 Cache.** If `GOLANGCI_LINT_CACHE` is unset or empty, create one with `mktemp -d`, export
  it, and remove it on exit with `trap 'rm -rf "$cache"' EXIT`. A caller's non-empty value is
  used as is and never removed.
- **D4 Toolchain.** If `GOTOOLCHAIN` is unset or empty: read the `go1.N[.P]` after
  `built with ` in `golangci-lint version`, and read `go env GOVERSION`. If the linter's
  major.minor is lower than the local one, export `GOTOOLCHAIN` set to the linter's full
  version (for example `go1.25.12`). Otherwise leave it unset. If either version cannot be
  parsed, leave it unset. A caller's non-empty `GOTOOLCHAIN` is never changed.
- **D5 Run.** `cd` to the repository root (two directories above the script), run
  `golangci-lint run "$@"` without `exec` so the trap fires, and exit with its status.
- **D6 Test.** `lint_test.sh` never needs a real golangci-lint or a real `go`. It writes stub
  `golangci-lint` and `go` executables into a `mktemp -d` directory, puts that directory first
  on `PATH` ahead of `/usr/bin:/bin`, and drives the stubs with environment variables. The
  `golangci-lint` stub answers `version` with a line in the real format and, on `run`, writes its
  arguments, `GOLANGCI_LINT_CACHE`, `GOTOOLCHAIN` and whether the cache directory exists to a
  file the test reads. It exits with a status the test chooses. The `go` stub answers
  `env GOVERSION`. The test prints `ok <case>` or `FAIL <case>: <why>` per case and exits
  non-zero if any case failed.
- **D7 Test cases.** (a) no cache given: the stub saw an existing directory that is not
  `$HOME/.cache/golangci-lint`, and it is gone after the script exits; (b) a caller's cache is
  passed through and still exists after; (c) linter go1.25.12, local go1.26.8: stub saw
  `GOTOOLCHAIN=go1.25.12`; (d) linter go1.26.1, local go1.26.8: stub saw no `GOTOOLCHAIN`;
  (e) caller `GOTOOLCHAIN=local` is kept; (f) the stub's exit status 1 is the script's exit
  status; (g) extra arguments reach `run` unchanged; (h) with no `golangci-lint` on `PATH` the
  script exits 127 and stderr contains `golangci-lint not found`.
- **D8 Contracts.** Each of the three lint bullets named in "Facts measured" changes only its
  command: `` `golangci-lint run` `` becomes `` `bash .github/scripts/lint.sh` ``. The rest of
  each line stays. No bullet is added or removed.
- **D9 Out of scope.** `AGENTS.md` (its Lint row stays `golangci-lint run`), `.golangci.yml`,
  `.github/workflows/`, `k8s/`, reinstalling `~/bin/golangci-lint`, and the fix-7 notebook's
  `PROMPT.md` and `IMPLEMENTATION_PLAN.md`. Record in PROGRESS.md that `AGENTS.md`'s Lint row
  and the host's go1.25-built linter are left for a human.

## Pre-commit commands (every task, in this order)

1. `shellcheck -S warning .github/scripts/*.sh` exits 0. If shellcheck is not installed, record that in PROGRESS.md.
2. `bash .github/scripts/lint_test.sh` exits 0. Skip this step for a task that runs before T1 is ticked.
3. `go build ./...` exits 0.
4. `test ! -e go.sum` exits 0.
5. `git status --short` lists only paths in "Files touched" or this notebook directory.

## Tasks

- [ ] T1 Write `.github/scripts/lint_test.sh` per D6 and D7, run it and record its failure (lint.sh absent), then write `.github/scripts/lint.sh` per D1 to D5. Verify: `bash .github/scripts/lint_test.sh` exits 0 and prints 8 `ok` lines, and `shellcheck -S warning .github/scripts/lint.sh .github/scripts/lint_test.sh` exits 0.
- [ ] T2 Per D8, repoint the lint bullet in the three contracts named in "Facts measured" to `bash .github/scripts/lint.sh`. Verify: `grep -c 'lint.sh' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `1`, same for the fix-7 and fix-9 contracts, and `grep -c '^- ' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `23`.
- [ ] T3 Append one line to `docs/fixes-log.md` in its format, dated 2026-10-27, tagged `(lm-steer-crswd-018-fix-7-fix-5)`: a bare lint run read other checkouts' findings from the shared cache and panicked on a go1.25-built linter; `.github/scripts/lint.sh` sets a private `GOLANGCI_LINT_CACHE` and pins `GOTOOLCHAIN`. Verify: `grep -q 'GOLANGCI_LINT_CACHE' docs/fixes-log.md` exits 0.
- [ ] F1 Run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Where golangci-lint is not installed, record that for the three assertions that need it. Verify: `bash .github/scripts/lint_test.sh` exits 0 and `go build ./...` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-7-fix-5/` is always in scope):

```allowlist
.github/scripts/lint.sh
.github/scripts/lint_test.sh
ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md
ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md
ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `.golangci.yml`, `.github/workflows/`,
`k8s/`, `internal/`, `cmd/`, `specs/`, `go.mod`, `go.sum`, and every other file under `ralph/`.
