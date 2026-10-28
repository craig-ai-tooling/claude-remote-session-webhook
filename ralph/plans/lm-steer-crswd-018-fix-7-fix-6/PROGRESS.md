# Progress: a git-free "no new package under internal/" check for fix-7 (lm-steer-crswd-018-fix-7-fix-6)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Baseline (planning, 10/28/26)

- Branch `plan/lm-steer-crswd-018-fix-7-fix-6`, cut from `main` at `97a0f58`.
- `wc -l ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` prints `31`; `grep -c '^- '` on it prints `15`; `grep -c 'C-u'` on it prints `3`.
- `grep -n 'git' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` prints only line 30.
- `ls internal` prints the seventeen names in D1, nothing else.
- D1's command, run with `bash -c` in a `.git`-less copy of `internal/` on the planning host: exit 0, no output. With `internal/zzzprobe` added: exit 1, output `18d17` / `< internal/zzzprobe`.
- fix-16 and fix-17 notebooks: the loop container's Bash guard refuses `bash -c '...'`, `$(...)`, pipes and `C=<path>; ... $C`. Inline paths and run each piece as its own command.
- fix-17 notebook: `safe-commit.sh` exists in the container, but `"$PWD"` is refused and the literal `/workspace` needs approval a headless run cannot give. Go straight to PROMPT.md's fallback (`git add <path>` per path, then `git commit -m`).
- Earlier loops found `internal/sessionpod` and `internal/loginrelay` flaky under a loaded `go test ./...`. If either fails, re-run it alone with `go test -count=1 -p 1 ./internal/sessionpod/ ./internal/loginrelay/` before believing it. This milestone touches no Go.

## Iterations
