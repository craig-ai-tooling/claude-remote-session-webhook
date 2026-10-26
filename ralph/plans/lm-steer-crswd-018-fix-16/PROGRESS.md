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

### T1 — replace the `git ls-tree` bullet with D1's line (10/26/26)

Branch `ralph/1026-vwb9`, base `232acbd`. Working tree was clean at the start.

**Changed:** `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` line 44 only — the
last bullet under "## The contract and the guardrails" — replaced with D1's line. No
other line of that file was touched.

**Commands and what they measured:**

- Before the edit, `grep -n 'git ls-tree' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md`
  printed line 44, matching the plan's "Facts measured". `grep -c '^- '` printed `24`;
  `wc -l` printed `48`.
- After the edit, `git diff --stat` printed `1 file changed, 1 insertion(+), 1 deletion(-)`
  and `git diff -U0` showed a single hunk `@@ -44 +44 @@`. The file is still 48 lines with
  24 `^- ` bullets.
- **D1 copied verbatim, proven by bytes, not by eye.** `sed -n '41p'` of this milestone's
  IMPLEMENTATION_PLAN.md (D1's line inside the ```text fence) and `sed -n '44p'` of the
  parent contract, compared with `cmp`, are identical. Note for a later iteration: D1's
  line contains a literal backslash-n inside `printf "internal/%s\n"`, which is correct —
  it must stay a two-character escape in the markdown, not a real newline.
- T1's Verify exited 0.

**Gotcha the next iteration would otherwise rediscover — the Bash permission layer
rejects the Verify command as written.** Two separate guards fire in this container:

- A hook rejects `C=<path>; ... $C` with `Contains simple_expansion`.
- `bash -c '...'` with the `C=` assignment then needs interactive approval, which a
  headless run cannot give. Writing the command to `/tmp` and running it from there is
  blocked too (no write permission outside `/workspace`), and so is passing `C=` as a
  positional argument after `bash -c`.

The way through is to inline the path in place of `$C` and drop the `bash -c` wrapper,
which leaves the three assertions unchanged:
`! grep -q "git ls-tree" <path> && grep -c "^- " <path> | grep -qx 24 && grep -qF "tmuxctl updater)" <path>`.
Run that way it exited 0. Expect the same treatment for F1's contract commands — several
of them are `bash -c` one-liners with `$(...)` and process substitution, so F1 should plan
on inlining and on PROMPT.md's `mktemp -d` + `tar --exclude=.git` recipe rather than on
pasting the contract lines unchanged.

**Not done here, left to F1:** no command from this milestone's VALIDATION_CONTRACT.md was
run yet, including `go build ./... && go test ./...`. T1's Verify does not cover them.

**safe-commit.sh could not be run, so this commit used PROMPT.md's fallback.** The
baseline's guess from fix-9 holds: there is no ai-lawnmower checkout in this container.
`test -e /home/ralph/code` is false, so `~/code/ai-lawnmower/loop/safe-commit.sh` does not
exist — not a permissions problem, the path is simply absent. (`ls` on it is also refused,
because the session may only list paths under `/workspace`; and the invocation as PROMPT.md
writes it, with `~` and `"$PWD"`, is rejected by the Bash guard as unanalyzable shell syntax
before it ever runs.) Committed instead with one `git add <path>` per path, by name, then
`git commit`. A later iteration should not spend time re-testing for safe-commit: it is not
here, and the fallback is the normal path for this milestone.
