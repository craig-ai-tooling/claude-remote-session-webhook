# Ralph loop prompt: a git-free "no new package under internal/" check for fix-9 (lm-steer-crswd-018-fix-9-fix-3)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/VALIDATION_CONTRACT.md`
- The one file this milestone edits: `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md`

## Start an iteration

1. Read `AGENTS.md`.
2. Read `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/IMPLEMENTATION_PLAN.md`, including "Why it failed", "Facts measured" and "Decisions already made".
3. Read `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/PROGRESS.md`.
4. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
5. Take the topmost task marked `- [ ]`. Take exactly one.
6. If the task is T1, confirm line 37 of `ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` still begins `- No new package under` and contains `git ls-tree`. If it does not, follow "If the task is unclear or cannot be finished".
7. Do the task.
8. Follow "Finish an iteration".

## Rules

- Change only the path listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Copy D1's replacement line verbatim. Do not reword it.
- Never edit any line of fix-9's VALIDATION_CONTRACT.md other than line 37.
- Never edit fix-9's IMPLEMENTATION_PLAN.md, PROGRESS.md, PROMPT.md or STORY_CLASSES.json.
- Never edit anything under `ralph/plans/lm-steer-crswd-018-fix-7/`.
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

1. Try each assertion's command exactly as VALIDATION_CONTRACT.md writes it.
2. If the Bash guard refuses `bash -c`, `$(...)` or a pipe, run each piece as its own command with the paths inlined, and record that you did.
3. For the "no git" assertion run without `bash -c`, run `grep -c "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-9/VALIDATION_CONTRACT.md` and confirm it prints `0`.
4. For the package-list assertion run without `bash -c`, run `find internal -mindepth 1 -maxdepth 1 -type d` and confirm it prints exactly the seventeen `internal/<name>` lines D1 lists, no more and no fewer.
5. For the "catches a new package" assertion run without `bash -c`, create `.ralph-tmp/probe/internal/` in the repository root (it is gitignored).
6. Inside it, make one empty directory per name in D1's list, plus `zzzprobe`, with `mkdir`.
7. Run `find .ralph-tmp/probe/internal -mindepth 1 -maxdepth 1 -type d` and confirm it prints eighteen lines, one of them ending `internal/zzzprobe`.
8. Remove the scratch directories with `rmdir`, innermost first, ending with `.ralph-tmp/probe`.
9. Record each command, its output and its exit code in PROGRESS.md.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each path you touched. If `git checkout` is refused, undo the edit with the Edit tool and confirm `git diff --stat` prints nothing outside this notebook directory.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-9-fix-3/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-9-fix-3/PROGRESS.md`. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-9-fix-3/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they measured, and anything the next iteration would otherwise rediscover.
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
3. Commit PROGRESS.md as in step 5 of "Finish an iteration".
4. Stop.
