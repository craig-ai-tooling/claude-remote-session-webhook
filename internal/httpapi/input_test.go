package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/auth"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

// A nil bucket would panic on the first typed message, and nothing else in the
// default build would notice until a route called it.
func TestTheInputBudgetIsBuilt(t *testing.T) {
	t.Parallel()

	if newTestServer(t, loopbackListen).inputs == nil {
		t.Fatal("newServer built no input limiter")
	}
}

// typer is the registered type route with the store, the fake host and the trail
// behind it readable. It borrows the compactor's fixture because what it needs is
// the same: an audited server, a signing key and a planted session.
type typer struct {
	*compactor
}

func newTyper(t *testing.T) *typer {
	t.Helper()

	// The fixture's own body cap is far below what a 16 KiB message needs, so the
	// gate would refuse the too-long case before the handler saw it. The
	// production default is what makes that case reach the handler.
	keys := newKeyServer(t)
	cfg := testConfig(loopbackListen)
	cfg.MaxBodyBytes = config.DefaultMaxBodyBytes
	ts := newAuditedServerOn(t, cfg, keys.validator(t))
	// Pinned for the reason newTestServer pins the create bucket: on the host
	// clock the bucket refills two tokens a second, so a budget test that takes
	// longer than half a second to spend 120 finds one back and is admitted.
	inputs, err := newLimiter[auth.CallerID]("input", inputRatePerMin, fixedClock{at: testTime})
	if err != nil {
		t.Fatalf("newLimiter(input) = _, %v; want a limiter", err)
	}
	ts.inputs = inputs
	return &typer{compactor: &compactor{testServer: ts, keys: keys}}
}

// typed posts one form at the type route as the browser this daemon rendered the
// page for.
func (ty *typer) typed(t *testing.T, id, text, enter string) *httptest.ResponseRecorder {
	t.Helper()

	form := ty.asked(t)
	form.Set(fieldText, text)
	if enter != absent {
		form.Set(fieldEnter, enter)
	}
	return ty.send(t, http.MethodPost, "/dashboard/sessions/"+id+"/type", secFetchSiteSameOrigin, form)
}

// ops is the operations the host was asked for, in order.
func (ty *typer) ops() []tmuxctl.Op {
	var ops []tmuxctl.Op
	for _, call := range ty.fixture.tmux.Calls() {
		switch call.Op {
		case tmuxctl.OpPasteBracketed, tmuxctl.OpSendKeys:
			ops = append(ops, call.Op)
		}
	}
	return ops
}

// reachedTheHost is whether any delivery operation was asked of tmux at all.
func (ty *typer) reachedTheHost() bool {
	return len(ty.ops()) > 0
}

func TestTypeDeliversAndAnswersNoContent(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)

	w := ty.typed(t, live.ID, "hello", confirmYes)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
	}
	if got := w.Body.String(); got != "" {
		t.Errorf("body = %q; want none", got)
	}
	want := []tmuxctl.Op{tmuxctl.OpPasteBracketed, tmuxctl.OpPasteBracketed, tmuxctl.OpSendKeys}
	got := ty.ops()
	if len(got) != len(want) {
		t.Fatalf("host operations = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("host operations = %v; want %v", got, want)
		}
	}
}

func TestTypeWithoutEnterPressesNothing(t *testing.T) {
	t.Parallel()

	// Absent, empty, and a near miss: only the exact word yes presses Enter.
	for _, enter := range []string{absent, "", "YES", "true", "1"} {
		t.Run("enter="+enter, func(t *testing.T) {
			t.Parallel()

			ty := newTyper(t)
			live := ty.live(t)

			w := ty.typed(t, live.ID, "hello", enter)

			if w.Code != http.StatusNoContent {
				t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
			}
			for _, op := range ty.ops() {
				if op == tmuxctl.OpSendKeys {
					t.Fatalf("enter=%q pressed a key; only enter=yes may", enter)
				}
			}
			if !ty.reachedTheHost() {
				t.Fatal("the text never reached the host")
			}
		})
	}
}

func TestTypeNormalisesCRLF(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)

	w := ty.typed(t, live.ID, "a\r\nb", absent)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
	}
	var payloads []string
	for _, call := range ty.fixture.tmux.Calls() {
		if len(call.Stdin) > 0 {
			payloads = append(payloads, string(call.Stdin))
		}
	}
	if len(payloads) != 1 || payloads[0] != "a\nb" {
		t.Fatalf("payloads = %q; want [%q]", payloads, "a\nb")
	}
}

