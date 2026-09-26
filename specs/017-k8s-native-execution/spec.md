# Feature Specification: Kubernetes-Native Execution (crswd v2)

**Feature Branch**: `spec/017-k8s-native-execution`
**Created**: 2026-09-25
**Status**: Decided 9/25/26: **option B, the operator**. The operator chose it (ai-lawnmower
inbox 01M3DCKTP2X3H5V99FMVGWXTXA) over this spec's recommendation of A. The recommendation and
its numbers stay below as the record of what B costs. The requirements now describe B, and
[plan.md](plan.md) was replaced for it. Nothing is built.
**Input**: Operator, 9/14/26: "a new version that's a rewrite, that is Kubernetes-centric,
that we can work on and test and iterate on while CRSWD continues to operate in production
on the VM... I want to make sure that we're not disrupting my work week." Operator, 9/22/26:
"I want crswd to still be [a daemon] for others that want to use that but also k8s native."
Backlog item k8s-19, then k8s-20. Evidence: [research.md](research.md), and ai-lawnmower
`docs/crswd-v2-kill-test.md`, `docs/k8s-12-session-capacity-scout.md`,
`docs/crswd-k8s-scout.md`, `docs/claude-login-keeper.md`.

## The decision, and what it costs

| | A. Session host (not chosen) | B. Operator (chosen) |
|---|---|---|
| Shape | One StatefulSet pod. tmux and every session in it, one PVC | `ClaudeSession` CRD, a controller, one pod and one PVC per session |
| New code | a pod mode for config, a Helm chart, one journal field | CRD types, controller, a per-pod agent that replaces `tmuxctl`, and all of that beside the tmux path the daemon mode keeps |
| Session start | same as today, 3.1 s (k8s-12) | 24 s for one PVC alone. 93 to 98 s each when three start together (measured) |
| Daemon upgrade | every session restarts and resumes from the PVC (8 to 44 s measured), unless only the daemon container restarts (7.3 s, sessions untouched, measured) | sessions untouched |
| Node loss | every session, resumed on the same PVC | the sessions on that node |
| `kubectl get` shows what runs where | one pod; sessions via the dashboard, as today | yes |
| Two modes, one execution layer | yes: both run `tmuxctl` | no: tmux on a host, pods in a cluster |

The spec recommended A because the one thing B buys, sessions untouched by a control-plane
upgrade, A gets from a daemon container restart, and B pays for it with a second execution
layer and per-session storage that provisions in 24 to 98 s. The operator chose B anyway. This
spec now owns those costs. Each one below names the requirement that manages it:

- **Session start of 24 to 98 s**: FR-010 (storage), measured before it is built.
- **A second execution layer**: FR-002 keeps it behind the one interface that already exists,
  `tmuxctl.Controller`, so `internal/session` and `internal/httpapi` do not fork.
- **A second way to create a session**: anyone allowed to create the object can bypass the
  daemon's checks. FR-007 makes the reconciler enforce them.
- **A per-pod credential path, not one**: FR-015 to FR-017, and the credential gate.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A control-plane restart costs no sessions (Priority: P1)

The operator's sessions keep running when crswd itself restarts or upgrades, and keep their
conversations when one session's pod is deleted, evicted or rescheduled.

**Independent Test**: Create two sessions. Delete the crswd pod and confirm both session pods
keep their start time and container IDs. Then delete one session pod and confirm it comes back
running `claude --resume <its own conversation>` in its own working directory.

**Acceptance Scenarios**:

1. **Given** running sessions, **When** the crswd pod restarts, **Then** the session pods are
   untouched and crswd lists them again from their `ClaudeSession` objects.
2. **Given** a running session, **When** its pod is deleted, **Then** the reconciler recreates
   the pod, which revives the session with `--resume` and its conversation identifier.
3. **Given** a named session, **When** it is revived, **Then** its start command renders. (On
   9/22/26 it did not, for 8 of 8 sessions; research D3.)
4. **Given** any restart of crswd or of a session pod, **When** the first `GET /sessions`
   answers, **Then** it carries a fresh token for that session and the pre-restart token is
   dead.

### User Story 2 - The daemon on a plain host stays first class (Priority: P1)

Anyone can still install crswd on a host with systemd and tmux, with no cluster.

**Independent Test**: The existing `quickstart` and `tmux` suites pass unchanged with the
mode unset.

**Acceptance Scenarios**:

1. **Given** no mode in the configuration, **When** the daemon starts, **Then** it behaves
   exactly as v0: systemd unit, self-updater, local `/login`, phone sign-in relay.
2. **Given** the `kubernetes` mode, **When** the daemon starts, **Then** the self-updater and
   the sign-in relay are off and say so, and the Helm chart is the only way to upgrade.

### User Story 3 - The v2 track never touches the work week (Priority: P1)

v2 is built, tested and iterated on in the cluster while v0 serves every real session on
the VM until k8s-15.

**Acceptance Scenarios**:

1. **Given** v2 running, **When** anything happens to it, **Then** nothing on the VM
   changes: its own namespace `crswd-next`, hostname `crswd-next.craigcloud.io` behind
   Cloudflare Access, HMAC secret and Claude login.
2. **Given** a v2 release, **When** the VM's updater runs, **Then** it does not install it
   (a major is never crossed, k8s-18, already shipped in `internal/updater/fetch.go`).
3. **Given** the `ClaudeSession` CRD is installed, **When** v0 runs on the VM, **Then** v0 does
   not see it and nothing on the VM depends on it. The CRD is the one cluster-scoped object v2
   adds, so it is installed by the chart's owner and never by a session.

### User Story 4 - A session cannot be created around the daemon's checks (Priority: P1)

A `ClaudeSession` object written directly with `kubectl` or by another controller is held to
the same working-directory allowlist, concurrency cap and lifetime as one made through the API.

**Independent Test**: Create an object with a working directory outside the allowlist. No pod
appears and the object's status says why.

### User Story 5 - The lawnmower state has a live home in the cluster (Priority: P2)

The console and the group-A timers read the same files the sessions write, as they do on
the VM today. `NEEDS CLARIFICATION`: see FR-019. B changes the answer A gave.

### Edge Cases

- A forced delete of a session pod: the old containers may still run for a moment while the
  new pod mounts the volume on the same node. Two writers to one conversation must not corrupt
  it, which the per-session volume allows only if the mount is exclusive; FR-010 decides it.
- A revival whose working directory the allowlist no longer admits: refused, as v0 does.
- The keeper is down past the access token's expiry: sessions answer 401 and recover when
  the keeper writes the Secret again (`docs/claude-login-keeper.md`, measured 9/22/26).
- A session with an OAuth MCP server: the credential re-link sidecar drops its token and it
  re-authenticates. Headless sessions must not use one.
- The reconciler is down: existing pods keep running, new objects wait, and nothing is lost
  because the object is the record.

## Requirements *(mandatory)*

### Modes

- **FR-001**: One binary, two modes, chosen by a configuration key (`execution.mode`,
  `host` or `kubernetes`). Absent means `host`, which is v0's behaviour byte for byte.
- **FR-002**: Both modes MUST drive sessions through `tmuxctl.Controller`. `host` keeps
  `tmuxctl.Exec`. `kubernetes` supplies a second implementation of the same interface, backed
  by `ClaudeSession` objects and session pods, so `internal/session` and `internal/httpapi` do
  not fork. The table above priced a per-pod agent that replaces `tmuxctl`, and research D8b
  removes it: tmux runs inside each session pod, and `Paste` and `SendKeys` keep `tmuxctl`'s argv
  builders, so no byte that reaches a pane is built by new code. `CapturePane` runs the same
  capture argv inside a read loop in the pod (FR-011), which is new code that only reads. The
  second implementation sends all of it through the Kubernetes exec API.
