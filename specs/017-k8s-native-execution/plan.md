# Implementation Plan: Kubernetes-Native Execution (crswd v2)

**Branch**: `spec/017-k8s-native-execution` | **Date**: 2026-09-25 | **Spec**: [spec.md](spec.md)

This plan is written for option A, the recommendation. If the operator chooses B, this
plan is replaced, not amended. No code is written until he decides.

## Summary

v2 adds a `pod` execution mode to the same binary. Under it, crswd runs in one
StatefulSet pod beside a `tmux-host` container that owns every pane, with one PVC for
`CLAUDE_CONFIG_DIR`, the session journal and the working directories. A daemon container
restart is handled by `Adopt`, as on the VM. A pod restart is handled by `ReplayJournal`
and `--resume`, which v0 already ships (spec 012) and which the kill test showed is the
only path that can work. The `host` mode, v0 today, stays the default.

## Technical Context

**Language**: Go, standard library. **New**: a Helm chart under `deploy/chart/`, a
configuration key, one journal field. **Storage**: one PVC, `linstor-replicated`
(`Retain`), or an existing claim. **Target**: rpi-inference, node lm-amd64-1, namespace
`crswd-next`, hostname `crswd-next.craigcloud.io` behind Cloudflare Access.
**Testing**: table-driven unit tests for the mode switch and the journal field. The
`tmux` suite, unchanged, covers `tmuxctl` in both modes because it is the same code.
A kill test in `crswd-next` repeats D1 with the real daemon and a keeper login before
k8s-15.

## Constitution Check

| Principle | Assessment | Pass |
|---|---|---|
| **I — Security** | FR-021 unchanged: no token or hash persisted, every restart re-mints. The daemon never holds a refresh token in pod mode. Its own HMAC secret and Access app. | ✅ |
| **II — Unknowns surfaced** | A versus B is the operator's; FR-010 and FR-014 are marked `NEEDS CLARIFICATION`; research D6 lists what was not measured. | ✅ |
| **III — Verifiable** | SC-001 is a kill test with times; SC-002 and SC-003 are tests. | ✅ |
| **IV — Smallest change** | A reuses `session` and `tmuxctl` whole. The only v0 behaviour change is the journal name (FR-012), which fixes a measured failure. | ✅ |
| **V — Standards** | CI runs the same Install, Lint, Typecheck, Test and Build; the chart adds `helm lint` and `helm template` to CI. | ✅ |
| **VI — Blast radius** | Pod mode turns off the self-updater and the login relay. A session in a pod runs as uid 10001 with no host mounts; the pod is the boundary the systemd hardening is on the VM. | ✅ |
| **VII — Design system** | No UI change. The dashboard names the mode in the existing settings page. | ✅ |

## Design

**Two containers, one socket.** `tmux-host` runs the Claude Code image, starts the tmux
server on `/run/crswd/tmux.sock` (emptyDir), and waits. `crswd` runs the daemon with
`tmuxctl` pointed at that socket. Panes are children of the tmux server, so they live in
`tmux-host`, and a `crswd` container restart leaves them running (7.3 s, measured).

**Startup.** Unchanged: `ReplayJournal`, `Adopt`, supervisor. The journal and
`CLAUDE_CONFIG_DIR` are on the PVC, so a pod restart reaches `ReplayJournal` with the
conversation identifiers intact.

**Journal.** `journalRecord` gains `name`. `createRecord` writes it; `ReplayJournal`
restores it into `Session.Name`. Old records without it replay as today.

**Credentials.** The keeper contract: Secret `claude-credentials` in `crswd-next` mounted
as a directory at `/var/run/claude-credentials`, `CLAUDE_CONFIG_DIR/.credentials.json` a
symlink to it, `creds-link` as a native sidecar. In pod mode `internal/loginrelay` is off,
and `internal/claudeauth` reports a session stuck on sign-in as `needs-auth` as it does
today, pointing at the keeper instead of the relay.

**Upgrades.** The chart and the image come from k8s-20 (ghcr, Flux `HelmRelease` pinned to
2.x). The self-updater is off in pod mode. Whether an upgrade rolls the pod or patches the
`crswd` container in place is FR-010.

**Lawnmower state.** `persistence.existingClaim: lawnmower-home`. The lawnmower chart owns
that claim and pins the console and the group-A CronJobs to the same node.

## Project Structure

```text
internal/config/            MOD  execution.mode (host|pod), socket path
internal/session/journal.go MOD  journalRecord.Name
internal/session/manager.go MOD  createRecord and ReplayJournal carry Name
internal/updater/           MOD  refuses to run in pod mode
internal/loginrelay/        MOD  refuses to run in pod mode
cmd/crswd/                  MOD  unit subcommands refuse in pod mode
deploy/chart/               NEW  StatefulSet, Service, PVC or existingClaim, keeper consumer wiring
.github/workflows/ci.yml    MOD  helm lint, helm template
docs/k8s-mode.md            NEW  operating the pod mode
```

## Sequence

1. FR-012 alone, to v0: the journal name. It fixes the 9/22/26 failure on the VM too.
2. The mode switch and its refusals, behind a default of `host`.
3. The chart, installed in `crswd-next` with its own keeper login and HMAC secret.
4. The kill test again, with crswd and a real `claude --resume`, including FR-010.
5. k8s-15: cutover in a weekend window. v0 on the VM stays as the rollback.
