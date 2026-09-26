# k8s-20c decomposition: crswd `kubernetes` mode (spec 017, plan steps 4 and 5)

Written 9/26/26 by an Opus planner for backlog item k8s-20c, after the credential gate (D8a, D8c) and the three costs (D8b) were measured. Each slice below is a backlog item `k8s-20c-s1` to `k8s-20c-s7`; k8s-20c closes when S7 does. Two decisions belong to Craig (section 5) and are posted as holds `k8s-20c-decide-lib` and `k8s-20c-decide-group`; the third was taken by the Chief of Staff.

Read-only investigation, 9/26/26, against crswd origin/main `979ceab` (#185). The local checkout is
`main`, 1 commit behind, and that commit touches only spec/research. Line numbers are from
origin/main. Nothing was changed in any repo, cluster or backlog. "Unverified" marks what I did not run.

## 0. Findings that change the shape (read first)

1. **The repo forbids go.sum, and three tests enforce it.** `docs/security.md:387-395` (a CODEOWNERS path) says "`go.sum` must not exist". Asserted by `internal/config/docs_test.go:43` (TestNoDependencies), `internal/release/readme_test.go:140` (TestReleaseHasNoDependencies) and `cmd/crswd/quickstart_test.go:2338` (TestQuickstartNoDependencies). go.mod today is 3 lines, `go 1.23.0`, no require. Adding client-go to the root module reddens all three. That contradicts SC-003 ("quickstart passes with no test edited") and needs a protected-path edit. This is decision 1 in section 5.
2. **client-go v0.32.8 as resolved fails dependency-review.** Throwaway module, `go mod tidy`: `go 1.23.0` kept (CI runs Go 1.24, `ci.yml:174`), 42 modules. OSV today reports two HIGH advisories: `moby/spdystream v0.5.0` (GHSA-pc3f-x583-g7j2, fixed 0.5.1) and `x/oauth2 v0.23.0` (GHSA-6v2p-p543-phr9, fixed 0.27.0). `dependency-review.yml` has `fail-on-severity: high`. Pinning `spdystream@v0.5.1 oauth2@v0.27.0 gorilla/websocket@v1.5.3` builds and keeps `go 1.23.0` (measured). What remains is moderate only: x/net v0.30.0 has fixes at 0.36/0.38/0.55; x/sys and x/text have no severity. Licenses: not checked, unverified. The known set is Apache/BSD/MIT, and the deny list is GPL/AGPL/LGPL-3.
3. **Binary size.** The root `crswd` today is 14.95 MB. A stub that only imports the client-go packages below is 47.6 MB. The host binary grows by roughly 30 MB if client-go goes into the root module. That is an estimate from a stub; the real wiring is measured in S7.
4. **A nested module can import the root's `internal/`.** Verified in a scratch pair of modules (`example.com/a/k8s` imports `example.com/a/internal/x`, with replace `../`). Root `go test ./...` does not descend into a nested module. So option A of decision 1 needs a `ci.yml` edit to test it.
5. **SC-002 is FR-012, and FR-012 is backlog item k8s-20j (queued).** It is not in k8s-20c's blocked-by. `journalRecord` has no Name (`internal/session/journal.go:60-80`). k8s-20c cannot meet its done-when until k8s-20j lands. Recommend adding `blocked-by:k8s-20j` (I did not change the backlog).
6. **SC-001's "under 60 s" is a live timing.** A fake client proves the logic: an old pod gone leads to one new pod, and the supervisor sends `--resume`. The seconds belong to plan step 7 (the kill test).
7. **No GitHub Actions secrets or vars exist for image builds in crswd.** The repo has only `CLAUDE_CODE_OAUTH_TOKEN, QUEUE_APP_KEY, QUEUE_TOKEN, RELEASE_SIGNING_KEY` and var `QUEUE_APP_CLIENT_ID`. ai-lawnmower has `DOCKERHUB_*` and `CI_RUNNER(_AMD64)` at repo level. The VM is x86_64 and logged in to docker.io and ghcr.io (hosts read from ~/.docker/config.json, no values printed).
8. **Open PRs on crswd: none.** k8s-20c reads `in_flight`. `crswd-next` does not exist in the cluster. The cluster has no ClaudeSession CRD. The server is v1.32. Classes: `linstor-replicated` (Retain, WFFC) and `linstor-fs-storage-enc` (Delete, default).

## 1. Seams (origin/main)

- **Boot.** `cmd/crswd/main.go:160` run() does loadConfig, then `cfg.CheckDependencies` (`internal/config/depcheck.go:99`, which probes tmux), then `updater.NewStager().Sweep()` (`main.go:202`), `updater.NewUnit` report (`:216`) and `newDaemon`. Subcommands are dispatched at `main.go:88` (keygen) and `:95` (unit). The seams are `bypass_prod.go:24` loadConfig and `:26` newDaemon, which is `httpapi.New`.
- **Controller construction.** `internal/httpapi/server.go:326` New builds `tmuxctl.NewExec` (`:337`), then `NewWith(cfg, tmux, audit.New())` (`:405`). After that it wires the sign-in relay `loginrelay.New` (`:364`) and the updater `releaseFeed` (`:380`). NewWith is the injection seam for a second Controller. FR-003 falls out of it: a kubernetes path calls NewWith and skips both.
- **Controller interface.** `internal/tmuxctl/controller.go:21`: New, SetOption, SendKeys, Paste, CapturePane, Resize, Kill, Has, List, ReconcileServerEnvironment. `SessionInfo` is at `:226`. The argv builders are unexported in `fake.go:50-185` (argvNew…argvList). Exec is `exec.go:87` (NewExec) and `:220` (CapturePane). The only consumers are `session.Manager` (`manager.go:202`, `NewManager :364`) and `loginrelay` (`loginrelay.go:121`, a narrowed interface).
- **Session create.** HTTP `httpapi/sessions.go:438` and `actions.go:520` call `Manager.Create` (`manager.go:644`). Inside: `ResolveWorkDir` (`:675`, `session/workdir.go:66`, symlink-resolving on the daemon's own fs), then `start` (`:1936`): tmux.New, then SetOption ×8 (managed, owner, name, workdir, start, lifetime, conversation, binary), then SendKeys(start command). The conversation id is minted up front (`conversationFor :2198`).
- **Revival.** Supervisor `supervisor.go:116` Sweep and `:141` judge, then `revive :303` (tmux.New when the shell is gone) and `sendStart :328`. Startup is `server.go:1157` ReplayJournal, then `:1183` Adopt (`manager.go:1697`, which rebuilds from `tmux.List` options). Journal: `server.go:490` `SetJournal(NewJournal(JournalPath))`, `journal.go:60` record, `manager.go:2213` createRecord, `:2269` ReplayJournal.
- **Pane read.** `httpapi/stream.go:97` `streamInterval = time.Second`, and `:425` `sessions.Output` goes to `tmux.CapturePane` (`manager.go:1421`).
- **Config.** Keys are `CRSW_*` env vars, and the file key is the lowercased var (`config/file.go:265-270`). `Vars()` (`file.go:192`) must list a new var (TestVarsNamesEveryDeclaredVariable). README's var table (`docs_test.go` readmeVars) and `.env.example` (`envexample_test.go:25`) must name it. `LoadFrom` is at `config.go:652`. So `execution.mode` is spelled `CRSW_EXECUTION_MODE` / `execution_mode`.
- **Guards.** CODEOWNERS covers `.claude/`, `.github/workflows/`, `.specify/memory/`, `AGENTS.md` (147/150 lines, CI cap) and `docs/security.md`. golangci: gosec, errcheck(blank), staticcheck, with no depguard. CI YAML parse covers only `.github/workflows/*.yml`. The constitution's quality gate is "No NEEDS CLARIFICATION left unanswered", and VI says "listener binds loopback only".

## 2. Slices (each one Sonnet sitting, one PR to main, host mode byte-identical)

Every new test must fail against the old code. Run it by hand on main first, and never commit a test
that reads origin/main. Always run: `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./...
&& go test -tags quickstart ./cmd/crswd && golangci-lint run`, plus the same in `k8s/` once it exists.

**S1: mode switch and refusals.** Root module, stdlib. FR-001, FR-003, FR-004, SC-003.
- Files: `internal/config/config.go` (EnvExecutionMode, `type ExecutionMode` host|kubernetes, default host, unknown value refused), `file.go` Vars(), README var row, `.env.example`. `cmd/crswd/main.go`: `unit *` refuses in kubernetes mode, naming the mode, and run() skips Sweep/unit report. `depcheck.go`: no tmux probe in kubernetes mode. `httpapi/server.go` New: no relay, no releaseFeed, and the update routes answer "disabled in kubernetes mode". Journal off in kubernetes mode, so ReplayJournal and Adopt can never both revive one session. The settings page gains a row through `config.Vars()`, and `settings_test.go:637` ranges over Vars(), so it follows with no edit. I found no quickstart or tmux test that pins the var count (grep). Until S7, the root binary refuses to *start* in kubernetes mode with a sentence saying why.
- Tests: config parses `kubernetes` (the old code rejects it as an unknown key), rejects `k8s`, and absent means host. Unit refusal, update-route refusal, relay nil, journal nil in kubernetes mode. No existing test edited.
- Deps: none. **Parallel** with S2, S3 and S4.

**S2: ClaudeSession API, admission, manifests.** Root module, stdlib. FR-005, FR-007 (logic),
FR-013, SC-004, SC-005.
- `api/v1alpha1/types.go`: plain structs with JSON tags. Spec: sessionName, owner, workDir, startCommand (the configured *key*, never a command line), conversation, lifetime. Status: phase (Pending|Running|Rejected|Reviving|Failed), reason, conversation. No token field.
  **No pod-shaping field** (image, SA, env, volumes, Secret, command), or anyone who can write the object gets pod creation through the reconciler.
- `internal/admit/admit.go`: pure `Admit(obj, roots, cap, others, now) (ok, reason)`. Lexical allowlist (path.Clean plus `session.underAnyRoot` exported as `UnderAnyRoot`), cap counted by creationTimestamp, lifetime. `session`: add `SetWorkDirResolver` so the daemon checks lexically in kubernetes mode, because it cannot see the pod's filesystem (`workdir.go:66` EvalSymlinks). The resolved-and-verified check (constitution VI) moves into the pod (S3). Host mode is unchanged.
- `deploy/k8s/` **JSON** manifests (kubectl applies JSON, no YAML dependency) generated from Go values by `go run ./deploy/k8s/gen`, with a byte-drift test. Contents: the CRD, the reconciler Role (pods create/get/list/watch/delete, claudesessions get/list/watch/update, claudesessions/status update, all in the session namespace, plus leases get/create/update in its own namespace), and the daemon Role (claudesessions full CRUD, pods get/list/watch, pods/exec create+get: the verbs D8b measured, in the session namespace only).
- Tests (SC-005): walk every generated rule: any rule naming `secrets` or `*` fails; a daemon rule with pods create fails; daemon pods/exec in the reconciler namespace fails. Admit table: outside allowlist, `..` escape, over cap, past lifetime each come back Rejected with a reason. A reflect test fails if the Spec gains a field outside the allowed set. A grep test fails if any fixture or manifest names namespace `lawnmower` Secrets or `~/.claude` (SC-004).
- Amends spec FR-013, which lists no Lease verbs and no `claudesessions/status`, and fills FR-005's group.
- Deps: **decision 2 (API group)**. **Parallel** with S1 and S4.

**S3: in-pod side.** Root module, stdlib. FR-002, FR-011 (pod half), FR-015 (pod shape).
- `tmuxctl`: exported wrappers `ArgvNew…ArgvList` that call the unexported builders. No rename, so the tmux-tagged tests stay unedited. New root stdlib package `internal/sessionpod` (tested by root CI). `Run(name, workdir, roots)` first runs `session.ResolveWorkDir` (EvalSymlinks) inside the pod and exits non-zero if the dir escapes the root, and the reconciler then marks the object Failed. Then it runs `tmuxctl.Exec.New` (login shell only, as v0) and returns when the tmux session is gone. `PaneLoop(name, w)` runs `Exec.CapturePane` every 1 s and writes frame + `0x1E` to stdout. This is the FR-011 loop with no shell string, and it exits when stdout breaks, so a cut stream leaves no orphan. `deploy/session-image/Dockerfile`: `FROM docker.io/nctiggy/ralph-runner:2.1.246-ci10` (claude 2.1.246 + tmux 3.5a, D8a/D8b) plus COPY of the k8s-module binary. Under option A these are dispatched as `session-pod`/`pane-loop` only from `k8s/cmd/crswd`, so the v0 binary gains nothing (FR-004).
- Tests: PaneLoop emits separated frames from the fake and exits on a closed writer. Run exits when Has turns false and refuses a workdir that is a symlink out of the root (a temp dir). Exported wrappers equal the unexported builders.
- Deps: none (no `cmd/crswd` edit). **Parallel** with S1, S2 and S4.

**S4: Kubernetes client foundation.** Shape set by decision 1. Recommended A: a nested module
`k8s/go.mod` (`module …/claude-remote-session-webhook/k8s`, replace `../`), which leaves root go.sum absent.
- `go get k8s.io/client-go@v0.32.8` (matches server v1.32.8 and D8b's generator, keeps go 1.23.0), then pin `github.com/moby/spdystream@v0.5.1 golang.org/x/oauth2@v0.27.0 github.com/gorilla/websocket@v1.5.3`.
  **No controller-runtime.** It adds prometheus, zap and friends for nothing that `kubernetes/fake` + `dynamic/fake` + `tools/leaderelection` do not already cover. ClaudeSession goes through the dynamic client, converted to and from `api/v1alpha1` in one file.
- `k8s/internal/kube/` holds the in-cluster rest.Config and the Lease elector (LeaseLock). Test: two electors on one fake clientset, exactly one leads. First-PR check: dependency-review green, OSV clean of HIGH.
- Deps: decision 1, and S4w first under A. **Serial gate**: the only slice that touches go.mod/go.sum, and S5/S6 follow it.
- Also under A: `.github/dependabot.yml` gets a gomod entry for `/k8s` (not a workflow path), and `docs/security.md` §5 is rescoped to the host module (CODEOWNERS path, so Craig merges that PR).
- S4w (option A only), lands BEFORE S4: `.github/workflows/ci.yml` adds `go -C k8s vet/test/build` and golangci in `k8s/`, guarded by `[ -f k8s/go.mod ]` in the Detect-stack pattern (`ci.yml:155`), so it is green before the module exists. A workflow file: its own branch of that one file, pushed over SSH and fast-forwarded (no token here can merge `.github/workflows/**`). Without it S4-S6 merge green with their tests never run.

**S5: podctl, the second Controller.** `k8s/internal/podctl`. FR-002, FR-011, FR-014.
- `New`: create the ClaudeSession if absent, then wait (bounded, ~90 s; D8b cold 45-52 s) for pod Running and `has-session`. It is idempotent, which revival needs. `SetOption`: annotation on the object (the object is the record; pod tmux options die with the pod). `SendKeys`/`Paste`/`Resize`/`Has`: exec with the exported argv, and Paste sends the payload on exec stdin (load-buffer), as v0. `CapturePane`: newest frame of one held `crswd pane-loop` exec stream per watched session, reopened on EOF, with ANSI-stripped again on the daemon side. `Kill`: delete the object and confirm the pod gone (never "assumed"). `List`: SessionInfo from objects+pods. `ReconcileServerEnvironment`: empty Reconciliation.
- Exec goes behind an `Executor` interface. The real one is `remotecommand` WebSocket with SPDY fallback, and tests use a recording fake, since the fake clientset cannot exec. Tests: argv per method equals `tmuxctl.Argv*`; Paste bytes reach stdin, never argv; Kill without pod-gone returns an error; Has distinguishes an API error from absent; a restart re-mints (Adopt over podctl.List marks CredentialPending, FR-014).
- Deps: S2, S3, S4. **Parallel** with S6 (different package; go.mod untouched).

**S6: reconciler.** `k8s/internal/reconcile` and a `reconcile` subcommand. FR-006, FR-007, FR-008, FR-010,
FR-015, SC-001 (logic), SC-005 (objects).
- Loop: informers on ClaudeSession and pods (label + ownerRef), under the Lease. Admit fails → status Rejected, no pod. Pod name `crswd-<id>`, so AlreadyExists is the one-pod guard. **No replacement while the old pod object exists** (terminating included). Delete with default grace, never GracePeriodSeconds=0. Deleted object leads to ownerRef GC, with the finalizer confirming the pod gone. Pod Failed with DeadlineExceeded means the object is Failed and the pod is not recreated.
- Pod template (all fixed in code or reconciler config, none from the object): session image, uid/fsGroup 10001, requests 50m/570Mi with no CPU limit, `nodeSelector` lm-amd64-1 (value), `activeDeadlineSeconds` = created+lifetime−now, `automountServiceAccountToken: false`, `restartPolicy: Never`, **no** lost-quorum toleration, claim subPath `<id>/work` → `/work` and `<id>/projects` → `$CLAUDE_CONFIG_DIR/projects`, `CLAUDE_CONFIG_DIR` an emptyDir, Secret `claude-credentials` mounted as a directory (never subPath), and native sidecar `creds-link` copied from ai-lawnmower `ralph-runner/job.yaml:500-525`. The entrypoint is `crswd session-pod`.
- **Start command owner (E1 below): the daemon's supervisor**, as v0. Amend plan.md's Design "recreate … with `--resume`" in this PR.
- Tests (fake clientset): missing pod → one create. Terminating pod → zero creates. Delete opts have no zero grace. Rejected leads to zero pods (outside allowlist, over cap). Deadline set and decreasing on recreate. Pod spec has no `secrets` read, no hostPath, no SA token. Two reconcilers leads to one acting.
- Deps: S2, S4. **Parallel** with S5.

**S7: wiring, images, crswd-next manifest, acceptance.** `k8s/cmd/crswd` main: host mode delegates to
the root code path, and kubernetes mode runs `httpapi.NewWith(cfg, podctl, audit.New())` with no relay or feed.
`reconcile`, `session-pod` and `pane-loop` are dispatched here only. `deploy/image/Dockerfile` (COPY-only, distroless static nonroot),
`deploy/k8s/crswd-next/*.json`, `docs/k8s-mode.md`. Record binary size and RSS (FR-009's missing number).
Deps: all. Serial, last.

## 3. Images

- **crswd image** (daemon and reconciler, same image): `go build` with `GOARCH=amd64` and `arm64`, then a COPY-only Dockerfile, so `buildx --platform linux/amd64,linux/arm64` needs no qemu. Built by hand on the VM for k8s-20c. `docker.io/nctiggy/crswd:2.0.0-dev.<sha7>` (Docker Hub, because the cluster has no private-registry pull secret). Immutable: never re-push a tag, since IfNotPresent caches it.
- **session image**: `docker.io/nctiggy/crswd-session:2.1.246-<sha7>`, **amd64 only** (the session host is lm-amd64-1; ralph-runner's amd64 slice), built natively on the VM.
- **Workflow files**: k8s-20c needs none except S4w. The v2.* tag workflow to ghcr is k8s-20's. It is a protected path landed over SSH, on ARC `arc-amd64`/`arc-arm64` runners. crswd would first need `CI_RUNNER*` vars and registry secrets, which it lacks (§0.7). Whether ARC runners serve this repo is unverified.

## 4. Manifest for crswd-next (hand-applied, k8s-20c) versus the chart (k8s-20)

k8s-20c (JSON, `kubectl apply -f`):
- Namespaces `crswd-next` (daemon + session pods) and `crswd-next-reconciler` (FR-009: the daemon has no exec there). PR #184 and the spec name no namespace (checked); these names are mine.
- SA `crswd` in crswd-next and SA `crswd-reconciler` in crswd-next-reconciler. Role+RoleBinding `crswd-reconciler` in crswd-next bound to the reconciler SA from the other namespace. Lease Role in its own namespace. **No verb on secrets** in either.
- CRD `claudesessions.<group>`. One PVC `crswd-sessions` in crswd-next on `linstor-replicated` (Retain, FR-010), sized for the sum of sessions. This also measures what D8b left out: that class and its subPath behaviour. Cleanup of a Retain volume: delete the PVC, then patch the Released PV to `persistentVolumeReclaimPolicy: Delete` so the provisioner removes it. That is standard, but unverified on this driver, so the cleanup ends with a `kubectl get pv` check.
- Deployments: reconciler (1 replica, no port, tolerates `drbd.linbit.com/lost-quorum`) and daemon (listener on 127.0.0.1, reached by `kubectl port-forward`, no Service/ingress, tolerates lost-quorum).
- A placeholder Secret `claude-credentials` in crswd-next (`{}`), so no pod is logged in (§7). A daemon Secret with a fresh `CRSW_SHARED_SECRET` from `crswd keygen`, never the VM's (US3.1). No browser door is required: `validateDoors` (`config.go:1847`) allows the API alone, so `port-forward` + `crswd-api` is enough.
- Namespace reading of the done-when: "runs in `crswd-next`" and FR-016 (keeper login in `crswd-next`) make `crswd-next` the session+daemon namespace. FR-009/013 then put the reconciler's Deployment in `crswd-next-reconciler`, reconciling `crswd-next` through its Role there. I take this as meeting the done-when, and the executor states it in the PR.

k8s-20: Helm chart with CRD (`install.crds`/`upgrade.crds`), Retain claim, keeper login (FR-016), ghcr,
Flux HelmRelease 2.x, hostname crswd-next.craigcloud.io behind Access (cloudflared sidecar, keeping loopback).

## 5. Decisions

DECISION FOR CRAIG
what: the Kubernetes mode needs the Kubernetes client library, and crswd forbids any dependency
need: DECIDE — where the library lives
how:
  A) separate build for the cluster (recommended) → the VM daemon and its releases stay exactly as today,
     with no dependency and the rule untouched. The cluster image is built from a second module in the same repo.
     Cost: two build paths, one CI step added through the SSH route, and "one binary" becomes one codebase.
     The no-dependency rule still needs one sentence scoping it to the VM build, and you merge that.
  B) one binary for both → the VM daemon also carries the library (about 30 MB larger, pinned
     versions, Dependabot noise). The no-dependency rule and its three tests are rewritten. You merge it.
  C) no library → we write our own small Kubernetes client, including the WebSocket exec path.
     No rule changes, but it becomes the riskiest code in crswd.

DECISION FOR CRAIG
what: the session object needs a permanent API group name, and changing it later means migrating every object
need: DECIDE — the group name
how:
  A) buy crswd.dev (recommended) → the group is `crswd.dev`, reads as the project's (spec FR-005),
     and nobody else can take it. About $12 a year. Unregistered on 9/26/26 (DNS lookup).
  B) your domain → `crswd.craigcloud.io`. Free now, but it names you in other people's clusters.
  C) crswd.dev without buying it → free, but someone else could register it later.

Why client-go at all (security.md §5 asks): exec over WebSocket/SPDY, watches and leader election. The stdlib has none of these, and D8b measured the exec path on client-go v0.32.8.

DECIDED 9/26/26 (Chief of Staff, option A): crswd-next stays a scratch track with its own empty disk until the k8s-15 cutover. Sessions in it cannot reach the lawnmower state (backlog, ~/code), which lives in another namespace; the build and the kill test go ahead, and k8s-15 decides where the state lives. Low stakes and reversible, so it is not Craig's card.

Executor takes these (recorded in the PR and spec):
- E1: the start command has one owner, the daemon supervisor (`supervisor.go:303,328`). The pod runs tmux with a login shell only. It reuses v0's bounded revival and keeps "no byte to a pane built by new code" (FR-002), and research D2 already rejected a second reviver. The cost is that a pod deleted while the daemon is down sits at a shell until the daemon returns.
- E2: kubernetes mode has no journal. The object is the record, and Adopt over podctl.List revives.
- E3: the daemon and the reconciler check the workdir lexically. The pod re-checks it with symlinks resolved (S3), which keeps constitution VI's "resolved and verified". The root is the in-pod `/work`.
- E4: JSON manifests generated from Go, drift-tested. No YAML library, no controller-gen.
- E5: no controller-runtime. client-go v0.32.8 with the three pins.
- FR-016 (keeper login in crswd-next) is a DO for k8s-20 and blocks nothing in k8s-20c: Craig's browser once, `loop/claude-keeper-login.sh`.

## 6. Acceptance ("reconciler runs in crswd-next from a hand-applied manifest")

```
kubectl get nodes -o custom-columns=N:.metadata.name,T:.spec.taints[*].key   # expect no lost-quorum
kubectl apply -f deploy/k8s/crswd-next/            # CRD, 2 ns, SAs, Roles, PVC, placeholder Secret, 2 Deployments
kubectl -n crswd-next-reconciler rollout status deploy/crswd-reconciler --timeout=180s
kubectl auth can-i create pods -n crswd-next --as=system:serviceaccount:crswd-next:crswd        # no
kubectl auth can-i get secrets -n crswd-next --as=system:serviceaccount:crswd-next-reconciler:crswd-reconciler  # no
kubectl auth can-i create pods/exec -n crswd-next-reconciler --as=system:serviceaccount:crswd-next:crswd  # no
kubectl -n crswd-next-reconciler get lease                                      # holder = reconciler pod
kubectl apply -f <ClaudeSession workDir=/etc>  → status Rejected, no pod
kubectl apply -f <ClaudeSession workDir=/work> → pod crswd-<id> Running, 2/2 (tmux + creds-link)
kubectl delete pod crswd-<id>   (default grace) → exactly one new pod, created only after the old is gone
kubectl -n crswd-next-reconciler delete pod -l app=crswd-reconciler → session pod UID and start time unchanged
kubectl top pod -n crswd-next-reconciler                                         # FR-009 RSS number
daemon via port-forward: create a session; restart the daemon pod; in the session pod `ps` shows one pane-loop
```

Cleanup: delete the ClaudeSessions and wait for the pods to be gone (never `--force`). Delete the two namespaces, then the CRD. For the Released PV of `crswd-sessions`: `kubectl patch pv <pv> -p '{"spec":{"persistentVolumeReclaimPolicy":"Delete"}}'`. Then `kubectl get pv | grep crswd-next` must be empty. If it is not, report it and leave the PV. Never touch LINSTOR (break-glass). Leave the images.

## 7. Hazards and where each is tested

- **Keeper login**: acceptance uses a placeholder Secret, and no manifest names `lawnmower` Secrets (SC-004 grep test, S2). The reconciler never reads a Secret (Role test S2, pod-spec test S6).
- **NetworkPolicy is not enforced (D8b §3)**: no pod port. The daemon stays on loopback, the reconciler has no port, and the pane path is RBAC-gated exec (S5/S6 tests: no containerPort in the pod template).
- **Session pods appear in Craig's claude.ai list and run what he types (D8c)**: no logged-in pod in k8s-20c. Session pods get no SA token and no hostPath (S6 test). Before k8s-20 logs one in, tell Craig the name prefix.
- **Two live writers fork a conversation**: deterministic pod name, no replacement while the old object exists, and the Lease (S6 tests plus acceptance step "created only after the old is gone").
- **Never force-delete**: S6 delete-options test. The cleanup text says so.
- **DRBD lost-quorum**: session pods do not tolerate it and the volumeless daemon/reconciler do (S6 template test). A Pending pod is left alone, not recreated in a loop (S6). The acceptance pre-check reads taints; check volumes with `drbdsetup status --verbose | grep quorum:no`.
- **Orphaned capture loops (D8b unmeasured)**: pane-loop exits on broken stdout (S3 test), and the acceptance counts them.
- **Unverified**: licenses of the client-go closure; ARC runner availability for crswd; held exec across API-server/kubelet restart; `linstor-replicated` subPath; the VM's claude 2.1.283 in a pod.
