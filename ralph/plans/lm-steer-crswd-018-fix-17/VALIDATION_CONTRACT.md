# Validation contract: move the post-merge daemon check out of the black-box contract (lm-steer-crswd-018-fix-17)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history, no diff and no `.git` directory runs these against the tree.
All commands run from the repository root.

The parent milestone (`lm-steer-crswd-018`) ends its contract with an "After merge"
assertion: the running daemon's `vcs.revision` must descend from the merge commit,
checked with `systemctl` and `git merge-base`. The black-box verifier copies the
tree without `.git` and has no merge commit and no running daemon, so it can never
check that line and always fails it. This milestone moves the check to the parent's
"After the loop" steps, where the merging session runs it.

- The parent milestone's contract no longer asks the verifier for a running daemon or git history: `bash -c '! grep -qE "merge-base|MainPID|vcs.revision|systemctl" ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md'` exits 0.
- The parent milestone's contract has no "After merge" section: `bash -c '! grep -q "^## After merge" ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md'` exits 0.
- No bullet in the parent milestone's contract runs a git subcommand: `grep -cE '^- .*git (merge-base|ls-tree|log|diff|rev-parse|show)' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `0`.
- The parent milestone's contract kept every other assertion: `grep -c '^- ' ralph/plans/lm-steer-crswd-018/VALIDATION_CONTRACT.md` prints `23`.
- The post-merge daemon check is still written down for the merging session: `grep -c 'merge-base --is-ancestor' ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` prints `1`, and `grep -c 'MainPID' ralph/plans/lm-steer-crswd-018/IMPLEMENTATION_PLAN.md` prints `1`.
- The earlier fix milestone's count of the parent's assertions agrees with the parent: `grep -c "prints \`23\`" ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` prints `1`, and `grep -c "prints \`24\`" ralph/plans/lm-steer-crswd-018-fix-16/VALIDATION_CONTRACT.md` prints `0`.
- The product still builds and its tests pass: `go build ./... && go test ./...` exits 0.
