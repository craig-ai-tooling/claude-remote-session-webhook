# Ralph loop prompt: k8s-20c-s5

You are running autonomously, one iteration at a time, with a fresh context. You have no
memory of previous iterations. Everything you need is on disk.

Milestone: k8s-20c-s5, `podctl` (the second `tmuxctl.Controller`), the in-pod helpers and the
`session.Manager` hooks, for Claude Code and Codex sessions in pods (spec 017 FR-002, FR-011,
FR-014; spec 019 FR-023, FR-024). One PR against `main` when the loop is done.

## Blocked work

1. Run the check in the `## Dependency` section of `IMPLEMENTATION_PLAN.md`.
2. If it fails and the last line of `PROGRESS.md` is already `BLOCKED: dependency not merged`, change no file and exit.
3. If it fails otherwise, append `BLOCKED: dependency not merged` as the last line of `PROGRESS.md`, commit only that file, and exit.
4. If any line in `IMPLEMENTATION_PLAN.md` starts with `- [!]`, change no file and exit.

## Do exactly this, in order

1. **Read the contract.** `AGENTS.md` at the repo root, then `docs/conventions.md` (you are
   writing Go) and `docs/security.md` (this slice runs sessions in pods and reaches them through
   the Kubernetes exec API). Also
   `.specify/memory/constitution.md`.
2. **Read the plan.** `ralph/plans/k8s-20c-s5/IMPLEMENTATION_PLAN.md`, including its Design
   section and its "Files touched" list. For background only:
   `specs/017-k8s-native-execution/k8s-20c-plan.md` section 2 "S5", sections 5 and 7.
3. **Read the notebook.** `ralph/plans/k8s-20c-s5/PROGRESS.md`. It is what past iterations did.
   Do not redo finished work.
4. **Apply "Blocked work" above.** If it says exit, exit.
5. **Pick exactly ONE task**: the topmost unchecked `- [ ]` item in IMPLEMENTATION_PLAN.md. One.
6. **Write the test first and watch it fail.** Every new test must fail against the code as it
   was before your change. Run it before implementing and copy the failing line into PROGRESS.md.
   A test for a package that does not exist yet fails by not compiling; say so. Never write a
   test that reads `origin/main` or any git ref.
7. **Implement it**, inside the "Files touched" list only. If the task is ambiguous, do not
   guess: write it under `NEEDS CLARIFICATION` in PROGRESS.md, mark the task `- [!]` in
   IMPLEMENTATION_PLAN.md, commit those two files, and exit.
8. **Run the gate.** All of these must pass:
   `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./... && go test -tags quickstart ./cmd/crswd && golangci-lint run`
   Also `test ! -e go.sum` and `grep -c require go.mod` printing `0`. The quickstart suite needs
   `127.0.0.1:8765` free; if the deployed daemon holds it, record that in PROGRESS.md rather than
   skipping silently. If you cannot make the gate pass, revert your change and log why. Never
   commit a broken tree.
9. **Commit.** Stage only the paths you touched (never `git add -A`; other sessions share this
   tree). One focused commit, conventional subject naming the task, e.g.
   `feat(podctl): exec-backed Controller methods (k8s-20c-s5 T7)`. Write git commands
   bare, never `git -C`.
10. **Update the notebook.** Append to `ralph/plans/k8s-20c-s5/PROGRESS.md`: what you did in one
   or two lines, the failing-first evidence, what the next iteration would waste time
   rediscovering, and any problem you noticed but did not fix.
11. **Tick the task** in `ralph/plans/k8s-20c-s5/IMPLEMENTATION_PLAN.md` and include that in
    the commit (amend is fine before push; never amend a pushed commit).
12. **Exit.** Do not start another task.

## Completion

When every task in IMPLEMENTATION_PLAN.md is checked and the gate is green, check each bullet in
`ralph/plans/k8s-20c-s5/VALIDATION_CONTRACT.md` by running its command, record each result in
PROGRESS.md, then append a line containing exactly `RALPH_COMPLETE` to PROGRESS.md. The loop
watches for that string and stops.

## Hard limits

- Host mode stays byte-identical. No existing `*_test.go` is edited. No edit to the root `go.mod`,
  `cmd/crswd/`, `AGENTS.md`, `docs/security.md`, `.github/workflows/` or `.claude/`.
- The gate for this slice also covers the cluster module. Run each of these:
  1. `go -C k8s vet ./...`
  2. `go -C k8s test ./...`
  3. `go -C k8s build ./...`
  4. Do not lint the cluster module yourself: the sandbox refuses `cd`. CI lints it, and the operator runs it before the PR.
- The root module stays standard library only and the root `go.sum` must not exist. `k8s/go.sum` is expected.
- No manifest, fixture or test names the `lawnmower` namespace or its Secrets, or the VM's `~/.claude`.
- Line numbers in the plan may be stale. Find the named function with `grep -n` before editing.
- The session's text and pane content are secret (`docs/security.md` §3). Never put a payload, a screen or a credential in an argv, an error or a log line.
- Never push to `main`. Never force-push. Do not open the PR; the runner does.
- Never commit a secret. Never disable or route around a hook.
