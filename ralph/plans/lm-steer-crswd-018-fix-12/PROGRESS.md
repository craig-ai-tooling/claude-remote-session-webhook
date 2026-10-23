# Progress: `request_id` on the prompt route (lm-steer-crswd-018-fix-12)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/23/26)

- Branch `plan/lm-steer-crswd-018-fix-12`, cut from `main` at `ebefd36`.
- Baseline: `go test ./internal/httpapi/ -run 'RequestID|Prompt' -v` exits 0. Its only `RequestID` test is `TestAHandlerCanAmendTheOneRecordWithARequestID`, which covers only the audit setter.
- Baseline: a prompt body carrying `request_id` is refused 400 with reason `errBodyUnknownField`, because `promptRequest` has only `Text` and the decoder rejects unknown fields.
- `grep -n 'errPromptBadRequestID\|validRequestID' internal/httpapi/*.go` prints nothing on the baseline.
- The verifier's evidence said `request_id` appears in no .go file. It does now: the parent's T001 and T002 landed the audit half (T002 is PR #225, `fc58093`). Only the handler half is missing.
- Earlier loops in this repo found golangci-lint and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If that holds here, record it and use the fallback in PROMPT.md.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations
