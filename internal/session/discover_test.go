package session

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	discoverID  = "01a10e8c-d5f5-7452-8561-233103b38287"
	discoverID2 = "11a10e8c-d5f5-7452-8561-233103b38287"
)

type discoverTree struct {
	t        *testing.T
	proc     string
	sessions string
}

func newDiscoverTree(t *testing.T) *discoverTree {
	t.Helper()
	return &discoverTree{t: t, proc: t.TempDir(), sessions: filepath.Join(t.TempDir(), "sessions")}
}

func (d *discoverTree) children(pid int, kids ...int) {
	d.t.Helper()
	parts := make([]string, len(kids))
	for i, k := range kids {
		parts[i] = strconv.Itoa(k)
	}
	d.childrenRaw(pid, strings.Join(parts, " "))
}

func (d *discoverTree) childrenRaw(pid int, body string) {
	d.t.Helper()
	dir := filepath.Join(d.proc, strconv.Itoa(pid), "task", strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		d.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "children"), []byte(body), 0o600); err != nil {
		d.t.Fatal(err)
	}
}

func (d *discoverTree) rollout(id string) string {
	d.t.Helper()
	dir := filepath.Join(d.sessions, "2026", "10", "06")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		d.t.Fatal(err)
	}
	p := filepath.Join(dir, "rollout-2026-10-06T00-11-13-"+id+".jsonl")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		d.t.Fatal(err)
	}
	return p
}

func (d *discoverTree) fd(pid, n int, target string) {
	d.t.Helper()
	dir := filepath.Join(d.proc, strconv.Itoa(pid), "fd")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		d.t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, strconv.Itoa(n))); err != nil {
		d.t.Fatal(err)
	}
}

func TestDiscoverCodexConversation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		build   func(d *discoverTree) int
		want    string
		wantErr error
	}{
		{"found at depth 2", func(d *discoverTree) int {
			d.children(100, 200)
			d.children(200, 300)
			d.fd(300, 37, d.rollout(discoverID))
			return 100
		}, discoverID, nil},
		{"found on the pane itself", func(d *discoverTree) int {
			d.fd(100, 3, d.rollout(discoverID))
			return 100
		}, discoverID, nil},
		{"none open", func(d *discoverTree) int {
			d.children(100, 200)
			d.fd(200, 1, "/dev/null")
			return 100
		}, "", nil},
		{"two distinct ids", func(d *discoverTree) int {
			d.children(100, 200)
			d.fd(100, 3, d.rollout(discoverID))
			d.fd(200, 4, d.rollout(discoverID2))
			return 100
		}, "", ErrAmbiguousConversation},
		{"same id on two fds and two pids", func(d *discoverTree) int {
			p := d.rollout(discoverID)
			d.children(100, 200)
			d.fd(100, 3, p)
			d.fd(100, 4, p)
			d.fd(200, 5, p)
			return 100
		}, discoverID, nil},
		{"link outside sessionsDir", func(d *discoverTree) int {
			other := filepath.Join(t.TempDir(), "2026", "10", "06")
			if err := os.MkdirAll(other, 0o750); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(other, "rollout-2026-10-06T00-11-13-"+discoverID+".jsonl")
			if err := os.WriteFile(p, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			d.fd(100, 3, p)
			return 100
		}, "", nil},
		{"deleted rollout does not match", func(d *discoverTree) int {
			d.fd(100, 3, d.rollout(discoverID)+" (deleted)")
			return 100
		}, "", nil},
		{"depth 6 is read", func(d *discoverTree) int {
			for p := 100; p < 106; p++ {
				d.children(p, p+1)
			}
			d.fd(106, 3, d.rollout(discoverID))
			return 100
		}, discoverID, nil},
		{"depth 7 is not read", func(d *discoverTree) int {
			for p := 100; p < 107; p++ {
				d.children(p, p+1)
			}
			d.fd(107, 3, d.rollout(discoverID))
			return 100
		}, "", nil},
		{"cycle is bounded by the visited set", func(d *discoverTree) int {
			d.children(100, 200)
			d.children(200, 100)
			d.fd(200, 3, d.rollout(discoverID))
			return 100
		}, discoverID, nil},
		{"pane pid zero", func(d *discoverTree) int {
			d.fd(100, 3, d.rollout(discoverID))
			return 0
		}, "", nil},
		{"4097 fd entries on one pid", func(d *discoverTree) int {
			for i := 0; i <= discoverMaxFDs; i++ {
				d.fd(100, i, "/dev/null")
			}
			return 100
		}, "", ErrDiscoveryBounds},
		{"65 child pids", func(d *discoverTree) int {
			kids := make([]int, 65)
			for i := range kids {
				kids[i] = 200 + i
			}
			d.children(100, kids...)
			return 100
		}, "", ErrDiscoveryBounds},
		{"children file over 64 KiB", func(d *discoverTree) int {
			d.childrenRaw(100, "200 "+strings.Repeat(" ", discoverChildrenRead))
			return 100
		}, "", ErrDiscoveryBounds},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := newDiscoverTree(t)
			pid := tt.build(d)
			got, err := DiscoverCodexConversation(d.proc, pid, d.sessions)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("id = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiscoverCodexConversationEmptySessionsDir(t *testing.T) {
	t.Parallel()
	d := newDiscoverTree(t)
	d.fd(100, 3, d.rollout(discoverID))
	got, err := DiscoverCodexConversation(d.proc, 100, "")
	if err != nil || got != "" {
		t.Fatalf("got %q, %v; want empty, nil", got, err)
	}
}
