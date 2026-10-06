# Implementation Plan: Interactive Input on the Session Page

**Branch**: `plan/interactive-input` | **Date**: 2026-10-06 | **Spec**: [spec.md](spec.md)

## Summary

The session page gains three things under the pane:

- a textarea that delivers the operator's text by bracketed paste, optionally
  followed by Enter;
- a key bar that sends one of twelve allowlisted keys;
- a closed Scrollback disclosure that fetches up to 5000 lines of tmux history
  when opened.

Underneath, `tmuxctl` gains `PasteBracketed` and `CaptureHistory`, and every
session is created with `history-limit 5000`. `session` gains `Type`, `PressKey`
and `History`. `httpapi` gains two `handleAction` routes, one `handleBrowser`
GET, one rate bucket and three audit actions. Nothing here is specific to
Claude Code, so spec 019 inherits it for Codex unchanged.

## Technical Context

**Language**: Go stdlib only, server-rendered templates, one hand-written
`crswd.js`. **Storage**: none. **Testing**: table-driven, `t.Parallel()`, the
tmux fake for everything except the three `-tags tmux` cases in
`internal/tmuxctl/exec_tmux_test.go`, which need a real tmux. **Measured inputs**:
research.md R1 (bracketed paste), R2 (history-limit), R3 (key bytes).

## Constitution Check

| Principle | Assessment | Pass |
|---|---|---|
| **I — Security is a gate** | Both writes go through `handleAction`: identity, `Sec-Fetch-Site` with absent refused, page token, body cap. The ownership check uses `View`. Caller text travels on stdin only. Keys come from a closed map to daemon constants. Control bytes are refused, so a typed payload cannot close the bracketed paste. The history GET refuses cross-site as the stream does. The audit records the action and never the text, key or history. | ✅ |
| **II — Unknowns surfaced** | The two questions code cannot answer, the phone-only ones, ship UNANSWERED with fallbacks (FR-021). The two behaviours that could have been guessed, multi-line paste and history-limit timing, were measured first. | ✅ |
| **III — Verifiable** | Every FR is a status code, an argv, a byte sequence on a real tmux, a template string, or an audit record a test reads. | ✅ |
| **IV — Smallest change** | Reuses the token bucket, the action gate, the outcome banner, `Strip`, `.pane`, `.button` and `.field-input`. `Paste` and `/prompt` are untouched (research R1). | ✅ |
| **V — Standards enforced** | The leak suite, the refusal-shape sweep and the stylesheet sweeps gain the new routes and classes instead of being relaxed. | ✅ |
| **VI — Blast radius** | Bounds are constants: 16384 bytes of text, 240/min, 5000 history lines, 4 MiB of history. No caller value reaches an argv. The rule amendment (FR-001) narrows the exception to an operator pressing a control. | ✅ |
| **VII — Design system** | Five new classes, all built from existing tokens. No new breakpoint, no inline style, no new font size. The pane's text-only rendering is unchanged. | ✅ |

## Project Structure

```text
internal/tmuxctl/
├── controller.go        MOD  PasteBracketed, CaptureHistory on the interface; HistoryLimit
├── fake.go              MOD  argvNew chain; argvPasteBufferBracketed; argvCaptureHistory;
│                             OpPasteBracketed, OpCaptureHistory; Fake methods; SetHistory
├── exec.go              MOD  Exec.PasteBracketed, Exec.CaptureHistory, ErrHistoryTooLarge
├── argv.go              MOD  ArgvPasteBracketed, ArgvCaptureHistory wrappers
├── argv_test.go         MOD  wrapper equality rows
├── fake_test.go         MOD  new-session argv row; new method rows
├── exec_test.go         MOD  new-session argv row; new method rows
└── exec_tmux_test.go    MOD  three real-tmux cases
internal/session/
├── input.go             NEW  Key, ParseKey, Keys, MaxTypeBytes, ValidateTyped, Type, PressKey, History
├── input_test.go        NEW
└── manager_test.go      MOD  new-session argv row (line 219)
internal/httpapi/
├── input.go             NEW  three routes, fields, sentinels
├── input_test.go        NEW
├── outcome.go           MOD  six outcome codes + sentences
├── server.go            MOD  inputs limiter; three registrations
├── dashboard_test.go    MOD  pane-note assertion keeps passing (no edit expected); input panel render test
└── stylesheet_test.go   MOD  script sweeps for the input client
internal/audit/
├── audit.go             MOD  ActionDashboardType, ActionDashboardKey, ActionDashboardHistory
├── audit_test.go        MOD  both action tables
└── leak_test.go         MOD  drive dashboard.type with a canary
web/templates/partials/pane.html   MOD  note copy; input panel; key bar; scrollback
web/static/crswd.css                MOD  .input-panel .type-form .key-bar .scrollback .scrollback-summary
web/static/crswd.js                 MOD  input client; shared handler skips data-session-input
cmd/crswd/quickstart_dashboard_test.go  MOD  only if the note assertion breaks (it should not: FR-020 keeps the phrase)
specs/001-crswd-daemon-core/contracts/tmuxctl.md  MOD  new-session chain; two new commands
AGENTS.md, docs/security.md, docs/components.md, docs/mobile-open-questions.md  MOD
```

## Cross-spec notes

- **Spec 017 (kubernetes)**: its second `tmuxctl.Controller` (`internal/podctl`,
  not yet on main) must implement `PasteBracketed` and `CaptureHistory` using
  `ArgvPasteBracketed` and `ArgvCaptureHistory`. `ArgvNew` already carries the
  history-limit chain, so the pod gets it for free. Record this in spec 017's
  next plan revision. It is not edited here.
- **Spec 019 (Codex)**: start Codex with `--no-alt-screen`, or tmux keeps no
  history for it (research R1 table, `alternate_on=0` measured). Codex can also
  open on an update prompt and a "Hooks need review" prompt (seen during the
  R1 probe). The key bar answers both, and 019 should still avoid them at start.
