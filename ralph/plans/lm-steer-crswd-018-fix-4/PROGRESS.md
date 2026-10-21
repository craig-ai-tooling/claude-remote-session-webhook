# Progress: shell-fallback prompt refusal (lm-steer-crswd-018-fix-4)

The loop's only memory between iterations. Append; do not rewrite earlier entries.

## Origin

- Filed by `loop/fix-features.py` from verify run 1014-la1i, parent task lm-steer-crswd-018.
- Failed assertion: "shell-fallback pane gives 409 `session not ready`, no paste".
- Evidence at filing: the string `session not ready` was in no `.go` file, no shell-detection
  test existed, and `go test -run Prompt` passed only on tests that predate the requirement.

## Planning notes (10/21/26)

- Branch `plan/lm-steer-crswd-018-fix-4` starts from `main` at `1eeb59c`.
- The parent milestone's loop (PR #225, branch `ralph/1015-yo8i`) had finished the spec tasks
  and T001 to T002 (audit `request_id`). It had not touched `Manager.Prompt` or the prompt
  handler, so nothing there overlaps this plan's Go files except `internal/httpapi/sessions.go`,
  which T003 to T005 of that plan will also edit later.
- Measured on tmux 3.4: a `#!/bin/sh` script named `claude` reports `pane_current_command` = `sh`.
  The same script re-exec'd through `exec -a claude bash` reports `claude`. This is why T1 exists.
- `127.0.0.1:8765` was held by the deployed daemon on the planning host, so the quickstart suite
  was not run during planning.

## Failing-before evidence

| Task | Test | Failure line against the old code |
|---|---|---|

## Iterations
