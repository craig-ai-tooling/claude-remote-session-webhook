# Validation contract: `request_id` on the prompt route (lm-steer-crswd-018-fix-12)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018 (run 1014-la1i). The
assertion was "request_id echoed in the 202 and in the audit record, bad id gives
400". The audit record can carry the field, but the API door's prompt route does
not accept it: a body with `request_id` is refused today as an unknown field. A
bare pass of `go test ./internal/httpapi/ -run RequestID` proves nothing, because
an older test of the audit plumbing already matches that pattern. The assertions
below require named passing cases in verbose output.

The rule (spec 020 FR-017, FR-018; `specs/001-crswd-daemon-core/contracts/http-api.md`
prompt section): `POST /sessions/{id}/prompt` accepts an optional `request_id` of 1
to 32 characters of `[a-z0-9-]`. A present value outside that rule answers `400`
`{"error":"bad request"}` before any tmux command. A valid value is echoed as
`request_id` in the `202` body and written as `request_id` on the request's one
audit record. An absent value appears in neither.

## Behaviour

- A signed prompt with `request_id` `opp-lease-7`, a 32-character value, or the single character `a` answers `202`, the body's `request_id` equals the value sent, and the request's one audit record carries the same `request_id`: `go test ./internal/httpapi/ -run 'RequestID' -v 2>&1 | grep -cE -- '--- PASS: TestPromptEchoesAValidRequestID/'` prints 3 or more.
- A signed prompt with no `request_id` answers `202` with a body whose only keys are `id` and `delivered`, and its audit record has no `request_id` key: `go test ./internal/httpapi/ -run 'RequestID' -v 2>&1 | grep -cE -- '--- PASS: TestPromptWithoutARequestIDAnswersAsBefore'` prints 1 or more, and `go test ./internal/httpapi/ -run 'TestPromptAnswersTheContractResponse'` exits 0.
- A signed prompt whose `request_id` is 33 characters, holds an uppercase letter, holds `_`, holds a space, or is the empty string answers `400` with exactly `{"error":"bad request"}`, runs no tmux command, and leaves one deny record with no `request_id` key: `go test ./internal/httpapi/ -run 'RequestID' -v 2>&1 | grep -cE -- '--- PASS: TestPromptRefusesAMalformedRequestID/'` prints 5 or more.
- A body carrying any other unknown field next to a valid `request_id` is still refused with `400`: `go test ./internal/httpapi/ -run 'Prompt'` exits 0 and `go test ./internal/httpapi/ -run 'RequestID' -v 2>&1 | grep -cE -- '--- PASS: TestPromptRefusesAMalformedRequestID/another_unknown_field'` prints 1.
- Prompt text still never reaches a response, a log line or an audit record: `go test ./internal/audit/ ./internal/httpapi/` exits 0.
- The fix is recorded for operators: `grep -n 'request_id' docs/fixes-log.md` prints a line dated 2026-10-23 or later.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
