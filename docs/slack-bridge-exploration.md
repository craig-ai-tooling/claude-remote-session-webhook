# Slack as a remote surface for coding agents: exploration

Status: exploration, second pass 11/9/26. Nothing here is built and no spec exists (next would be
spec 021). Every claim below is tagged as measured (run on this box), internal (Super / Spectro
docs), or docs (vendor or third-party docs, not run).

## The ask

Drive Claude Code, Codex, Gemini and Grok sessions from Slack, Devin-style. One channel per
session, live progress, Slack messages become prompts, archiving the session archives the channel.
The reason is PaletteAI Launchpad: once a CLI's model traffic goes through the router, Claude Code's
Remote Control stops working. "Slack remote control" should be a per-session switch beside native
Remote Control.

## What changes the plan

1. **Native Claude features are closed behind the router. Measured, Claude Code 2.1.319.** With
   router-style auth (`ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN`, separate config dir, no
   claude.ai login), `/remote-control` is an unknown command and `--channels` prints "Channels are
   not currently available". Channels are Claude's native "push a chat message into a running
   session" feature. They would have been the cheapest Slack path for Claude.
2. **The same channel works under a claude.ai login, even with a custom base URL. Measured.** A
   30-line MCP channel server injected `PING-FROM-SLACK-TEST` into the session, and Claude then
   called the custom URL. This is not a usable route to Launchpad: Claude would send the user's
   claude.ai OAuth bearer to the router, which leaks a subscription credential to another host.
3. **Gemini CLI cannot reach Launchpad. Internal (Super, AIL-4, AIL-731, AIL-723).** The router
   exposes OpenAI `/v1` (chat completions and Responses) and Anthropic `/v1/messages`. It has no
   Gemini-protocol endpoint, and Gemini CLI only speaks Gemini's protocol
   (`GOOGLE_GEMINI_BASE_URL`). "All four behind Launchpad" is three at best until Launchpad adds a
   Gemini endpoint. Gemini through Slack direct to Google still works.
4. **Grok behind Launchpad is unverified.** Grok Build takes a custom `base_url` per model in
   `~/.grok/config.toml` (docs). Super says it would have to target `/v1`. Nobody has confirmed
   which wire format Grok Build sends there.
5. **Codex already has a multi-client protocol. Measured, codex-cli 0.160.1.** `codex app-server
   --listen ws://…` serves JSON-RPC, and the TUI can attach to it (`codex --remote ws://…`). Two
   websocket clients connected to one app-server. Client B received `thread/started` and
   `thread/name/updated` for a thread client A created, without subscribing. The protocol has
   approval requests (`item/commandExecution/requestApproval`, `item/fileChange/requestApproval`)
   and a `serverRequest/resolved` notification. **Not tested:** turn events, and whether an
   approval request reaches every client. That needs a model call.
6. **Someone has built most of this already.** [openab](https://github.com/openabdev/openab)
   (MIT, Rust, 834 stars, last push 10/31/26) bridges Slack (Socket Mode) to any ACP agent. Its
   docs cover Claude (`claude-agent-acp`), Codex (`codex-acp`), Gemini (`gemini --acp`) and Grok
   (`grok agent stdio`), and it ships a Helm chart. Verified from its repo docs.
7. **Slack has an API for exactly "a channel per session". Docs.** `agents.conversations.create`
   (with `session_id` for idempotency), `agents.sessions.setStatus`, and
   `agents.conversations.archive` with a summary message. It is gated: `feature_disabled` unless
   the app is a recognized agent-session app. Plain `conversations.create` / `.archive` work
   regardless.

No Launchpad was reachable from this box (`launchpad.lab.internal` does not resolve), so nothing
was run against a real router.

## What the first pass got wrong

- It implied all four providers work behind Launchpad. Gemini does not (finding 3).
- It treated Slack streaming as unknown. `chat.startStream` exists, but a stream lives at most 10
  minutes, so long agent turns need `chat.postMessage` + status updates anyway.
- It did not check prior art. openab, cc-connect and others exist (below).
- It proposed hooks + screen scraping for every harness. Codex has a structured, multi-client
  protocol that removes the scraping for Codex entirely.

## The common layer: ACP

The [Agent Client Protocol](https://agentclientprotocol.com) is JSON-RPC over stdio. A client
spawns the agent and calls `session/new` or `session/load`, then `session/prompt`. The agent
streams `session/update` (message chunks, tool calls, plan), asks `session/request_permission`
(maps to Slack approve/deny buttons), and takes `session/cancel`. Docs, not run here:

| Agent | ACP entry point | Permission requests | Resume | Custom endpoint |
|---|---|---|---|---|
| Claude Code | `claude-agent-acp` adapter over the Agent SDK | yes | yes (`session/resume`) | SDK honours `ANTHROPIC_BASE_URL`; adapter does not document it |
| Codex | `codex-acp` adapter over app-server | yes | unverified | `[model_providers]` with `wire_api = "responses"`, as Launchpad's own Codex setup uses |
| Gemini | native `gemini --acp` | yes (third-party report) | yes (`loadSession`) | Gemini protocol only, so not Launchpad |
| Grok | native `grok agent stdio` | yes | unverified | `base_url` per model, format unverified |

The cost of ACP: the agent runs headless. There is no TUI in a tmux pane for crswd's dashboard to
show, and slash commands and TUI-only features are gone.

## Prior art

| Project | Fit | Gaps for this ask |
|---|---|---|
| **openab** | Slack + all four via ACP, Helm, `allowed_users`, Slack streaming, session pool with idle TTL | Thread per session, not channel. Permission requests are **auto-replied**, so nobody approves actions on an unsandboxed host. No cancel on Slack (its docs say so). Router env pass-through unverified |
| cc-connect (16k stars, active) | Slack, Telegram and others; Claude, Codex, Gemini, any ACP | Grok not listed. Licence unclear (README says MIT, GitHub API shows none). Slack session model and buttons unverified |
| Untether | Best approval UX (inline buttons) | Telegram only, dropped Gemini |
| Claude Code in Slack (Anthropic) | Polished | Runs Claude Code on the web. Disqualified by the router |
| happy, omnara | Active | Own apps, not Slack |

No project found does channel-per-session on Slack. All use threads.

## How this relates to crswd

crswd is a tmux runtime. Its signed API is six routes: create, list, get and destroy a session,
send a prompt, read the screen. Keys and the live stream are on the browser door only. It never
opens a conversation transcript (spec 013 FR-025). What each option asks of it:

| Need | ACP route (openab or own) | crswd-native route |
|---|---|---|
| Session lifecycle | owned by the bridge; crswd not involved | crswd, as today |
| Structured output | ACP `session/update` | Claude: hooks via `--settings`. Codex: app-server second client. Others: screen diff |
| Approvals | `session/request_permission` to Slack buttons | Claude: none in the TUI except key presses. Codex: app-server approval requests |
| Dashboard visibility | none | as today, the TUI is still there |
| crswd changes | none | signed key route, lifecycle feed, hook injection, Codex app-server launch, a `surfaces` field beside `Mode` |

## Security

A Slack message that reaches an agent is unsandboxed code execution on the host. Whichever option:

- Authorize by Slack user ID allowlist on every event. Channel membership is not authorization.
- Private channels only. Drop events with `bot_id`, a subtype, or a user not on the list.
- Approvals for shell and file changes go to a button only an allowlisted user can press. openab's
  auto-reply has to be replaced before it runs on a host that matters.
- Output posted to Slack leaves the host and may hold secrets. Default it per session, opt-in.
- Never route a claude.ai OAuth token through the router (finding 2).

## Slack limits that shape the bot

`chat.postMessage` about 1/sec per channel, which favours a channel per session over one shared
channel. `chat.update` rejects text over 4,000 chars. Long output goes up as a file
(`files.getUploadURLExternal` + `files.completeUploadExternal`). `conversations.create` and
`.archive` are Tier 2 (20/min). Socket Mode allows 10 connections per app and keeps the host free
of inbound ports. On an Enterprise Grid org, admin approval of apps is on by default.

## Options

**A. Spike openab (adopt).** Deploy its Helm chart on the Pi cluster with Claude and Codex pointed
at Launchpad. Measure the router env pass-through and how the thread UX feels. Then upstream or
fork: channel-per-session, archive on close, human approvals, cancel. Cheapest way to learn, with
no crswd change. Leaves crswd and Slack sessions as two separate worlds.

**B. Build crswd-native.** A `crswd-slack` bridge as a signed crswd client. Claude: TUI in tmux,
hooks for output, prompts via the existing route, approvals limited to key presses. Codex: TUI
attached to an app-server, bridge as a second client with real approvals. Gemini and Grok: screen
diff until each is wired. Keeps one session world and the dashboard. Most crswd work.

**C. Headless ACP runner inside crswd.** crswd gains a second runtime: ACP sessions with a
transcript view instead of a pane, Slack as one client. Uniform across all four agents. Largest
change, and it means two runtimes in one daemon.

Recommendation: **A first, for one or two days**, because it answers the router and UX questions
with real sessions for little cost. Decide between forking openab and B with that evidence.

## Phases after the decision

| Phase | Work |
|---|---|
| 0 | Slack app in a sandbox or the chosen workspace; openab on the cluster with Claude + Codex on Launchpad |
| 1 | Channel per session + archive on end; human approvals; allowlist; opt-in output |
| 2 | Grok on Launchpad once its wire format is confirmed; Gemini direct to Google, or on Launchpad once it has a Gemini endpoint |
| 3 | crswd integration chosen from A/B/C: a `surfaces=[slack]` field beside `Mode`, so the Slack switch lives on the session |

## What was run

- Claude Code 2.1.319 in a private tmux socket: token auth plus custom base URL, Remote Control
  and channels both checked; claude.ai login plus custom base URL, channel injection checked. Test
  server at the session scratchpad, not committed.
- codex-cli 0.160.1 app-server on loopback with a scratch `CODEX_HOME`, two websocket clients, no
  model calls.
- Super query on Launchpad's client protocols.
- Repo docs read for openab (Slack, Claude, Codex, Gemini, Grok pages).
- Not run: any Slack API call, any Gemini or Grok CLI, any request to a real Launchpad.
