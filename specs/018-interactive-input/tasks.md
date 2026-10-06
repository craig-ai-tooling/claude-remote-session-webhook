---
description: "Task list for 018-interactive-input"
---

# Tasks: Interactive Input on the Session Page

**Tests**: REQUIRED (`AGENTS.md`). Every task ends with the gate in **Every task
ends green** below, and every task adds at least one test that fails without it.

**Read before any task**: `AGENTS.md`, `docs/conventions.md`, `docs/security.md`
§1–§3, `specs/018-interactive-input/spec.md` and `research.md`. The tasks are in
dependency order. Do them top to bottom.

## Every task ends green

1. `gofmt -l .` prints nothing.
2. `go build ./...`
3. `go vet ./... && go vet -tags tmux ./... && go vet -tags quickstart ./cmd/crswd`
4. `go test ./...`
5. `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run` (check `golangci-lint --version` is 2.x first, #26)
6. `go test -tags tmux ./internal/tmuxctl/...` when the task touched `internal/tmuxctl`.
7. `test ! -e go.sum`

## Conventions every task follows

- Comments explain why, never what. Match the density of the file you edit.
- Errors wrap with `%w` and context. They never carry typed text, a key name, pane
  content or history (FR-012, `docs/conventions.md` Errors).
- No refactoring outside the task (AR-008).
- A guard is proven by breaking it. Comment out the line, see the named test fail,
  then restore it. Record the test name in `PROGRESS.md`.

---

## Phase 1 — tmux (`internal/tmuxctl`), serial: T001 → T002 → T003

### T001 — Create every session with `history-limit 5000`

- **Files**: `internal/tmuxctl/controller.go`, `internal/tmuxctl/fake.go`,
  `internal/tmuxctl/fake_test.go`, `internal/tmuxctl/exec_test.go`,
  `internal/tmuxctl/exec_tmux_test.go`, `internal/session/manager_test.go`.
- **Interface**:
  - In `controller.go`, after the `Controller` interface, add
    `const HistoryLimit = 5000`, with a comment citing research.md R2.
  - In `fake.go:50`, replace the body of `argvNew` with
    `return []string{"tmux", "set-option", "-g", "history-limit", strconv.Itoa(HistoryLimit), ";", "new-session", "-d", "-s", name, "-c", workDir}`.
    `strconv` is already imported in `fake.go`. The comment on `argvNew` says that
    `";"` is tmux's command separator as its own argv element, not a shell string,
    and that a per-session `set-option` after creation does not change an existing
    pane (R2).
- **Approach**: one builder change. `Exec.New` (`exec.go:153`) and `Fake.New` both
  read `argvNew`, so neither changes. `ArgvNew` (`argv.go`) wraps it, so the
  spec-017 pod inherits the chain.
- **Edge cases**:
  - No tmux server running: the chained form starts one (measured, R2), and
    `Exec.New` is unchanged.
  - The `-L` socket flag is prepended by `Exec.args`, ahead of `set-option`. That
    is correct, because the option must land on this daemon's own server.
- **Mirror**: `argvResize` at `fake.go:137` (strconv in a builder).
- **Tests**:
  1. Update the three literal `new-session` argv expectations to the new slice:
     - `internal/tmuxctl/fake_test.go:65`
     - `internal/tmuxctl/exec_test.go:240` (it carries `"-L", execSocket` after
       `"tmux"`)
     - `internal/session/manager_test.go:219`
  2. Add `TestTmuxNewSessionKeepsFiveThousandLinesOfHistory` to
     `exec_tmux_test.go`:
     - `e := newTestExec(t)`, then `e.New` a session named
       `crswd-6a000000000000000000000000000000` in `t.TempDir()`.
     - `e.SendKeys(ctx, name, "seq 1 9000", "Enter")`.
     - Wait with `waitFor` until
       `exec.Command("tmux", "-L", e.socket, "display", "-p", "-t", PaneTarget(name), "#{history_size}")`
       reports ≥ 4900. The test is in package `tmuxctl`, so `e.socket` is
       readable. `newTestExec` is at `exec_tmux_test.go:37`. Add the same
       `//nolint:gosec // socket is socketFor(t.Name())` comment that line 46 uses.
     - Assert `#{history_limit}` == `5000`.
- **Acceptance**: `go test ./internal/tmuxctl ./internal/session` passes.
  `go test -tags tmux ./internal/tmuxctl -run FiveThousand` passes. With the
  `set-option` elements removed from `argvNew`, the tmux test fails.
- **Depends on**: nothing.
- **Guardrails**:
  - Do not add `history-limit` to any configuration.
  - Do not touch `cmd/crswd/quickstart_test.go:314` or `:1619`. They call tmux
    directly for their own probes.

### T002 — `PasteBracketed`

- **Files**: `internal/tmuxctl/controller.go`, `internal/tmuxctl/fake.go`,
  `internal/tmuxctl/exec.go`, `internal/tmuxctl/argv.go`,
  `internal/tmuxctl/argv_test.go`, `internal/tmuxctl/fake_test.go`,
  `internal/tmuxctl/exec_test.go`, `internal/tmuxctl/exec_tmux_test.go`.
- **Interface**:
  - Add to `Controller`, directly after `Paste`, with a comment saying that it is
    `Paste` with `paste-buffer -p` and why (research R1):
    ```go
    PasteBracketed(ctx context.Context, name string, payload []byte) error
    ```
  - In `fake.go`:
    ```go
    const OpPasteBracketed Op = "PasteBracketed" // in the Op const block, after OpPaste
    func argvPasteBufferBracketed(name string) []string {
        return []string{"tmux", "paste-buffer", "-p", "-d", "-b", name, "-t", PaneTarget(name)}
    }
    func (f *Fake) PasteBracketed(_ context.Context, name string, payload []byte) error
    ```
  - In `exec.go`:
    ```go
    func (e *Exec) PasteBracketed(ctx context.Context, name string, payload []byte) error
    ```
  - In `argv.go`:
    ```go
    func ArgvPasteBracketed(name string) (loadBuffer, pasteBuffer []string) {
        return argvLoadBuffer(name), argvPasteBufferBracketed(name)
    }
    ```
- **Approach**:
  - `Fake.PasteBracketed` copies `Fake.Paste` (`fake.go:315`) line for line, with
    `OpPasteBracketed` and `argvPasteBufferBracketed`.
  - `Exec.PasteBracketed` copies `Exec.Paste` (`exec.go:176`), including the
    deliberate omission of tmux's stderr from the load-buffer error. It swaps the
    second argv for `argvPasteBufferBracketed` and uses error text
    `tmux paste-buffer -p %s`.
- **Edge cases**:
  - Empty payload: deliver it, as `Paste` does. Refusing empty text is the
    manager's job (T005).
  - Unknown session: the fake returns `errNoSession(name)` after recording, as
    `Paste` does.
- **Mirror**: `Fake.Paste` (`fake.go:315`), `Exec.Paste` (`exec.go:176`), and the
  `ArgvPaste` wrapper in `argv.go`.
- **Tests**:
  1. `argv_test.go` `TestArgvWrappersEqualBuilders` (line 19): add a row asserting
     that `ArgvPasteBracketed` returns `argvLoadBuffer` and
     `argvPasteBufferBracketed`.
  2. `fake_test.go` `TestFakeRecordsExactArgv` (line 30): call
     `f.PasteBracketed(ctx, fakeName, []byte("hello"))` after the `Paste` call.
     Add the two expected `Call`s with `Op: tmuxctl.OpPasteBracketed`, and the
     second argv containing `-p`.
  3. `fake_test.go`: add `TestFakePasteBracketedPayloadNeverEntersArgv`, a copy of
     `TestFakePastePayloadNeverEntersArgv` (line 243) calling `PasteBracketed`.
  4. `exec_test.go`: add `TestExecPasteBracketedKeepsCallerTextOffTheCommandLine`,
     a copy of `TestExecPasteKeepsCallerTextOffTheCommandLine` (line 269). It
     additionally asserts that `calls[1].Argv` contains `"-p"`.
  5. `exec_tmux_test.go`: add `TestTmuxPasteBracketedWrapsTheText`.
     - New session `crswd-6b000000000000000000000000000000` in `t.TempDir()`.
     - `e.SendKeys(ctx, name, "printf '\\033[?2004h'; stty -icanon -echo; cat -v", "Enter")`.
     - Wait 500ms with `waitFor` on a capture containing nothing new. Simplest:
       wait until `CapturePane` contains `cat -v`.
     - `e.PasteBracketed(ctx, name, []byte("alpha\nbeta"))`.
     - `waitFor` until `CapturePane` contains `^[[200~alpha`.
     - Assert it also contains `^[[201~`.
- **Acceptance**: `go test ./internal/tmuxctl` passes.
  `go test -tags tmux ./internal/tmuxctl -run Bracketed` passes. With `"-p"`
  removed from `argvPasteBufferBracketed`, both the fake row and the tmux test
  fail.
- **Depends on**: T001 (same files).
- **Guardrails**:
  - Do not change `Paste`, `argvPasteBuffer`, `Manager.Prompt` or
    `Manager.Compact` (spec FR-006, research R1 "Rejected (a)").

### T003 — `CaptureHistory`

- **Files**: `internal/tmuxctl/controller.go`, `internal/tmuxctl/fake.go`,
  `internal/tmuxctl/exec.go`, `internal/tmuxctl/argv.go`,
  `internal/tmuxctl/argv_test.go`, `internal/tmuxctl/fake_test.go`,
  `internal/tmuxctl/exec_test.go`, `internal/tmuxctl/exec_tmux_test.go`.
- **Interface**:
  - Add to `Controller`, after `CapturePane`, with a comment stating the
    no-`-e` rule and the refuse-not-truncate bound:
    ```go
    CaptureHistory(ctx context.Context, name string) (string, error)
    ```
  - In `fake.go`:
    ```go
    const OpCaptureHistory Op = "CaptureHistory" // after OpCapturePane
    func argvCaptureHistory(name string) []string {
        return []string{"tmux", "capture-pane", "-p", "-S", "-" + strconv.Itoa(HistoryLimit), "-E", "-1", "-t", PaneTarget(name)}
    }
    // fakeSession gains:  history string
    func (f *Fake) CaptureHistory(_ context.Context, name string) (string, error)
    func (f *Fake) SetHistory(name, content string) // mirror SetPane, fake.go:567
    ```
  - In `exec.go`:
    ```go
    var ErrHistoryTooLarge = errors.New("tmuxctl: the captured history is past the bound")
    const maxHistoryBytes = 4 << 20
    func (e *Exec) CaptureHistory(ctx context.Context, name string) (string, error)
    ```
  - In `argv.go`:
    ```go
    func ArgvCaptureHistory(name string) []string { return argvCaptureHistory(name) }
    ```
- **Approach**: `Exec.CaptureHistory` runs `argvCaptureHistory`. On a run error
  it returns `fmt.Errorf("tmux capture-pane history %s: %w", name, withStderr(err, stderr))`.
  Then:
  1. If `countLines(stdout) > HistoryLimit`, return `""` and
     `fmt.Errorf("tmux capture-pane history %s: %w: %d lines past the %d-line bound", name, ErrHistoryTooLarge, lines, HistoryLimit)`.
  2. If `len(stdout) > maxHistoryBytes`, return `""` and
     `fmt.Errorf("tmux capture-pane history %s: %w: %d bytes past the %d-byte bound", …)`.
  3. Otherwise return `stdout` verbatim.

  It does not use `e.paneBound`, which is the live screen's bound (see the
  `CapturePane` comment at `exec.go:190-219`). `-E -1` excludes the visible
  screen, so the history and the pane never overlap (measured: 4979 lines, all
  history, for a 9000-line job at limit 5000).
- **Edge cases**:
  - Empty history: return `""`, nil.
  - Unknown session: the fake returns `errNoSession`. `Exec` returns tmux's
    error, wrapped.
  - Exactly 5000 lines: allowed.
  - 5001 lines: `ErrHistoryTooLarge`.
- **Mirror**: `Exec.CapturePane` (`exec.go:220-235`), `Fake.CapturePane`
  (`fake.go:330`), `SetPane` (`fake.go:567`).
- **Tests**:
  1. `argv_test.go`: add a wrapper equality row.
  2. `fake_test.go`: in `TestFakeRecordsExactArgv`, call `CaptureHistory` and
     expect its `Call`. Add `TestFakeCaptureHistoryNeverAsksForEscapes`, mirroring
     line 127: no argv element is `-e`, and the result equals what `SetHistory`
     stored.
  3. `exec_test.go`: add `TestExecCaptureHistoryRefusesPastTheBound`. It is
     table-driven on the stub's stdout:
     - 5000 lines → ok
     - 5001 lines → `errors.Is(err, ErrHistoryTooLarge)`
     - one line of 4 MiB + 1 bytes → `ErrHistoryTooLarge`
     - `""` → ok

     Mirror the existing `CapturePane` bound tests in that file. Find them with
     `grep -n ErrPaneTooLarge internal/tmuxctl/exec_test.go`.
  4. `exec_tmux_test.go`: add `TestTmuxCaptureHistoryReturnsOnlyHistory`. New
     session `crswd-6c000000000000000000000000000000`, run `seq 1 6000`, wait until
     the pane shows `6000`, then `CaptureHistory`. Assert:
     - the line count is ≤ 5000 and ≥ 4900;
     - the last line does not equal the visible screen's last non-empty line;
     - no byte is `0x1b`.
- **Acceptance**: `go test ./internal/tmuxctl` passes.
  `go test -tags tmux ./internal/tmuxctl -run History` passes. Deleting the
  line-count check fails the 5001 row.
- **Depends on**: T002.
- **Guardrails**:
  - Do not change `CapturePane`, `argvCapturePane` or `DefaultPaneBound`.
  - Do not add `-e` or `-J`.

---

## Phase 2 — session (`internal/session`), serial: T004 → T005 → T006

### T004 — Keys and text validation (pure functions)

- **Files**: `internal/session/input.go` (NEW), `internal/session/input_test.go`
  (NEW).
- **Interface**:
  ```go
  // Key is a symbolic key name from the closed set the session page offers.
  type Key string

  const (
      KeyEnter     Key = "enter"
      KeyEscape    Key = "escape"
      KeyInterrupt Key = "interrupt"
      KeyTab       Key = "tab"
      KeyBackTab   Key = "backtab"
      KeyUp        Key = "up"
      KeyDown      Key = "down"
      KeyLeft      Key = "left"
      KeyRight     Key = "right"
      KeyPageUp    Key = "pageup"
      KeyPageDown  Key = "pagedown"
      KeyBackspace Key = "backspace"
  )

  // tmuxKeys is the whole allowlist: research.md R3 measured each mapping.
  var tmuxKeys = map[Key]string{
      KeyEnter: "Enter", KeyEscape: "Escape", KeyInterrupt: "C-c", KeyTab: "Tab",
      KeyBackTab: "BTab", KeyUp: "Up", KeyDown: "Down", KeyLeft: "Left",
      KeyRight: "Right", KeyPageUp: "PageUp", KeyPageDown: "PageDown", KeyBackspace: "BSpace",
  }

  // Keys returns the allowlist in key-bar order, which is the order above.
  func Keys() []Key

  // ParseKey maps a caller's string to a Key. Anything outside the set is ErrUnknownKey.
  func ParseKey(name string) (Key, error)

  // MaxTypeBytes bounds one typed message (spec FR-004).
  const MaxTypeBytes = 16384

  // ValidateTyped applies FR-004 to text already CRLF-normalised by the caller.
  func ValidateTyped(text string) error

  var (
      ErrUnknownKey   = errors.New("that is not a key the session page offers")
      ErrInputTooLong = errors.New("typed text is longer than the bound")
      ErrInputInvalid = errors.New("typed text holds a control character or is not UTF-8")
  )
  ```
- **Approach**:
  - `ValidateTyped` checks, in this order:
    1. `text == ""` → `ErrEmptyPrompt`, the existing sentinel at `manager.go:158`.
    2. `len(text) > MaxTypeBytes` → `ErrInputTooLong`.
    3. `!utf8.ValidString(text)` → `ErrInputInvalid`.
    4. Any byte `b < 0x20 && b != '\t' && b != '\n'`, or `b == 0x7f` →
       `ErrInputInvalid`.
  - `ParseKey` does an exact, case-sensitive map lookup.
  - `Keys` returns a fresh slice literal in const order.
- **Edge cases**:
  - `"Escape"` with a capital: unknown.
  - `" enter"`: unknown.
  - `""`: unknown.
  - Text of exactly 16384 bytes: ok.
  - `"\r"`: invalid.
  - `"\x1b[201~"`: invalid.
  - `"a\tb\nc"`: ok.
  - Multibyte `"é"`: ok.
  - `"\xff"`: invalid.
- **Mirror**: the sentinel block style at `internal/session/manager.go:111-160`,
  and the table tests in `internal/session/name_test.go`
  (`ls internal/session/name_test.go`).
- **Tests** (`input_test.go`, table-driven, `t.Parallel()`):
  - `TestParseKeyAcceptsExactlyTheAllowlist`: every `Keys()` value round-trips,
    and the cases above are refused with `errors.Is(err, ErrUnknownKey)`.
  - `TestKeysMapToMeasuredTmuxNames`: the map equals the R3 table literally.
  - `TestValidateTyped`: every edge case above, with its sentinel.
- **Acceptance**: `go test ./internal/session -run 'Key|Typed'` passes.
- **Depends on**: T003. That is only for order. There is no code dependency.
- **Guardrails**: do not export `tmuxKeys`. The tmux name is reached only through
  `PressKey` (T005).

### T005 — `Manager.Type` and `Manager.PressKey`

- **Files**: `internal/session/input.go`, `internal/session/input_test.go`.
- **Interface**:
  ```go
  // Type delivers operator text by bracketed paste, then Enter when submit is true.
  func (m *Manager) Type(ctx context.Context, s Session, text string, submit bool) error
  // PressKey sends one allowlisted key.
  func (m *Manager) PressKey(ctx context.Context, s Session, key Key) error
  ```
- **Approach**: both copy `Manager.Compact` (`manager.go:1084-1136`) step by step.
  1. `s.ID == ""` → `ErrSessionNotFound`.
  2. `s.State == StateDead` → `ErrSessionDead`.
  3. Validate:
     - `Type` runs `ValidateTyped(text)`.
     - `PressKey` looks up `tmuxKeys[key]`. A missing key → `ErrUnknownKey`.
  4. `now := m.clock.Now()`, then `displayed := s.DisplayState(now)`, then
     `m.store.Touch(s.ID, now)`. A Touch error is returned wrapped. Then set
     `s.LastActivity = now`, and emit `FleetChanged` if the display state changed,
     exactly as in `manager.go:1114-1129`.
  5. Deliver:
     - `Type`: `m.tmux.PasteBracketed(ctx, s.TmuxName(), []byte(text))`, then, if
       `submit`, `m.tmux.SendKeys(ctx, s.TmuxName(), enterKey)`.
     - `PressKey`: `m.tmux.SendKeys(ctx, s.TmuxName(), tmuxKeys[key])`.
  6. Error strings name the session ID and the step. Examples:
     - `"type into session %s: %w"`
     - `"submit typed text in session %s: %w"`
     - `"press a key in session %s: %w"`

     They never include the text or the key.
- **Edge cases**:
  - Validation failure: no Touch and no tmux call. Assert this with the fake's
    `Calls()` being empty.
  - Paste fails: no Enter sent. The error wraps the fake's failure.
  - `submit=false`: exactly two recorded calls (load-buffer, paste-buffer -p).
- **Mirror**: `Manager.Compact` (`manager.go:1084`); the tests
  `TestCompactUsesBufferPath` (`manager_test.go:1258`),
  `TestCompactRecordsTheDriving` (`:1302`) and
  `TestCompactRefusesWhatItCannotDeliver` (`:1330`).
- **Tests** (`input_test.go`, using `newManagerFixture` from
  `manager_test.go:66`):
  - `TestTypeDeliversByBracketedPasteThenEnter`: the recorded ops are
    `[PasteBracketed, PasteBracketed, SendKeys]`, Stdin equals the text, and the
    SendKeys argv ends in `Enter`.
  - `TestTypeWithoutSubmitPressesNothing`.
  - `TestTypeRecordsTheDriving`: the store record's `LastActivity` equals the
    fixture clock.
  - `TestTypeRefusesWhatItCannotDeliver`: empty, too long, control char, dead,
    empty ID. Each has its sentinel and zero tmux calls.
  - `TestTypeNamesNoTextInItsError`: `f.tmux.FailOp(tmuxctl.OpPasteBracketed,
    errors.New("boom"))` (`fake.go:554`). Assert the error string does not
    contain the canary text.
  - `TestPressKeySendsTheMappedConstant`: a table over all 12 keys. The argv's
    last element is the tmux name.
  - `TestPressKeyRefusesAnUnknownKey`: zero calls.
- **Acceptance**: `go test ./internal/session -run 'Type|PressKey'` passes.
- **Depends on**: T002, T004.
- **Guardrails**:
  - `Type` is unconditionally bracketed. It calls `PasteBracketed` for
    every session, whatever its start command or runtime. It never consults a
    per-runtime paste choice, and no later spec (019 included) may route it
    through one: R1 measured that plain `paste-buffer` submits line one early on
    Claude Code, so a runtime switch here would break multi-line text on Claude.
  - Do not call `Paste` from `Type`.
  - Do not add a key parameter to `Prompt`.
  - Do not consult `DetectDialog`, since spec says input is never gated.

### T006 — `Manager.History`

- **Files**: `internal/session/input.go`, `internal/session/input_test.go`.
- **Interface**:
  ```go
  // History returns the session's tmux scrollback, excluding the visible screen, stripped.
  func (m *Manager) History(ctx context.Context, s Session) (Capture, error)
  ```
- **Approach**:
  1. Apply the two guards from `Manager.Output` (`manager.go:1422-1431`).
  2. Call `text, err := m.tmux.CaptureHistory(ctx, s.TmuxName())`. On error,
     return `fmt.Errorf("capture history of session %s: %w", s.ID, err)`. Do
     **not** call `m.unreadable`. A history refusal (`ErrHistoryTooLarge`) is not
     evidence that the session died.
  3. Return `Capture{Text: tmuxctl.Strip(text), At: m.clock.Now()}`.

  History does not Touch the record, because reading is not driving (FR-011).
- **Edge cases**:
  - Empty history: `Capture{Text: ""}`, nil.
  - `ErrHistoryTooLarge` passes through `errors.Is`.
  - Dead or empty ID: refused with no tmux call.
- **Mirror**: `Manager.Output` (`manager.go:1422`).
- **Tests**:
  - `TestHistoryStripsEscapes`: `SetHistory` with `"\x1b[31mred\x1b[0m\n"`, and the
    result text is `"red\n"`.
  - `TestHistoryDoesNotRecordTheDriving`.
  - `TestHistoryPassesTheBoundThrough`: `FailOp(tmuxctl.OpCaptureHistory, …)` with
    `tmuxctl.ErrHistoryTooLarge`. Assert `errors.Is`, and that the session is
    still in the store.
  - `TestHistoryRefusesWhatItCannotRead`: the dead and empty-ID cases.
- **Acceptance**: `go test ./internal/session -run History` passes.
- **Depends on**: T003, T005.
- **Guardrails**: do not change `Output` or `unreadable`.

---

## Phase 3 — HTTP and audit

### T007 — Three audit actions

- **Files**: `internal/audit/audit.go`, `internal/audit/audit_test.go`.
- **Interface**: after `ActionDashboardReflow` (`audit.go:158`), add:
  ```go
  ActionDashboardType    Action = "dashboard.type"
  ActionDashboardKey     Action = "dashboard.key"
  ActionDashboardHistory Action = "dashboard.history"
  ```
  Each gets a comment saying the record never carries the text, the key or the
  history, because the shape is frozen and FR-012 forbids it.
- **Tests**:
  1. Add all three to the map in `TestEmitAcceptsEveryDocumentedAction`
     (`audit_test.go:223`, map near line 254).
  2. Add all three to the `added` map in `TestDashboardActionsAreDistinctFromAPI`
     (`audit_test.go:357`, map at line 367).
- **Acceptance**: `go test ./internal/audit` passes.
- **Depends on**: nothing. It can run in parallel with Phase 1 (see Lanes).
- **Guardrails**: do not add a field to the audit record.

### T008 — Outcomes and the input rate bucket

- **Files**: `internal/httpapi/outcome.go`, `internal/httpapi/server.go`,
  `internal/httpapi/ratelimit.go` (the `inputRatePerMin` constant),
  `internal/httpapi/input_test.go` (NEW, created here for
  `TestTheInputBudgetIsBuilt`), `internal/httpapi/outcome_test.go` (only if a
  table there needs the new codes;
  `TestEveryOutcomeThisPackageSpellsHasASentence` at line 60 finds them by
  itself).
- **Interface**:
  - In `outcome.go`, in the const block near line 146, add:
    ```go
    outcomeTypeEmpty    outcome = "type-empty"
    outcomeTypeInvalid  outcome = "type-invalid"
    outcomeTypeTooLong  outcome = "type-too-long"
    outcomeTypeFailed   outcome = "type-failed"
    outcomeKeyUnknown   outcome = "key-unknown"
    outcomeKeyFailed    outcome = "key-failed"
    outcomeInputLimited outcome = "input-limited"
    ```
  - Add map entries (near line 266) with exactly these `Message`s:

    | Code | Message |
    |---|---|
    | type-empty | `Nothing was sent: the box was empty.` |
    | type-invalid | `Nothing was sent: the text holds a control character. Use the key bar for Esc, Ctrl-C and the arrows.` |
    | type-too-long | `Nothing was sent: the text is longer than 16384 bytes.` |
    | type-failed | `The text could not be delivered. The session may have stopped; reload to see.` |
    | key-unknown | `Nothing was sent: that is not a key this page offers.` |
    | key-failed | `The key could not be delivered. The session may have stopped; reload to see.` |
    | input-limited | `Nothing was sent: too many keys and messages in a short time. Wait a moment and try again.` |

  - In `server.go`:
    - Add the field `inputs *limiter[auth.CallerID]` after `logins` (line 206),
      with a comment citing research R5.
    - Add `const inputRatePerMin = 240` beside `loginRatePerMin`
      (`ratelimit.go:265`). Put it in `ratelimit.go`, directly after that
      constant, with a comment.
    - In `newServer`, after the `logins` limiter (line 660), add
      `inputs, err := newLimiter[auth.CallerID]("input", inputRatePerMin, systemClock{})`.
      On error, return `fmt.Errorf("httpapi: build the input rate limiter: %w", err)`.
    - Set `inputs: inputs,` in the `Server` literal (line ~692).
