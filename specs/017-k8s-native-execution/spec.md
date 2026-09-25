# Feature Specification: Kubernetes-Native Execution (crswd v2)

**Feature Branch**: `spec/017-k8s-native-execution`
**Created**: 2026-09-25
**Status**: Draft. The execution shape (A or B below) is the operator's decision and is
`NEEDS CLARIFICATION` until he makes it. This spec recommends A.
**Input**: Operator, 9/14/26: "a new version that's a rewrite, that is Kubernetes-centric,
that we can work on and test and iterate on while CRSWD continues to operate in production
on the VM... I want to make sure that we're not disrupting my work week." Operator, 9/22/26:
"I want crswd to still be [a daemon] for others that want to use that but also k8s native."
Backlog item k8s-19. Evidence: [research.md](research.md), and ai-lawnmower
`docs/crswd-v2-kill-test.md`, `docs/k8s-12-session-capacity-scout.md`,
`docs/crswd-k8s-scout.md`, `docs/claude-login-keeper.md`.

## The decision this spec waits on

| | A. Session host | B. Operator |
|---|---|---|
| Shape | One StatefulSet pod. tmux and every session in it, one PVC | `ClaudeSession` CRD, a controller, one pod and one PVC per session |
| New code | a pod mode for config, a Helm chart, one journal field | CRD types, controller, a per-pod agent that replaces `tmuxctl`, and all of that beside the tmux path the daemon mode keeps |
| Session start | same as today, 3.1 s (k8s-12) | 24 s for one PVC alone. 93 to 98 s each when three start together (measured) |
| Daemon upgrade | every session restarts and resumes from the PVC (8 to 44 s measured), unless only the daemon container restarts (7.3 s, sessions untouched, measured) | sessions untouched |
| Node loss | every session, resumed on the same PVC | the sessions on that node |
| `kubectl get` shows what runs where | one pod; sessions via the dashboard, as today | yes |
| Two modes, one execution layer | yes: both run `tmuxctl` | no: tmux on a host, pods in a cluster |

**Recommendation: A.** The one thing B buys, sessions untouched by a control-plane upgrade,
A gets from a daemon container restart, which the kill test measured at 7.3 s with zero
session gap. B pays for it with a second execution layer, per-session storage that
provisions in 24 to 98 s, and a per-session PVC that the lawnmower state cannot share.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A pod restart costs seconds, not sessions (Priority: P1)

The operator's sessions keep their conversations when the pod hosting them is deleted,
evicted or rescheduled.

**Independent Test**: Create two sessions, delete the pod, and confirm both come back
running `claude --resume <their own conversation>` in their own working directories.

**Acceptance Scenarios**:

1. **Given** a running session, **When** the daemon container alone restarts, **Then**
   the session's process is the same process and `Adopt` takes it back, as on a VM daemon
   restart.
2. **Given** a running session, **When** the pod is deleted, **Then** the replacement pod
   revives it from the journal with `--resume` and its conversation identifier.
3. **Given** a named session, **When** it is revived from the journal, **Then** its start
   command renders. (On 9/22/26 it did not, for 8 of 8 sessions; research D3.)
4. **Given** any revival, **When** the first `GET /sessions` answers, **Then** it carries
   a fresh token for that session and the pre-restart token is dead.

### User Story 2 - The daemon on a plain host stays first class (Priority: P1)

Anyone can still install crswd on a host with systemd and tmux, with no cluster.

**Independent Test**: The existing `quickstart` and `tmux` suites pass unchanged with the
mode unset.

**Acceptance Scenarios**:

1. **Given** no mode in the configuration, **When** the daemon starts, **Then** it behaves
   exactly as v0: systemd unit, self-updater, local `/login`, phone sign-in relay.
2. **Given** the pod mode, **When** the daemon starts, **Then** the self-updater and the
   sign-in relay are off and say so, and the Helm chart is the only way to upgrade.

### User Story 3 - The v2 track never touches the work week (Priority: P1)

v2 is built, tested and iterated on in the cluster while v0 serves every real session on
the VM until k8s-15.

**Acceptance Scenarios**:

1. **Given** v2 running, **When** anything happens to it, **Then** nothing on the VM
   changes: its own namespace `crswd-next`, hostname `crswd-next.craigcloud.io` behind
   Cloudflare Access, HMAC secret and Claude login.
2. **Given** a v2 release, **When** the VM's updater runs, **Then** it does not install it
   (a major is never crossed, k8s-18, already shipped in `internal/updater/fetch.go`).

### User Story 4 - The lawnmower state has a live home in the cluster (Priority: P2)

The console and the group-A timers read the same files the sessions write, as they do on
the VM today.

**Independent Test**: A pod pinned to the session host's node mounts the state volume,
appends a line, and the session host reads it.

### Edge Cases

- A forced delete: the old containers may still run for a moment while the new pod mounts
  the volume on the same node. The journal must tolerate a second writer's last line.
- A revival whose working directory the allowlist no longer admits: refused, as v0 does.
- The keeper is down past the access token's expiry: sessions answer 401 and recover when
  the keeper writes the Secret again (`docs/claude-login-keeper.md`, measured 9/22/26).
