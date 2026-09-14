// Internal test, matching the rest of the package. GET /dashboard/quota is the
// header's own route (spec 016), so most claims here drive it through the real
// router and the real browser door, the way authstatus_test.go drives its
// neighbour.
package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// quotaPath is derived from the pattern the server registers rather than
// spelled again here, for the reason authPath is: a renamed route would
// otherwise leave every test in this file passing against a path nothing
// claims.
var quotaPath = strings.TrimPrefix(patternDashboardQuota, http.MethodGet+" ")

// writeQuotaCache writes body to a fresh path under t.TempDir and returns it —
// never this host's real ~/.cache/quota-axi/quotas.json, so this suite cannot
// read or race a live quota-axi run on whatever machine executes it.
func writeQuotaCache(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "quotas.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the fixture cache: %v", err)
	}
	return path
}

// quotaCacheOK is the live shape from specs/016-weekly-quota-bar/spec.md's
// example, trimmed to what the reader looks at, with refreshedAt fresh
// relative to testTime so the ok-path tests below are not also, by accident,
// staleness tests.
const quotaCacheOK = `{"generatedAt":"2026-08-02T18:00:00Z","schemaVersion":2,"providers":[` +
	`{"provider":"claude","label":"Claude","source":"oauth",` +
	`"windows":[` +
	`{"id":"five_hour","kind":"session","percentUsed":4},` +
	`{"id":"seven_day","label":"week","kind":"weekly","percentUsed":64,"resetsAt":"2026-08-04T04:00:00.090669+00:00","windowSeconds":604800}` +
	`],` +
	`"state":{"status":"fresh","stale":false,"refreshedAt":"2026-08-02T19:00:00Z"}}]}`

// quotaAnswer decodes what the route said, and fails on a body that is not the
// documented object.
func quotaAnswer(t *testing.T, w *httptest.ResponseRecorder) quotaStatusResponse {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d (%s); want %d", quotaPath, w.Code, w.Body.String(), http.StatusOK)
	}

	var got quotaStatusResponse
	dec := json.NewDecoder(strings.NewReader(w.Body.String()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("GET %s answered %q, which is not a quota response: %v", quotaPath, w.Body.String(), err)
	}
	return got
}

// TestQuotaStatusRefusesRatherThanGuesses is spec 016 D4's whole list, driven
// through the real route: every one of these must answer "unknown" and carry
// none of the value fields — never a percentUsed of 0 standing in for "this
// daemon could not tell".
//
// **Must fail when** any case below answers "ok", or "unknown" with a
// percentUsed, resetsAt, refreshedAt or stale field present.
func TestQuotaStatusRefusesRatherThanGuesses(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"no cache file at all": filepath.Join(t.TempDir(), "does-not-exist", "quotas.json"),
		"not JSON at all":      writeQuotaCache(t, "not json"),
		"no claude provider": writeQuotaCache(t, `{"generatedAt":"2026-08-02T18:00:00Z","schemaVersion":2,"providers":[`+
			`{"provider":"codex","windows":[],"state":{"status":"fresh","stale":false}}]}`),
		"claude provider with no seven_day window": writeQuotaCache(t, `{"generatedAt":"2026-08-02T18:00:00Z","schemaVersion":2,"providers":[`+
			`{"provider":"claude","windows":[{"id":"five_hour","kind":"session","percentUsed":4}],`+
			`"state":{"status":"fresh","stale":false}}]}`),
		"percentUsed out of range": writeQuotaCache(t, `{"generatedAt":"2026-08-02T18:00:00Z","schemaVersion":2,"providers":[`+
			`{"provider":"claude","windows":[{"id":"seven_day","kind":"weekly","percentUsed":101}],`+
			`"state":{"status":"fresh","stale":false}}]}`),
	}

	for name, path := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFleet(t)
			f.quotaCachePath = path

			got := quotaAnswer(t, f.open(t, quotaPath))
			if got.State != quotaUnknown {
				t.Fatalf("state = %q; want %q", got.State, quotaUnknown)
			}
			if got.PercentUsed != nil || got.ResetsAt != nil || got.RefreshedAt != nil || got.Stale != nil {
				t.Errorf("an unknown reading carries a value field: %+v", got)
			}
		})
	}
}

// TestQuotaStatusReportsTheWeeklyWindow is the ok path: a cache this daemon
// can read answers with the claude provider's seven_day percentUsed and its
// own resetsAt and refreshedAt, verbatim.
//
// **Must fail when** the route reports a different window, drops a field, or
// reformats a timestamp quota-axi already wrote correctly.
func TestQuotaStatusReportsTheWeeklyWindow(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	f.quotaCachePath = writeQuotaCache(t, quotaCacheOK)
	f.clock = fixedClock{at: testTime}

	got := quotaAnswer(t, f.open(t, quotaPath))
	if got.State != quotaOK {
		t.Fatalf("state = %q; want %q", got.State, quotaOK)
	}
	if got.PercentUsed == nil || *got.PercentUsed != 64 {
		t.Errorf("percentUsed = %v; want 64", got.PercentUsed)
	}
	if got.ResetsAt == nil || *got.ResetsAt != "2026-08-04T04:00:00.090669+00:00" {
		t.Errorf("resetsAt = %v; want the window's own resetsAt verbatim", got.ResetsAt)
	}
	if got.RefreshedAt == nil || *got.RefreshedAt != "2026-08-02T19:00:00Z" {
		t.Errorf("refreshedAt = %v; want state.refreshedAt verbatim", got.RefreshedAt)
	}
}

// TestQuotaStatusIsStaleByQuotaAxisFlag is D4's first OR clause: quota-axi
// calling its own reading stale must reach the browser as stale even when the
// timestamp alone would read as fresh.
func TestQuotaStatusIsStaleByQuotaAxisFlag(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	f.clock = fixedClock{at: testTime}
	f.quotaCachePath = writeQuotaCache(t, `{"generatedAt":"2026-08-02T18:00:00Z","schemaVersion":2,"providers":[`+
		`{"provider":"claude","windows":[{"id":"seven_day","kind":"weekly","percentUsed":50}],`+
		`"state":{"status":"stale","stale":true,"refreshedAt":"`+testTime.Format(time.RFC3339)+`"}}]}`)

	got := quotaAnswer(t, f.open(t, quotaPath))
	if got.Stale == nil || !*got.Stale {
		t.Errorf("stale = %v; want true — quota-axi's own state.stale is true", got.Stale)
	}
}

// TestQuotaStatusIsStaleByAge is D4's other OR clause: a reading quota-axi
// still calls fresh is shown as stale once it is old enough by this daemon's
// own clock, so an operator is told a number might no longer be true even
// when the tool that produced it has not noticed.
//
// **Must fail when** a refreshedAt more than two hours old is reported fresh.
func TestQuotaStatusIsStaleByAge(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	f.clock = fixedClock{at: testTime}
	old := testTime.Add(-3 * time.Hour).Format(time.RFC3339)
	f.quotaCachePath = writeQuotaCache(t, `{"generatedAt":"2026-08-02T18:00:00Z","schemaVersion":2,"providers":[`+
		`{"provider":"claude","windows":[{"id":"seven_day","kind":"weekly","percentUsed":50}],`+
		`"state":{"status":"fresh","stale":false,"refreshedAt":"`+old+`"}}]}`)

	got := quotaAnswer(t, f.open(t, quotaPath))
	if got.Stale == nil || !*got.Stale {
		t.Errorf("stale = %v; want true — the reading is 3h old against a 2h bound, even though quota-axi calls it fresh", got.Stale)
	}
}

// TestQuotaStatusIsFreshWithinTheBound is the other side of the age check: a
// reading well inside two hours, and not flagged by quota-axi either, must not
// be shown as stale.
func TestQuotaStatusIsFreshWithinTheBound(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	f.clock = fixedClock{at: testTime}
	recent := testTime.Add(-30 * time.Minute).Format(time.RFC3339)
	f.quotaCachePath = writeQuotaCache(t, `{"generatedAt":"2026-08-02T18:00:00Z","schemaVersion":2,"providers":[`+
		`{"provider":"claude","windows":[{"id":"seven_day","kind":"weekly","percentUsed":50}],`+
		`"state":{"status":"fresh","stale":false,"refreshedAt":"`+recent+`"}}]}`)

	got := quotaAnswer(t, f.open(t, quotaPath))
	if got.Stale == nil || *got.Stale {
		t.Errorf("stale = %v; want false — 30 minutes old is well inside the two-hour bound", got.Stale)
	}
}

