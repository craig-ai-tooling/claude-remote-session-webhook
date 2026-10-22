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
| T1 | `TestFakeExitAfterPasteStopsThePaneOnlyOnce`, `TestFakeExitAfterPasteSeedsTheSession` | `internal/tmuxctl/fake_test.go:930:6: f.ExitAfterPaste undefined (type *tmuxctl.Fake has no field or method ExitAfterPaste)` |

## Iterations

### Iteration 1 — T1 (tmuxctl fake: `ExitAfterPaste`)

**Changed**

- `internal/tmuxctl/fake.go`: new `exitAfterPaste bool` on `fakeSession`, a
  `(*fakeSession).pasteLanded()` helper called at the end of the success path of both
  `Paste` and `PasteBracketed`, and the exported `ExitAfterPaste(name)` knob. It seeds the
  session with `paneCommand: fakeAliveCommand` when absent, exactly as `SetPane` does, and
  disarms itself when it fires so the flip happens once.
  - Both pastes previously discarded the session with `if _, ok := f.sessions[name]; !ok`;
    they now bind `s` so `pasteLanded` can reach it. Nothing else about either changed.
  - The flip sets `paneCommand = "bash"`, the literal `SendKeys`' `quitAfter` path and
    `Seed`'s `LivenessStopped` case already use. No constant was introduced: one would have
    left two other sites on the literal.
- `internal/tmuxctl/fake_test.go`: `TestFakeExitAfterPasteStopsThePaneOnlyOnce`
  (table-driven over `Paste` and `PasteBracketed`: running before, a failed paste consumes
  nothing, stopped after, and a second paste does not re-fire) and
  `TestFakeExitAfterPasteSeedsTheSession`. Helper `claudeLiveness` reads `Claude` out of
  `List`, which is the only way the daemon learns it.
  - The table seeds with `f.Seed(tmuxctl.SessionInfo{..., Claude: tmuxctl.LivenessRunning})`
    rather than `New` + `SetOption`, because `Seed` sets `@crswd-binary` *and* the pane
    command together and records no call. **`ExitAfterPaste` deliberately does not set
    `@crswd-binary`** — a session it seeds on its own reads `LivenessUnknown`, which is the
    plan's note about the httpapi fixture needing `SetOption(OptionBinary, "claude")` first.

**Commands**

- Failing-before: `go test ./internal/tmuxctl/ -run ExitAfterPaste` → build failure, row
  recorded above.
- Verify after: `go test ./internal/tmuxctl/ -run ExitAfterPaste` → `ok` (exit 0).
- `gofmt -l .` → nothing. `go build ./...`, `go vet ./...`, `go vet -tags tmux ./...`,
  `go vet -tags quickstart ./cmd/crswd`, `test ! -e go.sum` → all exit 0.
- `golangci-lint` is **not installed in this container** (`which golangci-lint` exits 1), as
  the parent milestone's notebook predicted. Lint was not run.
- `git status --short` → only the three paths this iteration touched.

**For the next iteration**

- **`go test ./...` is flaky in this container, in `internal/sessionpod` only, and it is not
  this milestone's doing.** `TestRun*` there spin goroutines polling at 1ms and busy-wait
  with a 5s `deadline`; under the load of the full parallel run they time out ("timed out
  waiting for the session to exist", "Run did not return within 5s"), and a different subset
  fails each run. `go test -count=1 -p 1 ./internal/sessionpod/` passes in 3s. T2's Verify is
  `go test ./...` — if it fails only in `internal/sessionpod` with those messages, re-run that
  package with `-p 1` rather than treating it as a regression. Nothing in `sessionpod` touches
  `Paste`; it exercises `New`, `Has` and `List`.
- Every other package passed: `internal/session` 55s, `internal/httpapi` 35s. The whole
  default suite takes about two minutes here.
- `grep -rn '"C-u"' internal/` still prints nothing. T2 is where that changes.
- **safe-commit could not be run.** `/home/ralph/code/ai-lawnmower/loop/safe-commit.sh` is
  present and executable, but invoking it was refused by this container's Bash permission
  layer ("This command requires approval"), which is not something to route around. This
  iteration therefore committed by the prompt's documented fallback: `git add <path>` once per
  path, by name, then `git commit`. Expect the same in later iterations.
