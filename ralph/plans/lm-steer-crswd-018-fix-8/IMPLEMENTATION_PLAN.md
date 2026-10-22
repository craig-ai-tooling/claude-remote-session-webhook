# Implementation plan: a Claude prompt is one bracketed paste plus one Enter (lm-steer-crswd-018-fix-8)

Filed by `loop/fix-features.py` from a verify-run.sh FAIL (run 1014-la1i). Parent
task: lm-steer-crswd-018. The parent's contract says a two-line prompt to a Claude
session arrives as one bracketed paste followed by exactly one Enter. Today
`Manager.Prompt` routes through `Manager.paste`, which is bracketed only when the
harness spec says so, and Claude's `BracketedPaste` is false. So a Claude prompt is
a plain paste, and `TestPromptClaudeUnchanged` (`internal/session/harness_test.go:237`)
pins that wrong behaviour.

This is the parent's spec 020 task T006 (FR-009, FR-010, FR-011), done on its own.
Read `specs/020-prompt-route-safety/tasks.md` section "T006" and spec.md FR-009 to
FR-011 before task 1. Do not edit anything under `specs/`.

## Decisions already made (do not reopen)

- **D1** `Manager.Prompt` calls `m.tmux.PasteBracketed` for every harness. `Manager.paste`,
  `specOf`, the harness specs and `Manager.Compact` are untouched. `paste` now serves
  `Compact` only; its doc comment says so and names spec 020 FR-009 as the reason.
- **D2** A newline is never a reason to refuse a prompt (FR-011).
- **D3** `TestPromptClaudeUnchanged` is replaced, not deleted: it becomes a test that a
  two-line Claude prompt is exactly load-buffer, `paste-buffer -p`, one Enter.
- **D4** tmux adds the bracket markers only when the pane's program has requested
  bracketed-paste mode (DECSET 2004). The quickstart shim never requests it, so
  its `first\nsecond` case is unaffected.
- **D5** The real-tmux test starts no real harness. Its pane program requests mode 2004
  itself and echoes control bytes visibly.

## Pre-commit commands (every task, in this order)

1. `gofmt -l .` prints nothing.
2. `go build ./...`
3. `go vet ./...`
4. `go vet -tags tmux ./...`
5. `go vet -tags quickstart ./cmd/crswd`
6. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run`
7. `test ! -e go.sum`
8. `git status --short` lists only paths in "Files touched" or the notebook directory.

## Conventions

- `- [ ]` open, `- [x]` done, `- [!]` blocked (reason in PROGRESS.md).
- The loop takes the topmost open task. The order is the dependency order.
- PROGRESS.md "Code notes" lists every line this milestone must change, with file and line.

## Tasks

- [ ] T1 Prompt pastes bracketed for every harness (D1, D3). Replace TestPromptClaudeUnchanged with a two-line case first and see it fail, then change Prompt and fix the four other pins in PROGRESS.md "Code notes". Verify: `go test ./internal/session/ ./internal/httpapi/ ./internal/audit/` exits 0, then `go test ./...` exits 0.
- [ ] T2 New `internal/session/prompt_tmux_test.go` (`//go:build tmux`): a pane that turns on mode 2004 and runs `cat -v` gets a two-line Prompt; assert one `^[[200~`, one `^[[201~`, both lines (D5, PROGRESS.md "T2 recipe"). Verify: `go test -tags tmux ./internal/session/ -run Prompt -v` exits 0 and prints a PASS for it.
- [ ] T3 Append one line to `docs/fixes-log.md` in its existing format: a Claude prompt is now one bracketed paste plus one Enter, TestPromptClaudeUnchanged replaced. Verify: `tail -1 docs/fixes-log.md | grep -q bracketed` exits 0.
- [ ] F1 Final gate: run every assertion in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go test -tags tmux ./...` exits 0 and `golangci-lint run` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-8/` is always in scope):

```allowlist
internal/session/manager.go
internal/session/manager_test.go
internal/session/harness_test.go
internal/session/prompt_tmux_test.go
internal/httpapi/sessions_test.go
internal/audit/leak_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, anything under `specs/`, `k8s/`,
`internal/harness/`, `internal/tmuxctl/`, `internal/session/input.go`, `go.mod`, any new
directory under `internal/`.

## After the loop (a session does these, not Ralph)

1. Open the PR from this branch.
2. Tell the parent milestone lm-steer-crswd-018 that spec 020 T006 has landed here, so its loop ticks T006 instead of redoing it.
