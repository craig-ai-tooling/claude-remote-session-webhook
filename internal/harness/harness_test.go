package harness

import (
	"reflect"
	"testing"
)

func TestOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		command string
		want    Name
	}{
		{"claude", Claude},
		{"/usr/local/bin/claude --x", Claude},
		{"codex", Codex},
		{"/home/x/sf-cli/bin/codex --flag", Codex},
		{"codex.js", Codex},
		{"bash", Other},
		{"", Other},
		{"   ", Other},
		{"CLAUDE", Other},
		{"env FOO=1 claude", Other},
	}
	for _, tc := range tests {
		t.Run(tc.command, func(t *testing.T) {
			t.Parallel()
			if got := Of(tc.command); got != tc.want {
				t.Fatalf("Of(%q) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
}

func TestLabel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name Name
		want string
	}{
		{Claude, "Claude Code"},
		{Codex, "Codex"},
		{Other, "Other"},
		{Name("bogus"), "Other"},
		{Name(""), "Other"},
	}
	for _, tc := range tests {
		if got := tc.name.Label(); got != tc.want {
			t.Errorf("Name(%q).Label() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestForTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          Name
		paneProcesses []string
		requiredFlags [][]string
		freshIDFlag   string
		resume        []string
		bracketed     bool
		stepped       bool
		remote        bool
		resumesByID   bool
	}{
		{Claude, nil, nil, "--session-id", []string{"--resume", "X"}, false, false, true, true},
		{Codex, []string{"codex", "node"},
			[][]string{{"--no-alt-screen"}, {"-c", "check_for_update_on_startup=false"}},
			"", []string{"resume", "X"}, true, true, false, true},
		{Other, nil, nil, "", nil, false, false, false, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.name), func(t *testing.T) {
			t.Parallel()
			s := For(tc.name)
			if s.Name != tc.name {
				t.Errorf("Name = %q, want %q", s.Name, tc.name)
			}
			if !reflect.DeepEqual(s.PaneProcesses, tc.paneProcesses) {
				t.Errorf("PaneProcesses = %#v, want %#v", s.PaneProcesses, tc.paneProcesses)
			}
			if !reflect.DeepEqual(s.RequiredFlags, tc.requiredFlags) {
				t.Errorf("RequiredFlags = %#v, want %#v", s.RequiredFlags, tc.requiredFlags)
			}
			if s.FreshIDFlag != tc.freshIDFlag {
				t.Errorf("FreshIDFlag = %q, want %q", s.FreshIDFlag, tc.freshIDFlag)
			}
			switch {
			case tc.resume == nil && s.ResumeArgs != nil:
				t.Errorf("ResumeArgs must be nil")
			case tc.resume != nil && s.ResumeArgs == nil:
				t.Errorf("ResumeArgs is nil")
			case tc.resume != nil:
				if got := s.ResumeArgs("X"); !reflect.DeepEqual(got, tc.resume) {
					t.Errorf("ResumeArgs(X) = %#v, want %#v", got, tc.resume)
				}
			}
			if s.BracketedPaste != tc.bracketed {
				t.Errorf("BracketedPaste = %v, want %v", s.BracketedPaste, tc.bracketed)
			}
			if s.SteppedQuit != tc.stepped {
				t.Errorf("SteppedQuit = %v, want %v", s.SteppedQuit, tc.stepped)
			}
			if s.RemoteControl != tc.remote {
				t.Errorf("RemoteControl = %v, want %v", s.RemoteControl, tc.remote)
			}
			if s.ResumesByID != tc.resumesByID {
				t.Errorf("ResumesByID = %v, want %v", s.ResumesByID, tc.resumesByID)
			}
		})
	}
}

func TestForUnknownIsOther(t *testing.T) {
	t.Parallel()
	s := For(Name("bogus"))
	if s.Name != Other {
		t.Fatalf("Name = %q, want %q", s.Name, Other)
	}
	if s.ResumeArgs != nil || s.PaneProcesses != nil || s.RequiredFlags != nil || s.ResumesByID {
		t.Fatalf("unknown name must carry Other's facts, got %#v", s)
	}
}
