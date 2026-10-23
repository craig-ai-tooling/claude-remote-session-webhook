# Implementation plan: concurrent prompts use distinct buffers and never interleave (lm-steer-crswd-018-fix-11)

Fix lane, filed from a verify-run FAIL on parent milestone `lm-steer-crswd-018`
(run 1014-la1i). The behaviour already exists in the code. What is missing is a
test that sends prompts concurrently. This milestone adds tests only, plus one
`docs/fixes-log.md` line. No production Go file changes.

## Facts measured at planning time (10/23/26, main `ebefd36`)

- `go test -race ./internal/session/ -run 'Prompt|Interleave'` exits 0 on the baseline.
- `grep -rn 'TwoConcurrent\|ConcurrentPastes' internal` prints nothing. Neither new test exists yet.
- `Manager.Prompt` is in `internal/session/manager.go` (near line 1194). It takes
  `unlock := m.lockSession(s.ID)` then `defer unlock()`, then calls `m.paste` (load-buffer +
  paste-buffer) and `m.tmux.SendKeys(ctx, name, enterKey)`. The lock is what serialises two prompts.
- For a Claude session, `m.paste` uses plain `Paste`. The fake records two `tmuxctl.OpPaste` calls
  (load-buffer, then paste-buffer) and one `tmuxctl.OpSendKeys` whose last Argv element is `"Enter"`.
  See `TestPromptPastesThenSubmits` in `internal/session/manager_test.go` (near line 1149) for the
  exact Argv shapes:
  - load-buffer: `{"tmux","load-buffer","-b",<buffer>,"-"}`, payload in `Call.Stdin`. Buffer is `Argv[3]`.
  - paste-buffer: `{"tmux","paste-buffer","-d","-b",<buffer>,"-t",<pane>}`. Buffer is `Argv[4]`.
- The fake names buffers `tmuxctl.FakeBufferName(n)` from a counter, standing in for `Exec`'s random
  `newBufferName` in `internal/tmuxctl/exec.go` (near line 189).
- Model for the session test: `TestInputOnOneSessionNeverInterleaves` in `internal/session/input_test.go`
  (near line 380). It uses `newManagerFixture(t)`, `mustCreate(t, f, f.request())`,
  `NewManagerWithClock(slowEnter{f.tmux}, f.store, f.roots(), capNotUnderTest, stoppedClock{now: f.now})`
  and replaces `f.mgr`. `slowEnter` is defined in that same file and is visible to `manager_test.go`.
- Model for the tmux test: `TestTmuxPasteDeliversHostileTextByteForByte` in
  `internal/tmuxctl/exec_tmux_test.go` (near line 461): `newTestExec(t)`, `e.New`, `SendKeys "cat > "+sink`
  then `"Enter"`, `waitFor(...)`, `e.Paste`, read the sink. Session names there are `crswd-` plus 32 hex.
- `tmux 3.4` is at `/usr/bin/tmux` on the planning host. `golangci-lint` is at
  `/home/nctiggy/bin/golangci-lint`. Earlier loops recorded both missing inside the loop container.
- The parent plan's T010 (restart claim inside `Prompt`) is not done on `main`. It is out of scope here.

## Decisions already made (do not reopen them)

- **H1 Session test.** `TestTwoConcurrentPromptsNeverInterleave` in `internal/session/manager_test.go`,
  `t.Parallel()`, fake tmux wrapped in `slowEnter`. Two goroutines, 10 prompts each, every payload distinct
  (for example `fmt.Sprintf("prompt %d-%d", g, i)`), joined by a `sync.WaitGroup`. Assert: every `Prompt`
  returns nil; the recorded calls after `before` are exactly 20 triples of load-buffer, paste-buffer, Enter
  in that order; each paste-buffer's buffer equals the load-buffer's directly before it; all 20 buffer
  names are distinct; each payload appears exactly once across load-buffer `Stdin`.
- **H2 Failing-before proof for H1.** Temporarily delete the two lines `unlock := m.lockSession(s.ID)` and
  `defer unlock()` from `Prompt` in `internal/session/manager.go`, run the test, copy the failure line,
  then restore with `git checkout -- internal/session/manager.go`. That file must not appear in the commit.
- **H3 tmux test.** `TestTmuxConcurrentPastesLandWhole` in `internal/tmuxctl/exec_tmux_test.go`, behind the
  file's existing `tmux` build tag, no `t.Parallel()` (the file's rule). One session running `cat > sink`;
  eight goroutines each call `e.Paste` with a distinct payload ending in `"\n"`; collect errors through a
  channel or a mutex-guarded slice. Assert: no error; the sink eventually holds 8 lines; each payload's line
  appears exactly once. Kill the session at the end.
- **H4 Failing-before proof for H3.** Temporarily make `newBufferName` in `internal/tmuxctl/exec.go` return
  `BufferPrefix + "shared", nil`, run the test with `-count=5`, copy a failure line, then restore with
  `git checkout -- internal/tmuxctl/exec.go`. If five runs never fail, record that in PROGRESS.md as the
  measured result and do not block.
- **H5 Out of scope.** Any production code change, the restart claim (parent T010), the parent plan's boxes,
  and anything under `specs/`.

## Pre-commit commands (every task, in this order)

1. `gofmt -l .` prints nothing.
2. `go build ./...` exits 0.
3. `go vet ./...` exits 0.
4. `go vet -tags tmux ./...` exits 0.
5. `go vet -tags quickstart ./cmd/crswd` exits 0.
6. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` exits 0. If golangci-lint is not installed, record that in PROGRESS.md.
7. `test ! -e go.sum` exits 0.
8. `git diff --name-only HEAD -- internal/session/manager.go internal/tmuxctl/exec.go` prints nothing.
9. `git status --short` lists only paths in "Files touched" or this notebook directory.

## Tasks

- [ ] T1 session: add `TestTwoConcurrentPromptsNeverInterleave` per H1 in `internal/session/manager_test.go`, proving it fails per H2 first. Verify: `go test -race ./internal/session/ -run 'Prompt' -v 2>&1 | grep -q -- '--- PASS: TestTwoConcurrentPromptsNeverInterleave'` exits 0.
- [ ] T2 tmuxctl: add `TestTmuxConcurrentPastesLandWhole` per H3 in `internal/tmuxctl/exec_tmux_test.go`, proving it fails per H4 first. Verify: `go test -tags tmux ./internal/tmuxctl -run 'Concurrent' -v 2>&1 | grep -q -- '--- PASS: TestTmuxConcurrentPastesLandWhole'` exits 0.
- [ ] T3 docs: append one line dated 2026-10-23 to `docs/fixes-log.md` in its existing format: concurrent prompts into one session are now tested to use distinct buffers and never interleave; no code changed. Use the word `interleave` and end with `(lm-steer-crswd-018-fix-11)`. Verify: `grep -n 'interleave' docs/fixes-log.md | grep -q 'lm-steer-crswd-018-fix-11'` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go test -race ./internal/session/ -run Prompt && go vet -tags tmux ./...` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-11/` is always in scope):

```allowlist
internal/session/manager_test.go
internal/tmuxctl/exec_tmux_test.go
docs/fixes-log.md
```

`internal/session/manager.go` and `internal/tmuxctl/exec.go` are edited only temporarily for the
failing-before proofs (H2, H4) and must be restored before commit.

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `specs/`, any non-test `.go` file,
`go.mod`, `go.sum`, any new directory under `internal/`.
