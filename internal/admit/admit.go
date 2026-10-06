// Package admit decides whether one AgentSession may run. It is pure: no I/O,
// no clock, no globals, so the reconciler and the tests reach the same verdict
// from the same inputs.
package admit

import (
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
)

// Admit returns true, or false with a fixed sentence saying why. The reason is
// bound for status and the audit trail, so it never repeats the caller's path.
//
// The checks run cheapest and most fundamental first: where, how long, then
// how many. others is every other AgentSession in the namespace; cap bounds
// how many live ones may be older than obj. maxLifetime is the configured
// ceiling (config.SessionLifetimeMax); zero or negative means none, as
// CRSW_SESSION_LIFETIME_MAX=never does for a session made through the API.
func Admit(obj v1alpha1.AgentSession, roots []config.ApprovedRoot, cap int, maxLifetime time.Duration, others []v1alpha1.AgentSession, now time.Time) (bool, string) {
	// Lexical only: the daemon and reconciler cannot see the session's
	// filesystem. The resolved check runs in the pod.
	if _, err := session.LexicalWorkDir(obj.Spec.WorkDir, roots); err != nil {
		return false, "working directory is not under an approved root"
	}

	// A pod needs a finite activeDeadlineSeconds, so "never" and garbage fail
	// here along with zero and negative durations.
	lifetime, err := time.ParseDuration(obj.Spec.Lifetime)
	if err != nil || lifetime <= 0 {
		return false, "lifetime must be a positive duration such as 8h"
	}
	// An object written with kubectl skips the API's override check, so the
	// ceiling is enforced here too (FR-007).
	if maxLifetime > 0 && lifetime > maxLifetime {
		return false, "lifetime is longer than the configured maximum"
	}
	if !obj.Metadata.CreationTimestamp.Add(lifetime).After(now) {
		return false, "session is past its lifetime"
	}

	// Fail closed, as an empty root list does.
	if cap <= 0 {
		return false, "session limit reached"
	}
	older := 0
	for _, o := range others {
		if sameObject(obj, o) || holdsNoSlot(o) || !createdBefore(o, obj) {
			continue
		}
		older++
	}
	if older >= cap {
		return false, "session limit reached"
	}
	return true, ""
}

// sameObject matches on UID when both sides have one. Without it, two objects
// in different namespaces sharing a name would be taken for one another.
func sameObject(a, b v1alpha1.AgentSession) bool {
	if a.Metadata.UID != "" && b.Metadata.UID != "" {
		return a.Metadata.UID == b.Metadata.UID
	}
	return a.Metadata.Namespace == b.Metadata.Namespace && a.Metadata.Name == b.Metadata.Name
}

// holdsNoSlot is true for the phases that run no pod, so a rejected object
// cannot starve a later one of its place.
func holdsNoSlot(o v1alpha1.AgentSession) bool {
	return o.Status.Phase == v1alpha1.PhaseRejected || o.Status.Phase == v1alpha1.PhaseFailed
}

// createdBefore orders by creationTimestamp, ties by smaller name. Ranking by
// age rather than arrival makes the verdict the same whichever object the
// reconciler happens to look at first, so a burst over the cap admits the
// oldest ones and does not flap.
func createdBefore(a, b v1alpha1.AgentSession) bool {
	ta, tb := a.Metadata.CreationTimestamp, b.Metadata.CreationTimestamp
	if !ta.Equal(tb) {
		return ta.Before(tb)
	}
	return a.Metadata.Name < b.Metadata.Name
}
