# Running harnesses other than Claude Code

**Status: shipped for Codex (spec 019).** Claude Code and Codex are the two runtimes the
dashboard knows. Any other command still runs, as `Other`, with no per-runtime behaviour.

## How a harness is chosen

A harness is **derived from the configured command, never stored**. `harness.Of(command)` is
the base name of the command's first token, with one trailing `.js` removed: `claude` is Claude
Code, `codex` is Codex, anything else is Other. A command that starts with a wrapper
(`env FOO=1 claude`) is Other. The session records its start-command *name*, as it always has,
and the harness is recomputed from the configured command each time it is needed, so it cannot
disagree with the configuration it came from. `internal/harness` is the only place a binary name
is compared to `claude` or `codex`.

A create still carries **a name and never a command line**. Codex is a named entry in
`start_commands`:

```ini
start_commands = codex=/home/nctiggy/sf-cli/bin/codex --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust
```

The browser create form offers a Runtime choice only when an entry named exactly `codex` exists
and its command is Codex. The signed API names it with `start_command`, as before.

## The three questions this note used to ask

**Is the goal "run another tool" or "run another tool as well as this one runs Claude Code"?**
The second, for Codex. What differs between runtimes is data in `harness.Spec`: which process
names tmux reports, which flags crswd must add, how a conversation resumes, whether input is
pasted with brackets, whether a restart quits one key at a time, and whether remote control
exists. Every other harness gets the `Other` spec and the behaviour it had before.

**Does `Mode` generalise, or does it gain a sibling?** Neither. `ModeRemote` means
`claude --remote-control`, so Mode stays Claude's. A Codex session is always `local`, the
browser refuses `remote_control=on` with `harness=codex`, and changing mode on a non-Claude
session is refused. Harness and mode are different axes. Codex's `remote-control` pairs a
phone app with an app-server daemon and does not attach to a TUI in tmux, so it is a non-goal;
typed input (spec 018) is how a Codex session is driven from a phone.

**What does the pill say for a harness the daemon has no states for?** `running` and `stopped`
come from tmux alone and hold for every harness. A blocking screen is recognised per harness:
`DetectDialogFor` reads Claude's signatures unchanged for Claude and a separate Codex set
(trust, hooks review, command approval, update) for Codex. Other tries Claude's and then
Codex's, so a dialog from either is still seen. Codex idle and working screens are never read
as dialogs.

## What is different for Codex

| Concern | Behaviour |
|---|---|
| Flags crswd adds | `--no-alt-screen` (without it tmux keeps no scrollback) and `-c check_for_update_on_startup=false` (without it typed input can install software). Skipped when the exact tokens are already present. A configured command that sets the update check back to true is a startup refusal |
| Liveness | `@crswd-binary` is `codex\|node`, because the npm launcher is a node process. While Codex runs, any `node` in the pane foreground reads as alive |
| Trust | A `[projects."<dir>"]` table with `trust_level = "trusted"` in `$CODEX_HOME/config.toml`, seeded before the line is typed |
| Prompt, compact | Bracketed paste (`paste-buffer -p`). Claude keeps plain paste |
| Conversation id | Cannot be given at start. Found from `/proc`: the native process holds its rollout file open. Before the first prompt there is no id and the card says so |
| Restart, continue | Stepped quit: one `C-c`, 1.5 s, check the pane, at most 5 presses. Still running after the fifth returns `ErrQuitUnconfirmed` and types nothing |
| Conversation list | Reads the first line of each rollout, bounded (see `docs/security.md`) |
| Approval and sandbox flags | The operator's choice, as `--dangerously-skip-permissions` is for Claude |

## What is not built

Sign-in relay for Codex, the Codex quota meter and Kubernetes mode for Codex belong to later
phases of spec 019. Do not describe them as working.

## What is not in question

A create keeps carrying **a name and never a command line**. That is the property that makes
an operator's configured set an allowlist rather than a suggestion.
