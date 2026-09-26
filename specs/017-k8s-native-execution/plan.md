# Implementation Plan: Kubernetes-Native Execution (crswd v2)

**Branch**: `spec/017-k8s-native-execution` | **Date**: 2026-09-25 | **Spec**: [spec.md](spec.md)

This plan was written for option A and replaced on 9/25/26 when the operator chose B, as the
first version of it said it would be. The A design is in git history (`a2f8037`). No code is
written until the credential gate (FR-017) has a result.

## Summary

v2 adds a `kubernetes` execution mode to the same binary. Under it, crswd is the front door
(the authenticated dashboard and API) and a reconciler. A session is a `ClaudeSession` object;
the reconciler keeps one pod per object, and that pod runs `claude` under tmux with the
`creds-link` sidecar. Deleting a pod costs one session a `--resume`. Restarting or upgrading
crswd costs no session anything. The `host` mode, v0 today, stays the default and stays the
only mode that runs `tmuxctl.Exec` on the daemon's own host.

## Technical Context

**Language**: Go, standard library plus `client-go` for the API. **New**: CRD types, a
reconciler, a second `tmuxctl.Controller`, a session-pod image, a Helm chart with the CRD, a
configuration key, one journal field. **Storage**: FR-010, one shared claim with a `subPath` per session (research D8b). **Target**: rpi-inference,
node lm-amd64-1, namespace `crswd-next`, hostname `crswd-next.craigcloud.io` behind Cloudflare
Access. **Testing**: table-driven unit tests for the reconciler against a fake client, the
`tmux` suite unchanged for `tmuxctl`, and a kill test in `crswd-next` before k8s-15.

## Constitution Check

| Principle | Assessment | Pass |
|---|---|---|
| **I. Security** | FR-014: no token or hash persisted, in the journal or in an object. The reconciler has no verb on Secrets (FR-013, SC-005). The daemon keeps its own HMAC secret and Access app. The pane path is the exec API, gated by RBAC, so no agent port needs its own authentication (FR-011). | ✅ |
| **II. Unknowns surfaced** | Three `NEEDS CLARIFICATION` remain in the spec: API group, namespace and state, keeper in a second namespace. Reconciler placement, storage, the pane path and tmux-in-pod are decided (D8b). The credential gate blocks the build. | ✅ |
| **III. Verifiable** | SC-001 is a kill test with times. SC-005 fails on a Secret verb, an unallowlisted directory or a cap breach. SC-006 requires the gate's result on record. | ✅ |
| **IV. Smallest change** | Second implementation of one existing interface, so `session` and `httpapi` do not fork. The only v0 change is the journal name (FR-012). | ✅ |
| **V. Standards** | CI runs Install, Lint, Typecheck, Test and Build, plus `helm lint` and `helm template` with the CRD. | ✅ |
| **VI. Blast radius** | A second creation path exists, so the reconciler re-checks allowlist, cap and lifetime on every object (FR-007, US4), and sets `activeDeadlineSeconds` so the lifetime holds with the reconciler down. One shell per pod is a stronger boundary than one tmux server for all. What becomes reachable: a ServiceAccount that can create pods, bounded to one namespace and held by the reconciler alone, not the daemon (FR-009, FR-013). | ✅ |
| **VII. Design system** | No UI change. The dashboard names the mode in the existing settings page. | ✅ |

## Design

**Object.** `ClaudeSession` carries name, owner, working directory, start options and lifetime
in `spec`, and phase (`Pending`, `Running`, `Rejected`, `Reviving`, `Failed`) and conversation
identifier in `status`. Nothing in it is a secret.

**Reconcile.** List objects, list pods by owner reference, and converge: create a missing pod,
recreate a deleted one with `--resume <conversation>`, delete the pod of a deleted object and
confirm it is gone, reject an object that fails FR-007. One reconciler by Lease, as its own
Deployment (FR-009).

**Session pod.** tmux as the entrypoint's child, `claude` started by the same start command
`tmuxctl` sends today, `creds-link` as a native sidecar, `CLAUDE_CONFIG_DIR` and the working
directory on the session's `subPath` of the shared claim (FR-010), uid 10001, no host mounts. The image is the Claude Code
image k8s-12 measured (~570 MiB a session), plus tmux.

**Pane path.** The second `tmuxctl.Controller` sends what the first would, to the pod through
the exec API, with one held exec stream per watched session (FR-011, D8b). There is no in-pod agent.

**Tokens.** Unchanged from spec 001 FR-021. The daemon mints a hash per session in memory. A
crswd restart or a pod restart makes every affected session `CredentialPending` again.

**Credentials.** The keeper contract, per pod: `claude-credentials` mounted as a directory,
`.credentials.json` a symlink into it, `creds-link` re-linking every 60 s. The reconciler puts
the Secret's name in the pod spec and never reads it. `crswd-next` gets its own keeper login
(FR-016). Whether that login works for `--remote-control` and `--resume` is the gate (FR-017).

**Upgrades.** The chart and images come from k8s-20 (ghcr, Flux `HelmRelease` pinned to 2.x). A
crswd upgrade touches no session pod. A session pod keeps the image in its object until the
session ends, which is proposed and not decided.

**Lawnmower state.** FR-019, undecided. It fixes which node the session pods run on.

## Project Structure

```text
api/v1alpha1/               NEW  ClaudeSession types, CRD manifest generation
internal/config/            MOD  execution.mode (host|kubernetes)
internal/session/journal.go MOD  journalRecord.Name
internal/session/manager.go MOD  createRecord and ReplayJournal carry Name
internal/podctl/            NEW  second tmuxctl.Controller, backed by objects and pods
internal/reconcile/         NEW  the loop, the FR-007 checks, the Lease
internal/updater/           MOD  refuses to run in kubernetes mode
internal/loginrelay/        MOD  refuses to run in kubernetes mode
cmd/crswd/                  MOD  unit subcommands refuse in kubernetes mode; a `reconcile` subcommand
deploy/chart/               NEW  CRD, RBAC (FR-013), the daemon and reconciler Deployments, Service, session pod template
deploy/session-image/       NEW  Claude Code plus tmux
.github/workflows/ci.yml    MOD  helm lint, helm template, CRD generation drift check
docs/k8s-mode.md            NEW  operating the kubernetes mode
```

## Sequence

1. **The credential gate (FR-017)**, alone, with no crswd code: a pod on a keeper login runs
   `claude --remote-control` and `claude --resume` across a pod restart. Record versions.
   Needs the operator's browser once, for the `crswd-next` login (FR-016).
2. FR-012 alone, to v0: the journal name. It fixes the 9/22/26 failure on the VM too.
3. Measured 9/26/26 (research D8b): the pane path, per-session storage with a `subPath` on a
   shared claim, and NetworkPolicy. FR-009, FR-010 and FR-011 are decided from the numbers.
4. The mode switch and its refusals, behind a default of `host`.
5. CRD, reconciler and the second controller, against a fake client, then in `crswd-next`.
6. The chart with the CRD and RBAC, installed in `crswd-next` with its own keeper login and
   HMAC secret. This is k8s-20's delivery half.
7. The kill test with the real daemon: restart crswd, delete a session pod, delete a node's
   worth, with times against SC-001.
8. k8s-15: cutover in a weekend window. v0 on the VM stays as the rollback.
