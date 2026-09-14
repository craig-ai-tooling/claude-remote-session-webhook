package httpapi

// signin.go is the browser's half of the sign-in relay: three actions that
// summon Claude Code's own login in a window of its own, carry a code the
// operator types into it, and end it.
//
// # Why these carry no {id}
//
// A sign-in names no session. There is one Claude credential store on this host
// and every session shares it, so what these repair belongs to the daemon rather
// than to anything an operator owns — which is why there is no ownership check
// behind them and no uniform not-found for them to give. They are the restart's
// shape, not the rename's.
//
// # What may never reach a record, a URL, or a page
//
// docs/auth-and-sessions.md is binding here and it is stricter than anywhere
// else on this door:
//
//   - The code is a live credential. It is read from PostForm, handed straight to
//     internal/loginrelay, and never logged, never put in an audit record, never
//     echoed into an outcome, and never rendered back into the form. Every
//     refusal below is a sentinel authored in this file, so no record and no
//     redirect can carry a byte the operator typed (FR-042).
//   - The sign-in URL is a one-shot PKCE challenge. It reaches exactly one place
//     — the sign-in dialog's own fragment (GET /dashboard/signin/view), as a
//     link — and it is never in a query string, never in the trail, never in
//     GET /dashboard/auth's JSON, and never on the fleet grid. claudeauth.Prompt
//     redacts it under %v so the ordinary ways a value leaks cannot be the way
//     this one does.
//   - Nothing is ever submitted that the operator did not type. These routes
//     summon a screen and carry what they are handed; they answer nothing on the
//     operator's behalf.

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/nctiggy/claude-remote-session-webhook/internal/access"
	"github.com/nctiggy/claude-remote-session-webhook/internal/loginrelay"
)

// The three patterns. Each is a POST with no {id}, for the reason above.
const (
	patternDashboardSignIn       = "POST /dashboard/signin"
	patternDashboardSignInCode   = "POST /dashboard/signin/code"
	patternDashboardSignInCancel = "POST /dashboard/signin/cancel"

	// patternDashboardSignInView is the fourth (spec 015): the fragment the
	// header's sign-in dialog fetches to fill itself in. It carries no {id} for
	// the same reason the three above do not — this door has nothing to do with
	// a session, and this route only reads.
	patternDashboardSignInView = "GET /dashboard/signin/view"
)

// signInPanel is what an operator is shown about Claude Code's login.
//
// It is deliberately thin. The pane behind it holds the most sensitive screen on
// this host, and what a person needs from it is: is this host signed in, is a
// sign-in waiting, and if so what is the link. Everything else on that screen is
// none of the dashboard's business.
//
// It used to be composed for one section of the settings page (settings.go,
// before spec 015). It is composed for GET /dashboard/signin/view now — the
// sign-in dialog's own fragment — and nothing about the type changed to move:
// the settings page executed this same struct against this same template
// content, under a heading that page supplied. The dialog supplies its own.
type signInPanel struct {
	// Available is whether this daemon has a relay at all. False means the
	// configured start command names nothing runnable, which the panel says
	// rather than hiding behind a button that would refuse.
	Available bool

	// SignedIn is what `claude auth status` answered.
	//
	// It is asked of the CLI rather than read off the screen, because the wording
	// Claude Code prints after a successful sign-in has never been captured here
	// and this project does not add a signature it has not seen. Nil when the
	// question could not be put — a host that could not answer is not a host that
	// is signed out, and saying so would send an operator to fix what is not
	// broken.
	SignedIn *bool

	// Running is whether a sign-in window is open.
	Running bool

	// Link is the sign-in URL, and it is the one place in this daemon it is ever
	// rendered.
	//
	// It carries a one-shot PKCE challenge, so it is on this fragment and nowhere
	// else: not on the fleet, not in a query string, not in the trail, and not in
	// GET /dashboard/auth's JSON. Empty while the screen is still drawing, which
	// the template renders as waiting rather than as no link — the two mean
	// different things to somebody deciding whether to press again.
	Link string

	// Token is what this panel's forms carry, minted for this render and this
	// identity. Empty draws no forms, for the reason the edit form is not drawn
	// without one: a control that could not be submitted is worse than none.
	Token string

	// AuthState is the header pill's own word for this exact state (D4), carried
	// so the fragment's root element can say it once in a data attribute rather
	// than the script re-deriving Available/SignedIn/nil into the same three
	// words a second time.
	AuthState authState
}

// SignedInTrue is what SignedIn points at, and it exists because the template
// cannot ask that question itself.
//
// html/template's {{ if }} on a pointer tests whether the pointer is nil, not
// what it points at — a *bool holding false is exactly as truthy as one holding
// true. Measured 2026-09-09: a host `claude auth status --json` reported as
// signed out rendered "This host is signed in" anyway, because the template
// branched on `.SignedIn` directly. Meaningful only once a caller has ruled out
// nil, the way the template does — the sentence for that case never calls this.
func (p *signInPanel) SignedInTrue() bool {
	return p.SignedIn != nil && *p.SignedIn
}

// signInPanelFor composes the fragment GET /dashboard/signin/view answers with
// (spec 015).
//
// It does a **fresh** ask — never the cache authstatus.go keeps for the header
// pill — and stores what it learns back into that cache (refreshAuthCache), so
// a poll of GET /dashboard/auth shortly afterwards does not repeat the exec
// this route just paid for. The panel is answering "is this host signed in
// right now"; the pill is answering "was it, recently enough" — the same
// underlying fact, asked on two different terms, which is why one ask can
// answer both without either lying to the other.
func (s *Server) signInPanelFor(r *http.Request, operator *access.VerifiedOperator) *signInPanel {
	panel := &signInPanel{}
	if s.signin == nil {
		// A daemon whose start command names nothing runnable. The panel says so
		// rather than offering a button that would refuse, which is the same rule
		// the edit form follows about a field that could not be submitted.
		panel.AuthState = authUnknown
		return panel
	}
	panel.Available = true

	// Minted before the reads rather than after, unlike the session page's: this
	// panel has no uniform not-found to fall into, so there is no render that
	// would be handing out a token for a page it is not going to serve.
	panel.Token, _ = s.mintPageToken(r, operator)

	switch state := s.refreshAuthCache(r.Context()); state {
	case authOK:
		signedIn := true
		panel.SignedIn = &signedIn
		panel.AuthState = state
	case authBad:
		signedIn := false
		panel.SignedIn = &signedIn
		panel.AuthState = state
	default:
		// Left nil. A host that could not answer is not a host that is signed
		// out, and rendering it as signed out would send an operator to repair a
		// credential that is fine. askAuthState (authstatus.go) already reported
		// the failure once, so there is nothing further to say here.
		panel.AuthState = authUnknown
	}

	state, err := s.signin.State(r.Context())
	if err != nil {
		s.report(fmt.Errorf("read the sign-in window: %w", err))
		return panel
	}
	panel.Running = state.Running
	if state.Prompt != nil {
		panel.Link = state.Prompt.URL
	}
	return panel
}

// signInView serves GET /dashboard/signin/view (spec 015, D4): the sign-in
// dialog's own fragment, replacing the panel that used to live on the settings
// page. It only reads, on the same terms GET /dashboard/version does, so it
// goes through handleBrowser and not handleAction.
func (s *Server) signInView(w http.ResponseWriter, r *http.Request) {
	operator, ok := OperatorFrom(r.Context())
	if !ok {
		AuditFrom(r.Context()).Deny(errDashboardNoOperator.Error())
		s.refuseBrowser(w)
		return
	}

	// No SetSessionID: this fragment is about the daemon's one shared credential
	// and not about any session.
	s.renderPage(w, r, http.StatusOK, "signin-panel", s.signInPanelFor(r, operator))
}

// fieldCode is the name of the one field the code arrives in.
//
// Spelled once, so the name the form writes and the name the handler reads
// cannot drift into a route that silently receives nothing and reports an empty
// code for one the operator really typed.
const fieldCode = "code"

