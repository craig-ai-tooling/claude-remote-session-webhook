# Ralph loop prompt — Phase 2 — Codex sign-in

You are running autonomously, one iteration at a time, with a fresh context.
You have no memory of previous iterations. Everything you need is on disk.

## Start an iteration

1. Read `AGENTS.md` at the repo root.
2. Read `.specify/memory/constitution.md`.
3. Read `specs/019-codex-runtime/spec.md`, then `specs/019-codex-runtime/research.md`.
4. Read `ralph/plans/codex-auth/IMPLEMENTATION_PLAN.md`.
5. Read `ralph/plans/codex-auth/PROGRESS.md`. Do not redo finished work.
6. Apply "Blocked work" below. If it tells you to stop, stop.
7. Pick the topmost task marked `- [ ]` in `ralph/plans/codex-auth/IMPLEMENTATION_PLAN.md`.
8. Read that task's full entry in `specs/019-codex-runtime/tasks.md`. It is the contract.
9. Read the `docs/` file the AGENTS.md disclosure table names for the files the task touches.

## Rules

- Do exactly one task per iteration.
- Touch only files named in the task's Files line.
- Touch only paths inside the allowlist in `IMPLEMENTATION_PLAN.md`.
- Line numbers in tasks.md may be stale; find the named function with `grep -n`.
- If the task needs a decision it does not state, do not guess.
- Stage each path you changed by name.
- Never stage with `git add -A`.
- Never push to `main`, never force-push, never disable a hook.
- Never commit a secret, a real `auth.json`, or a real device code.

## Blocked work

1. If any line in `IMPLEMENTATION_PLAN.md` starts with `- [!]`, change no file and stop.
2. Run the check in the `## Dependency` section of `IMPLEMENTATION_PLAN.md`.
3. If that check fails, leave every task `- [ ]`.
4. If the last line of `PROGRESS.md` is already `BLOCKED: <that same reason>`, change no file and stop.
5. Otherwise append `BLOCKED: <reason>` as the last line of `PROGRESS.md`, commit only that file, and stop.
6. Apply steps 3 to 5 also when the picked task's own entry in tasks.md names a precondition that fails.
7. Never skip the topmost open task to work on a later one.
8. The one exception is a task whose tasks.md entry says it "may run before" a named task.

The loop stops only when an iteration exits non-zero, which this session cannot cause. A blocked
notebook therefore repeats these checks each iteration and changes nothing.

## When a task is ambiguous

1. Append the question to `PROGRESS.md` under `NEEDS CLARIFICATION`, quoting the task text.
2. Mark the task `- [!]` in `IMPLEMENTATION_PLAN.md`.
3. Commit those two files.
4. Stop.

## Prove the test fails against the old code

1. Write the task's tests first.
2. Run the task's acceptance command and confirm the new tests fail.
3. Implement the task.
4. Run the acceptance command again and confirm it passes.

## Before you commit

1. Run `gofmt -l .` and confirm it prints nothing.
2. Run `go build ./...`.
3. Run `go vet ./...`.
4. Run `go test ./...`.
5. Run `golangci-lint run`.
6. If the task touched `internal/tmuxctl` or `internal/session`, run `go test -tags tmux ./...`.
7. If the task touched `cmd/crswd` or a typed command line, run `go test -tags quickstart ./cmd/crswd`.
8. If the task touched `k8s/`, run `go -C k8s vet ./...` and then `go -C k8s test ./...`.
9. Run `git status --short` and confirm every changed path is in the allowlist.

## Finish an iteration

1. Confirm every command in "Before you commit" exited 0.
2. Tick the task: change its `- [ ]` to `- [x]` in `ralph/plans/codex-auth/IMPLEMENTATION_PLAN.md`.
3. Append to `ralph/plans/codex-auth/PROGRESS.md`: what you did, what you learned, what is left, and any finding you did not fix.
4. Stage each changed path by name.
5. Commit with one focused message whose subject names the task id.
6. If every task is `- [x]`, append a line containing exactly `RALPH_COMPLETE` to `PROGRESS.md` and commit it.
7. Stop. Do not start another task.
