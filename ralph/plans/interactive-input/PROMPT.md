# Ralph loop prompt — interactive input (spec 018)

You are running autonomously, one iteration at a time, with a fresh context.
You have no memory of earlier iterations. Everything you need is on disk.

## Files you work from

- The plan: `ralph/plans/interactive-input/IMPLEMENTATION_PLAN.md`
- The notebook: `ralph/plans/interactive-input/PROGRESS.md`
- The task details: `specs/018-interactive-input/tasks.md`
- The why: `specs/018-interactive-input/spec.md`
- The evidence: `specs/018-interactive-input/research.md`

## Do exactly this, in order

1. Read `AGENTS.md`.
2. Read `.specify/memory/constitution.md`.
3. Read `docs/conventions.md`.
4. Read `docs/security.md`.
5. Read `ralph/plans/interactive-input/IMPLEMENTATION_PLAN.md`.
6. Read `ralph/plans/interactive-input/PROGRESS.md`. Do not redo finished work.
7. If any task in the plan is marked `- [!]`, the plan is blocked. Make no change, commit nothing, and exit. Do not skip past it.
8. Pick the topmost task marked `- [ ]`. Pick exactly one. Never pick a task below an open one.
9. Read that task's full entry in `specs/018-interactive-input/tasks.md`.
10. If the task touches `web/`, read `docs/components.md` and `docs/design-system.md`.
11. Implement the task as its entry specifies.
12. Run the checks in "Before you commit".
13. Commit.
14. Run the steps in "Finish an iteration".

## Rules

- Change only files in the plan's "Files touched (allowlist)" section.
- Change only the files the task's own entry names.
- Do not invent a requirement.
- Do not refactor code the task does not name.
- Do not edit `Manager.Prompt`, `Manager.Compact` or `Paste`.
- Do not add a dependency. `go.sum` must not exist.
- Do not stage with `git add -A`.
- Do not stage with `git add .` either.
- Never push to `main`.
- Never disable or route around a hook.

## If the task is ambiguous or cannot be completed

Use this for an ambiguity, and also for an external reason the task cannot
finish (a tool missing, tmux absent for a `-tags tmux` test, a precondition
not met).

1. Do not guess, and do not skip to a later task.
2. Append a `NEEDS CLARIFICATION` entry to `ralph/plans/interactive-input/PROGRESS.md` saying what is unclear.
3. Change the task's box to `- [!]` in the plan.
4. Commit those two files.
5. Exit.

## Prove a guard by breaking it

1. Comment out or alter the line the guard depends on.
2. Run the test the task names.
3. Confirm the test fails.
4. Restore the line.
5. Run the test again and confirm it passes.
6. Write the test name and both results in PROGRESS.md.

## Before you commit

1. Run `gofmt -l .` and confirm it prints nothing.
2. Run `go build ./...`.
3. Run `go vet ./...`.
4. Run `go vet -tags tmux ./...`.
5. Run `go vet -tags quickstart ./cmd/crswd`.
6. Run `go test ./...`.
7. Run `golangci-lint --version` and confirm it is 2.x.
8. Run `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run`.
9. If the task touched `internal/tmuxctl`, run `go test -tags tmux ./internal/tmuxctl/...`.
10. Run `test ! -e go.sum`.
11. Run `git status --short` and confirm every changed path is in the allowlist.
12. Stage each changed path by name with `git add <path>`.

If any step fails, fix it. If you cannot fix it, revert your change with
`git checkout -- <path>` for each path, log why in PROGRESS.md, and exit.

## Commit

One focused commit. The subject is imperative and names the task ID, for
example `feat(tmuxctl): T002 add PasteBracketed`. The body says why.

## Finish an iteration

1. Confirm every step in "Before you commit" exited 0.
2. Change the task's `- [ ]` to `- [x]` in `ralph/plans/interactive-input/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/interactive-input/PROGRESS.md`: the task ID, what you did in one or two lines, and anything the next iteration would waste time rediscovering.
4. Append any problem you noticed but did not fix to PROGRESS.md under `Findings`.
5. Stage the plan and the notebook by name.
6. Commit them with subject `chore(ralph): tick <task ID>`.
7. Exit. Do not start another task.

## Completion

When every task in the plan is `- [x]` and the tree is green, append a line
containing exactly `RALPH_COMPLETE` to `ralph/plans/interactive-input/PROGRESS.md`,
commit it, and exit.