// TestQuotaStatusIsNoStore is the same rule authstatus.go's carries: the
// header polls this every 60 seconds from every open tab, and a browser's own
// idea of "unchanged since last time" must never stand in for a read this
// daemon has not made.
func TestQuotaStatusIsNoStore(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	w := f.open(t, quotaPath)
	if got := w.Header().Get(headerCacheControl); got != cacheControlNoStore {
		t.Errorf("Cache-Control = %q; want %q", got, cacheControlNoStore)
	}
}

// TestQuotaStatusRequiresIdentity is FR-006 at this route, the claim
// TestAuthStatusRequiresIdentity makes at its neighbour: total weekly usage is
// exactly the fact a scanner would like for free, and this door gives it
// nothing.
//
// **Must fail when** the route is registered ahead of the door.
func TestQuotaStatusRequiresIdentity(t *testing.T) {
	t.Parallel()

	f := newFleet(t)

	w := f.openWith(t, quotaPath, absent)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET %s with no assertion at all was answered %d (%s); want %d — the door's uniform refusal",
			quotaPath, w.Code, w.Body.String(), http.StatusUnauthorized)
	}

	elsewhere := f.openWith(t, "/", absent)
	if got, want := w.Body.String(), elsewhere.Body.String(); got != want {
		t.Errorf("an unverified caller was refused at %s with\n%s\nand at the fleet with\n%s\nthe two are distinguishable",
			quotaPath, got, want)
	}
}

// TestQuotaStatusIsGETOnly is TestNoMutatingVerbRegistered's shape at this
// route: a method it does not serve is a path nothing claims, never a 405
// that would name the route table (FR-033).
func TestQuotaStatusIsGETOnly(t *testing.T) {
	t.Parallel()

	f := newFleet(t)
	w := f.ask(t, http.MethodPost, quotaPath)

	if w.Code == http.StatusMethodNotAllowed {
		t.Fatalf("POST %s was answered %d with %s: %q — which method a path serves is not a caller's to learn",
			quotaPath, w.Code, headerAllow, w.Header().Get(headerAllow))
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("POST %s was answered %d (%s); want %d — the unknown-route answer", quotaPath, w.Code, w.Body.String(), http.StatusNotFound)
	}
	if got := w.Header().Get(headerAllow); got != "" {
		t.Errorf("POST %s answered with %s: %q; want no such header", quotaPath, headerAllow, got)
	}
}

// TestScriptFetchesQuotaOnTheSameTermsAsAuth is the static half of the poll:
// crswd.js cannot be executed by this suite (script(t)'s own comment), so what
// is checked is the bytes a browser is handed — the same route the header
// renders a placeholder for, fetched with credentials a cookie-based door
// needs and a cache setting that refuses a stale answer masquerading as one
// this daemon just gave.
//
// **Must fail when** the fetch drops same-origin credentials, allows a cached
// response, or is never made at all.
func TestScriptFetchesQuotaOnTheSameTermsAsAuth(t *testing.T) {
	t.Parallel()

	source := script(t)
	if !strings.Contains(source, "fetch('/dashboard/quota'") {
		t.Fatal("crswd.js does not fetch /dashboard/quota at all")
	}
	for _, want := range []string{"credentials: 'same-origin'", "cache: 'no-store'"} {
		if !strings.Contains(source, want) {
			t.Errorf("crswd.js's quota fetch does not carry %q", want)
		}
	}
}

// TestScriptRepaintsBothTheMeterAndTheLabel is FR-030 on the client side: a
// script that only touched textContent, or only the meter's value, would
// leave the two disagreeing the moment a real answer diverges from what the
// page was told to assume — which is exactly this daemon's own most-repeated
// mistake (docs, "unknown quota rendering as healthy").
func TestScriptRepaintsBothTheMeterAndTheLabel(t *testing.T) {
	t.Parallel()

	source := script(t)
	for _, want := range []string{
		"data-quota-meter",
		"data-quota-label",
		"meter.hidden = true",
		"meter.hidden = false",
		"quota-label-unknown",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("crswd.js does not carry %q; the meter and the label can drift", want)
		}
	}
}
