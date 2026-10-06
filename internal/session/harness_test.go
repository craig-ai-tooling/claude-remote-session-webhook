package session

import (
	"errors"
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
