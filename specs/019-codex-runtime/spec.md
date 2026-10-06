# Feature Specification: Codex as a First-Class Runtime

**Feature Branch**: `plan/codex-core` (Phase 1), `plan/codex-auth`, `plan/codex-quota`, `plan/codex-k8s`
**Created**: 2026-10-06
**Status**: Draft (decisions recorded in [research.md](research.md) §D, measurements in §M)
**Depends on**: spec 018 (interactive input) merged to `main`. Phase 4 also depends on spec 017 (S7) merged.
**Input**: Craig, 10/5/26: "I want Codex to be a first-class citizen in this tool, where we can
start a new session and it can use either Codex or Claude. I need it to have all the same
capabilities, functions, features, and stuff. … I don't think Codex has a remote control-capable
thing like Claude … remote control matters a little bit less." Scope chosen 10/5/26: core session
parity, Codex sign-in relay, Codex usage/quota, Kubernetes mode for Codex.

## NEEDS CLARIFICATION (Craig's calls only)

- **NC-1 (Phase 3).** Craig's Codex account is Business with unlimited credits, so neither `/status`
  nor quota-axi has a usage window for it today (research M18). As specified, the Codex meter will
  read `codex quota: unknown` on this host until a window exists. Keep the meter (current plan), or
  hide it while the provider has no window? The tasks implement "keep"; answering "hide" changes
  T032 only.

## Non-goals

- **Remote control for Codex.** `codex remote-control` pairs the ChatGPT mobile app with a Codex
  app-server daemon, not with a TUI running in tmux (research D4). Spec 018's typed input is the
  remote path for Codex sessions.
- **Lawnmower integration.** Nothing in ai-lawnmower changes.
- **Harnesses other than Claude Code and Codex.** An unknown binary keeps today's behaviour (it runs,
  is supervised, is never revived by id). The general picker in docs/harnesses.md stays future work.
- **Updating Codex.** crswd never answers or triggers Codex's update prompt (research M12).
- **Switching a running session between runtimes.**

## User Scenarios & Testing *(mandatory)*

### Phase 1 — Core parity

#### User Story 1 - Start a Codex session from the dashboard (Priority: P1)

Craig opens the create dialog, picks Codex, names the session, picks a directory, and gets a
Codex session that starts without stopping at a trust prompt, a hooks prompt or an update menu.

**Independent Test**: With `start_commands` holding a `codex=` entry, create a session with
harness Codex; the pane reaches Codex's composer (`› Ask Codex to do anything`) and the card says
`Codex`.

**Acceptance Scenarios**:
1. **Given** a `codex` start command is configured, **When** the create dialog opens, **Then** it
   offers Claude Code and Codex, and the preview shows the exact line Codex would run.
2. **Given** no `codex` entry, **When** the dialog opens, **Then** it shows no harness control and
   behaves exactly as today.
3. **Given** a Codex create for a directory Codex never trusted, **When** the session starts,
   **Then** `$CODEX_HOME/config.toml` holds `[projects."<dir>"]` with `trust_level = "trusted"`
   before the command is typed.
4. **Given** a Codex create, **When** the line is typed, **Then** it carries `--no-alt-screen` and
   `-c check_for_update_on_startup=false` whether or not the operator configured them.
5. **Given** `remote_control=on` and harness Codex, **When** submitted, **Then** the create is
   refused and nothing starts.

#### User Story 2 - A Codex session is supervised and revived like a Claude one (Priority: P1)

**Independent Test**: Start a Codex session, send one prompt, kill the Codex process; the
supervisor revives it with `codex resume <id>` and the conversation is the same.

**Acceptance Scenarios**:
1. **Given** a running Codex session, **When** the supervisor sweeps, **Then** it reads as running
   although tmux reports `node`.
2. **Given** a Codex session that has had one prompt, **When** the supervisor sweeps, **Then** the
   record, `@crswd-conversation` and the journal carry the conversation id read from `/proc`.
3. **Given** Codex has exited to the shell and an id is known, **When** the supervisor revives,
   **Then** it types `codex resume <id> …` with the required flags.
4. **Given** no prompt has been sent yet, **When** Codex exits, **Then** the session is marked
   failed with `reasonNoConversation`, as a pre-012 Claude session is.

#### User Story 3 - Prompt, compact and continue work on Codex (Priority: P1)

