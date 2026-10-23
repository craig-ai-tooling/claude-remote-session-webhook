# Validation contract: leading `!` or `/` on the prompt route (lm-steer-crswd-018-fix-9)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018 (run 1014-la1i). The
assertion was "prompt starting with `!` or `/` gives 400, type route still
accepts". The evidence was that no non-test code checked the first character,
and that `go test ./internal/httpapi/ -run 'Prompt|Type'` passed only on tests
that predate the rule. A bare pass of that command therefore proves nothing; the
assertions below require named passing cases in its verbose output.

The rule (spec 020 FR-012, FR-013; `specs/001-crswd-daemon-core/contracts/http-api.md`
prompt section): on the API door's `POST /sessions/{id}/prompt`, find the first
character of `text` that is not space, tab, CR or LF. If it is `!` or `/`, answer
`400` with the body `{"error":"bad request"}` before any tmux command runs. The
browser's `POST /dashboard/sessions/{id}/type` route is not on this rule.

## Behaviour

- A signed prompt whose text is `/status`, `!ls`, or either of those after leading spaces, tabs, CR or LF answers `400` with exactly `{"error":"bad request"}`, runs no tmux command, and leaves one audit record whose decision is deny: `go test ./internal/httpapi/ -run 'Prompt' -v 2>&1 | grep -E -- '--- PASS: TestPromptRefusesALeadingCommandCharacter/'` prints at least 4 lines, and `go test ./internal/httpapi/ -run 'Prompt'` exits 0.
- A signed prompt with `!` or `/` only after its first non-whitespace character (`fix a/b` then a newline then `/compact`, and `say hi!`) is still delivered byte-for-byte and answers `202`: `go test ./internal/httpapi/ -run 'Prompt' -v 2>&1 | grep -E -- '--- PASS: TestAPromptIsDeliveredByteForByte/a_later_(slash|bang)'` prints at least 2 lines.
- The session page's type route still delivers `/compact` to the pane and answers `204`: `go test ./internal/httpapi/ -run 'Type' -v 2>&1 | grep -E -- '--- PASS: TestTypeStillDeliversASlashCommand'` prints at least 1 line.
- The refusal's audit record and response never carry the prompt text: `go test ./internal/audit/ ./internal/httpapi/` exits 0.
- The fix is recorded for operators: `grep -n 'leading' docs/fixes-log.md` prints a line dated 2026-10-22 or later that mentions both `!` and `/`.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
