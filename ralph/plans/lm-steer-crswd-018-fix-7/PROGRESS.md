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

## NEEDS CLARIFICATION — T2 (10/22/26)

**A fourth test fails on the new `List` call, and it is outside "Files touched".**

The plan names three exact-call tests that will see the post-paste `List`
(`TestPromptPastesThenSubmits`, `TestPromptCodexIsBracketed`,
`TestAPromptIsDeliveredByteForByte`). There is a fourth:

```
--- FAIL: TestNoRequestScopedToOneSessionAddressesAnothersWindow/POST_/sessions/{id}/prompt
    isolation_test.go:265: POST /sessions/{id}/prompt ran List [tmux list-sessions -F …]
        against a target the resolved session does not name; want "crswd-<id>"
```

`internal/httpapi/isolation_test.go` sweeps **every** tmux call a session-scoped
request issues and requires each one to name the resolved session's tmux target.
It carries exactly one exemption today, for `load-buffer`/`delete-buffer`, whose
argv names a per-call buffer rather than a window. `list-sessions` is the same
shape of exception — it is a server-wide read that names no session at all — but
there is no exemption for it, and the file is **not in "Files touched"**, so this
iteration may not add one.

What I checked before filing this:

- **There is no per-session liveness call to use instead.** The `Controller`
  interface (`internal/tmuxctl/controller.go`) has `Has` (existence only, and the
  session does exist — it is the harness inside it that exited) and `List`.
  Nothing else reports `Liveness`. Decision F2 names `List` for that reason, and
  the loop may not add a method to `Controller`.
- **Moving the re-check does not help.** `internal/httpapi/sessions.go` is in
  scope, but a `List` issued from the handler lands in the same
  `f.fixture.tmux.Calls()` the isolation test sweeps, so it fails identically.
- **No other session-scoped route runs `List` today**, which is why the test is
  green on the baseline and why the prompt route is the only subtest that fails.

What the plan needs to say, either one:

1. Add `internal/httpapi/isolation_test.go` to "Files touched" and state the
   exemption to write — presumably `call.Op == tmuxctl.OpList` alongside the
   existing `bufferOnly` carve-out, with the reason (a server-wide read names no
   window, so it cannot name the wrong one). This is the straightforward reading,
   but it widens a test that exists to catch cross-session targeting, and that
   widening is the plan author's call rather than the loop's.
2. Or say that the re-check must not be a server-wide `List`, in which case F2
   needs a different mechanism and the "never add a method to `Controller`" rule
   has to be reconciled with it.

Work done and then reverted (`git restore` on all five, tree clean):
`internal/session/session.go` (`ErrSessionNotReady`), `internal/session/manager.go`
(`clearLineKey`, `withdrawIfStopped`), `internal/session/manager_test.go`
(`TestPromptWithdrawsWhenClaudeExitsMidPrompt` + `OpList` in the exact-call want),
`internal/session/harness_test.go`, `internal/httpapi/sessions_test.go`.

`~/code/ai-lawnmower/loop/safe-commit.sh` could not be run again, for the reason
T1 recorded: it lives outside `/workspace`, which is the only directory this
loop's tooling may reach. This block was committed with the documented fallback,
`git add <each path by name>` then `git commit`.

Two things the next attempt would otherwise rediscover:

- **`codexFixture` leaves the pane running `claude`.** `Manager.Create` writes
  `@crswd-binary` = `codex|node` for a Codex session, but the fake starts every
  created session's pane on `fakeAliveCommand` (`"claude"`), so `livenessOf`
  reads **`LivenessStopped`** and the post-paste re-check withdraws.
  `TestPromptCodexIsBracketed` therefore needs
  `f.tmux.SetPaneCommand(s.TmuxName(), "codex")` before the prompt, on top of the
  `OpList` the plan already calls for. A default-command session made by
  `mustCreate` is fine — `@crswd-binary` and the pane command are both `claude`.
- **`argvList` being unexported is handled by a nil `Argv` in the want slice**,
  with the comparison loop reading
  `want[i].Argv != nil && !slices.Equal(...)`. That works in both
  `TestPromptPastesThenSubmits` and `TestAPromptIsDeliveredByteForByte`.

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
