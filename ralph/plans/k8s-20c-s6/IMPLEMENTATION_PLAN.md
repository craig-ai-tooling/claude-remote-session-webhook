# Implementation plan: k8s-20c-s6

S6 of k8s-20c: the reconciler. It keeps one pod per AgentSession, holds every object to spec 001's
containment whoever wrote it, and builds the one pod template that runs a Claude Code **or** a
Codex session (spec 017 FR-006 to FR-010, FR-013, FR-015, FR-021, FR-022; SC-001 logic, SC-005
objects). Source: `specs/017-k8s-native-execution/k8s-20c-plan.md` section 2 "S6", sections 5 (E1
to E5) and 7. The definition of done is `ralph/plans/k8s-20c-s6/VALIDATION_CONTRACT.md`.

## Dependency

S2, S4 and S5 are merged into the branch this loop runs on (S6 uses S5's
`k8s/internal/agentsession` and `sessionpod.WorkRoot`). Check, from the repo root:
`test -f internal/admit/admit.go && test -f k8s/internal/kube/kube.go && test -f k8s/internal/agentsession/agentsession.go && grep -q 'WorkRoot' internal/sessionpod/sessionpod.go`.
If it exits non-zero, follow PROMPT.md "Blocked work". If S5 named the agentsession file
differently, use `ls k8s/internal/agentsession/` and record the real name in PROGRESS.md.

## Tasks

Take the topmost open task. One per iteration. All of them are in the cluster module `k8s/`.

- [x] T1: `k8s/internal/reconcile` config, constants and `ConfigFromEnv`, per Design §1. Verify: `go -C k8s test ./internal/reconcile -run Config -v` passes.
- [x] T2: `PodFor`, the session pod template, per Design §2. Verify: `go -C k8s test ./internal/reconcile -run PodFor -v` passes.
- [x] T3: `ReconcileOne`, the decision table, per Design §3. Verify: `go -C k8s test ./internal/reconcile -run Reconcile -v` passes.
- [x] T4: The loop: informers, queue, `Run`, and `RunWithLease`, per Design §4. Verify: `go -C k8s test ./internal/reconcile -run 'Loop|Lease' -v` passes.
- [ ] T5: The spec and plan amendment per Design §5. Verify: `grep -n 'finalizer' specs/017-k8s-native-execution/plan.md` prints at least one line inside the Reconcile paragraph.
- [ ] T6: Run every command in VALIDATION_CONTRACT.md, record each result in PROGRESS.md, then append `RALPH_COMPLETE`. Verify: `go -C k8s test ./... && go test ./...` exits 0.

## Files touched

A diff outside this list is rejected.

- `k8s/internal/reconcile/` (new package: `config.go`, `pod.go`, `reconcile.go`, `loop.go` and their `_test.go` files)
- `k8s/go.mod`, `k8s/go.sum` (only if `go mod tidy` changes them)
- `specs/017-k8s-native-execution/plan.md`, `specs/017-k8s-native-execution/k8s-20c-plan.md`
- `ralph/plans/k8s-20c-s6/`

Never touch: anything in the root module outside `specs/`, `k8s/cmd/` (S7 owns it),
`k8s/internal/podctl/` and `k8s/internal/agentsession/` (S5's), `k8s/internal/kube/` (S4's),
`deploy/`, `AGENTS.md`, `docs/security.md`, `.github/`, `.claude/`.

## Design

Decided here. Do not reopen it. If one item cannot be built as written, log it under NEEDS
CLARIFICATION in PROGRESS.md, mark the task `- [!]`, and stop.

### §1 Config (`config.go`)

```go
const (
    LabelManagedBy  = "app.kubernetes.io/managed-by"
    ManagedByValue  = "crswd-reconciler"
    LabelSession    = v1alpha1.Group + "/session"
    LeaseName       = "crswd-reconciler"
    SessionContainer = "session" // podctl.Container names the same string; S7's test holds them equal
)
type Config struct {
    SessionNamespace string        // where AgentSessions and session pods live
    LeaseNamespace   string        // the reconciler's own namespace (FR-009)
    Image            string        // the session image, an immutable tag
    Node             string        // kubernetes.io/hostname for nodeSelector; "" sets none
    Claim            string        // the shared ReadWriteOnce claim (FR-010)
    Cap              int           // the concurrency cap Admit enforces
    LifetimeMax      time.Duration // the lifetime ceiling Admit enforces; 0 is none
    ClaudeSecret     string        // the keeper's access-token Secret (FR-015)
    CodexSecret      string        // the namespace's own Codex login (FR-022)
    Home             string        // HOME inside the session image
    UID              int64         // runAsUser, runAsGroup and fsGroup
    Now              func() time.Time
}
func ConfigFromEnv(lookup func(string) string) (Config, error)
```