- **Approach**: `newServer`'s signature does not change. The bucket is built
  inside it, as `logins` is, so none of the 23 test callers of `newServer`
  change.
- **Tests**: `go test ./internal/httpapi -run Outcome` passes. A new
  `TestTheInputBudgetIsBuilt` in `internal/httpapi/input_test.go` (create the
  file here) asserts `newTestServer(t, loopbackListen).inputs != nil`
  (`newTestServer` is at `server_test.go:42`).
- **Depends on**: T007.
- **Guardrails**: do not make the rate configurable. Do not reuse `creates`.

### T009 — `POST /dashboard/sessions/{id}/type`

- **Files**: `internal/httpapi/input.go` (NEW), `internal/httpapi/input_test.go`,
  `internal/httpapi/server.go`.
- **Interface** (`input.go`):
  ```go
  const patternDashboardType = "POST /dashboard/sessions/{" + pathValueID + "}/type"
  const (
      fieldText  = "text"
      fieldEnter = "enter" // "yes" presses Enter after the paste
  )
  var errTypeRefused = errors.New("the typed text could not be delivered")
  var errInputRateExceeded = errors.New("the input budget for this operator is spent")
  func (s *Server) typeFromBrowser(w http.ResponseWriter, r *http.Request)
  ```
  Register it in `server.go` directly after the compact line (`server.go:806`):
  `s.handleAction(patternDashboardType, audit.ActionDashboardType, s.typeFromBrowser)`.
- **Approach**: the handler steps, in order, mirror `compactFromBrowser`
  (`actions.go:850`):
  1. `OperatorFrom`. If it is missing: `Deny(errDashboardNoOperator)` and
     `refuseBrowser`.
  2. `routableID(id)`. If false: `Deny(errScopeNoRoute)` and `renderNotFound`.
  3. `s.inputs.allow(operator.Owner)`. If false: `Deny(errInputRateExceeded)` and
     `redirectOutcome(outcomeInputLimited)`.
  4. `text := strings.ReplaceAll(r.PostForm.Get(fieldText), "\r\n", "\n")`, then
     `session.ValidateTyped(text)`. Map the errors:
     - `ErrEmptyPrompt` → `outcomeTypeEmpty`
     - `ErrInputTooLong` → `outcomeTypeTooLong`
     - `ErrInputInvalid` → `outcomeTypeInvalid`

     Each one also does `Deny(err.Error())`. None of these sentinel strings
     carries text.
  5. `live, err := s.sessions.View(id, operator.Owner)`. On error:
     `Deny(resolveReason(err))` and `notFoundAction`.
  6. `AuditFrom(r.Context()).SetSessionID(live.ID)`.
  7. `s.sessions.Type(r.Context(), live, text, r.PostForm.Get(fieldEnter) == confirmYes)`.
     On error, use the `refuseBrowserCompact` shape (`actions.go:922`):
     - `ErrSessionNotFound` or `ErrSessionDead` → `notFoundAction`
     - anything else → `Deny(errTypeRefused)` and `redirectOutcome(outcomeTypeFailed)`.
  8. On success, `w.WriteHeader(http.StatusNoContent)`.
- **Edge cases**:
  - `enter=yes` presses Enter. `enter` absent or `enter=YES` does not.
  - A form missing the page token is refused by the gate before the handler runs.
    No handler code is needed for that.
  - Text over `MaxBodyBytes` is refused by the gate's `MaxBytesReader`.
- **Tests** (`input_test.go`). Build requests the way `actions_test.go` does,
  with `newRefuser` (`actions_test.go:5043`), `r.mine` (`:5058`) and
  `r.wellFormed` (`:5089`):
  - `TestTypeDeliversAndAnswersNoContent`: status 204, empty body, fake calls
    `[PasteBracketed×2, SendKeys Enter]`.
  - `TestTypeWithoutEnterPressesNothing`.
  - `TestTypeNormalisesCRLF`: text `"a\r\nb"` arrives on Stdin as `"a\nb"`.
  - `TestTypeRefusesBadTextWithItsOutcome`: a table of empty / too long / `"\x1b"`
    → 303 with `Location` `/?outcome=<code>`, and zero tmux calls.
  - `TestTypeRefusesLikeEveryAction`: build a `mutatingRoute` (struct at
    `actions_test.go:5158`) for this path with `fields` setting
    `text=hello, enter=yes`. Loop `refusalShapes()` (`:5326`) as
    `TestRefusalIsNotARedirect` does (`:5461`), asserting each `c.status` and
    `c.body`. **Do not** add the route to `mutatingRoutes()`: its success is 204,
    not that function's 303 (research R4).
  - `TestTypeSpendsTheInputBudget`: 121 rapid posts. The 121st answers 303 to
    `input-limited` and delivers nothing.
  - `TestTypeAuditsNoText`: the trail record for a successful type has action
    `dashboard.type`. Its raw JSON does not contain the canary text.
  - Registration is covered by `TestTypeDeliversAndAnswersNoContent`. With the
    `handleAction` line commented out, the request falls to the unrouted handler
    and the 204 assertion fails. Do this once and record it in PROGRESS.md.
- **Acceptance**: `go test ./internal/httpapi -run Type` passes.
- **Depends on**: T005, T008.
- **Guardrails**:
  - Do not add the route to `s.registered`. That is the API set.
  - Do not touch `promptSession`.
  - Do not write the text anywhere except into `Manager.Type`.

### T010 — `POST /dashboard/sessions/{id}/key`

- **Files**: `internal/httpapi/input.go`, `internal/httpapi/input_test.go`,
  `internal/httpapi/server.go`.
- **Interface**:
  ```go
  const patternDashboardKey = "POST /dashboard/sessions/{" + pathValueID + "}/key"
  const fieldKey = "key"
  var errKeyRefused = errors.New("the key could not be delivered")
  func (s *Server) keyFromBrowser(w http.ResponseWriter, r *http.Request)
  ```
  Register it after the type route:
  `s.handleAction(patternDashboardKey, audit.ActionDashboardKey, s.keyFromBrowser)`.
- **Approach**: the same eight steps as T009, with these differences:
  - Step 4 is `key, err := session.ParseKey(r.PostForm.Get(fieldKey))`. On error:
    `Deny(err.Error())` and `outcomeKeyUnknown`.
  - Step 7 is `s.sessions.PressKey`, with failure outcome `outcomeKeyFailed` and
    deny reason `errKeyRefused`.
  - It shares `s.inputs` with T009.
