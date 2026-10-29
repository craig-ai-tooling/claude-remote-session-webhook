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
