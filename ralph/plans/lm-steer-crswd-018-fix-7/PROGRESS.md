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
| T1 | `TestFakeExitAfterPaste` | `internal/tmuxctl/fake_test.go:410:6: f.ExitAfterPaste undefined (type *tmuxctl.Fake has no field or method ExitAfterPaste)` |

## Iterations

### T1 — the fake can model a process that exits on the next paste

Changed:

- `internal/tmuxctl/fake.go`: new `(*Fake).ExitAfterPaste(name)` knob beside
  `QuitAfterInterrupts`, a `exitAfterPaste bool` on `fakeSession`, and
  `(*fakeSession).exitIfArmed()`. `Paste` and `PasteBracketed` now bind the
  session they looked up (`s, ok := f.sessions[name]`) and call `s.exitIfArmed()`
  on the success path only, so a refused paste does not consume the knob. The
  pane command becomes `"bash"`, the same literal `SendKeys` and `Seed` use for
  a pane whose program died into its login shell.
- `internal/tmuxctl/fake_test.go`: `TestFakeExitAfterPaste` (table-driven over
  `Paste` and `PasteBracketed`), `TestFakeExitAfterPasteOutlivesARefusedPaste`,
  and the `fakeLiveness` helper that reads `List`'s one row.

Commands:

- `go test ./internal/tmuxctl/ -run ExitAfterPaste` — build failure before the
  change (the row above), exits 0 after, all three cases pass.
- `gofmt -l .` — prints nothing. `go build ./...`, `go vet ./...`,
  `go vet -tags tmux ./...`, `go vet -tags quickstart ./cmd/crswd` — all exit 0.
- `go test ./...` — exits 0, every package ok.
- `test ! -e go.sum` — exits 0.
- `golangci-lint` is **not installed in this loop container**: the SessionStart
  hook reports it is not on PATH, as the parent milestone's PROGRESS.md predicted.
  The lint step could not run for this task.

For the next iteration:

- **`ExitAfterPaste` seeds no `@crswd-binary`.** A session it creates on its own
  reads `LivenessUnknown`, not `LivenessStopped`, because `livenessOf` needs the
  option to compare against. T2 and T3 must set
  `SetOption(ctx, name, tmuxctl.OptionBinary, "claude")` on the session before
  the paste, or the post-paste re-check will see `LivenessUnknown` and send
  Enter. A session made by `Manager.Create` already carries it.
- `go test ./...` was green on the whole tree after T1, so any failure T2 sees in
  the three exact-call tests is T2's own new `OpList` call.
- **`~/code/ai-lawnmower/loop/safe-commit.sh` could not be run**, as the planning
  notes predicted. It lives outside `/workspace`, which is the only directory
  this loop's tooling may reach, so invoking it needs an approval a headless
  iteration cannot give. T1 was committed with the documented fallback:
  `git add <each path by name>` then `git commit`. Later iterations should expect
  the same and not waste a turn on it.
