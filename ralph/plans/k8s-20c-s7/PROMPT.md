# Ralph loop prompt: k8s-20c-s7

You are running autonomously, one iteration at a time, with a fresh context. You have no
memory of previous iterations. Everything you need is on disk.

Milestone: k8s-20c-s7, the last slice of k8s-20c. It wires the cluster build: the
`k8s/cmd/crswd` binary, the image Dockerfiles, the crswd-next manifests and `docs/k8s-mode.md`
(spec 017, FR-002, FR-003, FR-009, FR-021). One PR against `main` when the loop is done.

## Start an iteration

1. Read `AGENTS.md` at the repo root.
2. Read `docs/conventions.md`. You are writing Go.
3. Read `docs/security.md`. This slice opens a second binary and a second way to start.
4. Read `.specify/memory/constitution.md`.
5. Read `ralph/plans/k8s-20c-s7/IMPLEMENTATION_PLAN.md`, including Design and "Files touched".
6. Read `ralph/plans/k8s-20c-s7/PROGRESS.md`. Do not redo finished work.
7. Run the check in the `## Dependency` section of IMPLEMENTATION_PLAN.md. If it fails, apply
   "Blocked work" below.
8. Pick the topmost task marked `- [ ]` in IMPLEMENTATION_PLAN.md. Exactly one.

## Rules

- Do exactly one task per iteration.
- Touch only files in the "Files touched" list.
- Line numbers in the plan may be stale. Find a named function with `grep -n`.
- If a task needs a decision it does not state, do not guess. Follow "When a task is ambiguous".
- Stage each path you changed by name.
- Never stage with `git add -A`.
- Write git commands bare, never `git -C`.
- Never push to `main`, never force-push, never disable a hook.
- Never commit a secret, a real `auth.json`, or a real credential file.
- Never run `cd`. The sandbox refuses it. Use `go -C k8s` for the cluster module.
- Never run `docker`, `kubectl` or `helm`. Image builds and the cluster check are operator-run
  (IMPLEMENTATION_PLAN.md "Operator-run acceptance").

## Blocked work

1. If any line in IMPLEMENTATION_PLAN.md starts with `- [!]`, change no file and stop.
2. If the Dependency check fails, leave every task `- [ ]`.
3. If the last line of PROGRESS.md is already `BLOCKED: <that same reason>`, change no file and stop.
4. Otherwise append `BLOCKED: <reason>` as the last line of PROGRESS.md, commit only that file,
   and stop.

## When a task is ambiguous

1. Append the question to PROGRESS.md under `NEEDS CLARIFICATION`, quoting the task text.
2. Mark the task `- [!]` in IMPLEMENTATION_PLAN.md.
3. Commit those two files.
4. Stop.

## Prove the test fails against the old code

1. Write the task's tests first.
2. Run the task's verify command and confirm the new tests fail.
3. Copy the failing line into PROGRESS.md. A test for a package that does not exist yet fails
   by not compiling; say so.
4. Implement the task.
5. Run the verify command again and confirm it passes.

## Before you commit

1. Run `gofmt -l .` and confirm it prints nothing.
2. Run `go build ./... && go vet ./... && go test ./...` and confirm it exits 0.
3. Run `go test -tags tmux ./...` and confirm it exits 0.
4. Run `go test -tags quickstart ./cmd/crswd` and confirm it exits 0. It needs `127.0.0.1:8765`
   free. If the deployed daemon holds the port, record that in PROGRESS.md.
5. Run `golangci-lint run` and confirm it exits 0.
6. Run `go -C k8s vet ./...` and confirm it exits 0.
7. Run `go -C k8s test ./...` and confirm it exits 0.
8. Run `go -C k8s build ./...` and confirm it exits 0.
9. Run `test ! -e go.sum` and confirm it exits 0.
10. Run `grep -c require go.mod` and confirm it prints `0`.

Do not lint the cluster module yourself: that needs `cd`. CI lints it.

If you cannot make every step pass, revert your change, log why in PROGRESS.md, and stop.

## Finish an iteration

1. Confirm the task's verify command exits 0.
2. Tick the task in IMPLEMENTATION_PLAN.md.
3. Append to PROGRESS.md: what you did, the failing-first line, what the next iteration would
   waste time rediscovering, and any problem you noticed but did not fix.
4. Commit the task's files, the plan and PROGRESS.md in one commit, with a conventional subject
   naming the task, e.g. `feat(k8s): the cluster binary (k8s-20c-s7 T3)`.
5. Stop. Do not start another task.

## Completion

1. When every task is ticked, run each command in VALIDATION_CONTRACT.md.
2. Record each result in PROGRESS.md.
3. Append a line containing exactly `RALPH_COMPLETE` to PROGRESS.md.
4. Commit PROGRESS.md and stop.