`ConfigFromEnv` reads, in this order, and returns the first error:

| Field | Variable | Default | Refused when |
|---|---|---|---|
| SessionNamespace | `CRSW_SESSION_NAMESPACE` | none | empty |
| LeaseNamespace | `CRSW_RECONCILER_NAMESPACE` | none | empty, or equal to SessionNamespace (FR-009: the daemon has exec in the session namespace) |
| Image | `CRSW_SESSION_IMAGE` | none | empty, or ends in `:latest`, or has no `:` or `@` after its last `/`. The operator pins a tag that is never re-pushed, or a digest; this check only refuses the two spellings that are certainly mutable |
| Node | `CRSW_SESSION_NODE` | "" | never |
| Claim | `CRSW_SESSION_CLAIM` | `crswd-sessions` | never |
| Cap | `CRSW_MAX_SESSIONS` (`config.EnvMaxSessions`) | `config.DefaultMaxSessions` | not an integer, or < 1 |
| LifetimeMax | `CRSW_SESSION_LIFETIME_MAX` (`config.EnvSessionLifetimeMax`) | `session.AbsoluteLifetime` (24h, the daemon's default ceiling) | `never` (any case) loads as 0, no ceiling; otherwise not a `time.ParseDuration` value, or <= 0. The chart sets the same value on the daemon and the reconciler |
| ClaudeSecret | `CRSW_CLAUDE_SECRET` | `claude-credentials` | never |
| CodexSecret | `CRSW_CODEX_SECRET` | `codex-auth` | never |
| Home | `CRSW_SESSION_HOME` | `/home/ralph` | not absolute |
| UID | none | `10001` | n/a |

Names are validated after defaults: `SessionNamespace` and `LeaseNamespace` with
`validation.IsDNS1123Label`, `Claim`, `ClaudeSecret` and `CodexSecret` with
`validation.IsDNS1123Subdomain` (`k8s.io/apimachinery/pkg/util/validation`); a non-empty result is
an error naming the variable.

Roots are not configurable: every pod's one root is `sessionpod.WorkRoot` (`/work`, E3), so
`func (c Config) Roots() []config.ApprovedRoot { return []config.ApprovedRoot{{Path: sessionpod.WorkRoot}} }`.
`Now` nil means `time.Now`; `func (c Config) now() time.Time`.

Tests (`config_test.go`): defaults; each refusal row; `CRSW_RECONCILER_NAMESPACE` equal to the
session namespace is refused; `CRSW_SESSION_CLAIM=Bad_Name` is refused; `Roots()` is exactly `/work`.

### §2 The pod template (`pod.go`)

`var ErrLifetimeOver = errors.New("reconcile: the session is past its lifetime")`

`func PodFor(obj v1alpha1.AgentSession, cfg Config, now time.Time) (*corev1.Pod, error)`.
Pure. Every field comes from `cfg` or from the object's name, UID, `spec.workDir` and
`spec.lifetime`, never from anything else on the object: the spec has no pod-shaping field
(S2), and nothing here may grow one.

- `activeDeadlineSeconds` counts from the pod's start, not its creation, so a pod that waits to be
  scheduled can outlive the object's deadline by that wait. It is the backstop for when the daemon
  is down; the daemon's reaper ends the session at its exact deadline through `podctl.Kill`.
- `lifetime, err := time.ParseDuration(obj.Spec.Lifetime)`; the deadline is
  `obj.Metadata.CreationTimestamp.Add(lifetime)`; `secs := int64(math.Ceil(deadline.Sub(now).Seconds()))`;
  `secs < 1` returns `ErrLifetimeOver`.
- Let `n := obj.Metadata.Name`, `claudeDir := cfg.Home + "/.claude"`, `codexDir := cfg.Home + "/.codex"`.
- `ObjectMeta`: name `n`, namespace `cfg.SessionNamespace`, labels `{LabelManagedBy: ManagedByValue, LabelSession: n}`,
  one owner reference `{APIVersion: v1alpha1.APIVersion, Kind: v1alpha1.Kind, Name: n, UID: types.UID(obj.Metadata.UID), Controller: ptr(true)}`.
  `BlockOwnerDeletion` stays nil: setting it needs permission on the owner's finalizers, which the
  reconciler's Role does not grant (S2's RBAC).
