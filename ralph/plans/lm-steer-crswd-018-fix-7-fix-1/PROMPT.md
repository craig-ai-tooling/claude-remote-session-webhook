# Ralph loop prompt: pasted prompt, Claude gone (lm-steer-crswd-018-fix-7-fix-1)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-7-fix-1/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-7-fix-1/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-7-fix-1/VALIDATION_CONTRACT.md`
- The parent milestone's notebook, read only: `ralph/plans/lm-steer-crswd-018-fix-7/PROGRESS.md` ("Iteration 2 — T2, blocked")

## Start an iteration

1. Read `AGENTS.md`.
2. Read `docs/conventions.md`.
3. Read `docs/security.md`.
4. Read `ralph/plans/lm-steer-crswd-018-fix-7-fix-1/IMPLEMENTATION_PLAN.md`, including "Facts measured" and "Decisions already made".
5. Read `ralph/plans/lm-steer-crswd-018-fix-7-fix-1/PROGRESS.md`.
6. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
7. Take the topmost task marked `- [ ]`. Take exactly one.
8. Do the task.
9. If the task is T3 or T4, follow "Prove a new test fails against the old code" while you do it.
10. Follow "Finish an iteration".

## Rules

- Change only paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Never edit `AGENTS.md`.
- Never edit anything under `k8s/`.
- Never edit anything under `specs/`.
- Never edit anything under `internal/tmuxctl/`.
- Never edit the parent's notebook directory `ralph/plans/lm-steer-crswd-018-fix-7/`.
- Never add a method to the `Controller` interface.
- Never add a package under `internal/`.
- Never add a dependency.
- Never create `go.sum`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never put prompt text, its length or a hash of it in an error, a log line or an audit record.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Tests are table-driven where there is more than one case.
- Tests call `t.Parallel()`.
- Tests use the tmux fake, never a real tmux.
- Do not redo a task already marked `- [x]`.
- If `go test ./...` fails only in `internal/sessionpod` with a 5 s timeout, run `go test -count=1 -p 1 ./internal/sessionpod/`. If that exits 0, treat the suite as green and record both results in PROGRESS.md.

## Prove a new test fails against the old code

1. Write the test before you change the code it covers.
2. Run the task's Verify command.
3. Confirm the new test fails, and copy its failure line.
4. Add a row to the "Failing-before evidence" table in PROGRESS.md: task ID, test name, failure line.
5. Make the code change.
6. Run the task's Verify command again.
7. Confirm it exits 0.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes to each path you touched. Use `git checkout -- <path>`; if that is refused, undo the edits by hand.
3. Run `git status --short` and confirm only the two notebook files are changed.
4. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
5. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
6. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-7-fix-1/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-7-fix-1/PROGRESS.md`. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
7. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-7-fix-1/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-7-fix-1/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what each measured, and anything the next iteration would otherwise rediscover.
4. Run `gofmt -l .` and confirm it prints nothing.
5. Run `go build ./...` and confirm it exits 0.
6. Run `go vet ./...` and confirm it exits 0.
7. Run `go vet -tags tmux ./...` and confirm it exits 0.
8. Run `go vet -tags quickstart ./cmd/crswd` and confirm it exits 0.
9. Run `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` and confirm it exits 0. If golangci-lint is not installed, write that in PROGRESS.md.
10. Run `test ! -e go.sum` and confirm it exits 0.
11. Run `git status --short` and confirm every changed path is in "Files touched" or this notebook directory.
12. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "<type>(<scope>): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
13. Stop. Do not start another task.

## If safe-commit cannot be reached

1. Record in PROGRESS.md that safe-commit could not be run, and why.
2. Run `git add <path>` once for each path you touched, by name.
3. Run `git commit -m "<type>(<scope>): <task ID> <summary>"`.
4. Stop.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `go test ./...` and confirm it exits 0.
2. Append a line containing exactly the loop's completion sentinel, `RALPH_` followed by `COMPLETE`, to PROGRESS.md.
3. Commit PROGRESS.md with safe-commit as in step 12 of "Finish an iteration".
4. Stop.