- A session with an OAuth MCP server: the credential re-link sidecar drops its token and it
  re-authenticates. Headless sessions must not use one.

## Requirements *(mandatory)*

### Modes

- **FR-001**: One binary, two modes, chosen by a configuration key (`execution.mode`,
  `host` or `pod`). Absent means `host`, which is v0's behaviour byte for byte.
- **FR-002**: Both modes MUST drive sessions through `internal/tmuxctl`. Under A the pod
  mode adds a socket path and nothing else to it.
- **FR-003**: `pod` mode MUST disable `internal/updater` and `internal/loginrelay` and
  refuse the `unit` subcommands, each with a message naming the mode.
- **FR-004**: v0 code paths and `release.yml` MUST NOT change for v2 beyond FR-001's
  default and FR-012's journal field. v2 ships as major 2 through k8s-20's image and chart.

### Execution (option A)

- **FR-005**: A StatefulSet of one replica, pinned to `lm-amd64-1` by values, with two
  containers sharing the tmux socket through an emptyDir: `tmux-host` (the Claude Code
  image, owns the tmux server and every pane) and `crswd` (the daemon).
- **FR-006**: One PVC, `persistentVolumeClaimRetentionPolicy` `Retain` on both, a
  StorageClass that reclaims `Retain` (`linstor-replicated`). It holds
  `CLAUDE_CONFIG_DIR`, the journal directory, and the working-directory roots.
- **FR-007**: The daemon's startup order is unchanged: `ReplayJournal`, then `Adopt`, then
  the supervisor. A daemon container restart reaches `Adopt`. A pod restart reaches
  `ReplayJournal`.
- **FR-008**: Per-session CPU is not requested. The pod requests the idle total
  (~50m per expected session, k8s-12) and sets no CPU limit.
- **FR-009**: `terminationGracePeriodSeconds` covers one journal fsync and a tmux
  `kill-server`, and the daemon handles SIGTERM. (The stand-in that did not paid the full
  30 s.)
- **FR-010**: `NEEDS CLARIFICATION`: whether a daemon upgrade patches only the `crswd`
  container's image in place (`updateStrategy: OnDelete`) so sessions survive it, or
  rolls the pod and relies on FR-007. Not measured. Research D6.

### Tokens across a restart (FR-021 of spec 001)

- **FR-011**: FR-021 is unchanged in both modes. No token and no token hash is persisted.
  Every adopted or revived session is `CredentialPending`, and the first `GET /sessions`
  mints and hands over its new token (`ClaimPending`). The lawnmower keyring already
  captures it (`loop/session_keyring.py`).
- **FR-012**: The journal record MUST carry the session's name, and a revival MUST render
  the start command with it. This fixes v0 as well: revival failed 8 of 8 sessions after
  the VM reboot of 9/22/26.

### Credentials

- **FR-013**: `pod` mode consumes the login keeper's session contract
  (ai-lawnmower `docs/claude-login-keeper.md`, "Sessions: how to consume it"): the
  access-token-only Secret mounted as a directory (never `subPath`), `.credentials.json` in
  `CLAUDE_CONFIG_DIR` a symlink to it, and the `creds-link` native sidecar re-linking every
  60 s. The daemon never holds a refresh token.
- **FR-014**: The `crswd-next` namespace gets its own keeper login, never a copy of the
  VM's `~/.claude` and never the `lawnmower` namespace's Secret. `NEEDS CLARIFICATION`:
  whether the lawnmower chart's keeper installs into a second namespace as is.
- **FR-015**: `host` mode keeps its local login and the phone sign-in relay unchanged.

### Lawnmower state

- **FR-016**: The chart accepts an existing claim for the session host's volume
  (`persistence.existingClaim`). The lawnmower chart owns that claim, `lawnmower-home`,
  and pins the console and the group-A CronJobs to the same node so they mount it too.
  RWO admits every pod on one node (measured).
- **FR-017**: `lawnmower-home` holds `~/.local/share/lawnmower` (backlog.md, ledger.jsonl,
  outcomes.jsonl, flight-watch.json, metrics.db, session-tokens.jsonl) and the `~/code`
  checkouts. The hourly offsite copy continues and still excludes session-tokens.jsonl.

**Key Entities**: **Journal record**: v0's `journalRecord` plus `name`. **Session host**:
the pod, its two containers, the socket, the volume.

## Success Criteria *(mandatory)*

- **SC-001**: A graceful pod delete brings every session back on its own conversation in
  under 60 s. A daemon container restart leaves every pane pid unchanged.
- **SC-002**: A test fails if a journal record written at create lacks the name, and a
  replay test fails if a named session's start command does not render.
- **SC-003**: With the mode unset, `go test -tags quickstart ./cmd/crswd` and
  `go test -tags tmux ./...` pass with no test edited.
- **SC-004**: No test fixture or manifest in v2 names the `lawnmower` namespace's Secrets
  or the VM's `~/.claude`.

**Assumptions**: lm-amd64-1 (8 cores, 12 GB) is the session host, as k8s-12 and k8s-13
chose; at ~570 MiB a session it holds ~15. One node is a single point of failure, the same
one the VM is today. The tool credentials (`sf`, `gws`, `op`) stay behind the tool gateway
and are out of scope here.