- **Tests**:
  - `TestKeySendsEachAllowlistedKey`: a table over `session.Keys()`. Each answers
    204 with one SendKeys call.
  - `TestKeyRefusesAnythingElse`: `"Escape"`, `"C-c"`, `"q"`, `""` and `"enter;"`
    each answer 303 `key-unknown` with zero calls.
  - `TestKeyRefusesLikeEveryAction`: as in T009.
  - `TestKeyAndTypeShareOneBudget`: 120 keys, then a type. The type answers
    `input-limited`.
  - `TestKeyAuditsNoKeyName`: the record JSON does not contain `"escape"`.
- **Acceptance**: `go test ./internal/httpapi -run Key` passes.
- **Depends on**: T009.
- **Guardrails**: never pass `r.PostForm.Get(fieldKey)` to anything except
  `session.ParseKey`.

### T011 — `GET /sessions/{id}/history`

- **Files**: `internal/httpapi/input.go`, `internal/httpapi/input_test.go`,
  `internal/httpapi/server.go`.
- **Interface**:
  ```go
  const patternSessionHistory = "GET /sessions/{" + pathValueID + "}/history"
  var errHistoryCrossSite = errors.New("a history read was initiated cross-site")
  var errHistoryUnreadable = errors.New("the session's history could not be read")
  func (s *Server) sessionHistory(w http.ResponseWriter, r *http.Request)
  ```
  Register it beside the stream (`server.go:730`):
  `s.handleBrowser(patternSessionHistory, audit.ActionDashboardHistory, s.sessionHistory)`.
- **Approach**: copy `sessionStream`'s first three checks (`stream.go:285-305`):
  operator, `crossSite`, then `View` with `renderNotFound`. Then:
  1. `SetSessionID`.
  2. `c, err := s.sessions.History(r.Context(), live)`. On error:
     `Deny(errHistoryUnreadable.Error())`, then
     `s.report(fmt.Errorf("read history of session %s: %w", live.ID, err))`, then
     `w.WriteHeader(http.StatusInternalServerError)`, and return.
  3. Set `Content-Type: text/plain; charset=utf-8` and `Cache-Control: no-store`.
  4. Write `c.Text`.
- **Edge cases**:
  - `Sec-Fetch-Site: cross-site` → refuseBrowser, which is the stream's answer.
  - Absent `Sec-Fetch-Site` → allowed, as for the stream (GET).
  - Another operator's session → not-found page.
  - Empty history → 200 with an empty body. The script renders the sentence.
- **Tests**:
  - `TestHistoryAnswersStrippedText`: `SetHistory` with an escape-laden text. The
    body equals the stripped text, the headers are as above, and the status is
    200.
  - `TestHistoryRefusesCrossSite`.
  - `TestHistoryIsOwnerScoped`.
  - `TestHistoryBoundIsA500`: `FailOp(tmuxctl.OpCaptureHistory, …)` with
    `tmuxctl.ErrHistoryTooLarge` → 500 with an empty body.
  - `TestHistoryAuditsNoContent`: the canary in `SetHistory` is absent from the
    trail.
- **Acceptance**: `go test ./internal/httpapi -run History` passes.
- **Depends on**: T006, T010.
- **Guardrails**:
  - Do not stream.
  - Do not render HTML.
  - Do not add `-e` anywhere.

### T012 — The audit leak suite covers typed text

- **Files**: `internal/audit/leak_test.go`.
- **Approach**:
  1. Next to the compact act (`leak_test.go:910-930`), add an `r.act` with
     `http.StatusNoContent`. Its `"POST /dashboard/sessions/{id}/type"` is built
     like the compact's `browserAction`, with form fields
     `text=<canaryTyped>, enter=yes`.
  2. Define `const canaryTyped = "crswd-canary-typed-7f3a"` beside
     `compactDelivered` (`:225`).
  3. Add a row `{"the text an operator typed", canaryTyped}` to the secret list
     near line 1390.
  4. Add `"dashboard.type"` to the expected-actions list in
     `TestTheLeakSuiteReallyDrivesTheDaemon` (line 1542).

  `browserAction` (`leak_test.go:754`) carries the fields in its `form url.Values`.
- **Acceptance**: `go test ./internal/audit -run 'Leak|Secret|Drives'` passes.
  Temporarily adding the text to an audit `Deny` reason in `typeFromBrowser`
  makes it fail. Revert that change.
- **Depends on**: T009.
- **Guardrails**: test-only change.

---

## Phase 4 — the page

### T013 — Markup and styles

- **Files**: `web/templates/partials/pane.html`, `web/static/crswd.css`,
  `internal/httpapi/dashboard_test.go`, `internal/httpapi/stylesheet_test.go`
  (only if a sweep needs a new allowed class).
- **Markup**: replace the note at `pane.html:115` with the FR-020 sentence. After
  it, add the block below. The template's leading comment explains why the
  controls exist (spec 018), why the key bar sends names and not bytes (R3), and
  why Scrollback is fetched on open.

  ```gotemplate
  {{ with .PageToken }}<section class="input-panel" aria-label="Type into this session">
  <form class="type-form" method="post" action="/dashboard/sessions/{{ $.ID }}/type" data-session-input>{{ template "page-token" . }}
  <label class="field-label" for="type-{{ $.ID }}">Message</label>
  <textarea class="field-input" id="type-{{ $.ID }}" name="text" rows="3" maxlength="16384" autocapitalize="off" autocorrect="off" spellcheck="false" required></textarea>
  <button class="button button-primary" type="submit" name="enter" value="yes">Send</button>
  <button class="button" type="submit" name="enter" value="no">Type only</button>
  </form>
  <form class="key-bar" method="post" action="/dashboard/sessions/{{ $.ID }}/key" data-session-input>{{ template "page-token" . }}
  <button class="button" type="submit" name="key" value="enter">Enter</button>
  <button class="button" type="submit" name="key" value="escape">Esc</button>
  <button class="button" type="submit" name="key" value="interrupt">Ctrl-C</button>
  <button class="button" type="submit" name="key" value="tab">Tab</button>
  <button class="button" type="submit" name="key" value="backtab">Shift-Tab</button>
  <button class="button" type="submit" name="key" value="up" aria-label="Up arrow">↑</button>
  <button class="button" type="submit" name="key" value="down" aria-label="Down arrow">↓</button>
  <button class="button" type="submit" name="key" value="left" aria-label="Left arrow">←</button>
  <button class="button" type="submit" name="key" value="right" aria-label="Right arrow">→</button>
  <button class="button" type="submit" name="key" value="pageup">PgUp</button>
  <button class="button" type="submit" name="key" value="pagedown">PgDn</button>
  <button class="button" type="submit" name="key" value="backspace" aria-label="Backspace">⌫</button>
  </form>
  </section>{{ end }}
  <details class="scrollback" data-history="/sessions/{{ .ID }}/history">
  <summary class="scrollback-summary">Scrollback</summary>
  <pre class="pane" id="history-{{ .ID }}" tabindex="0">Scrollback needs JavaScript.</pre>
  </details>
  ```

  Notes on the block:
  - The `{{ with .PageToken }}` scope changes the dot. `$.ID` is the pane's ID
    because the partial's root is `paneView` (`internal/httpapi/view.go:142`).
  - Inside `{{ with .PageToken }}`, `{{ template "page-token" . }}` is the same
    call the reflow form makes (`pane.html:109`).
  - Send is `.button-primary`. The session page renders no other primary
    (checked 2026-10-06: `button-primary` appears only in `create-form.html`,
    which `session.html` does not include).
  - The block goes after the note at line 115, outside the `{{ if .Unread }}`
    branch. A screen that could not be read is still a session the operator may
    need to press Esc in.
