# Progress — Phase 1 — Codex core parity

Notebook for `ralph/plans/codex-core`. Each iteration appends below.

## Iteration 0 (planning, 10/6/26)

- Plan written from `specs/019-codex-runtime/`. No code changed.
- Measurements behind every Codex behaviour are in `specs/019-codex-runtime/research.md` section M.
- Left: every task in `IMPLEMENTATION_PLAN.md`, starting with T001.

## Iteration 1 (T001)

- Added `internal/harness` (`harness.go`, `harness_test.go`): `Name`, `Of`, `Label`, `Spec`, `For`, values per plan.md's table. Stdlib only.
- Tests written first and failed to compile (no package); after the implementation `go test ./internal/harness/...` passes. Full gate green (gofmt, build, vet, test, golangci-lint). No tmux or quickstart suite needed: nothing in tmuxctl, session or cmd changed.
- Nothing calls the package yet; T002 onward consume it.
- Left: T002 onward. No findings unfixed.

## Iteration 2 (T002, stopped)

- No code changed. T002 is ambiguous; marked `- [!]` in IMPLEMENTATION_PLAN.md.

## NEEDS CLARIFICATION

- **T002**: quoted text: "Files: `internal/tmuxctl/fake.go`, `internal/tmuxctl/controller.go` (comments on `OptionBinary` ... only), `internal/tmuxctl/fake_test.go`, `internal/tmuxctl/exec_test.go` is not listed" and "`argvList` liveness expression becomes `#{?#{@crswd-binary},#{m/r:^(#{@crswd-binary})$,#{pane_current_command}},?}`". Changing that expression breaks `TestExecSessionLifecycle`-style argv assertion at `internal/tmuxctl/exec_test.go:284` (default build, literal contains the old `#{==:...}` expression). `internal/tmuxctl/fake_test.go:83` has the same literal (that file is listed). The acceptance line requires `go test ./internal/tmuxctl/...` to pass, which is impossible without editing `exec_test.go`, and the Files line forbids it. Also, `argvList` itself lives in `fake.go` but the real `Exec.List` parses it in `exec.go`; no change needed there. Question: may T002 also edit the literal at `internal/tmuxctl/exec_test.go:284` (one-line change, `internal/tmuxctl/` is inside the allowlist)? Suggested fix: add `internal/tmuxctl/exec_test.go` to T002's Files line in tasks.md.


- Operator (CS, 2026-10-06): T002 Files line now includes `internal/tmuxctl/exec_test.go` (the `:284` literal). T002 unblocked.

## Iteration 3 (T002, stopped again)

- Wrote the tests, confirmed they failed on the old code, implemented `argvList`/`livenessOf`, and `go test ./internal/tmuxctl/...` and `go test -tags tmux ./internal/tmuxctl/...` both passed. Then `go test ./...` failed in another package. Reverted all five edited files with `git restore`; nothing is committed except this notebook.
- T002 is marked `- [!]` again.

## NEEDS CLARIFICATION (2)

- **T002**: quoted text: "Files: `internal/tmuxctl/fake.go`, `internal/tmuxctl/controller.go` (comments on `OptionBinary` ... only), `internal/tmuxctl/fake_test.go`, `internal/tmuxctl/exec_tmux_test.go`, `internal/tmuxctl/exec_test.go` (the argv literal at `:284` only ...)". Changing the `argvList` liveness expression breaks `TestAdoptTakesBackASurvivingSessionWithAFreshCredential` at `internal/session/manager_test.go:2311` (default build), which asserts the old `#{==:#{pane_current_command},#{@crswd-binary}}` literal. The gate requires `go test ./...` green and the Files line does not list `internal/session/manager_test.go`. `internal/session/` is inside the allowlist. Question: may T002 also edit the literal at `internal/session/manager_test.go:2311` (one-line change, same text as the other two)? Suggested fix: add `internal/session/manager_test.go` (the argv literal only) to T002's Files line in tasks.md. I grepped `internal`, `cmd` and `docs` for the old expression: only that file and a stale comment at `exec_tmux_test.go:740` (in the Files line) still carry it. When T002 reruns, also refresh that comment.

- Operator (CS, 2026-10-06): pre-scan of T003+ for Files-line gaps. Patched T002 (manager_test.go :2302), T004 (fixture stand-ins get a `claude ` prefix), T014a (CSS moves to T015), T015, T016 (crswd.css, server_test.go frozenEntry, mode-row guard). T002 unblocked.