// The refusals these routes author. Sentinels written here, so no record can
// carry a byte the caller chose — and on this door that rule is not a
// formality, because one of the fields is a credential.
var (
	errSignInUnconfirmed = errors.New("a browser sign-in arrived without the confirming step")
	errSignInUnwired     = errors.New("the sign-in route was reached on a daemon with no relay behind it")
	errSignInRefused     = errors.New("the sign-in could not be started on this host")
	errSignInAlready     = errors.New("a sign-in was already in progress")

	// errSignInNoCode and errSignInBadCode are kept apart because only one of
	// them is the operator's to fix by typing again. Neither names any part of
	// what was submitted.
	errSignInNoCode  = errors.New("a code was submitted with nothing in it")
	errSignInBadCode = errors.New("a code was submitted carrying characters a code does not")

	errSignInNotRunning = errors.New("a code was submitted with no sign-in waiting for one")
	errSignInCancel     = errors.New("the sign-in window could not be ended")
)

// signInFromBrowser is POST /dashboard/signin.
//
// Everything that authorises it has already run: handleAction wrapped this in
// the gate, so an identity is verified, the browser has said the request came
// from this page, and the form carried a token minted for that identity.
func (s *Server) signInFromBrowser(w http.ResponseWriter, r *http.Request) {
	if _, ok := OperatorFrom(r.Context()); !ok {
		AuditFrom(r.Context()).Deny(errDashboardNoOperator.Error())
		s.refuseBrowser(w)
		return
	}

	// This host's credential may be about to change under any of the three
	// routes on this door (D7, spec 015), so the header pill's cache is cleared
	// unconditionally rather than only on the branch that actually touched the
	// relay: the next ask is one cheap exec, and a stale "ok" surviving a
	// refused or half-run attempt is the failure worth avoiding.
	s.authCache.invalidate()

	// The confirming step first, ahead of anything else, which is this door's
	// ordering rule throughout. A sign-in started by accident is a window holding
	// a live challenge that nobody meant to create.
	if r.PostForm.Get(fieldConfirm) != confirmYes {
		AuditFrom(r.Context()).Deny(errSignInUnconfirmed.Error())
		s.redirectSignIn(w, r, outcomeSignInUnconfirmed)
		return
	}

	if s.signin == nil {
		AuditFrom(r.Context()).Deny(errSignInUnwired.Error())
		s.redirectSignIn(w, r, outcomeSignInRefused)
		return
	}

	switch err := s.signin.Start(r.Context()); {
	case errors.Is(err, loginrelay.ErrAlreadyRunning):
		// Not an error and not a success: the operator asked for something that
		// is already true, and the reason it is refused rather than restarted is
		// that restarting abandons a challenge they may be part-way through
		// answering on their phone.
		AuditFrom(r.Context()).Deny(errSignInAlready.Error())
		s.redirectSignIn(w, r, outcomeSignInRunning)
	case err != nil:
		// The host's own account of the failure goes to the report channel where
		// an operator is already reading, never into the record or the redirect.
		s.report(err)
		AuditFrom(r.Context()).Deny(errSignInRefused.Error())
		s.redirectSignIn(w, r, outcomeSignInRefused)
	default:
		s.redirectSignIn(w, r, outcomeSignInStarted)
	}
}

