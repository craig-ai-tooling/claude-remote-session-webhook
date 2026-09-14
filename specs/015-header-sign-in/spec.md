# Feature Specification: Auth in the Header, Not in Settings

**Feature Branch**: `feat/header-auth-indicator`

**Created**: 2026-09-13

**Status**: Draft

**Input**: Operator: "Can we not put auth in settings? just in the header have a
auth: ok/bad indicator and if it is bad, maybe pop up a modal to allow me to auth?
How often should the UI check?"

The cadence and every other open question below were answered by the operator's
chief of staff before this spec was written (D1–D9). Nothing here is
`NEEDS CLARIFICATION`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - The header always says whether this host can sign in (Priority: P1)

An operator glances at any page of the dashboard — not only Settings — and sees
`auth: ok`, `auth: bad`, `auth: unknown` or `auth: checking` in the header, next
to their own identity.

**Why this priority**: this is the whole of what was asked for. The fact used
to exist only on one section of one page, found by nobody unless a session's own
card was already stuck on `needs-auth`.

**Independent Test**: Load any page with a signed-in relay reporting a real
answer; confirm the header pill reads it within 60 seconds of load.

**Acceptance Scenarios**:

1. **Given** any page this daemon serves, **When** it renders, **Then** the
   header carries a control reading one of `auth: ok`, `auth: bad`,
   `auth: unknown`, `auth: checking`.
2. **Given** a daemon whose relay reports signed in, **When** the pill polls,
   **Then** it reads `auth: ok`.
3. **Given** a daemon whose relay reports signed out, **When** the pill polls,
   **Then** it reads `auth: bad`.
4. **Given** a daemon with no relay at all, or one whose exec failed, **When**
   the pill polls, **Then** it reads `auth: unknown` — the same word for both,
   because neither is a fact "sign in" fixes.
5. **Given** a page that has just loaded and asked nothing yet, **When** it
   renders, **Then** the pill reads `auth: checking`, server-side, before any
   script runs.

---

### User Story 2 - Pressing the indicator opens a way to fix it (Priority: P1)

An operator presses the `auth:` control and a dialog opens holding the sign-in
flow that used to live on the settings page: a button that starts a sign-in, the
link to open, a box for the code, and a way to cancel.

**Why this priority**: an indicator with no next step is a status an operator
still has to go somewhere else to act on. This is the second half of the ask.

**Independent Test**: Press the control; confirm the dialog opens and, once a
script has loaded its content, offers the same controls the settings page's
Sign-in section used to.

**Acceptance Scenarios**:

1. **Given** any page, **When** the auth control is pressed, **Then** a dialog
   opens with no script required to open it (Invoker Commands).
2. **Given** the dialog has just opened, **When** a script is running, **Then**
   it fetches the sign-in panel and fills the dialog's body with it.
3. **Given** no script is running, **When** the dialog opens, **Then** its body
   states plainly that this one control needs a script, rather than showing a
   form that cannot be submitted.
4. **Given** a sign-in is started, delivered a code, or cancelled from inside
   the dialog, **When** the daemon answers, **Then** the outcome appears inside
   the dialog (scripted) or on the dashboard the operator lands on (scriptless).

---

### User Story 3 - A bad answer interrupts, once (Priority: P2)

The pill turns from anything else to `auth: bad`, and the dialog opens on its
own so the operator does not have to notice a colour change to be told the
fleet cannot make a request.

**Why this priority**: this is what makes the indicator more than decoration —
it is the mechanism that answers "how often should the UI check?" with "often
enough that going bad gets noticed without being asked."

**Independent Test**: Start signed in; flip the relay to signed out; confirm the
next poll opens the dialog once and does not reopen it on the poll after, even
though the state is still `bad`.

**Acceptance Scenarios**:

1. **Given** the first answer this page has seen is `bad`, **When** it arrives,
   **Then** the dialog opens.
