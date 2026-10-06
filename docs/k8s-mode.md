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

Neither is a reduced version of the other. The host install is unchanged. The Kubernetes column is
[Install with Helm](#install-with-helm), or the hand install above it.

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

## Install with Helm

The chart is `deploy/chart`, published as an OCI artifact. Images and chart share one version,
`2.0.N` for commit count N, built by `.github/workflows/images.yml` and pushed to `ghcr.io`.
Make the three packages (`crswd`, `crswd-session`, `charts/crswd`) public once, because the
cluster has no pull secret.

The chart creates no secret values. It references Secrets you create in the release namespace:

| Secret | Key | Needed |
|---|---|---|
| `sharedSecret.existingSecret` | `secret` (`openssl rand -hex 32`) | always |
| `dashboardPassword.existingSecret` | `password` | for the password door |
| `access.allowedEmailsSecret.name` | `access.allowedEmailsSecret.key` (comma-separated addresses) | for the Cloudflare Access door |
| `claudeCredentials.secretName` (default `claude-credentials`) | the keeper login | for Claude Code sessions |
| `codexAuth.secretName` (default `codex-auth`) | the namespace's Codex login | for Codex sessions |
| `cloudflared.tokenSecret` | `token` | when `cloudflared.enabled` |

Values:

| Key | Default | Meaning |
|---|---|---|
| `image.repository`, `image.tag`, `image.pullPolicy` | `ghcr.io/craig-ai-tooling/crswd`, chart `appVersion`, `IfNotPresent` | daemon and reconciler image |
| `sessionImage.repository`, `sessionImage.tag` | `ghcr.io/craig-ai-tooling/crswd-session`, chart `appVersion` | what a session pod runs |
| `reconciler.namespace` | `<release namespace>-reconciler` | where the reconciler and its Lease live |
| `reconciler.createNamespace` | `true` | create that namespace |
| `sharedSecret.existingSecret` | none, required | HMAC secret for the API |
| `dashboardPassword.existingSecret` | none | the password door, no Access |
| `startCommands` | Claude Code and `codex` | `CRSW_START_COMMANDS`, both runtimes (FR-021) |
| `sessionNode` | none, required | the amd64 node that holds the claim; every session pod runs there |
| `claudeCredentials.secretName`, `codexAuth.secretName` | `claude-credentials`, `codex-auth` | per-namespace logins (FR-022) |
| `allowedRoots` | `/work` | `CRSW_ALLOWED_ROOTS` |
| `maxSessions` | `10` | `CRSW_MAX_SESSIONS` |
| `storage.storageClassName`, `storage.size` | `linstor-replicated`, `50Gi` | the claim `crswd-sessions` |
| `nodeSelector`, `tolerations` | none, the DRBD lost-quorum toleration | daemon pod placement |
| `service.enabled`, `service.port` | `false`, `8765` | ClusterIP Service, and the listener moves to `0.0.0.0` |
| `access.teamDomain`, `access.aud`, `access.allowedEmailsSecret.name`, `access.allowedEmailsSecret.key` | none | the Cloudflare Access door; the allow list is read from an existing Secret, never a literal value |
| `cloudflared.enabled`, `cloudflared.image`, `cloudflared.tokenSecret` | `false`, image pinned by digest, none | tunnel sidecar over loopback |

`reconciler.namespace` must differ from the release namespace, and `sessionNode` must be set;
the chart fails at render time otherwise.

To bump cloudflared, look up the digest of the new tag with
`docker buildx imagetools inspect docker.io/cloudflare/cloudflared:<tag>`, then set
`cloudflared.image` to `docker.io/cloudflare/cloudflared@sha256:<digest>` in
`deploy/chart/values.yaml` and keep the tag in the comment beside it.

`service.enabled` fails at render time unless `access.teamDomain` or
`dashboardPassword.existingSecret` is set, because the daemon refuses a non-loopback listener
with no browser door.

```
kubectl create namespace crswd
kubectl -n crswd create secret generic crswd-shared-secret --from-literal=secret="$(openssl rand -hex 32)"
SESSION_NODE=$(kubectl get nodes -l kubernetes.io/arch=amd64 -o name | head -1 | cut -d/ -f2)  # every session pod runs on this node, beside the session disk
helm install crswd oci://ghcr.io/craig-ai-tooling/charts/crswd -n crswd --set sharedSecret.existingSecret=crswd-shared-secret --set sessionNode="$SESSION_NODE"
helm upgrade crswd oci://ghcr.io/craig-ai-tooling/charts/crswd -n crswd --reuse-values
helm uninstall crswd -n crswd
```

The CRD is in the chart's `crds/` directory, and Helm installs it once and never upgrades or
deletes it. The claim carries `helm.sh/resource-policy: keep`, so `helm uninstall` leaves it and
every conversation. Finish with the Teardown steps below for the claim, the reconciler namespace
and the retained volume.

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
