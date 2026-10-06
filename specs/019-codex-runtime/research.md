# Research: Codex as a first-class runtime

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · **Tasks**: [tasks.md](tasks.md)

Everything below marked Measured was run on `claude-server` on 10/6/26 against
`codex-cli 0.153.4` (`/home/nctiggy/sf-cli/bin/codex`) and tmux 3.4, in a private tmux
server (`tmux -L <name>`) with an isolated `CODEX_HOME`, so the operator's
`~/.codex/config.toml` was not touched. Anything not measured says so.

## M. Measurements

| # | Question | Measured answer |
|---|---|---|
| M1 | What does tmux report as `#{pane_current_command}` while Codex runs? | `node`. `~/sf-cli/bin/codex` is a symlink to `@openai/codex/bin/codex.js`, a Node wrapper that spawns the native binary `…/codex-linux-x64/vendor/x86_64-unknown-linux-musl/bin/codex`. After Codex exits the pane reports `bash`. |
| M2 | Does a pre-written `[projects."<dir>"] trust_level = "trusted"` in `$CODEX_HOME/config.toml` suppress the trust prompt? | Yes. With the table written before start, Codex went straight to the composer. |
| M3 | Does crswd's prompt delivery (`load-buffer`, `paste-buffer -d`, then `send-keys Enter`) submit on Codex? | **No.** The text lands in the composer and the Enter is swallowed. A second Enter sent ~12 s later submitted it. |
| M4 | Does bracketed paste (`paste-buffer -p -d`) followed by `send-keys Enter` submit? | **Yes**, for one line and for a two-line paste. Both turns completed. |
| M5 | Does `/compact` delivered as bracketed paste + Enter work? | Yes. The pane printed `• Context compacted`. |
| M6 | Where is the conversation id before and after the first prompt? | No rollout file exists until the first prompt. After it, the native `codex` process holds the rollout **open**: `/proc/<native pid>/fd/37 -> $CODEX_HOME/sessions/2026/10/06/rollout-2026-10-06T00-11-13-01a10e8c-d5f5-7452-8561-233103b38287.jsonl`, still open while idle after the turn. |
| M7 | Process tree under the pane | `pane_pid` (bash) → `node …/bin/codex` → native `codex`. `/proc/<pid>/task/<pid>/children` lists children on this kernel. |
| M8 | What does Ctrl-C do? | Idle with an empty composer: one `C-c` exits to the shell and prints `To continue this session, run: codex resume <id>`. During a turn: `C-c C-c` interrupts (`■ Conversation interrupted …`) and **does not exit**. Esc then `C-c C-c` one second later also did not exit. |
| M9 | Does a stepped quit converge? | Yes. During a turn with half-typed text in the composer: `C-c`, wait 1.5 s, check `pane_current_command`; repeated, the pane was back at `bash` after the **third** press. |
| M10 | Does `codex resume <id> <flags…>` accept the top-level flags after the subcommand? | Yes. `codex resume <id> --dangerously-bypass-approvals-and-sandbox --no-alt-screen --dangerously-bypass-hook-trust -c check_for_update_on_startup=false` restored the conversation (history visible, `Context compacted` line included). |
| M11 | `codex resume <id that does not exist>` | Prints `ERROR: No saved session found with ID <id>. Run codex resume without an ID to choose from existing sessions.` and exits to the shell. **No picker.** |
| M12 | Is the update prompt dangerous? | **Yes. This is the most important finding.** On `codex resume`, Codex 0.153.4 drew a blocking menu: `✨ Update available! 0.153.4 -> 0.160.1` / `› 1. Update now (runs npm install -g @openai/codex)` / `2. Skip` / `3. Skip until next version` / `Press enter to continue`. A prompt delivered blind (paste + Enter) selected item 1 and started a global `npm install -g @openai/codex`. It was interrupted by the next Ctrl-C; npm rolled back (verified: `codex --version` still 0.153.4, files still dated 9/8). |
| M13 | Can the update check be turned off per launch? | Yes. `-c check_for_update_on_startup=false` on the command line suppressed the menu on the same `resume` that showed it before. The key is in the binary's config schema. |
| M14 | `codex login status` exit codes | Signed in: prints `Logged in using ChatGPT`, exit 0. Empty `CODEX_HOME`: prints `Not logged in`, exit 1. |
| M15 | `codex login --device-auth` screen (CLI, not the TUI) | Captured below (§F2). The code is two upper-case alphanumeric groups joined by `-`. |
| M16 | `session_meta` first line of a rollout | JSON object `{"timestamp","ordinal","type":"session_meta","payload":{…}}`; payload keys include `id`, `session_id`, `cwd`, `originator` (`codex-tui`), `cli_version`, `timestamp`, `base_instructions`. Line length on this host: 18–23 KB (62 rollouts, max 22,455 bytes) because `base_instructions` is inline. |
| M17 | tmux regex format match | `#{m/r:^(#{@crswd-binary})$,#{pane_current_command}}` with `@crswd-binary` = `codex\|node\|sleep` (raw bytes `codex|node|sleep`; the backslashes are table escaping) returned `1` for a pane running `sleep`, and `0` with `codex\|node`. tmux 3.4 here; the session image ships 3.5a. |
| M18 | quota-axi `codex` provider | `providers/codex.js:134` names the windows `five_hour` and `weekly`. The cache file on this host (`~/.cache/quota-axi/quotas.json`) holds only the `claude` provider. `quota-axi --provider codex --full` reports `auth_required` although `codex login status` says signed in; this account is Business with unlimited credits, and `account/rateLimits/read` returns `primary: null, secondary: null`. Cause of `auth_required`: **not measured**. |

