# Validation contract: `golangci-lint run` exits 0 (lm-steer-crswd-018-fix-9-fix-2)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these on the host that carries `golangci-lint`.
Every assertion says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018-fix-9 (run 1022-6xpl,
PR #234). The assertion was "`golangci-lint run` exits 0 where installed". The
evidence: with the shared cache it exited 1 with 98 issues, many on other trees'
paths; with a fresh cache it exited 2 on the panic "file requires newer Go version
go1.26 (application built with go1.25)". It never exited 0.

## Behaviour

- The installed linter is the version CI pins: `golangci-lint version` prints `version 2.12.2`.
- The installed linter was built by a Go toolchain at least as new as the local one: `printf '%s\n%s\n' "$(go version | grep -o 'go1\.[0-9]*' | head -1)" "$(golangci-lint version | grep -o 'built with go1\.[0-9]*' | grep -o 'go1\.[0-9]*')" | sort -C -V` exits 0.
- With an empty cache the linter finishes clean: `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` exits 0 and its output contains `0 issues.` and does not contain `panic`.
- With the default shared cache the linter finishes clean: `golangci-lint run` exits 0 and prints `0 issues.`.
- No reported path points outside this tree: `golangci-lint run 2>&1 | grep -c '^\.\./'` prints `0`.
- The cause and the remedy are recorded for operators: `grep -n 'golangci-lint' docs/fixes-log.md` prints a line dated 2026-10-29 or later that mentions `go1.26` and `cache clean`.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go vet ./...` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `.golangci.yml` and the CI lint pin are unchanged: `git diff --quiet main -- .golangci.yml .github/workflows/ci.yml go.mod` exits 0.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
