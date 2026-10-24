# Ralph loop prompt: every lawnmower payload gives 202 with delivered:true (lm-steer-crswd-018-fix-13)

You run headless, one task per iteration, with a fresh context. You remember nothing
from earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/lm-steer-crswd-018-fix-13/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/lm-steer-crswd-018-fix-13/PROGRESS.md`
- The definition of done: `ralph/plans/lm-steer-crswd-018-fix-13/VALIDATION_CONTRACT.md`
- The fixture source, read only: `ralph/plans/lm-steer-crswd-018/PROGRESS.md`, section "Payload fixtures (for T011)"

## Start an iteration

1. Read `AGENTS.md`.
2. Read `docs/conventions.md`.
3. Read `ralph/plans/lm-steer-crswd-018-fix-13/IMPLEMENTATION_PLAN.md`, including "Facts measured" and "Decisions already made".
4. Read `ralph/plans/lm-steer-crswd-018-fix-13/PROGRESS.md`.
5. If any task in IMPLEMENTATION_PLAN.md is marked `- [!]`, stop. Change nothing and commit nothing.
6. Take the topmost task marked `- [ ]`. Take exactly one.
7. Do the task.
8. If the task is T2, follow "Prove the new test bites" after the test passes.
9. Follow "Finish an iteration".

## Rules

- Change only paths listed under "Files touched" in IMPLEMENTATION_PLAN.md, plus this notebook directory.
- Never edit `AGENTS.md`.
- Never edit anything under `specs/`.
- Never edit anything under `k8s/`.
- Never edit anything under `internal/session/` or `internal/tmuxctl/`.
- Never edit `internal/httpapi/sessions.go` or `internal/httpapi/sessions_test.go`.
- Never edit anything under `ralph/plans/lm-steer-crswd-018/`.
- Never retype a fixture by hand. Copy it with the extraction script in IMPLEMENTATION_PLAN.md.
- Never add a package under `internal/`.
- Never add a dependency.
- Never create `go.sum`.
- Never invent a requirement. The decisions in IMPLEMENTATION_PLAN.md are the requirements.
- Never put prompt text, its length or a hash of it in an error, a log line or an audit record.
- Never stage with `git add -A`.
- Never stage with `git add .`.
- Never push to `main`.
- Never route around a hook.
- Tests call `t.Parallel()`.
- Tests use the tmux fake, never a real tmux.
- Do not redo a task already marked `- [x]`.

## Prove the new test bites

The behaviour already exists, so the new test passes on first run. This procedure shows it reads the files and would catch a refused payload.

1. Run `go test ./internal/httpapi/ -run Lawnmower -v` and confirm 4 `--- PASS` subtest lines.
2. Run `sed -i '1s|^|/|' internal/httpapi/testdata/lawnmower/answer.txt`.
3. Run `go test ./internal/httpapi/ -run Lawnmower`.
4. Confirm it exits non-zero and the `answer` subtest reports status 400. Copy that failure line.
5. Run `git checkout -- internal/httpapi/testdata/lawnmower/answer.txt`.
6. Run `wc -c internal/httpapi/testdata/lawnmower/answer.txt` and confirm it prints 605.
7. Add a row to the "Failing-before evidence" table in PROGRESS.md: task ID, test name, the failure line.

## If the task is unclear or cannot be finished

Use this for an ambiguity in the plan, a fixture size that does not match, a new test that fails against the unchanged handler, and an outside cause such as a precondition not met.

1. Do not guess, and do not move to a later task.
2. Revert your uncommitted changes with `git checkout -- <path>` for each tracked path you touched.
3. Delete each untracked file you created with `rm <path>`, by name.
4. Append a `NEEDS CLARIFICATION` entry to PROGRESS.md saying what is unclear and what you checked.
5. Change the task's box to `- [!]` in IMPLEMENTATION_PLAN.md.
6. Commit those two files with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "chore(ralph): block <task ID>" -- ralph/plans/lm-steer-crswd-018-fix-13/IMPLEMENTATION_PLAN.md ralph/plans/lm-steer-crswd-018-fix-13/PROGRESS.md`.
7. Stop.

## Finish an iteration

1. Run the task's Verify command from IMPLEMENTATION_PLAN.md. It must exit 0. If it does not, fix the work or follow "If the task is unclear or cannot be finished".
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/lm-steer-crswd-018-fix-13/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/lm-steer-crswd-018-fix-13/PROGRESS.md` under "Iterations": the task ID, what you changed, the commands you ran and what they printed, and anything the next iteration would otherwise rediscover.
4. Run `gofmt -l .` and confirm it prints nothing.
5. Run `go build ./...` and confirm it exits 0.
6. Run `go vet ./...` and confirm it exits 0.
7. Run `go vet -tags tmux ./...` and confirm it exits 0.
8. Run `go vet -tags quickstart ./cmd/crswd` and confirm it exits 0.
9. Run `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` and confirm it exits 0. If golangci-lint is not installed, write that in PROGRESS.md.
10. Run `test ! -e go.sum` and confirm it exits 0.
11. Run `git status --short` and confirm every changed path is in "Files touched" or this notebook directory.
12. Commit with `~/code/ai-lawnmower/loop/safe-commit.sh "$PWD" -m "<type>(<scope>): <task ID> <summary>" -- <each path you touched, by name>`. Include IMPLEMENTATION_PLAN.md and PROGRESS.md in the path list. If safe-commit exits non-zero, nothing was committed: fix the cause and run it again.
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
