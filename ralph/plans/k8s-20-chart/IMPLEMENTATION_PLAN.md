# Implementation plan: k8s-20-chart

k8s-20, the Kubernetes-native install: a Helm chart, images published by a workflow, CI that
lints and renders the chart, and a README section that puts the Kubernetes install beside the
host install (spec 017 "Deployment options", FR-003, FR-009, FR-013, FR-021). The definition of
done is `ralph/plans/k8s-20-chart/VALIDATION_CONTRACT.md`.

After this lands there are two first-class installs, and both run Claude Code and Codex:
- **Host**: `install.sh` (or the release binary) plus `crswd unit install`, updated from the
  dashboard. Unchanged by this plan.
- **Kubernetes**: `helm install crswd oci://ghcr.io/craig-ai-tooling/charts/crswd`, upgraded
  with `helm upgrade`.

## Dependency

S7 is merged. Check, from the repo root:

`test -f k8s/cmd/crswd/main.go && test -f deploy/image/Dockerfile && test -f deploy/session-image/fetch-codex.sh && test -f docs/k8s-mode.md`

If it fails, the reason is "spec 017 S7 not merged".

## Tasks

Take the topmost open task. One per iteration.

- [x] T1: Allow the loop's helm commands per Design §1. Verify: `grep -c 'Bash(helm lint:\*)\|Bash(helm template:\*)' .claude/settings.json` prints `2`.
- [x] T2: The generator writes the chart's CRD and RBAC rules per Design §2. Run `go run ./deploy/k8s/gen` to write them. Verify: `go test ./deploy/k8s/...` exits 0 (the drift test compares the generator's bytes with the files on disk).
- [x] T3: Chart skeleton, values, helpers and NOTES per Design §3. Verify: `helm lint deploy/chart --set sharedSecret.existingSecret=s` exits 0.
- [x] T4: Namespaces, ServiceAccounts and RBAC templates per Design §4. Verify: `helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s` exits 0 and `go test -run ChartRBAC ./deploy/chart -v` passes.
- [x] T5: The claim, the two Deployments, the optional Service and the optional cloudflared sidecar per Design §5. Verify: `helm lint deploy/chart --set sharedSecret.existingSecret=s` exits 0, `helm template t deploy/chart -n crswd` exits non-zero, and `go test ./deploy/chart -v` passes.
- [x] T6: CI renders the chart, and a workflow publishes images and the chart, per Design §6. Verify: `grep -c 'helm lint deploy/chart' .github/workflows/ci.yml` prints `1` and `grep -c 'packages: write' .github/workflows/images.yml` prints `1`.
- [x] T7: README "Install on Kubernetes" and the Helm section of `docs/k8s-mode.md` per Design §7. Verify: `go test ./internal/release/ ./internal/config/` exits 0 and `grep -c '^## Install on Kubernetes' README.md` prints `1`.
- [x] T8: Run every command in VALIDATION_CONTRACT.md, record each result in PROGRESS.md, then append `RALPH_COMPLETE`. Verify: `go test ./... && helm lint deploy/chart` exits 0.

## Files touched

A diff outside this list is rejected.

- `.claude/settings.json` (T1, two allow entries only)
- `deploy/k8s/manifest/chart.go`, `deploy/k8s/manifest/chart_test.go` (new, T2)
- `deploy/k8s/manifest/drift_test.go` (T2: also checks the chart files)
- `deploy/k8s/gen/main.go` (T2: also writes the chart files)
- `deploy/chart/` (new: `Chart.yaml`, `values.yaml`, `.helmignore`, `templates/*`, `crds/*`,
  `files/rules.json`, and `chart_test.go` plus `doc.go` for the render-free Go tests)
- `.github/workflows/ci.yml` (T6: the helm steps only)
- `.github/workflows/images.yml` (new, T6)
- `README.md` (T7)
- `docs/k8s-mode.md` (T7)
- `ralph/plans/k8s-20-chart/`

Never touch: the root `go.mod`, a root `go.sum`, `.github/workflows/release.yml` (FR-004),
`AGENTS.md`, `docs/security.md`, anything under `internal/` or `cmd/`, `k8s/`.

## Design

Decided here. Do not reopen these.

### §0 Decisions this plan takes (and why)

- **The Go generator stays the one source of the CRD and the RBAC rules.** It writes the chart's
  CRD (`deploy/chart/crds/`, byte-identical to `deploy/k8s/crd.json`) and a namespace-free rule
  file (`deploy/chart/files/rules.json`) that the RBAC templates read with
  `.Files.Get "files/rules.json" | fromJson`. Helm reads a `.json` file in `crds/` and
  `fromJson` works (checked with helm 3.16.3 on 10/6/26). The drift test is then a plain Go byte
  comparison that needs no helm, and the SC-005 walk S2 wrote over the JSON covers the chart's
  rules too. The rejected option was hand-written YAML RBAC plus a `helm template` drift test,
  which would need YAML parsing (no library allowed in the root module) or helm in `go test`.
- **Images go to ghcr, published by a workflow; the chart is an OCI artifact there too.**
  `GITHUB_TOKEN` with `packages: write` needs no stored secret. Docker Hub would need a token
  secret from the operator.
- **Version scheme.** For commit count N (the same N as host release `v0.N`), the images and the
  chart are `2.0.N`. FR-004 says v2 ships as major 2. The host updater never crosses a major
  (k8s-18), so a 2.x image is never offered to a host.
- **No Service by default.** The daemon listens on `127.0.0.1:8765` in its pod. Reaching it is
  `kubectl port-forward`, or the optional cloudflared sidecar over loopback, which is the VM's
  shape. `service.enabled=true` switches the listener to `0.0.0.0:8765` and adds a ClusterIP
  Service. The daemon refuses a non-loopback listener when no browser door admits anyone
  (`loadListen` in `internal/config/config.go`), so the chart refuses `service.enabled` unless
  `access.teamDomain` or `dashboardPassword.existingSecret` is set (a `fail` in `daemon.yaml`).
- **Both runtimes are configured by default.** `startCommands` defaults to a Claude Code entry and
  a `codex` entry, the same line S7's crswd-next manifest uses (FR-021).
- **The chart creates no secret values.** It references Secrets the operator creates:
  `sharedSecret.existingSecret` (required, key `secret`, made with `openssl rand -hex 32`),
  `dashboardPassword.existingSecret` (key `password`), `claudeCredentials.secretName`,
  `codexAuth.secretName`, and `cloudflared.tokenSecret`. (`crswd keygen` prints a release-signing
  key pair and is not the shared secret.)

### §1 `.claude/settings.json`

Add `"Bash(helm lint:*)"` and `"Bash(helm template:*)"` to `permissions.allow`. Change nothing
else. Run helm only from the repo root: `helm lint deploy/chart`, `helm template t deploy/chart ...`.
Never `cd`.

### §2 Generator output for the chart (`deploy/k8s/manifest/chart.go`)

```go
// ChartFiles returns the chart's generated files, relative to deploy/chart.
func ChartFiles() (map[string][]byte, error)
```

- `crds/agentsessions.crswd.craigcloud.io.json`: the same bytes `Files()` returns for `crd.json`.
- `files/rules.json`: `{"daemon": [...], "lease": [...], "reconciler": [...]}`. Each value is the
  `rules` array of the matching Role from S2's RBAC builders, marshalled the way `Files()`
  marshals (`json.MarshalIndent(v, "", "  ")` plus `\n`). Take the rules from the same Go values
  `rbac.go` uses. Do not re-type a verb.
- `gen/main.go` writes `ChartFiles()` under `deploy/chart/`, creating `crds/` and `files/` with
  `os.MkdirAll(..., 0o750)`.
