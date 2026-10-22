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
| T2 | `TestPromptWithdrawsWhenClaudeExitsMidPrompt` | `internal/session/manager_test.go:1207:21: undefined: ErrSessionNotReady` (and `clearLineKey`, twice) — written, proven failing, then reverted with the rest of T2; see the NEEDS CLARIFICATION below |

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

### Iteration 2 — T2, blocked

Nothing in `internal/` changed. T2's box is now `- [!]`; the tree was returned to
`6a63f28` exactly (`git status --short` and `git diff --stat` both print nothing).

**What was built, and that it worked.** F1, F2 and F3 were implemented in full and the
new test passed:

- `internal/session/session.go`: `ErrSessionNotReady` in the `var (...)` block after
  `ErrSessionDead`.
- `internal/session/manager.go`: `clearLineKey = "C-u"` beside `enterKey`, and a
  `submitPaste(ctx, s)` holding the whole of F2 — one `List`, withdraw on
  `LivenessStopped` only, `errors.Join` when the `C-u` itself fails, Enter on running,
  unknown or no row, and the wrapped `List` error on a failed listing. `Prompt`'s last
  two lines became `return m.submitPaste(ctx, s)`.
- `internal/session/manager_test.go`: `TestPromptWithdrawsWhenClaudeExitsMidPrompt`
  (via `ExitAfterPaste`) asserted `OpPaste, OpPaste, OpList, OpSendKeys`, a trailing
  `C-u`, no `Enter` in any argv, and no prompt text in the error.
  `go test ./internal/session/ -run TestPromptWithdrawsWhenClaudeExitsMidPrompt` → `ok`.

So the shape of the fix is settled and reproducible. What stops T2 is that its Verify
command cannot go green inside "Files touched".

**NEEDS CLARIFICATION — T2: F2's post-paste `List` fails a test in a file T2 may not
touch, and the Codex fixture reads `LivenessStopped`.**

Two things, one of them a hard stop.

*1. The hard stop: `internal/httpapi/isolation_test.go` is not in "Files touched".*

`TestNoRequestScopedToOneSessionAddressesAnothersWindow` (`isolation_test.go:240`) sweeps
**every** tmux call a session-scoped request issued and requires each one's argv to
contain the resolved session's tmux name. Its only exemption is `load-buffer` /
`delete-buffer` on a `BufferPrefix` buffer, "because [they] name a buffer drawn per call,
not a window, so there is no session in them to be the wrong one."

`tmuxctl.List` is `list-sessions -F <format>`. It names **no** session at all — a third
shape the exemption does not cover — so with F2 in place the prompt row fails:

```
isolation_test.go:265: POST /sessions/{id}/prompt ran List [tmux list-sessions -F #{session_name}|…]
    against a target the resolved session does not name; want "crswd-5e2f5e0f3d3bf25d856f21615d11fadc"
```

This is not avoidable from inside the allowlist:

- `List(ctx)` is global by its interface contract ("returns every session on the server
  in one call"), and F2 names it specifically. The only per-session reads on `Controller`
  are `Has` (a bool, no liveness) and `CapturePane`; neither answers the question, and
  adding a method to `Controller` is forbidden by the loop's rules.
- The fix is small and obvious — widen the exemption to a call that names no window,
  `list-sessions` — but `internal/httpapi/isolation_test.go` is outside "Files touched",
  and it is the test asserting that no request scoped to one session acts on another's
  window. Quietly editing a security-invariant test to admit a new call shape is exactly
  the change that should get a human's eye rather than a loop's.

**What the plan needs to say:** either add `internal/httpapi/isolation_test.go` to "Files
touched" with the intended exemption spelled out, or state a different way for `Prompt`
to re-read one session's liveness. Note the exemption would have to be written as "a call
that addresses no window" rather than "skip `OpList`" — the latter would let a future
targeted call slip the sweep.

*2. A second, smaller gap in the same task: `codexFixture` models a stopped session.*

"Facts measured" says a session from `Manager.Create` "carries `@crswd-binary` = `claude`
and pane command `claude`, so it reads `LivenessRunning`". True for the default command,
**false for Codex**: `codexFixture` (`harness_test.go:192`) creates with the `codex`
template, so `markSession` writes `@crswd-binary` = `codex`, while the fake's `New` sets
`paneCommand` to `fakeAliveCommand` ("claude") regardless of what was started. So
`livenessOf("codex", "claude")` = `LivenessStopped`, and with F2 in place a Codex prompt
is withdrawn:

```
harness_test.go:221: Prompt() unexpected error: submit the prompt in session 83008…: session not ready
```

The plan expected this test to need only an added `OpList`. It needs the fixture to hold
what the host holds as well — `f.tmux.SetPaneCommand(s.TmuxName(), "codex")`, which the
fake already exports (`fake.go:803`). `internal/session/harness_test.go` **is** in "Files
touched", so this one is fixable in place; it is recorded because it is a stated fact that
is wrong, and because the same wrongness will reach T3 if a Codex case is added there.
It is also, on its own, a fake that disagrees with the host in the way
`docs/conventions.md` warns about — every Codex test in this package currently runs
against a session tmux would call stopped.

**What was checked**

- `go test ./internal/session/ ./internal/httpapi/` with F2 in place: exactly four
  failures, all of them the above — `TestPromptPastesThenSubmits` and
  `TestAPromptIsDeliveredByteForByte` are the planned exact-call updates,
  `TestPromptCodexIsBracketed` is gap 2, and
  `TestNoRequestScopedToOneSessionAddressesAnothersWindow` is the hard stop. Nothing else
  in either package moved.
- `grep -n "^type Controller interface" -A 60 internal/tmuxctl/controller.go` — confirmed
  `Has`, `List` and `CapturePane` are the only reads, and that `List` is server-wide.
- `newIsolationFixture` plants rather than creates (`isolation_test.go:100`, and the
  comment at :119 says why), so `f.fixture.tmux.Calls()` holds only the request's own
  calls. There is no `before:` slice to take and nothing to re-scope.
- `grep -rn ErrSessionNotReady internal/ cmd/` and `grep -rn 'C-u' internal/` both still
  print nothing on `6a63f28`. F1 and F4 have not landed from the sibling branch, so
  whoever unblocks this still adds both.
- `git status --short`, `git diff --stat` → nothing but the two notebook files.

**For whoever unblocks this**

The revert was done with `Edit`, not `git checkout -- <path>`: `git checkout` is refused
by this container's Bash permission layer, the same way `safe-commit.sh` is (Iteration 1).
The prompt's "revert your uncommitted changes" step therefore cannot be run as written
here. The result was verified equivalent — both `git status --short` and `git diff --stat`
print nothing.

**safe-commit could not be run again**, refused by the same permission layer
("This command requires approval") for both `~/…` and the absolute path. This block was
therefore committed by the prompt's documented fallback: `git add <path>` once per path,
by name, then `git commit`.
