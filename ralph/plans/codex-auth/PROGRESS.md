# Progress — Phase 2 — Codex sign-in

Notebook for `ralph/plans/codex-auth`. Each iteration appends below.

## Iteration 0 (planning, 10/6/26)

- Plan written from `specs/019-codex-runtime/`. No code changed.
- Measurements behind every Codex behaviour are in `specs/019-codex-runtime/research.md` section M.
- Left: every task in `IMPLEMENTATION_PLAN.md`, starting with T020.

## Iteration 1 (T020, 10/6/26)

- Added `internal/codexauth`: `DetectPrompt`, `Kind`, `Prompt` (String shows kind and URL host only), three golden panes from research F2/F3/F4 and a testdata README.
- Copied `quotes`, `containsUnquoted` and `flatten` from claudeauth rather than importing. Stdlib only (`net/url`, `regexp`, `strings`).
- Tests written first; they failed to build before the package existed, then passed.
- Learned: `oneTimeCode` skips quoted mentions of the phrase and reads the code after the first unquoted one, so a pane quoting the phrase above a real screen still yields the code.
- Not fixed: nothing calls `codexauth` yet. T021 onward wires it.
- Left: T021 to T026.

## Iteration 2 (T021, 10/6/26)

- `loginrelay.NewCodex`, `CodexWindowName`, `ErrNoCodeToDeliver` and `State.Device` added. `Relay` gained `flow`, `window` and `executable`; `New` keeps the Claude behaviour and every method uses `r.window`.
- Codex `Start` types `<executable> -c check_for_update_on_startup=false login --device-auth`. `SignedIn` runs `<executable> login status` (exit 0 true, exit 1 plus `Not logged in` false, else an error wrapped `read the Codex sign-in state`).
- Tests written first; they failed to compile before the change, then passed. Fake codex binary lives at an absolute path in `t.TempDir()`, off `PATH`. The window-name test now covers both names.
- Learned: the sandbox's command guard rejects a Bash heredoc containing a brace followed by a quote, so Go table literals were added with the Edit tool.
- Not run: `go test -tags tmux` (task did not touch `tmuxctl` or `session`); `go vet -tags tmux ./internal/loginrelay/` passes.
- Not fixed: nothing in `httpapi` calls `NewCodex` yet. T022a to T022c wire it.
- Left: T022a to T026.

## NEEDS CLARIFICATION

(none yet)
