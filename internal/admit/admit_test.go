package admit

import (
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
)

var (
	now   = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	roots = []config.ApprovedRoot{{Path: "/code"}}
)

func obj(name, uid string, created time.Time, mut func(*v1alpha1.AgentSession)) v1alpha1.AgentSession {
	o := v1alpha1.AgentSession{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.ObjectMeta{Name: name, Namespace: "crswd-next", UID: uid, CreationTimestamp: created},
		Spec: v1alpha1.AgentSessionSpec{
			SessionName:  name,
			Owner:        "craig",
			WorkDir:      "/code/repo",
			StartCommand: "claude",
			Lifetime:     "8h",
		},
	}
	if mut != nil {
		mut(&o)
	}
	return o
}

func TestAdmit(t *testing.T) {
	t.Parallel()

	early := now.Add(-3 * time.Hour)
	mid := now.Add(-2 * time.Hour)
	late := now.Add(-1 * time.Hour)

	withPhase := func(p v1alpha1.Phase) func(*v1alpha1.AgentSession) {
		return func(o *v1alpha1.AgentSession) { o.Status.Phase = p }
	}
	workDir := func(p string) func(*v1alpha1.AgentSession) {
		return func(o *v1alpha1.AgentSession) { o.Spec.WorkDir = p }
	}
	lifetime := func(l string) func(*v1alpha1.AgentSession) {
		return func(o *v1alpha1.AgentSession) { o.Spec.Lifetime = l }
	}

	tests := []struct {
		name   string
		obj    v1alpha1.AgentSession
		cap    int
		others []v1alpha1.AgentSession
		want   bool
		reason string
	}{
		{name: "admitted", obj: obj("a", "u-a", mid, nil), cap: 2, want: true},
		{name: "outside allowlist", obj: obj("a", "u-a", mid, workDir("/etc")), cap: 2, reason: "approved"},
		{name: "outside sibling boundary /codeEVIL", obj: obj("a", "u-a", mid, workDir("/codeEVIL")), cap: 2, reason: "approved"},
		{name: "dotdot escape", obj: obj("a", "u-a", mid, workDir("/code/../etc")), cap: 2, reason: "approved"},
		{name: "relative path", obj: obj("a", "u-a", mid, workDir("code/repo")), cap: 2, reason: "approved"},
		{name: "empty workdir", obj: obj("a", "u-a", mid, workDir("")), cap: 2, reason: "approved"},
		{name: "bad lifetime garbage", obj: obj("a", "u-a", mid, lifetime("soon")), cap: 2, reason: "lifetime"},
		{name: "bad lifetime never", obj: obj("a", "u-a", mid, lifetime("never")), cap: 2, reason: "lifetime"},
		{name: "bad lifetime zero", obj: obj("a", "u-a", mid, lifetime("0s")), cap: 2, reason: "lifetime"},
		{name: "bad lifetime negative", obj: obj("a", "u-a", mid, lifetime("-1h")), cap: 2, reason: "lifetime"},
		{name: "past lifetime", obj: obj("a", "u-a", early, lifetime("1h")), cap: 2, reason: "past its lifetime"},
		{name: "past lifetime exactly now", obj: obj("a", "u-a", now.Add(-time.Hour), lifetime("1h")), cap: 2, reason: "past its lifetime"},
		{name: "cap zero rejects everything", obj: obj("a", "u-a", mid, nil), cap: 0, reason: "limit"},
		{name: "cap negative rejects everything", obj: obj("a", "u-a", mid, nil), cap: -1, reason: "limit"},
		{
			name: "over cap by earlier objects",
			obj:  obj("c", "u-c", late, nil),
			cap:  2,
			others: []v1alpha1.AgentSession{
				obj("a", "u-a", early, nil),
				obj("b", "u-b", mid, nil),
			},
			reason: "limit",
		},
		{
			name: "under cap, later objects do not count",
			obj:  obj("a", "u-a", early, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("b", "u-b", mid, nil),
				obj("c", "u-c", late, nil),
			},
			want: true,
		},
		{
			name: "cap ordering by creationTimestamp admits the oldest",
			obj:  obj("z", "u-z", early, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("a", "u-a", late, nil),
			},
			want: true,
		},
		{
			name: "cap tie broken by smaller name, smaller admitted",
			obj:  obj("a", "u-a", mid, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("b", "u-b", mid, nil),
			},
			want: true,
		},
		{
			name: "cap tie broken by smaller name, larger rejected",
			obj:  obj("b", "u-b", mid, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("a", "u-a", mid, nil),
			},
			reason: "limit",
		},
		{
			name: "rejected and failed do not hold a slot",
			obj:  obj("c", "u-c", late, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("a", "u-a", early, withPhase(v1alpha1.PhaseRejected)),
				obj("b", "u-b", mid, withPhase(v1alpha1.PhaseFailed)),
			},
			want: true,
		},
		{
			name: "running holds a slot",
			obj:  obj("c", "u-c", late, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("a", "u-a", early, withPhase(v1alpha1.PhaseRunning)),
			},
			reason: "limit",
		},
		{
			name: "the object itself is not counted by uid",
			obj:  obj("a", "u-a", mid, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("a", "u-a", early, nil),
			},
			want: true,
		},
		{
			name: "the object itself is not counted by namespace and name",
			obj:  obj("a", "", mid, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("a", "", early, nil),
			},
			want: true,
		},
		{
			name: "same name in another namespace is a different object",
			obj:  obj("a", "", mid, nil),
			cap:  1,
			others: []v1alpha1.AgentSession{
				obj("a", "", early, func(o *v1alpha1.AgentSession) { o.Metadata.Namespace = "other" }),
			},
			reason: "limit",
		},
		{
			name:   "workdir is checked before the cap",
			obj:    obj("c", "u-c", late, workDir("/etc")),
			cap:    0,
			reason: "approved",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, reason := Admit(tt.obj, roots, tt.cap, tt.others, now)
			if got != tt.want {
				t.Fatalf("Admit = %v (%q), want %v", got, reason, tt.want)
			}
			if tt.want {
				if reason != "" {
					t.Fatalf("admitted with a reason %q", reason)
				}
				return
			}
			if !strings.Contains(reason, tt.reason) {
				t.Fatalf("reason %q does not contain %q", reason, tt.reason)
			}
			if strings.Contains(reason, tt.obj.Spec.WorkDir) && tt.obj.Spec.WorkDir != "" {
				t.Fatalf("reason %q echoes the caller's path", reason)
			}
		})
	}
}

func TestAdmitDoesNotMutateOthers(t *testing.T) {
	t.Parallel()
	others := []v1alpha1.AgentSession{
		obj("b", "u-b", now.Add(-time.Hour), nil),
		obj("a", "u-a", now.Add(-2*time.Hour), nil),
	}
	Admit(obj("c", "u-c", now.Add(-30*time.Minute), nil), roots, 5, others, now)
	if others[0].Metadata.Name != "b" || others[1].Metadata.Name != "a" {
		t.Fatal("Admit reordered the caller's slice")
	}
}
