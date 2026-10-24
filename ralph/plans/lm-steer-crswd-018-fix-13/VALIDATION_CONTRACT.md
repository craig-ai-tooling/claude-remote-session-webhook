# Validation contract: every lawnmower payload gives 202 with delivered:true (lm-steer-crswd-018-fix-13)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018 (run 1014-la1i). The
assertion was "every lawnmower payload gives 202 with delivered:true". The
evidence was that `go test ./internal/httpapi/ -run Lawnmower` printed
`[no tests to run]` and exited 0. A bare exit 0 of that command therefore proves
nothing; the assertions below require named passing cases in its verbose output.

The four payload shapes are the prompts lawnmower sends to `POST /sessions/{id}/prompt`
today: a board answer, a requeue resume, an opp-lease batch handoff and an opp-lease
follow-up. Each is multi-line and carries some of: double quotes, `$`, `;`, `&`, a
backtick, and a later line that starts with `/`.

## Behaviour

- Each of the four payloads, posted as a signed, credentialled prompt, answers `202` with `"delivered":true`: `go test ./internal/httpapi/ -run Lawnmower -v 2>&1 | grep -cE -- '--- PASS: TestLawnmower[A-Za-z]*/(answer|resume|opp-lease-batch|opp-followup) '` prints `4`.
- The run is real, not empty: `go test ./internal/httpapi/ -run Lawnmower -v 2>&1 | grep -c 'no tests to run'` prints `0`, and `go test ./internal/httpapi/ -run Lawnmower` exits 0.
- The payloads exist as data beside the tests, one file per shape, at the sizes lawnmower's builders produce: `wc -c internal/httpapi/testdata/lawnmower/*.txt` prints `605` for `answer.txt`, `461` for `opp-followup.txt`, `730` for `opp-lease-batch.txt` and `622` for `resume.txt`, and no other file.
- The hostile shapes survive in the data: `grep -c '^/compact' internal/httpapi/testdata/lawnmower/resume.txt` prints `1`, `grep -c '\$0 cost' internal/httpapi/testdata/lawnmower/answer.txt` prints `1`, and `grep -c 'du -sh' internal/httpapi/testdata/lawnmower/opp-followup.txt` prints `1`.
- The check reads the files and would catch a payload the route refuses. Run `sed -i '1s|^|/|' internal/httpapi/testdata/lawnmower/answer.txt`, then `go test ./internal/httpapi/ -run Lawnmower`: it exits non-zero. Then run `git checkout -- internal/httpapi/testdata/lawnmower/answer.txt` and confirm `git status --short internal/httpapi/testdata/` prints nothing.
- The prompt text never reaches an audit record or a log while these payloads are delivered: `go test ./internal/audit/ ./internal/httpapi/` exits 0.
- The fix is recorded for operators: `grep -n 'lawnmower' docs/fixes-log.md` prints a line dated 2026-10-24 or later that ends with `(lm-steer-crswd-018-fix-13)`.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go test ./...` exits 0.
- `go vet -tags tmux ./...` exits 0.
- `go vet -tags quickstart ./cmd/crswd` exits 0.
- `golangci-lint run` exits 0 where golangci-lint is installed.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The API door still has exactly six routes: `grep -cE '^## \`(GET|POST|DELETE|PATCH|PUT) ' specs/001-crswd-daemon-core/contracts/http-api.md` prints `6`.
- No new package under `internal/`: `bash -c 'diff <(git ls-tree -d --name-only main internal/) <(git ls-tree -d --name-only HEAD internal/)'` exits 0 and prints nothing.
