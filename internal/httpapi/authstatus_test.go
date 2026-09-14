// Internal test, matching the rest of the package. GET /dashboard/auth is the
// header pill's own route (spec 015), so most claims here drive it through the
// real router and the real browser door, the way version_test.go drives its
// neighbour; the cache claims drive the cache directly, because they are about
// how many times a fake relay's SignedIn ran rather than about a page.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/claudeauth"
	"github.com/nctiggy/claude-remote-session-webhook/internal/loginrelay"
)

// authPath is derived from the pattern the server registers rather than
// spelled again here, for the reason versionPath is: a renamed route would
// otherwise leave every test in this file passing against a path nothing
// claims.
var authPath = strings.TrimPrefix(patternDashboardAuth, http.MethodGet+" ")

// fakeRelay is a signInRelay whose SignedIn call count a test can read, which
// a real *loginrelay.Relay cannot offer without a fake `claude` binary on the
// test process's own PATH — driving the cache's claims through one would be
// asking the developer's own machine whether it happens to be signed in.
// Start, Deliver and Stop answer nil unconditionally: nothing in this file
// drives them, and signin_test.go already covers what they do through the
// real relay.
type fakeRelay struct {
	mu    sync.Mutex
	calls int

	signedIn bool
	err      error

	state    loginrelay.State
	stateErr error
}

func (f *fakeRelay) SignedIn(context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.signedIn, f.err
}

func (f *fakeRelay) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeRelay) State(context.Context) (loginrelay.State, error) { return f.state, f.stateErr }
func (f *fakeRelay) Start(context.Context) error                     { return nil }
func (f *fakeRelay) Deliver(context.Context, string) error           { return nil }
func (f *fakeRelay) Stop(context.Context) error                      { return nil }

// authAnswer decodes what the route said, and fails on a body that is not the
// documented object — a route answering something else has not reported a
// state, whatever text it contains. It takes the response rather than a
// caller to drive, so both *fleet (f.open) and *signInDoor (d.get) can hand
// it one without this file needing a shape both share.
func authAnswer(t *testing.T, w *httptest.ResponseRecorder) authStatusResponse {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d (%s); want %d", authPath, w.Code, w.Body.String(), http.StatusOK)
	}

	var got authStatusResponse
	dec := json.NewDecoder(strings.NewReader(w.Body.String()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("GET %s answered %q, which is not an auth response: %v", authPath, w.Body.String(), err)
	}
	return got
}

// TestAuthStatusStates is D1's whole vocabulary, each driven from a fake
// relay rather than a real one: "signed out" and "could not ask" must read
// apart, because only one of them is a fault an operator can fix by signing
// in — telling them to for the other sends them to fix the wrong thing.
//
// **Must fail when** a signed-out host reports "ok", or a host that could not
// be asked at all — no relay, or an exec that failed — reports "bad" rather
// than "unknown".
func TestAuthStatusStates(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		relay signInRelay
		want  authState
	}{
		"signed in":       {&fakeRelay{signedIn: true}, authOK},
		"signed out":      {&fakeRelay{signedIn: false}, authBad},
		"could not ask":   {&fakeRelay{err: errors.New("exec: \"crswd-no-such-binary-for-tests\": executable file not found in $PATH")}, authUnknown},
		"no relay at all": {nil, authUnknown},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFleet(t)
			f.signin = tc.relay

			if got := authAnswer(t, f.open(t, authPath)); got.State != tc.want {
				t.Errorf("GET %s = %q; want %q", authPath, got.State, tc.want)
			}
		})
	}
}

