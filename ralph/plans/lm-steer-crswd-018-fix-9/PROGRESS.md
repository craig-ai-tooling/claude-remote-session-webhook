# Progress: leading `!` or `/` on the prompt route (lm-steer-crswd-018-fix-9)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/22/26)

- Branch `plan/lm-steer-crswd-018-fix-9`, cut from `main` at `9a938b4`.
- Baseline: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0 (`ok ... 0.205s`). Every test it runs predates the rule.
- `grep -n 'errPromptLeadingCommand' internal/httpapi/*.go` prints nothing on the baseline.
- No `hostilePrompts` entry and no quickstart prompt payload begins with `!` or `/`, so the new refusal breaks no existing test.
- T2's tests are regression guards. They are expected to pass before and after T1; they cannot be proven failing-before, and that is correct.
- Earlier loops in this repo found golangci-lint and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If that holds here, record it and use the fallback in PROMPT.md.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations
