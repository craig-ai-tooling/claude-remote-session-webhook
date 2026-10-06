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
