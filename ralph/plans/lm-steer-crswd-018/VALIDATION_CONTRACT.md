# Validation contract: prompt route safety (lm-steer-crswd-018)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

The design calls this "crswd spec 018". This repo already has
`specs/018-interactive-input` and `specs/019-codex-runtime`, so the spec lives at
`specs/020-prompt-route-safety`.

## The spec

- The spec, plan and task list exist: `ls specs/020-prompt-route-safety/spec.md specs/020-prompt-route-safety/plan.md specs/020-prompt-route-safety/tasks.md` exits 0.
- The spec carries no open question: `! grep -rq "NEEDS CLARIFICATION" specs/020-prompt-route-safety` exits 0.

## The suites

- `go test ./...` exits 0.
- `go test -tags tmux ./...` exits 0 on a host with tmux installed.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0.

## Behaviour of `POST /sessions/{id}/prompt`

- A prompt to a session whose pane has fallen back to a shell answers `409` `{"error":"session not ready"}` and no paste reaches the pane: `go test ./internal/... -run 'Prompt'` exits 0.
- A prompt to a real tmux pane that has fallen back to a shell is refused and the pane reads back empty: `go test -tags tmux ./internal/session/ -run 'Prompt'` exits 0.
- A prompt to a session in state `failed` answers `409` `{"error":"session not ready"}` and runs no tmux command: `go test ./internal/... -run 'Prompt'` exits 0.
- If Claude exits between the paste and the submit, the daemon sends `C-u`, sends no Enter, and answers `409`: `go test ./internal/session/ -run 'Prompt'` exits 0.
- A two-line prompt to a Claude session arrives as one bracketed paste followed by exactly one Enter: `go test ./internal/session/ -run 'Prompt'` and `go test -tags tmux ./internal/session/ -run 'Prompt'` both exit 0.
- A prompt whose first non-whitespace character is `!` or `/` answers `400` and runs no tmux command, while the browser type route still accepts such text: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0.
- A prompt sent while a revive or continue holds the session's restart claim answers `409` `{"error":"session not ready"}`: `go test -race ./internal/session/ -run 'Prompt'` exits 0.
- Two prompts sent to one session at the same moment use distinct buffer names and never interleave paste and submit: `go test -race ./internal/session/ -run 'Prompt'` exits 0.
- A prompt carrying `"request_id":"abc-123"` answers `202` echoing it, the audit record carries it, and a 33-character or uppercase `request_id` answers `400`: `go test ./internal/httpapi/ -run 'RequestID'` exits 0.
- No audit record or log line for a prompt carries the prompt text, its length or a hash of it: `go test ./internal/audit/ ./internal/httpapi/` exits 0.
- Every payload shape lawnmower sends today (board answer, requeue resume, opp-lease batch handoff, opp-lease follow-up) answers `202` with `"delivered":true`: `go test ./internal/httpapi/ -run 'Lawnmower'` exits 0.

## The contract and the guardrails

- The API contract documents the not-ready refusal: `grep -q 'session not ready' specs/001-crswd-daemon-core/contracts/http-api.md` exits 0.
- The API contract documents the new field: `grep -q 'request_id' specs/001-crswd-daemon-core/contracts/http-api.md` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- The real router sweep still passes: `go test ./internal/httpapi/` exits 0.
- AGENTS.md stays under the CI limit: `bash -c 'test "$(wc -l < AGENTS.md)" -lt 150'` exits 0.
- No new package under `internal/` (the seventeen on `main` at `8a30044`; checked from the files alone, because the verifier's copy has no `.git`): `bash -c 'diff <(find internal -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort) <(printf "internal/%s\n" access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater)'` exits 0 and prints nothing.