## F. Captured screens (fixture sources)

The tasks copy these into `testdata/*.pane` files verbatim. A captured code or path is
replaced by a fixed placeholder, and the fixture's header comment says so.

### F1. Trust prompt (research-codex.md §7, 0.153.4, 120 columns)
```
> You are in /tmp/claude-1000/-home-nctiggy-code-claude-remote-session-webhook/fc11d877-2e91-49c0-8351-5903e08d9817/scra

  Do you trust the contents of this directory? Working with untrusted contents comes with higher risk of prompt
  injection. Trusting the directory allows project-local config, hooks, and exec policies to load.

› 1. Yes, continue
  2. No, quit

  Press enter to continue
```

### F2. Device-code sign-in, `codex login --device-auth` (M15)
```
Welcome to Codex [v0.153.4]
OpenAI's command-line coding agent
Follow these steps to sign in with ChatGPT using device code authorization:
1. Open this link in your browser and sign in to your account
   https://auth.openai.com/codex/device
2. Enter this one-time code (expires in 15 minutes)
   ABCD-EFGH1
Continue only if you started this login in Codex. If a website or another person gave you this code, cancel.
```
(`ABCD-EFGH1` replaces the real code.)

### F3. Device-code sign-in, TUI variant (research-codex.md §6)
```
  Welcome to Codex, OpenAI's command-line coding agent
  Finish signing in via your browser
  1. Open this link in your browser and sign in
  https://auth.openai.com/codex/device
  2. Enter this one-time code after you are signed in (expires in 15 minutes)
  ABCD-EFGH1
  Continue only if you started this login in Codex. If a website or another person gave you this code, cancel.
  Press esc to cancel
```

### F4. Signed-out TUI start (research-codex.md §6)
```
  Welcome to Codex, OpenAI's command-line coding agent
  Sign in with ChatGPT to use Codex as part of your paid plan
  or connect an API key for usage-based billing
> 1. Sign in with ChatGPT
     Usage included with Plus, Pro, Business, and Enterprise plans
  2. Sign in with Device Code
     Sign in from another device with a one-time code
  3. Provide your own API key
     Pay for what you use
  Press enter to continue
```

### F5. Hooks review (research-codex.md §7)
```
  Hooks need review
  2 hooks are new or changed.
  Hooks can run outside the sandbox after you trust them.
› 1. Review hooks
  2. Trust all and continue
  3. Continue without trusting (hooks won't run)
  Press enter to confirm or esc to go back
```

### F6. Approval prompt (research-codex.md §7)
```
  Would you like to run the following command?
  Environment: local
  Reason: Allow running the exact command outside the sandbox after the sandbox failed to start?
  $ touch probe.txt
› 1. Yes, proceed (y)
  2. Yes, and don't ask again for commands that start with `touch probe.txt` (p)
  3. No, and tell Codex what to do differently (esc)
  Press enter to confirm or esc to cancel
```

