# Ralph loop prompt: concurrent prompts never interleave (lm-steer-crswd-018-fix-11)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-11/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-11/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-11/VALIDATION_CONTRACT.md`
- The rule as specified, read only: `specs/020-prompt-route-safety/spec.md` (FR-014 to FR-016)

## Start an iteration

1. Read `AGENTS.md`.
2. Read `docs/conventions.md`.
3. Read `ralph/plans/lm-steer-crswd-018-fix-11/IMPLEMENTATION_PLAN.md`, including "Facts measured" and "Decisions already made".
4. Read `ralph/plans/lm-steer-crswd-018-fix-11/PROGRESS.md`.
5. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
6. Take the topmost task marked `- [ ]`. Take exactly one.
7. Do the task.
8. If the task is T1 or T2, follow "Prove a new test fails against the old code" while you do it.
9. Follow "Finish an iteration".

## Rules

- Change only paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- This milestone adds tests. Never commit a change to a non-test `.go` file.
- Never edit `AGENTS.md`.
- Never edit anything under `specs/`.
- Never edit anything under `k8s/`.
- Never add a package under `internal/`.
- Never add a dependency.
- Never create `go.sum`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never put prompt text in an error string or a log line.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Session-package tests call `t.Parallel()` and use the tmux fake.
- Tests in `internal/tmuxctl/exec_tmux_test.go` do not call `t.Parallel()`. That file's header says why.
- Do not redo a task already marked `- [x]`.

## Prove a new test fails against the old code

The behaviour already exists, so the "old code" is a temporary mutation named in the plan (H2 for T1, H4 for T2).

1. Write the new test.
2. Run the task's Verify command and confirm it exits 0 on unchanged code.
3. Apply the mutation the plan names for this task.
4. Run the test again (for T2, with `-count=5`).
5. Copy the failure line.
6. Restore the mutated file with `git checkout -- <that file>`.
7. Run `git diff --name-only HEAD -- internal/session/manager.go internal/tmuxctl/exec.go` and confirm it prints nothing.
8. Add a row to the "Failing-before evidence" table in PROGRESS.md: task ID, test name, mutation, failure line. For T2, if five runs never failed, write that as the failure line.
9. Run the task's Verify command again and confirm it exits 0.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as tmux not being installed.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each path you touched.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-11/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-11/PROGRESS.md`.
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-11/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-11/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and their results, and anything the next iteration would otherwise rediscover.
4. Run `gofmt -l .` and confirm it prints nothing.
5. Run `go build ./...` and confirm it exits 0.
6. Run `go vet ./...` and confirm it exits 0.
7. Run `go vet -tags tmux ./...` and confirm it exits 0.
8. Run `go vet -tags quickstart ./cmd/crswd` and confirm it exits 0.
9. Run `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` and confirm it exits 0. If golangci-lint is not installed, write that in PROGRESS.md.
10. Run `test ! -e go.sum` and confirm it exits 0.
11. Run `git diff --name-only HEAD -- internal/session/manager.go internal/tmuxctl/exec.go` and confirm it prints nothing.
12. Run `git status --short` and confirm every changed path is in "Files touched" or this notebook directory.
13. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "<type>(<scope>): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again.
14. Stop. Do not start another task.

## If safe-commit cannot be reached

1. Record in PROGRESS.md that safe-commit could not be run, and why.
2. Run `git add <path>` once for each path you touched, by name.
3. Run `git commit -m "<type>(<scope>): <task ID> <summary>"`.
4. Stop.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `go test ./...` and confirm it exits 0.
2. Append a line containing exactly the loop's completion sentinel, `RALPH_` followed by `COMPLETE`, to PROGRESS.md.
3. Commit PROGRESS.md with safe-commit as in step 13 of "Finish an iteration".
4. Stop.
