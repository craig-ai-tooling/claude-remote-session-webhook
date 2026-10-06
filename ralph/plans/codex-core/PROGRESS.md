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

## Iteration 11 (T006)

- Wrote `TestSetModeRefusesCodex` in `harness_test.go` first; it failed on the old code. Then added the `m.specOf(s).Name == harness.Codex` guard to `SetMode` after the dead-state check, returning `ErrModeUnavailable`. Codex only; Other keeps today's behaviour. `mode_test.go` and `commandForMode` untouched.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Left: T007 onward. No findings unfixed.

## Iteration 12 (T007)

- Wrote `trust_codex_test.go` first (`TestCodexHome`, `TestWithCodexTrust` 17 rows with exact bytes, `TestSeedCodexTrust`); it failed to compile on the old code. Then added `trust_codex.go` (`CodexHome`, `SeedCodexTrust`, `withCodexTrust`, `ErrCodexConfigShape`, `ErrUntrustablePath`) and gave `replaceFile` a `prefix` parameter; its one caller passes `".claude.json.crswd-*"`. `TestSeedTrust` passes unedited.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: the "dir appears on another line" check is a plain substring match, so a header for `/ab` refuses when the session dir is `/a`. That follows the task text; a false refusal fails the create loudly rather than editing the wrong table.
- Nothing calls `SeedCodexTrust` yet; T008 wires it.
- Left: T008 onward. No findings unfixed.

## Iteration 13 (T008)

- Wrote `TestCreateCodexTrustsTheWorkDir`, `TestCreateCodexFailsOnShape` and `TestCreateClaudeDoesNotTouchCodexConfig` first (in `trust_codex_test.go`); they failed to compile on the old code (no `SetCodexHome`). Then added `Manager.codexHome`, `SetCodexHome` and `seedTrustFor` (Claude, Codex, Other seeds nothing), replaced both `SeedTrust` calls (`start` in manager.go, `sendStart` in supervisor.go) with `seedTrustFor(harness.For(harness.Of(template)), ...)`, and wired `SetCodexHome(session.CodexHome(sessionEnv))` in `httpapi.New` beside `SetClaudeConfig`, after the kubernetes-mode return.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: an Other start command now seeds no Claude trust where it used to. No existing test depended on it, since the fixtures use `claude`-prefixed stand-ins (T004).
- Left: T009 onward. No findings unfixed.

## Iteration 14 (T009)

- Wrote `TestRestartCodexQuitsThenTypes`, `TestRestartCodexNeverTypesWhenStillRunning`, `TestRestartCodexStopsWhenTheSessionVanishes`, `TestRestartCodexReturnsTheContextError`, `TestRestartClaudeUnchanged`, `TestContinueCodexFailedQuitChangesNothing` and `TestContinueCodexPersistsAfterQuit` first (in `harness_test.go`); they failed to compile on the old code (no `sleep` field). Then added `Fake.QuitAfterInterrupts`, `Manager.sleep` (`sleepContext`), `steppedQuitPresses`/`steppedQuitWait`, `ErrQuitUnconfirmed`, `quitStepped`, the `SteppedQuit` branch in `restartInto`, and the quit-before-persist branch in `Continue` (which then calls `sendStart`). Claude's path keeps one `SendKeys(C-c, C-c)`.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: a Codex fixture's fake pane reports `claude`, which is outside the Codex process set, so a test must `SetPaneCommand` to `codex` or `node` before the quit means anything. The Continue tests set HOME through `conversationHome`, so they are not parallel. They pass on Claude's transcript layout because `HasTranscript` is not yet per harness (T010b).
- Left: T010a onward. No findings unfixed.

## Iteration 15 (T010a)

- Wrote `conversation_codex_test.go` first (`TestCodexConversations`, `TestReadCodexMeta`, `TestCodexHasTranscript`, `TestCodexRolloutRejectsSymlinks`); it failed to compile on the old code. Then added `conversation_codex.go`: `codexConversations`, `codexHasTranscript`, `readCodexMeta`, `codexRollout`, the three limits, and two small helpers (`sortedSubdirs`, `sortConversations`).
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: listing sorts by mtime as `Conversations` does, so tests set mtimes with `Chtimes`; the walk order only decides which files fall inside the scan and list limits. The unreadable-day case is skipped when the tests run as root, since root reads a mode-0 directory.
- Added `//nolint` on the `os.Open` (gosec G304, path proven by `codexRollout`) and the read-only `Close`, following `journal.go` and `trust_codex.go`.
- Nothing calls these yet; T010b dispatches to them.
- Left: T010b onward. No findings unfixed.

## Iteration 16 (T010b, stopped)

- No code changed. Wrote the dispatchers (`ConversationsFor`, `hasTranscriptFor`, the `Continue` `ResumeArgs == nil` guard, both call sites) and three tests; the tests failed on the old code. Then `go test ./internal/session/...` failed in two tests outside T010b's Files line. Reverted every edited file with `git restore`; nothing is committed except this notebook.
- T010b is marked `- [!]`.

