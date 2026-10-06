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