### F7. Update menu (M12)
```
  ✨ Update available! 0.153.4 -> 0.160.1
  Release notes: https://github.com/openai/codex/releases/latest
› 1. Update now (runs `npm install -g @openai/codex`)
  2. Skip
  3. Skip until next version
  Press enter to continue
```

### F8. Idle (no dialog; must not match anything)
```
╭──────────────────────────────────────────────────────────╮
│ >_ OpenAI Codex (v0.153.4)                               │
│                                                          │
│ model:       gpt-5.6-sol   /model to change              │
│ directory:   /home/op/code/repo                          │
│ permissions: YOLO mode                                   │
╰──────────────────────────────────────────────────────────╯
  Tip: See the Codex keymap documentation for supported actions and examples.
› Ask Codex to do anything
  gpt-5.6-sol default · /home/op/code/repo
```

### F9. Working (no dialog; must not match anything)
```
› say hi in one word
◦ Working (0s • esc to interrupt)
```

## D. Decisions

Each decision names the rejected alternative and why.

**D1. A harness is derived from the configured command, never stored.**
`harness.Of(command)` is the base name of the command's first token (`startBinary`'s rule,
`internal/session/conversation.go:362`): `claude` → Claude, `codex` → Codex, anything else →
Other. A session already persists its start-command *name* in `@crswd-start` and the journal's
`start` field, and adoption restores both; the harness is recomputed from the configured
command each time it is needed.
*Rejected: a stored `@crswd-runtime` option.* Spec 009's lesson is that a fact tmux does not
hold is lost on adoption. Storing a second copy of something derivable creates a fact that can
disagree with the configuration it came from. *Rejected: a `harness=` field in the
configuration.* It would let an operator label `claude` as Codex; the binary already says what it is.

**D2. A create still carries a name, never a command line.** The operator configures Codex as
a named entry in `start_commands`, e.g.
`codex=/home/nctiggy/sf-cli/bin/codex --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust`.
The browser offers Codex only when an entry named exactly `codex` exists and `harness.Of` of its
command is Codex. The signed API can already name `codex` (`start_command`).
*Rejected: offering every configured name.* That is spec 011's "harness picker" in general form;
this spec ships the two runtimes Craig asked for and keeps the browser's allowlist two entries long.

**D3. Mode (local/remote) stays Claude-only.** `ModeRemote` means `claude --remote-control`.
A Codex session is always `local`; the browser refuses `remote_control=on` with `harness=codex`.
*Rejected: generalising Mode into "which named command".* docs/harnesses.md asked; the answer is
that Mode and harness are different axes, and Codex has no remote mode to put on the second value.

**D4. Remote control for Codex is a non-goal.** `codex remote-control` manages an app-server
daemon that the ChatGPT mobile app pairs with; it does not attach to a TUI running in tmux
(research-codex.md §1). Spec 018's typed input is how a Codex session is driven from a phone.

**D5. Required flags are added by the daemon, not left to configuration.** Codex's harness spec
carries two flag groups that crswd inserts after the binary on every line it types, unless the
exact token sequence is already in the configured command:
- `--no-alt-screen`: without it Codex draws on the alternate screen and tmux keeps no history,
  which defeats spec 018's scrollback.
- `-c check_for_update_on_startup=false`: M12. Without it any typed input can install software.
The approval/sandbox and hook-trust flags stay the operator's choice, as
`--dangerously-skip-permissions` is for Claude. *Rejected: documenting the two flags and trusting
the operator.* M12 shows a forgotten flag costs a global package install; a daemon that types into
the pane has to own the flags that make typing safe.

**D6. Liveness compares against a set of names.** `@crswd-binary` changes from one name to names
joined by `|` (each still `[A-Za-z0-9._-]`), and the tmux format becomes
`#{?#{@crswd-binary},#{m/r:^(#{@crswd-binary})$,#{pane_current_command}},?}` (M17). Codex writes
`codex|node`; every other harness writes `startBinary(template)` exactly as today, so a Claude
session's option value and liveness answer are unchanged.
*Rejected: walking `/proc` from the daemon on every sweep.* One tmux format is cheaper and is what
podctl already reuses through `tmuxctl.ArgvList`. *Rejected: requiring the operator to configure
the native binary path.* The npm path is what is installed; a session that reads as permanently
stopped would be revived in a loop.
*Accepted risk:* while Codex runs, any `node` in the pane foreground reads as alive. The pane runs
nothing but the configured command.

