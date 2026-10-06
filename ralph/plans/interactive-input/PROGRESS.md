# Progress: interactive input (spec 018)

Append one entry per iteration, newest last. Do not rewrite earlier entries.

## Iteration 0: planning (2026-10-06)

- Plan written from measured behaviour. See `specs/018-interactive-input/research.md` R1–R3.
- The probes ran on private `tmux -L` sockets and left no server behind. The
  Codex probe ran in `~/code/dispatch`, which was already trusted, and wrote
  nothing to `~/.codex/config.toml`.
- Not yet dispatched. `ralph/loop.sh` resolves this notebook with
  `RALPH_PLAN=interactive-input`, or on a branch named `plan/interactive-input`.

## Findings

(none yet)
