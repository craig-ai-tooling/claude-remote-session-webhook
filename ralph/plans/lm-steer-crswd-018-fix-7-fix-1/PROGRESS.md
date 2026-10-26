# Progress: pasted prompt, Claude gone (lm-steer-crswd-018-fix-7-fix-1)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/26/26)

- Branch `plan/lm-steer-crswd-018-fix-7-fix-1`, cut from `main` at `97a0f58`.
- `grep -rn '"C-u"' internal/` prints nothing on the baseline. That is the verifier's evidence.
- `grep -rn ErrSessionNotReady internal/ cmd/` prints nothing on the baseline.
- The fake's `ExitAfterPaste` is already on `main` from the parent milestone. Do not re-add it.
- The parent's blocked T2 built the whole session-side fix and proved it passes. Its notebook,
  `ralph/plans/lm-steer-crswd-018-fix-7/PROGRESS.md`, "Iteration 2 — T2, blocked", lists the
  four tests that broke and why. Read it, never edit it.
- The parent's NEEDS CLARIFICATION is answered in IMPLEMENTATION_PLAN.md decisions D2 and D3.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations
