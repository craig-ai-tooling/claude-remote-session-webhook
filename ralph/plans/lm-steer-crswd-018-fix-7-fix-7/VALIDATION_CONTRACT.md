# Validation contract: the C-u withdrawal is logged (lm-steer-crswd-018-fix-7-fix-7)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: verify-run `1022-tbfh` on PR #229 (parent `lm-steer-crswd-018-fix-7`) failed
"Fix logged: `grep -q 'C-u' docs/fixes-log.md`" because grep found nothing.

A log line is only done if what it records is true. So this contract also checks
the behaviour the line describes.

## Behaviour

- The quick-fix log names the `C-u` withdrawal: `grep -q 'C-u' docs/fixes-log.md` exits 0.
- That entry is a dated one-line entry in the log's own format: `grep -E '^- [0-9]{4}-[0-9]{2}-[0-9]{2} — .*C-u' docs/fixes-log.md` prints at least one line.
- The entry sits in the append-only section, below the "Append below" marker: `awk '/Append below/{m=1} m && /C-u/{f=1} END{exit !f}' docs/fixes-log.md` exits 0.
- No earlier entry was removed: `grep -c '^- 20' docs/fixes-log.md` prints a number of at least `39` (38 dated entries existed at planning time, plus this one).
- The append-only marker is still present: `grep -q 'Append below. Do not edit or reorder existing entries.' docs/fixes-log.md` exits 0.
- The behaviour the entry records exists in the daemon: when Claude exits between the paste and the submit, the daemon sends `C-u` and never `Enter`. `grep -rn '"C-u"' internal/session/` prints at least one line, and `go test ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0.

## The suites and the guardrails

- `go build ./...` exits 0.
- `go vet ./...` exits 0.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
