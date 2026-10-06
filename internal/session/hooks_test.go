package session

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
)

func TestCodexConversationFinderIsUsed(t *testing.T) {
	f, s := codexFixture(t)
	// The fixture's own manager, so the codex start command resolves.
	sup, err := NewSupervisor(f.mgr, audit.NewTo(&bytes.Buffer{}, func() time.Time { return f.now }))
	if err != nil {
		t.Fatalf("NewSupervisor() unexpected error: %v", err)
	}
	f.mgr.SetJournal(tempJournal(t))
	f.tmux.SetPaneCommand(s.TmuxName(), "codex")

	calls := 0
	sup.mgr.SetCodexConversationFinder(func(context.Context, Session) (string, error) {
		calls++
		return discoverID, nil
	})
	if err := sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v", err)
	}
	if calls != 1 {
		t.Errorf("finder calls = %d, want 1", calls)
	}
	if got := mustStored(t, f, s.ID).ConversationID; got != discoverID {
		t.Errorf("stored conversation = %q, want %q", got, discoverID)
	}

	// nil restores the host's own lookup, which a manager with no Codex home
	// answers with nothing.
	sup.mgr.SetCodexConversationFinder(nil)
	id, err := sup.mgr.findCodexConversation(context.Background(), *s)
	if err != nil || id != "" {
		t.Errorf("restored finder = %q, %v; want empty, nil", id, err)
	}
}

func TestTranscriptCheckerIsUsed(t *testing.T) {
	f := newManagerFixture(t)
	t.Setenv("HOME", t.TempDir())
	s, _ := mustCreate(t, f, f.request())

	f.mgr.SetTranscriptChecker(func(context.Context, Session, string) (bool, error) { return true, nil })
	ok, err := f.mgr.transcriptFor(context.Background(), *s, s.ConversationID)
	if err != nil || !ok {
		t.Fatalf("transcriptFor with a true checker = %v, %v; want true, nil", ok, err)
	}

	f.mgr.SetTranscriptChecker(nil)
	ok, err = f.mgr.transcriptFor(context.Background(), *s, s.ConversationID)
	if err != nil || ok {
		t.Fatalf("transcriptFor with the host check = %v, %v; want false, nil", ok, err)
	}
}

func TestTranscriptCheckerErrorDoesNotGiveUp(t *testing.T) {
	f := newManagerFixture(t)
	s := revivableSession(t, f)
	claudeDied(f, s)

	sup, _ := supervisorAt(t, f, f.now)
	sup.mgr.SetTranscriptChecker(func(context.Context, Session, string) (bool, error) {
		return false, errors.New("pod unreachable")
	})
	if err := sup.Sweep(context.Background()); err == nil {
		t.Fatal("Sweep() = nil, want the checker's failure")
	}
	got, ok := f.store.lookup(s.ID)
	if !ok {
		t.Fatal("the session vanished")
	}
	if got.State == StateFailed {
		t.Errorf("state = %q, an unanswered question must not end the session", got.State)
	}
	if got.ReviveAttempts != s.ReviveAttempts {
		t.Errorf("ReviveAttempts = %d, want %d", got.ReviveAttempts, s.ReviveAttempts)
	}
}

func TestPodRecordFindsByTmuxName(t *testing.T) {
	f := newManagerFixture(t)
	req := f.request()
	s, _ := mustCreate(t, f, req)

	rec, ok := f.mgr.PodRecord(s.TmuxName())
	if !ok {
		t.Fatal("PodRecord() found no record for a created session")
	}
	if rec.ID != s.ID || rec.Name != s.Name || rec.Owner != string(s.Owner) ||
		rec.WorkDir != s.WorkDir || rec.StartCommand != s.StartCommand ||
		rec.ConversationID != s.ConversationID {
		t.Errorf("PodRecord() = %+v, does not match session %+v", rec, *s)
	}
	if !rec.Deadline.Equal(s.AbsoluteDeadline()) || rec.LifetimeDisabled != s.LifetimeDisabled() {
		t.Errorf("PodRecord() lifetime = %v/%v, want %v/%v", rec.Deadline, rec.LifetimeDisabled, s.AbsoluteDeadline(), s.LifetimeDisabled())
	}
	if _, ok := f.mgr.PodRecord("crswd-nope"); ok {
		t.Error("PodRecord() found a record for an unknown name")
	}
}

func TestSuperviseLexicalWorkDir(t *testing.T) {
	f := newManagerFixture(t)
	s := revivableSession(t, f)
	claudeDied(f, s)
	// The daemon cannot see this directory; it only knows it is under a root.
	if err := os.RemoveAll(s.WorkDir); err != nil {
		t.Fatalf("remove the working directory: %v", err)
	}

	sup, _ := supervisorAt(t, f, f.now)
	sup.mgr.SetWorkDirResolver(LexicalWorkDir)
	sup.mgr.SetTranscriptChecker(func(context.Context, Session, string) (bool, error) { return true, nil })
	if err := sup.Sweep(context.Background()); err != nil {
		t.Logf("Sweep() = %v", err)
	}

	got, ok := f.store.lookup(s.ID)
	if !ok {
		t.Fatal("the session vanished")
	}
	if got.State == StateFailed {
		t.Errorf("state = %q, a lexically valid directory must not be given up on", got.State)
	}
}
