# Ralph loop prompt: the quickstart suite compiles under the module's go 1.23 floor (lm-steer-crswd-018-fix-9-fix-1)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-9-fix-1/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-9-fix-1/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-9-fix-1/VALIDATION_CONTRACT.md`
- The Go file this milestone edits: `cmd/crswd/quickstart_test.go`
- The log this milestone appends one line to: `docs/fixes-log.md`

## Start an iteration

1. Read `AGENTS.md`.
2. Read `docs/conventions.md`.
3. Read `ralph/plans/lm-steer-crswd-018-fix-9-fix-1/IMPLEMENTATION_PLAN.md`, including "Why it failed", "Facts measured" and "Decisions already made".
4. Read `ralph/plans/lm-steer-crswd-018-fix-9-fix-1/PROGRESS.md`.
5. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
6. Take the topmost task marked `- [ ]`. Take exactly one.
7. If the task is T1, run `grep -n "t.Context()" cmd/crswd/quickstart_test.go` and confirm it prints exactly one line containing `ctl.List(t.Context())`. If it does not, follow "If the task is unclear or cannot be finished".
8. Do the task.
9. Follow "Finish an iteration".

## Prove the old code fails (T1 only)

1. Before editing, run `go vet -tags quickstart ./cmd/crswd`.
2. Confirm it exits 1 and prints `testing.Context requires go1.24 or later`.
3. Record the command, output and exit code in PROGRESS.md.

## Rules

- Change only the paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Copy D2's three lines verbatim. Do not reword them.
- Never edit `go.mod`.
- Never edit `go.sum`.
- Never edit any `.go` file other than `cmd/crswd/quickstart_test.go`.
- Never edit an existing line of `docs/fixes-log.md`. Only append.
- Never edit `AGENTS.md`.
- Never edit anything under `specs/`, `internal/` or `k8s/`.
- Never edit any other directory under `ralph/plans/`.
- Never run `go test -tags quickstart`. Port 8765 is held by the deployed daemon.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Do not redo a task already marked `- [x]`.
- Run each shell command on its own. The Bash guard refuses `bash -c`, `$(...)` and pipes.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each path you touched. If `git checkout` is refused, undo the edit with the Edit tool and confirm `git diff --stat` prints nothing outside this notebook directory.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-9-fix-1/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-9-fix-1/PROGRESS.md`. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-9-fix-1/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-9-fix-1/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they measured, and anything the next iteration would otherwise rediscover.
4. Run the plan's pre-commit commands, in order:
   1. Run `gofmt -l cmd/crswd` and confirm it prints nothing.
   2. Run `go vet -tags quickstart ./cmd/crswd` and confirm it exits 0.
   3. Run `git diff --name-only` and confirm it prints only paths in "Files touched" or this notebook directory.
5. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "<type>(<scope>): <task ID> <summary>" -- <each path you touched, by name>`. Use `fix(quickstart)` for T1 and `chore(ralph)` for F1. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
6. Stop. Do not start another task.

## If safe-commit cannot be reached

1. Record in PROGRESS.md that safe-commit could not be run, and why.
2. Run `git add <path>` once for each path you touched, by name.
3. Run `git commit -m "<type>(<scope>): <task ID> <summary>"`.
4. Stop.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `go test ./...` and confirm it exits 0.
2. Append a line containing exactly the loop's completion sentinel, `RALPH_` followed by `COMPLETE`, to PROGRESS.md.
3. Commit PROGRESS.md as in step 5 of "Finish an iteration".
4. Stop.