2. **Given** the state was anything else and becomes `bad`, **When** that
   transition is observed, **Then** the dialog opens.
3. **Given** the dialog opened for a `bad` state and the operator closed it,
   **When** a later poll still reads `bad`, **Then** the dialog does not reopen
   on its own.
4. **Given** the state leaves `bad` and returns to it, or the page reloads,
   **When** that happens, **Then** the next `bad` answer opens the dialog again.
5. **Given** the state is `unknown`, **When** it is read, **Then** the dialog
   never opens on its own — `unknown` is not a fact "sign in" fixes, and
   auto-opening for it would send the operator to press a button that cannot
   help.

---

### Edge Cases

- **A daemon with no relay at all.** The pill reads `unknown`, not `bad` — a
  missing binary is not a credential problem, and telling the operator to sign
  in sends them to fix the wrong thing.
- **A relay whose exec fails.** Same word, same reason, and the existing report
  line still fires once per failed exec (unchanged from before this spec).
- **Two tabs open at once.** Each polls independently; nothing here coordinates
  them, and nothing needs to — the cache bounds the cost regardless of how many
  tabs are asking.
- **The dialog is open when the tab is backgrounded and the pill's own poll
  fires.** The poll still runs and may repaint the pill; it does not touch the
  dialog's own three-second refresh, which is independent.
- **A scriptless browser presses "Start a sign-in".** The three-oh-three lands
  back on the dashboard carrying the outcome and a marker; the outcome banner
  says what happened and the marker opens the dialog so the next step is where
  the operator would expect it.

## Requirements *(mandatory)*

### Functional Requirements

#### The header control

- **FR-001**: Every page MUST carry a `<button>` — never an `<a>` — in the
  header, between the operator's identity and the settings link, styled as a
  status pill.
- **FR-002**: The control MUST render `auth: checking` on the server before any
  script has run, using the existing `unknown` pill state.
- **FR-003**: The control MUST open `#signin-dialog` via `command="show-modal"`,
  requiring no script to open on a capable browser.
- **FR-004**: New pill states `ok` and `bad` MUST get their own tokens
  (`--state-ok`, `--state-bad`) and rules (`.pill-ok`, `.pill-bad`); the
  `checking` and could-not-ask cases MUST reuse the existing `unknown` state and
  differ only in their text.
- **FR-005**: State MUST never be conveyed by colour alone; every state carries
  a distinct text label.

#### The status endpoint

- **FR-006**: `GET /dashboard/auth` MUST answer JSON `{"state": "ok"|"bad"|"unknown"}`
  and nothing else — never a link, a window's running state, or any other
  fact.
- **FR-007**: The route MUST require the same identity every other dashboard
  read requires, and MUST answer `Cache-Control: no-store`.
- **FR-008**: No verb but GET MUST be registered on this path.
- **FR-009**: The answer MUST come from a cache with a 60-second TTL, one
  in-flight exec shared by concurrent callers, and an injectable clock.
- **FR-010**: A could-not-ask failure MUST be cached for the same TTL as a real
  answer — a fresh attempt every request would defeat the cache's purpose the
  moment the relay is unreachable.
- **FR-011**: Each of the three sign-in action routes (start, code, cancel)
  MUST invalidate the cache.

#### The sign-in dialog and its fragment

- **FR-012**: `#signin-dialog` MUST live in the header partial, so every page
  that carries the header carries the dialog.
- **FR-013**: The dialog's body MUST ship a short line stating that this
  control needs a script, until a script replaces it.
- **FR-014**: `GET /dashboard/signin/view` MUST answer the HTML fragment that
  used to be the settings page's Sign-in section — signed-in / signed-out /
  could-not-ask text, the start form, the link and code form while a sign-in is
  running, and the cancel form — behind the same identity check as any other
  read.
- **FR-015**: That route MUST perform a fresh ask of the relay — never read the
  `GET /dashboard/auth` cache — and MUST store its answer into that cache
  afterward.