## Iteration 4 (T002)

- Wrote `TestLivenessOfSet` (fake_test.go) and `TestLivenessAlternatives` (exec_tmux_test.go, tmux tag) first; both failed on the old code. Then changed the `argvList` liveness expression to the anchored `m/r` form, rewrote `livenessOf` to split on `|` and skip empty elements, and documented the set form on `OptionBinary`. Updated the old argv literal in `exec_test.go:284`, `fake_test.go:83`, `session/manager_test.go:2302` and the stale comment in `exec_tmux_test.go`.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: the real tmux evaluates `m/r:^(codex|sleep)$` as alternation, confirmed by the tmux-tagged test. `.` in a name matches any character, as the task accepts.
- Left: T003 onward. No findings unfixed.

## Iteration 5 (T003)

- Precondition held: `PasteBracketed` is in `controller.go`, so spec 018 has landed. `k8s/internal/podctl` does not exist, so nothing under `k8s/` was touched.
- Wrote `TestArgvPanePID`, `TestFakePanePID` (seeded, unseeded, missing session, injected failure) and the tmux-tagged `TestPanePIDIsTheShell` first; they failed to compile on the old code. Then added `Controller.PanePID`, `Exec.PanePID`, `Fake.PanePID`/`SetPanePID`, `OpPanePID`, `argvPanePID`/`ArgvPanePID` and `ErrUnexpectedOutput` (no existing sentinel had that meaning).
- Gate green: gofmt, build, vet, `go vet -tags tmux`, `go test ./...`, `go test -tags tmux ./...`, golangci-lint. `TestPanePIDIsTheShell` ran and passed, it was not skipped. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: the Fake answers an unseeded pid as `ErrUnexpectedOutput`, so a test that walks `/proc` must name its pid. A missing session returns the same message `Fake.CapturePane` gives.
- Nothing calls `PanePID` yet; T011/T012 consume it.
- Left: T004 onward. No findings unfixed.

## Iteration 6 (T004)

- Wrote `internal/session/harness_test.go` first (`TestRenderStartPerHarness`, `TestWithRequiredFlags*`, `TestPaneProcesses`, `TestCreateCodexWritesBinarySet`, `TestClaudeFlagConstantsUnchanged`); it failed to compile on the old code. Then added `paneProcesses`, `withRequiredFlags` and `containsRun` to `conversation.go`, made `conversationCapable` ask the harness (deleted `claudeBinary`), rewrote `resumeFlagged` around `harness.Spec`, and switched both `OptionBinary` writes (`start`, `markSession`) to `paneProcesses`.
- Fixture stand-ins changed to `claude local-command` / `claude remote-command` in `session/manager_test.go` and `httpapi/actions_test.go`. No assertion needed editing: the tests build their expected lines from the constants. Refreshed three comments that cited `claudeBinary`.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed. Acceptance grep for `"claude"` outside tests prints nothing.
- Learned: a Claude stand-in now mints a conversation on create, which the old `local-command` did not; no existing test depended on the absence. `Other` now refuses a resume with `ErrInvalidResume`; before, it got `--resume` inserted.
- Not added: `specOf` (the Interface lists it, no caller yet; T005/T006 will add it with one).
- Nothing in `manager.go` calls `harness` beyond `resumeFlagged`; T005 onward consume the rest. `partials_test.go:4110` still holds an unprefixed `local-command` literal, outside T004's Files line and passing.
- Left: T004a onward. No findings unfixed.

## Iteration 7 (T004a)

- Wrote `codexflags_test.go` first (`TestCodexUpdateCheckValues`, 14 rows, `TestLoadStartCommandsRefusesCodexUpdateCheck`). The Load test failed before `validateStartCommand` called the new check. Then added `codexflags.go` (`ErrCodexUpdateCheck`, `codexUpdateCheckValues`, `validateCodexUpdateCheck`) and made `validateStartCommand` return it last.
- Gate green: gofmt, build, vet, `go test ./...`, golangci-lint. tmux and quickstart suites not run: nothing in tmuxctl, session or cmd changed.
- Learned: a key that only starts with `check_for_update_on_startup` is not matched, since the key must equal it exactly. A trailing `-c` with no value is ignored.
- Left: T005 onward. No findings unfixed.