- `drift_test.go` also compares every `ChartFiles()` entry with `../../chart/<name>`.
- `chart_test.go`: run the S2 SC-005 rule walker over the decoded `files/rules.json`. Secrets or
  `*` fails, and pods `create` in `daemon` fails.

### §3 Chart skeleton (`deploy/chart`)

- `Chart.yaml`: `apiVersion: v2`, `name: crswd`, `description: Claude Code and Codex sessions, one pod each, behind an authenticated dashboard and API.`,
  `type: application`, `version: 2.0.0`, `appVersion: "2.0.0"`. CI overrides both at package
  time (§6).
- `values.yaml`, exactly these keys and defaults, each with a one-line comment:
  ```yaml
  image:
    repository: ghcr.io/craig-ai-tooling/crswd
    tag: ""            # empty means .Chart.AppVersion
    pullPolicy: IfNotPresent
  sessionImage:
    repository: ghcr.io/craig-ai-tooling/crswd-session
    tag: ""            # empty means .Chart.AppVersion
  reconciler:
    namespace: ""      # empty means <release namespace>-reconciler
    createNamespace: true
  sharedSecret:
    existingSecret: "" # required: a Secret with key "secret" (openssl rand -hex 32)
  dashboardPassword:
    existingSecret: "" # optional: a Secret with key "password"; the browser door without Access
  startCommands: "default=claude --dangerously-skip-permissions,codex=/usr/local/bin/codex --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust"
  sessionNode: ""      # the node that holds the claim (RWO) and is amd64; FR-008 sets lm-amd64-1
  claudeCredentials:
    secretName: claude-credentials
  codexAuth:
    secretName: codex-auth
  allowedRoots: /work
  maxSessions: 10
  storage:
    storageClassName: linstor-replicated
    size: 50Gi
  nodeSelector: {}
  tolerations:
    - key: drbd.linbit.com/lost-quorum
      operator: Exists
      effect: NoSchedule
  service:
    enabled: false
    port: 8765
  access:
    teamDomain: ""
    aud: ""
    allowedEmails: ""
  cloudflared:
    enabled: false
    image: docker.io/cloudflare/cloudflared:2026.10.0
    tokenSecret: ""    # a Secret with key "token"
  ```
- `templates/_helpers.tpl`: `crswd.reconcilerNamespace` (the value, else
  `printf "%s-reconciler" .Release.Namespace`), `crswd.image` and `crswd.sessionImage`
  (`repository:tag`, tag defaulting to `.Chart.AppVersion`), and two label helpers:
  `crswd.selectorLabels` taking a component (`app.kubernetes.io/name: crswd`,
  `app.kubernetes.io/instance: {{ .Release.Name }}`, `app.kubernetes.io/component: <component>`),
  and `crswd.labels` (the selector labels plus `app.kubernetes.io/part-of: crswd` and
  `helm.sh/chart`). Deployments use `crswd.selectorLabels` for both `selector.matchLabels` and
  the pod template; the Service selects `crswd.selectorLabels` with component `daemon`.
- `templates/NOTES.txt`: the `kubectl -n <ns> port-forward deploy/crswd 8765` line and a pointer
  to `docs/k8s-mode.md`.
- The guards live at the top of `templates/daemon.yaml`, which always renders:
  `{{- $_ := required "sharedSecret.existingSecret is required (a Secret with key secret: openssl rand -hex 32)" .Values.sharedSecret.existingSecret }}`
  and `{{- if and .Values.service.enabled (not .Values.access.teamDomain) (not .Values.dashboardPassword.existingSecret) }}{{ fail "service.enabled needs a browser door: set access.* or dashboardPassword.existingSecret" }}{{- end }}`.
  T3 writes a placeholder `daemon.yaml` holding only these guards; T5 fills in the Deployment.
- `.helmignore`: `*_test.go`, `doc.go`.
- `deploy/chart/doc.go`: `// Package chart holds render-free tests of the Helm chart's files.` and
  `package chart`.

### §4 Namespaces, ServiceAccounts, RBAC (`templates/rbac.yaml`, `templates/namespace.yaml`)

- `namespace.yaml`: when `reconciler.createNamespace`, a Namespace named
  `crswd.reconcilerNamespace`.
- ServiceAccounts: `crswd` in `.Release.Namespace`, `crswd-reconciler` in the reconciler namespace.
- Role `crswd` in `.Release.Namespace`, rules from `rules.json` `daemon`, and a RoleBinding to SA
  `crswd`.
- Role `crswd-reconciler` in `.Release.Namespace`, rules `reconciler`, and a RoleBinding to SA
  `crswd-reconciler` in the reconciler namespace.
- Role `crswd-reconciler-lease` in the reconciler namespace, rules `lease`, and a RoleBinding to
  the same SA.
- Each `rules:` line is `{{ (.Files.Get "files/rules.json" | fromJson).<key> | toJson }}`.
- No ClusterRole and no ClusterRoleBinding. The CRD is the one cluster-scoped object, in `crds/`.
- `chart_test.go` `TestChartRBACReadsGeneratedRules`: read `templates/rbac.yaml` as text. Assert
  each of the three keys appears in a `.Files.Get "files/rules.json"` expression, and that the
  text contains no literal `verbs:` (so no rule is hand-written). Assert no file under
  `templates/` contains `ClusterRole` or `secrets`.

### §5 Workloads (`templates/claim.yaml`, `templates/daemon.yaml`, `templates/reconciler.yaml`, `templates/service.yaml`)

- `claim.yaml`: PVC `crswd-sessions` in `.Release.Namespace`, `ReadWriteOnce`, class and size from
  values, annotation `helm.sh/resource-policy: keep` (an uninstall keeps every conversation; FR-010
  Retain).
- `daemon.yaml`: Deployment `crswd`, 1 replica, strategy `Recreate`, `serviceAccountName: crswd`,
  image `crswd.image`, no args, an `emptyDir` mounted at `/work` (`config.Load` requires the
  allowed root to exist on the daemon's filesystem; sessions' real `/work` is in their pods). Security context as S7 §5: `runAsNonRoot`, `readOnlyRootFilesystem`,
  `allowPrivilegeEscalation: false`, drop `ALL`. Env:
  `CRSW_EXECUTION_MODE=kubernetes`; `CRSW_SESSION_NAMESPACE={{ .Release.Namespace }}`;
  `CRSW_LISTEN` = `127.0.0.1:8765`, or `0.0.0.0:<service.port>` when `service.enabled`;
  `CRSW_ALLOWED_ROOTS` from `allowedRoots`; `CRSW_MAX_SESSIONS` from `maxSessions`;
  `CRSW_START_COMMANDS` from `startCommands`; `CRSW_SHARED_SECRET` from `secretKeyRef`
  `sharedSecret.existingSecret`/`secret`; `CRSW_DASHBOARD_PASSWORD` from `secretKeyRef`
  `dashboardPassword.existingSecret`/`password` only when set; `CRSW_ACCESS_TEAM_DOMAIN`,
  `CRSW_ACCESS_AUD` and `CRSW_ACCESS_ALLOWED_EMAILS` only when `access.teamDomain` is set. Every other
  env name the daemon needs in kubernetes mode is copied from `deploy/k8s/manifest/crswdnext.go`
  (S7), which is the tested source. Node selector and tolerations from values.
  When `cloudflared.enabled`, a second container `cloudflared`, image from values, args
  `["tunnel","--no-autoupdate","run"]`, env `TUNNEL_TOKEN` from `secretKeyRef`
  `cloudflared.tokenSecret`/`token`, the same security context.
