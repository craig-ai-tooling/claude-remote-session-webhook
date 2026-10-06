# Validation contract: k8s-20c-s5

Slice S5 of k8s-20c (spec 017): podctl, the in-pod helpers and the Manager hooks, for Claude Code
and Codex sessions. Run every command from the repository root on the milestone branch.

- The host module is unchanged in shape: `test ! -e go.sum` exits 0, `grep -c require go.mod` prints `0`, and `git diff --diff-filter=M --name-only origin/main...HEAD -- '*_test.go'` prints nothing.
- The root gate is green: `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./... && go test -tags quickstart ./cmd/crswd && golangci-lint run` exits 0.
- The cluster module is green: `go -C k8s vet ./... && go -C k8s test ./... && go -C k8s build ./...` exits 0.
- podctl is a Controller: `grep -n 'var _ tmuxctl.Controller = (\*Controller)(nil)' k8s/internal/podctl/podctl.go` prints one line.
- No byte that reaches a pane is built by new code: `go -C k8s test ./internal/podctl -run 'SendKeys|Paste|Resize|SetOption' -v` passes, and those tests compare each argv with `tmuxctl.Argv*`.
- A paste payload never rides an argv: `go -C k8s test ./internal/podctl -run Paste -v` passes and asserts the payload is on stdin and in no recorded argv.
- Teardown is confirmed, not assumed: `go -C k8s test ./internal/podctl -run Kill -v` passes, including the case where the pod object still exists and `Kill` errors.
- A recreated pod is revivable: `go -C k8s test ./internal/podctl -run List -v` passes, including the option-replay case.
- A Codex session's pod carries `CODEX_HOME` and its conversation is found inside the pod: `go test ./internal/sessionpod -run 'PassThrough|CodexConversation' -v` passes and `go -C k8s test ./internal/podctl -run CodexConversation -v` passes.
- A fresh pod gets no first-run screen: `go test ./internal/sessionpod -run Seed -v` passes.
- The supervisor can revive a pod session: `go test ./internal/session -run 'SuperviseLexical|TranscriptChecker|CodexConversationFinder' -v` passes.