- `PodSpec`: `RestartPolicy: Never`; `AutomountServiceAccountToken: ptr(false)`;
  `EnableServiceLinks: ptr(false)`; `ActiveDeadlineSeconds: &secs`;
  `TerminationGracePeriodSeconds: ptr(int64(30))`; `NodeSelector` always
  `{"kubernetes.io/arch": "amd64"}` (the session image is amd64 only, and the cluster's Pis are
  arm64), plus `"kubernetes.io/hostname": cfg.Node` when `cfg.Node != ""`; `SecurityContext{RunAsUser, RunAsGroup, FSGroup: &cfg.UID, RunAsNonRoot: ptr(true)}`.
  No `Tolerations` (k8s-20c-plan §7: session pods do not tolerate DRBD lost quorum), no
  `HostNetwork`, no `ServiceAccountName`, no port anywhere.
- `Volumes`:
  - `sessions`: `PersistentVolumeClaim{ClaimName: cfg.Claim}`.
  - `claude-config`: `EmptyDir{}`.
  - `claude-creds`: `Secret{SecretName: cfg.ClaudeSecret, Optional: ptr(true), DefaultMode: ptr(int32(0o440))}`.
  - `codex-creds`: `Secret{SecretName: cfg.CodexSecret, Optional: ptr(true), DefaultMode: ptr(int32(0o440))}`.
  Both Secrets are mounted as directories, never with `SubPath`: a `SubPath` mount never sees the
  keeper's rotation (FR-015). Both are optional, so a namespace that has not signed one runtime in
  yet still runs the other. Each is mounted in its sidecar **and** in the session container at the
  same path, because the link the sidecar writes is resolved in the session container's mount
  namespace, and a link to a path that container lacks dangles.
- `InitContainers`, two native sidecars (`RestartPolicy: ptr(corev1.ContainerRestartPolicyAlways)`),
  both `Image: cfg.Image`, `Command: []string{"bash", "-c", linkScript}`, requests `5m`/`16Mi`,
  limit memory `32Mi`:
  - `creds-link`: env `LINK=<claudeDir>/.credentials.json`, `TARGET=/var/run/claude-creds/.credentials.json`,
    `CREDS_LINK_INTERVAL=60`; mounts `claude-config` at `claudeDir`, `claude-creds` at
    `/var/run/claude-creds` read-only.
  - `codex-creds-link`: env `LINK=<codexDir>/auth.json`, `TARGET=/var/run/codex-creds/auth.json`,
    `CREDS_LINK_INTERVAL=60`; mounts `sessions` with `SubPath: n + "/codex"` at `codexDir`,
    `codex-creds` at `/var/run/codex-creds` read-only.
  - Each sidecar has a `StartupProbe` so the session container starts only after the first link
    exists: `Exec{Command: []string{"bash", "-c", "[ ! -e \"$TARGET\" ] || [ \"$(readlink \"$LINK\")\" = \"$TARGET\" ]"}}`,
    `PeriodSeconds: 1`, `FailureThreshold: 30`. A native sidecar with a startup probe holds the
    main container until the probe passes.
  - `linkScript` is a package constant, the loop from ai-lawnmower `ralph-runner/job.yaml` (the
    `creds-link` sidecar) with one change, a guard so an absent optional Secret leaves no dangling
    link:
    ```bash
    set -u
    while :; do
      if [ -e "$TARGET" ] && [ "$(readlink "$LINK" 2>/dev/null)" != "$TARGET" ]; then
        if [ -e "$LINK" ] || [ -L "$LINK" ]; then
          echo "creds-link: $(date -u +%FT%TZ) $LINK was replaced; re-linked to $TARGET"
        fi
        ln -sfn "$TARGET" "$LINK"
      fi
      sleep "$CREDS_LINK_INTERVAL"
    done
    ```
    It is the only shell string in the package, it is a constant, and it contains no value from
    the object.
