# Implementation plan: `golangci-lint run` exits 0 (lm-steer-crswd-018-fix-9-fix-2)

Fix lane, filed from a verify-run FAIL on milestone `lm-steer-crswd-018-fix-9`
(run 1022-6xpl, PR #234). The failure is in the host's lint tooling, not in the
Go code. Nothing under `internal/`, `cmd/` or `.golangci.yml` changes.

## Facts measured at planning time (10/29/26, main `5ca1507`)

- `go version` prints `go1.26.8 linux/amd64`. `GOROOT` is `/usr/local/go1.26.8`. `GOTOOLCHAIN` is `auto`.
- `go.mod` says `go 1.23.0` and has no `toolchain` line.
- `command -v golangci-lint` is `/home/nctiggy/bin/golangci-lint`. `golangci-lint version` prints
  `version 2.12.2 built with go1.25.12`. `~/go/bin/golangci-lint` is the same build.
- CI pins `version: v2.12.2` in `.github/workflows/ci.yml` (lines 192 and 251). Keep that version.
- Cause 1, the panic. A linter built with go1.25 cannot type-check the go1.26 standard library.
  With a fresh cache, `~/bin/golangci-lint run` panics with "file requires newer Go version go1.26".
- Cause 2, the polluted cache. The shared cache is `~/.cache/golangci-lint` (`golangci-lint cache status`).
  With it, both the old binary and a go1.26 build exit 1 with 33 errcheck, 122 gosec, 4 staticcheck
  issues. The paths are `../pr227/internal/...`, another worktree's files. This tree has none of them.
- Measured remedy. `GOTOOLCHAIN=local GOBIN=<scratch> go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`
  built `version 2.12.2 built with go1.26.8`. With a fresh cache it printed `0 issues.` and exited 0.
  After `golangci-lint cache clean`, two runs on the default cache each printed `0 issues.` and exited 0.
- Cross-check: `GOTOOLCHAIN=go1.25.12 ~/bin/golangci-lint run` with a fresh cache also exits 0, `0 issues.`
  So the code is clean. Only the tool and its cache are broken.
- Earlier loops recorded `golangci-lint` missing inside the ralph container. If it is missing for you,
  T1 cannot be done from where you are. Block it; do not fake it.
- Other sessions on this host run `golangci-lint` from other worktrees. They can pollute the shared
  cache again after T2. If `../` paths come back, run `golangci-lint cache clean` and rerun once.

## Decisions already made (do not reopen them)

- **D1 Version.** Reinstall exactly `v2.12.2`, the CI pin. Do not change the pin or `.golangci.yml`.
- **D2 Toolchain.** Build it with `GOTOOLCHAIN=local`, so the local go1.26.8 compiles it.
- **D3 Where.** Install over the binary that `command -v golangci-lint` names, by setting `GOBIN`
  to that file's directory. Do not add a second copy elsewhere on PATH.
- **D4 Cache.** Clear the shared cache with `golangci-lint cache clean`. Do not delete the directory by hand.
- **D5 No repo workaround.** Do not set `GOTOOLCHAIN` in `go.mod`, do not add a `toolchain` line,
  do not add a lint wrapper script.

## Pre-commit commands (every task, in this order)

1. `gofmt -l .` prints nothing.
2. `go build ./...` exits 0.
3. `go vet ./...` exits 0.
4. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` exits 0.
5. `test ! -e go.sum` exits 0.
6. `git status --short` lists only paths in "Files touched" or this notebook directory.

## Tasks

- [ ] T1 host: rebuild the linter. Run `GOTOOLCHAIN=local GOBIN="$(dirname "$(command -v golangci-lint)")" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2`. Verify: `golangci-lint version | grep -q 'version 2.12.2 built with go1.26'` exits 0 and `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` exits 0.
- [ ] T2 host: clear the shared cache. Run `golangci-lint cache clean`, then `golangci-lint run` twice and record both outputs' last line in PROGRESS.md. Verify: `golangci-lint run` exits 0 and `golangci-lint run 2>&1 | grep -c '^\.\./'` prints `0`.
- [ ] T3 docs: append one line dated 2026-10-29 to `docs/fixes-log.md` in its existing format: the host linter was built with go1.25 and panicked on the go1.26 stdlib; it was rebuilt with go1.26; the shared cache held another tree's issues and was cleared with `golangci-lint cache clean`. Verify: `grep -n 'golangci-lint' docs/fixes-log.md | grep 2026-10-29 | grep -q 'cache clean'` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && golangci-lint run` exits 0.

## Files touched

The loop may create or edit these repo paths and no others (its own notebook
directory `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/` is always in scope):

```allowlist
docs/fixes-log.md
```

Host paths outside the repo that T1 and T2 change, and no others:

- the `golangci-lint` binary that `command -v golangci-lint` names (`/home/nctiggy/bin/golangci-lint` at planning time)
- the shared lint cache, through `golangci-lint cache clean` only

Not in scope, ever, in this milestone: `.golangci.yml`, `.github/`, `go.mod`, `go.sum`,
`AGENTS.md`, `internal/`, `cmd/`, `k8s/`, `specs/`.
