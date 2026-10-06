# Validation contract — Phase 1 — Codex core parity

Behaviour this phase must show when done, checked as a black box.

- A session created with start command `codex` types a line that contains `--no-alt-screen` and `-c check_for_update_on_startup=false` exactly once each, verified by `go test -tags quickstart ./cmd/crswd -run Codex`.
- A Claude session types the same command line as before this phase: `go test ./internal/session/...` passes with no edit to any pre-existing expected Claude line (check with `git diff main -- internal/session/manager_test.go | grep "^-.*--session-id"` printing nothing).
- A Codex session whose pane runs `node` reads as running, and one whose pane runs `bash` reads as stopped, verified by `go test -tags tmux ./internal/tmuxctl/... -run Liveness`.
- Before a Codex start line is typed, the Codex config file holds a trusted table for the session directory, verified by `go test ./internal/session/... -run CreateCodexTrusts`.
- A Codex restart never types a command line while the pane still runs Codex, verified by `go test ./internal/session/... -run NeverTypes`.
- A running Codex session gets its conversation id from the rollout its process holds open and from nowhere else, verified by `go test ./internal/session/... -run Discover`.
- Browser create with `harness=codex` and `remote_control=on` starts nothing and returns the bad-mode outcome, verified by `go test ./internal/httpapi/... -run CodexRefusesRemote`.
