# Progress: leading `!` or `/` on the prompt route (lm-steer-crswd-018-fix-9)

The loop's notebook. Each iteration appends under "Iterations". Read this before
starting a task. Do not redo finished work.

## Planning notes (10/22/26)

- Branch `plan/lm-steer-crswd-018-fix-9`, cut from `main` at `9a938b4`.
- Baseline: `go test ./internal/httpapi/ -run 'Prompt|Type'` exits 0 (`ok ... 0.205s`). Every test it runs predates the rule.
- `grep -n 'errPromptLeadingCommand' internal/httpapi/*.go` prints nothing on the baseline.
- No `hostilePrompts` entry and no quickstart prompt payload begins with `!` or `/`, so the new refusal breaks no existing test.
- T2's tests are regression guards. They are expected to pass before and after T1; they cannot be proven failing-before, and that is correct.
- Earlier loops in this repo found golangci-lint and `~/code/ai-lawnmower/loop/safe-commit.sh` unreachable inside the loop container. If that holds here, record it and use the fallback in PROMPT.md.

## Failing-before evidence

| Task | Test | Failure line before the change |
|---|---|---|
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_slash_command` | `sessions_test.go:1254: status = 202 ("{\"id\":\"efe3d11fe90c1109f0b2b96151533fbf\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_bash_escape` | `sessions_test.go:1254: status = 202 ("{\"id\":\"d5be0daac4ea1137c20ce39248aea1aa\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_slash_behind_two_spaces` | `sessions_test.go:1254: status = 202 ("{\"id\":\"2e17a816bf52ea43594e1aea01ca4514\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_bang_behind_a_newline` | `sessions_test.go:1254: status = 202 ("{\"id\":\"6fbaddb611f0fa9352d26e53f6df5bd1\",\"delivered\":true}"); want 400` |
| T1 | `TestPromptRefusesALeadingCommandCharacter/a_slash_behind_a_bare_CR` | `sessions_test.go:1254: status = 202 ("{\"id\":\"b95d098f1f72dcbd51f8d60218660d0b\",\"delivered\":true}"); want 400` |

## Iterations

### Iteration 1 — T1 (G1 + G2)

**What changed**

- `internal/httpapi/sessions.go`: added the sentinel `errPromptLeadingCommand`
  (`"prompt text begins with a command character"`) to the prompt sentinels'
  `var (...)` block, between `errPromptUndelivered` and `errOutputNoSession`,
  with a why-comment naming FR-012 and FR-042. In `promptSession`, added the
  refusal immediately after the successful `decode[promptRequest]` and before
  the `req.Text != ""` dialog-check block:
  `if trimmed := strings.TrimLeft(req.Text, " \t\r\n"); trimmed != "" && (trimmed[0] == '!' || trimmed[0] == '/')`
  → `s.rejectBadRequest(w, r, errPromptLeadingCommand)` and return.
  `strings` was already imported; no new import, no new dependency.
- `internal/httpapi/sessions_test.go`: added the map `refusedLeadingCommands`
  (5 rows) and the table test `TestPromptRefusesALeadingCommandCharacter`,
  inserted just above `TestATmuxFailureDuringAPromptAnswersFiveHundredWithNoDetail`.
  It asserts `400`, body exactly `bodyBadRequest`, zero `f.fixture.tmux.Calls()`,
  and one audit record with `decision` deny, `reason` `errPromptLeadingCommand.Error()`
  and `action` `audit.ActionSessionPrompt`.

**How the failing-before evidence was produced**

The sentinel (G1) was added first on its own, so the new test compiled and
failed on *behaviour* rather than on `undefined: errPromptLeadingCommand`. All
five rows failed with `status = 202 … want 400` (table above). The handler
check (G2) was added second.

**Commands run**

| Command | Result |
|---|---|
| `go test ./internal/httpapi/ -run Prompt` (before G2) | FAIL — 5 subtests, all `status = 202 … want 400` |
| `go test ./internal/httpapi/ -run Prompt` (after G2) | `ok … 0.013s`, exit 0 |
| `gofmt -l .` | printed nothing |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `golangci-lint run` | **not run — `which golangci-lint` exits 1; golangci-lint is not installed in this loop container.** The session-start hook reports the same and names the pin `v2.12.2`. |
| `test ! -e go.sum` | exit 0 |
| `git status --short` | only `internal/httpapi/sessions.go` and `internal/httpapi/sessions_test.go` |

**For the next iteration**

- `errPromptLeadingCommand` now exists in `internal/httpapi/sessions.go`; T2 can
  reference it but does not need to.
- The refusal sits *before* `paneDialogState`, so a refused prompt records zero
  tmux calls. Any later test that expects a capture-pane on a `!`/`/` prompt is
  wrong by construction.
- T2's two new `hostilePrompts` rows (`"fix a/b\n/compact"`, `"say hi!"`) have
  their `!`/`/` after the first non-whitespace character, so the new check lets
  them through unchanged — `TestAPromptIsDeliveredByteForByte` still expects the
  four-call argv list beginning with `OpCapturePane`.
- `golangci-lint` is absent here; do not treat its absence as a failure, just
  record it.
- **`safe-commit.sh` could not be run, so this iteration used PROMPT.md's
  fallback.** The file is present and executable (`test -x
  ~/code/ai-lawnmower/loop/safe-commit.sh` exits 0), but the harness refused the
  invocation: with `"$PWD"` it answered `Contains shell syntax (string) that
  cannot be statically analyzed`, and with the literal `/workspace` it answered
  `This command requires approval` — and this session is non-interactive, so no
  approval can be given. `git add <path>` per path and `git commit -m …` were
  permitted and are what committed this iteration. A later iteration should try
  safe-commit first anyway; the block is the sandbox's, not the script's.

### Iteration 2 — T2 (regression guards, no production code)

**What changed**

- `internal/httpapi/sessions_test.go`: added two rows to `hostilePrompts` —
  `"a later slash": "fix a/b\n/compact"` and `"a later bang": "say hi!"` — with
  a why-comment saying the T1 refusal reads only the first non-whitespace
  character, so a `/` or `!` after it is ordinary text that must still arrive
  byte-for-byte. `TestAPromptIsDeliveredByteForByte` picks both up and still
  expects the four-call argv list beginning with `OpCapturePane`.
- `internal/httpapi/input_test.go`: added `TestTypeStillDeliversASlashCommand`,
  placed just above `TestTypeRefusesBadTextWithItsOutcome`. It types `/compact`
  with `confirmYes`, asserts `204` and that the one non-empty `call.Stdin` on the
  fake is exactly `/compact` — the same way `TestTypeNormalisesCRLF` reads the
  pasted bytes. No production code changed, per G4.

**Failing-before evidence: none, by design.** Both additions are regression
guards (planning note above). They pass before and after T1 because the refusal
is on the API prompt route only, so there is no old code they could fail
against. T1 is the only task PROMPT.md's "Prove a new test fails" applies to.

**Commands run**

| Command | Result |
|---|---|
| `go test ./internal/httpapi/ -run 'Prompt\|Type'` | `ok … 1.120s`, exit 0 |
| `go test ./internal/httpapi/ -run 'TestTypeStillDeliversASlashCommand\|TestAPromptIsDeliveredByteForByte' -v` | PASS; confirms the new test and the two new subtests `a_later_slash` / `a_later_bang` actually run rather than being filtered out |
| `gofmt -l .` | printed nothing |
| `go build ./...` | exit 0 |
| `go vet ./...` | exit 0 |
| `go vet -tags tmux ./...` | exit 0 |
| `go vet -tags quickstart ./cmd/crswd` | exit 0 |
| `golangci-lint run` | **not run — `which golangci-lint` exits 1; still not installed in this loop container**, as in iteration 1 and as the session-start hook reports. |
| `test ! -e go.sum` | exit 0 |
| `git status --short` | only `internal/httpapi/input_test.go` and `internal/httpapi/sessions_test.go` |

**For the next iteration**

- T3 is a one-line append to `docs/fixes-log.md` and touches no Go. Read the
  file's existing format first; its Verify is
  `grep -n 'leading' docs/fixes-log.md | grep -q 2026-10-22`, so the line must
  carry both the word `leading` and the date `2026-10-22`.
- After T3 only F1 remains, which runs VALIDATION_CONTRACT.md end to end.
- The `-v` run above is worth repeating for any future table-row addition here:
  `hostilePrompts` is a map, so a mistyped key silently adds a row nobody reads
  and `go test` still says `ok`.
- **`safe-commit.sh` again could not be run; PROMPT.md's fallback committed this
  iteration too.** Tried both forms iteration 1 tried, and got the same two
  refusals from the harness: `"$PWD"` → `Contains shell syntax (string) that
  cannot be statically analyzed`, and the literal path → `This command requires
  approval`, unavailable in a non-interactive session. Do not spend a third
  iteration's time on it — the block is the sandbox's, not the script's, and it
  needs a permission change outside this loop to lift.
