# Progress: move the post-merge daemon check out of the black-box contract (lm-steer-crswd-018-fix-17)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Baseline (planning, 10/26/26)

- Branch `plan/lm-steer-crswd-018-fix-17`, cut from `main` at `c133170`.
- `wc -l ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `48`; `grep -c '^- '` on it prints `24`.
- `grep -n '"After merge" check' ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` prints line 133.
- Line 16 of `ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` asserts `24`.
- fix-16's notebook found `~/code/ai-lawnmower/loop/safe-commit.sh` absent inside the loop container. If that holds, record it once and use PROMPT.md's fallback.
- fix-16's notebook found the Bash guard refuses `bash -c '...'`, `$(...)`, pipes, and `C=<path>; ... $C`. Inline the paths and run each piece as its own command.
- fix-16's notebook found `.ralph-tmp/` is gitignored and is the scratch location that works in the container.
- Earlier loops found `internal/sessionpod` and `internal/loginrelay` flaky under a loaded `go test ./...`. If either fails, re-run it alone with `go test -count=1 ./internal/sessionpod/ ./internal/loginrelay/` before believing it. This milestone touches no Go.

## Iterations
