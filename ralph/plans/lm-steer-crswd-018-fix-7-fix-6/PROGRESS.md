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

### T1 — replaced line 30 of fix-7's VALIDATION_CONTRACT.md (10/28/26)

Precondition checked first: line 30 of `ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md`
still began `- No new package under` and still contained `git ls-tree`. It did, so T1 went ahead.

Change: one Edit, replacing line 30 with D1's line verbatim. No other line touched.

Commands run and what they measured:

- `! grep -q "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → no output, exit 0.
  `git ls-tree` is gone from the file.
- `grep -c '^- ' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → `15`, exit 0. Matches the
  baseline, so one bullet replaced one bullet.
- `wc -l ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → `31`, unchanged.
- `grep -c 'C-u' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → `3`, unchanged.
- `grep -n 'git' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → only line 30, and only as
  the words `.git` inside D1's parenthetical. No `git` command remains in the file.
- `git diff --name-only` → only `ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md`.
- `git diff --stat` on that file → `1 insertion(+), 1 deletion(-)`.

Notes for the next iteration:

- The literal `\n` inside D1's `printf "internal/%s\n"` survived the Edit tool intact — `grep -n 'git'`
  printed it back as a backslash followed by `n`. No escaping workaround was needed. (The archived
  `\uXXXX` warning in `ralph/archive/progress-milestones-1-3.md` is about the Write tool and does not
  apply here.)
- Bash guard behaviour seen this iteration: `! grep -q ...` ran fine as its own command, and so did a
  single command with two `;`-separated `grep`/`wc` calls. What was refused was `; echo "exit=$?"`
  appended to a command — `$?` needs approval. Read exit codes from the tool result, not from `echo`.
- safe-commit could not be run, exactly as the baseline predicted. `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" ...`
  was refused with "Contains shell syntax (string) that cannot be statically analyzed", and the same command
  with the literal `/workspace` in place of `"$PWD"` was refused with "This command requires approval" — which
  a headless run cannot grant. Committed with PROMPT.md's fallback instead: one `git add <path>` per path, then
  `git commit -m`. A future iteration should go straight to the fallback and not spend two calls on safe-commit.
- T1's edit is done and committed. F1 is next: run this milestone's VALIDATION_CONTRACT.md assertions
  per PROMPT.md's "How F1 runs a git-free assertion" (expect `bash -c`, `$(...)` and pipes to be
  refused, so inline the paths and run each piece separately), then `go build ./...` and `go test ./...`.
