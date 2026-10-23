# Implementation plan: prompt during a revive or continue (lm-steer-crswd-018-fix-10)

Fix lane, filed from a verify-run FAIL on parent milestone `lm-steer-crswd-018`
(run 1014-la1i). The parent's decision D6 (task T010) says `Manager.Prompt` takes
the session's restart claim with `claimRestart` after `lockSession`, refuses with
`ErrSessionNotReady` when a revive or a continue holds it, and releases it on
return. Decision D3 (task T005) says the prompt handler maps `ErrSessionNotReady`
to `409` `{"error":"session not ready"}`. On this branch, cut from `main` at
`ebefd36`, none of that exists.

## Facts measured at planning time (10/23/26, `main` at `ebefd36`)

- `go test -race ./internal/session/ ./internal/httpapi/ -run Prompt` exits 0 on the baseline. It passes only on tests that predate the rule.
- `grep -rn 'ErrSessionNotReady\|session not ready' internal/` prints nothing.
- `Manager.Prompt` is in `internal/session/manager.go` (near line 1194). Its order today:
  the empty-ID guard, the `StateDead` guard, the empty-text guard, then
  `unlock := m.lockSession(s.ID)` / `defer unlock()`, then `m.paste` and
  `m.tmux.SendKeys(ctx, name, enterKey)`. It never calls `claimRestart`.
- `claimRestart(id) bool` and `releaseRestart(id)` are in `internal/session/manager.go`
  (near lines 2674 and 2688). Both are non-blocking. `claimRestart` returns false when the
  claim is already held. `releaseRestart` deletes the claim unconditionally, so calling it
  after a failed claim would steal a revive's claim.
- `Continue` (near line 2721) takes `claimRestart` first, then `lockSession`. The supervisor's
  revive takes `claimRestart` in `internal/session/supervisor.go` (near line 181). Prompt takes
  the lock first and the claim second, without blocking, so the opposite order cannot deadlock.
- The session sentinels are in a `var (...)` block in `internal/session/session.go`.
  `ErrRestartInFlight` (near line 667) is the neighbour the new sentinel goes beside.
- Session test fixture, package `session` (internal test): `newManagerFixture(t)` gives
  `f.mgr`, `f.tmux` (`*tmuxctl.Fake`). `mustCreate(t, f, f.request())` gives a live `*Session`.
  `f.tmux.Calls()` is the recorded command list. `TestPromptRefusesWhatItCannotDeliver` and
  `TestPromptPastesThenSubmits` in `internal/session/manager_test.go` (near lines 1149 and 1191)
  are the models. `TestOneRestartAtATime` in `internal/session/supervisor_test.go` shows a test
  calling `mgr.claimRestart` and `mgr.releaseRestart` directly. `manager_test.go` already imports `sync`.
- One Claude prompt on the fake is three calls: `OpPaste` `load-buffer -b <buf> -` (payload on
  `Stdin`), `OpPaste` `paste-buffer -d -b <buf> -t <pane>`, `OpSendKeys` `send-keys -t <pane> -- Enter`.
  `tmuxctl.FakeBufferName(n)` is the fake's n'th buffer name.
- `refusePrompt` is in `internal/httpapi/sessions.go` (near line 867). Today it maps
  `session.ErrEmptyPrompt` to 400 and everything else to `s.failInternal(w, r, errPromptUndelivered)` (500).
  Response bodies are package-level `[]byte`s near the top of that file (`bodyInternalError`
  line 21, `bodyTeardownUnverified` line 32).
- The httpapi handler cannot hold a restart claim from its tests: `claimRestart` is unexported
  in another package and the tmux fake has no way to pause a revive. So the 409 test drives
  `refusePrompt` through the audit middleware the way `TestAPanickingHandlerStillProducesARecord`
  (`internal/httpapi/middleware_test.go` near line 860) drives its handler:
  `s := newAuditedServer(t)`, a request signed with `signRequest(t, req, body, testTime)`,
  `s.authenticate(audit.ActionSessionPrompt, handler).ServeHTTP(rec, req)`, then `s.only(t)`
  returns the one audit record as `map[string]any` (`rec["decision"]`, `rec["reason"]`).
- Sibling plans `plan/lm-steer-crswd-018-fix-4` and `-fix-7` (not merged) also add
  `ErrSessionNotReady` and the 409 mapping. Whichever lands first defines them; the others reuse them.
- `golangci-lint` is at `/home/nctiggy/bin/golangci-lint` on the planning host. Earlier loops
  recorded it missing inside the loop container.

## Decisions already made (do not reopen them)

- **R1 Sentinel.** In the `var (...)` block of `internal/session/session.go`, beside
  `ErrRestartInFlight`: `ErrSessionNotReady = errors.New("session not ready")`, with a why-comment
  (the session exists but cannot take a prompt now; trying later is right). Add it only if
  `grep -rn 'ErrSessionNotReady =' internal/session/` prints nothing. If it exists, reuse it.
