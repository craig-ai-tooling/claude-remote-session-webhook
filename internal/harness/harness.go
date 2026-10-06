// Package harness names the runtime a configured start command runs and carries
// what differs between runtimes as data. The runtime is derived from the
// command every time it is needed and is never stored, so it cannot disagree
// with the configuration it came from (spec 019, D1).
//
// This is the only place a binary name is compared to "claude" or "codex".
// Imports are stdlib only: every other package asks this one, never the reverse.
package harness

import (
	"path/filepath"
	"strings"
)

// Name is a runtime.
type Name string

const (
	Claude Name = "claude"
	Codex  Name = "codex"
	Other  Name = "other"
)

// Of is the harness a configured command line runs: the base name of its first
// whitespace-separated token, with one trailing ".js" removed. The match is
// case-sensitive, like the session package's start-binary rule, and a command
// that begins with a wrapper (env FOO=1 claude) is Other.
func Of(command string) Name {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return Other
	}
	base := strings.TrimSuffix(filepath.Base(fields[0]), ".js")
	switch base {
	case "claude":
		return Claude
	case "codex":
		return Codex
	}
	return Other
}

// Label is what a page shows for the harness.
func (n Name) Label() string {
	switch n {
	case Claude:
		return "Claude Code"
	case Codex:
		return "Codex"
	}
	return "Other"
}

// Spec is everything crswd needs to know that differs between runtimes.
type Spec struct {
	Name Name
	// PaneProcesses are the names tmux may report as #{pane_current_command}
	// while the harness runs. Empty means the start binary's own base name.
	PaneProcesses []string
	// RequiredFlags are token groups inserted after the binary on every line
	// crswd types, each skipped when its exact sequence is already present.
	RequiredFlags [][]string
	// FreshIDFlag gives a fresh start its conversation id. Empty: no id at start.
	FreshIDFlag string
	// ResumeArgs are the tokens inserted after the binary to resume id. Nil
	// means the harness cannot be resumed by id.
	ResumeArgs func(id string) []string
	// BracketedPaste delivers prompts and compact with paste-buffer -p.
	BracketedPaste bool
	// SteppedQuit restarts by one C-c at a time, checked between presses.
	SteppedQuit bool
	// RemoteControl reports whether ModeRemote exists for this harness.
	RemoteControl bool
	// ResumesByID reports whether revival and Continue can work at all.
	ResumesByID bool
}

// For is the Spec for n. An unknown Name gets Other's.
func For(n Name) Spec {
	switch n {
	case Claude:
		return Spec{
			Name:        Claude,
			FreshIDFlag: "--session-id",
			ResumeArgs: func(id string) []string {
				return []string{"--resume", id}
			},
			RemoteControl: true,
			ResumesByID:   true,
		}
	case Codex:
		return Spec{
			Name:          Codex,
			PaneProcesses: []string{"codex", "node"},
			RequiredFlags: [][]string{
				{"--no-alt-screen"},
				{"-c", "check_for_update_on_startup=false"},
			},
			ResumeArgs: func(id string) []string {
				return []string{"resume", id}
			},
			BracketedPaste: true,
			SteppedQuit:    true,
			ResumesByID:    true,
		}
	}
	return Spec{Name: Other}
}
