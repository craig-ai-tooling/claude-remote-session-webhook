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
