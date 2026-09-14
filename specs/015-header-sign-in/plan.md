# Implementation Plan: Auth in the Header, Not in Settings

**Branch**: `feat/header-auth-indicator` | **Date**: 2026-09-13 | **Spec**: [spec.md](spec.md)

## Summary

The sign-in relay's panel moves from one section of the settings page to a
dialog every page's header can open. A new cached route,
`GET /dashboard/auth`, gives the header pill a cheap yes/no/unknown to poll;
a new fragment route, `GET /dashboard/signin/view`, gives the dialog the panel
itself, asked fresh and folded back into that same cache. The three existing
action routes keep their gate and their validation and change only where they
redirect and that they invalidate the cache. Settings loses the section
outright — no route, no menu entry, no special-cased fallback.

## Technical Context

**Language**: Go 1.24, standard library, server-rendered templates, one
embedded vanilla-JS asset.
**Storage**: none new. The auth cache is in-process and lives exactly as long
as the daemon does, like the existing release-feed cache it is modelled on.
**Testing**: `go test ./...`; table-driven, `t.Parallel()`, fakes for the
relay so the suite never shells out to a real `claude` binary.
**Constraints**: the dialog is the one control in this tree that needs a
script to reach its real content — named as an exception rather than hidden,
because reaching it costs a fetch no declarative attribute can make. Every
other control here still needs none.

### Why there is no research.md or data-model.md

Nothing here asks a question the host has to answer. The cache shape already
exists (`releaseCache`) and is copied rather than invented; the panel's own
type and template content already exist and move rather than get rewritten;
the modal vocabulary already exists (`docs/components.md`, Modal) and is
reused rather than extended. The one new design decision — the transition-based
auto-open rule (D8) — is small enough to state in the spec and needs no
separate document.

## Constitution Check

| Principle | Assessment | Pass |
|---|---|---|
| **I — Security is a gate** | `GET /dashboard/auth` and `GET /dashboard/signin/view` are reads behind the same identity check as every other dashboard read; neither carries a page token because neither writes. The three action routes keep their existing gate untouched. The sign-in URL's binding rule (never outside one `href`) is re-verified by test rather than merely carried over by inspection. | ✅ |
| **II — Unknowns surfaced** | Every open question (cadence, auto-open rule, could-not-ask wording, cache shape) was answered by the operator's chief of staff before this plan was written; none is guessed at here. | ✅ |
| **III — Verifiable** | Every FR is a status code, a JSON shape, a cache call count, or a byte comparison a test can make. | ✅ |
| **IV — Smallest correct change** | The panel's own type, template content and validation are moved, not rewritten. The cache mirrors an existing one rather than inventing a second shape. | ✅ |
| **V — Standards enforced** | The stylesheet/markup parity sweep, the route-secret sweep, and the no-mutating-verb sweep all gain the two new routes rather than being relaxed around them. | ✅ |
| **VI — Blast radius** | Untouched. This feature reads and relays a credential an operator already controlled from the settings page; nothing here changes what a session may reach. | ✅ |
| **VII — Design system** | No new component. The control is the existing status pill wearing a `<button>`; the dialog is the existing Modal family's second call site; two new state tokens are added to the one table that already lists every state. | ✅ |

## Design

### The cache

```go
type authCache struct {
    mu      sync.Mutex
    state   authState
    fetched time.Time
    has     bool
}
```

Held across the ask, exactly as `releaseCache` is: a concurrent caller that
reaches the lock after the first has finished finds an answer already fresh
rather than paying for a second exec. `authState` is `"ok"`, `"bad"`, or
`"unknown"` — a nil relay and a relay whose exec failed both read `"unknown"`,
because neither is a fact "sign in" fixes.

`GET /dashboard/auth` reads through the cache. `GET /dashboard/signin/view`
bypasses it — a fresh ask every time, because the dialog is answering "is this
host signed in right now" and the pill is answering "was it, recently enough"
— and stores what it learns back in, so the poll a tab makes shortly after
opening the dialog does not pay for a second exec the fragment already made.