// TestAuthStatusNeverCarriesTheSignInLink is docs/auth-and-sessions.md's
// binding rule, pinned at the one route with the least reason to ever need
// it: GET /dashboard/auth never calls the relay's State at all (askAuthState,
// authstatus.go), so there is no window, no link and no running state in
// scope here to leak. Driven against a relay that has one of each, rather
// than trusted to stay that thin because the response type has one field
// today.
//
// **Must fail when** the body carries the link, or says anything about a
// window being open.
func TestAuthStatusNeverCarriesTheSignInLink(t *testing.T) {
	t.Parallel()

	const link = "https://claude.ai/oauth/authorize?code_challenge=test-only-pkce-canary"

	f := newFleet(t)
	f.signin = &fakeRelay{
		signedIn: false,
		state: loginrelay.State{
			Running: true,
			Prompt:  &claudeauth.Prompt{URL: link},
		},
	}

	w := f.open(t, authPath)
	if strings.Contains(w.Body.String(), link) {
		t.Errorf("GET %s carries the sign-in link:\n%s", authPath, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "running") {
		t.Errorf("GET %s says something about a window's running state:\n%s", authPath, w.Body.String())
	}
}

// TestAuthStatusIsNoStore is D3: the header pill polls this every 60 seconds
// from every open tab, and a browser's own idea of "unchanged since last
// time" must never stand in for an ask this daemon has not made.
func TestAuthStatusIsNoStore(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	w := f.open(t, authPath)
	if got := w.Header().Get(headerCacheControl); got != cacheControlNoStore {
		t.Errorf("Cache-Control = %q; want %q", got, cacheControlNoStore)
	}
}

// TestAuthStatusRequiresIdentity is FR-006 at this route, the claim
// TestVersionRequiresIdentity makes at its neighbour: whether this host can
// still make a Claude request is exactly the fact a scanner would like for
// free, and this door gives it nothing.
//
// **Must fail when** the route is registered ahead of the door.
func TestAuthStatusRequiresIdentity(t *testing.T) {
	t.Parallel()

	f := newFleet(t)

	w := f.openWith(t, authPath, absent)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET %s with no assertion at all was answered %d (%s); want %d — the door's uniform refusal",
			authPath, w.Code, w.Body.String(), http.StatusUnauthorized)
	}

	elsewhere := f.openWith(t, "/", absent)
	if got, want := w.Body.String(), elsewhere.Body.String(); got != want {
		t.Errorf("an unverified caller was refused at %s with\n%s\nand at the fleet with\n%s\nthe two are distinguishable",
			authPath, got, want)
	}
}

// TestAuthStatusAndSignInViewAreGETOnly is TestNoMutatingVerbRegistered's
// shape at the two routes spec 015 adds: a method neither serves is a path
// nothing claims, never a 405 that would name the route table (FR-033).
func TestAuthStatusAndSignInViewAreGETOnly(t *testing.T) {
	t.Parallel()

	for _, path := range []string{authPath, signInViewPath} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			f := newFleet(t)
			w := f.ask(t, http.MethodPost, path)

			if w.Code == http.StatusMethodNotAllowed {
				t.Fatalf("POST %s was answered %d with %s: %q — which method a path serves is not a caller's to learn",
					path, w.Code, headerAllow, w.Header().Get(headerAllow))
			}
			if w.Code != http.StatusNotFound {
				t.Fatalf("POST %s was answered %d (%s); want %d — the unknown-route answer", path, w.Code, w.Body.String(), http.StatusNotFound)
			}
			if got := w.Header().Get(headerAllow); got != "" {
				t.Errorf("POST %s answered with %s: %q; want no such header", path, headerAllow, got)
			}
		})
	}
}

// --- The cache (D7) ---------------------------------------------------

// TestAuthCacheServesWithinTTL is D7's first half: two calls inside the TTL
// window share one exec.
//
// **Must fail when** the second call runs SignedIn again inside the window.
func TestAuthCacheServesWithinTTL(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	relay := &fakeRelay{signedIn: true}
	f.signin = relay
	f.clock = fixedClock{at: testTime}

	authAnswer(t, f.open(t, authPath))
	authAnswer(t, f.open(t, authPath))

	if got := relay.callCount(); got != 1 {
		t.Errorf("two calls inside the TTL ran SignedIn %d times; want 1", got)
	}
}

// TestAuthCacheAsksAgainAfterTTL is the other half: once the TTL has passed,
// the next call is a fresh exec.
func TestAuthCacheAsksAgainAfterTTL(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	relay := &fakeRelay{signedIn: true}
	f.signin = relay
	f.clock = fixedClock{at: testTime}

	authAnswer(t, f.open(t, authPath))

	f.clock = fixedClock{at: testTime.Add(authCacheTTL + time.Second)}
	authAnswer(t, f.open(t, authPath))

	if got := relay.callCount(); got != 2 {
		t.Errorf("two calls either side of the TTL ran SignedIn %d times; want 2", got)
	}
}

// TestAuthCacheSharesOneExecAmongConcurrentCallers is D7's "one in-flight
// exec shared by concurrent callers": the lock is held across the ask, the
// way releaseCache's is, so a caller that reaches it after the first has
// finished finds an answer already fresh rather than asking again.
//
// The clock is fixed for every caller — a real one would make this test's own
// timing decide whether two calls landed in the same instant — so what is
// being measured is the lock's serialisation and not a race this test would
// otherwise have to win.
func TestAuthCacheSharesOneExecAmongConcurrentCallers(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	relay := &fakeRelay{signedIn: true}
	f.signin = relay
	f.clock = fixedClock{at: testTime}

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			f.authStateCached(context.Background())
		}()
	}
	wg.Wait()

	if got := relay.callCount(); got != 1 {
		t.Errorf("%d concurrent callers ran SignedIn %d times; want 1 — the lock held across the ask is precisely what makes this one exec", n, got)
	}
}

// TestAuthCacheCachesCouldNotAskToo is D7's "a could-not-ask error is cached
// for the same TTL": an exec that fails is still an answer for the window's
// purposes, so a second caller inside it must not pay for a second attempt at
// something that just failed.
func TestAuthCacheCachesCouldNotAskToo(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	relay := &fakeRelay{err: errors.New("exec failed")}
	f.signin = relay
	f.clock = fixedClock{at: testTime}

	authAnswer(t, f.open(t, authPath))
	got := authAnswer(t, f.open(t, authPath))

	if got.State != authUnknown {
		t.Fatalf("a relay that could not be asked answered %q; want %q", got.State, authUnknown)
	}
	if calls := relay.callCount(); calls != 1 {
		t.Errorf("two calls inside the TTL, both against a relay that could not be asked, ran SignedIn %d times; want 1", calls)
	}
}

// TestTheThreeSignInPostsInvalidateTheAuthCache is D7's last clause: none of
// the three routes can be trusted to have left this host's credential exactly
// as the cache last recorded it, so each one clears it — proved by changing
// the fake relay's answer underneath the cache and confirming the very next
// ask reflects it rather than the stale one.
//
// **Must fail when** a POST to any of the three leaves a cached "ok" standing
// after the relay would now answer "bad", or the reverse.
func TestTheThreeSignInPostsInvalidateTheAuthCache(t *testing.T) {
	t.Parallel()

	for _, path := range []string{pathSignIn, pathSignInCode, pathSignInCancel} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			d := newSignInDoor(t)
			relay := &fakeRelay{signedIn: true}
			d.signin = relay
			d.clock = fixedClock{at: testTime}

			if got := authAnswer(t, d.get(t, authPath)); got.State != authOK {
				t.Fatalf("priming ask = %q; want %q", got.State, authOK)
			}
			if calls := relay.callCount(); calls != 1 {
				t.Fatalf("priming ask ran SignedIn %d times; want 1", calls)
			}

			d.post(t, path, d.form(t, url.Values{fieldConfirm: {confirmYes}, fieldCode: {"a-code"}}))

			// The credential changed underneath the cache; a cache that was
			// not invalidated would still answer the primed "ok" from
			// memory rather than asking again.
			relay.mu.Lock()
			relay.signedIn = false
			relay.mu.Unlock()

			got := authAnswer(t, d.get(t, authPath))
			if got.State != authBad {
				t.Errorf("after %s, GET %s = %q; want %q — the cache was not invalidated", path, authPath, got.State, authBad)
			}
			if calls := relay.callCount(); calls != 2 {
				t.Errorf("after %s, SignedIn ran %d times across two asks either side of it; want 2 — one cached hit survived the POST", path, calls)
			}
		})
	}
}
