package session

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
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
	// The sessions directory exists, as it does once Codex has run: discovery
	// resolves it, and one that cannot be resolved records nothing.
	sessions := filepath.Join(t.TempDir(), "sessions")
	if err := os.MkdirAll(sessions, 0o750); err != nil {
		t.Fatal(err)
	}
	return &discoverTree{t: t, proc: t.TempDir(), sessions: sessions}
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

// sweepRig is a Codex session on a fake host, a supervisor over the fixture's
// own manager (so the codex entry resolves), and a finder the test controls.
type sweepRig struct {
	f       managerFixture
	s       Session
	sup     *Supervisor
	journal *Journal
	calls   int
}

type finder func(ctx context.Context, s Session) (string, error)

func newSweepRig(t *testing.T, find finder) *sweepRig {
	t.Helper()

	f, s := codexFixture(t)
	sup, err := NewSupervisor(f.mgr, audit.NewTo(&bytes.Buffer{}, func() time.Time { return f.now }))
	if err != nil {
		t.Fatalf("NewSupervisor() unexpected error: %v", err)
	}
	// The fake pane says claude, which is outside Codex's process set.
	f.tmux.SetPaneCommand(s.TmuxName(), "codex")
	r := &sweepRig{f: f, s: *s, sup: sup, journal: tempJournal(t)}
	f.mgr.SetJournal(r.journal)
	f.mgr.findCodexConversation = func(ctx context.Context, s Session) (string, error) {
		r.calls++
		return find(ctx, s)
	}
	return r
}

func (r *sweepRig) stored(t *testing.T) string {
	t.Helper()
	return mustStored(t, r.f, r.s.ID).ConversationID
}

func (r *sweepRig) option(t *testing.T) string {
	t.Helper()
	infos, err := r.f.tmux.List(context.Background())
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	for _, info := range infos {
		if info.Name == r.s.TmuxName() {
			return info.ConversationID
		}
	}
	t.Fatalf("session %s is not on the fake host", r.s.ID)
	return ""
}

// journalled is the conversation and event of the journal's latest record for
// the session.
func (r *sweepRig) journalled(t *testing.T) (conversation, event string) {
	t.Helper()
	records, _, err := r.journal.Replay()
	if err != nil {
		t.Fatalf("Replay() unexpected error: %v", err)
	}
	for _, rec := range records {
		if rec.ID == r.s.ID {
			return rec.Conversation, rec.Event
		}
	}
	return "", ""
}

func found(id string) finder {
	return func(context.Context, Session) (string, error) { return id, nil }
}

func (r *sweepRig) wantRecorded(t *testing.T, id string) {
	t.Helper()
	if got := r.stored(t); got != id {
		t.Errorf("store conversation = %q, want %q", got, id)
	}
	if got := r.option(t); got != id {
		t.Errorf("tmux option conversation = %q, want %q", got, id)
	}
	if conv, event := r.journalled(t); conv != id || event != journalDiscovered {
		t.Errorf("journal latest = (%q, %q), want (%q, %q)", conv, event, id, journalDiscovered)
	}
}

func TestSweepRecordsCodexConversation(t *testing.T) {
	r := newSweepRig(t, found(discoverID))
	if got := r.stored(t); got != "" {
		t.Fatalf("a fresh Codex session already carries conversation %q", got)
	}

	if err := r.sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v", err)
	}
	r.wantRecorded(t, discoverID)
}

func TestSweepIgnoresClaude(t *testing.T) {
	f := newManagerFixture(t)
	s := liveSession(t, f)
	sup, err := NewSupervisor(f.mgr, audit.NewTo(&bytes.Buffer{}, func() time.Time { return f.now }))
	if err != nil {
		t.Fatalf("NewSupervisor() unexpected error: %v", err)
	}
	f.mgr.SetJournal(tempJournal(t))
	calls := 0
	f.mgr.findCodexConversation = func(context.Context, Session) (string, error) {
		calls++
		return discoverID, nil
	}
	before := mustStored(t, f, s.ID).ConversationID

	if err := sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v", err)
	}
	if calls != 0 {
		t.Errorf("the finder ran %d times for a Claude session, want 0", calls)
	}
	if got := mustStored(t, f, s.ID).ConversationID; got != before {
		t.Errorf("a Claude session's conversation moved from %q to %q", before, got)
	}
}

func TestSweepKeepsExistingConversation(t *testing.T) {
	r := newSweepRig(t, found(discoverID2))
	if err := r.f.store.SetConversation(r.s.ID, discoverID); err != nil {
		t.Fatalf("SetConversation() unexpected error: %v", err)
	}

	if err := r.sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v", err)
	}
	if r.calls != 0 {
		t.Errorf("the finder ran %d times for a session with a conversation, want 0", r.calls)
	}
	if got := r.stored(t); got != discoverID {
		t.Errorf("store conversation = %q, want the recorded %q kept", got, discoverID)
	}
}

func TestSweepAmbiguousRecordsNothing(t *testing.T) {
	testDiscoveryError(t, ErrAmbiguousConversation)
}

func TestSweepBoundsErrorRecordsNothing(t *testing.T) {
	testDiscoveryError(t, ErrDiscoveryBounds)
}

func testDiscoveryError(t *testing.T, want error) {
	t.Helper()

	r := newSweepRig(t, func(context.Context, Session) (string, error) { return "", want })

	err := r.sup.Sweep(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("Sweep() = %v, want it to join %v", err, want)
	}
	if got := r.stored(t); got != "" {
		t.Errorf("store conversation = %q, want none", got)
	}
	if got := r.option(t); got != "" {
		t.Errorf("tmux option conversation = %q, want none", got)
	}
	if _, event := r.journalled(t); event == journalDiscovered {
		t.Errorf("journal latest event = %q, want no discovered record", event)
	}
	if st := mustStored(t, r.f, r.s.ID); st.State == StateFailed || st.ReviveAttempts != 0 {
		t.Errorf("a discovery error changed the verdict: state %s, attempts %d", st.State, st.ReviveAttempts)
	}
}

func TestSweepRetriesAfterOptionFailure(t *testing.T) {
	r := newSweepRig(t, found(discoverID))
	boom := errors.New("tmux refused the option")
	r.f.tmux.FailOp(tmuxctl.OpSetOption, boom)

	if err := r.sup.Sweep(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("first Sweep() = %v, want it to join %v", err, boom)
	}
	if got := r.stored(t); got != "" {
		t.Fatalf("store conversation = %q after the option failed, want none so the next sweep retries", got)
	}

	r.f.tmux.FailOp(tmuxctl.OpSetOption, nil)
	if err := r.sup.Sweep(context.Background()); err != nil {
		t.Fatalf("second Sweep() = %v", err)
	}
	r.wantRecorded(t, discoverID)
}

func TestSweepRetriesAfterJournalFailure(t *testing.T) {
	r := newSweepRig(t, found(discoverID))
	// A regular file where the journal's directory belongs makes Append fail
	// until the file is removed.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("plant a blocker: %v", err)
	}
	r.f.mgr.SetJournal(NewJournal(filepath.Join(blocker, "sessions.jsonl")))

	if err := r.sup.Sweep(context.Background()); err == nil {
		t.Fatal("first Sweep() = nil, want the journal failure")
	}
	if got := r.stored(t); got != "" {
		t.Fatalf("store conversation = %q after the journal failed, want none so the next sweep retries", got)
	}

	if err := os.Remove(blocker); err != nil {
		t.Fatalf("remove the blocker: %v", err)
	}
	if err := r.sup.Sweep(context.Background()); err != nil {
		t.Fatalf("second Sweep() = %v", err)
	}
	if got := r.stored(t); got != discoverID {
		t.Errorf("store conversation = %q, want %q", got, discoverID)
	}
	if got := r.option(t); got != discoverID {
		t.Errorf("tmux option conversation = %q, want %q", got, discoverID)
	}
}

func TestSweepRefusesAnInvalidDiscoveredID(t *testing.T) {
	r := newSweepRig(t, found("not a conversation id"))

	if err := r.sup.Sweep(context.Background()); err == nil {
		t.Fatal("Sweep() = nil, want the id refused")
	}
	if got := r.stored(t); got != "" {
		t.Errorf("store conversation = %q, want none", got)
	}
	if got := r.option(t); got != "" {
		t.Errorf("tmux option conversation = %q, want none", got)
	}
}

func TestReplayRestoresDiscoveredConversation(t *testing.T) {
	r := newSweepRig(t, found(discoverID))
	if err := r.sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v", err)
	}

	// The restarted daemon: an empty store over the same journal, with the shell
	// gone from the host.
	if err := r.f.tmux.Kill(context.Background(), r.s.TmuxName()); err != nil {
		t.Fatalf("Kill() unexpected error: %v", err)
	}
	store := NewStore()
	mgr := r.f.managerAt(t, store, r.f.now)
	mgr.SetJournal(r.journal)
	if _, _, err := mgr.ReplayJournal(context.Background()); err != nil {
		t.Fatalf("ReplayJournal() = %v", err)
	}
	got, ok := store.lookup(r.s.ID)
	if !ok {
		t.Fatalf("session %s was not replayed", r.s.ID)
	}
	if got.ConversationID != discoverID {
		t.Errorf("replayed conversation = %q, want %q", got.ConversationID, discoverID)
	}
}

func TestDiscoverCodexConversationSymlinkedSessionsDir(t *testing.T) {
	t.Parallel()

	d := newDiscoverTree(t)
	d.fd(100, 3, d.rollout(discoverID))

	link := filepath.Join(t.TempDir(), "codex-sessions")
	if err := os.Symlink(d.sessions, link); err != nil {
		t.Fatal(err)
	}
	got, err := DiscoverCodexConversation(d.proc, 100, link)
	if err != nil || got != discoverID {
		t.Fatalf("DiscoverCodexConversation() through a symlinked sessions dir = %q, %v; want %q", got, err, discoverID)
	}

	got, err = DiscoverCodexConversation(d.proc, 100, filepath.Join(t.TempDir(), "absent"))
	if err != nil || got != "" {
		t.Fatalf("DiscoverCodexConversation() with an unresolvable dir = %q, %v; want nothing", got, err)
	}
}

// pausesAfterConversationOption lets the first conversation option write
// through and then holds the caller, which is the point in a discovery between
// "the host has the id" and "the journal and store do".
type pausesAfterConversationOption struct {
	tmuxctl.Controller
	once            sync.Once
	entered, resume chan struct{}
}

func (p *pausesAfterConversationOption) SetOption(ctx context.Context, name, option, value string) error {
	err := p.Controller.SetOption(ctx, name, option, value)
	if option == tmuxctl.OptionConversation {
		p.once.Do(func() {
			close(p.entered)
			<-p.resume
		})
	}
	return err
}

// TestDestroyDuringDiscoveryDoesNotResurrectTheSession is 019 core review #2.
// Discovery is held after the host has taken the id while the operator destroys
// the session; whatever order the two then take, the journal's last word on the
// session must be that it ended, and the store must not hold it.
func TestDestroyDuringDiscoveryDoesNotResurrectTheSession(t *testing.T) {
	r := newSweepRig(t, found(discoverID))
	pause := &pausesAfterConversationOption{Controller: r.f.tmux, entered: make(chan struct{}), resume: make(chan struct{})}
	r.f.mgr.tmux = pause

	sweepDone := make(chan error, 1)
	go func() { sweepDone <- r.sup.Sweep(context.Background()) }()
	<-pause.entered

	destroyDone := make(chan struct{})
	var destroyErr error
	go func() {
		defer close(destroyDone)
		destroyErr = r.f.mgr.Destroy(context.Background(), r.s)
	}()
	// Without the lock Destroy finishes here, before discovery has journalled.
	_ = blockedWithin(destroyDone)
	close(pause.resume)
	<-destroyDone
	if err := <-sweepDone; err != nil {
		t.Logf("Sweep() = %v", err)
	}
	if destroyErr != nil {
		t.Fatalf("Destroy() unexpected error: %v", destroyErr)
	}

	if _, event := r.journalled(t); event != journalEnded {
		t.Errorf("the journal's last record for the session is %q, want %q", event, journalEnded)
	}
	if _, err := r.f.store.Get(r.s.ID, r.s.Owner); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("the store still holds the destroyed session: %v", err)
	}
}

// TestDiscoveryRecordsNothingForAGoneRecord: a sweep working from a stale
// snapshot finds the record deleted and writes no option and no journal line.
func TestDiscoveryRecordsNothingForAGoneRecord(t *testing.T) {
	r := newSweepRig(t, found(discoverID))
	if err := r.f.store.Delete(r.s.ID); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}
	if err := r.sup.discover(context.Background(), r.s); err != nil {
		t.Fatalf("discover() = %v", err)
	}
	if r.calls != 0 {
		t.Errorf("the finder ran %d times for a record that is gone", r.calls)
	}
	if conv, event := r.journalled(t); conv == discoverID || event == journalDiscovered {
		t.Errorf("the journal recorded a discovery for a gone session: (%q, %q)", conv, event)
	}
}
