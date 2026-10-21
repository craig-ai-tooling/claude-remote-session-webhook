# Validation contract: shell-fallback prompt refusal (lm-steer-crswd-018-fix-4)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree, from the repository
root.

The behaviour: when Claude has exited and the session's pane has fallen back to the
login shell, `POST /sessions/{id}/prompt` refuses with `409` and body
`{"error":"session not ready"}`, and the prompt text never reaches the shell.

## Behaviour of `POST /sessions/{id}/prompt`

- A prompt to a session whose pane is running a shell instead of the binary it was started with answers `409` with the exact body `{"error":"session not ready"}`, and no paste and no Enter reaches tmux: `go test ./internal/httpapi/ -run Prompt -v` exits 0 and its output lists a passing test whose name contains `NotReady`.
- The same refusal is made by the session layer before any paste, for a shell-fallback pane, while a pane still running its start binary and a session that recorded no expected binary are both delivered as before: `go test ./internal/session/ -run Prompt -v` exits 0 and its output lists a passing test whose name contains `Shell`.
- On a real tmux server, a prompt sent to a session whose start command has exited is refused and the marker text it carried never appears in that pane: `go test -tags tmux ./internal/session/ -run Prompt -v` exits 0 and its output lists a passing test whose name contains `Shell`.
- A refused shell-fallback prompt leaves exactly one audit record, with decision `deny` and reason `session not ready`, and neither that record nor any log line carries the prompt text: `go test ./internal/httpapi/ ./internal/audit/` exits 0.
- The string `session not ready` is now in the Go source: `grep -rl --include=*.go 'session not ready' internal` prints at least one path.
- A prompt to a healthy session still answers `202` with `"delivered":true`: `go test ./internal/httpapi/ -run 'Prompt'` exits 0.

## The acceptance suite still delivers

- The quickstart stand-in for `claude` is seen by tmux as `claude`, so the acceptance prompts are still delivered: on a host where `127.0.0.1:8765` is free and `tmux` and `jq` are installed, `go test -tags quickstart ./cmd/crswd` exits 0. Where the port is held, `go vet -tags quickstart ./cmd/crswd` exits 0 and PROGRESS.md records that the run was skipped and why.

## The contract and the suites

- The API contract documents the refusal: `grep -q 'session not ready' specs/001-crswd-daemon-core/contracts/http-api.md` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- `go test ./...` exits 0.
- `go test -tags tmux ./...` exits 0 on a host with tmux installed.
- `golangci-lint run` exits 0.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
