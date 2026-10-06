# Implementation plan: k8s-20c-s2

S2 of k8s-20c: the AgentSession API, admission, JSON manifests and RBAC tests (FR-005, FR-007
logic, FR-013, SC-004, SC-005). Source: `specs/017-k8s-native-execution/k8s-20c-plan.md`
section 2 "S2", with sections 0, 4, 5 and 7 as context. The definition of done is
`ralph/plans/k8s-20c-s2/VALIDATION_CONTRACT.md`.

Root module, standard library only. `go.sum` must stay absent and `go.mod` gets no `require`
(three existing tests enforce it). Host mode stays byte-identical: no existing test is edited.

## Tasks

Take the topmost open task. One per iteration.

- [x] T1: Create `api/v1alpha1/types.go` and `types_test.go` per Design §1 (constants, AgentSession, Spec, Status, Phase). Tests: reflect allowlist of Spec and Status JSON names, no field name containing "token", group constant. Verify: `go test ./api/...` passes and `go vet ./...` exits 0.
- [x] T2: In `internal/session/workdir.go` export `UnderAnyRoot` (rename, update callers) and add `LexicalWorkDir` per Design §2. New test file `workdir_lexical_test.go`: `..` escape, relative path, `/codeEVIL` boundary, non-existent dir under root admitted. Verify: `go test ./internal/session/` passes.
- [x] T3: Add `Manager.SetWorkDirResolver` per Design §3 and route Create, journal replay and Continue through it. New test `resolver_test.go`: lexical resolver admits a missing dir under root, refuses one outside; no resolver still refuses the missing dir. Verify: `go test -run WorkDirResolver ./internal/session -v`.
- [x] T4: Create `internal/admit/admit.go` with pure `Admit` per Design §4, and a table test covering outside allowlist, `..` escape, relative path, over cap, past lifetime, bad lifetime, admitted, and cap ordering by creationTimestamp. Verify: `go test -v -run Admit ./internal/admit` passes.
- [x] T5: Create package `deploy/k8s/manifest` with the CRD value per Design §5 (`crd.go`) and a test that the CRD's spec and status schema property names equal the `api/v1alpha1` JSON names by reflection. Verify: `go test ./deploy/k8s/manifest` passes.
- [ ] T6: Add the three RBAC Lists to `deploy/k8s/manifest` per Design §6 (`rbac.go`) and the SC-005 walk tests in `rbac_test.go` (secrets or `*` anywhere fails; daemon pods create fails; daemon pods/exec outside the session ns fails; reconciler rules exact). Verify: `go test ./deploy/k8s/manifest -v`.
- [ ] T7: Create `deploy/k8s/gen/main.go` per Design §7, run `go run ./deploy/k8s/gen` to write the four JSON files, and add `drift_test.go` (byte equality, no stray *.json) and the SC-004 grep test. Verify: `go run ./deploy/k8s/gen && git status --porcelain deploy/k8s` is empty after commit.
- [ ] T8: Amend spec FR-013 per Design §8. Then run every command in VALIDATION_CONTRACT.md and record each result in PROGRESS.md. Verify: `go build ./... && go vet ./... && go test ./... && go test -tags tmux ./... && go test -tags quickstart ./cmd/crswd && golangci-lint run` exits 0.

## Files touched

A diff outside this list is rejected.

- `api/v1alpha1/types.go`
- `api/v1alpha1/types_test.go`
- `internal/session/workdir.go`
- `internal/session/manager.go`
- `internal/session/workdir_lexical_test.go`
- `internal/session/resolver_test.go`
- `internal/admit/admit.go`
- `internal/admit/admit_test.go`
- `deploy/k8s/manifest/manifest.go`
- `deploy/k8s/manifest/crd.go`
- `deploy/k8s/manifest/rbac.go`
- `deploy/k8s/manifest/crd_test.go`
- `deploy/k8s/manifest/rbac_test.go`
- `deploy/k8s/manifest/drift_test.go`
- `deploy/k8s/manifest/fixtures_test.go`
- `deploy/k8s/gen/main.go`
- `deploy/k8s/crd.json`
- `deploy/k8s/rbac-reconciler.json`
- `deploy/k8s/rbac-lease.json`
- `deploy/k8s/rbac-daemon.json`
- `specs/017-k8s-native-execution/spec.md`

