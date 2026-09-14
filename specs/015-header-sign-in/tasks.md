---
description: "Task list for 015-header-sign-in"
---

# Tasks: Auth in the Header, Not in Settings

**Tests**: REQUIRED per `AGENTS.md`.

## Phase 1: Setup
- [X] T001 Confirm the tree is green before touching it.

## Phase 2: User Story 1 — the header always says (P1)
- [X] T002 Add `--state-ok` / `--state-bad` tokens, `.pill-ok` / `.pill-bad` rules, and `button.pill` (the button-on-a-pill reset) to `web/static/crswd.css`.
- [X] T003 Add both tokens to `docs/design-system.md`'s state table and the `checking`/could-not-ask note beside it.
- [X] T004 Add the auth control (`<button>`, `pill pill-unknown`, `command="show-modal" commandfor="signin-dialog"`, `auth: checking`) to `web/templates/partials/header.html`, between `.operator` and `.masthead-link`.
- [X] T005 Add `authState`, `authStatusResponse`, `signInRelay`, `authCache`, `askAuthState`, `authStateCached` to a new `internal/httpapi/authstatus.go`.
- [X] T006 Narrow `Server.signin` from `*loginrelay.Relay` to `signInRelay`; add the `authCache` field.
- [X] T007 Register `GET /dashboard/auth` behind `handleBrowser` with a new `audit.ActionDashboardAuth`.
- [X] T008 Add the pill's own poll module to `web/static/crswd.js`: fetch on load, every 60s while visible, immediate + reset on `visibilitychange`.
- [X] T009 Tests: `TestAuthStatusStates`, `TestAuthStatusNeverCarriesTheSignInLink`, `TestAuthStatusIsNoStore`, `TestAuthStatusRequiresIdentity`, `TestHeaderRendersTheAuthControl`.

## Phase 3: User Story 2 — the dialog (P1)
- [X] T010 Add `#signin-dialog` (Modal family) to `web/templates/partials/header.html`, body shipping a no-script fallback line.
- [X] T011 Move `signInPanel` (struct + `SignedInTrue`) from `internal/httpapi/settings.go` into `internal/httpapi/signin.go`; add `AuthState` field.
- [X] T012 Add `web/templates/partials/signin-panel.html` (moved content, new root `data-auth-state` / `data-signin-running`).
- [X] T013 Add `signInPanelFor` (new signature, no `shown` gate) and `GET /dashboard/signin/view` (`signInView`) to `internal/httpapi/signin.go`; register with a new `audit.ActionDashboardSignInView`.
- [X] T014 Add `refreshAuthCache` (fresh ask, stores into the cache) to `internal/httpapi/authstatus.go`; call it from `signInPanelFor`.
- [X] T015 Add the dialog-fetch module to `web/static/crswd.js`: fetch on open (click or auto), swap `[data-signin-body]`, refresh every 3s while `data-signin-running="true"`, stop on close.
- [X] T016 Refresh the sign-in fragment after any of its own forms post (one line in the existing submit-interception module, exposed as `window.crswdReloadSignInPanel`).
- [X] T017 Tests moved and added in `internal/httpapi/signinview_test.go`: `TestSignInPanelDistinguishesSignedOutFromSignedIn`, `TestSignInPanelStates`, `TestSignInPanelNotAvailableStillCarriesAnAuthState`, `TestSignInPanelRootCarriesAuthStateAndRunning`, `TestSignInPanelLinkOnlyInHref`, `TestSignInViewRequiresIdentity`, `TestSignInViewIsNoStore`, `TestSignInViewAsksFreshAndStoresIntoTheCache`.
- [X] T018 `TestHeaderRendersTheSignInDialog`; confirm `TestHeaderHasExactlyTwoAnchors` is unchanged and still green.

## Phase 4: User Story 3 — the auto-open rule (P2)
- [X] T019 Add the transition-based auto-open comparison to the pill module (`opens := state === 'bad' && lastState !== 'bad'`), wired to both the poll and the dialog's own fresh answer.
- [X] T020 Add the `signin=open` on-load check to the same module.
- [X] T021 Update `TestTheInvokerFallbackIsFeatureDetected` to account for the one `showModal()` call this module makes outside the feature-detection block — a programmatic open with no click to race, counted by name rather than folded into the existing guard's count.

## Phase 5: The three action routes and settings
- [X] T022 Add `querySignInOpen` / `signInOpenMarker`; rewrite `redirectSignIn` to redirect to `pathFleet` with the outcome plus the marker.
- [X] T023 Invalidate `authCache` at the top of `signInFromBrowser`, `signInCodeFromBrowser`, `signInCancelFromBrowser`.
- [X] T024 Remove `pathSettingsPage` and `redirectSection` from `internal/httpapi/outcome.go` (dead once the sign-in redirect moved).
- [X] T025 Update `TestSignInStartReturnsToItsOwnPanel` → `TestSignInStartReturnsToTheDashboardWithTheOpenMarker`; add `TestTheThreeSignInPostsInvalidateTheAuthCache` to `authstatus_test.go`.
- [X] T026 Remove `SignIn` field, `sectionSignIn`, `signInPanelFor` from `internal/httpapi/settings.go`; remove the Sign-in menu entry and section block from `web/templates/settings.html`.
- [X] T027 Add `TestSettingsNoLongerOffersSignIn` pinning `?section=Sign-in`'s fallthrough is byte-identical to the default section.

## Phase 6: Cache correctness and parity
- [X] T028 `TestAuthCacheServesWithinTTL`, `TestAuthCacheAsksAgainAfterTTL`, `TestAuthCacheSharesOneExecAmongConcurrentCallers`, `TestAuthCacheCachesCouldNotAskToo`.
- [X] T029 Add the two new routes to `registeredPatterns` and the driven-route table in `TestFullRouteSweepLeaksNoSecret`; confirm `TestNoMutatingVerbRegistered`'s shape still holds for both via `TestAuthStatusAndSignInViewAreGETOnly`.
- [X] T030 Add `ok` / `bad` to `documentedStates` and `designTokens` in `stylesheet_test.go`.

## Phase 7: Polish
- [X] T031 Update `docs/components.md` (Header: the auth control; Modal: the second call site) and `docs/auth-and-sessions.md` (sign-in section rewritten; new Lifetimes row).
- [X] T032 Update `README.md`'s "Signing Claude in without going to the host" instructions.
- [X] T033 Run the full gate: `gofmt`, `go vet` (default/tmux/quickstart/dev), `go build`, `go test` (default/dev/tmux), `golangci-lint`.
