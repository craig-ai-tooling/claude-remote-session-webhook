// Internal test, matching the rest of the package. GET /dashboard/signin/view
// is the sign-in dialog's own fragment (spec 015), replacing the panel that
// used to be a section of the settings page — this file is where that
// panel's own tests moved to, alongside the route that now serves it.
package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/codexauth"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/loginrelay"
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
	d.signins[harness.Claude] = relay
	d.clock = fixedClock{at: testTime}

	// Primed stale on purpose, with the opposite answer, so a fragment that
	// merely read the cache instead of asking would report it.
	d.authCaches[harness.Claude].state = authBad
	d.authCaches[harness.Claude].fetched = testTime
	d.authCaches[harness.Claude].has = true

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

const (
	// codexTestCode and codexTestURL are placeholders in the shape research F2
	// records. The code is a canary: the real one is a live credential.
	codexTestCode = "ABCD-EFGH1"
	codexTestURL  = "https://auth.openai.com/codex/device"
)

// codexPanelPage renders the Codex panel in the state a sign-in is waiting.
func codexPanelPage(t *testing.T) string {
	t.Helper()

	signedOut := false
	return signInPanelPage(t, &signInPanel{
		Available: true,
		Token:     "test-token",
		Harness:   harness.Codex,
		SignedIn:  &signedOut,
		Running:   true,
		DeviceURL: codexTestURL,
		Code:      codexTestCode,
		AuthState: authBad,
	})
}

// TestCodexPanelShowsLinkAndCode: the device link as an href and the one-time
// code in a <code> element, which is what the operator reads off a phone.
func TestCodexPanelShowsLinkAndCode(t *testing.T) {
	t.Parallel()

	page := codexPanelPage(t)

	if !strings.Contains(page, `href="`+codexTestURL+`"`) {
		t.Errorf("the Codex panel does not render the device link as an href:\n%s", page)
	}
	if !strings.Contains(page, `rel="noopener noreferrer"`) {
		t.Errorf("the Codex link lacks rel=noopener noreferrer:\n%s", page)
	}
	if !strings.Contains(page, "<code>"+codexTestCode+"</code>") {
		t.Errorf("the Codex panel does not show the code in a <code> element:\n%s", page)
	}
}

// TestCodexPanelWaitsWhileTheScreenDraws: no link and no code yet reads as
// waiting, not as an empty link.
func TestCodexPanelWaitsWhileTheScreenDraws(t *testing.T) {
	t.Parallel()

	page := signInPanelPage(t, &signInPanel{
		Available: true, Token: "test-token", Harness: harness.Codex, Running: true, AuthState: authBad,
	})
	if strings.Contains(page, "<a ") || strings.Contains(page, "<code>") {
		t.Errorf("a Codex panel with nothing drawn rendered a link or a code:\n%s", page)
	}
	if !strings.Contains(page, "starting") {
		t.Errorf("a Codex panel with nothing drawn did not say it is waiting:\n%s", page)
	}
}

// TestCodexPanelHasNoCodeForm: Codex takes no code from the browser (D12).
func TestCodexPanelHasNoCodeForm(t *testing.T) {
	t.Parallel()

	page := codexPanelPage(t)
	for _, banned := range []string{"/dashboard/signin/code", `name="code"`, "signin-code"} {
		if strings.Contains(page, banned) {
			t.Errorf("the Codex panel carries a code form (%q):\n%s", banned, page)
		}
	}
}

// TestCodexPanelFormsCarryHarness: every form in a Codex panel, Start and
// Cancel in both states, names Codex, so none can reach the Claude relay.
// Claude's forms name Claude for the converse reason.
func TestCodexPanelFormsCarryHarness(t *testing.T) {
	t.Parallel()

	const codexField = `<input type="hidden" name="harness" value="codex">`
	const claudeField = `<input type="hidden" name="harness" value="claude">`

	for name, panel := range map[string]*signInPanel{
		"codex running": {Available: true, Token: "t", Harness: harness.Codex, Running: true, AuthState: authBad},
		"codex idle":    {Available: true, Token: "t", Harness: harness.Codex, AuthState: authBad},
	} {
		page := signInPanelPage(t, panel)
		forms := strings.Count(page, "<form ")
		if forms == 0 {
			t.Fatalf("%s: no form rendered:\n%s", name, page)
		}
		if got := strings.Count(page, codexField); got != forms {
			t.Errorf("%s: %d forms but %d carry the codex harness field:\n%s", name, forms, got, page)
		}
	}

	for name, panel := range map[string]*signInPanel{
		"claude running": {Available: true, Token: "t", Running: true, Link: "https://claude.ai/x", AuthState: authBad},
		"claude idle":    {Available: true, Token: "t", AuthState: authBad},
	} {
		page := signInPanelPage(t, panel)
		if got, forms := strings.Count(page, claudeField), strings.Count(page, "<form "); got != forms {
			t.Errorf("%s: %d forms but %d carry the claude harness field:\n%s", name, forms, got, page)
		}
	}
}

// TestCodexPanelHasNoClaudeWording: the Codex block has its own words.
func TestCodexPanelHasNoClaudeWording(t *testing.T) {
	t.Parallel()

	signedIn := true
	for name, panel := range map[string]*signInPanel{
		"waiting":     {Available: true, Token: "t", Harness: harness.Codex, Running: true, DeviceURL: codexTestURL, Code: codexTestCode, AuthState: authBad},
		"idle":        {Available: true, Token: "t", Harness: harness.Codex, AuthState: authBad},
		"signed in":   {Available: true, Token: "t", Harness: harness.Codex, SignedIn: &signedIn, AuthState: authOK},
		"unknown":     {Available: true, Token: "t", Harness: harness.Codex, AuthState: authUnknown},
		"unavailable": {Harness: harness.Codex, AuthState: authUnknown},
	} {
		if page := signInPanelPage(t, panel); strings.Contains(page, "Claude") {
			t.Errorf("%s: the Codex panel says Claude:\n%s", name, page)
		}
	}
}

// TestCodexSignInViewReadsTheCodexRelay: GET /dashboard/signin/view?harness=codex
// shows the Codex relay's link and code, asks the Codex relay, and never the
// Claude one.
func TestCodexSignInViewReadsTheCodexRelay(t *testing.T) {
	t.Parallel()

	d := newSignInDoor(t)
	claudeRelay := &fakeRelay{signedIn: true}
	codexRelay := &fakeRelay{state: loginrelay.State{
		Running: true,
		Device:  &codexauth.Prompt{Kind: codexauth.KindDeviceCode, URL: codexTestURL, Code: codexTestCode},
	}}
	d.signins[harness.Claude] = claudeRelay
	d.signins[harness.Codex] = codexRelay
	d.authCaches[harness.Codex] = &authCache{}

	w := d.get(t, signInViewPath+"?harness=codex")
	if w.Code != http.StatusOK {
		t.Fatalf("GET = %d (%s); want 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "<code>"+codexTestCode+"</code>") || !strings.Contains(body, `href="`+codexTestURL+`"`) {
		t.Errorf("the view did not show the Codex link and code:\n%s", body)
	}
	if claudeRelay.callCount() != 0 {
		t.Errorf("a Codex view asked the Claude relay %d times", claudeRelay.callCount())
	}
	if codexRelay.callCount() != 1 {
		t.Errorf("a Codex view asked the Codex relay %d times; want 1", codexRelay.callCount())
	}

	for _, bad := range []string{"?harness=other", "?harness=", "?harness=claude&harness=codex"} {
		if got := d.get(t, signInViewPath+bad).Code; got != http.StatusBadRequest {
			t.Errorf("GET view%s = %d; want 400", bad, got)
		}
	}
}
