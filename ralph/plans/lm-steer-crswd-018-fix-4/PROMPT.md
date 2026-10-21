# Ralph loop prompt: shell-fallback prompt refusal (lm-steer-crswd-018-fix-4)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

The goal: a prompt sent to a session whose pane has fallen back to the login shell is
refused with `409` `{"error":"session not ready"}` and never pasted.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-4/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-4/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-4/VALIDATION_CONTRACT.md`
- The API contract: `specs/001-crswd-daemon-core/contracts/http-api.md`
- The design (outside this repo, read only): `~/code/ai-lawnmower/docs/mcp-session-steering-design.md`, section "Phase 3 detail", requirement 1

## Start an iteration

1. Read `AGENTS.md`.
2. Read `docs/conventions.md`.
3. Read `docs/security.md`.
4. Read `ralph/plans/lm-steer-crswd-018-fix-4/IMPLEMENTATION_PLAN.md`, including "Facts measured while planning", "Decisions" and "Task notes".
5. Read `ralph/plans/lm-steer-crswd-018-fix-4/PROGRESS.md`. Do not redo finished work.
6. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
7. Take the topmost task marked `- [ ]`. Take exactly one.
8. Read that task's entry under "Task notes" in IMPLEMENTATION_PLAN.md, if it has one.
9. Do the task.
10. If the task adds a test, follow "Prove a new test fails against the old code" while you do it.
11. Follow "Finish an iteration".

## Rules

- Change only paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Never edit `AGENTS.md`.
- Never edit anything under `k8s/`.
- Never edit anything under `internal/tmuxctl/`.
- Never add a method to the `Controller` interface.
- Never add a package under `internal/`.
- Never add a dependency.
- Never create `go.sum`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never put prompt text, its length or a hash of it in an error, a log line or an audit record.
- Never add an API route.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Tests are table-driven where there is more than one case.
- Tests outside a `//go:build tmux` file call `t.Parallel()` and use the fake controller.
- Comments explain why, never what.

## Prove a new test fails against the old code

1. Write the test before you change the code it covers.
2. Run the task's Verify command.
3. Confirm the new test fails, and copy its failure line.
4. Append that failure line to PROGRESS.md under "Failing-before evidence", with the task ID.
5. Make the code change.
6. Run the task's Verify command again.
7. Confirm it exits 0.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, and for an outside cause such as tmux missing.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each path you touched.
3. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
4. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
5. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-4/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-4/PROGRESS.md`.
6. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-4/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-4/PROGRESS.md` under "Iterations" what you measured: the task ID, what you changed, each command you ran and its result, and anything the next iteration would otherwise rediscover.
4. Run `gofmt -l .` and confirm it prints nothing.
5. Run `go build ./...` and confirm it exits 0.
6. Run `go vet ./...` and confirm it exits 0.
7. Run `go vet -tags tmux ./...` and confirm it exits 0.
8. Run `go vet -tags quickstart ./cmd/crswd` and confirm it exits 0.
9. Run `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` and confirm it exits 0.
10. Run `test ! -e go.sum` and confirm it exits 0.
11. Run `git status --short` and confirm every changed path is in "Files touched" or this notebook directory.
12. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "<type>(<scope>): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again.
13. Stop. Do not start another task.

## Completion

1. When every task in IMPLEMENTATION_PLAN.md is `- [x]`, run `go test ./... && go test -tags tmux ./...` and confirm it exits 0.
2. Append a line containing exactly `RALPH_COMPLETE` to PROGRESS.md.
3. Commit PROGRESS.md with safe-commit as in step 12 of "Finish an iteration".
4. Stop.
