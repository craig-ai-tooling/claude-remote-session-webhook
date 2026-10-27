# Progress: Claude gone mid-prompt answers 409 over HTTP (lm-steer-crswd-018-fix-7-fix-2)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/27/26)

- Branch `plan/lm-steer-crswd-018-fix-7-fix-2`, cut from `main` at `97a0f58`.
- Baseline: `go test ./internal/httpapi/ ./internal/session/ -run 'Prompt|AddressesAnothersWindow|Codex'` exits 0.
- `grep -rn 'session not ready' internal/` prints nothing on the baseline. That is the verifier's evidence.
- `grep -rn ErrSessionNotReady internal/ cmd/` and `grep -rn '"C-u"' internal/` print nothing on the baseline.
- The fake's `ExitAfterPaste` is already on `main` from the parent milestone. Do not re-add it.
- The parent's blocked T2 built the session-side fix and proved it passes. Its notebook,
  `ralph/plans/lm-steer-crswd-018-fix-7/PROGRESS.md`, "Iteration 2 — T2, blocked", lists the four
  tests that broke and why. Read it, never edit it.
- Sibling branch `plan/lm-steer-crswd-018-fix-7-fix-1` plans T1-T3 of this plan with the same
  decisions. It was unmerged at planning time. If `main` gains any of its changes first, each task
  here says how to detect them and reuse them instead of adding a second copy.
- `docs/fixes-log.md` has no trailing newline. T5's line must start with one.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations
