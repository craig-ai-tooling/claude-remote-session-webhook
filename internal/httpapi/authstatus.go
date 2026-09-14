package httpapi

// authstatus.go is GET /dashboard/auth (spec 015): the header's auth pill
// asking, over and over from every open tab, whether this host can still make
// a Claude request. It answers from a cache rather than a fresh exec every
// time, on the same terms settings.go's releaseCache does and for the same
// reason — the difference here is that a fresh ask costs a subprocess rather
// than a network round trip, and the poll is the header's own, not something
// an operator presses.
//
// # What this route must never say
//
// docs/auth-and-sessions.md's rules about the sign-in link are unchanged by
// this file, and this file is why they still hold: the answer here is one of
// three bare words, never a link, never a window's running state, never
// anything /dashboard/signin/view exists to carry. A pill that could open a
// dialog with no fetch of its own would be a pill that had to know the URL to
// do it.

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/loginrelay"
)

// signInRelay is the part of *loginrelay.Relay this package uses, narrowed for
// loginrelay.Controller's own reason: the set of things this door can ask of
// the relay is readable in one place, and a test can double it to count how
// many times SignedIn really ran without shelling out to a `claude` binary to
// find out. *loginrelay.Relay satisfies this unchanged.
type signInRelay interface {
	Start(ctx context.Context) error
	State(ctx context.Context) (loginrelay.State, error)
	Deliver(ctx context.Context, code string) error
	Stop(ctx context.Context) error
	SignedIn(ctx context.Context) (bool, error)
}

// patternDashboardAuth is the route, method included for the reason every
// other pattern in this package carries one: net/http matches both halves, so
// a path registered for GET is not silently reachable by POST.
const patternDashboardAuth = "GET /dashboard/auth"

// authState is the pill's whole vocabulary, and the JSON wire value: a named
// string rather than a bool-and-an-error so that "could not ask" is a state
// this type can hold rather than a shape the caller has to reconstruct from an
// absent field.
type authState string

const (
	// authOK is "this host can make a Claude request", read off a signed-in
	// answer.
	authOK authState = "ok"

	// authBad is the same question answered the other way: `claude auth
	// status` ran and said this host is signed out.
	authBad authState = "bad"

	// authUnknown covers two different daemon facts the pill renders identically
	// (D1): no relay at all (the configured start command names nothing
	// runnable), and a relay that could not be asked (the exec failed). Neither
	// is "signed out" — telling an operator to sign in when the real problem is
	// a missing binary sends them to fix the wrong thing — so both answer with
	// the one word that admits this daemon does not know.
	authUnknown authState = "unknown"
)

// authStatusResponse is the whole of what GET /dashboard/auth answers with.
// One field, because a code carries no data beyond itself — see the package
// doc for what it must never carry instead.
type authStatusResponse struct {
	State authState `json:"state"`
}

// authCacheTTL is D7's number: long enough that a tab polling every 60 seconds
// never asks twice inside one interval, short enough that a sign-in the
// operator just completed shows up within the next poll or two rather than
// needing a reload.
const authCacheTTL = 60 * time.Second

// authCache is the one answer this server keeps about its own credential,
// shaped like releaseCache in settings.go and for the same reason: the lock is
// held across the ask rather than only around the fields, so two callers
// inside one TTL window share one exec instead of each paying for their own —
// the second one to reach the lock finds the first one's answer already
// fresh.
type authCache struct {
	mu      sync.Mutex
	state   authState
	fetched time.Time
	has     bool
}

// invalidate clears the cache, so the next ask is a fresh one. Called by each
// of the three sign-in action routes (signin.go): none of them can be trusted
// to have left this host's credential exactly as it was.
func (c *authCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.has = false
}

// authStateCached answers from the cache when it is fresh, and asks
// s.askAuthState otherwise — one exec per TTL window, shared by whoever is
// waiting on the lock when it completes.
//
// now is read from s.clock and not from time.Now, so a test can move the
// clock without waiting on the wall one (server.go's own reason for that
// field).
func (s *Server) authStateCached(ctx context.Context) authState {
	s.authCache.mu.Lock()
	defer s.authCache.mu.Unlock()

	now := s.clock.Now()
	if s.authCache.has && now.Sub(s.authCache.fetched) < authCacheTTL {
		return s.authCache.state
	}

	state := s.askAuthState(ctx)
	s.authCache.state = state
	s.authCache.fetched = now
	s.authCache.has = true
	return state
}

// refreshAuthCache is authStateCached with the cache check skipped: a fresh
// ask every time, its answer stored back into the cache all the same (D4), so
// a poll of GET /dashboard/auth shortly after does not repeat the exec
// signin/view just paid for.
//
// The one caller is the sign-in dialog's own fragment (signin.go), which needs
// this host's *current* credential rather than whatever the header pill last
// cached — the two questions look the same and are not: the panel is answering
// "is this host signed in right now", and the pill is answering "was it,
// recently enough".
func (s *Server) refreshAuthCache(ctx context.Context) authState {
	state := s.askAuthState(ctx)

	s.authCache.mu.Lock()
	s.authCache.state = state
	s.authCache.fetched = s.clock.Now()
	s.authCache.has = true
	s.authCache.mu.Unlock()

	return state
}

// askAuthState is the one place this package asks the relay, so the cache and
// the fresh-ask route share a single reading of "could not ask" rather than
// two that might disagree about what that means.
//
// A nil relay is a daemon whose configured start command names nothing
// runnable (server.go's own comment on the field) — not a fault to report
// again here, since New already reported it once at construction.
func (s *Server) askAuthState(ctx context.Context) authState {
	if s.signin == nil {
		return authUnknown
	}

	signedIn, err := s.signin.SignedIn(ctx)
	if err != nil {
		// The same report signInPanelFor always made for this exact failure,
		// kept to one call site now that there is one place that makes it.
		s.report(fmt.Errorf("ask whether this host is signed in: %w", err))
		return authUnknown
	}
	if signedIn {
		return authOK
	}
	return authBad
}

// dashboardAuth serves GET /dashboard/auth (spec 015, D3).
//
// No page token is minted and none is read: this route only reads, on the
// same terms GET /dashboard/version does, and answers no operator-scoped
// value at all — a cache shared by every viewer of this one daemon. The
// no-store default (render.go's setBrowserSecurityHeaders, applied by
// authenticateBrowser before this handler runs) is not lifted here the way it
// is for the two embedded assets, so nothing about this answer is cached past
// the request that asked for it.
func (s *Server) dashboardAuth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, r, http.StatusOK, authStatusResponse{State: s.authStateCached(r.Context())})
}