**D7. Codex conversations are discovered from `/proc`, not guessed.** Codex cannot be given an id
(no `--session-id`; `ThreadStartParams` has no id field). M6 shows the native process holds its
rollout open, so on each supervisor sweep a running Codex session with no conversation reads
`#{pane_pid}`, walks its descendants through `/proc/<pid>/task/<pid>/children` (depth ≤ 6, ≤ 64
processes), and reads each `/proc/<pid>/fd/*` link. A link matching
`<codexHome>/sessions/YYYY/MM/DD/rollout-*-<uuid>.jsonl` yields the id.
*Rejected: matching rollouts by `cwd` and start time.* Two Codex sessions in one directory make it
a guess, and a wrong guess resumes someone else's conversation.
*Before the first prompt there is no id.* The session is supervised but not revivable by id, which
is exactly how a Claude session created before spec 012 behaves (`reasonNoConversation`). The card
says "conversation not yet started".

**D8. Restarting a Codex session uses a stepped, verified quit.** Continue (spec 013) and revival
type a new command line into the pane. Into a live Codex composer that line becomes a prompt. So
for Codex `restartInto` sends one `C-c`, waits 1.5 s, asks tmux whether the pane is still running
the harness (D6), and repeats, at most 5 presses (M9 converged in 3). If the pane is still running
Codex after the fifth, it returns `ErrQuitUnconfirmed` and types nothing. Claude keeps `C-c C-c`.
*Rejected: `/quit` via paste.* While a turn runs, Codex queues typed input, so `/quit` would wait.

**D9. Prompt and compact use bracketed paste on Codex (M3, M4, M5).**
Spec 018 adds `Controller.PasteBracketed` (`paste-buffer -p`). Claude keeps `Paste`; spec 018 owns
whether Claude moves to bracketed paste too.

**D10. Codex trust is seeded by a line-level edit of `config.toml`.** No TOML library: the root
module has no dependencies (docs/security.md §5). The edit understands exactly one shape, the one
Codex itself writes (M2 and research-codex.md §5): a header line `[projects."<dir>"]` and a
`trust_level = "<value>"` line in that table. Anything else that mentions the directory refuses
rather than edits. Same lock file convention and atomic replace as `trust.go` (`<file>.lock`,
temp file, rename). A missing `config.toml` is created at 0600 when `$CODEX_HOME` exists, because
Codex's own trust acceptance creates the same table in the same file; a missing `$CODEX_HOME`
is left missing (Codex has never run, and will show sign-in first).
*Rejected: `-c 'projects."<dir>".trust_level="trusted"'` on the command line.* It puts a
caller-influenced path into a line typed at a shell. No line crswd types has ever carried a path.

**D11. Codex conversation listing opens the first line of a rollout, bounded.** Spec 013 FR-025
says no transcript is opened, because Claude's file name is the id and its directory is the cwd.
Codex keys rollouts by date, and the cwd exists only inside line 1. The reader opens a rollout,
reads at most 64 KiB, stops at the first newline, and decodes only `type`, `payload.id` and
`payload.cwd`. Nothing it reads is rendered. It scans at most 500 rollout files, newest date
directory first, and returns at most 50 matches. docs/security.md records this amendment.

**D12. Sign-in for Codex is a device-code relay with no paste-back.** `codex login --device-auth`
prints a link and a code (F2); the operator enters the code in a browser and the CLI polls. So the
Codex relay shows the link and the code and has no code form; `Deliver` refuses with
`ErrNoCodeToDeliver`. Status comes from `codex login status`'s exit code (M14): 0 signed in;
1 with `Not logged in` on stdout signed out; anything else unknown.
*Rejected: `codex login` (browser).* It opens a localhost callback a phone cannot reach.

**D13. One relay, one auth cache, one pill per harness that is configured.** The single
`Server.signin` becomes a map keyed by `harness.Name`, built for Claude when the default command is
Claude (unchanged) and for Codex when an entry named `codex` exists. `GET /dashboard/auth` keeps
its wire shape and gains `?harness=codex`; absent means Claude, so the existing client is unchanged.
The create gate asks the relay of the harness the create resolves to.

**D14. Codex quota reads the `codex` provider's `weekly` window (M18).** The reader becomes
`ReadProvider(path, provider, window)`. When the cache has no `codex` provider or no window
the Codex meter is hidden (NC-1, Craig 2026-10-06), which is what this host will show today. The quota-axi `auth_required`
cause is investigated by one spike task (T034) and changes no crswd code.

