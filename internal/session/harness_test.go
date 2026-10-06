package session

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

const harnessTestConversation = "7f3a1b2c-4d5e-4f60-8a71-b2c3d4e5f607"

// The expected Claude strings are the ones the daemon typed before harnesses
// existed (SC-002). They are spelled out rather than built from the flag
// constants so a change to either side fails here.
func TestRenderStartPerHarness(t *testing.T) {
	t.Parallel()

	const codexFlags = "--no-alt-screen -c check_for_update_on_startup=false"

	tests := []struct {
		name     string
		template string
		resume   string
		id       string
		want     string
		wantErr  error
	}{
		{
			name:     "claude fresh with an id",
			template: "claude --dangerously-skip-permissions",
			id:       harnessTestConversation,
			want:     "claude --session-id " + harnessTestConversation + " --dangerously-skip-permissions",
		},
		{
			name:     "claude fresh without an id",
			template: "claude --dangerously-skip-permissions",
			want:     "claude --dangerously-skip-permissions",
		},
		{
			name:     "claude resume",
			template: "claude --dangerously-skip-permissions",
			resume:   harnessTestConversation,
			want:     "claude --resume " + harnessTestConversation + " --dangerously-skip-permissions",
		},
		{
			name:     "codex fresh carries the required flags and no id",
			template: "codex --dangerously-bypass-approvals-and-sandbox",
			id:       harnessTestConversation,
			want:     "codex " + codexFlags + " --dangerously-bypass-approvals-and-sandbox",
		},
		{
			name:     "codex resume puts resume directly after the binary",
			template: "codex --dangerously-bypass-approvals-and-sandbox",
			resume:   harnessTestConversation,
			want:     "codex resume " + harnessTestConversation + " " + codexFlags + " --dangerously-bypass-approvals-and-sandbox",
		},
		{
			name:     "other fresh is untouched",
			template: "bash",
			id:       harnessTestConversation,
			want:     "bash",
		},
		{
			name:     "other refuses resume",
			template: "bash",
			resume:   harnessTestConversation,
			wantErr:  ErrInvalidResume,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newManagerFixture(t)
			got, err := f.mgr.resumeFlagged(tt.template, tt.resume, tt.id)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("resumeFlagged() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resumeFlagged() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("resumeFlagged() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithRequiredFlagsSkipsPresentGroups(t *testing.T) {
	t.Parallel()

	groups := harness.For(harness.Codex).RequiredFlags
	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"neither present", "codex --x", "codex --no-alt-screen -c check_for_update_on_startup=false --x"},
		{"first present", "codex --no-alt-screen --x", "codex -c check_for_update_on_startup=false --no-alt-screen --x"},
		{"second present", "codex -c check_for_update_on_startup=false --x", "codex --no-alt-screen -c check_for_update_on_startup=false --x"},
		{"both present", "codex --no-alt-screen -c check_for_update_on_startup=false --x", "codex --no-alt-screen -c check_for_update_on_startup=false --x"},
		{"bare binary gets both groups", "codex", "codex --no-alt-screen -c check_for_update_on_startup=false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := withRequiredFlags(tt.template, groups); got != tt.want {
				t.Errorf("withRequiredFlags(%q) = %q, want %q", tt.template, got, tt.want)
			}
		})
	}

	if got := withRequiredFlags("codex --x", nil); got != "codex --x" {
		t.Errorf("withRequiredFlags with no groups = %q, want the template unchanged", got)
	}
}

func TestWithRequiredFlagsIgnoresTokensAfterDoubleDash(t *testing.T) {
	t.Parallel()

	groups := harness.For(harness.Codex).RequiredFlags
	got := withRequiredFlags("codex -- --no-alt-screen", groups)
	want := "codex --no-alt-screen -c check_for_update_on_startup=false -- --no-alt-screen"
	if got != want {
		t.Errorf("withRequiredFlags() = %q, want %q", got, want)
	}
}

func TestPaneProcesses(t *testing.T) {
	t.Parallel()

	tests := []struct{ template, want string }{
		{"claude --dangerously-skip-permissions", "claude"},
		{"codex --x", "codex|node"},
		{"/abs/codex --x", "codex|node"},
		{"bash", "bash"},
	}
	for _, tt := range tests {
		if got := paneProcesses(tt.template); got != tt.want {
			t.Errorf("paneProcesses(%q) = %q, want %q", tt.template, got, tt.want)
		}
	}
}

// The caller, not only the callee: a Codex create must write the set to tmux.
func TestCreateCodexWritesBinarySet(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	f.mgr.SetStartCommands(config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: claudeStartCommand,
		"codex":                        "codex --dangerously-bypass-approvals-and-sandbox",
	}))
	req := f.request()
	req.StartCommand = "codex"
	s, _ := mustCreate(t, f, req)

	got, ok := f.tmux.Option("crswd-"+s.ID, tmuxctl.OptionBinary)
	if !ok || got != "codex|node" {
		t.Errorf("%s = %q (set %v), want %q", tmuxctl.OptionBinary, got, ok, "codex|node")
	}
	if s.ConversationID != "" {
		t.Errorf("a Codex create minted conversation %q, want none", s.ConversationID)
	}
}

func TestClaudeFlagConstantsUnchanged(t *testing.T) {
	t.Parallel()

	if SessionIDFlag != harness.For(harness.Claude).FreshIDFlag {
		t.Errorf("SessionIDFlag = %q, harness says %q", SessionIDFlag, harness.For(harness.Claude).FreshIDFlag)
	}
	if got := harness.For(harness.Claude).ResumeArgs("x"); len(got) != 2 || got[0] != ResumeOneFlag {
		t.Errorf("Claude ResumeArgs = %v, want [%s x]", got, ResumeOneFlag)
	}
}

// codexFixture is a manager that knows a codex entry, and a session started
// under it.
func codexFixture(t *testing.T) (managerFixture, *Session) {
	t.Helper()

	f := newManagerFixture(t)
	f.mgr.SetStartCommands(config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: claudeStartCommand,
		"codex":                        "codex --dangerously-bypass-approvals-and-sandbox",
	}))
	req := f.request()
	req.StartCommand = "codex"
	s, _ := mustCreate(t, f, req)
	return f, s
}

func opsOf(calls []tmuxctl.Call) []tmuxctl.Op {
	ops := make([]tmuxctl.Op, len(calls))
	for i, c := range calls {
		ops[i] = c.Op
	}
	return ops
}

func TestPromptCodexIsBracketed(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	before := len(f.tmux.Calls())

	if err := f.mgr.Prompt(context.Background(), *s, "say hi"); err != nil {
		t.Fatalf("Prompt() unexpected error: %v", err)
	}

	got := f.tmux.Calls()[before:]
	want := []tmuxctl.Op{tmuxctl.OpPasteBracketed, tmuxctl.OpPasteBracketed, tmuxctl.OpSendKeys}
	if !slices.Equal(opsOf(got), want) {
		t.Fatalf("Prompt() ran %v, want %v", opsOf(got), want)
	}
	if !slices.Contains(got[1].Argv, "-p") {
		t.Errorf("paste argv %q lacks -p", got[1].Argv)
	}
	if last := got[2].Argv[len(got[2].Argv)-1]; last != "Enter" {
		t.Errorf("send-keys ends in %q, want Enter", last)
	}
}

func TestPromptClaudeUnchanged(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())
	before := len(f.tmux.Calls())

	if err := f.mgr.Prompt(context.Background(), *s, "say hi"); err != nil {
		t.Fatalf("Prompt() unexpected error: %v", err)
	}
	for _, c := range f.tmux.Calls()[before:] {
		if c.Op == tmuxctl.OpPasteBracketed || slices.Contains(c.Argv, "-p") {
			t.Errorf("a Claude prompt used bracketed paste: %s %q", c.Op, c.Argv)
		}
	}
}

func TestCompactCodexSubmitsWithEnter(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	before := len(f.tmux.Calls())

	if err := f.mgr.Compact(context.Background(), *s); err != nil {
		t.Fatalf("Compact() unexpected error: %v", err)
	}

	got := f.tmux.Calls()[before:]
	want := []tmuxctl.Op{tmuxctl.OpPasteBracketed, tmuxctl.OpPasteBracketed, tmuxctl.OpSendKeys}
	if !slices.Equal(opsOf(got), want) {
		t.Fatalf("Compact() ran %v, want %v", opsOf(got), want)
	}
	if !bytes.Equal(got[0].Stdin, []byte("/compact")) {
		t.Errorf("stdin = %q, want %q with no newline", got[0].Stdin, "/compact")
	}
	if last := got[2].Argv[len(got[2].Argv)-1]; last != "Enter" {
		t.Errorf("send-keys ends in %q, want Enter", last)
	}
}

func TestCompactClaudeUnchanged(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())
	before := len(f.tmux.Calls())

	if err := f.mgr.Compact(context.Background(), *s); err != nil {
		t.Fatalf("Compact() unexpected error: %v", err)
	}

	got := f.tmux.Calls()[before:]
	if want := []tmuxctl.Op{tmuxctl.OpPaste, tmuxctl.OpPaste}; !slices.Equal(opsOf(got), want) {
		t.Fatalf("Compact() ran %v, want %v", opsOf(got), want)
	}
	if !bytes.Equal(got[0].Stdin, []byte("/compact\n")) {
		t.Errorf("stdin = %q, want %q", got[0].Stdin, "/compact\n")
	}
}

// Spec 018's Type is bracketed for every harness. Claude's own BracketedPaste
// is false, so routing Type through paste would break multi-line input.
func TestTypeStaysBracketedForClaude(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())
	before := len(f.tmux.Calls())

	if err := f.mgr.Type(context.Background(), *s, "a\nb", true); err != nil {
		t.Fatalf("Type() unexpected error: %v", err)
	}
	var bracketed bool
	for _, c := range f.tmux.Calls()[before:] {
		bracketed = bracketed || (c.Op == tmuxctl.OpPasteBracketed && slices.Contains(c.Argv, "-p"))
	}
	if !bracketed {
		t.Error("Type on a Claude session did not use paste-buffer -p")
	}
}

func TestSetModeRefusesCodex(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	before := len(f.tmux.Calls())

	for _, mode := range []Mode{ModeLocal, ModeRemote} {
		if _, err := f.mgr.SetMode(context.Background(), *s, mode); !errors.Is(err, ErrModeUnavailable) {
			t.Errorf("SetMode(%s) on Codex error = %v, want ErrModeUnavailable", mode, err)
		}
	}
	if got := f.tmux.Calls()[before:]; len(got) != 0 {
		t.Errorf("a refused Codex mode change touched the pane: %v", opsOf(got))
	}
}

// noSleep stands in for the real wait between interrupts. A test about the
// order of keystrokes has no use for the 1.5 seconds Codex needs to exit.
func noSleep(context.Context, time.Duration) error { return nil }

// interruptsAndStarts splits the send-keys calls into bare Ctrl-C presses and
// everything else, so a test can count the interrupts typed and whether a
// start line followed.
func interruptsAndStarts(calls []tmuxctl.Call) (interrupts, others int) {
	for _, c := range calls {
		if c.Op != tmuxctl.OpSendKeys {
			continue
		}
		if c.Argv[len(c.Argv)-1] == interruptKey {
			interrupts++
		} else {
			others++
		}
	}
	return interrupts, others
}

func TestRestartCodexQuitsThenTypes(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	f.mgr.sleep = noSleep
	f.tmux.SetPaneCommand(s.TmuxName(), "codex")
	f.tmux.QuitAfterInterrupts(s.TmuxName(), 3)
	s.ConversationID = harnessTestConversation
	before := len(f.tmux.Calls())

	if err := f.mgr.restartInto(context.Background(), *s); err != nil {
		t.Fatalf("restartInto() unexpected error: %v", err)
	}

	got := f.tmux.Calls()[before:]
	interrupts, others := interruptsAndStarts(got)
	if interrupts != 3 || others != 1 {
		t.Fatalf("restartInto() typed %d interrupts and %d other lines, want 3 and 1", interrupts, others)
	}
	last := got[len(got)-1]
	if last.Op != tmuxctl.OpSendKeys || !slices.Contains(last.Argv, "codex resume "+harnessTestConversation+" --no-alt-screen -c check_for_update_on_startup=false --dangerously-bypass-approvals-and-sandbox") {
		t.Errorf("the last call was %s %q, want the resume line", last.Op, last.Argv)
	}
}

func TestRestartCodexNeverTypesWhenStillRunning(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	f.mgr.sleep = noSleep
	f.tmux.SetPaneCommand(s.TmuxName(), "node")
	s.ConversationID = harnessTestConversation
	before := len(f.tmux.Calls())

	err := f.mgr.restartInto(context.Background(), *s)
	if !errors.Is(err, ErrQuitUnconfirmed) {
		t.Fatalf("restartInto() error = %v, want ErrQuitUnconfirmed", err)
	}

	interrupts, others := interruptsAndStarts(f.tmux.Calls()[before:])
	if interrupts != steppedQuitPresses || others != 0 {
		t.Errorf("restartInto() typed %d interrupts and %d other lines, want %d and 0", interrupts, others, steppedQuitPresses)
	}
}

func TestRestartCodexStopsWhenTheSessionVanishes(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	f.tmux.SetPaneCommand(s.TmuxName(), "node")
	f.mgr.sleep = func(context.Context, time.Duration) error {
		f.tmux.Vanish(s.TmuxName())
		return nil
	}

	if err := f.mgr.quitStepped(context.Background(), *s); !errors.Is(err, ErrSessionDead) {
		t.Fatalf("quitStepped() error = %v, want ErrSessionDead", err)
	}
}

func TestRestartCodexReturnsTheContextError(t *testing.T) {
	t.Parallel()

	f, s := codexFixture(t)
	f.tmux.SetPaneCommand(s.TmuxName(), "node")
	f.mgr.sleep = func(context.Context, time.Duration) error { return context.Canceled }

	if err := f.mgr.quitStepped(context.Background(), *s); !errors.Is(err, context.Canceled) {
		t.Fatalf("quitStepped() error = %v, want context.Canceled", err)
	}
}

func TestRestartClaudeUnchanged(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())
	f.mgr.sleep = func(context.Context, time.Duration) error {
		t.Error("a Claude restart waited between interrupts")
		return nil
	}
	before := len(f.tmux.Calls())

	if err := f.mgr.restartInto(context.Background(), *s); err != nil {
		t.Fatalf("restartInto() unexpected error: %v", err)
	}

	got := f.tmux.Calls()[before:]
	if len(got) < 2 {
		t.Fatalf("restartInto() made %d calls, want the interrupt then the start line", len(got))
	}
	if want := []string{interruptKey, interruptKey}; !slices.Equal(got[0].Argv[len(got[0].Argv)-2:], want) {
		t.Errorf("first call argv %q does not end in one send of two interrupts", got[0].Argv)
	}
}

// continueFixture is a Codex session with a transcript on disk for the
// conversation it is about to be moved to. It sets HOME, so its callers cannot
// run in parallel.
func continueFixture(t *testing.T) (managerFixture, *Session, *Journal) {
	t.Helper()

	f, s := codexFixture(t)
	f.mgr.sleep = noSleep
	j := tempJournal(t)
	f.mgr.SetJournal(j)
	home := t.TempDir()
	f.mgr.SetCodexHome(home)
	plantCodexRollout(t, home, harnessTestConversation, s.WorkDir)
	return f, s, j
}

func TestContinueCodexFailedQuitChangesNothing(t *testing.T) {
	f, s, j := continueFixture(t)
	f.tmux.SetPaneCommand(s.TmuxName(), "node")
	if err := f.tmux.SetOption(context.Background(), s.TmuxName(), tmuxctl.OptionConversation, "before"); err != nil {
		t.Fatalf("SetOption() unexpected error: %v", err)
	}
	wasID := s.ConversationID
	records, _, err := j.Replay()
	if err != nil {
		t.Fatalf("Replay() unexpected error: %v", err)
	}

	_, err = f.mgr.Continue(context.Background(), *s, harnessTestConversation)
	if !errors.Is(err, ErrQuitUnconfirmed) {
		t.Fatalf("Continue() error = %v, want ErrQuitUnconfirmed", err)
	}

	stored, getErr := f.store.Get(s.ID, s.Owner)
	if getErr != nil {
		t.Fatalf("Get() unexpected error: %v", getErr)
	}
	if stored.ConversationID != wasID {
		t.Errorf("store ConversationID = %q, want %q", stored.ConversationID, wasID)
	}
	infos, err := f.tmux.List(context.Background())
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	for _, info := range infos {
		if info.Name == s.TmuxName() && info.ConversationID != "before" {
			t.Errorf("tmux option = %q, want %q", info.ConversationID, "before")
		}
	}
	after, _, err := j.Replay()
	if err != nil {
		t.Fatalf("Replay() unexpected error: %v", err)
	}
	if len(after) != len(records) {
		t.Errorf("journal holds %d records, want %d", len(after), len(records))
	}
}

func TestContinueCodexPersistsAfterQuit(t *testing.T) {
	f, s, j := continueFixture(t)
	f.tmux.SetPaneCommand(s.TmuxName(), "codex")
	f.tmux.QuitAfterInterrupts(s.TmuxName(), 2)
	before := len(f.tmux.Calls())

	got, err := f.mgr.Continue(context.Background(), *s, harnessTestConversation)
	if err != nil {
		t.Fatalf("Continue() unexpected error: %v", err)
	}

	if got.ConversationID != harnessTestConversation {
		t.Errorf("Continue() ConversationID = %q, want %q", got.ConversationID, harnessTestConversation)
	}
	stored, err := f.store.Get(s.ID, s.Owner)
	if err != nil || stored.ConversationID != harnessTestConversation {
		t.Errorf("store ConversationID = %q (%v), want %q", stored.ConversationID, err, harnessTestConversation)
	}
	interrupts, others := interruptsAndStarts(f.tmux.Calls()[before:])
	if interrupts != 2 || others != 1 {
		t.Errorf("Continue() typed %d interrupts and %d other lines, want 2 and 1", interrupts, others)
	}
	records, _, err := j.Replay()
	if err != nil {
		t.Fatalf("Replay() unexpected error: %v", err)
	}
	if len(records) == 0 {
		t.Error("the journal holds no record of the continue")
	}
}

func TestStartCommandLineFor(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	f.mgr.SetStartCommands(config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: claudeStartCommand,
		"codex":                        "codex --dangerously-bypass-approvals-and-sandbox",
	}))

	t.Run("codex renders the fresh line with the placeholder standing", func(t *testing.T) {
		t.Parallel()

		got, err := f.mgr.StartCommandLineFor("codex", "")
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		const want = "codex --no-alt-screen -c check_for_update_on_startup=false --dangerously-bypass-approvals-and-sandbox"
		if got != want {
			t.Errorf("line = %q, want %q", got, want)
		}
	})
	t.Run("a name is substituted", func(t *testing.T) {
		t.Parallel()

		got, err := f.mgr.StartCommandLineFor("codex", "my-session")
		if err != nil || !strings.HasPrefix(got, "codex --no-alt-screen") {
			t.Errorf("line = %q, err = %v", got, err)
		}
	})
	t.Run("an unknown name is refused", func(t *testing.T) {
		t.Parallel()

		if _, err := f.mgr.StartCommandLineFor("nope", ""); !errors.Is(err, ErrUnknownStartCommand) {
			t.Errorf("err = %v, want ErrUnknownStartCommand", err)
		}
	})
}

func TestModeTargetResolvingToCodexIsRefused(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	f.mgr.SetStartCommands(config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: claudeStartCommand,
		"rc":                           "codex --dangerously-bypass-approvals-and-sandbox",
	}))
	f.mgr.SetRemoteControlCommand("rc")
	s, _ := mustCreate(t, f, f.request())
	before := len(f.tmux.Calls())

	if _, err := f.mgr.SetMode(context.Background(), *s, ModeRemote); !errors.Is(err, ErrModeUnavailable) {
		t.Errorf("SetMode(remote) onto a Codex command error = %v, want ErrModeUnavailable", err)
	}
	if got := f.tmux.Calls()[before:]; len(got) != 0 {
		t.Errorf("a refused mode change touched the pane: %v", opsOf(got))
	}
	if _, err := f.mgr.RemoteStartCommand(); !errors.Is(err, ErrModeUnavailable) {
		t.Errorf("RemoteStartCommand() = %v, want ErrModeUnavailable", err)
	}
}

// blockedWithin reports whether done stays open for a short while. The wait is
// only ever a way to let a goroutine that is going to be refused the lock get
// as far as the lock; a goroutine that is not refused finishes in microseconds
// on the fake host.
func blockedWithin(done <-chan struct{}) bool {
	select {
	case <-done:
		return false
	case <-time.After(150 * time.Millisecond):
		return true
	}
}

// TestContinueExcludesTypingWhileTheShellIsVerified is 019 core review #8:
// Continue confirms the shell and then writes the store, the option and the
// journal before typing the start line. A Type that lands in that window feeds
// a shell Continue believes is idle.
func TestContinueExcludesTypingWhileTheShellIsVerified(t *testing.T) {
	f, s, _ := continueFixture(t)
	f.tmux.SetPaneCommand(s.TmuxName(), "codex")
	f.tmux.QuitAfterInterrupts(s.TmuxName(), 2)

	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.mgr.sleep = func(context.Context, time.Duration) error {
		once.Do(func() { close(entered) })
		<-release
		return nil
	}
	before := len(f.tmux.Calls())

	contDone := make(chan error, 1)
	go func() {
		_, err := f.mgr.Continue(context.Background(), *s, harnessTestConversation)
		contDone <- err
	}()
	<-entered

	typeDone := make(chan struct{})
	var typeErr error
	go func() {
		defer close(typeDone)
		typeErr = f.mgr.Type(context.Background(), *s, "hello", true)
	}()
	if !blockedWithin(typeDone) {
		t.Error("Type completed while Continue held the session")
	}
	close(release)
	if err := <-contDone; err != nil {
		t.Fatalf("Continue() unexpected error: %v", err)
	}
	<-typeDone
	if typeErr != nil {
		t.Fatalf("Type() unexpected error: %v", typeErr)
	}

	calls := f.tmux.Calls()[before:]
	start, paste := -1, -1
	for i, c := range calls {
		switch {
		case c.Op == tmuxctl.OpSendKeys && start < 0 && strings.Contains(strings.Join(c.Argv, " "), "--dangerously-bypass"):
			start = i
		case c.Op == tmuxctl.OpPasteBracketed && paste < 0:
			paste = i
		}
	}
	if start < 0 || paste < 0 || paste < start {
		t.Errorf("the typed text (call %d) did not wait for the start line (call %d): %v", paste, start, opsOf(calls))
	}
}

// TestSessionOperationsWaitForTheLifecycleLock holds the one per-session lock
// and asks each operation to run. None may complete until it is released.
func TestSessionOperationsWaitForTheLifecycleLock(t *testing.T) {
	ops := map[string]func(f managerFixture, s Session) error{
		"Type": func(f managerFixture, s Session) error {
			return f.mgr.Type(context.Background(), s, "x", false)
		},
		"PressKey": func(f managerFixture, s Session) error {
			return f.mgr.PressKey(context.Background(), s, KeyEnter)
		},
		"Prompt": func(f managerFixture, s Session) error {
			return f.mgr.Prompt(context.Background(), s, "x")
		},
		"Compact": func(f managerFixture, s Session) error {
			return f.mgr.Compact(context.Background(), s)
		},
		"SetMode": func(f managerFixture, s Session) error {
			_, err := f.mgr.SetMode(context.Background(), s, ModeRemote)
			return err
		},
		"Destroy": func(f managerFixture, s Session) error {
			return f.mgr.Destroy(context.Background(), s)
		},
		"Continue": func(f managerFixture, s Session) error {
			_, err := f.mgr.Continue(context.Background(), s, harnessTestConversation)
			return err
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			f, s, _ := continueFixture(t)
			f.mgr.SetStartCommands(config.NewStartCommands(map[string]string{
				config.DefaultStartCommandName: claudeStartCommand,
				"rc":                           claudeStartCommand + " --remote-control",
				"codex":                        "codex --dangerously-bypass-approvals-and-sandbox",
			}))
			f.mgr.SetRemoteControlCommand("rc")
			sess := *s
			if name == "SetMode" {
				// SetMode refuses a Codex session before it reaches the lock.
				sess.StartCommand = config.DefaultStartCommandName
			}
			f.tmux.SetPaneCommand(sess.TmuxName(), "codex")
			f.tmux.QuitAfterInterrupts(sess.TmuxName(), 1)

			unlock := f.mgr.lockSession(sess.ID)
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = op(f, sess)
			}()
			held := blockedWithin(done)
			unlock()
			<-done
			if !held {
				t.Errorf("%s ran while the session lock was held", name)
			}
		})
	}
}
