// Internal test, matching the rest of the package. GET /dashboard/signin/view
// is the sign-in dialog's own fragment (spec 015), replacing the panel that
// used to be a section of the settings page — this file is where that
// panel's own tests moved to, alongside the route that now serves it.
package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// signInViewPath is derived from the pattern the server registers rather than
// spelled again here, for the reason authPath is.
var signInViewPath = strings.TrimPrefix(patternDashboardSignInView, http.MethodGet+" ")

// signInPanelPage executes the fragment's own template directly against a
// constructed signInPanel, the way settings_test.go's own template tests
// execute "settings" against a settingsView.
//
// Driving this through a real *loginrelay.Relay would mean a fake `claude`
// binary on the test binary's own PATH to control what SignedIn() answers,
// for a defect that lives entirely in the template's branching over a value
// it already has. Executing the template against a hand-built panel proves
// the same thing without any of that.
func signInPanelPage(t *testing.T, panel *signInPanel) string {
	t.Helper()

	var page strings.Builder
	if err := newTestServer(t, loopbackListen).templates.ExecuteTemplate(&page, "signin-panel", panel); err != nil {
		t.Fatalf("execute the signin-panel template: %v", err)
	}
	return page.String()
}

// TestSignInPanelDistinguishesSignedOutFromSignedIn is the failing-first proof
// of the bug this panel was built to fix, moved here with the panel itself:
// html/template's {{ if }} on a pointer tests whether it is nil, not what it
// points at, so a *bool holding false has always rendered exactly as truthy
// as one holding true.
//
// Measured 2026-09-09 against crswd v0.106, while this panel was still a
// section of the settings page: an isolated daemon with HOME pointed at an
// empty scratch dir, where `claude auth status --json` answered
// `{"loggedIn": false}`, rendered "This host <strong>is signed in</strong>."
// anyway — the one moment an operator opens this panel to find out why
// sessions are failing, it told them there was nothing to do.
//
// **Must fail when** a non-nil *bool pointing at false renders the signed-in
// sentence, or fails to render the signed-out one.
func TestSignInPanelDistinguishesSignedOutFromSignedIn(t *testing.T) {
	t.Parallel()

	signedOut := false
	page := signInPanelPage(t, &signInPanel{Available: true, Token: "test-token", SignedIn: &signedOut, AuthState: authBad})

	if strings.Contains(page, "<strong>is signed in</strong>") {
		t.Errorf("a *bool pointing at false rendered the signed-in sentence:\n%s", page)
	}
	if !strings.Contains(page, "<strong>not signed in</strong>") {
		t.Errorf("a *bool pointing at false did not render a signed-out sentence at all:\n%s", page)
	}
}

// TestSignInPanelStates is the table this bug argues for: three states, three
// mutually exclusive sentences, asserted together so a fix for one cannot
// silently break another.
//
// The wanted and refused strings carry the <strong> tags around "is signed
// in" and "not signed in" rather than the bare phrases: the "could not ask"
// sentence itself contains the bare substring "is signed in" ("...whether
// this host is signed in..."), which made an earlier draft of this test fail
// for the wrong reason.
func TestSignInPanelStates(t *testing.T) {
	t.Parallel()

	signedIn, signedOut := true, false

	tests := map[string]struct {
		panel  *signInPanel
		want   string
		refuse []string
	}{
		"signed in": {
			panel:  &signInPanel{Available: true, Token: "test-token", SignedIn: &signedIn, AuthState: authOK},
			want:   "<strong>is signed in</strong>",
			refuse: []string{"<strong>not signed in</strong>", "could not ask"},
		},
		"signed out": {
			panel:  &signInPanel{Available: true, Token: "test-token", SignedIn: &signedOut, AuthState: authBad},
			want:   "<strong>not signed in</strong>",
			refuse: []string{"<strong>is signed in</strong>", "could not ask"},
		},
		"could not ask": {
			panel:  &signInPanel{Available: true, Token: "test-token", SignedIn: nil, AuthState: authUnknown},
			want:   "could not ask",
			refuse: []string{"<strong>is signed in</strong>", "<strong>not signed in</strong>"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			page := signInPanelPage(t, tc.panel)
			if !strings.Contains(page, tc.want) {
				t.Errorf("rendered page does not contain %q:\n%s", tc.want, page)
			}
			for _, absent := range tc.refuse {
				if strings.Contains(page, absent) {
					t.Errorf("rendered page unexpectedly contains %q:\n%s", absent, page)
				}
			}
		})
	}
}

// TestSignInPanelNotAvailableStillCarriesAnAuthState covers the fourth branch
// none of the three above do: a daemon with no relay at all. It renders
// neither signed-in sentence — the "no relay" note pre-empts them — and it
// still has to carry a data-auth-state, because the dialog's auto-open logic
// reads that attribute unconditionally.
func TestSignInPanelNotAvailableStillCarriesAnAuthState(t *testing.T) {
	t.Parallel()

	page := signInPanelPage(t, &signInPanel{Available: false, AuthState: authUnknown})

	if !strings.Contains(page, "no sign-in relay") {
		t.Errorf("an unavailable relay does not render the no-relay note:\n%s", page)
	}
	if !strings.Contains(page, `data-auth-state="unknown"`) {
		t.Errorf("an unavailable relay's fragment does not carry data-auth-state=\"unknown\":\n%s", page)
	}
}

// TestSignInPanelRootCarriesAuthStateAndRunning is D4's root-element
// requirement: crswd.js reads these two attributes rather than re-deriving
// Available/SignedIn/nil and Running into the same words a second time, so
// the root element has to carry both whichever branch inside it rendered.
//
// **Must fail when** either attribute is missing, or reads the wrong value.
func TestSignInPanelRootCarriesAuthStateAndRunning(t *testing.T) {
	t.Parallel()

	signedIn := true
	page := signInPanelPage(t, &signInPanel{
		Available: true,
		Token:     "test-token",
		SignedIn:  &signedIn,
		Running:   true,
		AuthState: authOK,
	})

	if !strings.Contains(page, `data-auth-state="ok"`) {
		t.Errorf("the fragment's root does not carry data-auth-state=\"ok\":\n%s", page)
	}
	if !strings.Contains(page, `data-signin-running="true"`) {
		t.Errorf("the fragment's root does not carry data-signin-running=\"true\":\n%s", page)
	}

	notRunning := signInPanelPage(t, &signInPanel{Available: true, SignedIn: &signedIn, Running: false, AuthState: authOK})
	if !strings.Contains(notRunning, `data-signin-running="false"`) {
		t.Errorf("a panel with no window running does not carry data-signin-running=\"false\":\n%s", notRunning)
	}
}

// TestSignInPanelLinkOnlyInHref is docs/auth-and-sessions.md's binding rule,
// held at the one fragment that ever renders the sign-in URL at all: it must
// appear as an href and nowhere else — not as bare text, not in a second
// attribute, not repeated in prose.
//
// **Must fail when** the link appears anywhere in the markup other than
// inside one href="...".
func TestSignInPanelLinkOnlyInHref(t *testing.T) {
	t.Parallel()

	const link = "https://claude.ai/oauth/authorize?code_challenge=test-only-pkce-canary"

	page := signInPanelPage(t, &signInPanel{
		Available: true,
		Token:     "test-token",
		Running:   true,
		Link:      link,
		AuthState: authUnknown,
	})

	if !strings.Contains(page, `href="`+link+`"`) {
		t.Errorf("the fragment does not render the link as an href at all:\n%s", page)
	}
	if n := strings.Count(page, link); n != 1 {
		t.Errorf("the fragment carries the link %d times; want exactly 1, in the href:\n%s", n, page)
	}
}

// TestSignInViewRequiresIdentity is FR-006 at this route, the claim
// TestVersionRequiresIdentity makes at its neighbour, and TestAuthStatusRequiresIdentity
// makes at authstatus_test.go's: this door refuses only by the check that
// applies to it, and this route reads it just the same as any other page.
func TestSignInViewRequiresIdentity(t *testing.T) {
	t.Parallel()

	f := newFleet(t)

	w := f.openWith(t, signInViewPath, absent)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET %s with no assertion at all was answered %d (%s); want %d",
			signInViewPath, w.Code, w.Body.String(), http.StatusUnauthorized)
	}
}

// TestSignInViewIsNoStore is D4: the sign-in dialog re-fetches this while a
// sign-in is running, and a cached copy standing in for a fresh ask would be
// the one place this daemon lied about a live credential.
func TestSignInViewIsNoStore(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	w := f.open(t, signInViewPath)
	if got := w.Header().Get(headerCacheControl); got != cacheControlNoStore {
		t.Errorf("Cache-Control = %q; want %q", got, cacheControlNoStore)
	}
}

// TestSignInViewAsksFreshAndStoresIntoTheCache is D4 and D7 together: the
// fragment never reads the header pill's cache, and what it learns is stored
// back into it, so a poll of GET /dashboard/auth shortly afterwards does not
// repeat the exec this route just paid for.
//
// **Must fail when** signin/view answers from a stale cache instead of
// asking, or when GET /dashboard/auth re-asks anyway right after it.
func TestSignInViewAsksFreshAndStoresIntoTheCache(t *testing.T) {
	t.Parallel()

	d := newSignInDoor(t)
	relay := &fakeRelay{signedIn: true}
	d.signin = relay
	d.clock = fixedClock{at: testTime}

	// Primed stale on purpose, with the opposite answer, so a fragment that
	// merely read the cache instead of asking would report it.
	d.authCache.state = authBad
	d.authCache.fetched = testTime
	d.authCache.has = true

	w := d.get(t, signInViewPath)
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d (%s); want %d", signInViewPath, w.Code, w.Body.String(), http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), `data-auth-state="ok"`) {
		t.Errorf("GET %s answered the stale cached state rather than asking fresh:\n%s", signInViewPath, w.Body.String())
	}
	if calls := relay.callCount(); calls != 1 {
		t.Fatalf("GET %s ran SignedIn %d times; want 1", signInViewPath, calls)
	}

	if got := authAnswer(t, d.get(t, authPath)); got.State != authOK {
		t.Errorf("GET %s after signin/view = %q; want %q — it should have reused what that route just learned", authPath, got.State, authOK)
	}
	if calls := relay.callCount(); calls != 1 {
		t.Errorf("GET %s ran SignedIn again (%d calls total); want the fragment's ask to have been stored into the cache", authPath, calls)
	}
}
