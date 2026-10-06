---
description: "Task list for 019-codex-runtime"
---

# Tasks: Codex as a First-Class Runtime

**Tests**: REQUIRED per `AGENTS.md`. Every task names the test that must fail without it.
**Read first**: [spec.md](spec.md), [research.md](research.md) (§M measurements, §F fixtures,
§D decisions), [plan.md](plan.md) (the `harness.Spec` table).
**Line numbers** were read on `main` at `62da9d6` (10/6/26). Spec 018 lands before Phase 1 and may
move them; find the named function with `grep -n` and edit that, never the stale line.

**The gate** (every task ends with it green):
`go build ./... && go vet ./... && go test ./... && golangci-lint run`, plus
`go test -tags tmux ./...` when `internal/tmuxctl` or `internal/session` changed, plus
`go test -tags quickstart ./cmd/crswd` when `cmd/crswd` or a typed command line changed.

**Global guardrails** (apply to every task):
- No new Go dependency; `go.sum` must not appear.
- A Claude session's typed command line, `@crswd-binary` value and pill must not change.
- Never put a path, prompt text, code or URL into a typed command line, an error string or an audit record.
- Never compare a binary name to `"claude"` or `"codex"` outside `internal/harness`.

---

## Phase 1: Core parity (notebook `ralph/plans/codex-core`)

### T001 — `internal/harness` package

- **Files**: create `internal/harness/harness.go`, `internal/harness/harness_test.go`.
- **Interface**: exactly the block in plan.md §Design (`Name`, `Claude`, `Codex`, `Other`, `Of`,
  `Label`, `Spec`, `For`), with the values in plan.md's table.
  - `Of(command string) Name`: `strings.Fields(command)`; no fields → `Other`; take
    `filepath.Base(fields[0])`; strip one trailing `.js`; `"claude"` → `Claude`, `"codex"` → `Codex`,
    anything else → `Other`.
  - `Label`: `Claude` → `"Claude Code"`, `Codex` → `"Codex"`, every other value → `"Other"`.
  - `For(Claude).ResumeArgs("X")` returns `[]string{"--resume","X"}`; `For(Codex).ResumeArgs("X")`
    returns `[]string{"resume","X"}`; `For(Other).ResumeArgs` is nil.
  - `For(Codex).RequiredFlags` is `[][]string{{"--no-alt-screen"},{"-c","check_for_update_on_startup=false"}}`.
  - `For` of an unknown `Name` returns `For(Other)` with `Name` set to `Other`.
- **Edge cases**: `Of("")` → `Other`; `Of("   ")` → `Other`; `Of("/home/x/sf-cli/bin/codex --flag")`
  → `Codex`; `Of("codex.js")` → `Codex`; `Of("CLAUDE")` → `Other` (case-sensitive, like
  `startBinary`); `Of("env FOO=1 claude")` → `Other`.
- **Mirror**: `startBinary` at `internal/session/conversation.go:362` for the token rule; the
  package-comment style of `internal/quota/quota.go`.
