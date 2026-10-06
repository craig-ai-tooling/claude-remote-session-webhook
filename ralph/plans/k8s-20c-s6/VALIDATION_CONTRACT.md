# Validation contract: k8s-20c-s6

Slice S6 of k8s-20c (spec 017): the reconciler and the session pod template, for Claude Code and
Codex sessions. Run every command from the repository root on the milestone branch.

- The host module is unchanged: `git diff --name-only origin/main...HEAD -- . ':!k8s' ':!specs' ':!ralph'` prints nothing, and `test ! -e go.sum` exits 0.
- The root gate is green: `go build ./... && go vet ./... && go test ./... && golangci-lint run` exits 0.
- The cluster module is green: `go -C k8s vet ./... && go -C k8s test ./... && go -C k8s build ./...` exits 0.
- A session pod reads no Secret through the API, has no host access and no token: `go -C k8s test ./internal/reconcile -run PodForNoSecretReadNoHostAccess -v` passes.
- Every pod can run either runtime: `go -C k8s test ./internal/reconcile -run PodForBothRuntimes -v` passes.
- The lifetime holds with the reconciler down: `go -C k8s test ./internal/reconcile -run PodForDeadline -v` passes.
- No replacement while the old pod exists, and never a forced delete: `go -C k8s test ./internal/reconcile -run Reconcile -v` passes, with the terminating-pod case at zero creates and the delete case asserting `GracePeriodSeconds` is nil.
- Containment holds for objects written around the daemon: `go -C k8s test ./internal/reconcile -run 'Reconcile.*(Outside|Cap)' -v` passes, and those cases end `Rejected` with zero pods.
- One reconciler acts at a time: `go -C k8s test ./internal/reconcile -run Lease -v` passes.
- The plan records the dropped finalizer: `grep -n 'finalizer' specs/017-k8s-native-execution/plan.md` prints a line.
