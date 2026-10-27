# Validation contract: a refused "not ready" prompt carries no prompt text (lm-steer-crswd-018-fix-7-fix-3)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: verify-run 1022-tbfh on `lm-steer-crswd-018-fix-7` (PR #229). The assertion was
"Refused prompt's error/log/audit carries no prompt text". The evidence was that
`go test ./internal/audit/ ./internal/httpapi/` exits 0, but the "not ready" refusal path
does not exist, so nothing tests it.

## Behaviour

- A signed `POST /sessions/{id}/prompt` to a session whose Claude exits after the paste and before the submit answers status `409` with the exact body `{"error":"session not ready"}`, and the request's one audit record is a deny with reason `session not ready`: `go test ./internal/httpapi/ -run Prompt` exits 0, and `grep -rln 'session not ready' internal/httpapi/ | grep -c '_test.go$'` prints a number of at least `1`.
- For that refused prompt, the response body, the response headers, the audit trail and every error the daemon reports carry none of the prompt text: `go test ./internal/httpapi/ -run 'NotReady'` exits 0 and its output names at least one test when run with `-v` (`go test ./internal/httpapi/ -run 'NotReady' -v | grep -c '^--- PASS'` prints at least `1`).
- The whole-daemon leak sweep drives the "not ready" refusal and finds no prompt text in any audit record or log line it produced: `go test ./internal/audit/` exits 0, and `grep -c 'session not ready\|StatusConflict' internal/audit/leak_test.go` prints a number of at least `2`.
- At the session layer the same case clears the pasted line with `C-u`, sends no `Enter`, and returns an error that is "session not ready" and contains none of the prompt text: `go test ./internal/session/ -run Prompt` exits 0, and `grep -rn '"C-u"' internal/session/` prints at least one line.
- A prompt to a session whose Claude stays running is still pasted, submitted with exactly one `Enter`, and answers `202` with `"delivered":true`: `go test ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0.
- A Codex session's prompt is still a bracketed paste followed by one `Enter`: `go test ./internal/session/ -run Codex` exits 0.
- No session-scoped request issues a tmux command that targets another session's window, including the prompt route's liveness read: `go test ./internal/httpapi/ -run AddressesAnothersWindow` exits 0.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0. If it fails only in `internal/sessionpod` with a 5 s timeout, `go test -count=1 -p 1 ./internal/sessionpod/` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
- The fix is logged: `grep -q 'lm-steer-crswd-018-fix-7-fix-3' docs/fixes-log.md` exits 0.
