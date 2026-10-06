# Kubernetes mode

Operator guide for running crswd with `CRSW_EXECUTION_MODE=kubernetes` (spec 017). Read
`docs/security.md` first: a request that passes auth is still code execution, now inside a pod.

## What kubernetes mode is

One `AgentSession` object is one session, and one pod runs it. The daemon turns a dashboard or
API request into an `AgentSession`; a reconciler turns that object into a pod with tmux and the
chosen runtime inside it. The daemon never runs a session itself.

Two Deployments, each in its own namespace (FR-009):

| Deployment | Namespace | Runs | Does |
|---|---|---|---|
| `crswd` | `crswd-next` | `crswd` (no arguments) | the dashboard and API, one `AgentSession` per session |
| `crswd-reconciler` | `crswd-next-reconciler` | `crswd reconcile` | holds a Lease, creates and heals session pods |

Session pods live in `crswd-next`. The reconciler holds the pod-create permission and sits in a
namespace where the daemon has no `pods/exec`. The internet-facing daemon gets only the narrow
permission (FR-009).

Both Deployments use one image, built from `k8s/cmd/crswd`. That binary is the cluster build and
runs `kubernetes` mode only. Given a host configuration it refuses and points at the release
binary. The host binary refuses `kubernetes` mode in turn.

## The two deployments

crswd ships two supported deployments, and both run Claude Code and Codex sessions:

| | Host | Kubernetes |
|---|---|---|
| Install | `install.sh` or the release binary, `crswd unit install` (systemd user unit) | Helm chart (k8s-20): CRD, RBAC, daemon and reconciler Deployments, claim |
| Sessions | tmux on the host | one pod per `AgentSession` |
| Upgrade | the dashboard's self-updater | the chart (FR-003) |
| Sign-in | the host's own logins and the dashboard relays | per-namespace logins (FR-015, FR-016, FR-022) |

Neither is a reduced version of the other. The host install is unchanged. Until the Helm chart
exists, the Kubernetes column is this document.

## Images and how to build them

Two images. Both are built by the operator; no tag is committed with a real sha.

**crswd** (the daemon and the reconciler, `linux/amd64` and `linux/arm64`), tag
`docker.io/nctiggy/crswd:2.0.0-dev.<sha7>`:

```
BIN=$(mktemp -d)
for a in amd64 arm64; do CGO_ENABLED=0 GOOS=linux GOARCH=$a go -C k8s build -trimpath -ldflags "-X main.version=2.0.0-dev.$(git rev-parse --short=7 HEAD)" -o "$BIN/crswd-$a" ./cmd/crswd; done
docker buildx build --platform linux/amd64,linux/arm64 -f deploy/image/Dockerfile -t docker.io/nctiggy/crswd:2.0.0-dev.$(git rev-parse --short=7 HEAD) --push "$BIN"
```

**crswd-session** (what a session pod runs, `linux/amd64` only), tag
`docker.io/nctiggy/crswd-session:2.1.246-codex0.153.4-<sha7>`. It holds tmux and Claude Code from
the base image, Codex from `fetch-codex.sh` (pinned version, checked against a SHA-256), and the
cluster `crswd` binary:

```
BIN=$(mktemp -d)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go -C k8s build -trimpath -o "$BIN/crswd" ./cmd/crswd
deploy/session-image/fetch-codex.sh "$BIN"
docker build --network=host -f deploy/session-image/Dockerfile \
  -t docker.io/nctiggy/crswd-session:2.1.246-codex0.153.4-$(git rev-parse --short=7 HEAD) "$BIN"
```

Never push a tag twice. Pods pull with `IfNotPresent`, so a node keeps the layer it already has
under a name you have since reused.

## Install into crswd-next by hand

The manifests in `deploy/k8s/crswd-next/` are generated (`go run ./deploy/k8s/gen`) and carry
`REPLACE` in place of each image's sha7. Replace it with the sha7 you built, regenerate, and keep
that change uncommitted. A test fails if a real tag reaches the tree.

1. Create the namespaces and service accounts:
   `kubectl apply -f deploy/k8s/crswd-next/namespaces.json`
2. Create the shared secret. Use a fresh random value, never the VM's. `crswd keygen` prints a
   release-signing key pair and is the wrong tool here:
   `kubectl -n crswd-next create secret generic crswd-shared-secret --from-literal=secret="$(openssl rand -hex 32)"`
3. Apply the CRD, then the RBAC files:
   `kubectl apply -f deploy/k8s/crd.json`, then `deploy/k8s/rbac-daemon.json`,
   `deploy/k8s/rbac-reconciler.json` and `deploy/k8s/rbac-lease.json`.
4. Apply the rest of `deploy/k8s/crswd-next/`: the service accounts, the claim, the placeholder
   `claude-credentials` Secret (empty, so no pod is logged in by accident), then
   `reconciler.json` and `daemon.json`.
5. Reach the dashboard, which has no Service:
   `kubectl -n crswd-next port-forward deploy/crswd 8765`

Sessions run on `lm-amd64-1`, because the session image is amd64 only and the claim is
`ReadWriteOnce` on one node (FR-008). The claim `crswd-sessions` is 50Gi on `linstor-replicated`.

## Runtimes

- **Claude Code** uses the login keeper's session contract (FR-015, FR-016). The namespace has its
  own keeper login, never a copy of the VM's.
- **Codex** signs in with the namespace's own login (FR-022), never a copy of the host's
  `auth.json`. Its credential lives in the `codex-auth` Secret that the reconciler names in the
  pod spec. The background is in spec 019 research M21 to M23.

`CRSW_START_COMMANDS` on the daemon lists both runtimes, so `spec.startCommand` resolves to
either one (FR-021). Codex running fully inside a pod is completed by spec 019 Phase 4a
(`ralph/plans/codex-k8s`) and 4b.

## What is off in this mode

The daemon does not build these in the cluster build (FR-003):

- the self-updater (`internal/updater`): upgrade by deploying a new image;
- the sign-in relays (`internal/loginrelay`): sign-in is the namespace's keeper login and Codex
  login;
- `crswd unit`: there is no systemd user unit in a pod.

The settings page and the start check ask the same question and agree in both binaries.

## Teardown

The claim's class keeps its volume (`Retain`), so deleting the claim leaves data behind.

1. Delete the session objects and the Deployments, then the claim:
   `kubectl -n crswd-next delete agentsessions --all`, then
   `kubectl delete -f deploy/k8s/crswd-next/` (this removes the namespaces too).
2. Find the Released volume: `kubectl get pv | grep crswd-sessions`.
3. Patch its reclaim policy so the provisioner removes it:
   `kubectl patch pv <name> -p '{"spec":{"persistentVolumeReclaimPolicy":"Delete"}}'`
4. Confirm it is gone: `kubectl get pv`. This is standard behavior, but it has not been verified
   on this storage driver, so do not skip the check.