- **R2 Claim in Prompt.** Immediately after `defer unlock()` in `Manager.Prompt`, before any
  other statement that can reach tmux: `if !m.claimRestart(s.ID) { return fmt.Errorf("prompt session %s: %w", s.ID, ErrSessionNotReady) }`
  then `defer m.releaseRestart(s.ID)`. The release is deferred only after a successful claim.
  Add a why-comment: a revive or continue retypes the start command, so pasted text would land
  mid-restart; refused, not queued (FR-015). If a sibling fix already put a liveness check after
  the lock, the claim still goes first, directly after `defer unlock()`.
- **R3 HTTP answer.** Add `var bodySessionNotReady = []byte(`{"error":"session not ready"}`)` beside
  `bodyTeardownUnverified` with a why-comment. In `refusePrompt`, before the 500 fallback:
  `errors.Is(err, session.ErrSessionNotReady)` → `AuditFrom(r.Context()).Deny(session.ErrSessionNotReady.Error())`,
  `Content-Type: application/json` (use `headerContentType`, `contentTypeJSON`), `409`, that body.
  Report a write error with `s.report` as `failInternal` does. If
  `grep -n 'session not ready' internal/httpapi/sessions.go` already prints a mapping, reuse it.
- **R4 Out of scope.** The liveness gates (fix-4, fix-7), the `failed`-state refusal, bracketed
  paste for Claude (fix-8), the leading `!`/`/` rule (fix-9, done), `request_id`, `Compact`, the
  browser type route, `docs/security.md`, every file under `specs/`, and the parent plan's boxes.
  The contract in `specs/001-crswd-daemon-core/contracts/http-api.md` already documents this rule.

## Pre-commit commands (every task, in this order)

1. `gofmt -l .` prints nothing.
2. `go build ./...` exits 0.
3. `go vet ./...` exits 0.
4. `go vet -tags tmux ./...` exits 0.
5. `go vet -tags quickstart ./cmd/crswd` exits 0.
6. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` exits 0. If golangci-lint is not installed, record that in PROGRESS.md.
7. `test ! -e go.sum` exits 0.
8. `git status --short` lists only paths in "Files touched" or this notebook directory.

## Tasks

- [ ] T1 session: R1+R2. First add `TestPromptRefusesWhileARestartIsInFlight` to `manager_test.go`: hold `f.mgr.claimRestart(s.ID)`; Prompt errors.Is `ErrSessionNotReady`, adds no tmux call; `claimRestart` still false; release; Prompt returns nil; `claimRestart` now true. Verify: `go test -race ./internal/session/ -run Prompt` exits 0.
- [ ] T2 session test: add `TestConcurrentPromptsNeverInterleave` to `manager_test.go`: 8 goroutines prompt one session at once (`sync.WaitGroup`); all return nil; the 24 new calls form 8 load/paste/Enter triplets in order, each triplet one buffer name, all 8 names distinct. Verify: `go test -race -count=3 ./internal/session/ -run Prompt` exits 0.
- [ ] T3 httpapi: R3. First add `TestPromptNotReadyAnswers409` to `sessions_test.go`: via `s.authenticate(audit.ActionSessionPrompt, h)`, `h` calls `s.refusePrompt` with a wrapped `session.ErrSessionNotReady`; want 409, body exactly `{"error":"session not ready"}`, one deny, reason `session not ready`. Verify: `go test ./internal/httpapi/ -run Prompt` exits 0.
- [ ] T4 docs: append one line dated 2026-10-23 to `docs/fixes-log.md` in its existing format: a prompt sent while a revive or continue holds the session's restart claim now answers `409` `{"error":"session not ready"}` with no tmux command. Use the words `restart claim`. Verify: `grep -n 'restart claim' docs/fixes-log.md | grep -q 2026-10-23` exits 0.
- [ ] F1 final: run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go test ./... && go test -race ./internal/session/ ./internal/httpapi/ && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd` exits 0.

## Files touched

The loop may create or edit these paths and no others (its own notebook directory
`ralph/plans/lm-steer-crswd-018-fix-10/` is always in scope):

```allowlist
internal/session/session.go
internal/session/manager.go
internal/session/manager_test.go
internal/httpapi/sessions.go
internal/httpapi/sessions_test.go
docs/fixes-log.md
```

Not in scope, ever, in this milestone: `AGENTS.md`, `k8s/`, `specs/`, `internal/tmuxctl/`,
`internal/session/supervisor.go`, `internal/httpapi/input.go`, `docs/security.md`, `go.mod`,
`go.sum`, any new directory under `internal/`.