**D15. Kubernetes mode for Codex is gated on spec 017 and on one measurement.** Operator-run
measurement O-1 (tasks.md) measures whether Codex runs a turn with `auth.json` delivered the way spec
017 FR-015 delivers Claude's credential. It needs a cluster and a real credential, so it is not a
loop task. Phase 4's credential tasks are planned from that measurement
in a follow-up planning pass; this spec does not guess them. The tasks that do not depend on it
(env pass-through, the in-pod conversation lookup, the session image) are written here.

## Not verified

- How an expired Codex login looks mid-session (needs a real expiry).
- The rendered text of Codex's usage-limit message (binary strings only: `hit your usage limit`).
- Whether a trusted parent directory covers a child in Codex.
- Whether `--dangerously-bypass-hook-trust` still shows the hooks dialog on any version.
- Whether Codex tolerates a read-only or access-token-only `auth.json` (O-1).

## Critic findings (Codex critic pass, 10/6/26)

Two `codex exec` critic passes (one aimed at spec 018, which also reviewed 019) returned 50 items;
32 concerned this spec. After dedupe, 29 were verified against the code and folded into tasks.md:

| Finding | Fixed in |
|---|---|
| `[!]` tasks are skipped: `ralph/loop.sh` stops only on a non-zero iteration exit, and an iteration that marks `[!]` exits 0 | every codex-* `PROMPT.md` "Blocked" rule |
| Operator `-c check_for_update_on_startup=true` overrides the daemon's `false`; tokens after `--` treated as flags | T004a, T004 |
| Failed stepped quit after `Continue` persisted (`manager.go:2492-2506`) | T009 |
| `containedIn` (`conversation.go:249`) is lexical; symlinked rollouts | T010a `codexRollout` |
| Discovery write order not retry-safe | T012 (option → journal → store) |
| T014 / T022 / T013 / T003 Files lines missing files they edit | T014, T022a–c, T013, T003 + allowlists |
| No header model: templates pass `.Operator` | T025 `headerView` |
| `binaryOf` drops an absolute path | T021 `executable` |
| T040 needs a cluster and credentials; no cleanup | O-1 (operator-run) |
| `/proc` walk unbounded per process | T011 caps + `ErrDiscoveryBounds` |
| T010, T022 too big | T010a/b, T022a/b/c |
| Multi-pill script races; meter/label pairing | T025, T032 |
| T041 blocked by T040 against D15 | T041 |
| In-pod `CODEX_HOME` default | T042 |
| Other renders "Other" | T016 |
| No radio component exists | T014a |
| Update-check placement for `login` unmeasured | T021 uses top-level `-c` |
| T021 anchor in wrong package | T021 |
| testdata README absent | T013 |
| Harness value multiplicity | T014 `parseHarness`, used by T016, T022b, T024, T031 |
| `Other` routed to Claude conversation lookups | T010b exhaustive dispatch |
| `Type` would lose bracketed paste for Claude | T005 guardrail |
| `cardOf` has no receiver | T013 parameter |
| Codex panel forms default to the Claude relay | T024 hidden field |
| Redirect cannot reopen the Codex dialog | T024 `signin=codex`, T025 |
| No 409 path in sign-in | T024 keeps the 303 refused outcome |
| CRLF and duplicate headers in `config.toml` | T007 |
| T034 cannot run first under topmost-open | T034 moved above T030 |

### Critic findings rejected

- **R-1. M17 stores escaped bars.** Rejected: `\|` in M17 is Markdown table escaping. The bytes
  set with `tmux set-option` and stored in `@crswd-binary` are `codex|node|sleep`, raw, and T002's
  tmux test sets the raw value.
- **R-2. A sentinel for `[!]` in `loop.sh`.** Not rejected but not fixable here: `ralph/loop.sh`
  is outside this spec's allowlists. The PROMPT rule makes a blocked notebook a sequence of no-op
  iterations instead of a skip. A loop-level stop on `[!]` is a separate fix.
- **R-3. Device codes leak through the session pane and its SSE stream (018 critic #2).** Rejected
  as a defect of this spec. Claude sessions on a sign-in screen already show their URL in the pane;
  the pane viewer is the operator's own screen behind the same door that grants a shell. FR-019 is
  about the relay window the daemon itself drives, and is now worded to say so.