- **FR-016**: The fragment's root element MUST carry `data-auth-state` and
  `data-signin-running`.
- **FR-017**: The sign-in URL MUST appear only as an `href` inside this
  fragment — never in JSON, a `data-` attribute, a query string, a log line, or
  an audit record. This is unchanged from before this spec and re-verified
  here.

#### Client behaviour

- **FR-018**: The client MUST fetch `/dashboard/auth` on load, then every 60
  seconds while `document.visibilityState === 'visible'`, with an immediate
  fetch and a reset countdown when the tab becomes visible again. A fetch
  failure MUST paint the pill `unknown`.
- **FR-019**: Opening the dialog — by click or by the auto-open rule — MUST
  fetch `/dashboard/signin/view` and replace the dialog body's content with it.
- **FR-020**: While the dialog is open and the fragment's `data-signin-running`
  is `true`, the client MUST refresh the fragment every 3 seconds; it MUST stop
  when the dialog is closed.
- **FR-021**: The client MUST auto-open the dialog when the state is `bad` on
  the first answer observed, or transitions into `bad` from any other state,
  and MUST NOT reopen for the same `bad` streak once closed — only a
  transition out of and back into `bad`, or a page reload, arms it again. It
  MUST NOT auto-open for `unknown`.
- **FR-022**: The client MUST open the dialog on load when the URL carries the
  `signin=open` marker.
- **FR-023**: All of the above MUST introduce no new CSS animation and MUST
  honour `prefers-reduced-motion` (there is none to honour beyond what already
  exists, and none is added).

#### The three action routes

- **FR-024**: The start, code, and cancel routes MUST keep their existing gate
  and validation unchanged.
- **FR-025**: Each MUST invalidate the auth cache (FR-011, restated here
  against the routes rather than the cache).
- **FR-026**: Each MUST redirect 303 to `/` carrying the existing outcome-code
  query parameter plus a `signin=open` marker, rather than to a settings
  section.

#### Settings

- **FR-027**: The Sign-in menu entry, section, and panel MUST be removed from
  `GET /settings` entirely.
- **FR-028**: `?section=Sign-in` MUST fall through to whatever an unrecognised
  section already falls through to — no special case is added for it.

### Key Entities

- **Auth state**: one of `ok`, `bad`, `unknown` — the daemon's own word for
  whether it can currently make a Claude request. Distinct from a single
  session's `needs-auth`, which stays a per-session fact.
- **Auth cache**: one value, one timestamp, one mutex held across the ask —
  shared by every poller and by the sign-in fragment's own fresh ask.
- **Sign-in panel**: the same `signInPanel` type and template content that used
  to render inside settings.html, now rendered as its own fragment.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Every page this daemon serves carries the auth control and the
  dialog; none composes its own copy of either.
- **SC-002**: A daemon under test load answers `GET /dashboard/auth` from cache
  for any two calls inside 60 seconds, confirmed by counting the relay's own
  `SignedIn` calls.
- **SC-003**: The settings page's rendered bytes for `?section=Sign-in` are
  byte-identical to its default section — pinned by a test.
- **SC-004**: No response this daemon renders — the auth JSON included —
  contains the sign-in URL outside `GET /dashboard/signin/view`'s own `href`.
- **SC-005**: The three sign-in action routes redirect to `/`, never to
  `/settings`.

## Assumptions

- **A single session's own `needs-auth` stays separate.** Folding it into this
  header pill is out of scope, named as a decision rather than an oversight
  (D9).
- **The cache lives on the server, not the client.** Multiple open tabs each
  poll independently; nothing here coordinates them, and the server-side cache
  is what keeps that cheap.
- **"Often enough to notice, rarely enough to be cheap" is 60 seconds for the
  pill and 3 seconds for an open dialog watching a running sign-in.** Neither
  number is derived from a formula; both are chosen the way every other
  interval in `docs/auth-and-sessions.md`'s Lifetimes table is.
