# Progress: concurrent prompts use distinct buffers and never interleave (lm-steer-crswd-018-fix-11)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/23/26)

- Branch `plan/lm-steer-crswd-018-fix-11`, cut from `main` at `ebefd36`.
- Baseline: `go test -race ./internal/session/ -run 'Prompt|Interleave'` exits 0 (`ok ... 1.070s`).
- `grep -rn 'TwoConcurrent\|ConcurrentPastes' internal` prints nothing on the baseline.
- The code under test already serialises (`lockSession` in `Prompt`) and draws a fresh buffer per paste. The new tests pass against unchanged code. Their failing-before proof is a temporary mutation (plan H2, H4), reverted before commit.
- Earlier loops in this repo found golangci-lint, tmux and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If tmux is missing, T2 cannot run its Verify: follow "If the task is unclear or cannot be finished" in PROMPT.md.

## Failing-before evidence

| Task | Test | Mutation | Failure line |
|---|---|---|---|

## Iterations