- `reconciler.yaml`: Deployment `crswd-reconciler` in the reconciler namespace, 1 replica,
  `Recreate`, `serviceAccountName: crswd-reconciler`, image `crswd.image`, args `["reconcile"]`, the same security
  context, no ports. Env: `CRSW_SESSION_NAMESPACE={{ .Release.Namespace }}`,
  `CRSW_RECONCILER_NAMESPACE={{ include "crswd.reconcilerNamespace" . }}`,
  `CRSW_SESSION_IMAGE={{ include "crswd.sessionImage" . }}`, `CRSW_SESSION_NODE` from
  `sessionNode`, `CRSW_SESSION_CLAIM=crswd-sessions`, `CRSW_MAX_SESSIONS` from `maxSessions`,
  `CRSW_CLAUDE_SECRET` from `claudeCredentials.secretName`, `CRSW_CODEX_SECRET` from
  `codexAuth.secretName`. These are S6's `ConfigFromEnv` names; confirm each with
  `grep -n 'CRSW_' k8s/internal/reconcile/config.go`.
- `service.yaml`: only when `service.enabled`, a ClusterIP Service on `service.port` selecting
  the daemon.
- `chart_test.go` adds `TestChartWorkloadsAreLockedDown`: read `daemon.yaml` and `reconciler.yaml`
  as text and assert each contains `readOnlyRootFilesystem: true`, `runAsNonRoot: true`,
  `allowPrivilegeEscalation: false`, `serviceAccountName:`, `crswd.selectorLabels`, and no
  `hostPath`, `privileged: true` or `hostNetwork`. `TestChartOffersBothRuntimes`: `values.yaml`'s
  `startCommands` line contains `default=claude` and `codex=`.

### §6 CI and publishing

`.github/workflows/ci.yml`, inside the job `project` ("Build / test / lint"), after
`Build (k8s)`:

```yaml
      - name: Set up helm
        if: hashFiles('deploy/chart/Chart.yaml') != ''
        uses: azure/setup-helm@9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310 # v5.0.1
        with:
          version: v3.16.3

      - name: Chart (lint and render)
        if: hashFiles('deploy/chart/Chart.yaml') != ''
        run: |
          helm lint deploy/chart --set sharedSecret.existingSecret=s
          helm template t deploy/chart -n crswd --include-crds --set sharedSecret.existingSecret=s > /dev/null
          helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s --set service.enabled=true --set dashboardPassword.existingSecret=p --set cloudflared.enabled=true --set cloudflared.tokenSecret=t > /dev/null
          if helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s --set service.enabled=true > /dev/null 2>&1; then echo "service without a browser door rendered"; exit 1; fi
```

New `.github/workflows/images.yml`:
- `on: push: branches: [main]` with `paths: [k8s/**, internal/**, api/**, deploy/image/**, deploy/session-image/**, deploy/chart/**, deploy/k8s/**]`, and `workflow_dispatch`.
- `permissions: contents: read, packages: write`. `concurrency: { group: images, cancel-in-progress: false }`.
- One job on `ubuntu-latest`. Steps:
  1. `actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1` with `fetch-depth: 0`.
  2. Version: `echo "version=2.0.$(git rev-list --count HEAD)" >> "$GITHUB_OUTPUT"`.
  3. `actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0` with `go-version: '1.24'`.
  4. Build `crswd-amd64` and `crswd-arm64` into `$RUNNER_TEMP/bin` with the S7 §4 command and
     `-X main.version=<version>`.
  5. `mkdir -p "$RUNNER_TEMP/session"`, `deploy/session-image/fetch-codex.sh "$RUNNER_TEMP/session"`, then copy `crswd-amd64` there as `crswd`.
  6. `docker/setup-buildx-action@f87e5991a6d7451dcb8d9637bfbc97413f497069 # v4.4.1`.
  7. `docker/login-action@dbcb813823bdd20940b903addbd779551569679f # v4.6.0` to `ghcr.io` with
     `${{ github.actor }}` and `${{ secrets.GITHUB_TOKEN }}`.
  8. `docker/build-push-action@c3c9e263c25d99ce0380d002d59b67737d91b0dc # v7.4.0`: context
     `$RUNNER_TEMP/bin`, file `deploy/image/Dockerfile`, platforms `linux/amd64,linux/arm64`,
     tag `ghcr.io/craig-ai-tooling/crswd:<version>`, push true, label
     `org.opencontainers.image.source=https://github.com/craig-ai-tooling/claude-remote-session-webhook`.
  9. The same action for the session image: context `$RUNNER_TEMP/session`, file
     `deploy/session-image/Dockerfile`, platform `linux/amd64`, tag
     `ghcr.io/craig-ai-tooling/crswd-session:<version>`, the same label.
  10. `azure/setup-helm` as in ci.yml, then
      `helm package deploy/chart --version <version> --app-version <version> -d "$RUNNER_TEMP"`,
      `echo "${{ secrets.GITHUB_TOKEN }}" | helm registry login ghcr.io -u "${{ github.actor }}" --password-stdin`,
      `helm push "$RUNNER_TEMP/crswd-<version>.tgz" oci://ghcr.io/craig-ai-tooling/charts`.
- A comment at the top says why there is no tag trigger: the version is the commit count,
  as `release.yml` does it, and `release.yml` is not touched (FR-004).

The loop edits these files. It cannot push them: a `.github/workflows/**` change needs the
craig-ai-tooling App token, which the operator uses at push and merge time (PROGRESS.md
iteration 0).

### §7 Docs

- `README.md`: a new `## Install on Kubernetes` section placed directly before `## The two doors`.
  Contents: one paragraph saying crswd has two supported installs, both running Claude Code and
  Codex; the prerequisites (a cluster, a `ReadWriteOnce` class, `kubectl`, `helm` 3); the commands:
  ```
  kubectl create namespace crswd
  kubectl -n crswd create secret generic crswd-shared-secret --from-literal=secret="$(openssl rand -hex 32)"
  helm install crswd oci://ghcr.io/craig-ai-tooling/charts/crswd -n crswd --set sharedSecret.existingSecret=crswd-shared-secret
  kubectl -n crswd port-forward deploy/crswd 8765
  ```
  then upgrade (`helm upgrade`; the dashboard's Update button is off in this mode, FR-003),
  uninstall (the claim is kept), the `sessionNode` value (the amd64 node that holds the claim),
  and a link to `docs/k8s-mode.md`. Say that both runtimes are offered by default and that Codex
  in a pod signs in with the namespace's own `codex-auth` Secret, never a copy of a host's
  `~/.codex/auth.json` (spec 017 FR-022).
- At the top of the existing `## Install` section, add one sentence: this section is the host
  install (binary and systemd); the Kubernetes install is `## Install on Kubernetes`.
- `docs/k8s-mode.md`: a `## Install with Helm` section after the hand-install section: the values
  table (§3 keys, one line each), the Secrets the operator creates, and the upgrade and
  uninstall commands.
- The README tests in `internal/release/readme_test.go` and `internal/config/docs_test.go` must
  stay green. If one fails on the new section, change the section to satisfy it, never the test.

## Operator-run acceptance (not loop tasks)

1. Push the branch and merge the PR with the craig-ai-tooling App token, because the diff touches
   `.github/workflows/**`.
2. After `images.yml` runs on main, make the three ghcr packages public once:
   `crswd`, `crswd-session`, and `charts/crswd`. The cluster has no pull secret.
3. In the rpi-inference cluster (`KUBECONFIG=~/.kube/rpi-inference.yaml`), into a throwaway
   namespace: create the shared secret, `helm install` from the OCI chart, and check that both
   Deployments are Available, the Lease is held, and a session created through
   `kubectl port-forward` plus `crswd-api` gets a pod. Then `helm uninstall`, delete both
   namespaces, and release the kept PV (docs/k8s-mode.md teardown). Record the result in spec 017
   research.
