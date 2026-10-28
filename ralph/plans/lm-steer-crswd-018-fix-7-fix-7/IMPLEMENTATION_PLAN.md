# Implementation plan: the C-u withdrawal is logged (lm-steer-crswd-018-fix-7-fix-7)

Filed by `loop/fix-features.py` from a `verify-run.sh` FAIL (run `1022-tbfh`, PR #229,
parent task `lm-steer-crswd-018-fix-7`):

> Assertion: Fix logged: `grep -q 'C-u' docs/fixes-log.md`
> Evidence: grep found nothing in docs/fixes-log.md.

## Why it failed

Fix-7's T4 was to append the log line. Fix-7 stopped at T2 (`- [!]`, NEEDS CLARIFICATION
about `internal/httpapi/isolation_test.go` being outside its allowlist), so T3 and T4
never ran. PR #229 merged only T1 (the `ExitAfterPaste` fake knob) and the block commit.

The `C-u` withdrawal itself is not on `main`. A log line is a record of a fix that
shipped. Writing one for a fix that has not shipped makes the log lie. This milestone
therefore writes the line only once the behaviour is in the tree.

## Facts measured at planning time (10/28/26, main `5ca1507`)

- `grep -c 'C-u' docs/fixes-log.md` prints `0`.
- `grep -rn '"C-u"' internal/` prints nothing. `grep -n 'C-u\|clearLine' internal/session/manager.go` prints nothing.
- `docs/fixes-log.md` is 61 lines, holds 38 lines matching `^- 20`, and does NOT end in a
  newline (its last byte is the `)` of the 2026-10-22 entry ending `(lm-steer-crswd-018-fix-9)`).
  An appended line must start with a newline.
- The log's format, from its header: `- YYYY-MM-DD — What was wrong; what you changed. (#issue-or-PR)`.
  The dash after the date is an em-dash, U+2014, with one space each side. Entries sit below
  the line `<!-- Append below. Do not edit or reorder existing entries. -->`, newest last.
- Three unmerged sibling plan branches own the code for this fix, and each also plans its own
  fixes-log line: `plan/lm-steer-crswd-018-fix-7-fix-1` (session `C-u`, T5 logs `C-u`),
  `plan/lm-steer-crswd-018-fix-7-fix-2` (HTTP 409), `plan/lm-steer-crswd-018-fix-7-fix-3`
  (no prompt text). This milestone does not implement any of their work.
- `~/code/ai-lawnmower/loop/safe-commit.sh` exists on the planning host. Earlier loops in this
  repo (fix-7, fix-16, fix-17 notebooks) found it refused by the loop container's Bash
  permission layer and committed by PROMPT.md's fallback instead.
- `.ralph-tmp/` is gitignored and is the scratch location that works in the loop container.

## Decisions already made (do not reopen them)

- **D1 Gate.** T1 writes nothing to `docs/fixes-log.md` unless `grep -rn '"C-u"' internal/session/`
  prints at least one line AND `go test ./internal/session/ ./internal/httpapi/ -run Prompt`
  exits 0. If either fails, the fix has not landed: block T1 with NEEDS CLARIFICATION naming
  the sibling branches above. Never implement the `C-u` code in this milestone.
- **D2 Already logged.** If `grep -q 'C-u' docs/fixes-log.md` already exits 0 (a sibling
  landed its own line), do not add a second line. Tick T1 and record that in PROGRESS.md.
- **D3 The line.** Append, after a leading newline, exactly one line, with today's date in
  `YYYY-MM-DD` form in place of `DATE`:

```text
- DATE — A prompt whose Claude exited between the paste and the submit was still submitted with `Enter`, into whatever the pane was now running. The daemon now reads liveness once more after the paste; if Claude has stopped it sends `C-u` to clear the line, sends no `Enter`, and the API answers `409` `{"error":"session not ready"}` with an audit deny. (lm-steer-crswd-018-fix-7-fix-7)
```

- **D4 409 clause.** If `grep -rn 'session not ready' internal/httpapi/sessions.go` prints
  nothing, drop the clause `, and the API answers \`409\` \`{"error":"session not ready"}\` with an audit deny`
  from D3's line and end the sentence after `sends no \`Enter\``. Record which form was used.
- **D5 Append only.** Never edit, reorder or reflow an existing line of `docs/fixes-log.md`.

## Pre-commit commands (every task, in this order)

1. `go build ./...` exits 0.
2. `git diff --name-only` prints only paths in "Files touched" or this notebook directory.

## Tasks

- [ ] T1 log: apply D1 (gate), then D2, then D3 with D4, appending one line to `docs/fixes-log.md`. Change no other file outside this notebook. Verify: `grep -q 'C-u' docs/fixes-log.md && go test ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each command, its output and exit code in PROGRESS.md. Verify: `go build ./... && go vet ./...` exits 0 and every contract command recorded in PROGRESS.md passed.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-7-fix-7/` is always in scope):

```allowlist
docs/fixes-log.md
```

Not in scope, ever, in this milestone: any `.go` file, `AGENTS.md`, `specs/`, `k8s/`,
`go.mod`, `go.sum`, and every other `ralph/plans/` directory.
