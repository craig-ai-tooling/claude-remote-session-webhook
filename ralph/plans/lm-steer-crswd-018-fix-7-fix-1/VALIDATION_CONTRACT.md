# Validation contract: pasted prompt, Claude gone (lm-steer-crswd-018-fix-7-fix-1)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: verify-run 1022-tbfh FAIL on `lm-steer-crswd-018-fix-7` (PR #229). The
assertion was "Pasted prompt, Claude gone: sends C-u, no Enter". The evidence was
that `go test ./internal/session/ -run Prompt` exits 0 while
`grep -rn '"C-u"' internal/session/` prints nothing.

## Behaviour

- When a prompt has been pasted and the session's pane is found running a shell instead of Claude before the submit, the daemon sends the key `C-u` to that pane and sends no `Enter`: `go test ./internal/session/ -run Prompt` exits 0, and `grep -rn '"C-u"' internal/session/` prints at least one line.
- The same case, driven as a signed `POST /sessions/{id}/prompt` through the whole HTTP stack, answers status `409` with the exact body `{"error":"session not ready"}`, and the request's one audit record is a deny: `go test ./internal/httpapi/ -run Prompt` exits 0.
- A prompt to a session whose Claude stays running is still pasted, then submitted with exactly one `Enter`, and still answers `202` with `"delivered":true`: `go test ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0.
- A Codex session's prompt is still a bracketed paste followed by one `Enter`, and is not refused as not ready: `go test ./internal/session/ -run Codex` exits 0.
- No session-scoped request issues a tmux command that targets another session's window; a server-wide listing that names no window is the only new call shape admitted: `go test ./internal/httpapi/ -run AddressesAnothersWindow` exits 0.
- No error, log line or audit record for a refused prompt carries the prompt text: `go test ./internal/audit/ ./internal/httpapi/` exits 0.
- The fix is logged: `grep -q 'C-u' docs/fixes-log.md` exits 0.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0. If only `internal/sessionpod` fails with a 5 s timeout, `go test -count=1 -p 1 ./internal/sessionpod/` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API door still has exactly six routes: ``grep -cE '^## `(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md`` prints `6`.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
