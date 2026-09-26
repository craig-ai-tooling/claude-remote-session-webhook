// Spec 017 FR-012 and SC-002: the journal carries a session's name, and a replay
// that has one renders the start command.
//
// The failure these pin is the one the VM's reboot of 2026-09-22 produced. The
// deployed start command ends in `--remote-control {name}`, the journal had no
// name to give it back, and every one of the eight sessions failed at "render the
// start command" three times over before the supervisor stopped trying.
//
// This file reads the journal as raw bytes and touches only exported and
// long-standing names, so that it compiles against a build from before the field
// existed. That is what lets the same file be run against the previous code and
// fail there for the right reason, which is a missing name and not a build error.
package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
)

// nameTemplate is the deployed shape: the name is the last argument, after a flag
// that takes it.
const nameTemplate = "claude --dangerously-skip-permissions --remote-control {name}"

// useNameTemplate gives a Manager the command set the operator's configuration
// would have. restarted() and supervisorAt() each build a fresh Manager with none,
// which resolves "rc" to ErrUnknownStartCommand on any build — so a test that
// forgets this fails for a reason that has nothing to do with the name.
func useNameTemplate(m *Manager) {
	m.SetStartCommands(config.NewStartCommands(map[string]string{
		"default": claudeStartCommand,
		"rc":      nameTemplate,
	}))
}

// plantTranscript puts a conversation under home, which the supervisor requires
// before it will resume anything (FR-014). The caller has pointed HOME at home,
// so a test using it cannot be parallel.
func plantTranscript(t *testing.T, home string, s Session) {
	t.Helper()

	project := filepath.Join(home, ".claude", "projects", projectDirFor(s.WorkDir))
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatalf("plant a project directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, s.ConversationID+conversationFileSuffix), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("plant a transcript: %v", err)
	}
}

// namedSession creates a session called refactor-auth on the `rc` start command
// and returns it with its bearer token, both of which the journal must not confuse.
func namedSession(t *testing.T, f managerFixture) (Session, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	useNameTemplate(f.mgr)

	req := f.request()
	req.StartCommand = "rc"
	s, token := mustCreate(t, f, req)
	plantTranscript(t, home, *s)
	return *s, token
}

// supervisorOn is a supervisor whose Manager writes to j and has the command set,
// which supervisorAt does not give it.
func supervisorOn(t *testing.T, f managerFixture, j *Journal) (*Supervisor, *bytes.Buffer) {
	t.Helper()

	sink := &bytes.Buffer{}
	at := f.now
	sup, err := NewSupervisor(f.managerAt(t, f.store, at), audit.NewTo(sink, func() time.Time { return at }))
	if err != nil {
		t.Fatalf("NewSupervisor() = %v", err)
	}
	sup.mgr.SetJournal(j)
	useNameTemplate(sup.mgr)
	return sup, sink
}

// journalLines reads a journal as the raw JSON objects it holds.
func journalLines(t *testing.T, j *Journal) []map[string]any {
	t.Helper()

	raw, err := os.ReadFile(j.Path())
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("a journal line is not JSON: %v: %s", err, line)
		}
		out = append(out, rec)
	}
	return out
}

// TestTheCreateRecordCarriesTheSessionName is SC-002's first half: a session
// record written at create that lacks the name fails here.
func TestTheCreateRecordCarriesTheSessionName(t *testing.T) {
	f := newManagerFixture(t)
	j := tempJournal(t)
	f.mgr.SetJournal(j)

	s, _ := namedSession(t, f)

	lines := journalLines(t, j)
	if len(lines) != 1 {
		t.Fatalf("a create wrote %d journal records, want 1: %v", len(lines), lines)
	}
	if lines[0]["event"] != "created" {
		t.Fatalf("the first record is %v, want created", lines[0]["event"])
	}
	if got := lines[0]["name"]; got != s.Name {
		t.Errorf("the create record's name = %v, want %q: a revival cannot render its start command without it", got, s.Name)
	}
}

// TestAReplayedSessionRendersItsStartCommand is SC-002's second half: after a
// restart that finds the host without the session, the supervisor types the start
// command, and it carries the name. It fails against a build whose journal has no
// name, which is the build that failed 8 of 8 on 2026-09-22.
func TestAReplayedSessionRendersItsStartCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		// revivedFirst writes a later record for the session before the restart.
		// The revive path builds its own record, so a name only the create record
		// carried would be gone by the time the daemon came back after one death.
		revivedFirst bool
	}{
		{name: "from the record written at create"},
		{name: "from a record written by an earlier revival", revivedFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newManagerFixture(t)
			j := tempJournal(t)
			f.mgr.SetJournal(j)
			s, _ := namedSession(t, f)

			if tc.revivedFirst {
				claudeDied(f, s)
				before := len(typedInto(f, s))
				sup, _ := supervisorOn(t, f, j)
				if err := sup.Sweep(context.Background()); err != nil {
					t.Fatalf("the first revival: Sweep() = %v", err)
				}
				if len(typedInto(f, s)) == before {
					t.Fatal("the first revival typed nothing, so this case is not testing a later record")
				}
			}

			// The reboot: the host has nothing, the daemon has nothing, the file
			// has everything it was told.
			f.tmux.Vanish(s.TmuxName())
			restarted(t, &f, j, f.roots(), f.now)
			useNameTemplate(f.mgr)

			if _, _, err := f.mgr.ReplayJournal(context.Background()); err != nil {
				t.Fatalf("ReplayJournal() = %v", err)
			}
			got, ok := f.store.lookup(s.ID)
			if !ok {
				t.Fatal("the session was not put back")
			}
			if got.Name != s.Name {
				// Not fatal, so the sweep below also reports the failure this
				// causes: the render error the 2026-09-22 audit log showed.
				t.Errorf("the replayed session's Name = %q, want %q", got.Name, s.Name)
			}

			before := len(typedInto(f, s))
			sup, sink := supervisorOn(t, f, j)
			if err := sup.Sweep(context.Background()); err != nil {
				t.Fatalf("Sweep() = %v; a named session must revive after a reboot", err)
			}

			typed := typedInto(f, s)
			if len(typed) != before+1 {
				t.Fatalf("the revival typed %d new line(s), want 1: %q", len(typed)-before, typed)
			}
			line := typed[len(typed)-1]
			// Against the create's own line, which also names the session, this
			// is what says the new one is a revival.
			for _, want := range []string{ResumeOneFlag + " " + s.ConversationID, "--remote-control " + s.Name} {
				if !strings.Contains(line, want) {
					t.Errorf("the revival typed %q, want it to contain %q", line, want)
				}
			}
			if strings.Contains(line, config.StartCommandNamePlaceholder) {
				t.Errorf("the revival typed %q with the placeholder still in it", line)
			}
			for _, action := range trailActions(t, sink) {
				if action == string(audit.ActionSupervisorFailed) {
					t.Errorf("the supervisor gave up on the session; trail = %v", trailActions(t, sink))
				}
			}
		})
	}
}

// oldFormatRecord is a line as this daemon wrote it before the name existed:
// every field but that one, in the shape contracts/session-journal.md gave.
func oldFormatRecord(s Session, start string) string {
	at := s.CreatedAt.UTC().Format(time.RFC3339Nano)
	return fmt.Sprintf(
		`{"v":1,"at":%q,"id":%q,"event":"created","owner":"operator","conversation":%q,"workdir":%q,"start":%q,"created":%q,"attempts":0}`+"\n",
		at, s.ID, s.ConversationID, s.WorkDir, start, at)
}

// nameless replays a journal holding one record in the old format, for a session
// the host has lost and whose start command is the one named, and returns the
// fixture, the session and the replay's stats.
func nameless(t *testing.T, start string) (managerFixture, Session, ReplayStats) {
	t.Helper()

	f := newManagerFixture(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	s, _ := mustCreate(t, f, f.request())
	plantTranscript(t, home, *s)

	j := tempJournal(t)
	if err := os.WriteFile(j.Path(), []byte(oldFormatRecord(*s, start)), 0o600); err != nil {
		t.Fatalf("write an old-format journal: %v", err)
	}
	f.tmux.Vanish(s.TmuxName())
	restarted(t, &f, j, f.roots(), f.now)

	_, stats, err := f.mgr.ReplayJournal(context.Background())
	if err != nil {
		t.Fatalf("ReplayJournal() = %v", err)
	}
	return f, *s, stats
}

// TestARecordWithNoNameReplaysAsItAlwaysDid is the compatibility half. A journal
// file already on a host holds records with no name field, and they must read
// exactly as they did: understood, not skipped, not discarded, and a session on
// the default command revives with the line it always typed.
func TestARecordWithNoNameReplaysAsItAlwaysDid(t *testing.T) {
	f, s, stats := nameless(t, "")

	if stats.Records != 1 || stats.SkippedVersion != 0 || stats.Discarded != 0 {
		t.Fatalf("stats = %+v, want the old-format record read and neither skipped nor discarded", stats)
	}
	got, ok := f.store.lookup(s.ID)
	if !ok {
		t.Fatal("a record with no name was not put back")
	}
	if got.Name != "" {
		t.Errorf("Name = %q, want empty: nothing was written that could fill it", got.Name)
	}

	before := len(typedInto(f, s))
	sup, _ := supervisorOn(t, f, tempJournal(t))
	if err := sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v; the default command has no {name} and must render without one", err)
	}
	typed := typedInto(f, s)
	if len(typed) != before+1 {
		t.Fatalf("the revival typed %d new line(s), want 1: %q", len(typed)-before, typed)
	}
	line := typed[len(typed)-1]
	if !strings.HasPrefix(line, "claude ") || !strings.Contains(line, ResumeOneFlag+" "+s.ConversationID) ||
		strings.Contains(line, "--remote-control") {
		t.Errorf("the revival typed %q, want the default command resuming %s and nothing about a name", line, s.ConversationID)
	}
}

// TestARecordWithNoNameStillFailsClosedUnderANameTemplate is what the field does
// not change. A session whose record never held a name, revived under a command
// that needs one, is refused rather than given a line with a hole in it — and a
// name is never invented for it.
func TestARecordWithNoNameStillFailsClosedUnderANameTemplate(t *testing.T) {
	f, s, _ := nameless(t, "rc")

	before := len(typedInto(f, s))
	sup, _ := supervisorOn(t, f, tempJournal(t))
	err := sup.Sweep(context.Background())
	if !errors.Is(err, config.ErrStartCommandName) {
		t.Fatalf("Sweep() = %v, want the refusal to render without a name", err)
	}
	if after := len(typedInto(f, s)); after != before {
		t.Errorf("a refused revival still typed %d line(s) into the shell", after-before)
	}
}

// TestTheJournalAddsNoKeyBeyondTheKnownSet is FR-014 for the field this change
// adds. crswd re-mints every bearer on a restart and a journal must never hold a
// token or its hash, so the name is the only thing that was added: the set of keys
// on every record a session's life writes is closed, and a session's bearer token
// and its hash appear in none of them.
//
// The version stays 1 for the same reason. A previous build skips a record whose
// v it does not know, so a bump would make a rollback drop every session.
func TestTheJournalAddsNoKeyBeyondTheKnownSet(t *testing.T) {
	f := newManagerFixture(t)
	j := tempJournal(t)
	f.mgr.SetJournal(j)
	s, token := namedSession(t, f)

	// A create, a revival and a destroy: every kind of record a session writes.
	claudeDied(f, s)
	sup, _ := supervisorOn(t, f, j)
	if err := sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v", err)
	}
	if err := f.mgr.Destroy(context.Background(), s); err != nil {
		t.Fatalf("Destroy() = %v", err)
	}

	known := map[string]bool{
		"v": true, "at": true, "id": true, "event": true, "name": true, "owner": true,
		"conversation": true, "workdir": true, "start": true, "lifetime": true,
		"created": true, "attempts": true,
	}
	raw, err := os.ReadFile(j.Path())
	if err != nil {
		t.Fatalf("read the journal: %v", err)
	}
	if bytes.Contains(raw, []byte(token)) {
		t.Error("the journal contains the session's bearer token")
	}
	hash := fmt.Sprintf("%x", s.TokenHash)
	if bytes.Contains(raw, []byte(hash)) {
		t.Error("the journal contains the hash of the session's bearer token")
	}

	lines := journalLines(t, j)
	if len(lines) < 3 {
		t.Fatalf("wrote %d records, want a create, a revival and an end at least", len(lines))
	}
	for _, rec := range lines {
		if rec["v"] != float64(1) {
			t.Errorf("a record carries v = %v; it must stay 1 or a rolled-back daemon skips it", rec["v"])
		}
		for key := range rec {
			if !known[key] {
				t.Errorf("a journal record carries the key %q, which is not one this format has ever held", key)
			}
		}
	}
}
