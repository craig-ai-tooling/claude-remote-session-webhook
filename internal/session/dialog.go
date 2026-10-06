package session

import (
	"strings"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

// dialog.go answers one question — "is this pane sitting on a TUI dialog crswd
// did not spawn and cannot answer?" — for callers that already hold stripped
// pane text (tmuxctl.Strip has run; see Manager.Output). It answers nothing
// else: not whether the session is healthy, not whether it will ever recover,
// and not what to do about it. Those are FR-019a's business and this file does
// not touch it.
//
// # Why this exists
//
// PR #150 found a session parked on Claude Code's `/rc` menu: the process was
// up, tmux had a pane, the card said `running`, and `POST /prompt` answered
// `delivered: true` — because the keystroke did land, in a menu nobody was
// there to dismiss. Every observable this daemon has said the session was
// fine. The fix for `/rc` closed that one prompt by never sending it
// (`--remote-control` instead of the slash command); it gave the daemon no way
// to notice the *next* one — the workspace-trust dialog, a future Claude Code
// release's own new prompt, anything else that draws over the input box and
// waits. This file is that: a pane-content check for the strings a dialog like
// that leaves behind.
//
// # This is a heuristic over a TUI crswd does not own
//
// crswd does not speak Claude Code's protocol and has no readiness signal from
// it (that is the *other*, harder half of this problem — see the PR that
// pointed here). All it has is the rendered screen, which is unversioned,
// changes under crswd with every Claude Code release, and belongs entirely to
// a project this daemon does not control. A registry of literal phrases keyed
// to a specific release is exactly the "typed into a TUI blind" failure mode
// loop/trust-seed.sh's header describes for a *different* dialog — 2,826
// restarts in four hours against `Workspace not trusted`, a state no retry
// could fix — except here the daemon is reading the screen rather than typing
// into it, so the failure mode this heuristic risks is silence, not a retry
// storm: a phrase that changes, or a dialog nobody has catalogued, and the
// pane goes back to looking healthy.
//
// That is why suspiciousMarkers exists beside dialogSignatures. A named
// signature is confident: this exact phrase, this exact dialog. A suspicious
// marker is not — it is a fragment several dialogs are observed to share
// without being any one of them — and it exists so that a dialog this
// registry has not caught up with still reads as DisplayUnknown rather than
// silently as DisplayRunning. **An unmatched-but-suspicious pane must never
// render as healthy.** That is the whole of what this file is for: it is
// allowed to be wrong about *which* dialog a session is parked on, and it is
// allowed to say "unknown" when it cannot tell — it is not allowed to say
// "running" about a pane that is plainly waiting for a human.
//
// # How to add a signature
//
// A future dialog nobody has seen yet will first show up as DisplayUnknown
// (a suspicious marker matched, no signature named it). To give it a name,
// capture its rendered pane text — same as dialog_test.go's fixtures — and add
// one dialogSignature entry below with the shortest phrase that reliably
// identifies it and nothing else. Do not add a signature from memory or from a
// description; match dialog_test.go's own discipline and use captured text,
// because a phrase invented rather than observed is exactly the kind of
// heuristic that goes stale unnoticed.
var dialogSignatures = []dialogSignature{
	{
		// The workspace-trust dialog: Claude Code refuses to load CLAUDE.md,
		// hooks, or .mcp.json for a folder that has not been trusted, and asks
		// interactively. loop/trust-seed.sh (ai-lawnmower) answers this one by
		// writing the decision to Claude Code's own config before a session
		// starts, rather than racing it here; this entry exists so a session
		// that reaches it anyway — trust-seed not run, a directory it missed,
		// a future Claude Code release that asks somewhere new — is reported
		// rather than mistaken for healthy.
		name:    "workspace-trust",
		phrases: []string{"Do you trust the files in this folder?"},
	},
	{
		// The /rc remote-control menu PR #150 found: Disconnect this session /
		// Show QR code / Continue. Matched on the menu's own item text rather
		// than its "Enter to select · Esc to continue" footer, because that
		// footer is the generic, dialog-shared phrase suspiciousMarkers holds
		// below — matching it here would make this signature fire on any
		// dialog sharing the footer, which is a second copy of "unknown"
		// wearing a name it has not earned.
		name:    "rc-menu",
		phrases: []string{"Disconnect this session", "Show QR code"},
	},
}

// suspiciousMarkers are phrases several TUI dialogs are observed to share —
// none identifies which dialog is on the pane, and none may be promoted into a
// dialogSignature's own phrase list for exactly that reason: doing so would
// make that signature fire on every dialog carrying the marker, not only the
// one it is meant to name.
//
// A match here is what keeps a dialog this registry has not catalogued from
// rendering as DisplayRunning. It renders as DisplayUnknown instead — see the
// package comment above for why that is the whole point of keeping this list
// separate from dialogSignatures rather than folding it in.
var suspiciousMarkers = []string{
	"Enter to select",
	"Enter to confirm",
	"Esc to continue",
	"Esc to cancel",
	"Esc to exit",
}

// codexDialogSignatures are the dialogs Codex draws over its input box, each
// phrase copied from a captured screen (specs/019-codex-runtime/research.md
// section F; fixtures in testdata/). Matching is case-sensitive, so Claude's
// "Enter to confirm" markers and Codex's "Press enter to confirm" never cross.
var codexDialogSignatures = []dialogSignature{
	{name: "codex-trust", phrases: []string{"Do you trust the contents of this directory?"}},
	{name: "codex-hooks-review", phrases: []string{"Hooks need review"}},
	{name: "codex-approval", phrases: []string{"Would you like to run the following command?"}},
	{name: "codex-update", phrases: []string{"Update now (runs"}},
}

// codexSuspiciousMarkers are the footers Codex's dialogs share. Like
// suspiciousMarkers, none may appear in a signature's own phrases.
var codexSuspiciousMarkers = []string{
	"Press enter to continue",
	"Press enter to confirm or esc to cancel",
	"Press enter to confirm or esc to go back",
}

// DetectDialogFor is DetectDialog for the registry of the harness the session
// runs: Claude reads Claude's, Codex reads Codex's, and a harness this daemon
// has no registry for tries Claude's and then Codex's, so a dialog from either
// is still seen.
func DetectDialogFor(h harness.Name, paneText string) (name string, dialog bool) {
	switch h {
	case harness.Claude:
		return DetectDialog(paneText)
	case harness.Codex:
		return detectCodexDialog(paneText)
	default:
		if name, dialog := DetectDialog(paneText); dialog {
			return name, dialog
		}
		return detectCodexDialog(paneText)
	}
}

func detectCodexDialog(paneText string) (name string, dialog bool) {
	if paneText == "" {
		return "", false
	}
	for _, sig := range codexDialogSignatures {
		for _, phrase := range sig.phrases {
			if strings.Contains(paneText, phrase) {
				return sig.name, true
			}
		}
	}
	for _, marker := range codexSuspiciousMarkers {
		if strings.Contains(paneText, marker) {
			return "", true
		}
	}
	return "", false
}

// dialogSignature is one entry in the registry above: a name a card and an API
// response may show an operator, and the literal phrases that mean it. Any one
// phrase matching is enough — a dialog's own wording sometimes varies by
// terminal width or Claude Code version while the phrase this names does not.
type dialogSignature struct {
	name    string
	phrases []string
}

// DetectDialog scans stripped pane text — tmuxctl.Strip must already have run;
// this package never sees a control byte — for the registry above.
//
// name is non-empty only when a dialogSignature matched by name; dialog is
// true whenever anything matched at all. The two are deliberately not folded
// into one return: (name != "", dialog == true) is "parked on <name>", and
// ("", true) is "parked on something this registry has not catalogued" —
// DisplayUnknown, never DisplayRunning. A caller that checks only `dialog` and
// ignores `name` still gets the fail-closed half of this file's contract; a
// caller that drops straight to `name != ""` without checking `dialog` first
// would silently treat an unnamed match as no match, which is the bug this
// file exists to prevent one layer up.
func DetectDialog(paneText string) (name string, dialog bool) {
	if paneText == "" {
		return "", false
	}
	for _, sig := range dialogSignatures {
		for _, phrase := range sig.phrases {
			if strings.Contains(paneText, phrase) {
				return sig.name, true
			}
		}
	}
	for _, marker := range suspiciousMarkers {
		if strings.Contains(paneText, marker) {
			return "", true
		}
	}
	return "", false
}
