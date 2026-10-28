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

### F1 — ran every assertion in this milestone's VALIDATION_CONTRACT.md (10/28/26)

No file changed outside this notebook directory. F1 is a verification task.

`bash -c` was refused on the first try, exactly as the baseline predicted
(`bash -c '! grep -n "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md'`
→ "This command requires approval"). Every assertion written with `bash -c` was therefore run
piece by piece with the paths inlined, per PROMPT.md's "How F1 runs a git-free assertion".

Assertion 1 — fix-7's contract no longer depends on git for the package check:

- `grep -n "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → no output.
- `! grep -q "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → no output, exit 0.
- `grep -c "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → `0`, exit 0.
  Three forms, same answer: no `git ls-tree` left in the file. PASS.

Assertion 2 — the contract names all seventeen packages. Ran as written, no `bash -c` needed:

- `grep -cF 'access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md`
  → `1`, exit 0. PASS.

Assertion 3 — the tree has exactly those seventeen top-level packages, checked with no git. Run as
PROMPT.md step 3 says, `find` alone instead of the `diff <(...) <(...)` the contract writes:

- `find internal -mindepth 1 -maxdepth 1 -type d` → exit 0, seventeen lines and no more:
  `internal/` + `buildinfo codexauth quota audit claudeauth updater loginrelay httpapi session auth
  admit access tmuxctl sessionpod harness release config`. `find` prints in directory order, not
  sorted; checked against D1's list by name, and the two sets match exactly — seventeen in, seventeen
  out, no extras, none missing. (This is why the contract line pipes through `LC_ALL=C sort` before
  `diff`.) PASS.

Assertion 4 — the same check catches a new package. Run as PROMPT.md steps 4–7, in
`.ralph-tmp/probe/internal/` rather than the contract's `mktemp -d` copy:

- `grep -n 'ralph-tmp' /workspace/.gitignore` → `36:.ralph-tmp/`, exit 0. Scratch location is
  ignored, so it cannot pollute the commit. (`git check-ignore -v .ralph-tmp` was refused by the
  guard — "requires approval" — so the `.gitignore` grep stands in for it.)
- `mkdir -p .ralph-tmp/probe/internal` → no output, exit 0.
- one `mkdir` with eighteen inlined paths — the seventeen D1 names plus `zzzprobe`, all empty
  directories → no output, exit 0.
- `find .ralph-tmp/probe/internal -mindepth 1 -maxdepth 1 -type d` → exit 0, **eighteen** lines, one
  of them `.ralph-tmp/probe/internal/zzzprobe`. The extra package is visible to the file-only check,
  which is the half of the assertion that matters: `diff` against D1's seventeen-name list can only
  report it. PASS.
- Cleanup, innermost first: `rmdir` of the eighteen directories, then `rmdir .ralph-tmp/probe/internal`,
  then `rmdir .ralph-tmp/probe`. All three → no output, exit 0. `rmdir` refuses a non-empty directory,
  so its exiting 0 is itself proof nothing was left behind.

Assertion 5 — the contract kept every other assertion:

- `grep -c '^- ' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → `15`, exit 0.
- `grep -c 'C-u' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` → `3`, exit 0.
  Both match the baseline. PASS.

Assertion 6, which is also F1's Verify command. Run as two commands, because `&&` plus two
build steps is cheaper to read separately and the contract's `&&` adds nothing here:

- `go build ./...` → no output, exit 0.
- `go test ./...` → exit 0. All 21 packages `ok`, two (`deploy/k8s/gen`, `web`) with no test files.
  The flaky pair the baseline warned about both passed first time: `internal/sessionpod` 1.388s,
  `internal/loginrelay` 0.019s. No re-run needed.

Every assertion in this milestone's VALIDATION_CONTRACT.md passes.

Notes for the next iteration:

- What the guard refused this iteration: `bash -c '...'` and `git check-ignore -v .ralph-tmp`, both
  with "This command requires approval". What ran fine: bare `grep`/`find`/`mkdir`/`rmdir`/`go` with
  every path inlined, including one `mkdir` and one `rmdir` carrying eighteen path arguments. A long
  inlined argument list is not what the guard objects to — shell syntax it cannot statically analyse is.
- A bare `grep` that matches nothing reports "Bash completed with no output", which does not by itself
  distinguish exit 0 from exit 1. Where the exit code was the thing being asserted, it was pinned down
  with a second form (`! grep -q`, or `grep -c` printing a number). Worth repeating rather than
  assuming.
- safe-commit was not attempted this iteration. T1's entry above records both of its forms being
  refused (`"$PWD"` as unanalysable shell syntax, the literal `/workspace` as needing approval a
  headless run cannot grant), and left the instruction to go straight to PROMPT.md's fallback. Did
  that: one `git add <path>` per path, then `git commit -m`.
- Both tasks (T1, F1) are now `- [x]`. Per PROMPT.md "Completion", the next step is the completion
  sentinel: `go test ./...` has just been confirmed exit 0 above, so append the sentinel line to this
  notebook and commit it. No further assertion work remains.
