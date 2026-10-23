# Progress: prompt during a revive or continue (lm-steer-crswd-018-fix-10)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/23/26)

- Branch `plan/lm-steer-crswd-018-fix-10`, cut from `main` at `ebefd36`.
- Baseline: `go test -race ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0. Every test it runs predates the rule.
- `grep -rn 'ErrSessionNotReady\|session not ready' internal/` prints nothing on the baseline.
- `claimRestart` is called today only by `Continue` (`internal/session/manager.go`) and the supervisor's revive (`internal/session/supervisor.go`). `Prompt` does not call it.
- `releaseRestart` deletes unconditionally. A Prompt that deferred it before checking the claim would free a revive's claim; T1's test checks that the claim is still held after the refusal.
- No existing test holds a restart claim while prompting, so T1's refusal breaks no existing test.
- T2's test is a regression guard: `lockSession` already serialises paste and Enter. It passes before and after.
- T3 drives `refusePrompt` through `s.authenticate` because the httpapi tests cannot hold a restart claim (unexported, other package; the fake cannot pause a revive).
- Sibling plans fix-4 and fix-7 also add `ErrSessionNotReady` and the 409 mapping. Neither is merged at planning time. If one lands first, reuse its definitions.
- Earlier loops in this repo found golangci-lint and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If that holds here, record it and use the fallback in PROMPT.md.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations
