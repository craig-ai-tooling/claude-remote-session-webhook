# Research: Kubernetes-Native Execution

**Feature**: 017-k8s-native-execution | **Date**: 2026-09-25

Measured on rpi-inference (namespace `crswd-next-scratch`, created and deleted for this)
and on the VM, 9/25/26. Where a claim was checked, the check is shown. The full kill-test
record is ai-lawnmower `docs/crswd-v2-kill-test.md`.

---

## D1 — A pod restart kills every process. The disk survives.

**Kill test.** StatefulSet `session-host`, one replica on lm-amd64-1, image
`nctiggy/ralph-runner:2.1.246-ci8`, 1Gi RWO PVC on `linstor-fs-storage-enc`. Container
`tmux-host` owns the tmux server; container `daemon` creates and lists sessions over a
socket in a shared emptyDir. The session runs a worker that checkpoints a counter to the
PVC each second and continues from it when restarted: the `--resume` stand-in.

| Kill | Recovery | Process | PVC state | `@crswd-*` options |
|---|---|---|---|---|
| `kubectl exec -c daemon -- kill -TERM 1` | 7.3 s | same pane pid (15), no gap | intact | intact |
| `kubectl delete pod` | 44.4 s (32.2 s of it the stand-in ignoring SIGTERM) | new process, 41 s gap | counter contiguous from 154 | gone; re-set by the reviver from a PVC record |
| `kubectl delete pod --grace-period=0 --force` | 8.0 s | new process, 4 s gap | counter contiguous from 197 | same |

**Consequence.** A container restart is a VM daemon restart: `Adopt` takes the session
back. A pod restart is a VM reboot: `Adopt` finds nothing and only `ReplayJournal` can
bring a session back. Section 2 of ai-lawnmower `docs/crswd-k8s-scout.md` asked this; it
is now answered.

**Not run.** No `claude` ran in the scratch pod. The keeper-written session Secret lives in
namespace `lawnmower`, and a scratch pod could reach it only by copying it out, which the
task forbade. A new login needs the operator's browser. So `claude --resume` after a pod
restart and Remote Control re-registering are unmeasured. The daemon container ran a loop,
not crswd.

---

## D2 — v0 already has the revival path A needs

`internal/httpapi/server.go:1157` runs `ReplayJournal` before `Adopt` at startup.
`ReplayJournal` (`internal/session/manager.go`) puts back every session the journal says
should run and tmux does not have, as `StateStarting` and `CredentialPending`, and the
supervisor recreates it with `--resume <conversation>` (spec 012). Under A that code runs
unchanged if the journal directory and `CLAUDE_CONFIG_DIR/projects` are on the PVC.

**Alternatives rejected**: a reviver outside crswd (the kill test's stand-in). It works,
and it duplicates what `ReplayJournal` already does under the allowlist, cap and deadline
rules.

---

## D3 — That path failed 8 of 8 on its one real run

The VM rebooted 9/22/26 20:00 (`last -x reboot`), which is the pod-restart case. The
daemon's audit log from 20:05:01:

```
8  startup.adopt       "took back a session the journal recorded and the host no longer had"
8  supervisor.recreate attempt 1 of 3
16 supervisor.revive   attempts 2 and 3
8  supervisor.failed   "revival was attempted the maximum number of times and stopped"
revive session 1e49d8c2…: render the start command: the session name cannot be put into
the start command: the start command needs a session name and this session has none
```

`journalRecord` (`internal/session/journal.go:60`) carries id, owner, conversation,
workdir, start, lifetime, created and attempts. It has no name, and `ReplayJournal` builds
the session without one. The operator's start command needs the name. Every named session
fails to revive.

**Decision**: journal the name and restore it on replay (FR-012). It is a v0 bug as well
and can ship to v0 on its own.

---

## D4 — Option A versus option B, in numbers

| | A. Session host | B. Operator | Source |
|---|---|---|---|
| Memory per session | ~570 MiB | ~570 MiB plus a per-pod agent and the `creds-link` sidecar | k8s-12 |
| Session start | 3.1 s alone, 5.4 to 8.2 s six at once | 24 s with one PVC; 93, 94, 98 s with three PVCs at once, each after one `DeadlineExceeded` from the CSI driver | k8s-12; kill test |
| PVCs | 1 | 1 per session (15 at lm-amd64-1's memory ceiling) | |
| Daemon restarts on the VM, 8/26 to 9/25/26 | 37 starts, 3 of them reboots | | `journalctl --user -u crswd` |
| Releases, 8/24 to 9/24/26 | 29 (v0.97 to v0.126) | | `git tag` |
| What an upgrade costs sessions | pod roll: all restart and resume, 8 to 44 s. Daemon container only: nothing, 7.3 s | nothing | kill test |
| Host-bound code | reused: `session` 5,567 and `tmuxctl` 1,986 non-test lines | replaced for pod mode, kept for host mode | `wc -l` |
| FR-021 on a daemon restart | re-mint through `Adopt` | re-mint for every session unless token hashes persist in the CR, which changes FR-021 | code |
| Lawnmower state beside it | same RWO volume, same node | cannot share a per-session PVC | kill test |

**Superseded as a decision, not as evidence (9/25/26).** The operator chose B. The numbers
below stand and are what B has to manage; D8 and D9 list what choosing it leaves unmeasured.

B's benefit is real only at an upgrade cadence the k8s mode will not have: k8s-20 pins v2
to a Flux-managed 2.x, and the operator asked that nothing update itself. At 29 releases a
month rolled by hand in windows, A's cost is a 44 s resume per rollout. If FR-010's
in-place container patch works, the cost is zero. The 8/30/26 scout's "do not build an
operator, not yet earned" still holds with numbers.

---

## D5 — FR-021 needs no change

Spec 001 FR-021: a restart makes every pre-restart token dead. `Adopt` and
`ReplayJournal` both mint a hash, discard the token, and mark the session
`CredentialPending`. `ClaimPending` hands a fresh token to the first `GET /sessions`.
The lawnmower keyring captures it (`loop/session_keyring.py`, written after the 9/1/26
loss of 12 of 13 tokens).

In a pod the rule reads the same and fires on the same events: a daemon container restart
or a pod restart. A re-mints one token per session per restart, as the VM does now at 37
starts a month. Persisting hashes so tokens survive a restart would contradict FR-021's
reason ("undrivable by anyone, including whoever held the credential before the restart")
and is not proposed.

---

## D6 — What is still unmeasured

- `claude --resume` and `--remote-control` after a pod restart on a keeper login (D1).
- A StatefulSet with `updateStrategy: OnDelete` and a patched `crswd` container image:
  whether the kubelet restarts that container alone and leaves `tmux-host` running.
- The keeper chart installed into `crswd-next` with its own login.
- Long sessions: k8s-12's tasks ran 14 to 28 s.

---

## D7 — Where the lawnmower state lives

The console and the group-A timers (reconcile, metrics, repo-drift, mail) move in the
k8s-15 window (k8s-10 answer A, 9/23/26). They read and write backlog.md, ledger.jsonl,
outcomes.jsonl, flight-watch.json, metrics.db, session-tokens.jsonl and the `~/code`
checkouts, which sessions also write. The hourly S3 copy (#332) is a backup. Nothing reads it as a live
store.

**Decision**: one RWO volume, `lawnmower-home`, owned by the lawnmower chart
(`helm.sh/resource-policy: keep`, `linstor-replicated`). The crswd chart mounts it as an
existing claim. The console and the CronJobs pin to lm-amd64-1 and mount it too.

**Measured**: a second pod on lm-amd64-1 mounted the session host's RWO claim and wrote a
line the session host read. A pod on node4 stuck in `ContainerCreating` with
`Multi-Attach error ... Volume is already used by pod(s) session-host-0, reader-lm-amd64-1`.

**Rationale**: it keeps the VM's semantics (many processes, one filesystem, one host), so
tasks-axi's file locking, sqlite and `git -C` work unchanged. RWX for a filesystem is
refused by every StorageClass here (measured 9/12/26, ai-lawnmower
`docs/claude-auth-and-the-execution-plane.md`).

**Alternatives rejected**: a state service in front of the files (every CLI that writes
them would change); an S3 feed (a copy up to an hour old, with writes landing nowhere the
sessions see); state inside each session PVC under B (not shareable).

**Cost**: everything on one node, as everything is on one VM today.

---

## D8 — What choosing B leaves unmeasured

Written 9/25/26 when the operator chose B. Nothing here was run for this amendment; each line
is a measurement to make before the requirement it feeds is decided.

| Unmeasured | Feeds | How to measure |
|---|---|---|
| `claude --remote-control` and `claude --resume` on the keeper's access-token-only credentials, in a pod, across a pod restart | FR-017, the gate | A pod in the namespace that owns the Secret, mounting it as a directory and never copying it out (D1 refused a copy). Record the `claude` version. |
| Cost of the pane path per watched session: exec API against an in-pod agent | FR-011 | `streamInterval` is 1 s (`internal/httpapi/stream.go`), one `CapturePane` per watched session per interval. Run ten sessions' worth against the control plane on the Pis and read the API server's CPU and the call latency. |
| A `subPath` per session on one shared RWO claim | FR-010 | Start six pods at once, each on its own `subPath`, and time them against D4's 93 to 98 s for three PVCs. D7 measured whole-claim mounts only. |
| A session pod's `claude` under tmux, resuming after a forced delete with two writers briefly alive | FR-010, edge case | `kubectl delete pod --grace-period=0 --force` (D1's third row) with `claude` instead of the stand-in. |
| The keeper chart in a second namespace with its own login | FR-016 | Install `claudeKeeper` into `crswd-next`, bootstrap once with the operator's browser. |
| Whether the CNI enforces NetworkPolicy | FR-011 | The nodes report Flannel, which does not enforce it by itself. Create a deny policy and probe. |
| The CRD under Flux: `install.crds` and `upgrade.crds` behaviour on a chart upgrade | k8s-20 | Helm never upgrades a chart's `crds/` directory, and a CRD is cluster-scoped. |

---

## D8a — The credential gate (k8s-20a): PASS

Measured 9/26/26, 03:35Z to 03:49Z. This answers D8's first row and D6's first bullet, and
FR-017 now says pass. One run, one pod, one restart; what it did not cover is listed last.

**Versions.** Pod image `docker.io/nctiggy/ralph-runner:2.1.246-ci10` (index digest
`sha256:5986748b2442efdee9e23a96ce4351b317f8953676d7b750a290d03efdb47ad7`), `claude --version`
in the pod: 2.1.246. The keeper's own image is tagged `2.1.246-ci8`. The VM
runs 2.1.283 today; that version was not run against the keeper login.

**The pod.** `k8s-20a-probe` in namespace `lawnmower`, the Secret's own namespace, so nothing was
copied out. Node `lm-amd64-1` (amd64 only), requests 50m CPU and 512Mi, limit 1Gi, uid and fsGroup
10001. It followed the keeper's session contract, taken from `ralph-runner/job.yaml`:
`claude-credentials` mounted read-only as a directory at `/var/run/claude-creds` (no `subPath`),
`CLAUDE_CONFIG_DIR` an emptyDir holding `.credentials.json` as a symlink to it, the `creds-link`
native sidecar at 60 s. Storage was one 1Gi PVC per session (D4's shape B, `linstor-fs-storage-enc`,
reclaim policy Delete) mounted at `/data`, holding the working directory `/data/work` and the
transcripts, with `CLAUDE_CONFIG_DIR/projects` a symlink to `/data/projects`. The config dir stayed
an emptyDir, so `.claude.json` and `settings.json` were rebuilt from a seed step in the new pod.
The seed sets `hasCompletedOnboarding`, folder trust and the bypass-permissions acceptance;
no login screen or refresh prompt appeared at any point.

| Step | Command | Result |
|---|---|---|
| Inference baseline | `claude -p "Reply with the single word ok."` | `ok`. `claude auth status`: `loggedIn: true`, `authMethod: claude.ai`, `subscriptionType: max`, with no account block in the seeded `.claude.json` |
| Remote Control | in tmux: `claude --dangerously-skip-permissions --remote-control k8s-20a-probe` (crswd's `rc` command) | Pane after 15 s: `/remote-control is active · Continue here, on your phone, or at https://claude.ai/code/session_…`, status bar `/rc active` |
| A turn | prompt with the codeword `pelican-4471` | answered; transcript `e9d27344-….jsonl` written under `/data/projects/-data-work/` |
| Pod restart | `kubectl delete pod` (10 s grace, `claude` killed with the container), then `kubectl apply` of the same manifest | New pod UID, same PVC, Running 61 s after the delete began. Container restarts 0 in the new pod |
| Resume | in the new pod, same cwd: `claude --dangerously-skip-permissions --remote-control k8s-20a-probe --resume e9d27344-…` | Pane replayed the earlier turn. `/remote-control is active` again, on the same session URL as before the restart |
| Context after restart | "What codeword did I ask you to remember?" | `pelican-4471`. The same transcript file grew from 70,496 to 74,367 bytes |
| End | `/exit` | `claude` and the tmux server gone. Pod and PVC deleted afterwards |

**No refresh was needed or attempted.** `kubectl logs -c creds-link` was empty each time it was read (the first pod's
before its deletion, the second pod's at the end), so Claude never replaced the credential file,
and the symlink was intact at the end.
`lawnmower/keeper-state` on the keeper Secret read `ok` before and after, `refreshed-at`
unchanged at 00:30:13Z, `expires-at` 08:30:11Z, so 295 minutes left at the start and 281 at
the end. No keeper pass was run by hand and no second refresher existed.

**Cleanup.** `kubectl -n lawnmower get pods,pvc -l app=k8s-20a-probe` returns nothing and the
PV `pvc-ada2fcf6-…` is `NotFound`. The Remote Control session was ended with `/exit`; its state on
claude.ai was not checked, because listing a claude.ai session from here means reading the
credential.

**Not covered, so the pass does not extend to it.**

- **A token rotation under a live Remote Control session.** The keeper rotates the Secret about
  every 6 h (it refreshes under 2 h left of an 8 h token). The probe ran 14 minutes and never saw
  one. The keeper doc's measurement that a running session reads the file per request was for
  inference. Whether the Remote Control connection holds its own copy of the token is open, and
  every B session that lives past its first token will meet it.
- **A prompt sent from claude.ai.** Registration and reconnect were read from the pane. Nothing was
  typed into the session from the phone or the browser.
- **The VM's `claude` 2.1.283**, and the arm64 nodes. Both are one run away.
- **A hard kill.** `--force --grace-period=0` with two writers alive is D8's separate row.
- **A different working directory after the restart.** Transcripts sit under a directory named for
  the cwd (`-data-work`), so the pod must start `claude` in the same directory it had.
- **The login in `crswd-next`.** FR-016's own keeper login is a separate bootstrap. This was the
  `lawnmower` namespace's login, the same access-token-only shape.

---

## D8b — The three costs, measured (k8s-20b)

Measured 9/26/26, 03:42Z to 04:42Z, on rpi-inference in namespace `k8s-20b-scratch`. The
namespace is deleted (`kubectl get ns` lists nothing of it, and no PV has a claim in it), and
the node taints were read empty after each storage run. This settles D8's rows for the pane path,
`subPath` storage, the forced delete and NetworkPolicy. It does not touch D8a's credential gate.
Decisions are in FR-009, FR-010 and FR-011. Scripts and raw output were kept outside the repo.

### 1. The pane path (FR-011)

**Method.** Ten target pods on `lm-amd64-1`, each a tmux session with a 120x40 screen that
repaints every second (a capture is about 3.1 KB, the size of a `claude` screen). One generator
pod on the same node, a Go program on client-go v0.32.8 (server is v1.32.8) using an in-cluster
ServiceAccount whose Role holds `pods` get and list and `pods/exec` create and nothing else, the
FR-013 shape. Each of the ten workers reads its pane once a second at a random phase, as
`streamInterval` does. Calls run from inside the cluster, because the kubeconfig on the VM goes
through Palette's console proxy and would time that tunnel. API-server CPU is the cumulative
`container_cpu_usage_seconds_total` of the three `kube-apiserver` static pods, read from each
node's cadvisor every 20 s and interpolated over the generator's own start and end. Every window
carries the same sampling traffic. Latency is measured in the generator, per call, from dial to
last byte.

| Path, ten sessions, one read a second each | Calls | Failed | Median | p95 | p99 | Max | 3 API servers | lm-amd64-1 |
|---|---|---|---|---|---|---|---|---|
| No load, 146 s | | | | | | | 461 m | 132 m |
| No load, 169 s (after the runs) | | | | | | | 354 m | 123 m |
| Exec per call, SPDY, 300 s | 3000 | 0 | 52.6 ms | 84.5 ms | 113.3 ms | 260.4 ms | 564 m | 388 m |
| Exec per call, WebSocket, 150 s | 1500 | 0 | 46.6 ms | 66.9 ms | 82.2 ms | 274.9 ms | 543 m | 364 m |
| In-pod agent over HTTP on the pod IP, 150 s | 1500 | 0 | 2.5 ms | 3.9 ms | 5.4 ms | 11.6 ms | 361 m | 155 m |
| One held exec stream per session, 150 s | 1500 frames from 10 execs | 0 | 1002.5 ms | 1004.5 ms | 1008.0 ms | 1017.4 ms | 327 m | 151 m |

CPU is millicores summed over the three API servers, and the whole `lm-amd64-1` root cgroup,
which includes the generator process. On the held-stream row the latency columns are the gaps
between frames, because the pod loop sleeps one second; they show the stream did not stall.

- **Per call, the API servers rise by about 160 m at ten calls a second**, about 16 m per call a
  second. The mean of the two per-call runs is 553 m and the mean of the three windows with no
  exec load (the two baselines and the agent run) is 392 m. Against the spread of those quiet
  windows the gain is between 82 m and 210 m. The quiet windows themselves span 327 to 461 m,
  so any single window is uncertain by about 100 m.
- **Per call, `lm-amd64-1` rises by about 250 m** (388 and 364 against 132 and 123 with no
  load), and its kubelet from 22 and 25 m to 65 and 66 m. A `runc exec` per read is the cost.
  The agent and the held stream cost that node about 20 to 30 m.
- **The exec API answers a call in 53 ms at the median (47 ms over WebSocket) and 275 ms at
  the worst**, against a 1 s interval, so latency is not the objection to it. Two of 3000 SPDY
  captures came back at 40 bytes with exit status 0: the stand-in screen was caught between its
  `clear` and its repaint. That is a race in the stand-in and the API answered correctly.
- **No latency drift** over the 300 s run: the five one-minute medians were 50.7, 53.4, 56.3,
  51.5 and 53.5 ms.
- **What else ran.** Before and after each window I counted pods outside the scratch namespace
  that were not Running and Ready, and Jobs with `active` above zero. The SPDY, WebSocket, held
  and second no-load windows began and ended at zero of each. The first no-load window began with
  one `arc-runners` pod Pending, which had gone by its end. The agent window ended at 04:29:19Z and
  the keeper and token-refresh CronJobs of the 04:30 tick appeared in the count taken just after
  it. The windows ran 04:16:14 to 04:21:15Z (SPDY), 04:22:54 to 04:25:25Z (WebSocket), 04:26:48
  to 04:29:19Z (agent) and 04:35:06 to 04:37:36Z (held). The cluster also carries ambient load
  (metrics-server, vmagent, the Palette agents), which is in every row.
- **Scope.** The targets ran `tmux` and a repaint loop, not `claude`, so a real session's
  exec would be a little slower. Each per-call read opened its own connection. One held stream
  ran 150 s without a drop; an API-server restart, a VIP failover and a kubelet restart were not
  tried, and neither was what an orphaned capture loop does after its stream is cut.

### 2. Per-session storage (FR-010)

**Method.** Pods on `lm-amd64-1` by `nodeSelector` (not `nodeName`, which would bypass the
scheduler and leave a `WaitForFirstConsumer` claim Pending), image `ralph-runner:2.1.246-ci8`,
uid and fsGroup 10001, 1Gi claims on `linstor-fs-storage-enc`. That class is D4's, and its
reclaim policy is `Delete`, so nothing survives the namespace. `linstor-replicated`, which the
design would use, has the same layer stack (DRBD, LUKS, `placementCount` 2, pool `fs1`) and
reclaim `Retain`; it also sets `allowRemoteVolumeAccess`, which was not exercised. A pod is Ready
when it has written its marker file, so Ready means writable. The clock runs from the start of
`kubectl apply` to the last pod Ready. About 5 s of each figure is `kubectl` creating the objects
through the console proxy. The kill-test record that D4's clock came from is not on disk, so the
per-PVC rows were re-run under this clock.

| Case | Time to all Ready | Notes |
|---|---|---|
| One shared claim, six pods at once, claim not yet bound | 49.1 s and 52.0 s | includes the one provision |
| One shared claim, six pods at once, claim already bound | 33.5 s and 30.7 s | volume re-attached, six mounts |
| One shared claim, first pod alone, claim not yet bound | 45.0 s | |
| One shared claim, a pod added while others run | 5.6 s, 5.2 s and 6.7 s | the everyday start |
| One PVC per pod, one pod | 58.2 s | D4 measured 24 s |
| One PVC per pod, three at once | 82.2 s | D4 measured 93 to 98 s; `DeadlineExceeded` on at least two claims |
| One PVC per pod, six at once | 153.3 s | 10 `ProvisioningFailed` events |
| One PVC per pod, six at once, `linstor-thin` (LUKS, one replica, no DRBD) | 118.1 s | 16 `ProvisioningFailed` events |

The wait is the provisioner. An unreplicated class was no faster, so DRBD is not the cause. On the
shared claim the pods came Ready about 3 s apart, which is the kubelet mounting them one after
another.

**Writable.** Under `fsGroup: 10001` the kubelet made each `subPath` directory `root:ralph`
mode 2775, and uid 10001 wrote its marker in all six pods. Without `fsGroup` was not tried.

**Forced delete, with `claude`.** The image is reachable and has `claude` 2.1.246 and tmux 3.5a.
There is no login in this run: the keeper's Secret was not touched. `claude` ran as its
interactive terminal UI inside tmux, as `--dangerously-skip-permissions --model sonnet`, with
`ANTHROPIC_BASE_URL` pointed at a small server in the same pod that answers `/v1/messages`
with a fixed reply. So the binary, its terminal UI, its transcript and config writes and its
`--resume` are real, and the model is not. The pod wrote a heartbeat line every 100 ms and sent
the session a prompt every 2 s. `CLAUDE_CONFIG_DIR` and the working directory sat on one
`subPath` of the shared claim.

- **Force delete then recreate at once, three times** (`kubectl delete pod --grace-period=0
  --force`, then `kubectl apply` of the same pod). The old pod's last heartbeat and the new pod's
  first were 5.3, 4.3 and 3.7 s apart. No overlap in any of the three. Each new pod found the
  transcript and ran `claude --resume` on it. After the first, the transcript held 200 lines with
  0 invalid JSON lines, 0 duplicate message ids and no fork, the old pod's 27 turns and the new
  pod's 10 on one chain, and `.claude.json` parsed. The second and third forced deletes ran after
  the two-writer step below, and again showed 0 invalid lines and 0 duplicate ids and added no
  fork of their own.
- **Two live writers on purpose**, a second pod on the same `subPath` for 45 s, each `claude`
  taking a prompt every 1 to 2 s. Nothing was corrupted: 585 lines, 0 invalid, 0 duplicate ids,
  config still valid JSON. The conversation forked. 37 of the 119 turns written were not on the
  chain reachable from the file's last entry, which is what a later `--resume` replays. So two
  writers lose turns without any error.
- **A `ReadWriteOncePod` claim** binds on this CSI driver. A second pod on it, same node, stayed
  Pending (`node has pod using PersistentVolumeClaim with the same name and ReadWriteOncePod access
  mode`) while the first ran. After a forced delete of the first it was Ready 6 s later, 4.5 s
  after the old heartbeat stopped. It would give the one-writer rule from the scheduler, at the
  per-PVC start times above. It does not close a forced delete, since that removes the pod
  object the scheduler looks at.

### 3. Does Flannel enforce NetworkPolicy (FR-011)

**Not enforced.** The cluster runs flannel v0.27.0 and kube-proxy and no policy engine.
A `busybox:1.36` server on `lm-amd64-1` answered `wget` from a client on the same node and from a
client on `node4`, by pod IP. Then a default-deny ingress policy went on the server's pod, and the
probe was repeated after 35 s and 65 s. Then a default-deny egress policy went on both clients,
and it was repeated again.

```
                            client on the server's node          client on node4
before any policy           NP-PROBE-OK rc=0                     NP-PROBE-OK rc=0
ingress deny, after 35 s    NP-PROBE-OK rc=0                     NP-PROBE-OK rc=0
ingress deny, after 65 s    NP-PROBE-OK rc=0                     NP-PROBE-OK rc=0
egress deny added, +30 s    NP-PROBE-OK rc=0                     NP-PROBE-OK rc=0
control, closed port 9999   wget: can't connect to remote host (10.64.2.153): Connection refused rc=1
```

`NP-PROBE-OK` is the server's page. The control shows the probe reports a failure when there is
one, and the policies that should have stopped it did not. On this CNI NetworkPolicy cannot
isolate session pods from each other or from the rest of the cluster.

### What this leaves unmeasured

- `claude` against a real model under two writers, and the terminal UI's behaviour with
  `--remote-control` in these pods. D8a covers Remote Control on the keeper login.
- Subpath without `fsGroup`, and `linstor-replicated` itself.
- Held exec streams across an API-server restart or a kubelet restart, and orphaned loops.
- Disk-full on the shared claim: one session filling it and what the others see.
- The idle memory of a second Deployment for FR-009.

---

## D9 — The spec found a defect in its own state design

`persistence.existingClaim: lawnmower-home` in the `crswd-next` chart (FR-016 as first written,
now FR-019) names a claim the lawnmower chart owns in namespace `lawnmower`. A pod mounts only
claims in its own namespace, so that setting cannot work while the two are in different
namespaces. D7's measurement put the second pod in the same namespace as the claim and did not
test this. It is not specific to B, and under B it also decides which node every session pod
runs on. FR-019 lists the options and leaves the choice open.
