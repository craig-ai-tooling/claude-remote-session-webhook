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
