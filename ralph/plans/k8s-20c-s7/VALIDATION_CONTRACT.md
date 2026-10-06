# Validation contract: k8s-20c-s7

Slice S7 of k8s-20c (spec 017): the cluster binary, the images, the crswd-next manifests and the
operator doc. Run every command from the repository root on the milestone branch.

- The host module has no dependency: `test ! -e go.sum` exits 0 and `grep -c require go.mod` prints `0`.
- The root gate is green: `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./... && go test -tags quickstart ./cmd/crswd && golangci-lint run` exits 0.
- The cluster module is green: `go -C k8s vet ./... && go -C k8s test ./... && go -C k8s build ./...` exits 0.
- The host binary still refuses kubernetes mode: `go test -run 'Kubernetes|HostNeverBuildsKubernetes' ./cmd/crswd ./internal/config -v` passes.
- The cluster binary runs kubernetes mode and refuses a host config: `go -C k8s test -run ClusterBuildIsRunnable -v ./cmd/crswd/` exits 0, and `go -C k8s test -run DaemonRefusesHostConfig -v ./cmd/crswd/` exits 0.
- The cluster daemon checks working directories lexically and carries no relay or update feed: `go test -run NewForCluster ./internal/httpapi -v` passes.
- The session image carries both runtimes, pinned: `grep -c '^COPY' deploy/session-image/Dockerfile` prints `2`, and `grep -c f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30 deploy/session-image/fetch-codex.sh` prints `1`.
- The crswd image fetches nothing: `go test -run Dockerfile ./deploy/image ./internal/sessionpod -v` passes.
- The committed manifests are what the generator writes: `go run ./deploy/k8s/gen && git status --porcelain deploy/k8s` prints nothing.
- No real image tag is committed: `grep -c REPLACE deploy/k8s/crswd-next/daemon.json` prints at least `1`.
- No manifest names the lawnmower namespace or the VM's Claude home: `grep -rniE 'lawnmower|\.claude' deploy/k8s/` prints nothing and exits 1.
- The operator doc exists with its sections: `grep -c '^## ' docs/k8s-mode.md` prints at least `7`.