## NEEDS CLARIFICATION (5)

- **T010b**: quoted text: "Files: `internal/session/conversation.go` (dispatchers), `internal/session/supervisor.go` (`:197` HasTranscript gate), `internal/session/manager.go` (`Continue` `:2471`), `internal/session/conversation_codex_test.go`." and "Acceptance: `go test ./internal/session/...` passes." With `Continue` checking the Codex transcript for a Codex session, `TestContinueCodexFailedQuitChangesNothing` and `TestContinueCodexPersistsAfterQuit` (`internal/session/harness_test.go`, written in T009) fail with `ErrInvalidResume: there is no such conversation on this host`. Their `continueFixture` (`harness_test.go`) plants a Claude transcript with `conversationHome`, which iteration 14 noted only passed because `HasTranscript` was not yet per harness. `harness_test.go` is not in T010b's Files line. Question: may T010b also edit `continueFixture` in `internal/session/harness_test.go`? Suggested fix: add `internal/session/harness_test.go` (`continueFixture` only) to T010b's Files line. Verified in a scratch edit: replacing its `conversationHome(...)` line with `home := t.TempDir(); f.mgr.SetCodexHome(home); plantCodexRollout(t, home, harnessTestConversation, s.WorkDir)` turns both tests green. `plantCodexRollout` is a new helper in `conversation_codex_test.go` (`writeRollout` plus `codexMetaLine`).
- Also for the retry: `TestConversationsForDispatch`'s Claude row must use `f.repo()` as the work dir, not `/work/proj`, because `Conversations` resolves the directory against the approved roots and `/work/proj` is outside them. That is inside T010b's Files and needs no decision.

- Operator (CS, 2026-10-06): T010b may edit `continueFixture` in `internal/session/harness_test.go` as suggested (Codex rollout via `plantCodexRollout`). T010b unblocked.

## Iteration 17 (T010b)

- Wrote `TestConversationsForDispatch`, `TestContinueOtherRefusedEarly` and `TestContinueCodexChecksCodexTranscript` first, plus the `plantCodexRollout` helper in `conversation_codex_test.go`; they failed to compile on the old code. Then added `Manager.ConversationsFor` and `hasTranscriptFor` (exhaustive switches, no Claude fall-through) in `conversation.go`, replaced the calls in `supervisor.go` and `Continue`, and put the `ResumeArgs == nil` guard first in `Continue`, ahead of any store, option, journal or pane change.
- `continueFixture` (`harness_test.go`) now plants a Codex rollout through `plantCodexRollout`, per the operator decision; the two T009 Continue tests pass on it.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Nothing outside the session package calls `ConversationsFor` yet; T014/T015 consume it. `Conversations` and `HasTranscript` are unchanged.
- Left: T011 onward. No findings unfixed.

## Iteration 18 (T011)

- Wrote `discover_test.go` first (`TestDiscoverCodexConversation`, 14 rows, plus the empty `sessionsDir` case); it failed to compile on the old code. Then added `discover.go`: `DiscoverCodexConversation`, the two errors, the four limits, and two helpers (`discoverChildren`, `discoverRollouts`). BFS reads pids at depth 0 to 6 and follows children only below depth 6.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: the 64-pid cap counts the pane pid, so a root with 64 children already overflows. The task text does not say; the row with 65 children errors under either reading. Test dirs use 0o750 for gosec G301.
- Nothing calls `DiscoverCodexConversation` yet; T012 wires it.
- Left: T012 onward. No findings unfixed.

## Iteration 19 (T012)

- Wrote the `TestSweep*` tests and `TestReplayRestoresDiscoveredConversation` first (in `discover_test.go`, with a `sweepRig` helper); they failed to compile on the old code. Then added `journalDiscovered` (journal.go), `Manager.findCodexConversation` defaulted to `hostCodexConversation` in `NewManagerWithClock` (manager.go), and `Supervisor.discover`, called from `judge` branch 3 after the promotion logic (supervisor.go). Write order is option, journal, store, emit.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: the Codex fixture's fake pane reports `claude`, so a sweep judges it stopped and gives up; the rig sets the pane command to `codex`. The replay test needed no code change, since `reviveRecord` already carries `Conversation`. I added `TestSweepRefusesAnInvalidDiscoveredID` beyond the task's list to cover the `ValidateResume` branch.
- A discovery error is returned from `judge` and joined by `Sweep`, but leaves the verdict healthy.
- Left: T013 onward. No findings unfixed.

## Iteration 20 (T013)

