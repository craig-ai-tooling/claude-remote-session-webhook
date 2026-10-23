# Validation contract: prompt during a revive or continue (lm-steer-crswd-018-fix-10)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018 (run 1014-la1i). The
assertion was "prompt during revive/continue restart claim gives 409 not ready".
The evidence was that `go test -race ./internal/session/ -run Prompt` passes, but
no test or code covers the restart claim or the not-ready reply. A bare pass of
that command therefore proves nothing; the assertions below require named passing
cases in its verbose output.

The rule (spec 020 FR-007, FR-008, FR-014, FR-015; the prompt section of
`specs/001-crswd-daemon-core/contracts/http-api.md`): while a revive or a continue
is restarting a session's process, a prompt to that session is refused, not
queued. Nothing reaches tmux. The API answers `409` with exactly
`{"error":"session not ready"}`, and the request's one audit record is a deny with
reason `session not ready`. The prompt must not end the restart's hold on the
session.

## Behaviour

- A prompt to a session whose restart is in flight is refused as not ready and runs no tmux command: `go test -race ./internal/session/ -run Prompt -v 2>&1 | grep -E -- '--- PASS: TestPromptRefusesWhileARestartIsInFlight'` prints at least 1 line, and `go test -race ./internal/session/ -run Prompt` exits 0.
- A refused prompt leaves the restart's hold in place, and once the restart finishes a prompt to the same session is delivered and leaves no hold behind. Checked by the same test as the previous assertion: `go test -race ./internal/session/ -run 'Prompt' -v 2>&1 | grep -E -- '--- PASS: TestPromptRefusesWhileARestartIsInFlight'` prints at least 1 line.
- Prompts sent to one session at the same moment are each delivered whole. Each one's paste and Enter are adjacent, and no two share a buffer: `go test -race -count=3 ./internal/session/ -run 'Prompt' -v 2>&1 | grep -cE -- '--- PASS: TestConcurrentPromptsNeverInterleave'` prints `3`.
- The API answers a not-ready prompt with `409` and exactly `{"error":"session not ready"}`, and its one audit record is a deny with reason `session not ready`: `go test ./internal/httpapi/ -run 'Prompt' -v 2>&1 | grep -E -- '--- PASS: TestPromptNotReadyAnswers409'` prints at least 1 line.
- Every other prompt answer is unchanged: `go test ./internal/httpapi/ ./internal/session/ ./internal/audit/` exits 0.
- The fix is recorded for operators: `grep -n 'restart claim' docs/fixes-log.md` prints a line dated 2026-10-23 or later that contains `409`.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go test -race ./internal/session/ ./internal/httpapi/` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
