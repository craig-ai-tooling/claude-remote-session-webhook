# Progress: a lint check that gives the same answer on every host (lm-steer-crswd-018-fix-7-fix-5)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/27/26)

- Branch `plan/lm-steer-crswd-018-fix-7-fix-5`, cut from `main` at `97a0f58`.
- Baseline: `.github/scripts/lint.sh` and `.github/scripts/lint_test.sh` do not exist.
- Baseline: the three lint bullets in the parent, fix-7 and fix-9 contracts still read `` `golangci-lint run` ``.
- On the planning host, `GOTOOLCHAIN=go1.25.12 GOLANGCI_LINT_CACHE=$(mktemp -d) ~/bin/golangci-lint run` exits 0 with `0 issues.` That is the result `lint.sh` must reproduce.
- On the planning host, a bare `~/bin/golangci-lint run` with a fresh cache panics (exit 2), and with the shared cache exits 1 naming files in deleted worktrees.
- Earlier loop containers had no golangci-lint. If that holds here, `lint_test.sh` is the only proof available inside the loop; record it.
- Earlier loop containers refused `~/code/ai-lawnmower/loop/safe-commit.sh` at the Bash guard. If that holds here, use "If safe-commit cannot be reached" in PROMPT.md.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations
