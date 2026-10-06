# Implementation plan: k8s-20c-s7

S7 of k8s-20c, the last slice: wiring, images, the crswd-next manifests and the operator doc
(spec 017 FR-002, FR-003, FR-009, FR-021; source `specs/017-k8s-native-execution/k8s-20c-plan.md`
§2 "S7", §3, §4). The definition of done is `ralph/plans/k8s-20c-s7/VALIDATION_CONTRACT.md`.

After this slice the cluster build runs: one image holds the daemon and the reconciler, and the
session image holds both runtimes, Claude Code and Codex (FR-021). The host binary still refuses
`kubernetes` mode, because it never calls the switch this slice adds.

## Dependency

S2, S4, S5 and S6 are merged. Check, from the repo root:

`test -f api/v1alpha1/types.go && test -f k8s/go.mod && test -d k8s/internal/podctl && test -d k8s/internal/reconcile && test -f deploy/k8s/crd.json`

If it fails, the reason is "S2/S4/S5/S6 not all on this branch's base".

## Tasks

Take the topmost open task. One per iteration.

- [x] T1: The cluster switch in `internal/config/config.go` per Design §1. Verify: `go test ./internal/config/ ./cmd/crswd/` exits 0, and `go test -run HostNeverBuildsKubernetes ./cmd/crswd -v` passes.
- [x] T2: `httpapi.NewForCluster` per Design §2, in new `internal/httpapi/cluster.go` and `cluster_test.go`. Verify: `go test -run NewForCluster ./internal/httpapi -v` passes.
- [x] T3: The cluster binary `k8s/cmd/crswd` per Design §3. Verify: `go -C k8s test ./cmd/crswd/... -v` passes and `go -C k8s build -o /dev/null ./cmd/crswd` exits 0.
- [x] T4: The crswd image Dockerfile and the session image's Codex per Design §4. Verify: `go test -run Dockerfile ./internal/sessionpod ./deploy/image -v` passes, and `bash -n deploy/session-image/fetch-codex.sh` exits 0.
- [x] T5: The crswd-next manifests, generated, per Design §5. Run `go run ./deploy/k8s/gen` to write them. Verify: `go test ./deploy/k8s/...` exits 0 (the drift test compares the generator's bytes with the files on disk).
- [x] T6: `docs/k8s-mode.md` per Design §6. Verify: `grep -c '^## ' docs/k8s-mode.md` prints at least `7`, and `go test ./...` exits 0.
- [ ] T7: Run every command in VALIDATION_CONTRACT.md, record each result in PROGRESS.md, then append `RALPH_COMPLETE`. Verify: `go test ./... && go -C k8s test ./...` exits 0.

## Files touched

A diff outside this list is rejected.

- `internal/config/config.go` (T1 only: the switch and its comment)
- `cmd/crswd/hostswitch_test.go` (new, T1)
- `cmd/crswd/main_test.go` (T3: the nested-module skip in `parseTheDaemon` only; see PROGRESS.md "Operator decision")
- `k8s/cmd/crswd/stdout_test.go` (new, T3)
- `internal/httpapi/cluster.go`, `internal/httpapi/cluster_test.go` (new, T2; T3 may add `Server.PodRecord` to `cluster.go`)
- `k8s/cmd/crswd/` (new: `main.go`, `daemon.go`, `reconcile.go`, `main_test.go`, `daemon_test.go`)
- `k8s/go.mod`, `k8s/go.sum` (only if `go -C k8s mod tidy` changes them)
- `deploy/image/Dockerfile`, `deploy/image/dockerfile_test.go`, `deploy/image/doc.go` (new, T4)
- `deploy/session-image/Dockerfile`, `deploy/session-image/fetch-codex.sh` (T4)
- `internal/sessionpod/dockerfile_test.go` (T4: the image now copies two binaries; this is the
  one existing test this slice edits, because FR-021 changes what it pins)
- `deploy/k8s/manifest/crswdnext.go`, `deploy/k8s/manifest/crswdnext_test.go` (new, T5)
- `deploy/k8s/manifest/manifest.go` (T5: `Files()` gains the crswd-next entries)
- `deploy/k8s/manifest/drift_test.go` (T5: glob the `crswd-next/` subdirectory too)
- `deploy/k8s/gen/main.go` (T5: create `deploy/k8s/crswd-next/` before writing)
- `deploy/k8s/crswd-next/*.json` (generated, T5)
- `docs/k8s-mode.md` (new, T6)
- `ralph/plans/k8s-20c-s7/`

Never touch: the root `go.mod`, a root `go.sum`, `AGENTS.md`, `docs/security.md`,
`.github/`, `.claude/`, `cmd/crswd/` except the one new test and the T3 `parseTheDaemon` skip, `internal/session/`,
`internal/sessionpod/sessionpod.go`, `k8s/internal/podctl/`, `k8s/internal/reconcile/`.

## Design

Decided here. Do not reopen these. If one cannot be built as written, follow PROMPT.md "When a
task is ambiguous".

### §0 Decisions this plan takes (and why)

- **Two binaries, not one.** The cluster binary (`k8s/cmd/crswd`) runs `kubernetes` mode only.
  Given a host configuration it refuses with a sentence pointing at the release binary. The
  source plan said "host mode delegates to the root code path", but `cmd/crswd` is package
  `main` and cannot be imported. Making it importable would move v0's `run()`, which FR-004
  forbids. The host install keeps `install.sh` and the release binary, unchanged.
- **The switch is a setter, not a constant.** `kubernetesModeBuilt` becomes a package variable
  that only `k8s/cmd/crswd` sets, through `config.BuildKubernetesMode()`. The start check and the
  settings page both still read it through `Runnable`, so they still agree in either binary.
- **`codex-conversation` and `has-transcript` are in this slice.** S5 built their in-pod halves
  (`sessionpod.CodexConversation`, `sessionpod.HasTranscript`) and the daemon halves
  (`podctl.Controller.CodexConversation`, `HasTranscript`), and folded in spec 019 T041 to T043.
  This slice dispatches the two subcommands and wires the two Manager hooks. The
  `ralph/plans/codex-k8s` notebook is retired; do not open it.
- **No new journal, relay or update feed** in the cluster daemon (FR-003). `NewForCluster`
  builds on `NewWith`, which wires none of them.

### §1 The cluster switch (`internal/config/config.go`)

- Replace `const kubernetesModeBuilt = false` with `var kubernetesModeBuilt bool`.
- Add, beside it:
  ```go
  // BuildKubernetesMode marks this process as the cluster build ... (why: only
  // k8s/cmd/crswd calls it, first thing in main; the host binary never does, so
  // Runnable keeps refusing kubernetes there).
  func BuildKubernetesMode() { kubernetesModeBuilt = true }
  ```
- Rewrite the comment above the variable: it is false in the host binary forever, and the
  cluster binary flips it at start. Keep the reason the start and the settings page ask the
  same question.
- Do not edit `internal/config/executionmode_test.go` or `cmd/crswd/executionmode_test.go`. They
  assert the default refusal and stay green, because nothing in the root calls the setter.
- New `cmd/crswd/hostswitch_test.go`, `TestHostNeverBuildsKubernetesMode`: walk the repo root
  (`../..`), skipping `k8s/`, `.git/`, `.claude/` and `testdata`. Read every `*.go` file that is
  not `*_test.go`. Fail if any contains `BuildKubernetesMode(` other than the definition line in
  `internal/config/config.go`, which is `func BuildKubernetesMode()`. Also fail unless that
  definition occurs exactly once, so the test fails against the old code, where it is absent.
  Assert at least 50 files were read. Build the search string from two parts, so this test file does not match itself.
- The setter's positive case is tested in T3, in the cluster binary's own test process, because
  package `config` tests run in parallel and a global flip would race them.

### §2 `httpapi.NewForCluster` (`internal/httpapi/cluster.go`)

```go
// NewForCluster is the cluster build's constructor (spec 017). ...
func NewForCluster(cfg *config.Config, ctl tmuxctl.Controller, hooks ClusterHooks) (*Server, error)

// ClusterHooks are the session.Manager hooks S5 added for sessions that live in pods.
type ClusterHooks struct {
	CodexConversation func(ctx context.Context, s session.Session) (string, error)
	// HasTranscript gets the session's runtime from NewForCluster, which reads it with
	// srv.sessions.SpecOf(s).Name: a Session carries no runtime field of its own.
	HasTranscript     func(ctx context.Context, s session.Session, h harness.Name, id string) (bool, error)
}
```

1. `cfg == nil`: return an error.
2. `!cfg.ExecutionMode.Kubernetes()`: return `errors.New("httpapi: NewForCluster is for kubernetes mode; a host daemon is built by New")`.
3. `ctl == nil`: return an error.
4. `srv, err := NewWith(cfg, ctl, audit.New())`.
5. `srv.sessions.SetWorkDirResolver(session.LexicalWorkDir)`. S2 built both. The daemon cannot
   see a pod's filesystem; the pod re-checks the path with symlinks resolved (S3).
6. When `hooks.CodexConversation != nil`, `srv.sessions.SetCodexConversationFinder(hooks.CodexConversation)`.
   When `hooks.HasTranscript != nil`, `srv.sessions.SetTranscriptChecker(func(ctx context.Context, s session.Session, id string) (bool, error) { return hooks.HasTranscript(ctx, s, srv.sessions.SpecOf(s).Name, id) })`.
   Both setters are S5's (`grep -n 'func (m \*Manager) Set' internal/session/manager.go`).
7. Return `srv`.

Tests, in package `httpapi` (`cluster_test.go`), building `config.Config` by hand the way
`server_test.go` does (grep `func testConfig` or the nearest helper there and reuse it):
- `TestNewForClusterRefusesHostMode`: a host config returns an error that mentions `New`.
- `TestNewForClusterRefusesNilController`.
- `TestNewForClusterChecksWorkDirLexically`: kubernetes config, roots `[{Path: t.TempDir()}]`,
  `tmuxctl.NewFake()`. A `srv.sessions.Create` (find the request type with
  `grep -n 'func (m \*Manager) Create' internal/session/manager.go`) for
  `filepath.Join(root, "absent")` succeeds. The same create through `NewWith` alone fails with
  `session.ErrInvalidWorkDir`.
- `TestNewForClusterHasNoRelayOrFeed`: `len(srv.signins) == 0` and `srv.releaseFeed == nil`.
- `TestNewForClusterWiresHooks`: a finder returning a fixed UUID is the one the Manager calls.
  Reuse S5's `TestCodexConversationFinderIsUsed` pattern (`grep -n CodexConversationFinderIsUsed internal/session/*_test.go`).

### §3 The cluster binary (`k8s/cmd/crswd`)

`main.go`, package `main`. `main()` calls `config.BuildKubernetesMode()` first, then dispatches
on `os.Args[1:]`, and exits with the code `dispatch` returns:

```go
func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int
```

| args[0] | What it does | Exit |
|---|---|---|
| `--version` | prints `crswd <version> (cluster build)`; `version` is a package var, default `dev`, set by `-ldflags -X main.version=...` | 0 |
| `session-pod` | `sessionpod.Run(name, workdir, roots)` with the args S6's pod template passes | 0, or 1 with the error on stderr |
| `pane-loop` | `sessionpod.PaneLoop(name, stdout)` | 0, or 1 |
| `reconcile` | the reconciler's exported entry point from S6 | 0, or 1 |
| none | `runDaemon(ctx, stderr)` (`daemon.go`) | 0, or 1 |
| anything else | `crswd: unknown command "<x>"` on stderr | 2 |

Subcommands, with the exact calls (S5 and S6 fixed these interfaces; confirm each with
`grep -n '^func ' <package dir>/*.go` before writing, and if one differs, follow PROMPT.md "When a
task is ambiguous", quoting the grep output):

- `session-pod <name> <workdir>`: the args S6's pod template writes
  (`Args: []string{"session-pod", n, obj.Spec.WorkDir}`). Calls
  `sessionpod.Run(name, workdir, []config.ApprovedRoot{{Path: sessionpod.WorkRoot}})`.
  Wrong arg count: exit 2.
- `pane-loop <name>`: `sessionpod.PaneLoop(name, stdout)`.
- `codex-conversation <name>`: `sessionpod.CodexConversation(ctx, <exec>, name, "/proc", session.CodexHome(os.Environ()))`,
  where `<exec>` comes from `sessionpod.NewInPodExec()` (S5 T3); exit 1 if it returns an error. Prints the id or nothing, exit 0; error: exit 1.
- `has-transcript <name> <harness> <id> <workdir>`:
  `sessionpod.HasTranscript(harness.Name(h), id, workdir, os.Environ())`. Exit 0 when true, 1 when false.
- `reconcile`: `runReconciler(ctx)` in `reconcile.go`:
  1. `cfg, err := reconcile.ConfigFromEnv(os.Getenv)`.
  2. `rc, err := kube.InCluster()`.
  3. `kc, err := kubernetes.NewForConfig(rc)`; `dyn, err := dynamic.NewForConfig(rc)`.
  4. `r, err := reconcile.New(kc, dyn, cfg)`.
  5. `identity, err := os.Hostname()` (the pod name; unique per pod).
  6. `return reconcile.RunWithLease(ctx, kc, identity, r)`.

`daemon.go`:

```go
func runDaemon(ctx context.Context, stderr io.Writer) error
```

1. `cfg, err := config.Load()`.
2. If `!cfg.ExecutionMode.Kubernetes()`, return
   `errors.New("this is the cluster build of crswd and runs only with CRSW_EXECUTION_MODE=kubernetes; on a host, install the release binary (install.sh)")`.
3. `cfg.ExecutionMode.Runnable()` must be nil (proves the switch is on).
4. `cfg.CheckDependencies(stderr)` (it skips tmux in kubernetes mode, S1).
5. `ns := os.Getenv("CRSW_SESSION_NAMESPACE")`; empty is an error naming the variable. This is
   the same variable S6's `reconcile.ConfigFromEnv` reads, so one name means one namespace in
   both processes.
6. `rc, err := kube.InCluster()`; `kc, err := kubernetes.NewForConfig(rc)`;
   `dyn, err := dynamic.NewForConfig(rc)`.
7. `sessions, err := agentsession.New(dyn, ns)`.
8. `exec := &podctl.RemoteExecutor{Config: rc, Client: kc, Namespace: ns}`.
9. `ctl, err := podctl.New(sessions, kc, exec, podctl.Config{Namespace: ns, PaneBound: cfg.PaneBound})`.
10. `srv, err := httpapi.NewForCluster(cfg, ctl, httpapi.ClusterHooks{CodexConversation: func(ctx context.Context, s session.Session) (string, error) { return ctl.CodexConversation(ctx, s.TmuxName()) }, HasTranscript: func(ctx context.Context, s session.Session, h harness.Name, id string) (bool, error) { return ctl.HasTranscript(ctx, s.TmuxName(), h, id, s.WorkDir) }})`.
    Get the harness of `s` the way the Manager does (`grep -n 'func (m \*Manager) harnessOf\|harness.Of(' internal/session/manager.go`).
    If the Manager exposes it only unexported, pass `harness.Of(<start command line>)` using the
    record S5's `PodRecord` returns.
11. `ctl.SetDescriber(func(name string) (session.PodRecord, bool) { return srv.PodRecord(name) })`
    if `httpapi.Server` exposes one; otherwise add `func (s *Server) PodRecord(name string) (session.PodRecord, bool) { return s.sessions.PodRecord(name) }`
    to `internal/httpapi/cluster.go` in this task.
12. The run sequence, the same order as `cmd/crswd/main.go` `run()` from `srv.Reconcile` on:
   `Reconcile`, `StartReaper`, `StartSupervisor`, `Listen`, `Serve` in a goroutine, wait for
   ctx or Serve, then `Shutdown` with a 30 s budget on `context.WithoutCancel(ctx)`. Copy the
   ordering. Do not copy the long comments; one line pointing at `cmd/crswd/main.go run()` is
   enough. `signal.NotifyContext` on SIGINT and SIGTERM, as there.

Tests (`main_test.go`, `daemon_test.go`):
- `TestDispatchUnknownCommand`: exit 2, stderr names the command.
- `TestDispatchVersion`: exit 0, stdout contains `cluster build`.
- `TestClusterBuildIsRunnable`: after `config.BuildKubernetesMode()`,
  `config.ExecutionModeKubernetes.Runnable()` is nil. Before the call (first line of the test,
  before anything sets it) it is `config.ErrKubernetesModeUnbuilt`. Do not use `t.Parallel()` in
  this file.
- `TestDaemonRefusesHostConfig`: `t.Setenv` the minimum a config needs to load (copy the env a
  passing `config.Load` test uses: `grep -n 'Setenv' internal/config/*_test.go | head`), with
  `CRSW_EXECUTION_MODE` unset. `runDaemon` returns the error from step 2.
- `TestSessionPodArgs`: `[]string{"crswd-abc", "/work/x"}` parses to that name and workdir and
  roots `[{Path: sessionpod.WorkRoot}]`; one arg or three is an error. Put the parse in
  `func parseSessionPodArgs([]string) (name, workdir string, roots []config.ApprovedRoot, err error)`.
- `TestReconcileRefusesBadConfig`: with `CRSW_SESSION_NAMESPACE` unset, `runReconciler` returns
  `ConfigFromEnv`'s error before touching the cluster.
- `TestDaemonNeedsSessionNamespace`: kubernetes config loaded, `CRSW_SESSION_NAMESPACE` unset:
  `runDaemon` returns an error naming the variable, before `kube.InCluster`.
- `TestPodContainerNameAgrees`: `podctl.Container == reconcile.SessionContainer` (S6 asks S7 to hold them equal).

### §4 Images

**crswd image** (`deploy/image/Dockerfile`), the daemon and the reconciler, multi-arch:

```dockerfile
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETARCH
COPY --chmod=0755 crswd-${TARGETARCH} /usr/local/bin/crswd
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/crswd"]
```

Comment it like `deploy/session-image/Dockerfile`: the tag scheme
`docker.io/nctiggy/crswd:2.0.0-dev.<sha7>` (k8s-20c-plan §3), and the build:

```
BIN=$(mktemp -d)
for a in amd64 arm64; do CGO_ENABLED=0 GOOS=linux GOARCH=$a go -C k8s build -trimpath -ldflags "-X main.version=2.0.0-dev.$(git rev-parse --short=7 HEAD)" -o "$BIN/crswd-$a" ./cmd/crswd; done
docker buildx build --platform linux/amd64,linux/arm64 -f deploy/image/Dockerfile -t docker.io/nctiggy/crswd:2.0.0-dev.$(git rev-parse --short=7 HEAD) --push "$BIN"
```

No RUN, no ADD, no ENV, no secret. `deploy/image/doc.go` is `// Package image holds the crswd image's Dockerfile test.` plus `package image`, so the test has a package.
`deploy/image/dockerfile_test.go` copies the parsing helper from
`internal/sessionpod/dockerfile_test.go` (`instructions`) and asserts: exactly that FROM, one
COPY equal to the line above, no RUN/ADD/ENV, `USER 65532:65532`, the ENTRYPOINT, the tag
scheme documented.

**Session image** (`deploy/session-image/Dockerfile`): Codex joins Claude Code (FR-021).

- New `deploy/session-image/fetch-codex.sh <dir>`: `set -euo pipefail`; constants
  `CODEX_VERSION=0.153.4`,
  `ASSET=codex-x86_64-unknown-linux-musl.tar.gz`,
  `SHA256=f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30`
  (measured 10/6/26 from release `rust-v0.153.4`; the archive holds one file,
  `codex-x86_64-unknown-linux-musl`, which printed `codex-cli 0.153.4` inside the base image).
  Download `https://github.com/openai/codex/releases/download/rust-v${CODEX_VERSION}/${ASSET}`
  with `curl -fsSL` into a temp file, check it with `echo "$SHA256  $tmp" | sha256sum -c -`,
  `mkdir -p <dir>`, extract that one member, and `install -m 0755` it as `<dir>/codex`. Exit non-zero on any
  mismatch and print nothing secret.
- Dockerfile: add `COPY --chmod=0755 codex /usr/local/bin/codex` after the crswd COPY. Update
  the header comment: the tag becomes
  `docker.io/nctiggy/crswd-session:2.1.246-codex0.153.4-<sha7>`, and the BUILD block runs
  `deploy/session-image/fetch-codex.sh "$BIN"` before `docker build`. Note that the CA bundle the
  Codex binary needs (spec 019 research M21) is already in the base image (`ca-certificates`
  20250419, checked 10/6/26), so no RUN is added.
- `internal/sessionpod/dockerfile_test.go`: the COPY assertion becomes exactly the two lines,
  crswd then codex, and the tag-scheme string becomes the new one. Leave every other assertion
  as it is.

### §5 crswd-next manifests (generated, `deploy/k8s/crswd-next/`)

Generated by the S2 generator, from Go (source plan E4). Add `crswdnext.go` with
`func crswdNextFiles() (map[string][]byte, error)`, and merge its entries into `Files()` under
the names `crswd-next/<file>.json`. `gen/main.go` creates the `crswd-next` directory
(`os.MkdirAll`, `0o750`) before writing. The drift test also globs `../crswd-next/*.json`.

Constants in `crswdnext.go`:
`DaemonImage = "docker.io/nctiggy/crswd:2.0.0-dev.REPLACE"` and
`SessionImage = "docker.io/nctiggy/crswd-session:2.1.246-codex0.153.4-REPLACE"`. The operator
replaces `REPLACE` with the built sha7 before applying, and regenerates. A test asserts both
contain `REPLACE` in the committed tree, so a real tag is never committed by accident.

Files (each a `v1` `List` where it holds more than one object):

1. `namespaces.json`: `crswd-next` and `crswd-next-reconciler`.
2. `serviceaccounts.json`: SA `crswd` in `crswd-next`, SA `crswd-reconciler` in
   `crswd-next-reconciler`. Both `automountServiceAccountToken: true` (they call the API).
3. `claim.json`: PVC `crswd-sessions` in `crswd-next`, `storageClassName: linstor-replicated`,
   `ReadWriteOnce`, `50Gi`.
Labels: every Deployment has `selector.matchLabels` and pod-template labels exactly
`app.kubernetes.io/name: crswd`, `app.kubernetes.io/instance: crswd-next` and
`app.kubernetes.io/component: daemon` or `reconciler`. Every object also carries
`app.kubernetes.io/part-of: crswd`.

4. `reconciler.json`: Deployment `crswd-reconciler` in `crswd-next-reconciler`, 1 replica,
   `serviceAccountName: crswd-reconciler`,
   strategy `Recreate`, image `DaemonImage`, args `["reconcile"]`, no ports, toleration
   `drbd.linbit.com/lost-quorum` `Exists` `NoSchedule`, `runAsNonRoot: true`,
   `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, capabilities drop `ALL`.
   Env, from S6's `ConfigFromEnv` table: `CRSW_SESSION_NAMESPACE=crswd-next`,
   `CRSW_RECONCILER_NAMESPACE=crswd-next-reconciler`, `CRSW_SESSION_IMAGE=SessionImage`,
   `CRSW_SESSION_NODE=lm-amd64-1` (FR-008; the session image is amd64 only and the claim is RWO
   on one node), `CRSW_SESSION_CLAIM=crswd-sessions`, `CRSW_MAX_SESSIONS=10`,
   `CRSW_CLAUDE_SECRET=claude-credentials`, `CRSW_CODEX_SECRET=codex-auth`. Confirm each name with
   `grep -n 'CRSW_' k8s/internal/reconcile/config.go`.
5. `daemon.json`: Deployment `crswd` in `crswd-next`, 1 replica, `serviceAccountName: crswd`,
   `Recreate`, image `DaemonImage`, no args, the same security context and toleration. An
   `emptyDir` volume mounted at `/work`: `config.Load` requires every allowed root to exist and
   be a directory on the daemon's own filesystem, and the sessions' real `/work` is in their
   pods. Env:
   - `CRSW_EXECUTION_MODE=kubernetes`
   - `CRSW_SESSION_NAMESPACE=crswd-next`
   - `CRSW_LISTEN=127.0.0.1:8765`
   - `CRSW_ALLOWED_ROOTS=/work`
   - `CRSW_MAX_SESSIONS=10`
   - `CRSW_START_COMMANDS=default=claude --dangerously-skip-permissions,codex=/usr/local/bin/codex --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust`
     (both runtimes, FR-021; the daemon adds Codex's `--no-alt-screen` and update-check flags
     itself, `docs/harnesses.md`)
   - `CRSW_SHARED_SECRET` from Secret `crswd-shared-secret` key `secret` (`secretKeyRef`).
   No Service (reached by `kubectl port-forward`, source plan §4).
6. `placeholder-credentials.json`: Secret `claude-credentials` in `crswd-next` with no data
   (source plan §4, so no pod is logged in by accident).

The S2 RBAC files are reused as they are. The SC-004 test still walks all of `deploy/k8s`, so no
value here may contain the literal `.claude` or `lawnmower`. Use `/work` and leave the Claude
config directory to S6's pod template.

`crswdnext_test.go`: each Deployment has the security context above, its `serviceAccountName`,
`selector.matchLabels` equal to its pod-template labels, no `hostPath`, no `privileged`; the
daemon mounts an emptyDir at `/work`; `CRSW_START_COMMANDS` holds a `codex=` entry; the
reconciler's `CRSW_SESSION_NAMESPACE` equals the daemon's; the daemon's env never names a Secret other than `crswd-shared-secret`; images
contain `REPLACE`; the claim's class is `linstor-replicated`.

### §6 `docs/k8s-mode.md`

Sections, as `## ` headings, in this order:
1. What kubernetes mode is (one object per session, the two Deployments, FR-009's namespaces).
2. The two deployments (copy the table from spec 017 "Deployment options"; host is `install.sh`
   plus `crswd unit install`, unchanged; Kubernetes is this doc now and the Helm chart later).
3. Images and how to build them (the commands from §4, verbatim).
4. Install into crswd-next by hand: create the shared secret with
   `kubectl -n crswd-next create secret generic crswd-shared-secret --from-literal=secret="$(openssl rand -hex 32)"`,
   never the VM's (`crswd keygen` prints a release-signing key pair, not this); `kubectl apply -f deploy/k8s/crd.json`, then the RBAC files, then `deploy/k8s/crswd-next/`;
   `kubectl -n crswd-next port-forward deploy/crswd 8765`.
5. Runtimes: Claude Code uses the keeper contract (FR-015, FR-016); Codex uses its own login in
   the namespace, never a copy of the host's (FR-022, spec 019 research M21 to M23). Codex in a
   pod is completed by spec 019 Phase 4a (`ralph/plans/codex-k8s`) and 4b.
6. What is off in this mode: the self-updater, the sign-in relays, `crswd unit` (FR-003).
7. Teardown, including the Retain volume: delete the PVC, patch the Released PV's
   `persistentVolumeReclaimPolicy` to `Delete`, then confirm with `kubectl get pv`.

Plain style: no em-dash, dates M/D/YY.

## Operator-run acceptance (not loop tasks)

The loop cannot run docker or kubectl. After the PR merges, the operator session:
1. Builds both images with the §4 commands and pushes them.
2. Records M20 in `specs/019-codex-runtime/research.md`: `docker run --rm --entrypoint sh <session image> -c 'command -v codex; codex --version'`.
3. Regenerates the crswd-next manifests with the real sha7 (uncommitted), applies them, and runs
   the source plan §6 acceptance: the reconciler holds the Lease; a session made through the API
   gets a pod; deleting the pod brings it back.
4. Records binary size (`ls -l` of `crswd-amd64`) and the daemon pod's RSS
   (`kubectl -n crswd-next top pod`) in spec 017 research, FR-009's missing number.
