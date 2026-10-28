# Validation contract: a git-free "no new package under internal/" check for fix-7 (lm-steer-crswd-018-fix-7-fix-6)

What "done" means for this milestone, decided before the plan was cut. A checker
with no history, no diff and no `.git` directory runs these against the tree.
All commands run from the repository root.

Milestone `lm-steer-crswd-018-fix-7` states its "no new package under `internal/`"
assertion as a `git ls-tree` diff of `main` against `HEAD`. The black-box verifier
copies the tree without `.git` (run `1022-tbfh` reported `fatal: not a git repository`),
so that assertion can never be checked and always fails. This milestone restates it
so it needs nothing but the files.

- The fix-7 contract no longer depends on git for the package check: `bash -c '! grep -n "git ls-tree" ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md'` exits 0 and prints nothing.
- The fix-7 contract names all seventeen packages in its package check: `grep -cF 'access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` prints `1`.
- The tree as it stands has exactly those seventeen top-level packages under `internal/`, checked with no git: `bash -c 'diff <(find internal -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort) <(printf "internal/%s\n" access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater)'` exits 0 and prints nothing.
- The same check catches a new package: in a scratch copy of `internal/` with an extra directory added, it fails and names that directory. `bash -c 'd=$(mktemp -d) && cp -r internal "$d/" && mkdir "$d/internal/zzzprobe" && cd "$d" && diff <(find internal -mindepth 1 -maxdepth 1 -type d | LC_ALL=C sort) <(printf "internal/%s\n" access admit audit auth buildinfo claudeauth codexauth config harness httpapi loginrelay quota release session sessionpod tmuxctl updater)'` exits 1 and prints a line containing `internal/zzzprobe`.
- The fix-7 contract kept every other assertion: `grep -c '^- ' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` prints `15`, and `grep -c 'C-u' ralph/plans/lm-steer-crswd-018-fix-7/VALIDATION_CONTRACT.md` prints `3`.
- The product still builds and its tests pass: `go build ./... && go test ./...` exits 0.
