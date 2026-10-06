# Ralph loop prompt: k8s-20c-s2

You are running autonomously, one iteration at a time, with a fresh context. You have no
memory of previous iterations. Everything you need is on disk.

Milestone: k8s-20c-s2, the AgentSession API, admission, JSON manifests and RBAC tests (spec 017,
FR-005, FR-007, FR-013, SC-004, SC-005). One PR against `main` when the loop is done.

## Do exactly this, in order

1. **Read the contract.** `AGENTS.md` at the repo root, then `docs/conventions.md` (you are
   writing Go) and `docs/security.md` (this slice is authz: RBAC and admission). Also
   `.specify/memory/constitution.md`.
2. **Read the plan.** `ralph/plans/k8s-20c-s2/IMPLEMENTATION_PLAN.md`, including its Design
   section and its "Files touched" list. For background only:
   `specs/017-k8s-native-execution/k8s-20c-plan.md` section 2 "S2".
3. **Read the notebook.** `ralph/plans/k8s-20c-s2/PROGRESS.md`. It is what past iterations did.
   Do not redo finished work.
4. **Pick exactly ONE task**: the topmost unchecked `- [ ]` item in IMPLEMENTATION_PLAN.md. One.
5. **Write the test first and watch it fail.** Every new test must fail against the code as it
   was before your change. Run it before implementing and copy the failing line into PROGRESS.md.
   A test for a package that does not exist yet fails by not compiling; say so. Never write a
   test that reads `origin/main` or any git ref.
6. **Implement it**, inside the "Files touched" list only. If the task is ambiguous, do not
   guess: write it under `NEEDS CLARIFICATION` in PROGRESS.md, leave the task unticked with a
   `BLOCKED:` note, and exit.
7. **Run the gate.** All of these must pass:
   `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./... && go test -tags quickstart ./cmd/crswd && golangci-lint run`
   Also `test ! -e go.sum` and `grep -c require go.mod` printing `0`. The quickstart suite needs
   `127.0.0.1:8765` free; if the deployed daemon holds it, record that in PROGRESS.md rather than
   skipping silently. If you cannot make the gate pass, revert your change and log why. Never
   commit a broken tree.
8. **Commit.** Stage only the paths you touched (never `git add -A`; other sessions share this
   tree). One focused commit, conventional subject naming the task, e.g.
   `feat(admit): pure admission for AgentSession objects (k8s-20c-s2 T4)`. Write git commands
   bare, never `git -C`.
9. **Update the notebook.** Append to `ralph/plans/k8s-20c-s2/PROGRESS.md`: what you did in one
   or two lines, the failing-first evidence, what the next iteration would waste time
   rediscovering, and any problem you noticed but did not fix.
10. **Tick the task** in `ralph/plans/k8s-20c-s2/IMPLEMENTATION_PLAN.md` and include that in
    the commit (amend is fine before push; never amend a pushed commit).
11. **Exit.** Do not start another task.

## Completion

When every task in IMPLEMENTATION_PLAN.md is checked and the gate is green, check each bullet in
`ralph/plans/k8s-20c-s2/VALIDATION_CONTRACT.md` by running its command, record each result in
PROGRESS.md, then append a line containing exactly `RALPH_COMPLETE` to PROGRESS.md. The loop
watches for that string and stops.

## Hard limits

- Host mode stays byte-identical. No existing `*_test.go` is edited. No edit to `go.mod`,
  `cmd/crswd/`, `k8s/`, `AGENTS.md`, `docs/security.md`, `.github/` or `.claude/`.
- Standard library only. `go.sum` must not exist.
- No manifest or fixture names the `lawnmower` namespace or its Secrets, or `~/.claude`.
- Never push to `main`. Never force-push. Do not open the PR; the runner does.
- Never commit a secret. Never disable or route around a hook.
