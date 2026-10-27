# Validation contract: Claude gone mid-prompt answers 409 over HTTP (lm-steer-crswd-018-fix-7-fix-2)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: verify-run 1022-tbfh on `lm-steer-crswd-018-fix-7` (PR #229). The assertion was
"Same case over HTTP: 409 `{"error":"session not ready"}`, deny". The evidence was that
`go test ./internal/httpapi/ -run Prompt` exits 0, but `grep -rn 'session not ready' internal/`
finds nothing and no prompt test covers a 409.

## Behaviour of `POST /sessions/{id}/prompt`

- A signed prompt to a session whose Claude exits after the paste and before the submit answers status `409` with the exact body `{"error":"session not ready"}`, and the request's one audit record is a deny with reason `session not ready`: `go test ./internal/httpapi/ -run Prompt` exits 0.
- The 409 is covered by a test, not only by code: `grep -rln 'session not ready' internal/httpapi/ | grep -c '_test.go$'` prints a number of at least `1`, and `grep -rln 'session not ready' internal/httpapi/ | grep -vc '_test.go$'` prints a number of at least `1`.
- In that same case the daemon clears the pasted line with `C-u` and never sends `Enter`: `go test ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0, and `grep -rn '"C-u"' internal/session/` prints at least one line.
- A prompt to a session whose Claude stays running is still pasted, submitted with exactly one `Enter`, and answers `202` with `"delivered":true`: `go test ./internal/httpapi/ -run 'Prompt'` exits 0.
- A Codex session's prompt is still a bracketed paste followed by one `Enter`: `go test ./internal/session/ -run 'Codex'` exits 0.
- No session-scoped request issues a tmux command that targets another session's window, including the prompt route's new liveness read: `go test ./internal/httpapi/ -run 'AddressesAnothersWindow'` exits 0.
- No error, log line, response body or audit record for the refused prompt carries the prompt text: `go test ./internal/audit/ ./internal/httpapi/` exits 0.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0. If it fails only in `internal/sessionpod` with a 5 s timeout, `go test -count=1 -p 1 ./internal/sessionpod/` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API contract still documents the refusal: `grep -q 'session not ready' specs/001-crswd-daemon-core/contracts/http-api.md` exits 0.
- The fix is logged: `grep -q 'session not ready' docs/fixes-log.md` exits 0.
