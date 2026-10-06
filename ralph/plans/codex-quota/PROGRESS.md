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

## Iteration 3 (T031, 10/6/26)

- `readQuota` now takes a `harness.Name`: codex reads provider `codex` window `weekly`, anything else reads `claude`/`seven_day`. `dashboardQuota` parses `?harness=` with `parseHarness` and answers 400 via `rejectBadRequest(errHarnessParam)` on an unknown, empty or repeated value. Response shape unchanged.
- Added `TestDashboardQuotaCodex`, `TestDashboardQuotaCodexMissingIsUnknown`, `TestDashboardQuotaHarnessInvalid`. They failed before the change (codex returned Claude's value, bad values returned 200) and pass after. No existing test needed editing.
- Ran gofmt, build, vet, test, golangci-lint: all clean. tmux, quickstart and k8s checks not applicable.
- Note: `go -C` is refused by the approval allowlist in this session; bare `go` commands from the worktree work.
- Left: T032, T033.
