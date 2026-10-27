# Ralph loop prompt: a lint check that gives the same answer on every host (lm-steer-crswd-018-fix-7-fix-5)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-7-fix-5/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-7-fix-5/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-7-fix-5/VALIDATION_CONTRACT.md`

## Start an iteration

1. Read `AGENTS.md`.
2. Read `ralph/plans/lm-steer-crswd-018-fix-7-fix-5/IMPLEMENTATION_PLAN.md`, including "Why it failed", "Facts measured" and "Decisions already made".
3. Read `ralph/plans/lm-steer-crswd-018-fix-7-fix-5/PROGRESS.md`.
4. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
5. Take the topmost task marked `- [ ]`. Take exactly one.
6. Do the task.
7. If the task is T1, follow "Prove a new test fails against the old code" while you do it.
8. Follow "Finish an iteration".

## Rules

- Change only paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Never edit `AGENTS.md`.
- Never edit `.golangci.yml`.
- Never edit anything under `.github/workflows/`.
- Never edit anything under `k8s/`, `internal/`, `cmd/` or `specs/`.
- Never add a dependency.
- Never create `go.sum`.
- Never install, rebuild or replace golangci-lint on the host.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- In a contract file, change only the one lint bullet D8 names.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- `lint_test.sh` uses stubs, never the real golangci-lint or the real `go`.
- Do not redo a task already marked `- [x]`.

## Prove a new test fails against the old code

1. Write `.github/scripts/lint_test.sh` before `.github/scripts/lint.sh` exists.
2. Run `bash .github/scripts/lint_test.sh`.
3. Confirm it fails, and copy its first `FAIL` line or its error line.
4. Add a row to the "Failing-before evidence" table in PROGRESS.md: task ID, test name, failure line.
5. Write `.github/scripts/lint.sh`.
6. Run `bash .github/scripts/lint_test.sh` again.
7. Confirm it exits 0.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes to each path you touched. Use `git checkout -- <path>`; if the Bash guard refuses it, restore the file with your edit tool and confirm `git diff --stat` prints nothing for that path.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-7-fix-5/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-7-fix-5/PROGRESS.md`.
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-7-fix-5/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-7-fix-5/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they printed, and anything the next iteration would otherwise rediscover.
4. Run `shellcheck -S warning .github/scripts/*.sh` and confirm it exits 0. If shellcheck is not installed, write that in PROGRESS.md.
5. Run `bash .github/scripts/lint_test.sh` and confirm it exits 0. Skip this step only if T1 is not yet ticked.
6. Run `go build ./...` and confirm it exits 0.
7. Run `test ! -e go.sum` and confirm it exits 0.
8. Run `git status --short` and confirm every changed path is in "Files touched" or this notebook directory.
9. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "<type>(<scope>): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again.
10. Stop. Do not start another task.

## If safe-commit cannot be reached

1. Record in PROGRESS.md that safe-commit could not be run, and why.
2. Run `git add <path>` once for each path you touched, by name.
3. Run `git commit -m "<type>(<scope>): <task ID> <summary>"`.
4. Stop.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `bash .github/scripts/lint_test.sh` and confirm it exits 0.
2. Append a line containing exactly the loop's completion sentinel, `RALPH_` followed by `COMPLETE`, to PROGRESS.md.
3. Commit PROGRESS.md with safe-commit as in step 9 of "Finish an iteration".
4. Stop.