- **FR-003**: `kubernetes` mode MUST disable `internal/updater` and `internal/loginrelay` and
  refuse the `unit` subcommands, each with a message naming the mode.
- **FR-004**: v0 code paths and `release.yml` MUST NOT change for v2 beyond FR-001's default
  and FR-012's journal field. v2 ships as major 2 through k8s-20's image and chart.

### Execution (option B)

- **FR-005**: A namespaced `ClaudeSession` custom resource is the record of one session. Its
  spec carries the session name, owner, working directory, start options and lifetime. Its
  status carries the phase and the conversation identifier. It MUST NOT carry a token or a
  token hash (FR-014). The API group is `NEEDS CLARIFICATION`: crswd is meant for other
  operators too, so it should not be a domain only this operator owns.
- **FR-006**: The reconciler owns exactly one pod per `ClaudeSession`, through an owner
  reference. A deleted pod is recreated and revives its session with `--resume`. A deleted
  object deletes its pod, and teardown is confirmed by observing the pod gone, not assumed.
  Only one reconciler runs at a time, by a Lease.
- **FR-007**: The reconciler enforces spec 001's containment on every object, whoever created
  it: the working-directory allowlist, the concurrency cap, and the absolute lifetime. An object
  that fails a check gets a `Rejected` phase and a reason, and no pod. The lifetime is also set
  on the pod (`activeDeadlineSeconds`) so it holds if the reconciler is down.
- **FR-008**: Session pods request the idle CPU (~50m per session, k8s-12) and the k8s-12
  memory (~570 MiB), and set no CPU limit. The default node is `lm-amd64-1`, by values.
- **FR-009**: The reconciler runs as its own Deployment: the same image started as
  `crswd reconcile`, with its own ServiceAccount, in a namespace where the daemon has no
  `pods/exec`. Its pod permissions are a Role and RoleBinding in the session namespace, so the
  crswd daemon holds no pod-create permission. Decided 9/26/26 (research D8b). The argument is
  RBAC and has no measurement of its own. Creating pods is the wide permission: a process that can
  create them in the session namespace can run any spec there, privileged, `hostPath` or any
  Secret in the namespace. The daemon's `pods/exec` (FR-011) reaches only pods that already run
  sessions, which the daemon already drives. The daemon is the internet-facing process, so it
  gets the narrow one. The reconciler has to sit where the daemon cannot exec into it, because
  exec into its pod would hand over its ServiceAccount token and with it pod creation. This does
  not wait on FR-019, since the reconciler's namespace is independent of where sessions run. A
  restart of either process leaves the other running. The one number is the idle cost of a second
  process: the v0 daemon, the same binary, holds 15.5 MiB resident (16.2 MiB peak) on the VM. A
  reconciler with client-go informers will hold more, and none was built or measured.
- **FR-010**: Per-session storage is one shared `ReadWriteOnce` claim with a `subPath` per
  session, on a `Retain` class (`linstor-replicated`), and every session pod runs on the claim's
  node (FR-008, FR-019). Decided 9/26/26 (research D8b), measured on `linstor-fs-storage-enc`,
  which has the same layers. A session added while others run is Ready in 5.2 to 6.7 s, and six
  started together on a bound claim in 30.7 and 33.5 s. One PVC per session took 58.2 s alone,
  82.2 s for three (D4: 93 to 98 s) and 153.3 s for six, with CSI `DeadlineExceeded` errors, and
  an unreplicated class took 118.1 s for six, so the wait is the provisioner.
  `CLAUDE_CONFIG_DIR/projects` and the working directory MUST live on the session's `subPath`
  and survive a pod deletion, or `--resume` has nothing to resume, and pods MUST set `fsGroup` so
  the runner's uid can write it (measured with 10001). Three forced deletes each resumed the same
  conversation with `claude --resume`, with no overlap between the old and new pod (gaps of 3.7 to
  5.3 s). Two live writers on one `subPath` corrupted no file but forked the conversation and left
  37 of 119 turns off what `--resume` replays, so the reconciler MUST NOT start a replacement pod
  until the old pod object is gone and MUST NOT force-delete a session pod. The cost accepted is
  that one claim is one disk: a full disk stops every session, so the claim is sized for their
  sum. The retain policy MUST be `Retain`.
