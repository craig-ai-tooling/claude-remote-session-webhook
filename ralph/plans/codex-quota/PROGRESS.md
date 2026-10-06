# Progress — Phase 3 — Codex usage meter

Notebook for `ralph/plans/codex-quota`. Each iteration appends below.

## Iteration 0 (planning, 10/6/26)

- Plan written from `specs/019-codex-runtime/`. No code changed.
- Measurements behind every Codex behaviour are in `specs/019-codex-runtime/research.md` section M.
- Left: every task in `IMPLEMENTATION_PLAN.md`, starting with T030.

## Iteration 1 (T034, 10/6/26)

- Wrote the `M19` row in `specs/019-codex-runtime/research.md` from reading `quota-axi/dist/src/providers/codex.js`.
- **T034 left `- [ ]` on purpose.** The task requires running `quota-axi --provider codex --full` and `quota-axi auth`; both were refused approval in this session, so the failing request and status are not captured. The row says so and carries a hypothesis (expired `auth.json` access token), not a verdict.
- Left: run those two commands, then edit the existing M19 row in place (do not add a second one) and tick T034. Then T030 onward.
- Not run: gofmt, build, vet, test, lint. Only markdown changed.

## NEEDS CLARIFICATION

(none yet)
- Operator (CS, 2026-10-06): ran both quota-axi commands and measured the endpoints; M19 rewritten with the result. T034 done.

## Iteration 2 (T030, 10/6/26)

- Added `quota.ReadProvider(path, provider, window)`, `ErrNoProvider`, `ErrNoWindow`. `Read` now calls it with `claude`/`seven_day`. The old sentinel names are aliases, so existing callers and tests are untouched.
- Tests `TestReadProviderCodexWeekly` and `TestReadProviderMissing` failed to compile before the change and pass after.
- `ErrInvalidPercent` text still says "seven_day"; left as is, T030 did not name it.
- Ran gofmt, build, vet, test, golangci-lint: all clean. tmux, quickstart and k8s checks not applicable.
- Left: T031 onward.
