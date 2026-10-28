# Implementation plan: the quickstart suite compiles under the module's go 1.23 floor (lm-steer-crswd-018-fix-9-fix-1)

Filed by `loop/fix-features.py` from a `verify-run.sh` FAIL (run `1022-6xpl`, PR #234,
parent task `lm-steer-crswd-018-fix-9`):

> Assertion: `go vet -tags quickstart ./cmd/crswd` exits 0
> Evidence: exit=1: `cmd/crswd/quickstart_test.go:2599:27: testing.Context requires go1.24 or later (file is go1.23)`. go.mod says `go 1.23.0`.

## Why it failed

`TestQuickstartCodexSession` (starts at line 2539 of `cmd/crswd/quickstart_test.go`)
calls `ctl.List(t.Context())` at line 2599. `testing.T.Context` arrived in Go 1.24.
The module declares `go 1.23.0`, so the type checker rejects the call even on a newer
toolchain. The file carries `//go:build quickstart`, so `go vet ./...` and `go test ./...`
never see it, which is how it landed (commit `072027e`, #202).

## Facts measured at planning time (10/28/26, `5ca1507`)

- `go version` on the planning host: `go1.26.8 linux/amd64`. `go.mod`: `go 1.23.0`, no `toolchain` line.
- `go vet -tags quickstart ./cmd/crswd` exits 1 with exactly the one error above.
- `go vet ./...`, `go vet -tags tmux ./...`, `go vet -tags dev ./...` each exit 0.
- `grep -rn 't.Context()' --include=*.go .` finds one hit in the whole repo: `cmd/crswd/quickstart_test.go:2599`.
- `cmd/crswd/quickstart_test.go` does not import `"context"`. Its import block starts at line 47; stdlib imports are alphabetical (`"bytes"` first, `"crypto/hmac"` second).
- Sibling tests in the same package already use the go1.23 pattern, e.g. `cmd/crswd/executionmode_test.go:182`: `ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)`.
- `time` is already imported in `quickstart_test.go`.
- `gofmt -l cmd/crswd` prints nothing.
- `golangci-lint run` on the planning host walks a sibling `../pr227` tree and panics under `--build-tags quickstart`. It is not a usable gate here. The contract does not rely on it.
- The quickstart suite cannot be run on the planning host: `127.0.0.1:8765` is held by the deployed daemon (see AGENTS.md). `go vet -tags quickstart` is the check.
- Earlier loops in this repo found the loop container's Bash guard refuses `bash -c '...'`, `$(...)` and pipes. Run each command on its own.
- Earlier loops found `safe-commit.sh` refuses `"$PWD"` in the container. PROMPT.md has a fallback.

## Decisions already made (do not reopen them)

- **D1 Fix the test, not the module.** Do not change `go.mod` or `go.sum`. Raising the floor to 1.24 changes what every builder of this module needs; the parent milestone's contract asserts the vet, not a version bump.
- **D2 The replacement.** In `TestQuickstartCodexSession`, replace the single line `infos, err := ctl.List(t.Context())` with these three lines, at the same indentation:

```go
	listCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	infos, err := ctl.List(listCtx)
```

- **D3 The import.** Add `"context"` to the stdlib group of the import block, between `"bytes"` and `"crypto/hmac"`.
- **D4 Scope.** Touch nothing else in `quickstart_test.go`. No other Go file. No `docs/` file except the one line in `docs/fixes-log.md` T1 adds.

## Tasks

- [ ] T1 Apply D2 and D3 to `cmd/crswd/quickstart_test.go`, then append one line to `docs/fixes-log.md` in its existing format, dated 2026-10-28, ending `(lm-steer-crswd-018-fix-9-fix-1)`. Verify: `go vet -tags quickstart ./cmd/crswd` exits 0, `! grep -q 't.Context()' cmd/crswd/quickstart_test.go` exits 0, and `gofmt -l cmd/crswd` prints nothing.
- [ ] F1 Run every assertion in this milestone's VALIDATION_CONTRACT.md, one command at a time, and record each command, its output and its exit code in PROGRESS.md. Change no file outside this notebook. Verify: `go build ./...` exits 0 and `go test ./...` exits 0.

## Files touched

The only paths this milestone may change, besides its own notebook directory:

```allowlist
cmd/crswd/quickstart_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `go.mod`, `go.sum`, every other `.go` file, `internal/`,
`k8s/`, `specs/`, `AGENTS.md`, and every other directory under `ralph/plans/`.

## Pre-commit commands

1. `gofmt -l cmd/crswd` prints nothing.
2. `go vet -tags quickstart ./cmd/crswd` exits 0.
3. `git diff --name-only` prints only paths in "Files touched" or this notebook directory.
