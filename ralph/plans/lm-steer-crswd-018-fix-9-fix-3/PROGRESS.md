# Progress: a git-free "no new package under internal/" check for fix-9 (lm-steer-crswd-018-fix-9-fix-3)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Baseline (planning, 10/29/26)

- Branch `plan/lm-steer-crswd-018-fix-9-fix-3`, cut from `main` at `5ca1507`.
- `wc -l ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` prints `37`; `grep -c '^- '` on it prints `13`; `grep -c 'leading'` on it prints `3`.
- `grep -n 'git' ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` prints only line 37.
- `find internal -mindepth 1 -maxdepth 1 -type d` prints the seventeen names in D1, nothing else.
- D1's command, run with `bash -c` on the planning host: exit 0, no output. In a scratch copy of `internal/` with `zzzprobe` added: exit 1, output `18d17` / `< internal/zzzprobe`.
- fix-16, fix-17 and fix-7-fix-6 notebooks: the loop container's Bash guard refuses `bash -c '...'`, `$(...)`, pipes and `C=<path>; ... $C`. Inline paths and run each piece as its own command.
- fix-7-fix-6 notebook: a bare `grep` that matches nothing prints no output, which does not show exit 0 from exit 1. Pin the exit code with `! grep -q` or `grep -c`.
- fix-17 and fix-7-fix-6 notebooks: `safe-commit.sh` exists in the container, but `"$PWD"` is refused and the literal `/workspace` needs approval a headless run cannot give. Go straight to PROMPT.md's fallback (`git add <path>` per path, then `git commit -m`).
- Earlier loops found `internal/sessionpod` and `internal/loginrelay` flaky under a loaded `go test ./...`. If either fails, re-run it alone with `go test -count=1 -p 1 ./internal/sessionpod/ ./internal/loginrelay/` before believing it. This milestone touches no Go.

## Iterations

### T1 — replace line 37 of fix-9's VALIDATION_CONTRACT.md (10/29/26)

Precondition checked first: line 37 of `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md`
still began `- No new package under` and still contained `git ls-tree`. It did, so T1 was in scope.

Changed: line 37 only, replaced with D1's line verbatim via the Edit tool (one-line old_string →
one-line new_string, so no other line could move). The trailing newline is intact — `wc -l` still
prints `37`.

Commands run and what they measured:

| Command | Output | Exit |
|---|---|---|
| `! grep -q "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` | (none) | 0 |
| `grep -c '^- ' ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` | `13` | 0 |
| `wc -l ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` | `37 ralph/plans/…` | 0 |
| `grep -c 'leading' ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` | `3` | 0 |
| `git diff --name-only` | `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` | 0 |

Both T1 Verify conditions hold, and the two baseline counts the plan said must not move (`^- ` = 13,
`leading` = 3) are unchanged.

Notes for the next iteration (F1):

- The Edit tool took the `!`-prefixed and backtick-heavy replacement line without complaint; no
  escaping surprises, and the literal `\n` inside `printf "internal/%s\n"` survived as two characters.
- `! grep -q …` and `grep -c …` both ran as plain single commands — the Bash guard did not object to
  the leading `!`. No `bash -c`, `$(...)` or pipe was needed for T1, so nothing was worked around.
- F1 still has to run the contract's own assertions; expect the guard to refuse the `bash -c` forms
  and to need PROMPT.md's "How F1 runs a git-free assertion" fallback.

`safe-commit.sh` could not be run, exactly as the baseline said. Both forms were tried once:
`~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" …` was refused with "Contains shell syntax (string)
that cannot be statically analyzed", and `/home/ralph/code/ai-lawnmower/loop/safe-commit.sh
/workspace …` was refused with "This command requires approval", which a headless run cannot give.
Committed with PROMPT.md's "If safe-commit cannot be reached" fallback instead: one `git add <path>`
per path, by name, then `git commit -m`. Next iteration: go straight to the fallback, do not spend
turns on safe-commit.