// signInCodeFromBrowser is POST /dashboard/signin/code.
//
// The one route in this daemon that carries a credential the operator typed.
// What it does with it is hand it to internal/loginrelay and forget it: the
// value is never assigned to anything that outlives this call, never wrapped in
// an error, and never chosen as an outcome.
func (s *Server) signInCodeFromBrowser(w http.ResponseWriter, r *http.Request) {
	if _, ok := OperatorFrom(r.Context()); !ok {
		AuditFrom(r.Context()).Deny(errDashboardNoOperator.Error())
		s.refuseBrowser(w)
		return
	}
	s.authCache.invalidate()

	if s.signin == nil {
		AuditFrom(r.Context()).Deny(errSignInUnwired.Error())
		s.redirectSignIn(w, r, outcomeSignInRefused)
		return
	}

	// No confirming step. Typing a code into a box and pressing the one button
	// beside it IS the confirmation — a second one would be a dialog between an
	// operator and the thing they just typed, and docs/auth-and-sessions.md's
	// rule is that nothing is auto-submitted, not that everything is asked twice.
	switch err := s.signin.Deliver(r.Context(), r.PostForm.Get(fieldCode)); {
	case errors.Is(err, loginrelay.ErrEmptyCode):
		AuditFrom(r.Context()).Deny(errSignInNoCode.Error())
		s.redirectSignIn(w, r, outcomeSignInNoCode)
	case errors.Is(err, loginrelay.ErrUnusableCode):
		AuditFrom(r.Context()).Deny(errSignInBadCode.Error())
		s.redirectSignIn(w, r, outcomeSignInBadCode)
	case errors.Is(err, loginrelay.ErrNotRunning):
		AuditFrom(r.Context()).Deny(errSignInNotRunning.Error())
		s.redirectSignIn(w, r, outcomeSignInNotRunning)
	case err != nil:
		// Reported, not recorded. loginrelay's delivery error is already written
		// to carry none of the code, and this keeps the trail's account of it to
		// a sentinel either way.
		s.report(err)
		AuditFrom(r.Context()).Deny(errSignInRefused.Error())
		s.redirectSignIn(w, r, outcomeSignInRefused)
	default:
		s.redirectSignIn(w, r, outcomeSignInCodeSent)
	}
}

// signInCancelFromBrowser is POST /dashboard/signin/cancel.
//
// It is how an operator abandons an attempt, and it is also the tidy-up after a
// successful one — a window left sitting on the host is a Node process and a
// spent challenge with nothing watching them.
func (s *Server) signInCancelFromBrowser(w http.ResponseWriter, r *http.Request) {
	if _, ok := OperatorFrom(r.Context()); !ok {
		AuditFrom(r.Context()).Deny(errDashboardNoOperator.Error())
		s.refuseBrowser(w)
		return
	}
	s.authCache.invalidate()

	if s.signin == nil {
		AuditFrom(r.Context()).Deny(errSignInUnwired.Error())
		s.redirectSignIn(w, r, outcomeSignInRefused)
		return
	}

	// No confirming step here either, and for the opposite reason to the code's:
	// this is the undo. An operator who pressed it by accident has lost a
	// challenge they can replace by pressing start again, and putting a
	// confirmation in front of the way out of a flow is how people get stuck in
	// one.
	if err := s.signin.Stop(r.Context()); err != nil {
		s.report(err)
		AuditFrom(r.Context()).Deny(errSignInCancel.Error())
		s.redirectSignIn(w, r, outcomeSignInRefused)
		return
	}
	s.redirectSignIn(w, r, outcomeSignInCancelled)
}

// querySignInOpen is the marker a redirect from one of these three routes
// carries alongside the outcome code, so a browser that followed the 303
// rather than crswd.js's fetch still lands on a dashboard that opens the
// sign-in dialog on its own (D6, spec 015). It is read on load by crswd.js and
// by nothing else in this daemon — no handler ever reads it back, and it
// names no fact this daemon acts on.
const querySignInOpen = "signin"

// signInOpenMarker is the one value querySignInOpen ever carries. A named
// constant rather than a bare "open" repeated at each call site, so the
// spelling this file writes and the spelling crswd.js reads for are the same
// word once.
const signInOpenMarker = "open"

// redirectSignIn is redirectOutcome pointed at the dashboard with the marker
// above (spec 015). It used to point at a section of the settings page that no
// longer exists — the sign-in relay's panel moved behind the header's own auth
// control — and what replaces the panel as "the thing that reflects what the
// action did" is the dashboard's own outcome banner, the one every other
// action already renders there, with the marker asking the page to also open
// the dialog a scripted browser never left.
func (s *Server) redirectSignIn(w http.ResponseWriter, r *http.Request, code outcome) {
	to := url.URL{Path: pathFleet, RawQuery: url.Values{
		queryOutcome:    []string{string(code)},
		querySignInOpen: []string{signInOpenMarker},
	}.Encode()}
	http.Redirect(w, r, to.String(), http.StatusSeeOther)
}
