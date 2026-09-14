package httpapi

// quotastatus.go is GET /dashboard/quota (spec 016): the header's weekly-quota
// bar asking what quota-axi's own on-disk cache last said about the claude
// provider's seven-day window. Modelled line-for-line on authstatus.go — same
// door, same shape of route — with one deliberate difference: authstatus.go
// caches its answer because asking costs a subprocess exec, and this route
// costs a stat and a small file, cheap enough to repeat on every request
// rather than share across callers.
//
// # What a check that cannot run must do
//
// This daemon never asks quota-axi to run — no exec, no network, no
// credential file, internal/quota only reads a file another tool on this host
// already wrote. So every way that read can go wrong — the file missing, the
// JSON malformed, no claude provider, no seven_day window, a percentage out of
// range — answers "unknown" here, never a zero percent that would read as a
// healthy quota. That exact failure has bitten this owner before, and it is
// the one this file exists to not repeat.

import (
	"net/http"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/quota"
)

// patternDashboardQuota is the route, method included for the reason every
// other pattern in this package carries one.
const patternDashboardQuota = "GET /dashboard/quota"

// quotaStaleAfter is D4's other half, alongside quota-axi's own state.stale:
// a reading that is not stale by quota-axi's own account can still be old
// enough that showing it as current would be wrong. Two hours is long enough
// that the header's own 60-second poll never trips it on a host whose
// quota-axi timer is healthy, and short enough that a stopped one is visible
// inside the same working day rather than only the next time somebody looks
// closely.
const quotaStaleAfter = 2 * time.Hour

// quotaState is the route's whole vocabulary: two words rather than auth's
// three, because there is no "checking" to answer with here — a request that
// reaches this handler has already been sent, and its answer is either a
// reading or the admission that there is none.
type quotaState string

const (
	quotaOK      quotaState = "ok"
	quotaUnknown quotaState = "unknown"
)

// quotaStatusResponse is what GET /dashboard/quota answers with. The value
// fields are pointers rather than bare types so that "ok" with a real 0% used
// still marshals a percentUsed of 0 rather than omitting the field the way an
// int's zero value would under omitempty — the omission is reserved for
// "unknown" alone, where there is truly nothing to report.
type quotaStatusResponse struct {
	State       quotaState `json:"state"`
	PercentUsed *int       `json:"percentUsed,omitempty"`
	ResetsAt    *string    `json:"resetsAt,omitempty"`
	RefreshedAt *string    `json:"refreshedAt,omitempty"`
	Stale       *bool      `json:"stale,omitempty"`
}

// readQuota is the one place this package reads quota-axi's cache, so the
// route and any future caller share one reading of "no path resolved at
// construction" rather than each inventing its own.
func (s *Server) readQuota() (quota.Reading, error) {
	if s.quotaCachePath == "" {
		return quota.Reading{}, quota.ErrNoCacheFile
	}
	return quota.Read(s.quotaCachePath)
}

// dashboardQuota serves GET /dashboard/quota (spec 016).
//
// Every error internal/quota can return is folded into the same "unknown" —
// this route does not distinguish a missing file from a malformed one in its
// answer, because a browser acts on the three words the same way regardless
// of which one this daemon could not manage, and the one place a difference
// between them might matter is a report this handler does not make: quota-axi
// not having run yet is the ordinary state of a freshly installed host, not a
// fault this daemon should narrate on every poll.
func (s *Server) dashboardQuota(w http.ResponseWriter, r *http.Request) {
	reading, err := s.readQuota()
	if err != nil {
		s.writeJSON(w, r, http.StatusOK, quotaStatusResponse{State: quotaUnknown})
		return
	}

	stale := reading.Stale || s.clock.Now().Sub(reading.RefreshedAt) > quotaStaleAfter
	percentUsed := reading.PercentUsed
	resetsAt := reading.ResetsAt
	refreshedAt := reading.RefreshedAtRaw
	s.writeJSON(w, r, http.StatusOK, quotaStatusResponse{
		State:       quotaOK,
		PercentUsed: &percentUsed,
		ResetsAt:    &resetsAt,
		RefreshedAt: &refreshedAt,
		Stale:       &stale,
	})
}
