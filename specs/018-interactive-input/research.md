# Research: Interactive Input (spec 018)

These measurements were taken 2026-10-06 on this host: tmux 3.4, Claude Code
2.1.290, codex-cli 0.153.4. Each probe ran on a private `tmux -L` socket and
killed it afterwards. The broad option survey, with every file anchor, is in
the session scratchpad (`research-input.md`). This file keeps the decisions.

## R1 — Multi-line text needs bracketed paste (spike: done)

**Question.** `Paste` runs `paste-buffer` without `-p`, so tmux types each LF as
a CR. Does a two-line message arrive as one message?

**Probe.** Load `alpha line one\nbeta line two` into a buffer. Paste it into an
idle TUI once with `paste-buffer -p -d` and once with `paste-buffer -d`. Capture
the pane both times. Nothing pressed Enter afterwards.

| Program | `-p` (bracketed) | no `-p` |
|---|---|---|
| Claude Code 2.1.290 | both lines held in the input box, unsent | **line one submitted** as a prompt, line two left typed |
| Codex 0.153.4 `--no-alt-screen` | both lines held, unsent | both lines held (Codex's own paste-burst detection) |

Ctrl-C (`C-c`) cleared the input box in both programs.

**Decision.** The interactive route uses a new `PasteBracketed` (`paste-buffer -p
-d`), and then sends `Enter` when asked. Claude needs `-p` to receive one message.
Codex is correct either way, so `-p` is right for both.

**Rejected.** (a) Change `Paste` itself to `-p`. That changes the API door's
`POST /prompt` and `Compact`. `Compact` pastes `/compact\n` and depends on the LF
becoming a CR, and under `-p` that LF would sit in the input box unsubmitted.
That is a behaviour change to two shipped paths for no reported fault. (b) Forbid
newlines. Multi-line prompts are the ordinary case.

**Consequence: FR-004's control-character rule.** Inside a bracketed paste the
bytes `ESC [ 2 0 1 ~` end the paste, and everything after them arrives as
keystrokes. Refusing every C0 control except LF and TAB, plus DEL, makes that
payload unrepresentable. Keys have their own route.

**Pinned by.** A `-tags tmux` test runs `printf '\e[?2004h'; cat -v` in a real
pane (mode 2004 is what makes tmux honour `-p`). It pastes with `PasteBracketed`
and asserts the capture holds `^[[200~` and `^[[201~` around the text. It cannot
run Claude or Codex in CI. The table above is the evidence for them.

## R2 — `history-limit` takes effect only before the pane exists

| Probe | `#{history_limit}` of the pane |
|---|---|
| `new-session`, then `set-option -t =a: history-limit 10000` | **2000** (unchanged) |
| `set-option -g history-limit 10000`, then `new-session` (two commands, server running) | 10000 |
| `set-option -g history-limit 10000 ; new-session -d …` as **one** invocation, **no server running** | 10000 |

`set-option -g` alone fails when no server is running ("no server running"). The
chained form starts the server and applies the option before the pane is created.

**Decision.** `argvNew` becomes `tmux set-option -g history-limit 5000 ;
new-session -d -s <name> -c <dir>`, with `";"` as a separate argv element. That
is not a shell string: tmux's own parser splits commands on a standalone `;`
argument. Every session gets the limit. That covers create, revival, the
sign-in relay window, and the spec-017 pod that runs `ArgvNew`.

**Why 5000, not 10000 or unlimited.** tmux stores history as grid cells. At 80
columns, 5000 lines is 400,000 cells per session. At the 5-session default cap
that is well under 100 MB even at the 200-column extreme. It is 2.5× today's
2000, and one screen of Claude output is roughly 40 lines, so 5000 lines is
about 125 screens. The capture and the bound use the same constant, so the
viewer shows everything tmux kept.

**Rejected.** (a) Raise the limit from the operator's `~/.tmux.conf`. The daemon
runs its own `-L` server, and a config file is a second source of truth for a
bound. (b) A configuration knob. Principle VI keeps bounds as constants unless
there is a reason to vary them, and nothing here is one.

## R3 — The key allowlist, measured

`send-keys` with each name, into `stty -icanon -echo -isig; cat -v`:

| Symbolic | tmux name | Bytes seen |
|---|---|---|
| escape | `Escape` | `^[` |
| tab | `Tab` | TAB |
| backtab | `BTab` | `^[[Z` |
| up / down / right / left | `Up` `Down` `Right` `Left` | `^[[A` `^[[B` `^[[C` `^[[D` |
| pageup / pagedown | `PageUp` `PageDown` | `^[[5~` `^[[6~` |
| backspace | `BSpace` | `^?` |
| interrupt | `C-c` | `^C` |
| enter | `Enter` | CR |

**Why a symbolic name and not the tmux name on the wire.** tmux sends an unknown
key name as literal text, with no error. A route that forwarded a caller's
string would be `send-keys` with caller text, which `docs/security.md` §2 and the
`Controller.SendKeys` contract forbid. The browser sends `escape`, and the daemon
maps that to its own constant, so `send-keys` keeps receiving daemon constants
only.

**Why no printable keys (`y`, `n`, `1`…).** They are text, and Type only covers
them. One rule for each kind of input.

## R4 — Answer shape: 204, not the redirect every action uses

Every action today answers `303` to the fleet page. The shared fetch handler
(`web/static/crswd.js:1243`) follows that redirect, downloads the whole fleet
page and pulls one sentence out of it for the toast. That is fine for a destroy.
For a key press it would render the fleet once per tap.

**Decision.** Success is `204` with no body. A browser with no script stays on the
page when it receives a 204, which keeps the action usable without script.
Refusals keep the `303` and an outcome code, so the banner vocabulary, the
no-script landing and `TestRefusalIsNotARedirect`'s shapes all still apply. A
dedicated listener in `crswd.js` handles the two input forms. The shared
handler skips any form marked `data-session-input`.

## R5 — Rate: one bucket for both routes

The generic token bucket (`internal/httpapi/ratelimit.go:90`) refills at
`perMinute` and holds `perMinute/2`. 240 per minute gives 4 per second sustained
and a burst of 120, which covers a fast run of arrow presses. The budget is
per operator, not per session: one person typing is one budget, and splitting
by session would multiply it by the cap. It is a constant (`inputRatePerMin`),
as `loginRatePerMin` is.

## R6 — Why not a terminal (recorded so it is not re-argued)

The terminal options would need a hand-rolled RFC 6455 WebSocket, since the
stdlib has none and `go.sum` is forbidden. They would need vendored xterm.js,
which goes against "hand-written JS". They would hand raw escape bytes to the
browser, which contradicts "strip ANSI server-side" (Constitution VII). And
xterm.js injects a `<style>` element that `style-src 'self'` may block
(unverified). On iOS it would still need this key bar. If (a) proves too slow, the
next step is tmux control mode (`tmux -C` works over pipes, no pty), with an
amendment to the constitution first. That is not in this feature.

## R8 — Codex's "Update now" menu is spec 019's to prevent, not this key bar's

Measured during spec 019's research (Codex 0.153.4): a Codex session started
**without** `-c check_for_update_on_startup=false` can open on a blocking
"Update available" menu whose default selection is "Update now". A blind
paste plus Enter selected it and started `npm install -g @openai/codex`, a
global package install on the host. An interrupt rolled it back.

Spec 019 always passes `-c check_for_update_on_startup=false` to every Codex
start, resume and sign-in command, so the menu is never drawn. That is the fix,
and it lives at launch. This spec does not gate typed text or keys on the
menu: input stays ungated (FR on dialogs, Edge Cases), and the key bar is not a
place to detect or refuse it. An operator who sees the menu on the live pane can
answer it with the arrow keys and Enter like any other dialog.

## R7 — Echo latency

The stream captures once a second. Typed text appears in the pane within about
1s. A faster cadence while the input has focus was considered and is left out.
It changes a shared cost bound (`streamInterval`) for every viewer of the
session, and nobody has reported 1s as a problem yet.
