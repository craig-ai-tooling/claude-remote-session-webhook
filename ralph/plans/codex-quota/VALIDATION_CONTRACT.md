# Validation contract — Phase 3 — Codex usage meter

Behaviour this phase must show when done, checked as a black box.

- `GET /dashboard/quota?harness=codex` reads the `codex` provider `weekly` window and `GET /dashboard/quota` answers exactly as before, verified by `go test ./internal/httpapi/... -run DashboardQuota`.
- With no `codex` provider in the cache the Codex meter label prints `codex quota: unknown`, verified by `go test ./internal/httpapi/... -run CodexMeter`.
- With no `codex` start command configured the header markup is byte-identical to before, verified by `go test ./internal/httpapi/... -run MeterUnchangedWithoutCodex`.
