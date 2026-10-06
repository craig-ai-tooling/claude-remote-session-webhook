# Implementation Plan: Codex as a First-Class Runtime

**Branch**: one per phase (`plan/codex-core`, `plan/codex-auth`, `plan/codex-quota`, `plan/codex-k8s`)
| **Date**: 2026-10-06 | **Spec**: [spec.md](spec.md) | **Research**: [research.md](research.md)

## Summary

crswd already starts any configured command line in tmux; what it lacks is knowledge of a second
runtime. This plan adds one small package, `internal/harness`, that names the runtime a configured
command is (derived from the binary, never stored) and carries the per-runtime facts as data. Every
Claude-specific branch in `internal/session` and `internal/httpapi` then asks the harness spec
instead of assuming Claude. Codex-specific code is confined to new files (`trust_codex.go`,
`conversation_codex.go`, `discover.go`, `internal/codexauth`), so a Claude session's behaviour is
byte-identical before and after (SC-002).

The three hard problems were measured, not guessed (research §M):
- **Conversation identity**: read from the rollout file the native Codex process holds open (D7).
- **Liveness through the Node wrapper**: tmux matches a set of names (D6).
- **Typing into Codex safely**: bracketed paste (D9), a stepped verified quit (D8), and two flags
  crswd always adds (D5), the second of which stops a blind Enter from running `npm install -g`.

## Technical Context

**Language**: Go, standard library only in the root module (no `go.sum`). **Templates**:
server-rendered, hand-written JS, `go:embed`. **Host**: Linux, tmux ≥ 3.4. **Codex**: 0.153.4.
**Testing**: table-driven, `t.Parallel()`, `tmuxctl.Fake`, fixtures under `t.TempDir()`. Real tmux
only under `-tags tmux`; end-to-end only under `-tags quickstart` with a `codex` shim. No test runs
the real Codex binary.

## Constitution Check

| Principle | Assessment | Pass |
|---|---|---|
| **I — Security** | A create still carries a name, never a command line (D2). The browser's harness field accepts two literals. No path, prompt or code reaches a typed command line: the conversation id is validated by `ValidateResume`'s alphabet, and trust is written to a file, not passed with `-c` (D10). The device code follows the sign-in link's rules (FR-019). | ✅ |
| **II — Unknowns surfaced** | Every Codex behaviour the code depends on is in research §M with how it was measured. What was not measured is listed under "Not verified" and either has a spike task (T034), an operator-run measurement (O-1), or does not change code. NC-1 is Craig's call. | ✅ |
| **III — Verifiable** | Each FR maps to a test named in tasks.md; SC-002 pins Claude's lines unchanged. | ✅ |
| **IV — Smallest change** | Per-harness facts are data in one struct. No interface hierarchy, no plugin boundary. Claude code paths keep their functions; Codex gets new files beside them. | ✅ |
| **V — Standards** | No new dependency; new `Controller` methods follow `argvCapturePane`'s shared-builder pattern (`internal/tmuxctl/fake.go:83`). | ✅ |
| **VI — Blast radius** | Allowed roots, the cap and lifetimes are untouched. Trust seeding grants nothing `allowed_roots` did not (same argument as `trust.go`). The update-check flag removes a path by which a session could install software. Codex restart refuses to type when it cannot confirm the shell (D8). | ✅ |
| **VII — Design system** | The harness picker reuses the existing radio/segmented control in docs/components.md; the second pill and meter reuse `.auth-pill` and `.quota-*`. | ✅ |

## Phases

| Phase | Notebook | Ships | Depends on |
|---|---|---|---|
| 1 Core parity | `ralph/plans/codex-core` | US1–US4, FR-001–FR-014 | spec 018 merged |
| 2 Sign-in | `ralph/plans/codex-auth` | US5, FR-015–FR-019 | Phase 1 merged |
| 3 Usage | `ralph/plans/codex-quota` | US6, FR-020–FR-022 | Phase 1 merged; Phase 2 merged (shared header files) |
| 4a Kubernetes | `ralph/plans/codex-k8s` | US7 (4a), FR-023–FR-025 | Phase 1 merged; spec 017 S7 merged |

One notebook per phase because `ralph/loop.sh` runs one notebook per branch (`plan/<id>`,
`ralph/loop.sh:21-60`), each phase is one PR, and a single 40-task notebook would hold one PR open
across four reviews.

## Design

### `internal/harness` (new)

```go
package harness

type Name string

const (
	Claude Name = "claude"
	Codex  Name = "codex"
	Other  Name = "other"
)

// Of is the harness a configured command line runs: the base name of its first
// whitespace-separated token, with a trailing ".js" removed.
func Of(command string) Name

// Label is what a page shows: "Claude Code", "Codex", or "Other".
func (n Name) Label() string

type Spec struct {
	Name Name
	// PaneProcesses are the names tmux may report as #{pane_current_command}
	// while the harness runs. Empty means "the start binary's own base name".
	PaneProcesses []string
	// RequiredFlags are token groups inserted after the binary on every line
	// crswd types, each skipped when its exact sequence is already present.
	RequiredFlags [][]string
	// FreshIDFlag gives a fresh start its conversation id. Empty: no id at start.
	FreshIDFlag string
	// ResumeArgs are the tokens inserted after the binary to resume id.
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

func For(n Name) Spec
```

