# Slack as a remote surface for coding agents: exploration

Status: exploration, 11/9/26. Nothing here is built. No spec exists yet (next would be spec 021).

## The ask

Drive Claude Code, Codex, Gemini and Grok sessions from Slack, Devin-style: one channel per
session, live progress, Slack messages become prompts, archiving the session archives the channel.
The reason is PaletteAI Launchpad. A model router needs `ANTHROPIC_BASE_URL` pointed at it, and
Claude Code's native Remote Control turns itself off in that case (it needs a claude.ai login and
Anthropic's own endpoint; confirmed in the [Remote Control docs](https://code.claude.com/docs/en/remote-control)
and [issue 59062](https://claudeissues.com/issue/59062-bug-remote-control-wont-work-when-anthropic-auth-token-or-anthropic-api-key-is-s)).
So the Slack path must not depend on anything Anthropic hosts.

## What crswd gives us today

| Need | Today | Gap |
|---|---|---|
| Create / list / destroy a session | Signed API: `POST/GET /sessions`, `GET/DELETE /sessions/{id}` | None |
| Send a prompt | Signed `POST /sessions/{id}/prompt` | None |
| Read output | Signed `GET /sessions/{id}/output` (tmux screen text). Browser door has an SSE screen stream | Output is a terminal screen, not messages. Redraws, spinners and diffs make a bad Slack feed |
| Press keys (approve, Esc, Ctrl-C) | Browser door only (spec 018) | No signed route |
| Lifecycle events | Browser-door fleet SSE; signed door must poll `GET /sessions` | No signed event feed |
| Dialog detection (trust, approval) | `session.DetectDialogFor` per harness | Internal only |
| Harness differences | `internal/harness.Spec`: Claude and Codex, everything else is `Other` | Gemini and Grok are `Other` |
| Remote control | `Mode=remote` is `claude --remote-control`, Claude only | Slack is a different axis, see below |
| Isolation, audit, k8s pods, resume | Done (specs 008, 012, 017) | Reuse, do not rebuild |

crswd never opens a conversation `.jsonl` (spec 013 FR-025). Any structured-output design has to
respect that or change it deliberately in a spec.

## The central problem: structured output

Slack wants messages ("ran tests, 3 failed", "needs approval: rm -rf build"). tmux gives a
repainted screen. Four ways to get messages, per provider:

| Provider | Structured side channel (verify before relying) |
|---|---|
| Claude Code | Hooks (`Stop`, `Notification`, `PreToolUse`, `PostToolUse`, `UserPromptSubmit`) injected with `--settings`; hook payload names the transcript path. Or headless `claude -p --output-format stream-json` / Agent SDK with permission callbacks |
| Codex | Hooks (crswd already passes `--dangerously-bypass-hook-trust`), `codex exec --json`, rollout files, `codex app-server` |
| Gemini CLI | Headless `--output-format stream-json` ([docs](https://geminicli.com/docs/cli/headless/)). No stream-json input, so no interactive approve/deny headless. Hooks exist, whether they fire in headless is unverified |
| Grok Build | Headless `--output-format streaming-json`; 0.2.116 added tool calls, results and usage, and a `streaming-messages-json` format ([docs](https://docs.x.ai/build/cli/headless-scripting)). Schema not formally published |
| Any other CLI | Fallback: diff the tmux screen, post only settled frames |

I did not run Gemini or Grok. The table is from docs and third-party notes.

## Two architectures

**A. Bridge beside crswd, TUI stays in tmux (recommended).**
A new `crswd-slack` binary is an ordinary signed client of crswd, like `crswd-axi`. crswd keeps
owning sessions, isolation, audit, k8s, resume. The bridge owns Slack. Output comes from the
provider's side channel (hooks posting to the bridge, or the file path a hook names), with
screen-diff as fallback. Input goes through `/prompt`, plus a new signed key route for approvals.
Works unchanged for Claude, Codex, `Other`, and in k8s mode. Cost: hook injection per harness,
and Slack buttons for approvals have to be mapped onto key presses in a TUI.

**B. Headless runner, no tmux.**
Run each agent as `claude -p --input-format stream-json --output-format stream-json` (or the Agent
SDK) and speak JSON. Cleanest Slack UX: Block Kit approve/deny from a permission callback, real
streaming. Cost: it is a second runtime beside crswd's tmux one, duplicates lifecycle, resume and
isolation, and Gemini has no equivalent input mode. That is a structural decision, so it needs
Craig's call. Not recommended as the first move.

A can grow into B for Claude later without throwing the bridge away: the bridge's internal
interface is "session events in, prompts and decisions out".

## What is independent of crswd

All of the Slack side: app manifest, Socket Mode connection (no inbound port, matching crswd's
posture), channel create/archive, message formatting and chunking, rate-limit queue, mapping
`session id <-> channel id` (small on-disk store), Slack-user authorization.

## What touches crswd

1. Signed key/approve route (today browser door only).
2. Signed lifecycle feed, or the bridge polls `GET /sessions` every few seconds (fine for phase 1).
3. Per-session hook injection and an event sink, as `harness.Spec` data (`HookFlags`, `EventSource`).
4. A per-session `surfaces` field, independent of `Mode`: `Mode` stays `local|remote` (Claude's native
   RC, per `docs/harnesses.md`), `surfaces=[slack]` works for every harness. This is the "enable Slack
   remote control" switch, and it works behind Launchpad.
5. Env for the router: crswd starts commands by name only; how `ANTHROPIC_BASE_URL` and a Launchpad
   `lpai_` token reach a session needs a check against spec 008 (session environment boundary). Not
   verified here.
6. Close hook: on destroy or reap, crswd tells the bridge, the bridge archives the channel.

## Security: the part that decides the design

A Slack message that reaches `/prompt` is unsandboxed code execution on the host. So:

- Authorization is a Slack user ID allowlist in the bridge config, checked on every event. Channel
  membership alone is not authorization.
- Private channels only, created by the bot. Ignore bot messages, edits, and anything from a user
  not on the list, including in shared or Slack Connect channels.
- Output posted to Slack leaves the host. Pane and transcript content can hold secrets. Redaction
  and an opt-in per session are needed, and `docs/security.md` rules for rendering pane output apply.
- Slack tokens (bot, app-level) live only in the bridge. crswd's HMAC key lives in the bridge too,
  scoped to that client.
- Prompt injection: a session that reads Slack threads or web pages can be steered. Approvals for
  destructive actions should require the allowlisted user's button press, not text.

## Slack constraints that shape it

Channel names: lowercase, 80 chars. `conversations.create` and `chat.postMessage` are rate limited
(about one message per second per channel), so the bridge batches and edits one "status" message
rather than posting per tool call. Message text is capped (40k; section blocks 3k), so long output
goes as a file snippet. Needs scopes for channels/groups write and manage, chat write, files write,
and Socket Mode. Channel sprawl is real: offer `channel` (default, as asked) or `thread` per session
in one channel. Slack's newer streaming APIs for AI apps may suit live progress; unverified.

## Phases

| Phase | What | Crswd change | Rough size |
|---|---|---|---|
| 0 | Spike: Slack app, one hard-coded session, poll `/output`, post settled diffs, Slack text to `/prompt` | none | 1-2 days |
| 1 | Channel per session, archive on close, allowlist authz, rate-limited status message, Claude hooks as source | hook injection | about 1 week |
| 2 | Spec 021: `surfaces`, signed key route, lifecycle feed, Slack approve buttons | yes | 1-2 weeks |
| 3 | Codex adapter (hooks / `exec --json`) | `Spec` data | few days |
| 4 | Gemini, Grok adapters; fallback screen mode for the rest | `Spec` data | few days each, after verifying each CLI |
| 5 | Optional headless runner for Claude (architecture B) | large | decide after phase 2 |

Phase 0 needs a Slack app in Spectro's workspace; that is a workspace admin step.

## Decisions for Craig

1. Architecture A (bridge, tmux stays) or B (headless runner). Recommend A.
2. Channel per session (as asked) or thread per session.
3. Same repo (`cmd/crswd-slack`, shares `internal/harness`) or its own repo. Recommend same repo.
4. Whether output may leave the host to Slack by default, or only per-session opt-in.
5. Who creates the Slack app and in which workspace.

## Checked, not built

Repo read: AGENTS.md, `docs/harnesses.md`, `internal/harness`, `internal/session/input.go`,
`internal/httpapi` routes and stream. Web: Remote Control requirements, Gemini and Grok headless docs.
Not run: any Slack API call, any Gemini or Grok CLI, any hook.
