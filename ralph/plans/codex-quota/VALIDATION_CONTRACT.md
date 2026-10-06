# Validation contract — Phase 3 — Codex usage meter

Behaviour this phase must show when done, checked as a black box.

- `GET /dashboard/quota?harness=codex` reads the `codex` provider `weekly` window and `GET /dashboard/quota` answers exactly as before, verified by `go test ./internal/httpapi/... -run DashboardQuota`.
- The Codex label and meter are each rendered `hidden` and are shown together only when a `weekly` window exists, verified by `go test ./internal/httpapi/... -run 'CodexMeterStartsHidden|HidesCodexWithoutWindow'`.
- With no `codex` start command configured the header carries no Codex element and the Claude label and meter differ from before only by `data-harness="claude"`, verified by `go test ./internal/httpapi/... -run MeterUnchangedWithoutCodex`.