| Field | Claude | Codex | Other |
|---|---|---|---|
| PaneProcesses | nil | `codex`, `node` | nil |
| RequiredFlags | nil | `--no-alt-screen`; `-c check_for_update_on_startup=false` | nil |
| FreshIDFlag | `--session-id` | "" | "" |
| ResumeArgs | `--resume <id>` | `resume <id>` | nil |
| BracketedPaste | false | true | false |
| SteppedQuit | false | true | false |
| RemoteControl | true | false | false |
| ResumesByID | true | true | false |

### Session changes

- `Manager.specOf(s Session) harness.Spec` resolves the session's configured command and calls
  `harness.For(harness.Of(cmd))`; an unresolvable name yields `For(Other)`.
- `resumeFlagged` (`internal/session/manager.go:2072`) takes the spec: required flags first, then
  `FreshIDFlag`/`ResumeArgs`. For Claude both branches produce today's exact output.
- `conversationCapable` (`internal/session/conversation.go:345`) becomes
  `harness.For(harness.Of(t)).FreshIDFlag != ""`.
- `paneProcesses(template) string` writes `@crswd-binary`: the spec's names joined by `|`, or
  `startBinary(template)` when the spec has none.
- `Prompt`/`Compact` choose `PasteBracketed` when `BracketedPaste`.
- `restartInto` (`internal/session/supervisor.go:366`) calls `quitStepped` when `SteppedQuit`.
- `start` and `sendStart` call `seedTrustFor(spec, workDir)`: Claude → `SeedTrust` (unchanged),
  Codex → `SeedCodexTrust`.
- Supervisor branch 3 (`supervisor.go:155`) calls `discoverConversation` for running Codex sessions
  with no conversation.

### Files touched (all phases)

```text
internal/harness/                 NEW   harness.go, harness_test.go
internal/tmuxctl/controller.go    MOD   PanePID (PasteBracketed comes from spec 018); OptionBinary doc
internal/tmuxctl/fake.go          MOD   argv builders, Fake methods, livenessOf set, QuitAfterInterrupts
internal/tmuxctl/exec.go          MOD   PanePID
internal/tmuxctl/argv.go          MOD   ArgvPanePID
internal/tmuxctl/*_test.go        MOD   argv + tmux-tag tests
internal/session/conversation.go  MOD   conversationCapable, paneProcesses, ConversationsFor, hasTranscriptFor
internal/session/conversation_codex.go  NEW
internal/session/discover.go      NEW
internal/session/trust.go         MOD   replaceFile takes a temp prefix
internal/session/trust_codex.go   NEW
internal/session/dialog.go        MOD   DetectDialogFor, codex registry
internal/session/manager.go       MOD   specOf, resumeFlagged, Prompt, Compact, SetMode, start, SetCodexHome
internal/session/supervisor.go    MOD   sendStart, restartInto, quitStepped, discovery in judge
internal/session/journal.go       MOD   journalDiscovered
internal/session/testdata/codex-*.pane  NEW
internal/httpapi/server.go        MOD   SetCodexHome wiring; relay map (P2); quota (P3)
internal/httpapi/actions.go       MOD   harness field
internal/httpapi/dashboard.go     MOD   preview, effectiveDisplayState, card harness
internal/httpapi/view.go          MOD   createFormView.CodexCommand, sessionView.Harness
internal/httpapi/sessions.go      MOD   sessionEntry.Harness, paneDialogState harness
internal/httpapi/conversations.go MOD   harness-aware listing
internal/httpapi/authstatus.go, signin.go, signinview.go  MOD (P2)
internal/httpapi/quotastatus.go   MOD (P3)
internal/codexauth/               NEW (P2)
internal/loginrelay/loginrelay.go MOD (P2)
internal/quota/quota.go           MOD (P3)
internal/sessionpod/sessionpod.go MOD (P4a)
web/templates/partials/create-form.html, session-card.html, header.html, signin-panel.html  MOD
web/static/crswd.js, crswd.css    MOD
cmd/crswd/quickstart_test.go      MOD
docs/harnesses.md, docs/security.md, docs/auth-and-sessions.md, docs/components.md, README.md, config.example  MOD
```

## Risks

| Risk | Where it is handled |
|---|---|
| A Codex release changes a dialog's text | Fixtures pinned to 0.153.4; an unmatched dialog still reads `unknown` through Codex's suspicious markers (T013). |
| A Codex release stops holding the rollout open | Discovery returns no id; the session runs and is not revivable by id; the card says so. No wrong id is ever recorded (T011). |
| A Codex release renames `check_for_update_on_startup` | The update menu is in Codex's registry as `codex-update` and renders `blocked` (T013); docs say to re-measure on upgrade. |
| Spec 018 changes the same files | Every Phase 1 task depends on spec 018 merged; tasks read current code, not this plan's line numbers, where they differ. |