func TestTypeRefusesBadTextWithItsOutcome(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		name string
		text string
		want outcome
	}{
		{"empty", "", outcomeTypeEmpty},
		{"too long", strings.Repeat("a", session.MaxTypeBytes+1), outcomeTypeTooLong},
		{"escape", "\x1b", outcomeTypeInvalid},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			ty := newTyper(t)
			live := ty.live(t)
			before := len(ty.fixture.tmux.Calls())

			w := ty.typed(t, live.ID, c.text, confirmYes)

			wantOutcome(t, w, c.want)
			if after := len(ty.fixture.tmux.Calls()); after != before {
				t.Fatalf("a refused text made %d tmux calls; want none", after-before)
			}
		})
	}
}

// typeRoute is the type route in the shape refusalShapes drives. It is not added
// to mutatingRoutes(), whose success is a 303 and whose callers assume it: this
// route's success is a 204 (research R4).
func typeRoute() mutatingRoute {
	return mutatingRoute{
		name:          patternDashboardType,
		path:          func(id string) string { return "/dashboard/sessions/" + id + "/type" },
		namesASession: true,
		fields: func(t *testing.T, _ *refuser) url.Values {
			t.Helper()

			form := url.Values{}
			form.Set(fieldText, "hello")
			form.Set(fieldEnter, confirmYes)
			return form
		},
	}
}

func TestTypeRefusesLikeEveryAction(t *testing.T) {
	t.Parallel()

	route := typeRoute()
	for _, c := range refusalShapes() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			r := newRefuser(t)
			w := r.send(t, route, c.vary(t, r, r.wellFormed(t, r.mine(t).ID)))

			if w.Code >= http.StatusMultipleChoices && w.Code < http.StatusBadRequest {
				t.Fatalf("the refusal was answered %d to %q; a refusal is never a redirect",
					w.Code, w.Header().Get(headerLocation))
			}
			if got := w.Header().Get(headerLocation); got != "" {
				t.Errorf("the refusal carried %s: %q; want none", headerLocation, got)
			}
			if w.Code != c.status {
				t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), c.status)
			}
			if got := w.Body.String(); got != c.body {
				t.Errorf("body\n%s\nwant\n%s", got, c.body)
			}
			for _, call := range r.fixture.tmux.Calls() {
				if call.Op == tmuxctl.OpPasteBracketed || call.Op == tmuxctl.OpSendKeys {
					t.Fatalf("a refused request reached the host: %s", call.Op)
				}
			}
		})
	}

	// Non-vacuity: every row above is satisfied by a route that refuses everything.
	t.Run("a request nothing refuses is answered 204", func(t *testing.T) {
		t.Parallel()

		r := newRefuser(t)
		w := r.send(t, route, r.wellFormed(t, r.mine(t).ID))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
		}
	})
}

// The bucket holds burstFor(240) = 120, so the 121st back-to-back post is the
// first one refused. The refused one must deliver nothing.
func TestTypeSpendsTheInputBudget(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)

	for i := 1; i <= 120; i++ {
		if w := ty.typed(t, live.ID, "x", absent); w.Code != http.StatusNoContent {
			t.Fatalf("post %d = %d (%s); want %d", i, w.Code, w.Body.String(), http.StatusNoContent)
		}
	}
	delivered := len(ty.ops())

	w := ty.typed(t, live.ID, "x", absent)

	wantOutcome(t, w, outcomeInputLimited)
	if got := len(ty.ops()); got != delivered {
		t.Fatalf("the refused post made %d host calls; want none", got-delivered)
	}
}

// pressed posts one form at the key route as the browser this daemon rendered the
// page for.
func (ty *typer) pressed(t *testing.T, id, key string) *httptest.ResponseRecorder {
	t.Helper()

	form := ty.asked(t)
	if key != absent {
		form.Set(fieldKey, key)
	}
	return ty.send(t, http.MethodPost, "/dashboard/sessions/"+id+"/key", secFetchSiteSameOrigin, form)
}

func TestKeySendsEachAllowlistedKey(t *testing.T) {
	t.Parallel()

	for _, k := range session.Keys() {
		t.Run(string(k), func(t *testing.T) {
			t.Parallel()

			ty := newTyper(t)
			live := ty.live(t)

			w := ty.pressed(t, live.ID, string(k))

			if w.Code != http.StatusNoContent {
				t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
			}
			if got := w.Body.String(); got != "" {
				t.Errorf("body = %q; want none", got)
			}
			if ops := ty.ops(); len(ops) != 1 || ops[0] != tmuxctl.OpSendKeys {
				t.Fatalf("host operations = %v; want one SendKeys", ops)
			}
		})
	}
}

func TestKeyRefusesAnythingElse(t *testing.T) {
	t.Parallel()

	for _, key := range []string{"Escape", "C-c", "q", "", "enter;"} {
		t.Run("key="+key, func(t *testing.T) {
			t.Parallel()

			ty := newTyper(t)
			live := ty.live(t)

			w := ty.pressed(t, live.ID, key)

			wantOutcome(t, w, outcomeKeyUnknown)
			if ty.reachedTheHost() {
				t.Fatal("a refused key reached the host")
			}
		})
	}
}

// keyRoute is the key route in the shape refusalShapes drives; like typeRoute it
// stays out of mutatingRoutes() because its success is a 204.
func keyRoute() mutatingRoute {
	return mutatingRoute{
		name:          patternDashboardKey,
		path:          func(id string) string { return "/dashboard/sessions/" + id + "/key" },
		namesASession: true,
		fields: func(t *testing.T, _ *refuser) url.Values {
			t.Helper()

			form := url.Values{}
			form.Set(fieldKey, string(session.KeyEscape))
			return form
		},
	}
}

func TestKeyRefusesLikeEveryAction(t *testing.T) {
	t.Parallel()

	route := keyRoute()
	for _, c := range refusalShapes() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			r := newRefuser(t)
			w := r.send(t, route, c.vary(t, r, r.wellFormed(t, r.mine(t).ID)))

			if w.Code >= http.StatusMultipleChoices && w.Code < http.StatusBadRequest {
				t.Fatalf("the refusal was answered %d to %q; a refusal is never a redirect",
					w.Code, w.Header().Get(headerLocation))
			}
			if got := w.Header().Get(headerLocation); got != "" {
				t.Errorf("the refusal carried %s: %q; want none", headerLocation, got)
			}
			if w.Code != c.status {
				t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), c.status)
			}
			if got := w.Body.String(); got != c.body {
				t.Errorf("body\n%s\nwant\n%s", got, c.body)
			}
			for _, call := range r.fixture.tmux.Calls() {
				if call.Op == tmuxctl.OpPasteBracketed || call.Op == tmuxctl.OpSendKeys {
					t.Fatalf("a refused request reached the host: %s", call.Op)
				}
			}
		})
	}

	t.Run("a request nothing refuses is answered 204", func(t *testing.T) {
		t.Parallel()

		r := newRefuser(t)
		w := r.send(t, route, r.wellFormed(t, r.mine(t).ID))
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
		}
	})
}

func TestKeyAndTypeShareOneBudget(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)

	for i := 1; i <= 120; i++ {
		if w := ty.pressed(t, live.ID, string(session.KeyTab)); w.Code != http.StatusNoContent {
			t.Fatalf("key %d = %d (%s); want %d", i, w.Code, w.Body.String(), http.StatusNoContent)
		}
	}
	delivered := len(ty.ops())

	w := ty.typed(t, live.ID, "x", absent)

	wantOutcome(t, w, outcomeInputLimited)
	if got := len(ty.ops()); got != delivered {
		t.Fatalf("the refused type made %d host calls; want none", got-delivered)
	}
}

func TestKeyAuditsNoKeyName(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)

	w := ty.pressed(t, live.ID, string(session.KeyEscape))
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
	}

	rec := ty.only(t)
	if got, want := rec["action"], string(audit.ActionDashboardKey); got != want {
		t.Errorf("action = %v; want %v", got, want)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("re-encode the audit record %v: %v", rec, err)
	}
	// Both spellings, folded: the symbolic name the browser posts and the tmux
	// name it maps to ("Escape"). Checking one lowercase string would pass a
	// record that carried the other.
	lower := strings.ToLower(string(raw))
	for _, name := range []string{string(session.KeyEscape), "Escape"} {
		if strings.Contains(lower, strings.ToLower(name)) {
			t.Errorf("the record carries the key name %q: %s", name, raw)
		}
	}
}

func TestTypeAuditsNoText(t *testing.T) {
	t.Parallel()

	const canary = "canary-text-7f3a91"
	ty := newTyper(t)
	live := ty.live(t)

	w := ty.typed(t, live.ID, canary, confirmYes)
	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNoContent)
	}

	rec := ty.only(t)
	if got, want := rec["action"], string(audit.ActionDashboardType); got != want {
		t.Errorf("action = %v; want %v", got, want)
	}
	if got, want := rec["session_id"], live.ID; got != want {
		t.Errorf("session_id = %v; want %v", got, want)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("re-encode the audit record %v: %v", rec, err)
	}
	if strings.Contains(string(raw), canary) {
		t.Errorf("the record carries the typed text: %s", raw)
	}
}

// read gets the history route as the browser this daemon rendered the page for.
func (ty *typer) read(t *testing.T, id, site string) *httptest.ResponseRecorder {
	t.Helper()

	return ty.send(t, http.MethodGet, "/sessions/"+id+"/history", site, url.Values{})
}

func TestHistoryAnswersStrippedText(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)
	ty.fixture.tmux.SetHistory(live.TmuxName(), "\x1b[31mred\x1b[0m line\x1b]0;title\x07\nplain\n")

	w := ty.read(t, live.ID, secFetchSiteSameOrigin)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusOK)
	}
	if got, want := w.Body.String(), "red line\nplain\n"; got != want {
		t.Errorf("body = %q; want %q", got, want)
	}
	if got, want := w.Header().Get(headerContentType), contentTypePlain; got != want {
		t.Errorf("%s = %q; want %q", headerContentType, got, want)
	}
	if got, want := w.Header().Get(headerCacheControl), cacheControlNoStore; got != want {
		t.Errorf("%s = %q; want %q", headerCacheControl, got, want)
	}
}

func TestHistoryAllowsAnAbsentFetchSite(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)

	if w := ty.read(t, live.ID, absent); w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusOK)
	}
}

func TestHistoryRefusesCrossSite(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)
	ty.fixture.tmux.SetHistory(live.TmuxName(), "scrollback-canary")

	w := ty.read(t, live.ID, "cross-site")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusUnauthorized)
	}
	if strings.Contains(w.Body.String(), "scrollback-canary") {
		t.Errorf("a cross-site read was answered with history: %s", w.Body.String())
	}
	if ty.askedForHistory() {
		t.Error("a cross-site read reached the host")
	}
	rec := ty.only(t)
	if got, want := rec["reason"], errHistoryCrossSite.Error(); got != want {
		t.Errorf("reason = %v; want %v", got, want)
	}
}

func TestHistoryIsOwnerScoped(t *testing.T) {
	t.Parallel()

	const stranger auth.CallerID = "a-second-operator"
	ty := newTyper(t)
	theirs, _ := ty.fixture.plant(t, session.Session{Owner: stranger, Name: originalName, WorkDir: ty.fixture.repo})
	ty.fixture.tmux.SetHistory(theirs.TmuxName(), "not-yours-canary")

	w := ty.read(t, theirs.ID, secFetchSiteSameOrigin)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNotFound)
	}
	// An id that never existed is the baseline: a session someone else owns must
	// be indistinguishable from it.
	never := ty.read(t, strings.Repeat("c", session.IDLen), secFetchSiteSameOrigin)
	if never.Code != w.Code || never.Body.String() != w.Body.String() {
		t.Errorf("another operator's session answered %d (%d bytes); an unknown id answered %d (%d bytes)",
			w.Code, w.Body.Len(), never.Code, never.Body.Len())
	}
	if strings.Contains(w.Body.String(), "not-yours-canary") {
		t.Error("another operator's history was served")
	}
	if ty.askedForHistory() {
		t.Error("another operator's session reached the host")
	}
	// The unknown-id read adds a second record, so the owner-scoped one is first.
	rec := ty.records(t)[0]
	if got, want := rec["reason"], session.ErrSessionNotFound.Error(); got != want {
		t.Errorf("reason = %v; want %v", got, want)
	}
}

func TestHistoryBoundIsA500(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)
	ty.fixture.tmux.FailOp(tmuxctl.OpCaptureHistory, tmuxctl.ErrHistoryTooLarge)

	w := ty.read(t, live.ID, secFetchSiteSameOrigin)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusInternalServerError)
	}
	if got := w.Body.String(); got != "" {
		t.Errorf("body = %q; want none", got)
	}
	rec := ty.only(t)
	if got, want := rec["reason"], errHistoryUnreadable.Error(); got != want {
		t.Errorf("reason = %v; want %v", got, want)
	}
}

