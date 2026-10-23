# Validation contract: concurrent prompts use distinct buffers and never interleave (lm-steer-crswd-018-fix-11)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018 (run 1014-la1i). The
assertion was "concurrent prompts use distinct buffers and never interleave".
The evidence was that each paste already draws a random buffer name, but no test
sends prompts concurrently, so "never interleave" was unchecked. A bare pass of
`go test ./internal/session/` therefore proves nothing; the assertions below
require named passing cases in verbose output.

The rule (spec 020 FR-014 to FR-016): two or more prompts sent to one session at
the same moment each load their text into a tmux buffer no other delivery uses,
and no delivery's paste or key lands between another prompt's paste and its Enter.

## Behaviour

- Prompts sent to one session from two goroutines at once all succeed, each loads a buffer name no other delivery used, each pastes the buffer it loaded, every prompt's text is delivered exactly once, and every paste is followed directly by its own Enter: `go test -race ./internal/session/ -run 'Prompt' -v 2>&1 | grep -E -- '--- PASS: TestTwoConcurrentPromptsNeverInterleave'` prints at least 1 line, and `go test -race ./internal/session/ -run 'Prompt'` exits 0.
- The ordering guarantee is enforced, not incidental: with the per-session serialisation removed, the concurrent-prompt test fails: `grep -E '^\|.*TestTwoConcurrentPromptsNeverInterleave.*FAIL' ralph/plans/lm-steer-crswd-018-fix-11/PROGRESS.md` prints at least 1 line, a failing-before table row that quotes a real failure line.
- Against a real tmux server, eight pastes into one pane started at the same moment each land whole and exactly once, with no paste reporting an error: `go test -tags tmux ./internal/tmuxctl -run 'Concurrent' -v 2>&1 | grep -E -- '--- PASS: TestTmuxConcurrentPastesLandWhole'` prints 1 line on a host with tmux installed.
- Two pastes in a row into one session never share a buffer name: `go test ./internal/tmuxctl -run 'FreshBuffer' -v 2>&1 | grep -E -- '--- PASS: TestExecPasteUsesAFreshBufferEachCall'` prints 1 line.
- The check is recorded for operators: `grep -n 'interleave' docs/fixes-log.md` prints a line dated 2026-10-23 or later that mentions `lm-steer-crswd-018-fix-11`.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go test -tags tmux ./internal/tmuxctl` exits 0 where tmux is installed.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- No production Go file changed against `main`: `git diff --name-only main -- '*.go' | grep -v '_test.go$'` prints nothing.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