- **Styles** (`crswd.css`):
  - Add `.scrollback` to the selector list at line 1328 (`.rename, .continue,
    .unit-detail`).
  - Add `.scrollback-summary` to the list at line 1342.
  - Add:
    ```css
    .input-panel { position: sticky; inset-block-end: 0; margin-block-start: var(--s4); padding: var(--s3); background: var(--surface); border: var(--edge-width) solid var(--edge); border-radius: var(--r); }
    .type-form { display: grid; grid-template-columns: 1fr auto auto; gap: var(--s1) var(--s2); align-items: end; }
    .type-form .field-label, .type-form .field-input { grid-column: 1 / -1; }
    .key-bar { display: flex; flex-wrap: wrap; gap: var(--s2); margin-block-start: var(--s3); }
    ```
  - Inside the existing `@media (pointer: coarse)` block (line 1895), add
    `.key-bar .button { min-inline-size: var(--tap); }`.
    `.field-input` already gets `--fs-input` at line 1945.
  - Every value above is an existing token. `--surface`, `--edge`, `--s1`…`--s4`,
    `--r` and `--edge-width` are defined at `crswd.css:34-116` (checked
    2026-10-06). `TestNoRuleCarriesAValueThatBelongsInAToken` (`stylesheet_test.go:306`)
    fails any literal.
- **Tests**:
  - In `dashboard_test.go`, add `TestTheSessionPageOffersTypingKeysAndScrollback`.
    Render the session page with the same fixture
    `TestTheSessionPageRendersTheCardAndTheScreen` (`dashboard_test.go:920`) uses.
    Assert that:
    - it contains `action="/dashboard/sessions/<id>/type"`;
    - it contains `/key"`;
    - there are exactly 12 `name="key"` buttons, whose `value`s equal
      `session.Keys()` in order;
    - `data-history="/sessions/<id>/history"` is present;
    - there is a `<details class="scrollback">` with no `open` attribute;
    - both forms carry the page-token field.
  - Add `TestNoInputPanelWithoutAPageToken`: a page rendered with an empty
    `PageToken` has no `type-form`.
  - The existing assertions at `dashboard_test.go:958` and
    `cmd/crswd/quickstart_dashboard_test.go:534` must still pass unedited.
  - Run the stylesheet sweeps:
    `go test ./internal/httpapi -run 'Stylesheet|Token|Breakpoint|Markup|Element'`.
- **Acceptance**: `go test ./internal/httpapi` passes, including
  `TestTheStylesheetAndTheMarkupNameTheSameThings` (`stylesheet_test.go:580`) and
  `TestTheDashboardHasExactlyOneBreakpoint` (`:368`).
- **Depends on**: T011.
- **Guardrails**:
  - No `style=` attribute.
  - No new width breakpoint.
  - No class other than the five FR-017 names plus the existing ones.
  - Do not touch the reflow offer.

### T014 — The input client in `crswd.js`

- **Files**: `web/static/crswd.js`, `internal/httpapi/stylesheet_test.go`.
- **Approach**:
  1. In the shared submit handler (`crswd.js:1243`), add one early return directly
     after the `action` prefix check:
     `if (form.hasAttribute('data-session-input')) { return; }`.
  2. On the line directly before that IIFE's closing `})();` (`crswd.js:1381`),
     add `window.crswdShowToast = show; window.crswdSentence = sentence;`.
     `show` (line 1079) and `sentence` (line 1146) are defined in that IIFE. The
     `window.crswdReloadSignInPanel` export at line 1544 is the precedent for the
     pattern.
  3. Append a new IIFE at the end of the file, `/* The session page's input panel
     (spec 018). */`. It must:
     - Add a `document.addEventListener('submit', async (event) => { … })` that
       acts only when `event.target.matches('form[data-session-input]')`.
       - Call `event.preventDefault()`.
       - Resolve the pressed button as
         `const button = event.submitter || lastClicked.get(form) || null;`.
       - Build `body = new URLSearchParams(new FormData(form))`, then
         `body.set(button.name, button.value)` when `button?.name` is set.
     - Before that listener, declare `const lastClicked = new WeakMap();` and add
       `document.addEventListener('click', (event) => { … })` that, when
       `event.target.closest('form[data-session-input] button[type="submit"]')`
       returns a button, runs `lastClicked.set(button.form, button)`. This is the
       fallback for browsers without `SubmitEvent.submitter`, and it serves both
       the type form and the key bar.
       - `await fetch(form.action, { method: 'POST', body, headers: { 'Content-Type': 'application/x-www-form-urlencoded' }, credentials: 'same-origin' })`.
       - If `answer.status === 204` and the form is `.type-form`, clear the
         textarea and keep focus in it.
       - If `answer.status === 204` and the form is `.key-bar`, do nothing.
       - Any other answer: read `await answer.text()` and call
         `window.crswdShowToast(window.crswdSentence(said) || 'That could not be sent.', form)`.
       - On a thrown error:
         `window.crswdShowToast('That could not be sent. Nothing reached the session.', form)`.
     - Add a `keydown` listener on each `.type-form textarea`. On
       `event.key === 'Enter' && (event.ctrlKey || event.metaKey)`, call
       `event.preventDefault()` and then
       `form.requestSubmit(form.querySelector('button[value="yes"]'))`.
     - For each `details.scrollback[data-history]`, add a `toggle` listener. When
       `details.open`:
       - set the `pre`'s `textContent = 'Loading…'`;
       - `fetch(details.dataset.history, { credentials: 'same-origin', cache: 'no-store' })`;
       - if `ok`, set
         `pre.textContent = text === '' ? 'No scrollback yet.' : text` and then
         `pre.scrollTop = pre.scrollHeight`;
       - otherwise set `pre.textContent = 'Scrollback could not be read. Close and open it to try again.'`.