- **Tests**: `TestOf` (table: every edge case above plus `claude`, `/usr/local/bin/claude --x`),
  `TestLabel`, `TestForTable` (every field of the three specs equals plan.md's table),
  `TestForUnknownIsOther`.
- **Acceptance**: `go test ./internal/harness/...` passes; package imports only stdlib.
- **Depends**: spec 018 merged.
- **Guardrails**: no import of any `internal/` package from `internal/harness`.

### T002 — Liveness matches a set of names

- **Files**: `internal/tmuxctl/fake.go`, `internal/tmuxctl/controller.go` (comments on
  `OptionBinary` at `:199-214` only), `internal/tmuxctl/fake_test.go`,
  `internal/tmuxctl/exec_tmux_test.go`.
- **Interface**: no signature changes. `argvList` (`fake.go:185`) liveness expression becomes
  `#{?#{@crswd-binary},#{m/r:^(#{@crswd-binary})$,#{pane_current_command}},?}` (use the
  `OptionBinary` constant as today). `livenessOf(binary, paneCommand string) Liveness`
  (`fake.go:206`): `binary == ""` → Unknown; split `binary` on `"|"`; any element equal to
  `paneCommand` → Running; else Stopped.
- **Edge cases**: `binary` = `"claude"` behaves exactly as before. `binary` = `"codex|node"` with pane
  `node` → Running, pane `bash` → Stopped. An empty element (`"codex||node"`) never matches an empty
  pane command: skip empty elements.
- **Approach**: tmux's `m/r` is a POSIX extended regex; `|` is alternation (research M17). Names are
  `[A-Za-z0-9._-]` so no other metacharacter except `.` can occur; `.` matching any character is
  accepted (documented in the `OptionBinary` comment).
- **Mirror**: the existing `livenessOf` switch and its comment.
- **Tests**: `fake_test.go`: table `TestLivenessOfSet` (the cases above).
  `exec_tmux_test.go` (`//go:build tmux`): `TestLivenessAlternatives` — new session running
  `sleep 30`, set `@crswd-binary` to `codex|sleep`, `List` reports `LivenessRunning`; set it to
  `codex|node`, `List` reports `LivenessStopped`. Mirror the existing tmux liveness test in that file
  (grep `LivenessStopped`).
- **Acceptance**: `go test ./internal/tmuxctl/...` and `go test -tags tmux ./internal/tmuxctl/...` pass;
  `TestListFormatFieldCount` still passes unedited.
- **Depends**: T001 (none in code; ordering only).
- **Guardrails**: `listFieldCount` stays 10; the row still emits one character for liveness.

### T003 — `Controller.PanePID`

`Controller.PasteBracketed` is added by spec 018 (its plan.md: "tmuxctl gains `PasteBracketed`").
This task does not add it. Before starting, run
`grep -n 'PasteBracketed' internal/tmuxctl/controller.go`; if it prints nothing, spec 018 has not
landed: follow PROMPT.md "Blocked work" with the reason `spec 018 not merged` and stop.

- **Files**: `internal/tmuxctl/controller.go`, `internal/tmuxctl/exec.go`, `internal/tmuxctl/fake.go`,
  `internal/tmuxctl/argv.go`, `internal/tmuxctl/fake_test.go`, `internal/tmuxctl/exec_tmux_test.go`.
  `internal/loginrelay/loginrelay.go` is not touched (its narrowed `Controller` does not need it).
- **Interface**:
  ```go
  // in Controller
  PanePID(ctx context.Context, name string) (int, error)
  // fake.go builder
  func argvPanePID(name string) []string // {"tmux","display-message","-p","-t",PaneTarget(name),"#{pane_pid}"}
  // argv.go exported wrapper
  func ArgvPanePID(name string) []string
  // Fake helper
  func (f *Fake) SetPanePID(name string, pid int)
  ```
  `Exec.PanePID` runs `argvPanePID`, trims output, `strconv.Atoi`; result <= 0 or a parse failure →
  `fmt.Errorf("read the pane pid of %s: %w", name, ErrUnexpectedOutput)`. Add
  `ErrUnexpectedOutput = errors.New("tmux answered something this daemon cannot read")` to
  `controller.go` unless `grep -n 'errors.New' internal/tmuxctl/*.go` shows a sentinel with that
  meaning; then reuse it. `Fake.PanePID` returns the value set by `SetPanePID`; unset →
  `0, ErrUnexpectedOutput`; missing session → the same error `Fake.CapturePane` returns for one.
- **Edge cases**: unknown session → same error `Exec.CapturePane` returns for a missing session.
- **Mirror**: `argvCapturePane` (`fake.go:83`) and the one-builder wrappers in `argv.go`.
- **Tests**: `fake_test.go`: argv assertion for `argvPanePID`; `Fake.PanePID` set, unset and missing.
  `exec_tmux_test.go`: `TestPanePIDIsTheShell`: `PanePID` of a new session is a pid whose
  `/proc/<pid>/comm` equals `filepath.Base(os.Getenv("SHELL"))` (skip if `SHELL` is empty).
- **Acceptance**: `go test ./internal/tmuxctl/...` and `go test -tags tmux ./internal/tmuxctl/...`
  pass; `go vet ./...` passes (every Controller implementation compiles).
- **Depends**: T002, spec 018 merged.
- **Guardrails**: `k8s/` does not exist on `main` at planning time (spec 017 S5 has not landed). Run
  `test -d k8s/internal/podctl`. If it exits non-zero, touch nothing under `k8s/`. If it exits 0, add
  `func (c *Controller) PanePID(ctx context.Context, name string) (int, error) { return 0, tmuxctl.ErrUnexpectedOutput }`
  to the file `grep -ln 'func (.*) CapturePane' k8s/internal/podctl/*.go` prints, run
  `go -C k8s vet ./... && go -C k8s test ./...`, and record the file name in PROGRESS.md. The
  codex-core allowlist names `k8s/internal/podctl/` for this case only. Phase 4a replaces the stub.

### T004 — Session derives the harness and renders per-harness lines

- **Files**: `internal/session/manager.go` (`resumeFlagged` `:2072`, `markSession` caller list,
  `start` `:1948`), `internal/session/conversation.go` (`conversationCapable` `:345`, new
  `paneProcesses`), `internal/session/supervisor.go` (`markSession` `:377`),
  `internal/session/harness_test.go` (new).
- **Interface**:
  ```go
  func (m *Manager) specOf(s Session) harness.Spec   // resolveStartCommand(s.StartCommand); error → harness.For(harness.Other)
  func paneProcesses(template string) string          // conversation.go
  func withRequiredFlags(template string, groups [][]string) string // conversation.go
  ```
  - `paneProcesses`: `spec := harness.For(harness.Of(template))`; if `len(spec.PaneProcesses) > 0`
    return `strings.Join(spec.PaneProcesses, "|")`; else return `startBinary(template)`.
  - `withRequiredFlags`: let `head` be the tokens of `strings.Fields(template)` before the first
    token equal to `--` (all tokens when there is none). For each group in order, if the group's
    tokens appear as a contiguous run in `head` skip it; otherwise collect it. A group that appears
    only after `--` is an argument to Codex and is collected. Insert all collected tokens with one
    `config.InsertStartFlags(template, collected...)` call (`internal/config/config.go:1315`). No
    groups collected → return `template` unchanged.
  - `conversationCapable(template)` → `harness.For(harness.Of(template)).FreshIDFlag != ""`. Delete
    the `claudeBinary` constant and move its comment's substance to `harness.Spec.FreshIDFlag`'s doc.
  - `resumeFlagged(template, resume, conversationID)`: compute `spec` from `template`; set
    `base := withRequiredFlags(template, spec.RequiredFlags)`; fresh branch with id →
    `config.InsertStartFlags(base, spec.FreshIDFlag, conversationID)`; fresh branch with no id or
    `spec.FreshIDFlag == ""` → `base`; resume branch → `spec.ResumeArgs == nil` →
    return `ErrInvalidResume` wrapped `"this start command cannot resume a conversation"`;
    else `config.InsertStartFlags(base, spec.ResumeArgs(checked)...)`.
  - Replace both `startBinary(template)` writes of `OptionBinary` (`manager.go:2016` in `start`,
    `supervisor.go:390` in `markSession`) with `paneProcesses(template)`.
  - `SessionIDFlag`/`ResumeOneFlag` (`manager.go:57-59`) stay exported and equal
    `harness.For(harness.Claude).FreshIDFlag` / `"--resume"`; add a test pinning that equality.
- **Edge cases**: Codex template `codex --dangerously-bypass-approvals-and-sandbox` fresh →
  `codex --no-alt-screen -c check_for_update_on_startup=false --dangerously-bypass-approvals-and-sandbox`;
  resume id `I` → `codex resume I --no-alt-screen -c check_for_update_on_startup=false --dangerously-bypass-approvals-and-sandbox`.
  This order is what the two calls produce: `withRequiredFlags` inserts the flags right after the
  binary, then `InsertStartFlags(base, "resume", I)` (`config.go:1315`, it puts its tokens directly
  after the first token) puts `resume I` in front of them. Codex needs `resume <id>` immediately after
  the binary (research M10); the test asserts the exact string.
  Template already containing `--no-alt-screen` → not duplicated. Template
  `codex -- --no-alt-screen` → both groups inserted after `codex`, before `--`. Template `bash`
  (Other) → no flags, no id, `paneProcesses` = `bash`.
- **Mirror**: existing `resumeFlagged` structure; table tests in `internal/session/manager_test.go`
  that assert typed lines (grep `--session-id`).
- **Tests** (`harness_test.go`): `TestRenderStartPerHarness` (Claude fresh/resume byte-identical to
  today's expected strings copied from `manager_test.go`; Codex fresh/resume exact strings; Other),
  `TestWithRequiredFlagsSkipsPresentGroups`, `TestWithRequiredFlagsIgnoresTokensAfterDoubleDash`, `TestPaneProcesses` (claude → `claude`; codex →
  `codex|node`; `/abs/codex` → `codex|node`; bash → `bash`), `TestCreateCodexWritesBinarySet`
  (fake: `Option(name, OptionBinary)` = `codex|node`), `TestClaudeFlagConstantsUnchanged`.
- **Acceptance**: `go test ./internal/session/...` passes with no edit to any existing expected
  Claude line (SC-002); `grep -n '"claude"' internal/session/*.go | grep -v _test` prints nothing.
- **Depends**: T001, T002.
- **Guardrails**: do not change `startBinary`; do not change `config.InsertStartFlags`.

### T004a — Config refuses a Codex start command that re-enables the update check

Research M12: a Codex session that opens on the "Update now" menu turns a typed Enter into
`npm install -g @openai/codex`. T004 adds `-c check_for_update_on_startup=false`, but Codex applies
the last `-c` for a key, so an operator's own later `-c check_for_update_on_startup=true` would win.

- **Files**: `internal/config/config.go` (`validateStartCommand`, find with
  `grep -n 'func validateStartCommand' internal/config/config.go`), create
  `internal/config/codexflags.go`, `internal/config/codexflags_test.go`.
- **Interface**:
  ```go
  // codexflags.go
  var ErrCodexUpdateCheck = errors.New("a Codex start command may not turn the startup update check on")
  // codexUpdateCheckValues returns every value the command assigns to
  // check_for_update_on_startup before the first "--" token.
  func codexUpdateCheckValues(command string) []string
  func validateCodexUpdateCheck(variable, name, command string) error
  ```
  `codexUpdateCheckValues`: tokens = `strings.Fields(command)`, cut at the first `--`. Recognise four
  spellings: `-c K=V`, `--config K=V` (value in the next token), `-cK=V`, `--config=K=V`. `K` must
  equal `check_for_update_on_startup` exactly; `V` is the text after the first `=` with one pair of
  surrounding `"` or `'` removed.
  `validateCodexUpdateCheck`: if `harness.Of(command) != harness.Codex` → nil. Any value not equal
  to `false` → `fmt.Errorf("%s: start command %q: %w; refusing to start", variable, name, ErrCodexUpdateCheck)`.
  `validateStartCommand` calls it last, after its existing checks pass.
- **Edge cases** (each a test row): `codex` → nil; `codex -c check_for_update_on_startup=false` → nil;
  `codex -c check_for_update_on_startup=true` → error; `codex --config=check_for_update_on_startup=true`
  → error; `codex -ccheck_for_update_on_startup="true"` → error; `codex -c check_for_update_on_startup=false -c check_for_update_on_startup=true`
  → error; `codex -- -c check_for_update_on_startup=true` → nil (after `--`, a prompt argument);
  `claude -c check_for_update_on_startup=true` → nil (not Codex); `/abs/sf-cli/bin/codex -c check_for_update_on_startup=1` → error.
- **Mirror**: `validateStartCommandName` and the error wording of `loadStartCommands`
  (`config.go:1405`).
- **Tests**: `TestCodexUpdateCheckValues` (table above), `TestLoadStartCommandsRefusesCodexUpdateCheck`
  (`CRSW_START_COMMANDS=codex=codex -c check_for_update_on_startup=true` → `Load` error wrapping
  `ErrCodexUpdateCheck`).
- **Acceptance**: `go test ./internal/config/...` passes; existing config tests pass unedited.
- **Depends**: T001.
- **Guardrails**: `internal/config` may import `internal/harness` (stdlib-only); it imports nothing
  else new. Error strings name the variable and the command name, never the command line.

### T005 — Prompt and Compact use bracketed paste on Codex

- **Files**: `internal/session/manager.go` (`Prompt` `:1031`, `Compact` `:1084`),
  `internal/session/harness_test.go`.
- **Interface**: new unexported `func (m *Manager) paste(ctx context.Context, s Session, payload []byte) error`
  that calls `m.tmux.PasteBracketed` when `m.specOf(s).BracketedPaste`, else `m.tmux.Paste`.
  `Prompt` calls `m.paste`. `Compact`: when `BracketedPaste`, `m.paste(ctx, s, []byte("/compact"))`
  then `m.tmux.SendKeys(ctx, name, enterKey)`; otherwise unchanged (`Paste(compactCommand)`).
- **Edge cases**: errors wrap exactly as the existing calls do; prompt text never appears in an error.
- **Mirror**: `Prompt`'s existing two-step shape.
- **Tests**: `TestPromptCodexIsBracketed` (fake `Calls()` shows `paste-buffer -p` then `send-keys Enter`),
  `TestPromptClaudeUnchanged` (no `-p`), `TestCompactCodexSubmitsWithEnter`,
  `TestCompactClaudeUnchanged`.
- **Acceptance**: `go test ./internal/session/... -run 'Prompt|Compact'` passes; existing Prompt and
  Compact tests pass unedited.
- **Depends**: T003, T004.
- **Guardrails**: spec 018's `Manager.Type` stays unconditionally bracketed for every harness. Do
  not route it through `m.paste`: Claude's `BracketedPaste` is false, and plain paste would break
  018's multi-line guarantee. `m.paste` serves only `Prompt` and `Compact`. Add
  `TestTypeStaysBracketedForClaude` (fake `Calls()` shows `paste-buffer -p` for a Claude session's
  `Type`).

### T006 — `SetMode` refuses non-Claude sessions (FR-012a)

- **Files**: `internal/session/manager.go` (`SetMode` `:1297`), `internal/session/mode_test.go`.
- **Interface**: first check after the dead-state check: `if !m.specOf(s).RemoteControl { return Session{}, fmt.Errorf("change the mode of session %s: %w", s.ID, ErrModeUnavailable) }`.
- **Edge cases**: Claude session behaviour unchanged; a session whose start-command name is no longer
  configured resolves to Other and is refused the same way (it could not be switched today either:
  `resolveStartCommand` fails later).
- **Mirror**: the existing `ErrModeUnchanged` check.
- **Tests**: `TestSetModeRefusesCodex` (fake records zero SendKeys calls), existing mode tests unedited.
- **Acceptance**: `go test ./internal/session/... -run Mode` passes.
- **Depends**: T004.
- **Guardrails**: do not touch `commandForMode`.

### T007 — Codex trust seeding in `config.toml`

- **Files**: create `internal/session/trust_codex.go`, `internal/session/trust_codex_test.go`;
  edit `internal/session/trust.go` (`replaceFile` `:188` gains a `prefix string` parameter; its one
  caller passes `".claude.json.crswd-*"`).
- **Interface**:
  ```go
  var (
      ErrCodexConfigShape = errors.New("the Codex config names this directory in a shape crswd does not edit")
      ErrUntrustablePath  = errors.New("the directory cannot be written into a Codex config table header")
  )
  // CodexHome is $CODEX_HOME when absolute, else $HOME/.codex when HOME is absolute, else "".
  func CodexHome(env []string) string
  // SeedCodexTrust records dir as trusted in <codexHome>/config.toml.
  func SeedCodexTrust(codexHome, dir string) error
  func withCodexTrust(raw []byte, dir string) ([]byte, bool, error) // pure; changed=false when already trusted
  ```
- **Approach** (research D10), `SeedCodexTrust`:
  1. `codexHome == ""` → return nil. `os.Stat(codexHome)` not exist → return nil.
  2. `dir` not absolute → error (same text shape as `SeedTrust`). `dir` containing `"`, `\`, or any
     byte < 0x20 or 0x7f → `ErrUntrustablePath`.
  3. `path := filepath.Join(codexHome, "config.toml")`. Unlocked read; missing file → treat as empty.
     `withCodexTrust`; unchanged → return nil.
  4. Lock `path+".lock"` with `syscall.Flock` exactly as `SeedTrust` (`trust.go:79`), re-read, re-run
     `withCodexTrust`, unchanged → nil; else `replaceFile(path, out, mode, ".config.toml.crswd-*")`
     with the existing file's mode, or `0o600` when the file did not exist.
- **`withCodexTrust` rules** (line-based): split with `strings.SplitAfter(raw, "\n")`, so each line
  keeps its own ending (`"\n"`, `"\r\n"`, or none on a final unterminated line). Comparisons use the
  line with its ending trimmed.
  - `header := "[projects.\"" + dir + "\"]"`. A line whose `strings.TrimSpace` equals `header` is the
    table. More than one such line → `ErrCodexConfigShape`.
  - Any other line containing the substring `dir` → `ErrCodexConfigShape` (inline tables, dotted
    keys, literal-string headers and comments all land here; the refusal is deliberate).
  - Table found: scan following lines until a line whose trimmed form starts with `[` or EOF.
    A line matching `^\s*trust_level\s*=\s*"trusted"\s*(#.*)?$` → unchanged. A line matching
    `^\s*trust_level\s*=` → replace its content with `trust_level = "trusted"` and keep that line's
    original ending. No such line → insert `trust_level = "trusted"` right after the header, ending
    with the header line's own ending (`"\r\n"` after a CRLF header, `"\n"` otherwise; a header that
    is the unterminated last line first gains `"\n"`).
  - Table not found: append; if the file is non-empty and does not end with `"\n"`, append `"\n"`
    first; then append `"\n" + header + "\ntrust_level = \"trusted\"\n"`.
- **Edge cases** (each a test row): empty file; file with other projects; table with
  `trust_level = "untrusted"`; table with no trust_level and another key; table already trusted with a
  trailing comment; dir appearing in an inline table `projects = { "<dir>" = { … } }` → shape error;
  dir in a comment line → shape error; dir with `"` → `ErrUntrustablePath`; two identical headers for
  the dir → shape error; CRLF file with the table and an `untrusted` line → that line becomes
  `trust_level = "trusted"\r\n` and every other byte is unchanged; CRLF file with the table and no
  trust_level → inserted line ends `\r\n`; CRLF file without the table → appended block uses `"\n"`
  (Codex writes LF; the exact expected bytes are in the test row); file not ending in newline.
- **Mirror**: `trust.go` in full (lock, re-read, atomic replace, mode preservation, the
  `//nolint:gosec // G304` comments).
- **Tests**: `TestWithCodexTrust` (table above, asserting exact output bytes), `TestSeedCodexTrust`
  (missing home → no file created; home exists, no config → file created mode 0600 with the table;
  existing file mode 0644 preserved; already-trusted file byte-identical after), `TestCodexHome`
  (table like `TestClaudeConfigFile` in `trust_test.go`), and `TestSeedTrust` in `trust_test.go`
  passes unedited.
- **Acceptance**: `go test ./internal/session/... -run 'Codex|Trust'` passes; `golangci-lint run` clean.
- **Depends**: T001.
- **Guardrails**: do not parse TOML beyond these rules; never rewrite a line not named above.

### T008 — Seed trust per harness before typing

- **Files**: `internal/session/manager.go` (field + setter beside `claudeConfig` `:206`/`:267`;
  `start` `:1948` seeding call), `internal/session/supervisor.go` (`sendStart` `:329`),
  `internal/httpapi/server.go` (`:373-378`), `internal/session/trust_codex_test.go`.
- **Interface**:
  ```go
  func (m *Manager) SetCodexHome(path string)                       // field codexHome string
  func (m *Manager) seedTrustFor(spec harness.Spec, dir string) error
  ```
  `seedTrustFor`: `Claude` → `SeedTrust(m.claudeConfig, dir)`; `Codex` → `SeedCodexTrust(m.codexHome, dir)`;
  `Other` → nil. Replace both existing `SeedTrust(m.claudeConfig, s.WorkDir)` calls with
  `m.seedTrustFor(harness.For(harness.Of(template)), s.WorkDir)` using the template each function
  already resolved. In `server.go` after the existing `SetClaudeConfig` line:
  `srv.sessions.SetCodexHome(session.CodexHome(sessionEnv))`.
- **Edge cases**: a Codex seeding error fails the create exactly as a Claude one does
  (`TestCreateFailsWhenTrustCannotBeSeeded` is the pattern); an unset `codexHome` (tests, kubernetes
  mode) seeds nothing.
- **Mirror**: `TestCreateTrustsTheWorkDirBeforeStarting` in `internal/session/trust_test.go`.
- **Tests**: `TestCreateCodexTrustsTheWorkDir` (fake manager with a Codex start command named `codex`,
  temp `codexHome` with `config.toml`; after `Create` the table exists), `TestCreateCodexFailsOnShape`
  (config holds an inline table naming the dir → `Create` errors, fake shows no `send-keys`),
  `TestCreateClaudeDoesNotTouchCodexConfig`.
- **Acceptance**: `go test ./internal/session/... ./internal/httpapi/...` passes.
- **Depends**: T004, T007.
- **Guardrails**: the wiring line goes after the kubernetes-mode return in `httpapi.New`, beside
  `SetClaudeConfig`, never in `NewWith` (tests must not reach the real `~/.codex`).

### T009 — Stepped, verified quit for Codex restarts

- **Files**: `internal/session/supervisor.go` (`restartInto` `:366`), `internal/session/manager.go`
  (field `sleep`, sentinel, `Continue` `:2471-2510`), `internal/tmuxctl/fake.go` (test helper),
  `internal/session/harness_test.go`.
- **Interface**:
  ```go
  var ErrQuitUnconfirmed = errors.New("the session's process did not exit, so nothing was typed into it")
  // manager.go field, set in NewManagerWithClock to a ctx-aware time.Sleep:
  sleep func(ctx context.Context, d time.Duration) error
  const steppedQuitPresses = 5
  const steppedQuitWait = 1500 * time.Millisecond
  func (m *Manager) quitStepped(ctx context.Context, s Session) error
  // fake.go
  func (f *Fake) QuitAfterInterrupts(name string, n int) // the n-th "C-c" sent to name sets its pane command to "bash"
  ```
  `quitStepped`: loop `i < steppedQuitPresses`: `SendKeys(ctx, name, interruptKey)`; `m.sleep(ctx, steppedQuitWait)`;
  `infos, err := m.tmux.List(ctx)`; find the info whose `Name == s.TmuxName()`; absent → return
  `ErrSessionDead` wrapped; liveness `== tmuxctl.LivenessStopped` → return nil. The liveness field
  is `SessionInfo.Claude` at `controller.go:282`; if `grep -n 'Liveness$' internal/tmuxctl/controller.go`
  shows it renamed, use the new name.
  After the loop → `ErrQuitUnconfirmed` wrapped with the session id.
  `restartInto`: `if m.specOf(s).SteppedQuit { if err := m.quitStepped(ctx, s); err != nil { return err } } else { existing C-c C-c }`, then `m.sendStart`.
  `Continue` (`manager.go:2492-2506` today persists store → option → journal, then calls
  `restartInto`): when `m.specOf(s).SteppedQuit`, call `m.quitStepped(ctx, s)` **before**
  `store.SetConversation`. A quit error returns immediately, wrapped, with the store, the tmux option
  and the journal untouched. After a confirmed quit, persist exactly as today, then call
  `m.sendStart(ctx, s')` instead of `restartInto` (the process has already exited). Claude's
  `Continue` order is unchanged.
- **Edge cases**: `LivenessUnknown` keeps pressing (it is not confirmation); a `List` error returns
  immediately wrapped (do not type); ctx cancelled during sleep → return ctx error.
- **Mirror**: the `Clock` injection in `NewManagerWithClock` (`manager.go:395`).
- **Tests**: `TestRestartCodexQuitsThenTypes` (QuitAfterInterrupts 3 → exactly 3 `C-c` sends then the
  resume line), `TestRestartCodexNeverTypesWhenStillRunning` (pane stays `node`; 5 `C-c`, zero
  sends of the start line, `errors.Is(err, ErrQuitUnconfirmed)`), `TestRestartClaudeUnchanged`,
  `TestContinueCodexFailedQuitChangesNothing` (pane stays `node`: store `ConversationID`, the
  `OptionConversation` value and the journal record count equal their values before the call),
  `TestContinueCodexPersistsAfterQuit`.
  Tests set `m.sleep` to a no-op.
- **Acceptance**: `go test ./internal/session/... -run Restart` passes; Continue's existing tests pass unedited.
- **Depends**: T004.
- **Guardrails**: Claude's restart path keeps `SendKeys(interruptKey, interruptKey)` in one call.

### T010a — Codex conversation listing and transcript check (pure)

- **Files**: create `internal/session/conversation_codex.go`, `internal/session/conversation_codex_test.go`.
- **Interface**:
  ```go
  const (
      codexMetaReadLimit = 64 << 10
      codexScanLimit     = 500
      codexListLimit     = 50
  )
  func codexConversations(sessionsDir, workDir string) []Conversation
  func codexHasTranscript(sessionsDir, id, workDir string) bool
  func readCodexMeta(path string) (id, cwd string, ok bool)
  // codexRollout reports whether path is a regular, non-symlink file whose resolved parent lies
  // inside the resolved sessionsDir.
  func codexRollout(sessionsDir, path string) bool
  ```
  - `codexRollout`: `fi, err := os.Lstat(path)`; error or `!fi.Mode().IsRegular()` (a symlink is
    not regular under Lstat) → false. `root, err := filepath.EvalSymlinks(sessionsDir)`; `dir, err :=
    filepath.EvalSymlinks(filepath.Dir(path))`; either error → false. `rel, err := filepath.Rel(root, dir)`;
    error → false. `_, ok := containedIn(root, filepath.Join(rel, filepath.Base(path)))` (`conversation.go:249`)
    → return ok.
  - `codexConversations`: walk year → month → day directories in **descending name order** using
    `os.ReadDir`; skip any entry whose `Type()` is not a directory (`DirEntry.Type().IsDir()`, so a
    symlinked directory is skipped). In each day, entries with `Type().IsRegular()` whose name
    matches `^rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-f-]{36})\.jsonl$`, descending name
    order; stop after `codexScanLimit` files examined or `codexListLimit` matches. A match needs
    `codexRollout` true, `readCodexMeta` ok, `id` equal to the filename's uuid, `isConversationID(id)`,
    and `cwd == workDir`. `Modified` = file mtime. Sort as `Conversations` does (`conversation.go:158`).
  - `readCodexMeta`: open, read at most `codexMetaReadLimit` bytes, cut at the first `"\n"`; no
    newline within the limit → not ok; `json.Unmarshal` into
    `struct{Type string; Payload struct{ID, Cwd string} }` with tags `type`, `payload`, `id`, `cwd`;
    `Type != "session_meta"` → not ok.
  - `codexHasTranscript`: `isConversationID(id)` else false; `filepath.Glob(sessionsDir + "/*/*/*/rollout-*-" + id + ".jsonl")`;
    exactly one match, `codexRollout` true, and `readCodexMeta` cwd equals `workDir`.
- **Edge cases** (each a test row): a rollout with a 30 KB first line (built in-test with a long
  `base_instructions`) is read; a first line over 64 KiB is skipped; another cwd is skipped; a file
  named `rollout-…-<uuid>.jsonl` whose meta id differs is skipped; a symlinked rollout pointing
  outside `sessionsDir` is rejected; a symlinked rollout pointing to another file inside
  `sessionsDir` is also rejected (Lstat); a symlinked day directory is skipped; an unreadable day
  directory is skipped, not fatal; two globs matching one id → false; `sessionsDir` itself a
  symlink to a real tree → files inside it are accepted (resolution is on both sides).
- **Mirror**: `Conversations` and `HasTranscript` in `conversation.go`; their tests in `conversation_test.go`.
- **Tests**: `TestCodexConversations` (temp tree with 3 days, matches newest first, limit enforced
  with 60 matching files), `TestReadCodexMeta` (table), `TestCodexHasTranscript` (table),
  `TestCodexRolloutRejectsSymlinks`.
- **Acceptance**: `go test ./internal/session/... -run Codex` passes; existing `Conversations` tests unedited.
- **Depends**: T001.
- **Guardrails**: nothing read from a rollout is returned except `id`; never read past line 1.

### T010b — Per-harness conversation dispatch

- **Files**: `internal/session/conversation.go` (dispatchers), `internal/session/supervisor.go`
  (`:197` HasTranscript gate), `internal/session/manager.go` (`Continue` `:2471`),
  `internal/session/conversation_codex_test.go`.
- **Interface**:
  ```go
  func (m *Manager) ConversationsFor(h harness.Name, workDir string) []Conversation
  func (m *Manager) hasTranscriptFor(s Session, id string) bool
  ```
  Exhaustive switch, no default fall-through to Claude:
  `ConversationsFor`: `Claude` → `m.Conversations(workDir)`; `Codex` → `m.codexHome == ""` ? nil :
  `codexConversations(filepath.Join(m.codexHome, "sessions"), workDir)`; anything else → nil.
  `hasTranscriptFor`: `Claude` → `m.HasTranscript(id, s.WorkDir)`; `Codex` → `m.codexHome != "" &&
  codexHasTranscript(filepath.Join(m.codexHome, "sessions"), id, s.WorkDir)`; anything else → false.
  Replace the calls at `supervisor.go:197` and `manager.go:2471`.
  `Continue`: first check after resolving the session: `if harness.For(m.specOf(s).Name).ResumeArgs == nil`
  → return `fmt.Errorf("continue session %s: %w", s.ID, ErrInvalidResume)` before any store, option,
  journal or pane change.
- **Edge cases**: an Other session (`bash`) → `ConversationsFor` nil, `Continue` refused with zero
  fake `Calls()` and an unchanged record; a Codex session with `codexHome` unset → nil / false.
- **Mirror**: the existing call sites' error wrapping.
- **Tests**: `TestConversationsForDispatch` (table over the three harnesses),
  `TestContinueOtherRefusedEarly`, `TestContinueCodexChecksCodexTranscript`.
- **Acceptance**: `go test ./internal/session/...` passes.
- **Depends**: T004, T008 (for `m.codexHome`), T009, T010a.
- **Guardrails**: do not change `Conversations` or `HasTranscript`.

### T011 — `/proc` conversation discovery (pure)

- **Files**: create `internal/session/discover.go`, `internal/session/discover_test.go`.
- **Interface**:
  ```go
  var (
      ErrAmbiguousConversation = errors.New("more than one Codex conversation is open under this pane")
      ErrDiscoveryBounds       = errors.New("the process tree under this pane exceeds what discovery will read")
  )
  const (
      discoverMaxDepth     = 6
      discoverMaxProcs     = 64
      discoverMaxFDs       = 4096    // per process
      discoverChildrenRead = 64 << 10 // bytes read from one children file
  )
  // DiscoverCodexConversation returns the conversation id of the Codex rollout held open by
  // panePID or a descendant, "" when none is open.
  func DiscoverCodexConversation(procRoot string, panePID int, sessionsDir string) (string, error)
  ```
- **Approach**: BFS from `panePID`; children of `p` are the space-separated integers in the first
  `discoverChildrenRead` bytes of `<procRoot>/<p>/task/<p>/children` (missing file → no children;
  a file longer than that → `"", ErrDiscoveryBounds`); stop descending at depth 6. A 65th distinct
  pid queued → `"", ErrDiscoveryBounds` (overflow is an error, so no id is recorded from a partial walk). For each visited pid, open `<procRoot>/<p>/fd` and call
  `f.ReadDir(discoverMaxFDs + 1)` (open error → skip pid); more than `discoverMaxFDs` entries →
  `"", ErrDiscoveryBounds`. `os.Readlink` each entry (error → skip), and match the target against
  `^` + `regexp.QuoteMeta(filepath.Clean(sessionsDir))` + `/\d{4}/\d{2}/\d{2}/rollout-[^/]*-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.jsonl$`.
  Collect distinct ids. 0 → `"", nil`; 1 → that id; >1 → `"", ErrAmbiguousConversation`.
  `panePID <= 0` or `sessionsDir == ""` → `"", nil`.
- **Edge cases**: a `(deleted)` suffix on a link target does not match (correct: a deleted rollout is
  not resumable); the same id open on two fds or two pids → one id; a cycle in fake children files is
  bounded by the visited set.
- **Mirror**: package style of `trust.go`; test helpers that build temp trees in `conversation_test.go`.
- **Tests**: `TestDiscoverCodexConversation` with a fake proc root in `t.TempDir()`: dirs
  `<root>/100/task/100/children` = `"200"`, `<root>/200/task/200/children` = `"300"`,
  `<root>/300/fd/37` symlink → `<sessions>/2026/10/06/rollout-2026-10-06T00-11-13-01a10e8c-d5f5-7452-8561-233103b38287.jsonl`
  (create the target file). Rows: found at depth 2; none open; two distinct → ambiguous; same id twice
  → one; link outside sessionsDir → ""; depth 7 → ""; cycle; panePID 0; 4097 fd entries on one pid →
  `ErrDiscoveryBounds`; 65 child pids → `ErrDiscoveryBounds`; a children file over 64 KiB →
  `ErrDiscoveryBounds`.
- **Acceptance**: `go test ./internal/session/... -run Discover` passes.
- **Depends**: none beyond spec 018 merged (can run in parallel with T004–T010b).
- **Guardrails**: read-only; never opens a rollout file.

### T012 — Supervisor records discovered conversations

- **Files**: `internal/session/supervisor.go` (`judge` branch 3 `:155`), `internal/session/manager.go`
  (fields), `internal/session/journal.go` (event constant `:38-47`), `internal/session/discover_test.go`.
- **Interface**:
  ```go
  const journalDiscovered = "discovered" // journal.go, with a comment like journalContinued's
  // manager.go field, defaulted in NewManagerWithClock:
  findCodexConversation func(ctx context.Context, s Session) (string, error)
  func (m *Manager) hostCodexConversation(ctx context.Context, s Session) (string, error)
  func (s *Supervisor) discover(ctx context.Context, sess Session) error
  ```
  `hostCodexConversation`: `pid, err := m.tmux.PanePID(ctx, s.TmuxName())`; then
  `DiscoverCodexConversation("/proc", pid, filepath.Join(m.codexHome, "sessions"))`; `m.codexHome == ""` → `"", nil`.
  `discover`: only when `s.mgr.specOf(sess).Name == harness.Codex` and `sess.ConversationID == ""`.
  `id, err := s.mgr.findCodexConversation(ctx, sess)`; err → return it wrapped (the sweep joins it);
  `id == ""` → nil; `ValidateResume(id)` fails → return wrapped error; else, in this order:
  `tmux.SetOption(OptionConversation, id)`, then `journal.Append(reviveRecord(sess', journalDiscovered))`
  where `sess'` has `ConversationID = id`, then `store.SetConversation`, then
  `s.mgr.emit(FleetChanged, sess')`. The store is written last because it is what the next sweep
  reads: a failure at the option or the journal leaves `ConversationID` empty, so the next sweep
  rediscovers the same id and rewrites the option and journal (both idempotent for one id). Any step's
  error returns wrapped and stops the sequence.
  Call `s.discover` in `judge` branch 3 after the existing promotion logic, and return its error.
- **Edge cases**: discovery error does not change the healthy verdict (the session is not revived);
  an id already recorded is never replaced; journal replay of a `discovered` record restores the id
  (`Journal.Replay` keeps the latest record per id, `journal.go:198`).
- **Mirror**: `Continue`'s three writes (`manager.go:2490-2501`), in the reversed order above.
- **Tests**: `TestSweepRecordsCodexConversation` (fake finder returns id → store, option and journal
  carry it), `TestSweepIgnoresClaude` (finder never called), `TestSweepKeepsExistingConversation`,
  `TestSweepAmbiguousRecordsNothing`, `TestSweepBoundsErrorRecordsNothing` (finder returns
  `ErrDiscoveryBounds`), `TestSweepRetriesAfterOptionFailure` and `TestSweepRetriesAfterJournalFailure`
  (fail that step once: store stays empty; the next sweep records the id and every write carries it),
  `TestReplayRestoresDiscoveredConversation`.
- **Acceptance**: `go test ./internal/session/... -run 'Sweep|Replay'` passes.
- **Depends**: T003, T004, T008, T011.
- **Guardrails**: `findCodexConversation` is a field so Phase 4 can replace it; do not read `/proc`
  anywhere else.

### T013 — Dialog registry per harness

- **Files**: `internal/session/dialog.go`, `internal/session/dialog_test.go`, create
  `internal/session/testdata/codex-trust.pane`, `codex-hooks-review.pane`, `codex-approval.pane`,
  `codex-update.pane`, `codex-idle.pane`, `codex-working.pane` (contents: research §F1, F5, F6, F7,
  F8, F9 verbatim, each with no header line), and create `internal/session/testdata/README.md` (it
  does not exist yet: one table row per fixture naming its research §F source, the Codex version
  0.153.4 and the capture date 10/6/26); `internal/httpapi/dashboard.go` (`cardOf` `:468`,
  `effectiveDisplayState` `:513`, `:531`, and the `cardOf` callers in that file),
  `internal/httpapi/sessions.go` (`paneDialogState` `:784`), `internal/httpapi/partials_test.go`
  (its `cardOf` calls), `internal/httpapi/dashboard_test.go`.
- **Interface**:
  ```go
  func DetectDialogFor(h harness.Name, paneText string) (name string, dialog bool)
  ```
  `Claude` → existing `DetectDialog`. `Codex` → `codexDialogSignatures` then `codexSuspiciousMarkers`.
  `Other` → Claude registry first; no match → Codex registry.
  Codex signatures (name: phrase): `codex-trust`: `Do you trust the contents of this directory?`;
  `codex-hooks-review`: `Hooks need review`; `codex-approval`: `Would you like to run the following command?`;
  `codex-update`: `Update now (runs`. Codex suspicious markers: `Press enter to continue`,
  `Press enter to confirm or esc to cancel`, `Press enter to confirm or esc to go back`.
  Callers: `cardOf` is a free function with no `Server`, so it takes the harness as a parameter:
  `func cardOf(live session.Session, now time.Time, token, remoteCommand, paneText string, h harness.Name) sessionView`,
  and `effectiveDisplayState(live, now, paneText, h)`. Every `cardOf` caller computes `h` as
  `s.sessions.SpecOf(live).Name` (the callers in `dashboard.go` all have `s`); the calls in
  `partials_test.go` pass `harness.Claude`, which keeps their expectations unchanged. Export
  `func (m *Manager) SpecOf(s Session) harness.Spec` as a one-line wrapper of `specOf`. Find every
  call with `grep -rn 'cardOf(' internal/httpapi` (5 calls at planning time).
  `paneDialogState` uses `DetectDialogFor(s.sessions.SpecOf(resolved).Name, text)`.
- **Edge cases**: `codex-idle.pane` and `codex-working.pane` → `("", false)` under `Codex`. Matching
  is case-sensitive, so Claude's `Enter to confirm`/`Esc to cancel` markers never fire on Codex's
  lower-case `esc to interrupt`, and Codex's `Press enter to …` markers never fire on Claude's
  `Enter to …`; the Claude trust fixture (`workspaceTrustPane` in `dialog_test.go`) under `Codex` →
  `("", false)`. `DetectDialog` is unchanged for every existing test.
- **Mirror**: the package comment's rule "captured text only" (`dialog.go:51-60`); fixture loading in
  `internal/claudeauth/claudeauth_test.go` (`os.ReadFile("testdata/…")`).
- **Tests**: `TestDetectDialogForCodex` (each fixture → expected name), `TestCodexIdleAndWorkingAreNotDialogs`,
  `TestDetectDialogForOtherFallsBack`; in `internal/httpapi`: `TestCodexPaneOnTrustRendersBlocked`
  (session page for a Codex session whose fake pane is the trust fixture shows pill `blocked`).
- **Acceptance**: `go test ./internal/session/... ./internal/httpapi/...` passes.
- **Depends**: T004.
- **Guardrails**: do not add a phrase that is not in research §F.

### T014 — Browser create: the harness field

- **Files**: `internal/httpapi/actions.go` (`createFromBrowser` `:404`, field constants `:319`),
  `internal/httpapi/dashboard.go` (`previewCommands` `:395`), `internal/httpapi/view.go`
  (`createFormView` `:257`), create `internal/httpapi/harnessparam.go` and
  `internal/httpapi/harnessparam_test.go`, `internal/session/manager.go` (`StartCommandLineFor`),
  `internal/session/harness_test.go`, `internal/httpapi/actions_test.go`. `outcome.go` is not
  touched: reuse `outcomeBadMode`.
- **Interface**:
  ```go
  // harnessparam.go — the one parser every route uses for a harness value (T014, T016, T022b,
  // T024, T031 all call it; none reads the value with Get).
  var errHarnessParam = errors.New("the harness value is not one this daemon accepts")
  // parseHarness reads key from values. Absent (no entry) → (harness.Claude, nil). Exactly one
  // entry equal to "claude" or "codex" → that name. Two or more entries, an empty string, or any
  // other value (including "Codex") → ("", errHarnessParam).
  func parseHarness(values url.Values, key string) (harness.Name, error)
  const (fieldHarness = "harness"; harnessCodexValue = "codex"; harnessClaudeValue = "claude"; codexStartCommandName = "codex")
  // view.go createFormView gains:
  CodexCommand string // the preview line for the codex entry; "" means no harness control is rendered
  func (s *Server) codexOffered() (template string, ok bool) // cfg.StartCommands.Command("codex") and harness.Of(cmd) == harness.Codex
  ```
  In `createFromBrowser`, after `offersRemoteControlState`: `h, err := parseHarness(r.PostForm, fieldHarness)`.
  `err != nil` → `Deny(errCreateStateNotOffered)` + `outcomeBadMode`. `harness.Claude` → existing
  behaviour. `harness.Codex` → `codexOffered()` false → `Deny(errCreateStateNotOffered)` +
  `outcomeBadMode`; `mode == session.ModeRemote` → `Deny(errModeUnavailable)` + `outcomeBadMode`;
  else `startCommand = codexStartCommandName`.
  `CodexCommand` is filled in the same function that sets `Commands` (find it with
  `grep -n 'Commands:' internal/httpapi/*.go`), with `s.sessions.StartCommandLine`-equivalent
  output for the `codex` name: add `func (m *Manager) StartCommandLineFor(name, sessionName string) (string, error)`
  in `manager.go` that resolves `name` and calls the same `resumeFlagged` path as `StartCommandLine`
  (`manager.go:2131`) with empty resume.
- **Edge cases**: `harness=CODEX` refused (exact match); `harness=codex&harness=codex` refused
  (duplicate); `harness=` (empty) refused; `harness=codex` when `codex` names a non-Codex binary →
  refused; a refused create runs no tmux command (assert fake `Calls()` empty).
- **Mirror**: `offersRemoteControlState` (`actions.go:373`) and the remote-mode resolution (`:480-495`).
- **Tests**: `harnessparam_test.go`: `TestParseHarness` (table: absent, `claude`, `codex`, `Codex`,
  empty, duplicate, `codex`+`claude`, `evil`). `harness_test.go`: `TestStartCommandLineFor` (the
  `codex` name renders T004's exact fresh line). `actions_test.go`: `TestBrowserCreateCodex` (fake: start command `codex` used, typed line
  carries both required flag groups), `TestBrowserCreateCodexRefusedWhenNotConfigured`,
  `TestBrowserCreateCodexRefusesRemote`, `TestBrowserCreateHarnessUnknownValueRefused`,
  `TestBrowserCreateHarnessDuplicateRefused`,
  `TestBrowserCreateClaudeUnchanged` (no `harness` field → identical calls to today).
- **Acceptance**: `go test ./internal/httpapi/... -run BrowserCreate` passes; refusals never 500.
- **Depends**: T004.
- **Guardrails**: the field accepts two literals; it is never looked up as a configured name.

### T014a — The canonical radio group component

No radio or segmented control exists (`grep -rn 'type="radio"' web/templates` prints nothing at
planning time). T015 needs one; this task defines it once.

- **Files**: `docs/components.md` (new section "Radio group" after the switch section; find it with
  `grep -n -i 'switch' docs/components.md`), `web/static/crswd.css`, `internal/httpapi/partials_test.go`
  (a CSS presence test only).
- **Markup contract** (copy into components.md exactly):
  ```html
  <fieldset class="radio-group">
    <legend class="radio-group-legend">Runtime</legend>
    <label class="radio-option"><input type="radio" name="harness" value="claude" checked> <span>Claude Code</span></label>
    <label class="radio-option"><input type="radio" name="harness" value="codex"> <span>Codex</span></label>
    <p class="field-hint">…</p>
  </fieldset>
  ```
  Native `<input type="radio">` inside `<label>`: no ARIA roles are added (native semantics already
  give the group, the arrow-key behaviour and the checked state).
- **CSS**: `.radio-group` (no border, padding 0, margin 0 like the switch's container),
  `.radio-group-legend` (same font, size and colour tokens as the switch's label text),
  `.radio-option` (inline-flex, gap and min-height `44px` matching the switch's tap target, cursor
  pointer), `.radio-option input` (`accent-color` = the switch's on-state colour token),
  `.radio-option input:focus-visible` (the same focus ring rule the switch uses). Copy the token names
  from the switch rules in `crswd.css` (`grep -n 'switch' web/static/crswd.css`); introduce no new token.
- **Edge cases**: wraps to one option per line under the phone breakpoint already used by the create
  form (no horizontal scroll); disabled options use the switch's disabled opacity rule.
- **Tests**: `TestRadioGroupStylesExist` (served `crswd.css` contains `.radio-group`,
  `.radio-option` and `:focus-visible` for `.radio-option input`).
- **Acceptance**: `go test ./internal/httpapi/... -run RadioGroup` passes.
- **Depends**: none in code (ordering: before T015).
- **Guardrails**: no template uses the component in this task; no inline style; CSP unchanged.

### T015 — Create form markup, preview script, components doc

- **Files**: `web/templates/partials/create-form.html` (near `:277` and `:364`), `web/static/crswd.js`
  (`previewCommand` `:488`), `web/static/crswd.css`, `docs/components.md` (Create form / Modal
  section), `internal/httpapi/partials_test.go`.
- **Interface**: when `.CodexCommand` is non-empty, render before the remote switch a radio group
  `name="harness"` with two inputs, values `claude` (checked) and `codex`, labels `Claude Code` and
  `Codex`, one hint line (spec 011 FR-013), and add `data-command-codex="{{ .CodexCommand }}"` to the
  `<pre data-command-preview>`. `previewCommand` reads the checked `harness` radio: `codex` → show
  `data-command-codex` (substituting `{name}` the same way) and disable the remote switch
  (`disabled` attribute + `aria-disabled="true"`); `claude` → existing behaviour and re-enable it.
- **Edge cases**: no JS → both radios submit and the server rules in T014 apply; `.CodexCommand` empty
  → the markup is byte-identical to today (assert).
- **Mirror**: the radio group markup contract T014a wrote into `docs/components.md`, verbatim, with
  legend `Runtime`.
- **Tests** (`partials_test.go`): `TestCreateFormOffersCodexWhenConfigured`,
  `TestCreateFormUnchangedWithoutCodex` (existing exact-markup assertions pass unedited),
  `TestCreateFormScriptReadsHarness` (grep the served `crswd.js` for `name="harness"` handling, the
  same way existing script tests do).
- **Acceptance**: `go test ./internal/httpapi/... -run CreateForm` passes.
- **Depends**: T014, T014a.
- **Guardrails**: design tokens only (docs/design-system.md); no inline style; CSP unchanged.

### T016 — Show the harness on cards, the session page and the API

- **Files**: `internal/httpapi/view.go` (`sessionView` `:27`), `internal/httpapi/dashboard.go`
  (`cardOf` `:468`, session page conversations `:644`), `internal/httpapi/conversations.go` (`:67`),
  `internal/httpapi/sessions.go` (`sessionEntry` `:168`), `web/templates/partials/session-card.html`
  (`:96`), `internal/httpapi/dashboard_test.go`, `internal/httpapi/sessions_test.go`.
- **Interface**: `sessionView.Harness string`: `Claude` → `"Claude Code"`, `Codex` → `"Codex"`,
  `Other` → `""` (do not use `Label()` here, it returns `"Other"`); the template renders the span
  only when `.Harness` is non-empty. `sessionEntry.Harness string`
  `json:"harness,omitempty"` set to `string(spec.Name)` for Claude and Codex, empty for Other.
  The card renders `<span class="card-harness">{{ .Harness }}</span>` inside the existing
  `card-mode` paragraph. The mode row (`local`/`remote`) renders only when the harness is Claude.
  `conversationsForDir(now, dir)` gains a `harness.Name` parameter and calls
  `s.sessions.ConversationsFor(h, dir)`; the session page passes the session's harness. The signed
  route `GET /sessions/conversations?dir=` reads `harness` with `parseHarness(r.URL.Query(), "harness")`
  (T014); an error → 400 via the route's existing bad-request path.
- **Edge cases**: adopted sessions with no start command → Other → no harness span rendered
  (`TestCardOmitsHarnessForOther`).
- **Mirror**: how `StartCommand` flows into `sessionView` and `sessionEntry` today.
- **Tests**: `TestCardShowsHarness`, `TestSessionEntryHarnessField` (JSON contains
  `"harness":"codex"`; a Claude entry contains `"harness":"claude"`; Other has no key),
  `TestConversationsRouteHarness` (400 on `harness=x`), `TestSessionPageListsCodexConversations`.
- **Acceptance**: `go test ./internal/httpapi/...` passes; `deploy/crswd-api` needs no change.
- **Depends**: T010b, T013, T014 (`parseHarness`).
- **Guardrails**: keep `start_command` in the API entry unchanged.

### T017 — Acceptance: a Codex session end to end (quickstart)

- **Files**: `cmd/crswd/quickstart_test.go`.
- **Interface**: `func (h *host) writeCodexShim()` beside `writeShim` (`:367`): a `codex` script in
  `h.shimDir` that prints `shimReady`, then `printf 'shim-argv:%s\n' "$*"`, then echoes stdin like the
  claude shim. New test `TestQuickstartCodexSession`: create `<h.home>/.codex` (the daemon's
  `HOME` is `h.home`, so `CodexHome` resolves there and trust seeding has a home to write into),
  start the daemon with `CRSW_START_COMMANDS=codex=codex --dangerously-bypass-approvals-and-sandbox`,
  create through the signed API with `start_command=codex`, wait for `shimReady`.
- **Assertions**: `@crswd-binary` is `codex|node` (mirror `TestSessionCarriesWhatRevivalNeeds` `:2425-2460`);
  the pane shows `shim-argv:--no-alt-screen -c check_for_update_on_startup=false --dangerously-bypass-approvals-and-sandbox`;
  `<h.home>/.codex/config.toml` contains `[projects."<workdir>"]` and `trust_level = "trusted"`; the
  `GET /sessions` entry has `"harness":"codex"`.
- **Edge cases**: the shim reports `pane_current_command` = `codex` (not `node`), which the set
  accepts — assert `List` liveness is running.
- **Acceptance**: `go test -tags quickstart ./cmd/crswd -run Codex` passes (needs tmux, jq, port 8765
  free; see AGENTS.md tag table).
- **Depends**: T004, T008, T016.
- **Guardrails**: the real `codex` binary is never invoked; `PATH` has the shim dir first.

### T018 — Documentation and the full gate

- **Files**: `docs/harnesses.md` (rewrite "research" into the shipped design, answering its three
  questions with research D1/D3/D13), `docs/security.md` (amend the workspace-trust section for D10;
  add D11's bounded read as an amendment to spec 013 FR-025; add D5's update-check rationale; name
  `--dangerously-bypass-approvals-and-sandbox` and `--dangerously-bypass-hook-trust` as the operator's
  Codex equivalents of `--dangerously-skip-permissions`), `README.md` (configuration: a `codex=`
  example; prerequisite: Codex 0.153.4 tested; `CODEX_HOME` in `CRSW_SESSION_ENVIRONMENT` when
  non-default), `config.example` (commented `codex=` example), `docs/fixes-log.md` is NOT touched.
- **Acceptance**: `go test ./internal/config/...` passes (`TestEnvExampleNamesEveryVariable` and docs
  tests), and the full gate passes: `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./... && go test -tags quickstart ./cmd/crswd && golangci-lint run`.
- **Depends**: T001–T017.
- **Guardrails**: `AGENTS.md` stays ≤ 150 lines; `docs/security.md` changes are additive.

---

## Phase 2: Sign-in (notebook `ralph/plans/codex-auth`)

### T020 — `internal/codexauth`

- **Files**: create `internal/codexauth/codexauth.go`, `codexauth_test.go`, `testdata/device-code-cli.pane`
  (research F2), `testdata/device-code-tui.pane` (F3), `testdata/signed-out.pane` (F4),
  `testdata/README.md` (provenance and the placeholder code `ABCD-EFGH1`).
- **Interface**:
  ```go
  type Kind string
  const (KindSignedOut Kind = "signed-out"; KindDeviceCode Kind = "device-code")
  type Prompt struct { Kind Kind; URL string; Code string }
  func (p Prompt) String() string // Kind and URL host only; never Code
  func DetectPrompt(pane string) (*Prompt, bool)
  ```
  `KindSignedOut` when the pane contains `Sign in with ChatGPT to use Codex as part of your paid plan`.
  `KindDeviceCode` when it contains `Enter this one-time code`: `URL` = the first trimmed line starting
  with `https://auth.openai.com/`; `Code` = the first non-empty trimmed line after the line containing
  `Enter this one-time code`, accepted only if it matches `^[A-Z0-9]{4,}-[A-Z0-9]{4,}$`, else
  `Code = ""`. Device-code wins when both match.
- **Edge cases**: URL wrapped across lines is not expected (the URL is 36 characters); missing URL →
  `URL = ""` and still detected; a pane quoting the phrase inside a code block (session reading this
  repo) → reuse `claudeauth`'s unquoted check approach (`claudeauth.go:211-232`) by copying the
  function, not importing it.
- **Mirror**: `internal/claudeauth/claudeauth.go` structure and tests.
- **Tests**: each fixture → expected Kind/URL/Code; `TestPromptStringOmitsCode`; quoted-phrase pane → not detected.
- **Acceptance**: `go test ./internal/codexauth/...` passes.
- **Depends**: Phase 1 merged.
- **Guardrails**: the package imports only stdlib.

### T021 — Codex sign-in flow in `loginrelay`

- **Files**: `internal/loginrelay/loginrelay.go`, `internal/loginrelay/loginrelay_test.go`.
- **Interface**:
  ```go
  const CodexWindowName = "crswd-login-codex"
  var ErrNoCodeToDeliver = errors.New("this sign-in takes no code from the dashboard")
  func NewCodex(tmux Controller, startCommand, workDir string, env []string) (*Relay, error)
  // State gains: Device *codexauth.Prompt
  ```
  `Relay` gains `flow harness.Name`, `window string` and `executable string`. `New` sets
  `flow = harness.Claude`, `window = WindowName` (unchanged behaviour). `NewCodex` sets
  `flow = harness.Codex`, `window = CodexWindowName`. Every use of `WindowName` inside methods
  becomes `r.window`.
  `executable` is the configured command's first token as written (`strings.Fields(startCommand)[0]`),
  so an absolute path such as `/home/nctiggy/sf-cli/bin/codex` is kept; `binaryOf` (`loginrelay.go:179`)
  keeps returning the base name and is used only for the existing non-empty check. `NewCodex`
  refuses a relative first token containing `/` (`ErrNoBinary`); a bare name is kept as is.
  `Start` types `r.executable + " -c check_for_update_on_startup=false login --device-auth"` for Codex
  (top-level `-c` before the subcommand, the form `codex --help` documents; research M10 measured
  `-c` placement only for `resume`).
  `State` for Codex uses `codexauth.DetectPrompt` into `State.Device`. `Deliver` for Codex returns
  `ErrNoCodeToDeliver`. `SignedIn` for Codex runs `exec.CommandContext(ctx, r.binary, "login", "status")`
  with `r.env`: exit 0 → true; exit 1 and stdout contains `Not logged in` → false; anything else →
  error wrapped `"read the Codex sign-in state"`. `SignedIn` runs `r.executable`, never the base name.
  The Claude flow keeps using `r.binary` exactly as today.
- **Edge cases**: `CodexWindowName` must not be adoptable: extend the existing window-name test in
  `internal/loginrelay/loginrelay_test.go` (`grep -n 'WindowName' internal/loginrelay/loginrelay_test.go`)
  to cover both names. An absolute Codex path outside the daemon's `PATH` is the one executed.
- **Mirror**: the existing Claude methods line for line.
- **Tests**: Codex `Start` argv (the typed line begins with the absolute path); `State` with each
  fixture; `Deliver` refusal; `SignedIn` via a fake binary script at an absolute path in
  `t.TempDir()` that is not on `PATH`, printing each M14 output with each exit code.
- **Acceptance**: `go test ./internal/loginrelay/...` passes; existing tests unedited.
- **Depends**: T020.
- **Guardrails**: the Claude flow's typed line and window name do not change.

### T022a — Relays and auth cache become per-harness maps (Claude only)

A pure refactor: behaviour is identical and every commit stays green.

- **Files**: `internal/httpapi/server.go` (`signin` field `:296`, wiring `:373-388`),
  `internal/httpapi/authstatus.go` (`authCache` `:92-176`), `internal/httpapi/signin.go`,
  `internal/httpapi/authstatus_test.go`, `internal/httpapi/signin_test.go`,
  `internal/httpapi/signinview_test.go`, `internal/httpapi/executionmode_test.go`.
- **Interface**:
  ```go
  // server.go
  signin map[harness.Name]signInRelay   // nil map = no relay for any harness
  authCaches map[harness.Name]*authCache // replaces the single cache field
  func (s *Server) signInFor(h harness.Name) signInRelay        // nil when absent
  func (s *Server) setSignIn(h harness.Name, r signInRelay)       // allocates the map on first use
  func (s *Server) authCacheFor(h harness.Name) *authCache        // creates on first use, under s.mu (the mutex the cache uses today)
  ```
  Every existing read of `s.signin` becomes `s.signInFor(harness.Claude)`; every write
  (`srv.signin = x`) becomes `srv.setSignIn(harness.Claude, x)`; every `s.signin == nil` check
  becomes `s.signInFor(harness.Claude) == nil`. The single cache becomes `s.authCacheFor(harness.Claude)`.
  Find every site with `grep -rn '\.signin\b' internal/httpapi` and `grep -rn 'authCache' internal/httpapi`
  (at planning time: `authstatus.go`, `server.go`, `signin.go`, and the four test files above).
- **Edge cases**: a nil relay in tests (`executionmode_test.go:59-74`) still reads as nil.
- **Tests**: no new behaviour; `TestSignInForDefaultsToNil`, `TestAuthCacheForIsPerHarness` (two
  harnesses get distinct caches; the same harness twice gets the same pointer).
- **Acceptance**: `go test ./internal/httpapi/...` passes with only the mechanical edits above in
  existing tests.
- **Depends**: T021.
- **Guardrails**: no route, response or audit record changes in this task.

### T022b — Codex relay wiring and the auth status route

- **Files**: `internal/httpapi/server.go` (wiring), `internal/httpapi/authstatus.go`
  (`GET /dashboard/auth` handler), `internal/httpapi/authstatus_test.go`.
- **Interface**: in `httpapi.New`, after the Claude relay is wired: when
  `cfg.StartCommands.Command("codex")` exists and `harness.Of(cmd) == harness.Codex`,
  `relay, err := loginrelay.NewCodex(tmux, cmd, relayWorkDir(cfg), sessionEnv)`; error →
  `srv.report(...)` exactly as the Claude relay's error path; else `srv.setSignIn(harness.Codex, relay)`.
  `GET /dashboard/auth`: `h, err := parseHarness(r.URL.Query(), "harness")`; err → 400 with the
  route's existing bad-request handling; `s.signInFor(h) == nil` → `{"state":"unknown"}`; else the
  existing logic over `s.authCacheFor(h)` and `s.signInFor(h)`.
- **Edge cases**: no `harness` query → byte-identical JSON to today; `harness=codex` with no Codex
  configured → `{"state":"unknown"}`; `harness=codex&harness=claude` → 400.
- **Tests**: `TestDashboardAuthHarnessParam` (table: absent, claude, codex configured, codex not
  configured, duplicate, unknown), `TestDashboardAuthDefaultUnchanged`.
- **Acceptance**: `go test ./internal/httpapi/... -run DashboardAuth` passes.
- **Depends**: T022a, T014 (`parseHarness`).
- **Guardrails**: wiring stays in `New`, after the kubernetes-mode return, never in `NewWith`.

### T022c — Per-harness create gate

- **Files**: `internal/httpapi/authstatus.go` (`createRefusedWhileSignedOut` `:245`,
  `errCreateSignedOut` `:219`), `internal/httpapi/actions.go` (`:514`), `internal/httpapi/sessions.go`
  (signed create gate call; `grep -n 'createRefusedWhileSignedOut' internal/httpapi/sessions.go`),
  `internal/httpapi/authstatus_test.go`.
- **Interface**:
  ```go
  var errCreateCodexSignedOut = errors.New("this host is signed out of Codex, so the session was refused")
  func (s *Server) createRefusedWhileSignedOut(ctx context.Context, h harness.Name) bool
  ```
  Both create paths compute `h := harness.Of(command)` from the resolved start command (the browser
  path after T014's harness resolution; the signed path from `cfg.StartCommands.Command(req.StartCommand)`)
  and call the gate with it. The denial reason is `errCreateSignedOut` for Claude and
  `errCreateCodexSignedOut` for Codex; `Other` is never gated.
- **Tests**: `TestCreateCodexGatedOnCodexAuth`, `TestCreateClaudeNotGatedOnCodex`,
  `TestCreateOtherNeverGated`, existing gate tests unedited apart from the added argument.
- **Acceptance**: `go test ./internal/httpapi/...` passes.
- **Depends**: T022b.
- **Guardrails**: a refused create still runs no tmux command.

### T023 — `needs-auth` for Codex panes

- **Files**: `internal/httpapi/dashboard.go` (`effectiveDisplayState` `:513-523`), `internal/httpapi/dashboard_test.go`.
- **Interface**: for harness Codex, `codexauth.DetectPrompt(paneText)` found → `session.DisplayNeedsAuth`;
  Claude keeps `claudeauth.DetectPrompt`; Other checks both.
- **Tests**: `TestCodexSignedOutPaneRendersNeedsAuth` (F4 fixture), `TestCodexDevicePaneRendersNeedsAuth`
  (F3), and the rendered page does not contain the code `ABCD-EFGH1`.
- **Acceptance**: `go test ./internal/httpapi/... -run NeedsAuth` passes.
- **Depends**: T020, T013.
- **Guardrails**: the card's pill never renders the code or URL. The pane viewer shows a session's
  pane as it does for Claude today (spec FR-019 scope: research "Critic findings rejected", R-3).

### T024 — Sign-in routes and panel for Codex

- **Files**: `internal/httpapi/signin.go` (routes `:47-55`, handlers), `internal/httpapi/signinview.go`
  (or wherever `signInPanelFor` lives — grep `func signInPanelFor`), `web/templates/partials/signin-panel.html`,
  `internal/audit/audit.go` (no new actions; add a `harness` detail only if the audit record type has
  a detail field — grep `Detail`), `internal/httpapi/signinview_test.go`.
- **Interface**: each sign-in POST and `GET /dashboard/signin/view` read the harness with
  `parseHarness` (T014) from `r.PostForm` / `r.URL.Query()`; error → 400. The panel model gains
  `Harness harness.Name`, `Code string`, `DeviceURL string`. Every form the panel renders carries
  `<input type="hidden" name="harness" value="{{ .Harness }}">` (Claude's forms too, value `claude`,
  so a Codex panel's Start or Cancel can never reach the Claude relay). For Codex the template renders
  the link (`rel="noopener noreferrer"`, same as Claude's), the code in a `<code>` element, a waiting
  line, and no code form.
  `POST /dashboard/signin/code` with `harness=codex`: no 409 path exists; keep this file's
  convention, `AuditFrom(ctx).Deny(errSignInCodeNotTaken.Error())` (new sentinel
  `errSignInCodeNotTaken = errors.New("this sign-in takes no code from the dashboard")`) then
  `s.redirectSignIn(w, r, outcomeSignInRefused)`.
  Redirect marker: `redirectSignIn` (`signin.go:354` area, `querySignInOpen`/`signInOpenMarker`)
  writes `signin=open` for Claude (unchanged) and `signin=codex` for Codex; add
  `const signInOpenCodexMarker = "codex"` and pass the harness into `redirectSignIn`.
- **Edge cases**: the code never appears in the audit record, a redirect URL or a log line (assert by
  scanning the test audit sink for `ABCD-EFGH1`).
- **Mirror**: spec 015's panel and its no-leak tests (`signinview_test.go:177`).
- **Tests**: `TestCodexPanelShowsLinkAndCode`, `TestCodexPanelHasNoCodeForm`,
  `TestCodexPanelFormsCarryHarness` (every `<form>` in the Codex panel has the hidden field with
  value `codex`), `TestCodexStartReachesOnlyCodexRelay` (two fake relays; POST start with
  `harness=codex` calls only the Codex fake), `TestCodexCodeRouteRefused` (303 with the refused
  outcome), `TestCodexRedirectMarker` (`signin=codex`), `TestSignInHarnessDuplicateRefused`,
  `TestCodexCodeNeverAudited`.
- **Acceptance**: `go test ./internal/httpapi/... -run SignIn` passes.
- **Depends**: T022c.
- **Guardrails**: Claude's panel markup changes only by the added hidden field; its redirect marker
  stays `signin=open`.

### T025 — Header view, Codex pill and script

- **Files**: `internal/httpapi/view.go` (new `headerView`), `web/templates/partials/header.html`,
  `web/templates/dashboard.html`, `web/templates/session.html`, `web/templates/settings.html`,
  `web/templates/not-found.html`, `internal/httpapi/dashboard.go` (`fleetView` `:101`,
  `sessionPageView` `:156`, their construction sites), `internal/httpapi/settings.go`
  (`settingsView` `:84`), `internal/httpapi/browser.go` (`notFoundView` `:201`),
  `internal/httpapi/restart.go` (`:208`), `internal/httpapi/update.go` (`:547`),
  `internal/httpapi/render_test.go`, `internal/httpapi/partials_test.go`, `web/static/crswd.js`
  (`:1384-1557`), `web/static/crswd.css`.
  The construction sites are every match of
  `grep -rn 'fleetView{\|sessionPageView{\|settingsView{\|notFoundView{' internal/httpapi`
  (the files above at planning time).
- **Interface**:
  ```go
  // view.go
  type headerView struct {
      Operator        *access.VerifiedOperator
      CodexConfigured bool
  }
  func (s *Server) headerFor(op *access.VerifiedOperator) headerView // CodexConfigured = s.signInFor(harness.Codex) != nil
  ```
  Each of the four view types gains `Header headerView`; every construction site that sets
  `Operator: x` also sets `Header: s.headerFor(x)` (the `Operator` field stays). The four page
  templates change `{{ template "header" .Operator }}` to `{{ template "header" .Header }}`.
  `header.html` changes `.Email` to `.Operator.Email` (both occurrences at `:91`), and adds, when
  `.CodexConfigured`, a second `<button … data-auth-pill data-harness="codex">codex auth: checking</button>`
  beside the existing pill (copy the existing pill's attributes and classes).
- **Script contract** (crswd.js auth module): one state record per pill,
  `{pill, harness, timer, generation}`, where `harness` is the pill's `data-harness` or `"claude"`
  when absent. Each record polls on its own timer, fetching `/dashboard/auth` with no query for
  `claude` (unchanged URL) and `?harness=codex` for codex. The shared dialog has one module variable
  `activeHarness` (null when closed). Clicking a pill sets `activeHarness` to its harness, increments
  a module-level `dialogGeneration`, and fetches `/dashboard/signin/view?harness=<h>`; a response whose
  captured generation differs from the current `dialogGeneration` is discarded (stale), and an
  in-flight fetch is aborted with its `AbortController` on the next click. Only the pill whose harness
  equals `activeHarness` may refresh the open dialog after its poll. A form submitted from the dialog
  reloads `/dashboard/signin/view?harness=<activeHarness>`. On load, `?signin=open` opens the Claude
  dialog (unchanged) and `?signin=codex` opens the Codex dialog.
- **Edge cases**: Codex not configured → the rendered header is byte-identical to today (assert);
  `TestHeaderHasExactlyTwoAnchors` passes unedited (the pill is a `<button>`); a click on the Codex
  pill while a Claude view fetch is in flight shows only the Codex panel.
- **Mirror**: spec 015's pill and spec 016's module structure in `crswd.js`.
- **Tests**: `TestHeaderViewOnEveryPage` (each of the four pages renders the operator email),
  `TestHeaderCodexPillWhenConfigured`, `TestHeaderUnchangedWithoutCodex`,
  `TestAuthScriptPerPillState` (served `crswd.js` contains `activeHarness`, `dialogGeneration`,
  `AbortController` and `signin=codex` handling, the way existing script tests grep the file).
- **Acceptance**: `go test ./internal/httpapi/...` passes.
- **Depends**: T024.
- **Guardrails**: CSP unchanged; tokens only; no page template changes beyond the one header line.

### T026 — Docs and gate

- **Files**: `docs/auth-and-sessions.md` (a "Relaying Codex's device sign-in" section beside the
  Claude one: no paste-back, code display rules, `login status` exit codes), `docs/components.md`
  (Header: `headerView`, the second pill, the per-pill state contract).
- **Acceptance**: full gate passes (see top of file).
- **Depends**: T020–T025.

---

## Phase 3: Usage (notebook `ralph/plans/codex-quota`)

### T034 — Spike: why quota-axi says `auth_required` for Codex

- **Files**: append findings to `specs/019-codex-runtime/research.md` under a new `M19` row only.
- **Steps**: run `quota-axi --provider codex --full`, `quota-axi auth`, and read
  `~/.local/share/axi-tools/node_modules/quota-axi/dist/src/providers/codex.js` around its oauth source;
  record the failing request and status. Change no crswd code. If the cause is in quota-axi, record
  "fix belongs to quota-axi" and the evidence.
- **Acceptance**: `grep -n '^| M19 ' specs/019-codex-runtime/research.md` prints one line.
- **Depends**: none. This task may run before T030; it is listed first so the loop's topmost-open rule picks it before the Phase 2 check.

---

### T030 — `quota.ReadProvider`

- **Files**: `internal/quota/quota.go` (`:32-33`, `:82-83`, `Read` `:132`), `internal/quota/quota_test.go`.
- **Interface**:
  ```go
  var (ErrNoProvider = errors.New("quota: no such provider in the cache"); ErrNoWindow = errors.New("quota: the provider has no such window"))
  var (ErrNoClaudeProvider = ErrNoProvider; ErrNoWeeklyWindow = ErrNoWindow) // kept for existing callers
  func ReadProvider(path, provider, window string) (Reading, error)
  func Read(path string) (Reading, error) { return ReadProvider(path, "claude", "seven_day") }
  ```
- **Tests**: existing tests unedited; `TestReadProviderCodexWeekly` (fixture with a `codex` provider
  holding `five_hour` and `weekly`); `TestReadProviderMissing`.
- **Acceptance**: `go test ./internal/quota/...` passes.
- **Depends**: Phase 2 merged.

### T031 — Route `?harness=codex`

- **Files**: `internal/httpapi/quotastatus.go` (`dashboardQuota` `:84`), `internal/httpapi/quotastatus_test.go`.
- **Interface**: `h, err := parseHarness(r.URL.Query(), "harness")` (T014); err → 400.
  `harness.Claude` → `ReadProvider(path,"claude","seven_day")` (today); `harness.Codex` →
  `ReadProvider(path,"codex","weekly")`. Response shape unchanged.
- **Tests**: `TestDashboardQuotaCodex`, `TestDashboardQuotaHarnessInvalid` (unknown and duplicate),
  existing tests unedited.
- **Acceptance**: `go test ./internal/httpapi/... -run Quota` passes.
- **Depends**: T030.

### T032 — Codex meter in the header

- **Files**: `web/templates/partials/header.html` (`:90-95`), `web/static/crswd.js` (`:1561-1670`),
  `web/static/crswd.css`, `internal/httpapi/partials_test.go`.
- **Interface**: each meter and its label sit in one keyed wrapper, and the script finds the label
  only inside its own wrapper:
  ```html
  <span class="quota" data-quota data-harness="claude">  <!-- wraps today's meter and label unchanged -->
    <meter data-quota-meter …></meter> <span data-quota-label>…</span>
  </span>
  <span class="quota" data-quota data-harness="codex">   <!-- only when .CodexConfigured -->
    <meter data-quota-meter aria-label="Weekly Codex quota used" …></meter>
    <span data-quota-label>codex quota: checking</span>
  </span>
  ```
  Copy today's meter and label element attributes into the Claude wrapper unchanged. The module
  iterates `document.querySelectorAll('[data-quota]')`, and for each wrapper `w` uses
  `w.querySelector('[data-quota-meter]')` and `w.querySelector('[data-quota-label]')`; it fetches
  `/dashboard/quota` with no query for `claude` and `?harness=codex` for codex. Label text
  `Codex weekly N% used`; the unknown text `codex quota: unknown` (NC-1: kept visible).
- **Tests**: `TestHeaderCodexMeterWhenConfigured`, `TestHeaderMeterWrappedWithoutCodex` (the Claude
  meter and label are inside one `data-quota` wrapper; their own attributes unchanged),
  `TestQuotaScriptFetchesPerHarness` (served `crswd.js` uses `querySelectorAll('[data-quota]')`).
- **Acceptance**: `go test ./internal/httpapi/... -run 'Header|Quota'` passes.
- **Depends**: T031.

### T033 — Docs and gate

- **Files**: `docs/components.md` (Header: the second meter), `specs/016-weekly-quota-bar/spec.md`
  (append a dated note that `?harness=` exists; no FR edits).
- **Acceptance**: full gate passes.
- **Depends**: T030–T032.

## Phase 4a: Kubernetes (notebook `ralph/plans/codex-k8s`)

### T041 — `CODEX_HOME` passes into session pods

- **Files**: `internal/sessionpod/sessionpod.go` (`passThrough` `:81`), `internal/sessionpod/sessionpod_test.go`.
- **Interface**: `passThrough = []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME"}` with the comment extended.
- **Tests**: extend the existing pass-through test (grep `CLAUDE_CONFIG_DIR` in `sessionpod_test.go`).
- **Acceptance**: `go test ./internal/sessionpod/...` passes.
- **Depends**: spec 017 S7 merged. Not O-1 (research D15: pass-through and lookup do not depend on
  the credential measurement).

### T042 — In-pod conversation lookup

- **Files**: `internal/sessionpod/sessionpod.go`, `internal/sessionpod/sessionpod_test.go`,
  `k8s/cmd/crswd/main.go` (subcommand dispatch beside `session-pod`/`pane-loop`).
- **Interface**: `func CodexConversation(ctx context.Context, tmux tmuxctl.Controller, name, procRoot, codexHome string) (string, error)`:
  `PanePID` → `session.DiscoverCodexConversation(procRoot, pid, filepath.Join(codexHome, "sessions"))`.
  `codexHome == ""` → `"", nil`. The subcommand computes `codexHome` as
  `session.CodexHome(os.Environ())` (T007: `$CODEX_HOME` when absolute, else `$HOME/.codex`, else
  `""`), never by joining a possibly-empty `CODEX_HOME` itself.
  Subcommand `crswd codex-conversation <session-name>` prints the id or nothing, exit 0; error → exit 1
  with the error on stderr.
- **Tests**: fake Controller + fake proc root, as in T011; `TestCodexConversationHomeResolution`
  (explicit `CODEX_HOME`; unset with `HOME=/h` → searches `/h/.codex/sessions`; neither → `""`, no
  read attempted).
- **Acceptance**: `go test ./internal/sessionpod/...` and `go -C k8s test ./...` pass.
- **Depends**: T041.

### T043 — podctl uses the in-pod lookup

- **Files**: the file in `k8s/` that constructs the session `Manager` for kubernetes mode (grep
  `httpapi.NewWith` in `k8s/`), plus its test; `internal/session/manager.go` gains
  `func (m *Manager) SetCodexConversationFinder(f func(ctx context.Context, s Session) (string, error))`.
- **Interface**: in kubernetes mode, the finder execs `crswd codex-conversation <tmux name>` in the
  session pod through podctl's `Executor` and returns trimmed stdout.
- **Tests**: recording-fake Executor asserts the argv and that the id flows into the record.
- **Acceptance**: `go -C k8s test ./...` passes.
- **Depends**: T042.

### T044 — Docs and gate

- **Files**: `docs/k8s-mode.md` (Codex section: what works, what waits for Phase 4b).
- **Acceptance**: full gate, plus `go -C k8s vet ./... && go -C k8s test ./...`.
- **Depends**: T041–T043.

---

## Operator-run measurements (not loop tasks)

These need a cluster, an image registry and a real Codex credential, which an unattended iteration
does not have. Craig (or a supervised session) runs them; the loop never picks them. Phase 4b is
planned only after O-1's rows exist. Phase 4a (T041–T044) does not wait for them.

### O-1 — Codex credentials and image in a pod

- **Records**: rows `M20`–`M23` in `specs/019-codex-runtime/research.md`, values only: booleans,
  versions, HTTP status codes, file mtimes. Never a token, an account id or file contents.
- **Prerequisites** (each checked, stop if any fails): `kubectl config current-context` names the lab
  cluster spec 017's research D8b names; `docker` can pull the session image; `codex login status`
  on the host exits 0.
- **Procedure**:
  1. `NS=crswd-codex-probe-$(date +%s)`; `kubectl create namespace "$NS"`. Every object below lives in
     `$NS` and nowhere else.
  2. Copy the credential, never move or edit it: `TMPH=$(mktemp -d)`; `install -m 0600 ~/.codex/auth.json "$TMPH/auth.json"`.
     The source file `~/.codex/auth.json` is read once here and never written by any step.
  3. `kubectl -n "$NS" create secret generic codex-auth --from-file=auth.json="$TMPH/auth.json"`;
     then `shred -u "$TMPH/auth.json"; rmdir "$TMPH"`.
  4. M20: `docker run --rm --entrypoint sh <session image, pinned by the digest spec 017 S7 records> -c 'command -v codex; codex --version'`.
  5. M21: a pod in `$NS` with `CODEX_HOME=/codex` on an emptyDir, an init step copying the Secret's
     `auth.json` into it, running `codex -c check_for_update_on_startup=false exec --skip-git-repo-check 'say hi' </dev/null`
     with a 120 s `activeDeadlineSeconds`. Record exit code only.
  6. M22: in the same pod, record `stat -c %Y /codex/auth.json` before and after a second turn run
     after the access token's expiry (read `exp` from the JWT with `jq`, compare to `date +%s`; if not
     expired, record "not measured: token not expired" instead of forcing a refresh).
  7. M23: a second pod mounting the Secret read-only at `/codex/auth.json` (no copy), same command.
     Record exit code only.
  8. Teardown, always, even after a failure: `kubectl delete namespace "$NS" --wait=true`, then
     `kubectl get namespace "$NS"` must report NotFound.
- **Decision rule** (record which applies): M23 yes → Phase 4b mounts the Secret directly; M23 no and
  M21 yes → Phase 4b copies into an emptyDir at start and needs a keeper like spec 017 FR-015's;
  M21 no → Phase 4b uses an API key Secret and the spec records the billing change for Craig.
- **Guardrails**: output pasted into research.md is redacted to the values above; never copy
  `auth.json` into the repo, a log, a PR or a terminal capture.

---

## Parallel lanes

Lanes are disjoint in files and have no dependency edge between them. The Ralph loop runs one
notebook serially, so within a notebook lanes are ordering guidance only.

| Lane | Tasks | Files |
|---|---|---|
| 1a harness | T001 → T004a | `internal/harness/*`, `internal/config/codexflags*.go`, `internal/config/config.go` |
| 1b tmuxctl | T002 → T003 | `internal/tmuxctl/*` |
| 1c session pure | T007, T010a, T011 | `internal/session/trust_codex*.go`, `internal/session/conversation_codex*.go`, `internal/session/discover*.go`, `internal/session/trust.go` |
| 1d session wiring | T004 → T005 → T006 → T008 → T009 → T010b → T012 → T013 | `internal/session/manager.go`, `supervisor.go`, `conversation.go`, `dialog*.go`, `journal.go` |
| 1e web | T014 → T014a → T015 → T016 | `internal/httpapi/*`, `web/*`, `docs/components.md` |
| 1f acceptance | T017 → T018 | `cmd/crswd/*`, docs |
| 2 | T020 → T021 → T022a → T022b → T022c → T023 → T024 → T025 → T026 | after Phase 1 |
| 3 | T034 (any time) ; T030 → T033 | after Phase 2 (shared `header.html`, `crswd.js`) |
| 4a | T041 → T042 → T043 → T044 | after spec 017 S7; O-1 is not a prerequisite |
| 4b | unplanned | after O-1 |

1a, 1b and 1c can run concurrently (T010a needs only T001); 1d needs 1a and 1b; 1e needs T004 and
T013; 1f needs all of Phase 1.
Phase 1 tasks T013 and T016 also touch `internal/httpapi`, so 1e starts after T013 lands.
