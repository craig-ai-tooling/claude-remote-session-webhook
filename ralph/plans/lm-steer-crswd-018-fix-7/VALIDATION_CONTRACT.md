# Validation contract: Claude exits mid-prompt (lm-steer-crswd-018-fix-7)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018 (run 1014-la1i). The
assertion was "If Claude exits between the paste and the submit, the daemon sends
`C-u`, sends no Enter, and answers `409`", and the evidence was that `"C-u"`
appeared nowhere under `internal/` and no test covered it.

## Behaviour

- When a prompt has been pasted and the session's pane is found running a shell instead of Claude before the submit, the daemon sends the key `C-u` to that pane and never sends `Enter`: `go test ./internal/session/ -run 'Prompt'` exits 0, and `grep -rn '"C-u"' internal/session/` prints at least one line.
- The same case, driven as a signed `POST /sessions/{id}/prompt` through the whole HTTP stack, answers status `409` with the exact body `{"error":"session not ready"}`, and the request's one audit record is a deny: `go test ./internal/httpapi/ -run 'Prompt'` exits 0.
- A prompt to a session whose Claude stays running is still pasted and then submitted with exactly one `Enter`, and still answers `202` with `"delivered":true`: `go test ./internal/session/ ./internal/httpapi/ -run 'Prompt'` exits 0.
- A Codex session's prompt is still a bracketed paste followed by one `Enter`: `go test ./internal/session/ -run 'PromptCodex'` exits 0.
- No error, log line or audit record for the refused prompt carries the prompt text: `go test ./internal/audit/ ./internal/httpapi/` exits 0.
- The fake tmux controller can model a process that exits on the next paste, and its own test proves the pane reads as stopped only after that paste: `go test ./internal/tmuxctl/ -run 'ExitAfterPaste'` exits 0.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- No new package under `internal/` (the seventeen on `main` at `97a0f58`; checked from the files alone, because the verifier's copy has no `.git`): `bash -c 'diff <(find internal -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort) <(printf "internal/%s\n" access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater)'` exits 0 and prints nothing.
- The fix is logged: `grep -q 'C-u' docs/fixes-log.md` exits 0.
