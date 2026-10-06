package httpapi

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/access"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

const codexPill = `data-auth-pill data-harness="codex"`

// pageHeader is the masthead of a page the daemon served, failing rather than
// returning an empty string, which every Contains assertion would pass on.
func pageHeader(t *testing.T, d *signInDoor, target string) string {
	t.Helper()

	w := d.get(t, target)
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Fatalf("GET %s = %d; want a rendered page", target, w.Code)
	}
	return mastheadOf(t, target, w.Body.String())
}

// codexDoor is a signInDoor whose daemon offers Codex, which is what puts a
// relay in signins[harness.Codex] and so turns the header's second pill on.
func codexDoor(t *testing.T) *signInDoor {
	t.Helper()

	d := newSignInDoor(t)
	d.offersCodex("codex --no-alt-screen")
	d.signins[harness.Codex] = &fakeRelay{signedIn: true}
	d.authCaches[harness.Codex] = &authCache{}
	return d
}

// **Must fail when** a page template still hands the header the bare operator,
// or a construction site forgets to set Header, so the address disappears.
func TestHeaderViewOnEveryPage(t *testing.T) {
	t.Parallel()

	d := newSignInDoor(t)
	for _, target := range []string{"/", "/settings", "/sessions/no-such-session/view"} {
		masthead := pageHeader(t, d, target)
		if !strings.Contains(masthead, testOperatorEmail) {
			t.Errorf("GET %s renders no operator address in its header:\n%s", target, masthead)
		}
	}

	if got := renderComponent(t, "not-found", notFoundView{
		Operator: &access.VerifiedOperator{Email: testOperatorEmail},
		Header:   headerView{Operator: &access.VerifiedOperator{Email: testOperatorEmail}},
	}); !strings.Contains(got, testOperatorEmail) {
		t.Errorf("the not-found page renders no operator address:\n%s", got)
	}
}

// **Must fail when** a Codex entry is configured and the header draws no Codex
// pill, or draws it as a link, or without the harness it polls for.
func TestHeaderCodexPillWhenConfigured(t *testing.T) {
	t.Parallel()

	masthead := pageHeader(t, codexDoor(t), "/")
	if strings.Count(masthead, codexPill) != 1 {
		t.Fatalf("the header carries %d Codex pills; want one:\n%s", strings.Count(masthead, codexPill), masthead)
	}
	if !strings.Contains(masthead, "codex auth: checking") {
		t.Errorf("the Codex pill does not start on the word checking:\n%s", masthead)
	}
	if strings.Count(masthead, "data-auth-pill") != 2 {
		t.Errorf("the header carries %d auth pills; want Claude's and Codex's", strings.Count(masthead, "data-auth-pill"))
	}
	if got := len(cardAnchor.FindAllStringSubmatch(masthead, -1)); got != 2 {
		t.Errorf("the header renders %d links with the Codex pill present; the pill is a button:\n%s", got, masthead)
	}
}

// **Must fail when** a daemon with no Codex entry draws anything new in its
// header, which must stay byte-identical to what it was before Codex existed.
func TestHeaderUnchangedWithoutCodex(t *testing.T) {
	t.Parallel()

	masthead := pageHeader(t, newSignInDoor(t), "/")
	if strings.Contains(masthead, codexPill) || strings.Contains(strings.ToLower(masthead), "codex") {
		t.Errorf("the header mentions Codex with no Codex entry configured:\n%s", masthead)
	}

	op := &access.VerifiedOperator{Email: "operator@example.com"}
	without := renderComponent(t, "header", headerView{Operator: op})
	with := renderComponent(t, "header", headerView{Operator: op, CodexConfigured: true})
	if without == with {
		t.Fatal("CodexConfigured changes nothing, so the two renders prove nothing about each other")
	}
	if strings.Contains(without, codexPill) {
		t.Errorf("a header with CodexConfigured false drew the Codex pill:\n%s", without)
	}

	// The golden is origin/main's header.html rendered by this same helper
	// before the Codex pill existed, so any stray byte (a lost newline) shows.
	golden, err := os.ReadFile("testdata/header_no_codex.golden.html")
	if err != nil {
		t.Fatalf("read the golden header: %v", err)
	}
	if without != string(golden) {
		t.Errorf("the no-Codex header differs from the pre-Codex render\n got: %q\nwant: %q", without, string(golden))
	}
}

// **Must fail when** headerFor reads anything but the Codex relay, or invents a
// second operator for the header.
func TestHeaderForReadsTheCodexRelay(t *testing.T) {
	t.Parallel()

	op := &access.VerifiedOperator{Email: "operator@example.com"}

	plain := newSignInDoor(t)
	if got := plain.headerFor(op); got.Operator != op || got.CodexConfigured {
		t.Errorf("headerFor with no Codex relay = %+v; want the same operator and Codex off", got)
	}
	if got := codexDoor(t).headerFor(op); got.Operator != op || !got.CodexConfigured {
		t.Errorf("headerFor with a Codex relay = %+v; want the same operator and Codex on", got)
	}
}

// **Must fail when** the served script loses a per-pill state record, the dialog
// generation counter, the abort of an in-flight view fetch, or the Codex marker.
func TestAuthScriptPerPillState(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../web/static/crswd.js")
	if err != nil {
		t.Fatalf("read the script: %v", err)
	}
	script := string(raw)
	for _, want := range []string{"activeHarness", "dialogGeneration", "AbortController", "signin=codex", "marker === 'codex'", "/dashboard/auth?harness=", "/dashboard/signin/view?harness=", "data-harness"} {
		if !strings.Contains(script, want) {
			t.Errorf("crswd.js does not contain %q", want)
		}
	}
}