- **Edge cases**:
  - `fetch` missing: return without `preventDefault`, so native submit works.
    Mirror line 1248.
  - `event.submitter` missing (old Safari): the `lastClicked` WeakMap supplies
    the button, so the key bar still sends its `key` and the type form still
    sends `enter`. If neither is available (a submit with no click, e.g. a
    programmatic submit), the key bar sends no `key` and the server answers
    `key-unknown` with nothing pressed; the type form sends no `enter`, which is
    "type only". Both are safe refusals, never a wrong key.
  - A redirect answer: `fetch` follows it, and `said` is the fleet page, from
    which `sentence` pulls the outcome.
- **Mirror**:
  - the shared submit handler, `crswd.js:1243-1330`;
  - the pane client's `textContent` discipline, `crswd.js:277-303`;
  - the polling fetches with `credentials: 'same-origin', cache: 'no-store'` at
    line 1639.
- **Tests** (`stylesheet_test.go`, reading `script(t)` from line 871, in the
  style of `TestTheStreamClientReplacesTheScreenWithText` at line 967):
  - `TestTheInputClientSkipsTheSharedHandler`: the script contains
    `hasAttribute('data-session-input')`.
  - `TestTheInputClientPostsSameOrigin`: the new IIFE contains
    `credentials: 'same-origin'` and `data-session-input`, and no `innerHTML`.
  - `TestTheScrollbackIsText`: the IIFE assigns `textContent` and never
    `innerHTML` or `insertAdjacentHTML`.
  - `TestCtrlEnterSends`: the IIFE contains `event.ctrlKey || event.metaKey`.
  - `TestTheInputClientRemembersTheClickedButton`: the IIFE contains
    `new WeakMap()`, `lastClicked.set(` and
    `event.submitter || lastClicked.get(form)`.

  Isolate the new IIFE's source by slicing from its leading comment to the next
  `})();`.
- **Acceptance**: `go test ./internal/httpapi -run 'Input|Scrollback|CtrlEnter|ClickedButton'`
  passes.
- **Depends on**: T013.
- **Guardrails**:
  - Hand-written JS only.
  - No library.
  - No `innerHTML`.
  - Do not change the pane stream client.

---

## Phase 5 — documents and the gate

### T015 — Rules and docs

- **Files**: `AGENTS.md`, `docs/security.md`, `docs/components.md`,
  `docs/mobile-open-questions.md`,
  `specs/001-crswd-daemon-core/contracts/tmuxctl.md`. Not `docs/fixes-log.md`:
  this is feature work.
- **Edits**:
  1. In `AGENTS.md:10`, replace `It never types into a working session.` with the
     FR-001 sentence verbatim.
  2. In `docs/security.md` §2, replace the stale bullet "**`tmux send-keys` takes
     the prompt as a single literal argument** …" with two bullets:
     - Caller text reaches tmux only through `load-buffer` on stdin, then
       `paste-buffer` (bracketed `-p` on the session page's route). Text holding a
       C0 control other than TAB or LF is refused, because inside a bracketed paste
       `ESC[201~` would end the paste.
     - `send-keys` receives daemon constants only. The session page's key bar
       sends a symbolic name the daemon maps through a closed table.

     Then add the FR-001 sentence as its own bullet.
  3. In `docs/components.md` Pane viewer (`## Pane viewer`, line 643):
     - Replace the bullet "**The pane shows the live screen, not scrollback.** …
       History is what `tmux attach` is for" with: the pane shows the live
       screen, and history is the Scrollback disclosure below it, fetched on
       open and capped at 5000 lines.
     - Add a subsection `### Input panel (spec 018)` after `### Reflow offer`. It
       names `.input-panel`, `.type-form`, `.key-bar`, `.scrollback` and
       `.scrollback-summary`, the 204 answer, the twelve keys, Ctrl/Cmd+Enter, and
       the "never gated by the dialog heuristic" rule.
  4. In `docs/mobile-open-questions.md`, append Q4 and Q5 verbatim from spec
     FR-021, each with `**Status: UNANSWERED.**` and its fallback. Change the
     intro's "Three questions" to "Five questions".
  5. In `specs/001-crswd-daemon-core/contracts/tmuxctl.md`:
     - change line 70's `tmux new-session …` to the chained form;
     - add `paste-buffer -p` and `capture-pane -p -S -5000 -E -1` to the
       command list, with one line each.
- **Acceptance**:
  - `grep -c 'on its own initiative' AGENTS.md docs/security.md` prints `1` for
    each.
  - `grep -c 'UNANSWERED' docs/mobile-open-questions.md` is 2 higher than before
    the edit.
  - `go test ./...` passes, since doc sweeps exist in `internal/httpapi`.
- **Depends on**: T014.
- **Guardrails**:
  - Do not mark Q4 or Q5 answered.
  - Do not edit `.specify/memory/constitution.md`.

### T016 — Full gate and guard proofs

- **Files**: `ralph/plans/interactive-input/PROGRESS.md` only.
- **Steps**:
  1. Run every command in **Every task ends green**, and also
     `go test -tags tmux ./...` and `go test -tags dev ./...`.
  2. Break each guard below, run the named test, and confirm it fails. Restore
     each one immediately afterwards.
     - Remove `"-p"` from `argvPasteBufferBracketed`. The tmux bracketed test
       should fail.
     - Delete the control-byte check in `ValidateTyped`. `TestValidateTyped`
       should fail.
     - Change `tmuxKeys[KeyInterrupt]` to `"C-d"`. `TestKeysMapToMeasuredTmuxNames`
       should fail.
     - Register the type route with `handleBrowser` instead of `handleAction`.
       `TestTypeRefusesLikeEveryAction` should fail.
     - Drop `crossSite` from `sessionHistory`. `TestHistoryRefusesCrossSite`
       should fail.
  3. Record every result in PROGRESS.md, one line per guard.
- **Acceptance**: every command exits 0 after the restores, and
  `git diff --stat` shows only PROGRESS.md for this task.
- **Depends on**: T015.

---

## Parallel lanes

| Lane | Tasks | Files (disjoint across lanes) |
|---|---|---|
| A — tmux | T001 → T002 → T003 | `internal/tmuxctl/*`, plus `internal/session/manager_test.go` (T001 only) |
| B — audit | T007 | `internal/audit/audit.go`, `audit_test.go` |
| C — session | T004 → T005 → T006 | `internal/session/input.go`, `input_test.go` |
| D — http | T008 → T009 → T010 → T011 → T012 | `internal/httpapi/{input,outcome,server,ratelimit}.go`, `input_test.go`, `internal/audit/leak_test.go` |
| E — page | T013 → T014 | `web/**`, `internal/httpapi/{dashboard,stylesheet}_test.go` |
| F — docs | T015 → T016 | docs and contract files |

Lane B runs alongside lane A. Lane C needs A (T005 calls `PasteBracketed`). D needs B
and C. E needs D (the routes it posts to must exist for T013's render test to mean
anything). F is last. One Ralph notebook runs these serially, in the order above, so
the lanes are informational unless the operator splits them into separate
notebooks with separate allowlists.
