# Ralph loop prompt: move the post-merge daemon check out of the black-box contract (lm-steer-crswd-018-fix-17)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-17/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-17/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-17/VALIDATION_CONTRACT.md`
- The three files this milestone edits are listed under "Files touched" in IMPLEMENTATION_PLAN.md.

## Start an iteration

1. Read `AGENTS.md`.
2. Read `ralph/plans/lm-steer-crswd-018-fix-17/IMPLEMENTATION_PLAN.md`, including "Why it failed", "Facts measured" and "Decisions already made".
3. Read `ralph/plans/lm-steer-crswd-018-fix-17/PROGRESS.md`.
4. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
5. Take the topmost task marked `- [ ]`. Take exactly one.
6. Before editing, confirm the line numbers the task names still hold the text "Facts measured" says they hold. If they do not, follow "If the task is unclear or cannot be finished".
7. Do the task.
8. Follow "Finish an iteration".

## Rules

- Change only the paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Copy D2's and D3's replacement lines verbatim. Do not reword them.
- Never edit a line of any file other than the lines the task names.
- Never edit the parent's PROGRESS.md, PROMPT.md or STORY_CLASSES.json.
- Never edit fix-16's IMPLEMENTATION_PLAN.md, PROGRESS.md, PROMPT.md or STORY_CLASSES.json.
- Never edit a `.go` file.
- Never edit `AGENTS.md`.
- Never edit anything under `specs/`, `docs/` or `k8s/`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Do not redo a task already marked `- [x]`.

## How F1 runs a git-free assertion

1. Make a scratch directory under `.ralph-tmp/` in the repository root. It is gitignored.
2. Copy into it only the files the assertion reads, at their real relative paths, with flagless `cp`.
3. Run the assertion's command with the scratch directory as the working directory. If the Bash guard refuses `bash -c`, run each piece as its own command and record that you did.
4. Record the command, its output and its exit code in PROGRESS.md.
5. Remove the scratch files with flagless `rm`, then the directories with `rmdir`, innermost first.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each path you touched.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-17/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-17/PROGRESS.md`. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-17/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-17/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they measured, and anything the next iteration would otherwise rediscover.
4. Run `git diff --name-only` and confirm it prints only paths in "Files touched" or this notebook directory.
5. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
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
