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

## Iteration 3 (T022a, 10/6/26)

- `Server.signin` and `Server.authCache` became `signins map[harness.Name]signInRelay` and `authCaches map[harness.Name]*authCache`. `newWithLayer1` allocates both eagerly (Claude cache only); `New` writes `signins[harness.Claude]`. No mutex added.
- `authStateCached`, `refreshAuthCache` and `askAuthState` take `h harness.Name` after ctx. An absent cache or relay reads `authUnknown`. Every caller passes `harness.Claude`; routes, responses and audit records are unchanged.
- Tests written first (`TestSignInsDefaultToNil`, `TestAuthCachesHaveClaudeEntry`); they failed to compile before the change, then passed. Existing tests got only the mechanical `signins[harness.Claude]` / `authCaches[harness.Claude]` edits.
- Not run: `-tags tmux`, `-tags quickstart`, `k8s/` checks (task touched none of them).
- Not fixed: no Codex relay or cache is wired yet. T022b does that.
- Left: T022b to T026.

## Iteration 4 (T022b, 10/6/26)

- `New` now builds the Codex relay with `loginrelay.NewCodex` when `codexOffered` is true, and sets `signins[harness.Codex]` and `authCaches[harness.Codex]` together. It sits after the Claude relay and the kubernetes-mode return. A build failure is reported, not fatal.
- `dashboardAuth` reads `parseHarness(r.URL.Query(), fieldHarness)`; a bad value gets `rejectBadRequest(errHarnessParam)`. No query still yields the Claude answer with an identical body.
- Tests written first (`TestDashboardAuthHarnessParam`, `TestDashboardAuthDefaultUnchanged`, `TestNewWiresCodexRelayOnlyWhenOffered`); five subtests failed before the change, then all passed. The third drives `New` itself, since a `NewWith` server never has a Codex relay.
- Learned: the shell guard rejected a heredoc with a brace followed by a quote again, so the tests went in with Edit.
- Not run: `-tags tmux`, `-tags quickstart`, `k8s/` checks (task touched none of them).
- Not fixed: the create gate still reads only Claude's cache. T022c makes it per-harness.
- Left: T022c to T026.

## Iteration 5 (T022c, 10/6/26)

- `createRefusedWhileSignedOut(ctx, h)` reads the create's own harness cache and returns false for `harness.Other` without reading one. `failSignedOut(w, r, h)` and the browser gate record `errCreateCodexSignedOut` for Codex, `errCreateSignedOut` otherwise, through a small `signedOutReason(h)` helper. `harnessOfStartName` added; an unknown name resolves to Claude.
- Both callers (`actions.go` browser gate, `sessions.go` API gate) resolve the harness from the start-command name. 503 and `bodySignedOut` unchanged.
- Tests written first (`TestCreateCodexGatedOnCodexAuth` on both doors, `TestCreateClaudeNotGatedOnCodex`, `TestCreateOtherNeverGated`, `TestHarnessOfStartName`); they failed to build before the change, then passed. No existing test needed editing.
- Not run: `-tags tmux`, `-tags quickstart`, `k8s/` checks (task touched none of them).
- Not fixed: the `Other` branch is covered by calling the predicate directly, not by a create through a plain-command start entry.
- Left: T023 to T026.

## Iteration 6 (T023, 10/6/26)

- `effectiveDisplayState` now asks `paneNeedsAuth(paneText, h)`: Claude uses `claudeauth.DetectPrompt`, Codex uses `codexauth.DetectPrompt`, Other checks both. The prompt is still dropped, so the card never carries the code or URL.
- Tests written first (`TestCodexSignedOutPaneRendersNeedsAuth`, `TestCodexDevicePaneRendersNeedsAuth`, `TestNeedsAuthIsPerHarness`); three subtests failed before the change, then all passed. They read the F3 and F4 panes straight from `internal/codexauth/testdata`.
- Learned: before the change a Codex session on the signed-out screen read `unknown`, and a Claude screen under a Codex session read `needs-auth`. Both are fixed by the split.
- Not run: `-tags tmux`, `-tags quickstart`, `k8s/` checks (task touched none of them).
- Not fixed: the fleet grid still captures no panes, so a Codex session needing sign-in reads `running` there until its own page is opened. Same scope cut as Claude (`TestTheFleetGridDoesNotYetCheckPanesForALogin`).
- Left: T024 to T026.