## Iteration 8 (T005)

- Wrote `TestPromptCodexIsBracketed`, `TestPromptClaudeUnchanged`, `TestCompactCodexSubmitsWithEnter`, `TestCompactClaudeUnchanged` and `TestTypeStaysBracketedForClaude` first; the two Codex tests failed on the old code. Then added `Manager.specOf` and `Manager.paste`, pointed `Prompt` at `paste`, and gave `Compact` a Codex branch (bracketed `/compact`, then Enter). Claude's `Compact` is untouched.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: `specOf` reads the start-command name from the record and resolves it through the configured set. A name no longer configured reads as `Other`, so it gets plain paste.
- `Type` is not routed through `paste`.
- Left: T006 onward. No findings unfixed.

## Iteration 9 (T006, stopped)

- No code changed. T006 is ambiguous; marked `- [!]` in IMPLEMENTATION_PLAN.md.

## NEEDS CLARIFICATION (3)

- **T006**: quoted text: "Files: `internal/session/manager.go` (`SetMode` `:1297`), `internal/session/mode_test.go`." and "Tests: `TestSetModeRefusesCodex` (fake records zero SendKeys calls), existing mode tests unedited." and "Acceptance: `go test ./internal/session/... -run Mode` passes." `internal/session/mode_test.go` begins with `//go:build tmux` and holds only real-tmux round-trip tests, so a test added there is invisible to the default-build acceptance command and cannot be proven to fail on the old code. A fake-based test belongs in a default-build file. `SetMode`'s existing fake tests (`TestSetModeRefusesWhatItCannotCarryOut`, `codexFixture`) live in `manager_test.go` and `harness_test.go`, neither listed. Question: which default-build file may hold `TestSetModeRefusesCodex`? Suggested fix: change T006's Files line to name `internal/session/harness_test.go` (it already has `codexFixture`) instead of `mode_test.go`, and keep the acceptance as is. The `manager.go` change is one line and unambiguous (`specOf(s).RemoteControl` check after the dead-state check).

- Operator (CS, 2026-10-06): T006 test goes in `internal/session/harness_test.go` (default build). T006 unblocked.

## Iteration 10 (T006, stopped)

- Wrote `TestSetModeRefusesCodex` in `harness_test.go` (failed on old code) and added the `specOf(s).RemoteControl` guard after the dead-state check. Default build, vet and lint were green. Then `go test -tags tmux ./internal/session/...` failed. Reverted both edited files with `git restore`; nothing is committed except this notebook.
- T006 is marked `- [!]`.

## NEEDS CLARIFICATION (4)

- **T006**: quoted text: "Tests: `TestSetModeRefusesCodex` (fake records zero SendKeys calls), existing mode tests unedited." and "Files: `internal/session/manager.go` (`SetMode` `:1297`), `internal/session/harness_test.go`". With the guard in, `TestTogglePreservesSessionAndScrollback`, `TestToggleRestartsWithoutTheContinueFlag` and `TestToggleKeepsIdentifierAndLifetime` (`internal/session/mode_test.go`, `tmux` tag) fail with `ErrModeUnavailable`. Their fixture uses `modeLocalCommand = "seq -f ... 1 200"` and `modeRemoteCommand = "echo crswd-remote-marker"` (`mode_test.go:56-57`). Neither starts with `claude`, so `harness.Of` resolves them to `Other`, whose `RemoteControl` is false. The gate requires `go test -tags tmux ./...` green for `internal/session`, and `mode_test.go` is not in T006's Files line. Same class as the T004 fixture stand-ins. Question: may T006 also edit `internal/session/mode_test.go`, the two constants only? Suggested fix: add `internal/session/mode_test.go` (the two command constants only) to T006's Files line, and give them a `claude ` prefix the way T004 did: `"claude seq -f ..."` would not run, so use a form that still runs in a shell, e.g. `modeRemoteCommand = "claude=1 echo " + remoteMarker`, or a tiny stand-in script named `claude`. The operator should pick which, since `harness.Of` keys on the first token's basename. Also check T008+ tmux-tagged fixtures in `internal/session` for the same stand-ins before the next unblock.

- Operator (CS, 2026-10-06): T006 and FR-012a narrowed to Codex only (`Name == harness.Codex`); Other keeps today's behaviour per the spec non-goal, so the tmux mode tests stay valid. T006 unblocked.
