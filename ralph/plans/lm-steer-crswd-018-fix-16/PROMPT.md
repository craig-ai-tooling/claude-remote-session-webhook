# Ralph loop prompt: a git-free "no new package under internal/" check (lm-steer-crswd-018-fix-16)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-16/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-16/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md`
- The one file this milestone edits: `ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md`

## Start an iteration

1. Read `AGENTS.md`.
2. Read `ralph/plans/lm-steer-crswd-018-fix-16/IMPLEMENTATION_PLAN.md`, including "Why it failed", "Facts measured" and "Decisions already made".
3. Read `ralph/plans/lm-steer-crswd-018-fix-16/PROGRESS.md`.
4. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
5. Take the topmost task marked `- [ ]`. Take exactly one.
6. Do the task.
7. Follow "Finish an iteration".

## Rules

- Change only the path listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Copy D1's replacement line verbatim. Do not reword it.
- Never edit any line of the parent contract other than the `git ls-tree` bullet.
- Never edit the parent's IMPLEMENTATION_PLAN.md, PROGRESS.md, PROMPT.md or STORY_CLASSES.json.
- Never edit a `.go` file.
- Never edit `AGENTS.md`.
- Never edit anything under `specs/`, `docs/` or `k8s/`.
- Never add a package under `internal/`.
- Never create `go.sum`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Do not redo a task already marked `- [x]`.

## How F1 runs a git-free assertion

1. Make a scratch directory with `d=$(mktemp -d)`.
2. Copy the tree without `.git`: `tar -C "$PWD" --exclude=.git -cf - . | tar -C "$d" -xf -`.
3. Run the assertion's command with `$d` as the working directory.
4. Record the exit code and output in PROGRESS.md.
5. Remove the scratch directory with `rm -rf "$d"`.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each path you touched.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-16/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-16/PROGRESS.md`.
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-16/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-16/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they measured, and anything the next iteration would otherwise rediscover.
4. Run `git diff --name-only` and confirm it prints only paths in "Files touched" or this notebook directory.
5. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again.
6. Stop. Do not start another task.

## If safe-commit cannot be reached

1. Record in PROGRESS.md that safe-commit could not be run, and why.
2. Run `git add <path>` once for each path you touched, by name.
3. Run `git commit -m "chore(ralph): <task ID> <summary>"`.
4. Stop.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `go test ./...` and confirm it exits 0.
2. Append a line containing exactly the loop's completion sentinel, `RALPH_` followed by `COMPLETE`, to PROGRESS.md.
3. Commit PROGRESS.md with safe-commit as in step 5 of "Finish an iteration".
4. Stop.