## Iteration 7 (T024, 10/6/26)

- `signin.go`: the three POST routes and `GET /dashboard/signin/view` read the harness with `parseHarness`; a bad value gets `rejectBadRequest(errHarnessParam)` before any relay or cache is touched. Handlers use `s.signins[h]`; a new `invalidateAuth(h)` guards the nil Codex cache.
- `signInPanel` gained `Harness`, `DeviceURL`, `Code`. `signInPanelFor` takes `h` and fills the Codex fields from `State.Device`.
- `POST /dashboard/signin/code` with `harness=codex` records `errSignInCodeNotTaken` and redirects with the refused outcome (no 409). `redirectSignIn(w, r, code, h)` writes `signin=codex` for Codex, `signin=open` for Claude.
- `signin-panel.html`: separate Codex block (link, `<code>` code, waiting line, cancel/start forms, no code form, no "Claude"). Every form in both blocks carries the hidden `harness` field. No new CSS class.
- Tests written first (nine Codex/SignIn tests plus two extras: `TestCodexRouteWithoutARelayIsRefused`, `TestCodexPanelWaitsWhileTheScreenDraws`, `TestCodexSignInViewReadsTheCodexRelay`); they failed to compile before the change, then passed. No existing test needed editing.
- Not run: `-tags tmux`, `-tags quickstart`, `k8s/` checks (task touched none of them).
- Not fixed: `crswd.js` still fetches `/dashboard/signin/view` with no harness and does not read `signin=codex`, so the Codex dialog has no way to open from the browser yet. T025 does that.
- Left: T025 to T026.

## Iteration 8 (T025, 10/6/26)

- `headerView{Operator, CodexConfigured}` and `(*Server).headerFor` in `view.go`. `fleetView`, `sessionPageView`, `settingsView` and `notFoundView` gained `Header`; all six non-test construction sites set `Header: s.headerFor(operator)`. The four page templates pass `.Header`; `header.html` reads `.Operator.Email` and draws the second pill (`data-harness="codex"`, `codex auth: checking`) only when `.CodexConfigured`. The `{{ if }}` sits inside the line break so the header is byte-identical without Codex.
- `crswd.js` auth module rewritten around one record per pill (`{pill, harness, timer, generation, lastState}`), module `activeHarness` and `dialogGeneration`, an `AbortController` per panel fetch, stale answers discarded, `?signin=codex` handled. Claude's `/dashboard/auth` URL is unchanged; forms reload `signin/view?harness=<activeHarness>` through `window.crswdReloadSignInPanel`.
- Tests written first: `header_test.go` (`TestHeaderViewOnEveryPage`, `TestHeaderCodexPillWhenConfigured`, `TestHeaderUnchangedWithoutCodex`, `TestHeaderForReadsTheCodexRelay`, `TestAuthScriptPerPillState`). They failed to compile before the change; after the Go and template work only the script test still failed, then passed. Existing `partials_test.go` and `render_test.go` edited mechanically (`headerView{Operator: ...}`, `Header:` on every view).
- No CSS change: `.masthead-bar .pill` already covers the second pill.
- Decision made where tasks.md is silent: only Claude's pill auto-opens the dialog on a transition to bad. Codex opens by click or `?signin=codex`. Also added: a poll that changes state reloads the open dialog when its harness is the active one.
- Not run: `-tags tmux`, `-tags quickstart` (T026's gate), `k8s/`. The script has no browser test; `node --check` passes and the test greps the file only.
- Not fixed: the JS behaviour (stale-answer discard, abort on second click) is unexercised by any test.
- Left: T026.

## NEEDS CLARIFICATION

(none yet)
