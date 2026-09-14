// Package quota reads quota-axi's own on-disk cache and nothing else: no exec,
// no network, no credential file. quota-axi is another tool on this host that
// already writes ~/.cache/quota-axi/quotas.json atomically (temp+rename, mode
// 0600, same user as this daemon); this package only reads what is already
// there.
//
// The one fact this daemon wants out of it is the claude provider's seven-day
// window — total weekly usage, spec 016 — and every failure short of a clean
// answer is a sentinel a caller checks with errors.Is, never a zero Reading
// standing in for one. A quota nobody could read is not 0% used.
package quota

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"
)

// The sentinels httpapi's quota route maps to "unknown" (spec 016 D4). Each
// names the one thing that went wrong so a report, if one is ever added above
// this package, can say which — but never with the file's own bytes: a
// percentage is not a secret, and neither is a reason authored here.
var (
	ErrNoCacheFile      = errors.New("quota: no cache file")
	ErrUnreadable       = errors.New("quota: cache file could not be read")
	ErrMalformed        = errors.New("quota: cache file is not valid JSON")
	ErrNoClaudeProvider = errors.New("quota: no claude provider in the cache")
	ErrNoWeeklyWindow   = errors.New("quota: claude provider has no seven_day window")
	ErrInvalidPercent   = errors.New("quota: seven_day percentUsed is missing or out of range")
)

// Reading is the one window this daemon cares about, parsed into a small value
// rather than the whole cache document: the header renders three facts and
// nothing else needs to reach it.
type Reading struct {
	// PercentUsed is the seven_day window's percentUsed, 0..100 inclusive —
	// checked here so nothing downstream has to ask whether it might not be.
	PercentUsed int

	// ResetsAt is the seven_day window's resetsAt, verbatim as quota-axi wrote
	// it (an ISO 8601 string with quota-axi's own offset), and empty when the
	// window carried none. Passed through rather than reparsed: the browser
	// converts it to the viewer's local time, and a round trip through a Go
	// time.Time and back would risk losing precision or the zone quota-axi
	// chose to write.
	ResetsAt string

	// RefreshedAt is when quota-axi itself last refreshed this reading —
	// state.refreshedAt, falling back to the document's own generatedAt when
	// the provider carries none — parsed for age comparisons against a
	// caller's clock (D4's two-hour staleness rule lives in httpapi, which
	// holds the testable clock; this package holds no notion of "now").
	RefreshedAt time.Time

	// RefreshedAtRaw is the exact string RefreshedAt was parsed from, passed
	// through to the response for the reason ResetsAt is.
	RefreshedAtRaw string

	// Stale is quota-axi's own state.stale for the claude provider — one of
	// the two conditions D4 ORs together, the other being RefreshedAt's age.
	Stale bool
}

// cacheDirName is quota-axi's own directory name under the cache root
// (dist/src/lib/fs.js: cacheDirPath()), transcribed here because there is no
// package to import it from — quota-axi is a Node CLI, not a Go dependency.
const cacheDirName = "quota-axi"

// cacheFileName is quota-axi's own file name in that directory
// (fs.js: cacheFilePath()).
const cacheFileName = "quotas.json"

// claudeProviderID and sevenDayWindowID are quota-axi's own vocabulary
// (types.d.ts: ProviderId, QuotaWindow.id) for the provider and window this
// daemon reads. Nothing else in the cache is this daemon's business.
const (
	claudeProviderID = "claude"
	sevenDayWindowID = "seven_day"
)

// DefaultCachePath mirrors quota-axi's own cacheDirPath() exactly: XDG_CACHE_HOME
// when set, else ~/.cache, joined with quota-axi/quotas.json. No new
// configuration key — a daemon that read a different path than the tool
// writing it would silently read nothing forever.
func DefaultCachePath() (string, error) {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, cacheDirName, cacheFileName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("quota: resolve the cache directory: %w", err)
	}
	return filepath.Join(home, ".cache", cacheDirName, cacheFileName), nil
}

// cacheDocument is quota-axi's own schema (types.d.ts QuotaAxiResponse), read
// with plain json.Unmarshal rather than DisallowUnknownFields: this is not
// caller input crossing a security boundary (docs/security.md §2) — it is a
// file another tool on this same host and user already wrote, and a future
// schema version adding a field must not turn every read into ErrMalformed.
type cacheDocument struct {
	GeneratedAt string          `json:"generatedAt"`
	Providers   []cacheProvider `json:"providers"`
}

type cacheProvider struct {
	Provider string        `json:"provider"`
	Windows  []cacheWindow `json:"windows"`
	State    cacheState    `json:"state"`
}

type cacheWindow struct {
	ID          string   `json:"id"`
	PercentUsed *float64 `json:"percentUsed"`
	ResetsAt    string   `json:"resetsAt"`
}

type cacheState struct {
	Stale       bool   `json:"stale"`
	RefreshedAt string `json:"refreshedAt"`
}

// Read parses the cache at path and extracts the claude provider's seven_day
// window. Every failure is one of this package's sentinels — never a zero
// Reading, which would be indistinguishable from a real 0% (spec 016 D4: "a
// check that cannot run must refuse, never pass").
func Read(path string) (Reading, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is quota.DefaultCachePath()'s own resolution (or a test fixture); no request or caller-supplied value reaches this call.
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Reading{}, ErrNoCacheFile
		}
		return Reading{}, ErrUnreadable
	}

	var doc cacheDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return Reading{}, ErrMalformed
	}

	var provider *cacheProvider
	for i := range doc.Providers {
		if doc.Providers[i].Provider == claudeProviderID {
			provider = &doc.Providers[i]
			break
		}
	}
	if provider == nil {
		return Reading{}, ErrNoClaudeProvider
	}

	var window *cacheWindow
	for i := range provider.Windows {
		if provider.Windows[i].ID == sevenDayWindowID {
			window = &provider.Windows[i]
			break
		}
	}
	if window == nil {
		return Reading{}, ErrNoWeeklyWindow
	}

	if window.PercentUsed == nil || *window.PercentUsed < 0 || *window.PercentUsed > 100 {
		return Reading{}, ErrInvalidPercent
	}

	refreshedRaw := provider.State.RefreshedAt
	if refreshedRaw == "" {
		refreshedRaw = doc.GeneratedAt
	}
	// A timestamp this daemon cannot parse is read as the zero time rather than
	// a fourth failure mode: httpapi's staleness math (D4) then sees an
	// arbitrarily old refresh and calls it stale, which is the fail-safe
	// direction — never the fail-open one of treating an unreadable age as
	// fresh.
	refreshedAt, _ := time.Parse(time.RFC3339, refreshedRaw) //nolint:errcheck // an unparsable timestamp deliberately falls back to the zero time, read as arbitrarily old rather than as a fourth failure mode — see the comment above.

	return Reading{
		PercentUsed:    int(math.Round(*window.PercentUsed)),
		ResetsAt:       window.ResetsAt,
		RefreshedAt:    refreshedAt,
		RefreshedAtRaw: refreshedRaw,
		Stale:          provider.State.Stale,
	}, nil
}
