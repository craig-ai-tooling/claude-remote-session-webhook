# Progress: a git-free "no new package under internal/" check (lm-steer-crswd-018-fix-16)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Baseline (planning, 10/26/26)

- Branch `plan/lm-steer-crswd-018-fix-16`, cut from `main` at `8a30044`.
- `grep -n 'git ls-tree' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints line 44.
- `grep -c '^- ' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `24`.
- D1's command, run in a `.git`-less copy of `internal/`: exit 0, no output. With `internal/zzzprobe` added: exit 1, prints `< internal/zzzprobe`.
- Earlier loops in this repo (fix-9) found `golangci-lint` not installed and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If that holds, record it and use PROMPT.md's fallback.
- Earlier loops found `internal/sessionpod` and `internal/loginrelay` flaky under a loaded `go test ./...`. If either fails, re-run it alone with `go test -count=1 ./internal/sessionpod/ ./internal/loginrelay/` before believing it. This milestone touches no Go.

## Iterations
