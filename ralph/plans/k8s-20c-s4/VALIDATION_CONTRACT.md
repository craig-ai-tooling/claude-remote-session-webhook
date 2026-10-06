# Validation contract: k8s-20c-s4

Slice S4 of k8s-20c (spec 017): the nested `k8s/` module with client-go and the Lease elector.
Run every command from the repository root on the milestone branch.

- The host module is unchanged: `git diff --name-only origin/main...HEAD -- go.mod` prints nothing, `test ! -e go.sum` exits 0, and `grep -c require go.mod` prints `0`.
- The root gate is green: `go build ./... && go vet ./... && go test ./... && golangci-lint run` exits 0.
- The cluster module is green the way CI runs it: `go -C k8s mod download && go -C k8s vet ./... && go -C k8s test ./... && go -C k8s build ./...` exits 0, and `golangci-lint run` from inside `k8s/` exits 0.
- The module stays on the root's Go version: `grep -n '^go ' k8s/go.mod` prints `go 1.23.0` and `grep -c '^toolchain' k8s/go.mod` prints `0`.
- client-go is the pinned version and controller-runtime is absent: `grep -n 'k8s.io/client-go v0.32.8' k8s/go.mod` prints one line and `grep -c controller-runtime k8s/go.mod` prints `0`.
- Exactly one elector leads: `go -C k8s test ./internal/kube -run ExactlyOneLeads -v` passes.
- Nothing under the root imports the cluster module: `go -C k8s test ./internal/kube -run Boundary -v` passes.
- Dependabot watches the cluster module: `grep -n 'directory: "/k8s"' .github/dependabot.yml` prints one line.