The three action routes invalidate unconditionally, at the top of each
handler, right after the request is known to have passed the gate. None of
the three can be trusted to have left the credential exactly as the cache last
recorded it, and invalidating conservatively costs at most one extra exec
inside the next 60 seconds.

### The dialog

`#signin-dialog` lives in `partials/header.html`, beside the pill that opens
it, on the Modal family's own terms (`docs/components.md`): `.modal`,
`.modal-head`, `.modal-title`, `.modal-close`, `.modal-outcome`,
`.modal-body`. Its body ships a short no-script line until `crswd.js` fetches
`GET /dashboard/signin/view` and swaps it in.

The existing generic submit interception (the toast module in `crswd.js`)
already finds `.modal-outcome` inside `form.closest('dialog')` for any form
under `/dashboard/`, so the sign-in forms need no dialog-specific outcome
handling once they render inside `#signin-dialog` — this is inherited, not
added.

### The pill and the auto-open rule

One comparison carries the whole of D8's auto-open rule:

```js
const opens = state === 'bad' && lastState !== 'bad';
lastState = state;
return opens;
```

`lastState` starts `null`, so the first `bad` answer is a transition by
construction; once recorded as `bad`, every further poll that is still `bad`
fails the comparison whether or not the operator closed the dialog in
between — which is what makes "do not reopen until it leaves bad and comes
back" fall out for free rather than needing a `dismissed` flag or a `dialog`
`close` listener. A `close` listener was considered and rejected: this file
already documents, for the create dialog, that `command="close"` does not
reliably fire one.

### Settings

`sectionSignIn`, `signInPanelFor`, and the `SignIn` field on `settingsView`
are deleted rather than deprecated. `shownSection` gets no new special case —
`?section=Sign-in` falls through exactly as any other unrecognised value
already does, which is what SC-003 pins.

## Project Structure

```text
internal/httpapi/
├── authstatus.go       NEW  authState, authCache, signInRelay interface, GET /dashboard/auth
├── signin.go            MOD  signInPanel moved in from settings.go; GET /dashboard/signin/view;
│                             redirectSignIn points at "/" with the signin=open marker;
│                             each action route invalidates the cache
├── settings.go          MOD  SignIn field, sectionSignIn, signInPanelFor removed
├── server.go             MOD  Server.signin narrowed to signInRelay; authCache field; two new routes
├── outcome.go            MOD  pathSettingsPage / redirectSection removed (their only caller moved)
└── *_test.go             MOD/NEW  authstatus_test.go, signinview_test.go; signin_test.go's redirect
                                assertion updated; settings_test.go's route sweep and menu tests extended

internal/audit/
└── audit.go              MOD  ActionDashboardAuth, ActionDashboardSignInView

web/
├── templates/partials/header.html    MOD  the auth control and #signin-dialog
├── templates/partials/signin-panel.html  NEW  the fragment (moved content)
├── templates/settings.html           MOD  Sign-in section and menu entry removed
├── static/crswd.css                  MOD  --state-ok/--state-bad, .pill-ok/.pill-bad, button.pill
└── static/crswd.js                   MOD  the auth-control module; one line in the existing submit
                                            handler to refresh the sign-in fragment after its own forms post

docs/
├── auth-and-sessions.md  MOD  sign-in section now describes the header/dialog; new Lifetimes row
├── components.md         MOD  Header: the auth control; Modal: the second call site
├── design-system.md      MOD  --state-ok/--state-bad in the state table
└── README.md              MOD  "Settings → Sign-in" instructions rewritten
```

## Complexity Tracking

No violations. One thing worth naming for review: `Server.signin`'s type
changed from the concrete `*loginrelay.Relay` to a narrowed interface
(`signInRelay`) so the cache's call-count tests can double it without shelling
out to a real `claude` binary. This mirrors `loginrelay.Controller`'s own
narrowing of `tmuxctl.Controller` and changes no production behaviour —
`*loginrelay.Relay` satisfies the interface unchanged, and every existing
caller of `s.signin` reaches the same five methods it always did.
