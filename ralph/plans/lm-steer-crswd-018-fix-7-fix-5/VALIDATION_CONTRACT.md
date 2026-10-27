# Validation contract: a lint check that gives the same answer on every host (lm-steer-crswd-018-fix-7-fix-5)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history and no diff runs these against the built tree. Every assertion
says how it is checked. All commands run from the repository root.

Source: a verify-run FAIL on milestone lm-steer-crswd-018-fix-7 (run 1022-tbfh).
The assertion was "`golangci-lint run` exits 0 where installed". The evidence was
exit 1 with 71 issues whose paths pointed into other, deleted checkouts. The tree
itself lints clean. A bare `golangci-lint run` on the verifier's host reads the
shared default cache, which replays findings from other checkouts, and the
installed linter was built with an older Go than the host's `go`, which makes an
uncached run panic.

## Behaviour

- On a host where golangci-lint is installed, the repository's lint command exits 0 and reports no issues, run from a copy of the tree that has no `.git`: `bash .github/scripts/lint.sh` exits 0 and its output contains `0 issues.`
- The lint command never reports findings from another checkout, even when the user's shared golangci-lint cache holds results from deleted checkouts: `bash .github/scripts/lint.sh 2>&1 | grep -c 'no such file or directory'` prints `0`.
- The lint command does not crash when the installed golangci-lint was built with an older Go than the host's `go` (compare `golangci-lint version` with `go version`): `bash .github/scripts/lint.sh 2>&1 | grep -c '^panic:'` prints `0`.
- The lint command leaves no temporary cache directory behind: with `TMPDIR` unset, `ls -d /tmp/tmp.* 2>/dev/null | wc -l` prints the same number before and after one run of `bash .github/scripts/lint.sh`.
- Without golangci-lint on the PATH, the lint command fails loudly instead of passing: `env PATH=/usr/bin:/bin bash .github/scripts/lint.sh` exits `127` and prints `golangci-lint not found`.
- The lint command's own behaviour test passes on a host with no golangci-lint at all: `bash .github/scripts/lint_test.sh` exits 0.
- The milestone contracts that gate on lint name the lint command, so a re-run of any of them gets the same answer: `grep -rn 'golangci-lint run. exits 0' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` prints nothing.
- The parent milestone's contract still has 23 assertions: `grep -c '^- ' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `23`.

## The suites and the guardrails

- `shellcheck -S warning .github/scripts/*.sh` exits 0.
- `go build ./...` exits 0.
- No `go.sum` exists: `test ! -e go.sum` exits 0.
- The fix is logged: `grep -q 'GOLANGCI_LINT_CACHE' docs/fixes-log.md` exits 0.
