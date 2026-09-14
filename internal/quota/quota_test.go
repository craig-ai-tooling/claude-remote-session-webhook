// External test (package quota_test): the two callers this package has —
// httpapi's quota route and a test standing in for quota-axi itself — see
// exactly what this file sees.
package quota_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/quota"
)

// cacheWith writes fixture bytes to a fresh path under t.TempDir and returns
// it — never the operator's real cache, so this suite cannot read or race a
// live quota-axi run on the host it happens to execute on.
func cacheWith(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "quotas.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the fixture cache: %v", err)
	}
	return path
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// withSevenDay is the live example from specs/016, trimmed to what Read
// actually looks at: the claude provider's seven_day window plus its state.
func withSevenDay(percentUsed, resetsAt, refreshedAt string, stale bool) string {
	percentField := ""
	if percentUsed != "" {
		percentField = `,"percentUsed":` + percentUsed
	}
	resetsField := ""
	if resetsAt != "" {
		resetsField = `,"resetsAt":"` + resetsAt + `"`
	}
	return `{"generatedAt":"2026-09-14T18:00:00.000Z","schemaVersion":2,"providers":[` +
		`{"provider":"claude","label":"Claude","source":"oauth",` +
		`"windows":[` +
		`{"id":"five_hour","kind":"session","percentUsed":4},` +
		`{"id":"seven_day","label":"week","kind":"weekly"` + percentField + resetsField + `,"windowSeconds":604800}` +
		`],` +
		`"state":{"status":"fresh","stale":` + boolStr(stale) + `,"refreshedAt":"` + refreshedAt + `"}}]}`
}

// TestReadRefusesRatherThanGuesses is spec 016 D4's whole list: every one of
// these must come back as one of this package's sentinels, never a Reading
// that looks like a real 0%.
//
// **Must fail when** any case below returns a nil error, or an error that is
// not the one sentinel that names what actually went wrong.
func TestReadRefusesRatherThanGuesses(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path string
		want error
	}{
		"no file at all": {
			path: filepath.Join(t.TempDir(), "does-not-exist", "quotas.json"),
			want: quota.ErrNoCacheFile,
		},
		"unreadable": {
			// A directory where Read expects a file: os.ReadFile refuses it
			// with something other than ENOENT, which is exactly the case
			// ErrUnreadable exists for.
			path: t.TempDir(),
			want: quota.ErrUnreadable,
		},
		"not JSON at all": {
			path: cacheWith(t, "not json"),
			want: quota.ErrMalformed,
		},
		"JSON but not an object": {
			path: cacheWith(t, `[1,2,3]`),
			want: quota.ErrMalformed,
		},
		"no claude provider": {
			path: cacheWith(t, `{"generatedAt":"2026-09-14T18:00:00Z","schemaVersion":2,"providers":[`+
				`{"provider":"codex","windows":[],"state":{"status":"fresh","stale":false}}]}`),
			want: quota.ErrNoClaudeProvider,
		},
		"no providers at all": {
			path: cacheWith(t, `{"generatedAt":"2026-09-14T18:00:00Z","schemaVersion":2,"providers":[]}`),
			want: quota.ErrNoClaudeProvider,
		},
		"claude provider with no seven_day window": {
			path: cacheWith(t, `{"generatedAt":"2026-09-14T18:00:00Z","schemaVersion":2,"providers":[`+
				`{"provider":"claude","windows":[{"id":"five_hour","kind":"session","percentUsed":4}],`+
				`"state":{"status":"fresh","stale":false}}]}`),
			want: quota.ErrNoWeeklyWindow,
		},
		"percentUsed missing": {
			path: cacheWith(t, withSevenDay("", "2026-09-16T04:00:00Z", "2026-09-14T18:00:00Z", false)),
			want: quota.ErrInvalidPercent,
		},
		"percentUsed negative": {
			path: cacheWith(t, withSevenDay("-1", "2026-09-16T04:00:00Z", "2026-09-14T18:00:00Z", false)),
			want: quota.ErrInvalidPercent,
		},
		"percentUsed over 100": {
			path: cacheWith(t, withSevenDay("101", "2026-09-16T04:00:00Z", "2026-09-14T18:00:00Z", false)),
			want: quota.ErrInvalidPercent,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := quota.Read(tc.path)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Read(%q) error = %v; want %v", tc.path, err, tc.want)
			}
		})
	}
}

// TestReadExtractsTheWeeklyWindow is the live shape from specs/016's example,
// trimmed to the fields Read reads. It must succeed and report exactly what
// the fixture says — no rounding surprises, no substituted fallback in play.
func TestReadExtractsTheWeeklyWindow(t *testing.T) {
	t.Parallel()

	path := cacheWith(t, withSevenDay("64", "2026-09-16T04:00:00.090669+00:00", "2026-09-14T18:51:48.198Z", false))

	got, err := quota.Read(path)
	if err != nil {
		t.Fatalf("Read(%q) = _, %v; want a reading", path, err)
	}
	if got.PercentUsed != 64 {
		t.Errorf("PercentUsed = %d; want 64", got.PercentUsed)
	}
	if got.ResetsAt != "2026-09-16T04:00:00.090669+00:00" {
		t.Errorf("ResetsAt = %q; want the window's own resetsAt verbatim", got.ResetsAt)
	}
	if got.RefreshedAtRaw != "2026-09-14T18:51:48.198Z" {
		t.Errorf("RefreshedAtRaw = %q; want state.refreshedAt", got.RefreshedAtRaw)
	}
	want, err := time.Parse(time.RFC3339, "2026-09-14T18:51:48.198Z")
	if err != nil {
		t.Fatalf("parse the fixture's own timestamp: %v", err)
	}
	if !got.RefreshedAt.Equal(want) {
		t.Errorf("RefreshedAt = %v; want %v", got.RefreshedAt, want)
	}
	if got.Stale {
		t.Errorf("Stale = true; the fixture's state.stale is false")
	}
}

// TestReadRoundsAFractionalPercent covers a percentUsed quota-axi never
// actually emits today (its own windows are whole numbers) but the schema
// does not forbid: a value this close to a threshold must land on one side of
// it consistently rather than truncating toward whichever the caller's
// integer conversion happened to prefer.
func TestReadRoundsAFractionalPercent(t *testing.T) {
	t.Parallel()

	path := cacheWith(t, withSevenDay("74.6", "2026-09-16T04:00:00Z", "2026-09-14T18:00:00Z", false))

	got, err := quota.Read(path)
	if err != nil {
		t.Fatalf("Read(%q) = _, %v; want a reading", path, err)
	}
	if got.PercentUsed != 75 {
		t.Errorf("PercentUsed = %d; want 75 (74.6 rounds up)", got.PercentUsed)
	}
}

// TestReadFallsBackToGeneratedAt is D4's fallback clause: a provider whose own
// state carries no refreshedAt is not a reading with no age at all — the
// document's own generatedAt is the daemon's best remaining answer to "as of
// when".
func TestReadFallsBackToGeneratedAt(t *testing.T) {
	t.Parallel()

	path := cacheWith(t, withSevenDay("10", "2026-09-16T04:00:00Z", "", false))

	got, err := quota.Read(path)
	if err != nil {
		t.Fatalf("Read(%q) = _, %v; want a reading", path, err)
	}
	if got.RefreshedAtRaw != "2026-09-14T18:00:00.000Z" {
		t.Errorf("RefreshedAtRaw = %q; want the document's generatedAt when state.refreshedAt is absent", got.RefreshedAtRaw)
	}
}

// TestReadCarriesQuotaAxisOwnStaleFlag is the other half of D4's OR: quota-axi
// calling its own reading stale must survive the trip through Reading
// unchanged, whatever the age math on top of it later decides.
func TestReadCarriesQuotaAxisOwnStaleFlag(t *testing.T) {
	t.Parallel()

	path := cacheWith(t, withSevenDay("50", "2026-09-16T04:00:00Z", "2026-09-14T18:00:00Z", true))

	got, err := quota.Read(path)
	if err != nil {
		t.Fatalf("Read(%q) = _, %v; want a reading", path, err)
	}
	if !got.Stale {
		t.Errorf("Stale = false; quota-axi's own state.stale is true")
	}
}

// Not parallel: t.Setenv forbids it.
func TestDefaultCachePathPrefersXDGCacheHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	got, err := quota.DefaultCachePath()
	if err != nil {
		t.Fatalf("DefaultCachePath() = _, %v; want a path", err)
	}
	if want := filepath.Join(dir, "quota-axi", "quotas.json"); got != want {
		t.Errorf("DefaultCachePath() = %q; want %q", got, want)
	}
}

// Not parallel: t.Setenv forbids it.
func TestDefaultCachePathFallsBackToDotCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := quota.DefaultCachePath()
	if err != nil {
		t.Fatalf("DefaultCachePath() = _, %v; want a path", err)
	}
	if want := filepath.Join(home, ".cache", "quota-axi", "quotas.json"); got != want {
		t.Errorf("DefaultCachePath() = %q; want %q — the same fallback quota-axi's own cacheDirPath() uses", got, want)
	}
}