func TestHistoryAuditsNoContent(t *testing.T) {
	t.Parallel()

	const canary = "canary-history-5c1e44"
	ty := newTyper(t)
	live := ty.live(t)
	ty.fixture.tmux.SetHistory(live.TmuxName(), canary)

	w := ty.read(t, live.ID, secFetchSiteSameOrigin)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusOK)
	}

	rec := ty.only(t)
	if got, want := rec["action"], string(audit.ActionDashboardHistory); got != want {
		t.Errorf("action = %v; want %v", got, want)
	}
	if got, want := rec["session_id"], live.ID; got != want {
		t.Errorf("session_id = %v; want %v", got, want)
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("re-encode the audit record %v: %v", rec, err)
	}
	if strings.Contains(string(raw), canary) {
		t.Errorf("the record carries the history: %s", raw)
	}
}

// askedForHistory is whether the host was ever asked for a scrollback.
func (ty *typer) askedForHistory() bool {
	for _, call := range ty.fixture.tmux.Calls() {
		if call.Op == tmuxctl.OpCaptureHistory {
			return true
		}
	}
	return false
}

// An ID that is not routable still costs a token, on both routes, so a flood of
// malformed requests cannot be had for free (FR-009, spec 018 review #8).
func TestMalformedIDsSpendTheInputBudget(t *testing.T) {
	t.Parallel()

	ty := newTyper(t)
	live := ty.live(t)

	for i := 1; i <= 60; i++ {
		if w := ty.typed(t, "not-a-session-id", "x", absent); w.Code != http.StatusNotFound {
			t.Fatalf("malformed type %d = %d (%s); want %d", i, w.Code, w.Body.String(), http.StatusNotFound)
		}
		if w := ty.pressed(t, "not-a-session-id", string(session.KeyTab)); w.Code != http.StatusNotFound {
			t.Fatalf("malformed key %d = %d (%s); want %d", i, w.Code, w.Body.String(), http.StatusNotFound)
		}
	}

	wantOutcome(t, ty.typed(t, live.ID, "x", absent), outcomeInputLimited)
	wantOutcome(t, ty.pressed(t, live.ID, string(session.KeyTab)), outcomeInputLimited)
	if ty.reachedTheHost() {
		t.Error("a request refused for budget reached the host")
	}
}

// The paste landed and the Enter did not. The answer says so, and it is not the
// outcome that says nothing was delivered (spec 018 review #4).
func TestTypeWhoseEnterFailedSaysTheTextWasTyped(t *testing.T) {
	t.Parallel()

	const canary = "canary-unsubmitted-2b8d"
	ty := newTyper(t)
	live := ty.live(t)
	ty.fixture.tmux.FailOp(tmuxctl.OpSendKeys, tmuxctl.ErrNoSocket)

	w := ty.typed(t, live.ID, canary, confirmYes)

	wantOutcome(t, w, outcomeTypeUnsubmitted)
	if view := bannerFor(string(outcomeTypeUnsubmitted)); view == nil || !strings.Contains(view.Message, "typed but not submitted") {
		t.Errorf("the banner for type-unsubmitted = %v; want one that says the text was typed but not submitted", view)
	}
	raw, err := json.Marshal(ty.only(t))
	if err != nil {
		t.Fatalf("re-encode the audit record: %v", err)
	}
	if strings.Contains(string(raw), canary) {
		t.Errorf("the record carries the typed text: %s", raw)
	}
}

// View said live and tmux had already lost the window: every input route
// answers the uniform not-found, not a generic failure or a 500 (review #3).
func TestInputToAVanishedWindowIsTheUniformNotFound(t *testing.T) {
	t.Parallel()

	for name, drive := range map[string]func(*testing.T, *typer, string) *httptest.ResponseRecorder{
		"type": func(t *testing.T, ty *typer, id string) *httptest.ResponseRecorder {
			return ty.typed(t, id, "hello", confirmYes)
		},
		"key": func(t *testing.T, ty *typer, id string) *httptest.ResponseRecorder {
			return ty.pressed(t, id, string(session.KeyTab))
		},
		"history": func(t *testing.T, ty *typer, id string) *httptest.ResponseRecorder {
			return ty.read(t, id, secFetchSiteSameOrigin)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ty := newTyper(t)
			live := ty.live(t)
			// The record stays in the store and the window is gone, which is the
			// state between View and the delivery.
			if err := ty.fixture.tmux.Kill(t.Context(), live.TmuxName()); err != nil {
				t.Fatalf("Kill: %v", err)
			}

			w := drive(t, ty, live.ID)

			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d (%s); want %d", w.Code, w.Body.String(), http.StatusNotFound)
			}
		})
	}
}
