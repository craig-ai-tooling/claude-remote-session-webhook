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
BLOCKED: T034 needs `quota-axi --provider codex --full` and `quota-axi auth`, and this session is refused approval to run quota-axi; run both by hand or allow them, then edit the M19 row in place
