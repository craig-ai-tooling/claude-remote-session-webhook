# Progress: k8s-20c-s6

Notebook for the S6 slice of k8s-20c. Newest entry at the bottom.

## Iteration 0 (planning, 10/6/26)

Plan written by the operator's planner against main `50d7e9c`, with S2 and S4 in their loops and
S5 planned. Facts checked on disk, so no iteration has to rediscover them:

- S2's `api/v1alpha1.AgentSession`: spec fields `sessionName, owner, workDir, startCommand,
  conversation, lifetime` (lifetime a Go duration string); status `phase, reason, conversation`;
  phases `Pending Running Rejected Reviving Failed`. `ObjectMeta` has `Name, Namespace, UID,
  CreationTimestamp, Labels, Annotations, Finalizers` and no `DeletionTimestamp`.
- S2's `admit.Admit(obj, roots, cap, maxLifetime, others, now) (bool, string)` (the ceiling was added in review of PR #211) is pure.
- S4 merged (#207): `kube.InCluster() (*rest.Config, error)` and
  `kube.NewElector(client kubernetes.Interface, c kube.ElectorConfig, onStart func(context.Context), onStop func()) (*leaderelection.LeaderElector, error)`;
  `ElectorConfig` has `Namespace, Name, Identity, LeaseDuration, RenewDeadline, RetryPeriod`.
- The session image is amd64 only, so every session pod selects `kubernetes.io/arch: amd64`.
- S5's `agentsession.Client` (Get, List, Create, Delete, SetAnnotation, `UpdateStatus(ctx, name, status)`, `Update(ctx, name, mutate)`) and `GVR`. Both updates re-read the object, so they carry its resourceVersion.
- The session image is `ralph-runner` plus `crswd`; it runs as uid 10001 (`useradd -u 10001 ralph`)
  with HOME `/home/ralph`, and `ralph-runner:2.1.246-ci12` already carries `codex 0.153.4`
  (ai-lawnmower `ralph-runner/README.md`). Do not mount anything over `/home/ralph` itself: the
  image keeps a venv there.
- The credential sidecar shape is ai-lawnmower `ralph-runner/job.yaml`, the `creds-link` block.
- Lint of the cluster module runs in CI, not here: the loop cannot `cd` into `k8s/`.

## Notes for S7 (do not act on these here)

- The `reconcile` subcommand: `cfg, err := reconcile.ConfigFromEnv(os.Getenv)`, an in-cluster
  config from `kube.InCluster()`, `kubernetes.NewForConfig` and `dynamic.NewForConfig`, then
  `reconcile.RunWithLease(ctx, kc, <pod hostname>, r)`.
- Add a test that `reconcile.SessionContainer == podctl.Container`.
- The manifest needs the env names in `ConfigFromEnv` and the claim `crswd-sessions`.

## NEEDS CLARIFICATION

None open.
BLOCKED: dependency not merged

- Operator (10/6/26): S5 merged (#214); dependency met, branch rebased onto main. Ignore the BLOCKED line above.

## Iteration 1 (T1, config, 10/6/26)

- Added `k8s/internal/reconcile/config.go` and `config_test.go`: `Config`, the constants, `Roots()`, `now()`, `ConfigFromEnv` per Design 1.
- Failing first: `go -C k8s test ./internal/reconcile -run Config` printed `config_test.go:27:14: undefined: ConfigFromEnv` (package did not exist, so the test did not compile).
- Gate: all green, quickstart included (8765 was free). `k8s/go.mod` and `go.sum` unchanged by `go mod tidy`.
- Flake, not fixed: the first `go test ./...` run failed three `internal/sessionpod` tests (`TestRunTakesAnEmptyListAsTheSessionBeingGone`, `TestRunKillsAndConfirmsTheSessionOnCancel`) on 5s timeouts while other packages ran in parallel. The package passes alone (2.3s) and a second full run was clean. Root module untouched, so it predates this slice.
- Next iteration: `v1alpha1` lives at repo-root `api/v1alpha1`, imported as `github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1`. `agentsession.Client.Get` returns `(obj, found, err)`. `ApprovedRoot` is `{Path, IsDefault}`. An image whose registry has a port but no tag (`host:5000/img`) is refused, covered by a test.

## Iteration 2: T2 PodFor

- Added `k8s/internal/reconcile/pod.go` and `pod_test.go`: `PodFor`, `ErrLifetimeOver`, the `linkScript` constant, two native credential sidecars, the one session container, per Design 2.
- Failing first: `go -C k8s test ./internal/reconcile -run PodFor` printed `pod_test.go:73:12: undefined: PodFor` (did not compile; the function did not exist).
- Gate: all green, quickstart included (8765 free). Root `go.sum` absent, `grep -c require go.mod` is 0. Cluster module vet, test, build clean. `k8s/go.mod` untouched; lint of `k8s/` left to CI per the prompt.
- Next iteration: `pod.go` already defines a generic `ptr[T]`; reuse it in `reconcile.go`. `pod_test.go` defines `podNow`, `podCfg()`, `podObj()` and `envOf2` (`envOf` is taken by `config_test.go`), reuse them in `reconcile_test.go`. An unparseable `spec.lifetime` (including `never`) returns a plain wrapped error, not `ErrLifetimeOver`; admission in T3 rejects it first, so rule 5 only sees valid durations.
- Noticed, not fixed: none.

## Iteration 3: T3 ReconcileOne

- Added `k8s/internal/reconcile/reconcile.go` and `reconcile_test.go`: `Reconciler`, `New`, `ReconcileOne` (rules 1 to 7), `setStatus`, `AnnotationRecreates`, `MaxRecreates`, `AnnotationConversation`.
- Failing first: `go -C k8s test ./internal/reconcile -run Reconcile` printed `reconcile_test.go:23:8: undefined: Reconciler` (did not compile; the type did not exist).
- Gate: all green, quickstart included (8765 free). Root go.sum absent, `grep -c require go.mod` is 0. Cluster module vet, test, build clean. Lint of `k8s/` left to CI.
- Next iteration: `reconcile_test.go` has `newRig(t, cfg, objs, pods...)` returning `rig{r, kube, dyn}` with `count(verb, resource)` and `status(t, name)`; reuse it in `loop_test.go`. `Reconciler.dyn` is only written so far; T4 reads it for the informer. `Admit` filters the object itself out of `others` by UID, so List needs no filtering. Unparseable `pod-recreates` counts as 0.
- Noticed, not fixed: none.

## Iteration 4: T4 loop and lease

- Added `k8s/internal/reconcile/loop.go` and `loop_test.go`: `Run` (dynamic and pod informers, rate-limited queue, one worker), `RunWithLease`, `leaseTimings`, `runLoop`, per Design 4.
- Failing first: `go -C k8s test ./internal/reconcile -run 'Loop|Lease'` printed `loop_test.go:33:26: g.r.Run undefined (type *Reconciler has no field or method Run)` (did not compile).
- Gate: all green, quickstart included (8765 free). Root go.sum absent, `grep -c require go.mod` is 0. Cluster module vet, test, build clean; loop tests also pass under `-race`. `k8s/go.mod` unchanged. Lint of `k8s/` left to CI.
- Next iteration: T5 is spec text only (Design 5). T6 runs VALIDATION_CONTRACT.md. The lease tests take about 4 s and change package variables, so they must not call `t.Parallel()`. `createObject(t, g, ns)` in `loop_test.go` makes an object through the dynamic fake.
- Noticed, not fixed: none.

## Iteration 5: T5 spec amendment

- Edited `specs/017-k8s-native-execution/plan.md` (Reconcile paragraph: the supervisor types `--resume`, no finalizer) and `k8s-20c-plan.md` S6 (both runtimes' homes and credential sidecars in every pod), per Design 5. No em-dash, no dates added.
- Failing first: `grep -n 'finalizer' specs/017-k8s-native-execution/plan.md` printed nothing before the edit; it now prints line 48, inside the Reconcile paragraph.
- Gate: all green, quickstart included (8765 free). Root go.sum absent, `grep -c require go.mod` is 0. Cluster module vet, test, build clean. Lint of `k8s/` left to CI.
- Next iteration: T6 only. Run every command in VALIDATION_CONTRACT.md and record each result, then append `RALPH_COMPLETE`. Test results were cached this run; use `-count=1` when recording T6 evidence.
- Noticed, not fixed: none.

## Iteration 6 (T6): validation contract

All run with `-count=1` from the repo root.
- Host module unchanged: `git diff --name-only origin/main...HEAD -- . ':!k8s' ':!specs' ':!ralph'` printed nothing; `test ! -e go.sum` exit 0.
- Root gate: build, vet, test, `golangci-lint run` exit 0 (0 issues). `-tags tmux ./...` no failures. `-tags quickstart ./cmd/crswd` ok (8765 free). `grep -c require go.mod` prints 0.
- Cluster module: vet, test, build exit 0. Lint of `k8s/` left to CI.
- `PodForNoSecretReadNoHostAccess`, `PodForBothRuntimes`, `PodForDeadline`: PASS.
- `Reconcile`: PASS; the delete case checks `GracePeriodSeconds` nil (reconcile_test.go:125).
- `Reconcile.*(Outside|Cap)`: PASS (`TestReconcileOutsideRoot`, `TestReconcileRecreateCapReached`).
- `Lease`: PASS (`TestLeaseOneActs`, `TestRunWithLeaseReturnsLoopError`).
- `grep -n finalizer specs/017-k8s-native-execution/plan.md`: line 48.
- Noticed, not fixed: none.

RALPH_COMPLETE
