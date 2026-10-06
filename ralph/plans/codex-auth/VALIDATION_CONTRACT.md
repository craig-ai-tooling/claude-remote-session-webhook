# Validation contract — Phase 2 — Codex sign-in

Behaviour this phase must show when done, checked as a black box.

- With a `codex` start command configured, `GET /dashboard/auth?harness=codex` answers `{"state":...}` from `codex login status`, and with no `harness` it answers exactly as before, verified by `go test ./internal/httpapi/... -run DashboardAuthHarness`.
- A Codex create on a host signed out of Codex is refused while a Claude create is not, verified by `go test ./internal/httpapi/... -run CreateCodexGated`.
- The Codex sign-in panel shows the device link and code and offers no code form, and the code never reaches the audit trail, verified by `go test ./internal/httpapi/... -run CodexCode`.
- A Codex pane on its sign-in or device-code screen renders `needs-auth`, verified by `go test ./internal/httpapi/... -run NeedsAuth`.
