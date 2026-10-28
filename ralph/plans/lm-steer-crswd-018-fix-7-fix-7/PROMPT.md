# Ralph loop prompt: the C-u withdrawal is logged (lm-steer-crswd-018-fix-7-fix-7)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-7-fix-7/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-7-fix-7/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-7-fix-7/VALIDATION_CONTRACT.md`
- The one file this milestone edits: `docs/fixes-log.md`

## Start an iteration

1. Read `AGENTS.md`.
2. Read `ralph/plans/lm-steer-crswd-018-fix-7-fix-7/IMPLEMENTATION_PLAN.md`, including "Why it failed", "Facts measured" and "Decisions already made".
3. Read `ralph/plans/lm-steer-crswd-018-fix-7-fix-7/PROGRESS.md`.
4. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
5. Take the topmost task marked `- [ ]`. Take exactly one.
6. Do the task.
7. Follow "Finish an iteration".

## How T1 runs

1. Run `grep -rn '"C-u"' internal/session/`. If it prints nothing, follow "If the task is unclear or cannot be finished" and name D1.
2. Run `go test ./internal/session/ ./internal/httpapi/ -run Prompt`. If it exits non-zero, follow "If the task is unclear or cannot be finished" and name D1.
3. Run `grep -q 'C-u' docs/fixes-log.md`. If it exits 0, make no edit to `docs/fixes-log.md` (D2) and go to "Finish an iteration".
4. Run `grep -rn 'session not ready' internal/httpapi/sessions.go` to decide D4's form of the line.
5. Run `date +%F` for the date.
6. Append D3's line, as D4 shapes it, to the end of `docs/fixes-log.md`, preceded by a newline, because the file does not end in one.
7. Run `tail -n 2 docs/fixes-log.md` and confirm the old last entry is intact and the new line is on its own line.

## Rules

- Change only the path listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Copy D3's line verbatim apart from the date and D4's clause. Do not reword it.
- Never edit, reorder or reflow an existing line of `docs/fixes-log.md`.
- Never write the log line if D1's gate fails.
- Never implement the `C-u` code. Sibling milestones own it.
- Never edit a `.go` file.
- Never edit `AGENTS.md`.
- Never edit anything under `specs/` or `k8s/`.
- Never edit another directory under `ralph/plans/`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Do not redo a task already marked `- [x]`.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as D1's gate failing.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each path you touched. If `git checkout` is refused, undo the edit with the Edit tool and confirm `git diff --stat` prints nothing outside this notebook directory.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear, the commands you ran and what they printed.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-7-fix-7/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-7-fix-7/PROGRESS.md`. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-7-fix-7/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-7-fix-7/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they measured, and anything the next iteration would otherwise rediscover.
4. Run `go build ./...` and confirm it exits 0.
5. Run `git diff --name-only` and confirm it prints only paths in "Files touched" or this notebook directory.
6. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again. If safe-commit cannot be reached, use "If safe-commit cannot be reached".
7. Stop. Do not start another task.

## If safe-commit cannot be reached

1. Record in PROGRESS.md that safe-commit could not be run, and why.
2. Run `git add <path>` once for each path you touched, by name.
3. Run `git commit -m "chore(ralph): <task ID> <summary>"`.
4. Stop.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `go build ./...` and confirm it exits 0.
2. Append a line containing exactly the loop's completion sentinel, `RALPH_` followed by `COMPLETE`, to PROGRESS.md.
3. Commit PROGRESS.md as in step 6 of "Finish an iteration".
4. Stop.