**Acceptance Scenarios**:
1. **Given** a Codex session, **When** a prompt is delivered (API or spec 018's input),
   **Then** it is pasted bracketed and submitted by one Enter.
2. **Given** a Codex session, **When** Compact is pressed, **Then** `/compact` is pasted bracketed
   and submitted.
3. **Given** a Codex session in directory D, **When** the session page opens, **Then** Continue
   lists only Codex conversations whose `cwd` is D.
4. **Given** Continue is chosen while Codex is mid-turn, **When** the restart runs, **Then** crswd
   quits Codex by stepped Ctrl-C, confirms the shell is back, and only then types the resume line;
   if Codex is still running after 5 presses nothing is typed and the operator sees an error.

#### User Story 4 - The pill tells the truth about a Codex pane (Priority: P2)

**Acceptance Scenarios**:
1. **Given** a Codex pane on the trust, hooks, approval or update screen, **When** the session page
   renders, **Then** the pill is `blocked` with the dialog's name.
2. **Given** a Codex pane idle or working, **Then** the pill is `running`, never `unknown`.

### Phase 2 — Sign-in

#### User Story 5 - Sign Codex in from the header (Priority: P2)

**Acceptance Scenarios**:
1. **Given** a `codex` entry is configured, **When** any page renders, **Then** the header has a
   Codex auth pill beside the Claude one.
2. **Given** Codex is signed out, **When** Craig opens the Codex sign-in, **Then** the panel shows
   the device link and the one-time code and a waiting state, and no code form.
3. **Given** the device flow completes, **When** the pill next polls, **Then** it reads signed in.
4. **Given** Codex is signed out and Claude is signed in, **When** a Codex create is submitted,
   **Then** it is refused with a Codex sentence; a Claude create is not.

### Phase 3 — Usage

#### User Story 6 - See Codex usage beside Claude's (Priority: P3)

**Acceptance Scenarios**:
1. **Given** a `codex` entry and a cache holding a `codex` provider `weekly` window, **Then** a
   second meter reads `Codex weekly N% used`.
2. **Given** the cache has no `codex` provider, **Then** the second meter reads
   `codex quota: unknown` (NC-1).

### Phase 4 — Kubernetes mode

#### User Story 7 - A Codex session runs as a pod (Priority: P3)

**Acceptance Scenarios** (Phase 4a, this spec):
1. **Given** kubernetes mode, **When** a pod session's start command is Codex, **Then**
   `CODEX_HOME` passes through to the pod's tmux.
2. **Given** a running Codex pod, **When** the supervisor sweeps, **Then** the conversation id is
   read inside the pod and returned to the daemon.
Phase 4b (credentials) is specified after operator-run measurement O-1 (tasks.md) records its rows.

### Edge Cases

- `start_commands` names `codex` but its binary is something else: not offered as Codex (D2).
- Two Codex sessions in one directory: each id comes from its own process's open file (D7).
- Codex's rollout is not open yet (before the first prompt): no id, no error, swept again next time.
- More than one distinct rollout open under one pane: `ErrAmbiguousConversation`, nothing recorded,
  reported once per sweep to the supervisor's report.
- `config.toml` mentions the directory in a shape other than `[projects."<dir>"]` (inline table,
  dotted key, literal string): trust seeding refuses with `ErrCodexConfigShape`; the create fails.
- A directory path containing `"`, `\`, a newline or a control character: refused with
  `ErrUntrustablePath` before any edit.
- `$CODEX_HOME` does not exist: trust seeding does nothing; Codex will show sign-in first.
- The operator's command already has `--no-alt-screen`: not inserted twice.
- The operator wrote `-c check_for_update_on_startup=true`: crswd inserts its own `false` right
  after the binary and Codex reads both; the later (operator's) value wins. Documented, not guarded.
- `codex resume <id>` for an id whose rollout was deleted: Codex prints an error and exits (M11);
  the supervisor's existing transcript gate stops this earlier.
- A Codex session whose command name is no longer configured: same as Claude today, refused at revival.

## Requirements *(mandatory)*

### Phase 1

- **FR-001**: A new package `internal/harness` MUST define `Name` (`claude`, `codex`, `other`),
  `Of(command string) Name`, and `For(Name) Spec`, with `Spec` carrying the per-harness facts in
  research D5–D9. Nothing outside `internal/harness` may compare a binary name to `"claude"` or
  `"codex"`.
- **FR-002**: The harness of a session MUST be derived from its configured start command every time
  it is needed and MUST NOT be stored (D1).
- **FR-003**: `@crswd-binary` MUST accept names joined by `|`, each `[A-Za-z0-9._-]`, and the list
  format MUST compare with `#{m/r:^(…)$,…}` (D6). A Claude session's written value MUST be unchanged.
- **FR-004**: For Codex, every line crswd types MUST carry `--no-alt-screen` and
  `-c check_for_update_on_startup=false` inserted after the binary unless the exact token sequence is
  already present (D5). For Claude the typed line MUST be byte-identical to today's.
- **FR-005**: A fresh Codex start MUST carry no conversation id; a Codex resume MUST be
  `<binary> resume <id> <required flags> <rest of template>` (M10).
- **FR-006**: Prompt and Compact on a Codex session MUST use bracketed paste then one Enter (D9).
- **FR-007**: Restarting a Codex session (Continue, `restartInto`) MUST
  use the stepped quit in D8 and MUST NOT type a command line unless the pane is confirmed off Codex.
- **FR-008**: Before typing a Codex start or resume line, crswd MUST seed Codex trust for the
  session's resolved working directory per D10, and a seeding error MUST fail the create or revival.
- **FR-009**: The supervisor MUST discover a running Codex session's conversation id from `/proc` per
  D7, record it on the session, `@crswd-conversation` and the journal (event `discovered`), and
  never record an id it did not read from an open rollout.
- **FR-010**: Codex conversation listing and transcript checks MUST follow D11's bounds and MUST
  return only rollouts whose `session_meta.payload.cwd` equals the session's resolved directory.
- **FR-011**: Dialog detection MUST select the registry by harness; Codex's registry MUST name the
  trust, hooks-review, approval and update screens from captured text (research §F) and MUST NOT
  match the idle or working screen.
- **FR-012**: The browser create form MUST offer Codex only per D2, MUST send a `harness` field whose
  only accepted values are `claude` and `codex`, and MUST refuse `harness=codex` with
  `remote_control=on`.
- **FR-012a**: `Manager.SetMode` on a session whose harness is not Claude MUST return
  `ErrModeUnavailable` before touching the pane; otherwise a Codex session switched to remote would
  be restarted into the Claude `rc` command.
- **FR-013**: Session cards, the session page and `GET /sessions` entries MUST show the harness label;
  the API entry gains `"harness"` (omitted for `other`).
- **FR-014**: Prompt text MUST NOT appear in any audit record, error or log (FR-042 of spec 001
  holds unchanged).

### Phase 2

- **FR-015**: A Codex sign-in relay MUST run `<codex binary> login --device-auth` in its own tmux
  window `crswd-login-codex`, MUST show the link and code, and MUST NOT accept a code from the
  browser (D12).
- **FR-016**: Codex signed-in state MUST come from `codex login status`'s exit code per D12.
- **FR-017**: The relay, the auth cache, the header pill and the create gate MUST be per harness
  (D13); `GET /dashboard/auth` without `harness` MUST answer exactly as today.
- **FR-018**: A Codex pane on the sign-in screen (F4) or a device-code screen (F2, F3) MUST render
  `needs-auth`.
- **FR-019**: The one-time code the sign-in relay reads from its own window MUST NOT appear in
  the audit trail, logs, query strings or JSON responses other than the sign-in view fragment; it
  follows docs/auth-and-sessions.md's rules for the Claude sign-in link. A session's own pane is
  out of scope: the pane viewer shows whatever a session drew, exactly as it does for a Claude
  session on its sign-in screen today (research, critic finding R-3).
- **FR-019a**: A Codex start command that assigns `check_for_update_on_startup` any value other than
  `false` before `--` MUST be refused at configuration load (T004a).

### Phase 3

- **FR-020**: `internal/quota` MUST read any provider/window pair; Claude's reading MUST be unchanged.
- **FR-021**: `GET /dashboard/quota?harness=codex` MUST read provider `codex` window `weekly`; no
  `harness` MUST answer exactly as today.
- **FR-022**: The header MUST show a Codex meter only when a `codex` entry is configured.

### Phase 4a

- **FR-023**: `internal/sessionpod`'s pass-through MUST include `CODEX_HOME`.
- **FR-024**: A `crswd session-pod`-side helper MUST answer the D7 lookup inside the pod, so the
  daemon never reads a pod's `/proc` directly.
- **FR-025**: Phase 4b (Codex credentials in pods) MUST NOT be built until O-1 records a
  measurement and a follow-up plan names the design.

## Success Criteria *(mandatory)*

- **SC-001**: `go test ./internal/harness/...` proves `Of` for `claude`, `/abs/path/claude`, `codex`,
  `/home/x/sf-cli/bin/codex`, `codex.js`, `bash`, `""`.
- **SC-002**: Every existing test under `internal/session` and `internal/httpapi` passes with no
  edit to an expected Claude command line.
- **SC-003**: A `-tags tmux` test proves a pane running `sleep` reads `running` with
  `@crswd-binary` = `codex|sleep` and `stopped` with `codex|node`.
- **SC-004**: A test proves a Codex create types a line containing both required flag groups, once.
- **SC-005**: A test proves `restartInto` on a Codex session types no command line when the fake
  pane never leaves `node`.
- **SC-006**: A test proves the discovery walk returns the id from a fake `/proc` tree and returns
  `ErrAmbiguousConversation` for two distinct open rollouts.
- **SC-007**: Fixture tests prove each Codex dialog in research §F is named and F8/F9 are not.
- **SC-008**: A quickstart acceptance case creates a Codex session through the signed API against a
  `codex` shim and asserts `@crswd-binary` is `codex|node` and the typed line carries both flags.
- **SC-009**: Each Phase 2 and 3 route test fails with the route's harness parameter ignored.

## Assumptions

- Codex 0.153.4 behaviour (research §M) holds for the version Craig runs. The fixtures are pinned to
  0.153.4 and a version change is a re-capture, not a code change, per dialog.go's rule.
- The host is Linux (`/proc`); release builds are `GOOS=linux` only (`.github/workflows/release.yml:84`).
