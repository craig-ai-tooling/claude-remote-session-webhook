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

### F1 — run every assertion in this milestone's VALIDATION_CONTRACT.md (10/29/26)

No file in the repo was changed by F1 beyond this notebook directory: F1 only runs and records. The
scratch probe lived in `.ralph-tmp/` (gitignored) and was removed; `git status --porcelain` printed
nothing afterwards, so no stray path survived.

Each of the six assertions in `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/VALIDATION_CONTRACT.md`
was first tried exactly as written. Three are plain commands and ran as written. Three use
`bash -c` and were refused by the Bash guard with "This command requires approval" — for those,
PROMPT.md's "How F1 runs a git-free assertion" fallback was used, with paths inlined and each piece
run as its own command. Nothing was worked around: the fallback is what PROMPT.md prescribes.

| # | Assertion (contract line) | Command actually run | Output | Exit |
|---|---|---|---|---|
| 1 | no git in fix-9's package check (13) | `bash -c '! grep -n "git ls-tree" …'` — **refused**, "This command requires approval" | — | — |
| 1 | same, via PROMPT.md step 3 | `grep -c "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` | `0` | 0 |
| 2 | all seventeen packages named (14) | `grep -cF 'access admit … updater' ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` (as written) | `1` | 0 |
| 3 | the tree has exactly those seventeen (15) | `bash -c 'diff <(find …) <(printf …)'` — **refused**, "This command requires approval" | — | — |
| 3 | same, via PROMPT.md step 4 | `find internal -mindepth 1 -maxdepth 1 -type d` | the 17 `internal/<name>` lines, exactly D1's set, no more and no fewer | 0 |
| 4 | the check catches a new package (16) | `bash -c 'd=$(mktemp -d) && cp -r internal …'` — **refused**, "This command requires approval" | — | — |
| 4 | same, via PROMPT.md steps 5–8 | `mkdir -p .ralph-tmp/probe/internal`, then one `mkdir` creating all 18 dirs (the 17 + `zzzprobe`), then `find .ralph-tmp/probe/internal -mindepth 1 -maxdepth 1 -type d` | 18 lines, one of them `.ralph-tmp/probe/internal/zzzprobe` | 0 |
| 4 | cleanup | `rmdir` the 18 dirs, then `.ralph-tmp/probe/internal`, then `.ralph-tmp/probe` | (none) | 0 |
| 5 | every other assertion kept (17) | `grep -c '^- ' …fix-9/VALIDATION_CONTRACT.md` (as written) | `13` | 0 |
| 5 | same, second half (17) | `grep -c 'leading' …fix-9/VALIDATION_CONTRACT.md` (as written) | `3` | 0 |
| 6 | product builds and tests pass (18) | `go build ./...` | (none) | 0 |
| 6 | same, second half (18) | `go test ./...` | all 24 packages `ok` or `[no test files]`; no FAIL | 0 |

F1's Verify command is assertion 6's two halves, and both exit 0. `internal/sessionpod` and
`internal/loginrelay`, the two the baseline flagged as flaky, both passed on the first run
(0.980s and 0.010s), so no re-run was needed.

What assertion 4 does and does not show, for whoever reads this next: the `find` on the probe tree
proves the extra directory is visible to the same `find -mindepth 1 -maxdepth 1 -type d` the
assertion diffs against D1's fixed list, so the diff must report it. It does not re-run `diff`
itself, because the guard refuses process substitution. That is the limit of what the fallback can
show in this container, and it is the fallback PROMPT.md specifies.

Notes for whoever comes next:

- `.ralph-tmp/` works as scratch exactly as the baseline said, and `rmdir` innermost-first leaves no
  trace. One `mkdir` with 18 inlined paths is accepted; no loop or brace expansion is needed.
- The guard's refusal message for every `bash -c` form here was "This command requires approval",
  not the "shell syntax that cannot be statically analyzed" wording T1 saw for `"$PWD"`.
- All tasks in IMPLEMENTATION_PLAN.md are now `- [x]`, so the next step is PROMPT.md's "Completion":
  `go test ./...` (already green above, re-run it there), then the completion sentinel.

`safe-commit.sh` could not be run for F1 either, and was not retried: T1 established both of its
forms are refused in this container (`"$PWD"` as unanalyzable shell syntax, the literal `/workspace`
as needing an approval a headless run cannot give), and the baseline says go straight to the
fallback rather than spend turns on it. Committed with PROMPT.md's "If safe-commit cannot be
reached" fallback: one `git add <path>` per path, by name, then `git commit -m`.

### Completion — every task `- [x]` (10/29/26)

Both tasks in `IMPLEMENTATION_PLAN.md` (T1, F1) are `- [x]` and none is `- [!]`, so this iteration
ran PROMPT.md's "Completion". No repo file was changed beyond this notebook directory.

| Command | Output | Exit |
|---|---|---|
| `go test ./...` | all 24 packages `ok` (most cached) or `[no test files]`; no FAIL | 0 |

`internal/sessionpod` and `internal/loginrelay`, the two the baseline flagged as flaky, were both
`ok` from cache, so no re-run with `-count=1 -p 1` was needed. The working tree held no Go change in
this milestone, which is why the cache was valid.

Committed with PROMPT.md's "If safe-commit cannot be reached" fallback for the third time, for the
reason T1 established and F1 confirmed; safe-commit was not retried.

RALPH_COMPLETE