- Wrote `TestDetectDialogForCodex`, `TestCodexIdleAndWorkingAreNotDialogs`, `TestDetectDialogForClaudeIsUnchanged`, `TestDetectDialogForOtherFallsBack` and (httpapi) `TestCodexPaneOnTrustRendersBlocked` first, plus the six `testdata/*.pane` fixtures and `testdata/README.md`; the session tests failed to compile on the old code (no `DetectDialogFor`). Then added `codexDialogSignatures`, `codexSuspiciousMarkers`, `DetectDialogFor` (dialog.go) and `Manager.SpecOf` (manager.go), gave `cardOf` and `effectiveDisplayState` a harness parameter, and pointed the fleet card, the session page and `paneDialogState` at it. The three `cardOf` calls in `partials_test.go` pass `harness.Claude`.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: goimports in the edit hook strips an import that is not yet used, so add the import and its first use in one edit. The fixture read needs `//nolint:gosec` (G304), as in iteration 15.
- The fleet grid still passes an empty pane, so Codex cards read `running` there until the session page opens, same as Claude.
- Left: T014 onward. No findings unfixed.

## Iteration 21 (T014)

- Wrote `TestParseHarness`, `TestStartCommandLineFor` and the six `TestBrowserCreate*` harness tests first; they failed to compile on the old code (no `parseHarness`, `StartCommandLineFor`, `codexStartCommandName`). Then added `harnessparam.go` (`parseHarness`, `errHarnessParam`, the field and value constants), `Manager.StartCommandLineFor`, `createFormView.CodexCommand` with `Server.codexOffered` and `previewCodexCommand`, and the harness block in `createFromBrowser`, placed after the remote-control check and ahead of the lifetime parse.
- Gate green: gofmt, build, vet, `go test ./...`, `go test -tags tmux ./internal/session/... ./internal/tmuxctl/...`, golangci-lint. quickstart suite not run: nothing in `cmd/crswd` changed.
- Learned: a refused harness create reuses `errCreateStateNotOffered` and `outcomeBadMode`; Codex plus remote uses `errModeUnavailable` with `outcomeBadMode`, per the task, where the Claude no-remote-command case answers `outcomeCreateFailed`. `parseHarness` treats a present key with zero entries as refused. An empty `codexHome` seeds nothing, so httpapi tests need no HOME override.
- Nothing renders `CodexCommand` yet; T014a and T015 add the control and the preview.
- Left: T014a onward. No findings unfixed.

## Iteration 22 (T014a, stopped)

- No code changed. T014a is ambiguous; marked `- [!]` in IMPLEMENTATION_PLAN.md.

## NEEDS CLARIFICATION (6)

- **T014a**: quoted text: "Files: `docs/components.md` only ... The CSS rules for this contract are written in T015, together with the first markup that uses them: `TestTheStylesheetAndTheMarkupNameTheSameThings` ... fails on any class no template renders, so CSS without markup cannot be green" and "Tests: `TestRadioGroupStylesExist` (served `crswd.css` contains `.radio-group`, `.radio-option` and `:focus-visible` for `.radio-option input`)." and "Acceptance: `go test ./internal/httpapi/... -run RadioGroup` passes." The task contradicts itself. The test asserts CSS that the task says is not written until T015, and writing the CSS here would turn `TestTheStylesheetAndTheMarkupNameTheSameThings` red (no template renders `.radio-group`). Also no httpapi test file is in the Files line, and `grep -rn 'RadioGroup\|radio-group' internal web docs` prints nothing today. As written, the acceptance command either matches no test (exits 0 with "no tests to run", proving nothing) or the test cannot pass. Question: should T014a drop `TestRadioGroupStylesExist` and run as docs-only (acceptance becomes a grep for the `radio-group` section in `docs/components.md`), moving the CSS test into T015 beside the CSS and the markup? Suggested fix: edit T014a's Tests and Acceptance to a docs-only check, and add `TestRadioGroupStylesExist` to T015's Tests (partials_test.go is already in T015's Files). Then update the IMPLEMENTATION_PLAN T014a line's "Done when" to match.

- Operator (CS, 2026-10-06): T014a is docs-only; `TestRadioGroupStylesExist` moved to T015 with the CSS. T014a unblocked.

## Iteration 23 (T014a)

- Docs only, per the operator decision. Added the "Radio group" section to `docs/components.md` after Switch: the markup contract verbatim, a class table, and the rules (native radio in label, no ARIA, one `checked`, `accent-color: var(--phosphor)`, global focus ring, `--tap` row, wraps under the phone breakpoint). No test, no CSS, no template.
- Gate green: gofmt, build, vet, `go test ./...`, golangci-lint. `grep -n 'Radio group' docs/components.md` prints the heading. tmux and quickstart suites not run: nothing in tmuxctl, session or cmd changed.
- Finding, not fixed: the task says disabled options use "the switch's disabled opacity rule", but `crswd.css` has no switch disabled rule. The doc names `.button:disabled` (`--dim` text, default cursor, no opacity) as the nearest existing one. T015 should write the radio rule that way unless the operator says otherwise.
- The task's `min-height 44px` is written in the doc as `min-block-size: var(--tap)`, the token the switch row uses.
- Left: T015 onward.
