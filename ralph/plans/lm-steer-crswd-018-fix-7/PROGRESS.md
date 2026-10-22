# Progress: Claude exits mid-prompt (lm-steer-crswd-018-fix-7)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/22/26)

- Branch `plan/lm-steer-crswd-018-fix-7`, cut from `main` at `fc58093`.
- Baseline: `go test ./internal/session/ ./internal/tmuxctl/ ./internal/httpapi/` exits 0.
- `grep -rn '"C-u"' internal/` prints nothing on the baseline. That is the verifier's evidence.
- `grep -rn ErrSessionNotReady internal/ cmd/` prints nothing on the baseline.
- Sibling `plan/lm-steer-crswd-018-fix-4` is unmerged and also adds `ErrSessionNotReady` and
  the 409 mapping. If `main` gains either before this loop runs, reuse it rather than adding
  a second copy (plan decisions F1 and F4).
- The parent milestone's PROGRESS.md records that golangci-lint and
  `~/code/ai-lawnmower/loop/safe-commit.sh` were unreachable inside its loop container. If
  that holds here, record it and commit with `git add <each path by name>` then `git commit`.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations
