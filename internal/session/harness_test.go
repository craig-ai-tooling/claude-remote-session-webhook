package session

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"testing"

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