- `Containers`, one, named `SessionContainer`: `Image: cfg.Image`; `Args: []string{"session-pod", n, obj.Spec.WorkDir}`
  (the image's entrypoint is `tini -- /usr/local/bin/crswd`); `WorkingDir: sessionpod.WorkRoot`;
  env exactly `HOME=cfg.Home`, `CLAUDE_CONFIG_DIR=claudeDir`, `CODEX_HOME=codexDir`,
  `LANG=C.UTF-8`, `TERM=xterm-256color`; mounts:
  `sessions` `SubPath: n + "/work"` at `sessionpod.WorkRoot`; `claude-config` at `claudeDir`;
  `sessions` `SubPath: n + "/projects"` at `claudeDir + "/projects"`; `sessions`
  `SubPath: n + "/codex"` at `codexDir`; `claude-creds` at `/var/run/claude-creds` read-only;
  `codex-creds` at `/var/run/codex-creds` read-only. Requests `cpu: 50m`, `memory: 570Mi`, no
  limits (FR-008).
  `SecurityContext{AllowPrivilegeEscalation: ptr(false), Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}}`.
- The template does not branch on the runtime. Both runtimes' homes and credentials are present in
  every pod, and the start command typed by the daemon decides which one runs (FR-021).

Tests (`pod_test.go`), one object fixture with name `crswd-abc`, UID `u1`, workDir `/work/repo`,
lifetime `8h`, created at `now - 1h`:
- `TestPodForDeadline`: `ActiveDeadlineSeconds` is 25200; created at `now - 8h` returns `ErrLifetimeOver`.
- `TestPodForNoSecretReadNoHostAccess`: walk the pod: no `HostPath` volume, no `HostNetwork`,
  `AutomountServiceAccountToken` false, no container port, no `SubPath` on either Secret mount,
  no env var whose value comes from a Secret (`ValueFrom` nil everywhere).
- `TestPodForBothRuntimes`: the session container env has `CLAUDE_CONFIG_DIR` and `CODEX_HOME`;
  `CODEX_HOME`'s mount is the claim at `crswd-abc/codex`; `claudeDir/projects` is the claim at
  `crswd-abc/projects`; both sidecars exist with `RestartPolicy` Always and a startup probe; every
  `TARGET` a sidecar links to lies under a path the session container also mounts.
- `TestPodForOwnerAndLabels`: one controller owner reference to the object's UID; labels present;
  `BlockOwnerDeletion` nil.
- `TestPodForIgnoresObjectExtras`: annotations and labels on the object do not appear on the pod.
- `TestPodForNoTolerations` and `TestPodForNodeSelector` (`kubernetes.io/arch: amd64` always;
  the hostname key only when `cfg.Node` is set).

### §3 `ReconcileOne` (`reconcile.go`)

```go
type Reconciler struct { kube kubernetes.Interface; sessions *agentsession.Client; dyn dynamic.Interface; cfg Config }
func New(kube kubernetes.Interface, dyn dynamic.Interface, cfg Config) (*Reconciler, error) // builds agentsession.New(dyn, cfg.SessionNamespace)
func (r *Reconciler) ReconcileOne(ctx context.Context, name string) error
```

Constants: `AnnotationRecreates = v1alpha1.Group + "/pod-recreates"`, `MaxRecreates = 5`,
`AnnotationConversation = v1alpha1.Group + "/conversation"` (the key podctl writes for
`@crswd-conversation`).

The table, first match wins:

1. `sessions.Get(name)` absent: return nil. The pod's owner reference lets the garbage collector
   delete it, and `podctl.Kill` confirms it is gone (FR-006). The source plan's finalizer is
   dropped: S2's `ObjectMeta` has no `deletionTimestamp`, so the reconciler cannot see a deletion in
   progress, and podctl already observes the pod gone before it reports a teardown.
2. Phase `Rejected` or `Failed`: return nil. Terminal; nothing is created for it, ever.
3. Get pod `name` from `kube.CoreV1().Pods(cfg.SessionNamespace)`. A pod that exists but whose
   controller owner reference is not this object's UID, or that lacks `LabelManagedBy=ManagedByValue`,
   is not ours: set status `Failed`, reason `a pod with this session's name exists and is not
   owned by it`, touch nothing else, return.
   - a. Exists with `DeletionTimestamp != nil`: return nil. No replacement while the old pod object
     exists, terminating included (FR-010: two writers fork a conversation).
   - b. Exists, phase `Failed`, `Status.Reason == "DeadlineExceeded"`: set status `Failed`, reason
     `the session reached its lifetime`; return.
   - c. Exists, phase `Failed` or `Succeeded` otherwise: read `AnnotationRecreates` (absent is 0).
     At `MaxRecreates` or more, set status `Failed`, reason `the session pod failed 5 times in a row`,
     and return without deleting (the pod stays for `kubectl logs`). Otherwise delete it with
     `metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &pod.UID}}` and nothing else
     (never `GracePeriodSeconds`), increment the annotation with `sessions.Update`, set status
     `Reviving`, return. The next event recreates it. This bounds a pod that fails at every start,
     such as one whose working directory resolves outside `/work` in the pod (`sessionpod.Run`).
   - d'. Exists, phase `Running`, and `AnnotationRecreates` is present: remove it with
     `sessions.Update` (the pod came up, so the count starts over).
   - d. Exists, phase `Running`: set status `Running`; return.
   - e. Exists, any other phase: if the object's phase is empty set `Pending`; return. A Pending pod
     is left alone, never recreated in a loop (k8s-20c-plan §7).
4. Absent: `others, err := sessions.List(ctx)`; `ok, reason := admit.Admit(obj, cfg.Roots(), cfg.Cap, cfg.LifetimeMax, others, cfg.now())`.
   Not ok: status `Rejected` with `reason`; return. No pod.
5. `pod, err := PodFor(obj, cfg, cfg.now())`. `ErrLifetimeOver`: status `Failed`, reason
   `the session reached its lifetime`; return.
6. Re-read the object live, never from the informer cache: `live, err := sessions.Get(ctx, n)` through
   the dynamic client. Absent, or `live.Metadata.UID != obj.Metadata.UID`: return nil and create
   nothing. This narrows the window in which `podctl.Kill` deletes the object, sees no pod, and
   returns while this reconciler is about to create one (cross-family review of S5, 10/6/26). A
   pod that still slips through carries an owner reference to a deleted UID, so the garbage
   collector removes it, and it only ever runs a login shell because the daemon is the only writer
   of the start command (E1). Test: a fake whose Get returns NotFound on this second read gets no
   pod create.
7. Create the pod. `AlreadyExists` is nil (the deterministic name is the one-pod guard). Status
   `Reviving` when the object's phase was `Running` or `Reviving`, else `Pending`.

"Set status" means: compute the wanted status (phase, reason, and `conversation` = the object's
`AnnotationConversation` annotation when non-empty, else the current `status.conversation`); when
it differs from the object's status, call `sessions.UpdateStatus(ctx, name, wanted)`. A conflict is
returned, and the queue retries. A status write never touches `spec`. This is the only writer of
`status.conversation` (spec 017 FR-005), copied from the record the daemon keeps on the object.

Tests (`reconcile_test.go`, `k8s.io/client-go/kubernetes/fake` for pods and the dynamic fake for
objects, as S5's agentsession tests build it; fixed `cfg.Now`):
- missing pod: exactly one `create pods` action, status `Pending`;
- terminating pod (`DeletionTimestamp` set): zero creates;
- failed pod: one delete whose options have `GracePeriodSeconds == nil` and a UID precondition,
  status `Reviving`, zero creates in the same call;
- `DeadlineExceeded` pod: status `Failed`, no delete, no create;
- workDir outside `/work`: status `Rejected` with a reason, zero pods;
- cap 1 with an older admitted object: the newer one is `Rejected`, zero pods;
- past lifetime: `Rejected` or `Failed` with zero pods;
- object absent: no action at all;
- a previously `Running` object whose pod is gone gets `Reviving` and one create;
- `AlreadyExists` on create is not an error;
- a failed pod with `pod-recreates` at 5: status `Failed`, zero deletes;
- a pod named like the object but owned by another UID: status `Failed`, zero deletes, zero creates;
- an object with the conversation annotation: its status gains that conversation.
Count actions with `fake.Clientset.Actions()` filtered by verb and resource.
Name the outside-root test `TestReconcileOutsideRoot` and the cap test `TestReconcileOverCap`.

### §4 The loop (`loop.go`)

- `func (r *Reconciler) Run(ctx context.Context) error`:
  1. Objects: `dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.dyn, 30*time.Second, r.cfg.SessionNamespace, nil).ForResource(agentsession.GVR).Informer()`;
     add, update and delete enqueue the object's name (`cache.MetaNamespaceKeyFunc`, then
     `cache.SplitMetaNamespaceKey`; handle `cache.DeletedFinalStateUnknown`).
  2. Pods: `informers.NewSharedInformerFactoryWithOptions(r.kube, 30*time.Second, informers.WithNamespace(ns), informers.WithTweakListOptions(func(o *metav1.ListOptions) { o.LabelSelector = LabelManagedBy + "=" + ManagedByValue }))`;
     events enqueue `pod.Labels[LabelSession]` when non-empty.
  3. Queue: `workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())`.
  4. Start both factories, `cache.WaitForCacheSync`, then one worker: `ReconcileOne`; error is
     `AddRateLimited`, success is `Forget`; always `Done`.
  5. On `ctx.Done()`, shut the queue down and return `ctx.Err()`.
  The resync of 30 s re-runs every object, which is what turns a pod deleted while the reconciler
  was down into a recreate.
- `var leaseTimings = kube.ElectorConfig{LeaseDuration: 15 * time.Second, RenewDeadline: 10 * time.Second, RetryPeriod: 2 * time.Second}`,
  a package variable so a test can shorten it, and `var runLoop = func(r *Reconciler, ctx context.Context) error { return r.Run(ctx) }`,
  so a test can make the loop fail. Tests that change either do not call `t.Parallel()`.
- `func RunWithLease(ctx context.Context, kc kubernetes.Interface, identity string, r *Reconciler) error`:
  1. `ctx, cancel := context.WithCancel(ctx)`; `defer cancel()`; `runErr := make(chan error, 1)`.
  2. `c := leaseTimings; c.Namespace, c.Name, c.Identity = r.cfg.LeaseNamespace, LeaseName, identity`.
  3. `elector, err := kube.NewElector(kc, c, func(lctx context.Context) { if err := runLoop(r, lctx); err != nil && lctx.Err() == nil { runErr <- err; cancel() } }, func() {})`.
     S4's signature: `kube.NewElector(client kubernetes.Interface, c kube.ElectorConfig, onStart func(context.Context), onStop func()) (*leaderelection.LeaderElector, error)`.
  4. `elector.Run(ctx)`.
  5. Return the error from `runErr` if there is one, else `ctx.Err()`.
  A reconciler whose loop died must not keep renewing the Lease: it returns, the process exits,
  and another replica, or this one restarted, takes over. One reconciler acts at a time (FR-006).

Tests (`loop_test.go`): `TestLoopCreatesPodForNewObject` (run `Run` on fakes in a goroutine,
create an object through the dynamic fake, poll up to 5 s for one pod); `TestLeaseOneActs`
(set `leaseTimings` to `2s`/`1s`/`200ms` as S4's test does; two reconcilers with `RunWithLease` on
one fake clientset; create one object; after 4 s exactly one `create pods` action across the
shared clientset); `TestRunWithLeaseReturnsLoopError` (override `runLoop` to return
`errors.New("boom")` at once; `RunWithLease` with a 10 s context returns that error well before
the context ends).
Register the list kind for the dynamic fake as S5's agentsession tests do.

### §5 Spec amendment

1. In `specs/017-k8s-native-execution/plan.md` Design, "Reconcile." paragraph: replace "recreate a
   deleted one with `--resume <conversation>`" with "recreate a deleted one; the daemon's
   supervisor types `--resume <conversation>` into it (E1)", and add one sentence: there is no
   finalizer, because `podctl.Kill` confirms the pod gone and the object type carries no
   `deletionTimestamp` (S6 §3 rule 1).
2. In `specs/017-k8s-native-execution/k8s-20c-plan.md` §2 S6, add a line under the pod template
   bullet: both runtimes' homes and credential sidecars are in every pod (FR-021, FR-022), and
   the Codex login is a directory-mounted optional Secret re-linked like Claude's.
Keep the spec's plain style: no em-dash, dates M/D/YY.
