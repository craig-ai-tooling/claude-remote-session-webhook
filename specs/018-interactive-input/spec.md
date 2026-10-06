# Feature Specification: Interactive Input on the Session Page

**Feature Branch**: `plan/interactive-input`
**Created**: 2026-10-06
**Status**: Draft
**Input**: Operator (Craig), 2026-10-05: "enabling input when we click on a
specific session … regardless of whether it's Claude or Codex … being able to go
back in the history is probably something that would be kind of important."
Option chosen by the operator on 2026-10-06: **a prompt box, a key bar and a
scrollback viewer**. No terminal emulator, no WebSocket, no new dependency.

This is the surface spec 003 deferred by name (`specs/003-dashboard-actions/spec.md:223`:
"a general 'type into the session' control is a larger surface with its own
questions and is not part of it"). Those questions are answered below.

## Open for the operator

**NEEDS CLARIFICATION: none blocking.** Every decision is made in `research.md`
with the rejected alternative beside it. Two questions can only be answered on a
phone, and ship UNANSWERED with fallbacks named in advance (FR-021):

- **Q4**: does the key bar reach every key in portrait, without horizontal scroll?
- **Q5**: with the soft keyboard up, are the textarea and Send still on screen?

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Type a message into a session from the browser (Priority: P1)

The operator opens a session and types a reply, a multi-line prompt or a slash
command into a text box, and sends it. This works the same for Claude Code and
Codex, because the daemon delivers bytes and knows nothing about either program.

**Independent Test**: Open a live session's page, type `hello` and press Send.
Within 2s the pane shows the session working on `hello`.

**Acceptance Scenarios**:

1. **Given** a live session the operator owns, **When** they type text and press
   **Send**, **Then** the text reaches the pane by bracketed paste followed by one
   Enter. The request answers `204`, and the textarea is cleared.
2. **Given** a textarea holding two lines, **When** the operator presses "Send",
   **Then** the program receives one two-line message. It does not submit after
   line one. (This was measured; see research.md R1.)
3. **Given** text in the textarea, **When** the operator presses "Type only",
   **Then** the text reaches the program's input box and no Enter is pressed.
4. **Given** the textarea has focus on a desktop, **When** the operator presses
   Ctrl+Enter or Cmd+Enter, **Then** this does what Send does. A plain Enter inserts
   a newline.
5. **Given** empty text, or text holding a control character, or text over 16384
   bytes, **When** it is sent, **Then** nothing reaches the pane. The toast says
   which rule refused it.
6. **Given** a session that is gone or is not the caller's, **When** text is sent,
   **Then** the answer is the uniform not-found every action gives.

### User Story 2 - Press the keys a phone keyboard does not have (Priority: P1)

The operator answers a menu, interrupts a turn, or cycles Claude Code's mode
(Shift+Tab) from a phone, where the soft keyboard has no Esc, Ctrl, Tab or arrow
keys.

**Independent Test**: Open a session sitting on a numbered menu. Tap "↓" then
**Enter** and see the second option chosen.

**Acceptance Scenarios**:

1. **Given** a live session, **When** the operator taps a key-bar button, **Then**
   exactly that key reaches the pane once, and the request answers `204`.
2. **Given** a POST naming a key outside the allowlist, **When** it arrives,
   **Then** nothing reaches the pane, and the outcome is `key-unknown`.
3. **Given** a phone in portrait at 390px wide, **When** the session page renders,
   **Then** every key-bar button is at least `--tap` (44px) tall, and the bar
   wraps instead of scrolling sideways.

### User Story 3 - Read what scrolled off the screen (Priority: P2)

The operator opens "Scrollback" under the pane and reads the session's history,
which tmux keeps but the live pane never showed.

**Independent Test**: In a session that has printed 3000 lines, open Scrollback.
Line 1 of the 3000 is readable.

**Acceptance Scenarios**:

1. **Given** a session page, **When** it renders, **Then** a closed
   `<details>` labelled "Scrollback" sits under the input panel, and no history
   is fetched.
2. **Given** the disclosure is opened, **When** it opens, **Then** the page fetches
   `GET /sessions/{id}/history` and shows up to 5000 lines of history as text, with
   ANSI stripped. The history does not include the visible screen, which the pane
   above already shows.
3. **Given** the disclosure is closed and opened again, **When** it opens, **Then**
   the history is fetched again.
4. **Given** a session with no history yet, **When** the disclosure opens, **Then**
   it reads "No scrollback yet." and is not an empty box.
5. **Given** a session created before this feature, **When** its history is read,
   **Then** it has tmux's old 2000-line limit. Only new sessions get 5000.

### Edge Cases

- **A session parked on a dialog** (trust prompt, `/rc` menu, Codex update
  prompt): input is not gated. The key bar exists to answer exactly these. The
  dialog heuristic (`session.DetectDialog`) keeps reporting and never refuses.
  One such dialog is dangerous to answer blind: Codex's "Update available" menu,
  where Enter on the default starts a global `npm install` (research.md R8).
  Spec 019 prevents it at launch with `-c check_for_update_on_startup=false`;
  this spec does not gate input on it.
- **A program in the alternate screen** (a pager, `vim`, Codex without
  `--no-alt-screen`): tmux keeps no history for it. The scrollback reads "No
  scrollback yet." and says nothing false. Spec 019 starts Codex with
  `--no-alt-screen`, measured `alternate_on=0`.
- **A text payload containing `ESC [ 2 0 1 ~`** (the end-of-paste marker) would end
  the bracketed paste early and turn the rest of the payload into keystrokes.
  FR-004 refuses every C0 control except LF and TAB, so this payload cannot arrive.
- **A browser sends CRLF from a textarea**: the handler normalises `\r\n` to `\n`
  before validation (FR-004). A bare `\r` left after that is a control character
  and is refused.
- **Two tabs typing into one session**: both deliveries land, in arrival order.
  Nothing serialises them, and nothing needs to. They are two people at one
  keyboard.
- **A burst of key presses** past the budget: refused with `input-limited` and
  nothing delivered. The budget refills at 4 a second.
- **History larger than the bound** (impossible at `history-limit 5000` with
  `-S -5000 -E -1`, but a tmux that answers differently): `CaptureHistory`
  refuses with `ErrHistoryTooLarge` and the route answers `500` with no body.
  Truncation is never used.
- **No script running**: the textarea and key bar are ordinary forms. A `204`
  leaves the browser on the page. A refusal lands on the fleet page with the
  outcome banner, like every other action. Scrollback needs script, and without
  script its body says "Scrollback needs JavaScript." (FR-014).

## Requirements *(mandatory)*

### The rule this feature amends

- **FR-001**: The rule "it never types into a working session" (`AGENTS.md:10`)
  MUST be amended to read exactly: *"The daemon never types into a working session
  on its own initiative. The one exception is the session page's input panel
  (spec 018): text or a key the operator sent, delivered once, when they pressed
  it."* `docs/security.md` §2 MUST carry the same rule. The daemon MUST still
  never type into a session from a timer, a supervisor, a dialog detector or any
  path without an operator request behind it.

### Delivery

- **FR-002**: `POST /dashboard/sessions/{id}/type` MUST read form fields `text`
  and `enter`. It MUST deliver `text` with `tmux load-buffer` (payload on stdin)
  then `tmux paste-buffer -p -d` (bracketed). If `enter` is exactly `yes`, it MUST
  then send the `Enter` key. Any other value of `enter` means no Enter.
- **FR-003**: `POST /dashboard/sessions/{id}/key` MUST read form field `key`, map
  it through the daemon's allowlist (FR-005) to a tmux key name, and send that
  constant with `tmux send-keys`. Caller bytes MUST never reach a `send-keys`
  argv.
- **FR-004**: The handler MUST replace every `\r\n` in `text` with `\n`. It MUST
  then refuse text that is empty, longer than 16384 bytes, not valid UTF-8, or
  holding any byte in `0x00–0x1F` other than `0x09` and `0x0A`, or `0x7F`. The
  manager MUST make the same checks again.
- **FR-005**: The key allowlist is exactly these twelve symbolic names, mapped to
  these tmux names: `enter→Enter`, `escape→Escape`, `interrupt→C-c`, `tab→Tab`,
  `backtab→BTab`, `up→Up`, `down→Down`, `left→Left`, `right→Right`,
  `pageup→PageUp`, `pagedown→PageDown`, `backspace→BSpace`. Measured on tmux 3.4,
  each produces the expected byte sequence (research.md R3).
- **FR-006**: The API door's `POST /sessions/{id}/prompt` MUST be unchanged:
  plain paste, then Enter. Bracketed paste is for the interactive route only.

### Gates

- **FR-007**: Both POST routes MUST be registered with `handleAction`. That means
  layer 1, then `Sec-Fetch-Site` same-origin with an absent header refused, then
  the page token, then `MaxBytesReader`, all before the handler runs.
- **FR-008**: Both POST routes MUST resolve the session with `Manager.View(id,
  operator.Owner)` and answer every lookup failure with `notFoundAction`, the
  uniform not-found.
- **FR-009**: Both POST routes MUST spend one token from one shared per-operator
  bucket (`inputs`, keyed by `auth.CallerID`, 240 per minute, burst 120) before
  the lookup. A refused spend MUST answer outcome `input-limited` and deliver
  nothing.
- **FR-010**: A successful type or key MUST answer `204 No Content` with no body.
  Every refusal after the gate MUST answer `303` to the fleet page with the
  route's outcome code, as every other action does.
- **FR-011**: Type and key MUST move the session's last-driven clock with
  `store.Touch` **before** delivery, as `Manager.Compact` does
  (`internal/session/manager.go:1114-1119`). History MUST NOT move it.

### Audit

- **FR-012**: Each request MUST produce exactly one audit record, under the new
  actions `dashboard.type`, `dashboard.key` and `dashboard.history`. A record MUST
  carry no typed text, no key name and no history. The record shape is frozen
  (FR-016 of spec 003), so there is no field to put a key name in, and the leak
  suite MUST prove the text never appears.

### Scrollback

- **FR-013**: Every tmux session this daemon creates MUST be created with
  `history-limit 5000`. That means one tmux invocation,
  `tmux set-option -g history-limit 5000 ; new-session -d -s <name> -c <dir>`, with
  `;` as its own argv element. A per-session `set-option` after `new-session` does
  not change an existing pane's limit (measured, research.md R2).
- **FR-014**: `GET /sessions/{id}/history` MUST be registered with
  `handleBrowser`, refuse a cross-site request as the stream does
  (`crossSite`, `internal/httpapi/stream.go:464`), resolve with `View`, and answer
  `200 text/plain; charset=utf-8` with `Cache-Control: no-store`. The body is the
  output of `tmux capture-pane -p -S -5000 -E -1`, passed through `tmuxctl.Strip`.
- **FR-015**: `CaptureHistory` MUST refuse, never truncate, output over 5000 lines
  or over 4 MiB, with `ErrHistoryTooLarge`. The route MUST answer that with `500`
  and no body.
- **FR-016**: The page MUST fetch history only when the Scrollback disclosure
  opens. It MUST fetch again on every open, and MUST put the answer into a
  `<pre class="pane">` with `textContent`. An empty answer MUST render "No
  scrollback yet."

### Interface

- **FR-017**: The input panel, the key bar and the Scrollback disclosure MUST be
  rendered inside `partials/pane.html`, after the pane's notes, only when the
  pane has a page token (`{{ with .PageToken }}`). They MUST use only `.button`,
  `.field-label`, `.field-input` and the new classes `.input-panel`, `.type-form`,
  `.key-bar`, `.scrollback`, `.scrollback-summary`.
- **FR-018**: On a coarse pointer, every key-bar button MUST be at least
  `var(--tap)` in both dimensions, and the textarea font MUST be `var(--fs-input)`.
  The key bar MUST be `flex-wrap: wrap`. No new width breakpoint is allowed
  (`TestTheDashboardHasExactlyOneBreakpoint`).
- **FR-019**: The textarea MUST carry `autocapitalize="off" autocorrect="off"
  spellcheck="false"` and a `<label>`. The `.input-panel` MUST be
  `position: sticky; inset-block-end: 0`, so it stays on screen while the pane
  scrolls.
- **FR-020**: The pane note MUST read exactly: "This is the live screen, not
  scrollback. Open Scrollback below to read what came before it." The phrase "not
  scrollback" is pinned by two existing tests.
- **FR-021**: `docs/mobile-open-questions.md` MUST gain Q4 and Q5, both
  **UNANSWERED**, each with its fallback named. Q4 fallback: the key bar becomes a
  second `<details>` ("More keys") holding the arrows, PageUp/PageDown and
  Backspace, with Enter, Esc, Ctrl-C, Tab and Shift-Tab left visible. Q5 fallback:
  move the input panel above the pane.

### Not in this feature

- **FR-022**: No WebSocket, terminal emulator, vendored JavaScript, CSP change,
  API-door input route or change to the stream's 1s cadence. Each one is a
  separate decision with its own review. research.md R6 records why.

**Key Entities**: **Key**, a symbolic name from FR-005's closed set. **Typed
text**, operator bytes that are secret under `docs/security.md` §3, exactly like
prompt text. **History**, the stripped text of tmux's scrollback for one pane,
excluding the visible screen.

## Success Criteria *(mandatory)*

- **SC-001**: `go test ./internal/tmuxctl -tags tmux -run 'Bracketed|History'`
  proves on a real tmux that a two-line bracketed paste arrives wrapped in
  `ESC[200~ … ESC[201~`, and that a 6000-line job yields at most 5000 lines of
  history from a session created by `Exec.New`.
- **SC-002**: The leak suite (`go test ./internal/audit -run Leak`) drives
  `dashboard.type` with a canary string, and the canary appears in no audit
  record and no log line.
- **SC-003**: Every refusal shape that `TestRefusalIsNotARedirect` applies to an
  action is applied to both input routes, and each answers its uniform status.
- **SC-004**: Disabling either route's registration fails a handler test.
- **SC-005**: Q4 and Q5 exist in `docs/mobile-open-questions.md` marked
  UNANSWERED, and nothing in this plan marks them answered.

**Assumptions**: the operator is the only person typing, so arrival order is
good enough and no lock is needed. tmux 3.4 key names are stable across 3.x.
Claude Code 2.1.290 and Codex 0.153.4 both enable bracketed paste. That was
measured, and a later release that stops doing so degrades to "line one
submits", which an operator sees immediately.
