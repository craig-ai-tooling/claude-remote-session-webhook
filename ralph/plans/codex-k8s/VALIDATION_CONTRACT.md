# Validation contract — Phase 4a — Codex in kubernetes mode

Behaviour this phase must show when done, checked as a black box.

- research.md holds measurements M20 to M23 and names which Phase 4b design applies, verified by `grep -c "^| M2[0-3] " specs/019-codex-runtime/research.md` printing 4.
- A session pod tmux environment carries `CODEX_HOME` when the daemon has it, verified by `go test ./internal/sessionpod/... -run PassThrough`.
- In kubernetes mode a Codex conversation id is read inside the pod and recorded on the session, verified by `go -C k8s test ./... -run CodexConversation`.