- **FR-011**: The daemon reaches a session's pane through the Kubernetes exec API, with one
  held exec stream per watched session, and there is no in-pod agent. Decided 9/26/26 (research
  D8b). The stream runs the capture argv in a one-second loop inside the pod, framed by a
  separator byte, and `CapturePane` returns its newest frame; that loop is new code and only
  reads. The stream handler takes one `CapturePane` per watched session per second
  (`streamInterval`, `internal/httpapi/stream.go`). At ten sessions read once a second, one exec
  per read costs the three API servers about 160 millicores more (about 16 m per call a second)
  and `lm-amd64-1` about 250 m, at a 53 ms median, 275 ms worst and no API error in 4500 calls.
  One held stream per session left the API servers no higher than the no-load windows (327 m
  against 354 to 461 m) and cost `lm-amd64-1` 20 to 30 m, from 10 exec opens for 1500 frames. An in-pod agent
  answers in 2.5 ms and costs the API servers nothing, but Flannel does not enforce NetworkPolicy
  (measured: an ingress deny left the server reachable from both nodes), so an agent's port would
  be open to every pod in the cluster and would need its own authentication. The exec path is
  gated by RBAC instead. `pods/exec` goes to the daemon's ServiceAccount, in the session
  namespace only (FR-013).
- **FR-012**: The record of a session MUST carry its name, and a revival MUST render the start
  command with it. This fixes v0 as well: revival failed 8 of 8 sessions after the VM reboot of
  9/22/26. Under B the name is in the `ClaudeSession` spec; v0's journal record gains a `name`
  field.
- **FR-013**: Pod-creating permission is bounded and stays out of the daemon (FR-009). The
  reconciler's ServiceAccount may create, list, watch and delete pods, and read and update
  `ClaudeSession` objects, in the session namespace and nothing else. It needs no verb on PVCs,
  because the claim belongs to the chart (FR-010). The daemon's ServiceAccount may manage
  `ClaudeSession` objects, read pods and create `pods/exec` in the session namespace only, and MUST
  NOT create pods or hold `pods/exec` in the reconciler's namespace. Neither has any verb on
  `secrets`.

### Tokens across a restart (FR-021 of spec 001)

- **FR-014**: FR-021 is unchanged in both modes. No token and no token hash is persisted, in the
  journal or in a `ClaudeSession`. A restart of crswd or of a session pod marks every affected
  session `CredentialPending`, and the first `GET /sessions` mints and hands over its new token
  (`ClaimPending`). The lawnmower keyring already captures it (`loop/session_keyring.py`).

### Credentials

- **FR-015**: `kubernetes` mode consumes the login keeper's session contract (ai-lawnmower
  `docs/claude-login-keeper.md`, "Sessions: how to consume it") in every session pod: the
  access-token-only Secret mounted as a directory (never `subPath`), `.credentials.json` in
  `CLAUDE_CONFIG_DIR` a symlink to it, and the `creds-link` native sidecar re-linking every
  60 s. No pod holds a refresh token. The reconciler names the Secret in the pod spec and never
  reads it (FR-013).
- **FR-016**: The `crswd-next` namespace gets its own keeper login, never a copy of the VM's
  `~/.claude` and never the `lawnmower` namespace's Secret. `NEEDS CLARIFICATION`: whether the
  lawnmower chart's keeper installs into a second namespace as is. Creating that login needs the
  operator's browser once (`loop/claude-keeper-login.sh`).
- **FR-017**: **The credential gate.** Before any B code is written, one measurement decides
  whether B can start at all: does an interactive session run `claude --remote-control` and
  `claude --resume` on the keeper's access-token-only credentials, in a pod, after a pod restart?
  The keeper contract was measured for inference only (k8s-14). This is D6's first unmeasured
  item, and every session pod under B depends on it. If it fails, session pods need a
  refreshable login, which is a different design and the operator's decision.
  **Result (9/26/26): PASS**, on claude 2.1.246 in the pod; the evidence, and what it did not
  cover (the VM's 2.1.283), is research.md D8a.
  **Rotation (9/26/26): a session survives one.** A live `--remote-control` session stayed
  `/rc active` on the same URL across the keeper's 06:30:17Z rotation with no restart, answered a prompt
  after it, and answered again after the first token's own expiry at 08:30:11Z, on the rotated
  file the kubelet delivered in 65 s. `creds-link` logged nothing. A pod delete and `--resume`
  on the rotated credential reconnected on the same URL. Not shown: a prompt from claude.ai
  after the rotation, or a rotation during a turn (research.md D8c).
- **FR-018**: `host` mode keeps its local login and the phone sign-in relay unchanged.

### Lawnmower state

- **FR-019**: `NEEDS CLARIFICATION`: how the sessions and the lawnmower state meet. As written for
  A, the chart took `persistence.existingClaim: lawnmower-home` in `crswd-next`, and that cannot
  work: a pod mounts only claims in its own namespace, and the lawnmower chart owns that claim in
  `lawnmower` (found while amending this spec, and true for A as well). Under B it also fixes
  where session pods run: an RWO claim is mounted by pods on one node only (D7, measured: a pod on
  node4 stayed in `ContainerCreating` on a Multi-Attach error), so session pods that share
  `lawnmower-home` all run on `lm-amd64-1`. Options: sessions live in the namespace that owns the
  claim after k8s-15, with `crswd-next` a scratch track until then; or sessions get no direct
  access to the state and reach it through an interface.
- **FR-020**: `lawnmower-home` holds `~/.local/share/lawnmower` (backlog.md, ledger.jsonl,
  outcomes.jsonl, flight-watch.json, metrics.db, session-tokens.jsonl) and the `~/code`
  checkouts. The hourly offsite copy continues and still excludes session-tokens.jsonl.

**Key Entities**: **ClaudeSession**: the record of one session, in the cluster's API.
**Session pod**: the pod the reconciler makes for one, running tmux and `claude`. **Reconciler**:
the loop that keeps pods matching objects. **Journal record**: v0's `journalRecord` plus `name`.

## Success Criteria *(mandatory)*

- **SC-001**: Restarting the crswd pod leaves every session pod's start time and container ID
  unchanged. Deleting one session pod brings that session back on its own conversation in under
  60 s, with the storage choice of FR-010 measured against it.
- **SC-002**: A test fails if a session record written at create lacks the name, and a replay
  test fails if a named session's start command does not render.
- **SC-003**: With the mode unset, `go test -tags quickstart ./cmd/crswd` and
  `go test -tags tmux ./...` pass with no test edited.
- **SC-004**: No test fixture or manifest in v2 names the `lawnmower` namespace's Secrets
  or the VM's `~/.claude`.
- **SC-005**: A test fails if the reconciler's Role grants any verb on `secrets`, a test fails
  if the daemon's Role can create pods, has any verb on `secrets` or grants `pods/exec` in the
  reconciler's namespace, and a test
  fails if an object with a working directory outside the allowlist, or past the cap, produces
  a pod.
- **SC-006**: The credential gate (FR-017) has a recorded result, with the versions, before the
  reconciler is written.

**Assumptions**: lm-amd64-1 (8 cores, 12 GB) is the session host, as k8s-12 and k8s-13 chose; at
~570 MiB a session it holds ~15. One node is a single point of failure, the same one the VM is
today, so B's node-loss row in the table does not apply until a second node can host sessions.
The tool credentials (`sf`, `gws`, `op`) stay behind the tool gateway and are out of scope here.