Never touch: `go.mod`, `go.sum`, `AGENTS.md` (at its 150-line CI cap), `docs/security.md`,
`.github/`, `.claude/`, `.specify/memory/`, any existing `*_test.go`, anything under `k8s/`
(the nested module is S4's), `cmd/crswd/` (S1 and S7 own it).

## Design

Decided in the source plan. Do not reopen these; if one cannot be built as written, log it in
PROGRESS.md under NEEDS CLARIFICATION and stop that task.

### §1 `api/v1alpha1/types.go` (package `v1alpha1`)

- Constants: `Group = "crswd.craigcloud.io"`, `Version = "v1alpha1"`, `Kind = "AgentSession"`,
  `ListKind = "AgentSessionList"`, `Plural = "agentsessions"`, `Singular = "agentsession"`,
  and `APIVersion = Group + "/" + Version`. Group is fixed by FR-005 (decided 9/26/26).
- `ObjectMeta`: `Name`, `Namespace`, `UID` (strings), `CreationTimestamp time.Time`,
  `Labels`, `Annotations` (`map[string]string`), `Finalizers []string`. JSON tags in
  Kubernetes spelling (`creationTimestamp`, `omitempty` on all but name).
- `AgentSession{APIVersion, Kind, Metadata ObjectMeta, Spec AgentSessionSpec, Status AgentSessionStatus}`
  with tags `apiVersion`, `kind`, `metadata`, `spec`, `status`.
- `AgentSessionSpec`, exactly these six fields and JSON names: `SessionName` `sessionName`,
  `Owner` `owner`, `WorkDir` `workDir`, `StartCommand` `startCommand` (the configured command
  **key**, never a command line), `Conversation` `conversation,omitempty`, `Lifetime` `lifetime`
  (a Go duration string such as `"8h"`, parsed with `time.ParseDuration`).
- `AgentSessionStatus`, exactly: `Phase` `phase,omitempty`, `Reason` `reason,omitempty`,
  `Conversation` `conversation,omitempty`.
- `type Phase string` with `PhasePending`, `PhaseRunning`, `PhaseRejected`, `PhaseReviving`,
  `PhaseFailed` ("Pending" … "Failed").
- No pod-shaping field (image, serviceAccount, env, volumes, secret, command, node,
  resources) and no token field. A field on the Spec is pod creation for anyone who can
  write the object. The reflect test lists the allowed JSON names and fails on any other.
- Comments explain why, per `docs/conventions.md`.

### §2 Lexical workdir check in `internal/session/workdir.go`

- Rename `underAnyRoot` to `UnderAnyRoot` and update its callers in the package. Keep
  `underRoot` unexported (the existing `workdir_test.go` calls it; do not edit that file).
- Add `LexicalWorkDir(p string, roots []config.ApprovedRoot) (string, error)`: empty →
  `ErrInvalidWorkDir`; not `filepath.IsAbs` → `ErrInvalidWorkDir` wrapping `ErrWorkDirNotAbsolute`;
  `filepath.Clean`, then not `UnderAnyRoot` → `ErrInvalidWorkDir` wrapping `ErrWorkDirOutsideRoots`;
  else return the cleaned path. No filesystem call. Doc comment: this is for a daemon that cannot
  see the session's filesystem; the resolved-and-verified check (constitution VI) runs in the pod
  (`internal/sessionpod`, S3). Never put the caller's path in the error.
- `ResolveWorkDir` behaviour is unchanged.

### §3 `Manager.SetWorkDirResolver`

- Field `resolveWorkDir func(string, []config.ApprovedRoot) (string, error)`, nil by default.
- `func (m *Manager) SetWorkDirResolver(f func(string, []config.ApprovedRoot) (string, error))`,
  a setter for the reason `SetStartCommands` is one (read its comment at `manager.go:220-228`).
- One private helper `m.workDir(p)` returns `m.resolveWorkDir(p, m.roots)` when set, else
  `ResolveWorkDir(p, m.roots)`. Replace the three calls in `manager.go`: `Create` (~line 675),
  `ReplayJournal` (~2303), `Continue` (~2447). Leave `conversation.go` alone: it reads the host's
  transcript directory, which is a different question.
- Nothing in `cmd/` or `internal/httpapi` calls the setter in this slice (S7 wires it).
- Test with `tmuxctl.NewFake()` and `NewManagerWithClock`, the way `manager_test.go` builds one.
  Root is a `t.TempDir()`; the missing dir is `filepath.Join(root, "absent")`.

### §4 `internal/admit/admit.go` (package `admit`)

- `func Admit(obj v1alpha1.AgentSession, roots []config.ApprovedRoot, cap int, others []v1alpha1.AgentSession, now time.Time) (bool, string)`.
  Pure: no I/O, no clock, no globals. The reason is a fixed sentence, never the caller's path.
- Order: workdir via `session.LexicalWorkDir` (reason names the allowlist); lifetime parses and
  is > 0 (`never` and garbage are rejected, since a pod needs a finite `activeDeadlineSeconds`);
  `CreationTimestamp + lifetime` not after `now` → rejected as past its lifetime; cap last.
- Cap: count `others` that are not the same object (match on UID when both set, else
  namespace+name), not in phase `Rejected` or `Failed`, and created before `obj`
  (earlier `CreationTimestamp`, ties broken by smaller name). Rejected when that count `>= cap`.
  `cap <= 0` rejects everything (fail closed, as an empty root list does).
- Table test, `t.Parallel()`, with `now` fixed. Case names must contain `outside`, `dotdot`,
  `over cap`, `past lifetime`, `admitted` so `-v` output shows them.

### §5 CRD in `deploy/k8s/manifest`

- `manifest.go`: package doc, namespace constants `SessionNamespace = "crswd-next"`,
  `ReconcilerNamespace = "crswd-next-reconciler"`, ServiceAccount names `DaemonSA = "crswd"`,
  `ReconcilerSA = "crswd-reconciler"` (names from source plan §4), and
  `Files() (map[string][]byte, error)` returning file name → bytes. Bytes are
  `json.MarshalIndent(v, "", "  ")` plus a trailing `\n`. Use structs or `map[string]any` (maps
  marshal sorted, so output is deterministic).
- `crd.go`: `apiextensions.k8s.io/v1` `CustomResourceDefinition`, name
  `agentsessions.crswd.craigcloud.io`, `scope: Namespaced`, names from §1 constants, one
  version `v1alpha1` served and storage, `subresources: {status: {}}`, a structural
  `openAPIV3Schema` with `spec` (properties from §1, `required` = all but `conversation`) and
  `status` (`phase` with an `enum` of the five phases, `reason`, `conversation`). Import the
  constants from `api/v1alpha1`; do not retype the group string.
- `additionalPrinterColumns` on `v1alpha1`, in this order: `Start` (`.spec.startCommand`, string),
  `Phase` (`.status.phase`, string), `Age` (`.metadata.creationTimestamp`, date). Spec 017 FR-021:
  the start command key says which runtime (Claude Code or Codex) the session runs, so
  `kubectl get agentsessions` shows it. Add no runtime field to the object.
- `shortNames: ["as"]`.

### §6 RBAC in `deploy/k8s/manifest/rbac.go`

Each file is a `{"apiVersion":"v1","kind":"List","items":[Role, RoleBinding]}` (kubectl applies
a List). `rbac.authorization.k8s.io/v1`. No ClusterRole anywhere.

- `rbac-reconciler.json`: Role `crswd-reconciler` in `crswd-next`: `""` pods
  `create,get,list,watch,delete`; `crswd.craigcloud.io` agentsessions `get,list,watch,update`;
  `crswd.craigcloud.io` agentsessions/status `update`. RoleBinding to SA `crswd-reconciler` in
  namespace `crswd-next-reconciler`.
- `rbac-lease.json`: Role `crswd-reconciler-lease` in `crswd-next-reconciler`:
  `coordination.k8s.io` leases `get,create,update`. RoleBinding to the same SA.
- `rbac-daemon.json`: Role `crswd` in `crswd-next`: agentsessions
  `get,list,watch,create,update,patch,delete`; `""` pods `get,list,watch`; `""` pods/exec
  `create,get` (the verbs D8b measured). RoleBinding to SA `crswd` in `crswd-next`.
- `rbac_test.go` walks the decoded `Files()` output, so it tests the bytes kubectl would apply. Fails if: any rule's apiGroups/resources/verbs contain `secrets` or `*`; any Role
  bound to `crswd` grants `create` on `pods`; any Role bound to `crswd` lives in
  `ReconcilerNamespace` or grants `pods/exec` outside `SessionNamespace`; the reconciler rules
  differ from the list above. Also a negative self-check: feed the walker a hand-built Role with a
  `secrets` rule and assert it reports it, so the walker is proven able to fail.

### §7 Generator and drift

- `deploy/k8s/gen/main.go` (package `main`): writes each `manifest.Files()` entry to
  `deploy/k8s/<name>` relative to the repo root (run from the root: `go run ./deploy/k8s/gen`).
  Returns a non-zero exit with the error on failure. gosec flags file writes wider than 0600
  (G306) and variable paths (G304); use `0o600` or a `//nolint:gosec` with a reason.
- `drift_test.go`: for every `Files()` entry, `os.ReadFile("../<name>")` equals the bytes, and
  `filepath.Glob("../*.json")` has no file `Files()` does not produce. Failure message says to
  run `go run ./deploy/k8s/gen`.
- `fixtures_test.go` (SC-004): walk `../` (all of `deploy/k8s`), `../../../api` and
  `../../../internal/admit`, and fail if any file contains `lawnmower` (case-insensitive) or
  `.claude`. Assert at least four files were read so an empty walk cannot pass. The test's own
  source must not contain those strings literally: build them from parts, and skip `*_test.go`
  files only if that proves necessary, stating why in a comment.

### §8 Spec amendment (FR-013)

In `specs/017-k8s-native-execution/spec.md` FR-013, add that the reconciler also updates
`agentsessions/status` in the session namespace and holds a Lease (`coordination.k8s.io`
`leases` get, create, update) in its own namespace, and name the daemon's `pods/exec` verbs
(`create`, `get`). FR-005's group is already filled (`crswd.craigcloud.io`); leave it. Keep the
spec's plain style: no em-dash, dates M/D/YY.
