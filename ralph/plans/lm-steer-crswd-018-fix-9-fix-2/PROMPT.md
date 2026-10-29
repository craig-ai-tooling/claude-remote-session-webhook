# Ralph loop prompt: `golangci-lint run` exits 0 (lm-steer-crswd-018-fix-9-fix-2)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/VALIDATION_CONTRACT.md`
- The lint config, read only: `.golangci.yml`
- The CI lint pin, read only: `.github/workflows/ci.yml`

## Start an iteration

1. Read `AGENTS.md`.
2. Read `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/IMPLEMENTATION_PLAN.md`, including "Facts measured" and "Decisions already made".
3. Read `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/PROGRESS.md`.
4. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
5. Take the topmost task marked `- [ ]`. Take exactly one.
6. Do the task.
7. Follow "Finish an iteration".

## Rules

- Change only repo paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Change only the host paths listed under "Files touched" in IMPLEMENTATION_PLAN.md.
- Never edit `.golangci.yml`.
- Never edit anything under `.github/`.
- Never edit `go.mod`.
- Never create `go.sum`.
- Never edit `AGENTS.md`.
- Never edit Go source under `internal/` or `cmd/`.
- Never add a `//nolint` comment.
- Never install a golangci-lint version other than `v2.12.2`.
- Never delete the lint cache directory by hand. Use `golangci-lint cache clean`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Do not redo a task already marked `- [x]`.

## If golangci-lint is not on PATH

T1 and T2 change the host's linter. If `command -v golangci-lint` prints nothing, you are where they cannot be done.

1. Do not install a linter somewhere new.
2. Follow "If the task is unclear or cannot be finished".

## If the shared cache shows `../` paths again

1. Run `golangci-lint cache clean`.
2. Run `golangci-lint run` once more.
3. Record both runs' last line in PROGRESS.md.
4. If `../` paths still appear, follow "If the task is unclear or cannot be finished".

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each repo path you touched.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-9-fix-2/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-9-fix-2/PROGRESS.md`.
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-9-fix-2/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they printed, and anything the next iteration would otherwise rediscover.
4. Run `gofmt -l .` and confirm it prints nothing.
5. Run `go build ./...` and confirm it exits 0.
6. Run `go vet ./...` and confirm it exits 0.
7. Run `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` and confirm it exits 0.
8. Run `test ! -e go.sum` and confirm it exits 0.
9. Run `git status --short` and confirm every changed path is in "Files touched" or this notebook directory.
10. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "<type>(<scope>): <task ID> <summary>" -- <each repo path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again.
11. Stop. Do not start another task.

## If safe-commit cannot be reached

1. Record in PROGRESS.md that safe-commit could not be run, and why.
2. Run `git add <path>` once for each repo path you touched, by name.
3. Run `git commit -m "<type>(<scope>): <task ID> <summary>"`.
4. Stop.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `go test ./...` and confirm it exits 0.
2. Append a line containing exactly the loop's completion sentinel, `RALPH_` followed by `COMPLETE`, to PROGRESS.md.
3. Commit PROGRESS.md with safe-commit as in step 10 of "Finish an iteration".
4. Stop.
