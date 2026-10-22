# Progress: a Claude prompt is one bracketed paste plus one Enter (lm-steer-crswd-018-fix-8)

The loop's notebook. Each iteration appends under "Iterations". Read all of it
before starting a task.

## Planning notes (written 10/22/26, before iteration 1)

Measured on the planning host, in this worktree, at commit `fc58093`:

- `tmux -V` printed `tmux 3.4`. `golangci-lint` is at `/home/nctiggy/bin/golangci-lint`.
  The parent milestone found golangci-lint missing inside its loop container. If
  `golangci-lint version` exits 127 here, record the lint result as unverified
  in those words. Do not claim it passed.
- `Manager.Prompt` is at `internal/session/manager.go:1194`. Its paste is the line
  `if err := m.paste(ctx, s, []byte(text)); err != nil {` (about line 1217).
- `Manager.paste` is at `internal/session/manager.go:1253`. Its doc comment
  (lines 1249 to 1252) says it serves Prompt and Compact.
- `Manager.Type` (`internal/session/input.go:101`) already calls
  `m.tmux.PasteBracketed` for every harness. It is the model to copy. Do not edit it.
- The fake's bracketed paste-buffer argv is
  `{"tmux", "paste-buffer", "-p", "-d", "-b", <buffer>, "-t", <pane>}`
  (`internal/tmuxctl/fake.go:101`). Its load-buffer argv is the same as the plain
  path: `{"tmux", "load-buffer", "-b", <buffer>, "-"}`. Both calls record
  `Op == tmuxctl.OpPasteBracketed`.
- Spec 020 tasks.md section "T006" (line 373) describes this exact change.

## Code notes: every line T1 must change

Switching the Prompt paste to bracketed breaks these pins. Each must move to
`tmuxctl.OpPasteBracketed` (and `-p` in the paste-buffer argv where the argv is
spelled out). Measured with `grep -rn "OpPaste\b" --include=*_test.go`.

| File | Line | Test | What changes |
|---|---|---|---|
| `internal/session/harness_test.go` | 237 | `TestPromptClaudeUnchanged` | Replace with a test named with `Prompt` (for example `TestPromptClaudeIsBracketed`) that sends `"first line\nsecond line"` and asserts ops `[OpPasteBracketed, OpPasteBracketed, OpSendKeys]`, `-p` in the paste-buffer argv, the full text on load-buffer stdin, and the last send-keys argv `Enter`. Copy `TestPromptCodexIsBracketed` at line 214. |
| `internal/session/manager_test.go` | 1165-1166 | `TestPromptPastesThenSubmits` | Both `OpPaste` become `OpPasteBracketed`; the paste-buffer argv gains `"-p"` after `"paste-buffer"`. |
| `internal/session/manager_test.go` | 1239 | `TestPromptNamesNoPromptTextInItsError` | `FailOp(tmuxctl.OpPaste, …)` becomes `FailOp(tmuxctl.OpPasteBracketed, …)`. |
| `internal/httpapi/sessions_test.go` | 1071-1074 | the prompt delivery table test | Both `OpPaste` become `OpPasteBracketed`; paste-buffer argv gains `"-p"`. |
| `internal/httpapi/sessions_test.go` | 1222 | `TestATmuxFailureDuringAPromptAnswersFiveHundredWithNoDetail` | `"the paste failed": tmuxctl.OpPaste` becomes `tmuxctl.OpPasteBracketed`. |
| `internal/audit/leak_test.go` | 1358 and 1363 | the "a prompt the host refused" leak case | Both `FailOp(tmuxctl.OpPaste, …)` around the `/prompt` request become `OpPasteBracketed`. |

Leave these unchanged, which look similar but cover Compact or other packages:

- `internal/session/harness_test.go:289` (`TestCompactClaudeUnchanged`).
- `internal/session/manager_test.go:1272-1273` (`TestCompactUsesBufferPath`).
- `internal/audit/leak_test.go:955` and `:960` (a browser compact).
- `internal/httpapi/actions_test.go:4773` (compact).
- anything in `internal/loginrelay`, `internal/claudeauth`, `internal/codexauth`, `internal/tmuxctl`.

Also in T1, in `internal/session/manager.go`:

- The Prompt doc comment says "Paste writes the bytes to a tmux buffer". Add one
  sentence saying the paste is bracketed for every harness (spec 020 FR-009), so a
  newline inside the text is text and only the daemon's Enter submits.
- Update the `paste` doc comment: it now serves `Compact` only.

## T2 recipe: the real-tmux test

- First line `//go:build tmux`, package `session`. No `t.Parallel()`.
- Reuse from `internal/session/mode_test.go`, do not redefine: `newModeFixture(t)`,
  `f.manager(t)`, `f.repo()`, `f.waitForPane`, `f.withScrollback`, `containsLine`.
- After `mgr, _ := f.manager(t)`, call `mgr.SetStartCommands(config.NewStartCommands(map[string]string{...}))`
  with one entry of its own whose command is a POSIX `sh -c` line that runs
  `stty -echo`, prints `\033[?2004h` (turns on bracketed-paste mode), prints a
  ready marker, then `exec cat -v`. Keep the fixture's default entry
  (`config.DefaultStartCommandName`) pointing at a non-Claude command: never start a
  real harness (D5).
- Create the session with that start command name, `waitForPane` for the marker,
  then `mgr.Prompt(ctx, *s, "first line\nsecond line")`.
- `cat -v` prints ESC as `^[`. Wait for `^[[201~` to appear: it is printed only after
  the Enter completes the line, so it proves the submit. Then assert, on
  `withScrollback`, exactly one `^[[200~`, exactly one `^[[201~`, and both
  `first line` and `second line` present.
- Failing-before: with T1 reverted (`git stash` is forbidden; instead temporarily
  change the one Prompt line back to `m.paste(ctx, s, []byte(text))` and run the
  Verify), the test must fail because no `^[[200~` appears. Restore the line after.
- If the start line's quoting does not survive being typed into the pane's shell,
  write the `sh` program to a file under `t.TempDir()` and start `sh <that file>`.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|

## Iterations

## Findings
