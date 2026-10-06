# Implementation plan: k8s-20c-s5

S5 of k8s-20c: `podctl`, the second `tmuxctl.Controller`, plus the in-pod helpers and the
`session.Manager` hooks that let one daemon drive Claude Code **and Codex** sessions that live
in pods (spec 017 FR-002, FR-011, FR-014, FR-021, FR-022; spec 019 FR-023, FR-024). Source:
`specs/017-k8s-native-execution/k8s-20c-plan.md` section 2 "S5", sections 5 (E1 to E5) and 7.
The definition of done is `ralph/plans/k8s-20c-s5/VALIDATION_CONTRACT.md`.

This slice also absorbs spec 019 Phase 4a (tasks T041, T042 and the root half of T043 in
`specs/019-codex-runtime/tasks.md`). Do not open `ralph/plans/codex-k8s`; the operator retires it.

## Dependency

S2 and S4 are merged into the branch this loop runs on. Check, from the repo root:
`test -f api/v1alpha1/types.go && test -f internal/admit/admit.go && test -f k8s/go.mod && grep -q 'func (m \*Manager) SetWorkDirResolver' internal/session/manager.go`.
If it exits non-zero, follow PROMPT.md "Blocked work".

## Tasks

Take the topmost open task. One per iteration. T1 to T4 are root module, standard library only.
T5 to T10 are the cluster module `k8s/`.

- [x] T1: tmuxctl exports for a second Controller per Design §1. Verify: `go test ./internal/tmuxctl -run 'Export' -v` passes.
- [x] T2: sessionpod: `CODEX_HOME` pass-through, `WorkRoot`, and the config seed run by `Pod.Run`, per Design §2. Verify: `go test ./internal/sessionpod -run 'PassThrough|Seed|RunSeeds' -v` passes.
- [x] T3: session `TranscriptExists` and the sessionpod in-pod helpers `CodexConversation` and `HasTranscript`, per Design §3. Verify: `go test ./internal/session ./internal/sessionpod -run 'TranscriptExists|CodexConversation|PodHasTranscript' -v` passes.
- [x] T4: Manager hooks per Design §4: `SetCodexConversationFinder`, `SetTranscriptChecker`, `PodRecord`, and the supervisor's allowlist check through the S2 resolver. Verify: `go test ./internal/session -run 'CodexConversationFinder|TranscriptChecker|PodRecord|SuperviseLexical' -v` passes.
- [x] T5: `k8s/internal/agentsession`, the typed client over the dynamic client, per Design §5. Verify: `go -C k8s test ./internal/agentsession -v` passes.
- [x] T6: `k8s/internal/podctl` skeleton: `Executor`, the recording fake, `RemoteExecutor`, `Config`, `New`, `SetDescriber`, per Design §6. Verify: `go -C k8s test ./internal/podctl -run 'Executor|NewController' -v` passes.
- [x] T7: podctl `New`, `SetOption`, `SendKeys`, `Paste`, `PasteBracketed`, `Resize`, `PanePID`, `CaptureHistory`, `Has`, `Kill`, `ReconcileServerEnvironment`, per Design §7. Verify: `go -C k8s test ./internal/podctl -run 'New|SetOption|SendKeys|Paste|Resize|PanePID|CaptureHistory|Has|Kill|Reconcile' -v` passes.
- [x] T8: podctl `CapturePane` over one held `pane-loop` stream per session, per Design §8. Verify: `go -C k8s test ./internal/podctl -run CapturePane -v` passes.
- [ ] T9: podctl `List`, with option replay into a fresh pod, per Design §9. Verify: `go -C k8s test ./internal/podctl -run List -v` passes.
- [ ] T10: podctl `CodexConversation` and `HasTranscript` methods, and the compile-time Controller assertion, per Design §10. Verify: `go -C k8s test ./internal/podctl -v` passes and `go -C k8s vet ./...` exits 0.
- [ ] T11: Run every command in VALIDATION_CONTRACT.md, record each result in PROGRESS.md, then append `RALPH_COMPLETE`. Verify: `go test ./... && go -C k8s test ./...` exits 0.

## Files touched

A diff outside this list is rejected.

- `internal/tmuxctl/export.go` (new), `internal/tmuxctl/export_test.go` (new)
- `internal/sessionpod/sessionpod.go`, `internal/sessionpod/seed.go` (new), `internal/sessionpod/seed_test.go` (new), `internal/sessionpod/inpod.go` (new), `internal/sessionpod/inpod_test.go` (new)
- `internal/session/conversation.go`, `internal/session/transcript_test.go` (new)
- `internal/session/manager.go`, `internal/session/supervisor.go`, `internal/session/hooks_test.go` (new)
- `k8s/go.mod`, `k8s/go.sum` (only if `go mod tidy` changes them)
- `k8s/internal/agentsession/` (new package)
- `k8s/internal/podctl/` (new package)
- `ralph/plans/k8s-20c-s5/`

Never touch: the root `go.mod`, any root `go.sum`, `cmd/crswd/`, `k8s/cmd/` (S7 owns it),
`k8s/internal/reconcile/` (S6 owns it), `k8s/internal/kube/` (S4's), `api/`, `deploy/`,
`AGENTS.md`, `docs/security.md`, `.github/`, `.claude/`, any existing `*_test.go`.

## Design

Decided here. Do not reopen it. If one item cannot be built as written, log it under NEEDS
CLARIFICATION in PROGRESS.md, mark the task `- [!]`, and stop.

### §1 `internal/tmuxctl/export.go`

Three wrappers, each one call to existing unexported code, for the reason `argv.go`'s package
comment gives (read it first, and copy its style):

- `func ParseSessions(stdout string) ([]SessionInfo, error) { return parseSessions(stdout) }`
- `func NewBufferName() (string, error) { return newBufferName() }`
- `func Absent(exitCode int, stderr string) bool`: true when `exitCode == 1` and stderr contains
  `msgNoSession` or `msgNoServer`, or contains both `msgNoSocket` and `msgNoSuchFile`. This is
  the answer `Exec.Has` and `noServer` give, keyed on an exit code instead of an `error`, because
  a remote exec returns the code, not an `*exec.ExitError`.

Tests in `export_test.go` (new file, package `tmuxctl`): `TestExportParseSessionsEqualsParser`
(one valid row string built the way `exec_test.go` builds one; grep `parseSessions(` there for a
fixture), `TestExportNewBufferNamePrefix` (prefix `BufferPrefix`, 16 hex digits after it, two
calls differ), `TestExportAbsent` table: `(1,"can't find session: crswd-x")` true,
`(1,"no server running on /tmp/x")` true, `(1,"error connecting to /tmp/x (No such file or directory)")`
true, `(1,"error connecting to /tmp/x (Permission denied)")` false, `(0,"")` false,
`(2,"can't find session")` false.

### §2 sessionpod: pass-through, `WorkRoot`, seed

In `internal/sessionpod/sessionpod.go`:

- `passThrough` becomes `[]string{"CLAUDE_CONFIG_DIR", "CODEX_HOME"}`, and its comment gains one
  sentence: CODEX_HOME is where Codex keeps its login, trust and rollouts in a pod (spec 019
  FR-023). This is spec 019 T041.
- Add `const WorkRoot = "/work"`: the one approved root inside a session pod (k8s-20c-plan E3).
  The reconciler (S6) mounts the session's working directory there and the S7 entrypoint passes
  `[]config.ApprovedRoot{{Path: WorkRoot}}` to `Run`.
- In `(*Pod).Run`, immediately after `ResolveWorkDir` succeeds and before `p.Tmux.New`, call
  `if err := p.seed(resolved); err != nil { return fmt.Errorf("session pod %s: seed: %w", name, err) }`.
  `Pod` gains a field `Env []string` (nil means `os.Environ()`), and `p.seed(dir)` calls
  `Seed(p.env(), dir)`. The package-level `Run` leaves `Env` nil.

New file `internal/sessionpod/seed.go`:

`func Seed(env []string, workDir string) error`. A pod starts with an empty Claude config
directory and a Codex home that may be empty, and no person is there to answer the first-run
screens (research D8a: the probe pod seeded these three facts and saw no prompt).

1. `claudeDir` = the value of `CLAUDE_CONFIG_DIR` in env (last one wins, trimmed). Empty or not
   absolute: skip steps 2 to 4.
2. `os.MkdirAll(claudeDir, 0o700)`.
3. `<claudeDir>/.claude.json`: if absent, write `{"hasCompletedOnboarding":true,"projects":{"<workDir>":{"hasTrustDialogAccepted":true}}}`
   built with `encoding/json` (never by string concatenation), mode `0o600`, by writing a temp
   file in the same directory and renaming it. If present, call
   `session.SeedTrust(path, workDir)`, which adds the trust entry and keeps everything else.
4. `<claudeDir>/settings.json`: if absent, write `{"skipDangerousModePermissionPrompt":true}`,
   mode `0o600`, same temp-and-rename. If present, leave it alone.
5. `codexHome := session.CodexHome(env)`. If non-empty: `os.MkdirAll(codexHome, 0o700)`, then
   `session.SeedCodexTrust(codexHome, workDir)`.
6. Return the first error, wrapped with the file it concerns. Never put file contents in an error.

Tests, `seed_test.go` (new):
- `TestSeedWritesFreshClaudeConfig`: temp dirs for CLAUDE_CONFIG_DIR, CODEX_HOME and the workdir.
  After `Seed`, decode `.claude.json`: `hasCompletedOnboarding` true and
  `projects[<workdir>].hasTrustDialogAccepted` true; `settings.json` decodes to
  `skipDangerousModePermissionPrompt: true`; both files mode `0600`.
- `TestSeedKeepsExistingClaudeConfig`: a pre-existing `.claude.json` with an extra key `"keep":1`
  keeps it and gains the trust entry; a pre-existing `settings.json` is byte-identical after.
- `TestSeedCodexTrust`: `<codexHome>/config.toml` exists after and contains `trust_level = "trusted"`
  under the workdir's project table.
- `TestSeedNoConfigDirs`: env without either variable and HOME unset returns nil and creates nothing.
- `TestPassThroughCarriesCodexHome`: `environment([]string{"CODEX_HOME=/c","CLAUDE_CONFIG_DIR=/d","PATH=/bin","HOME=/h"})`
  contains `CODEX_HOME=/c` and `CLAUDE_CONFIG_DIR=/d`. Fails before T2 because CODEX_HOME is dropped.
- `TestRunSeedsBeforeNew`: a `Pod{Tmux: <fake>, Env: ...}` with temp dirs; after `Run` returns
  (the fake session ends when the test calls `Kill` on the fake, the way `sessionpod_test.go`
  ends a Run; grep `Run(ctx` there), `.claude.json` exists. Use `tmuxctl.NewFake()`.

### §3 `TranscriptExists` and the in-pod helpers

In `internal/session/conversation.go`:

- Add `func TranscriptExists(h harness.Name, id, workDir, home, codexHome string, roots []config.ApprovedRoot) bool`.
  Claude: the body of `(*Manager).HasTranscript` with `home` passed in instead of
  `os.UserHomeDir()` and `roots` instead of `m.roots`. Codex: `codexHome != "" && codexHasTranscript(filepath.Join(codexHome, "sessions"), id, workDir)`,
  after `ResolveWorkDir(workDir, roots)` succeeds, comparing against the resolved directory.
  Other: false.
- Rewrite `(*Manager).HasTranscript` to `home, err := os.UserHomeDir(); if err != nil || home == "" { return false }; return TranscriptExists(harness.Claude, conversationID, workDir, home, "", m.roots)`.
  Behaviour is unchanged; the existing tests prove it.

New file `internal/sessionpod/inpod.go`. These are what S7 dispatches as subcommands inside a pod:

- `func CodexConversation(ctx context.Context, tmux tmuxctl.Controller, name, procRoot, codexHome string) (string, error)`:
  `codexHome == ""` returns `"", nil` with no tmux call. Otherwise `pid, err := tmux.PanePID(ctx, name)`,
  then `session.DiscoverCodexConversation(procRoot, pid, filepath.Join(codexHome, "sessions"))`.
  This is spec 019 T042.
- `func HasTranscript(h harness.Name, id, workDir string, env []string) bool { return hasTranscriptIn(WorkRoot, h, id, workDir, env) }`
  and the unexported `func hasTranscriptIn(root string, h harness.Name, id, workDir string, env []string) bool`:
  `home` is `HOME` from env (empty when absent), and the call is
  `session.TranscriptExists(h, id, workDir, home, session.CodexHome(env), []config.ApprovedRoot{{Path: root}})`.
  The root is a parameter only so a test can use a temp dir; production always passes `WorkRoot`.
- `func NewInPodExec() (*tmuxctl.Exec, error) { return newExec() }`: the pod's own tmux server, for
  S7's `codex-conversation` dispatch.

Tests:
- `internal/session/transcript_test.go` (new): `TestTranscriptExistsClaude` (temp home; create
  `<home>/.claude/projects/<projectPath>/<id>.jsonl` using the same helper the existing
  HasTranscript tests use; grep `HasTranscript` in `conversation_test.go` and reuse their setup
  pattern, without editing that file), `TestTranscriptExistsCodex` (a rollout file named
  `rollout-<ts>-<id>.jsonl` under `<codexHome>/sessions/2026/10/06/` whose first line is a
  `session_meta` with `id` and `cwd` = the workdir; copy a fixture shape from
  `conversation_codex_test.go`), `TestTranscriptExistsOther` false, and a case where `workDir` is
  outside `roots` returns false for both harnesses.
- `internal/sessionpod/inpod_test.go` (new): `TestCodexConversationHomeResolution` (empty
  codexHome returns "" and the fake records no PanePID call), `TestCodexConversationFromProc`
  (a fake proc root as `discover_test.go` builds one; grep `DiscoverCodexConversation(` there),
  `TestPodHasTranscript` (call `hasTranscriptIn` with a temp root, HOME and CODEX_HOME set to temp
  dirs, and workDir inside the temp root: a Codex rollout under `<CODEX_HOME>/sessions` whose
  `cwd` is that workDir is found; a Claude transcript under `<HOME>/.claude/projects/...` is found;
  Other and a workDir outside the root are false).

### §4 Manager hooks

In `internal/session/manager.go`:

- Field `checkTranscript func(ctx context.Context, s Session, id string) (bool, error)`, nil by default.
- `func (m *Manager) SetCodexConversationFinder(f func(ctx context.Context, s Session) (string, error))`:
  nil restores `m.hostCodexConversation`. A setter for the reason `SetStartCommands` is one.
  Doc: in kubernetes mode the pane's process tree is in another pod, so the daemon asks the pod.
- `func (m *Manager) SetTranscriptChecker(f func(ctx context.Context, s Session, id string) (bool, error))`:
  nil restores the host check. The error exists because in a pod the answer comes over the
  network: "could not ask" must not read as "no transcript", which the supervisor turns into a
  permanent give-up.
- Add `func (m *Manager) transcriptFor(ctx context.Context, s Session, id string) (bool, error)`
  in `conversation.go`: with a checker set, return its answer; otherwise `return m.hasTranscriptFor(s, id), nil`.
  Leave `hasTranscriptFor` itself unchanged.
- In `supervisor.go` rule 7 (`grep -n 'reasonNoTranscript' internal/session/supervisor.go`),
  replace `if !s.mgr.hasTranscriptFor(sess, sess.ConversationID)` with
  `ok, err := s.mgr.transcriptFor(ctx, sess, sess.ConversationID); if err != nil { return fmt.Errorf("ask whether session %s has a transcript: %w", sess.ID, err) }; if !ok`.
  An error ends this sweep's judgement of the session with no attempt spent (the revive bound is
  written after this point), so the next sweep asks again.
- Every other caller of `hasTranscriptFor` (grep it) keeps its current behaviour; list them in
  PROGRESS.md for S7.
- `type PodRecord struct { ID, Name, Owner, WorkDir, StartCommand, ConversationID string; Deadline time.Time; LifetimeDisabled bool }`
  and `func (m *Manager) PodRecord(tmuxName string) (PodRecord, bool)`: the session in
  `m.store.snapshot()` whose `TmuxName()` equals tmuxName, copied field by field (`Owner` as
  `string(s.Owner)`, `Deadline` = `s.AbsoluteDeadline()`, `LifetimeDisabled` = `s.LifetimeDisabled()`).
  It exists because `tmuxctl.Controller.New` carries only a name and a directory, and a pod's
  object also needs the owner, start command and lifetime (S2's required spec fields). The record
  is in the store before `start` calls `New` (`m.store.AddCapped` precedes `m.start` in `Create`).

In `internal/session/supervisor.go`, rule 6 (`grep -n 'reasonWorkDirRefused' internal/session/supervisor.go`):
replace `ResolveWorkDir(sess.WorkDir, s.mgr.roots)` with the private helper S2 added that honours
`SetWorkDirResolver` (find it with `grep -n 'resolveWorkDir' internal/session/manager.go`; S2's
design names it `m.workDir`). With no resolver set, behaviour is unchanged.

Tests, `hooks_test.go` (new), built with `tmuxctl.NewFake()` and `NewManagerWithClock` as
`manager_test.go` builds one:
- `TestCodexConversationFinderIsUsed`: set a finder returning a fixed UUID; one supervisor `Sweep`
  over a healthy Codex session records that id (`ConversationID` on the stored session). Copy the
  Codex-session setup from `supervisor_test.go` (grep `findCodexConversation`).
- `TestTranscriptCheckerIsUsed`: a checker that returns `true, nil` makes `transcriptFor` true for a
  Claude session with no transcript on disk; nil restores false.
- `TestTranscriptCheckerErrorDoesNotGiveUp`: a checker returning an error; one `Sweep` over a
  stopped session with a conversation id leaves it not `StateFailed` and its `ReviveAttempts`
  unchanged, and `Sweep` returns an error.
- `TestPodRecordFindsByTmuxName`: after `Create`, `PodRecord(s.TmuxName())` returns the owner,
  name, workdir and start command; an unknown name returns false.
- `TestSuperviseLexicalWorkDir`: with `SetWorkDirResolver(LexicalWorkDir)` and a session whose
  workdir is a non-existent path under the root, a sweep of a session marked stopped does not give
  up with the workdir reason (assert the store's state is not `StateFailed` with that reason).

### §5 `k8s/internal/agentsession`

Files: `k8s/internal/agentsession/agentsession.go` and `agentsession_test.go`.

Package doc: the one place AgentSession objects cross the dynamic client, converted to and from
`api/v1alpha1` by JSON round-trip (k8s-20c-plan S4: no generated clientset).

```go
var GVR = schema.GroupVersionResource{Group: v1alpha1.Group, Version: v1alpha1.Version, Resource: v1alpha1.Plural}
type Client struct { dyn dynamic.Interface; ns string }
func New(dyn dynamic.Interface, namespace string) (*Client, error)        // empty namespace is an error
func (c *Client) Get(ctx context.Context, name string) (v1alpha1.AgentSession, bool, error) // NotFound → false, nil
func (c *Client) List(ctx context.Context) ([]v1alpha1.AgentSession, error)
func (c *Client) Create(ctx context.Context, obj v1alpha1.AgentSession) (v1alpha1.AgentSession, error)
func (c *Client) Delete(ctx context.Context, name string) error             // NotFound is nil; never sets GracePeriodSeconds
func (c *Client) SetAnnotation(ctx context.Context, name, key, value string) error // JSON merge patch {"metadata":{"annotations":{key:value}}}
func (c *Client) UpdateStatus(ctx context.Context, name string, status v1alpha1.AgentSessionStatus) error
func (c *Client) Update(ctx context.Context, name string, mutate func(*v1alpha1.AgentSession)) error
func ToObject(u *unstructured.Unstructured) (v1alpha1.AgentSession, error)
func FromObject(obj v1alpha1.AgentSession) (*unstructured.Unstructured, error)
```

- `FromObject` sets `APIVersion` and `Kind` from the `v1alpha1` constants, marshals with
  `encoding/json`, unmarshals into `map[string]any`, and deletes `metadata.creationTimestamp`
  when the object's `CreationTimestamp.IsZero()` (a zero `time.Time` marshals as year 1).
- `ToObject` is `json.Marshal(u.Object)` then `json.Unmarshal` into the struct.
- `UpdateStatus` and `Update` never send a typed object back: S2's `ObjectMeta` has no
  `resourceVersion`, and the API server refuses an update without one. Each does a fresh `Get` of
  the unstructured object (which carries `metadata.resourceVersion`), changes only its part, and
  sends that same unstructured object:
  - `UpdateStatus`: `unstructured.SetNestedField(u.Object, <status as map[string]any via JSON>, "status")`,
    then `dyn.Resource(GVR).Namespace(ns).UpdateStatus(ctx, u, metav1.UpdateOptions{})`.
  - `Update`: `ToObject(u)`, call `mutate` on the copy, write back only `metadata.annotations` and
    `metadata.labels` into `u`, then `Update(ctx, u, metav1.UpdateOptions{})`. `spec` is never written.
  - A conflict is returned as is; the caller retries on its next pass. Both use the `update` verb,
    which S2's RBAC grants the reconciler. `SetAnnotation` uses `patch`, which S2 grants the daemon.
- Tests with `dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{GVR: v1alpha1.ListKind})`:
  create then get round-trips every spec field; get of a missing name is `false, nil`; delete of a
  missing name is nil; `SetAnnotation` adds a key and keeps existing ones; `FromObject` of a zero
  timestamp has no `creationTimestamp` key; `New(dyn, "")` errors; `UpdateStatus` changes the
  status and leaves spec and annotations as they were; `Update` changes an annotation and leaves
  spec as it was.

### §6 podctl skeleton

`k8s/internal/podctl/podctl.go`:

```go
// Executor runs argv in the session container of one pod. exitCode is the remote
// process's status; err is a transport or API failure, never a non-zero exit.
type Executor interface {
    Exec(ctx context.Context, pod string, argv []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)
}
type Describer func(tmuxName string) (session.PodRecord, bool)
type Config struct {
    Namespace    string
    PaneBound    int           // config.Config.PaneBound; < 1 is an error, as tmuxctl.NewExec refuses it
    ReadyTimeout time.Duration // 0 → 90s (D8b cold start 45 to 52 s)
    KillTimeout  time.Duration // 0 → 60s
    PollInterval time.Duration // 0 → 1s
    StreamIdle   time.Duration // 0 → 30s
    Now          func() time.Time // nil → time.Now
}
// New refuses any negative duration (a zero one takes its default), because a negative interval
// reaches time.NewTicker, which panics.
type Controller struct { /* sessions *agentsession.Client; pods kubernetes.Interface; exec Executor; describe Describer; cfg Config; mu sync.Mutex; streams map[string]*stream */ }
func New(sessions *agentsession.Client, pods kubernetes.Interface, exec Executor, cfg Config) (*Controller, error)
func (c *Controller) SetDescriber(d Describer)
```

Constants: `Container = "session"`, `Binary = "/usr/local/bin/crswd"` (the session image's COPY
target, `deploy/session-image/Dockerfile`), `AnnotationPrefix = v1alpha1.Group + "/"`.
`func annotationKey(option string) string { return AnnotationPrefix + strings.TrimPrefix(option, "@crswd-") }`.
`func inPod(argv []string) []string { return append([]string{"tmux", "-L", sessionpod.Socket}, argv[1:]...) }`:
every `tmuxctl.Argv*` result goes through it (the wrappers carry no `-L`; `sessionpod.Socket`'s
comment says why).

`k8s/internal/podctl/fake_test.go`: `recorder`, an `Executor` that records each call's pod, argv
and stdin bytes, and answers from a table keyed by `strings.Join(argv, " ")` with
`{stdout, stderr string; code int; err error}`; an unmatched argv answers code 0.

`k8s/internal/podctl/remote.go`: `type RemoteExecutor struct { Config *rest.Config; Client kubernetes.Interface; Namespace string }`
and its `Exec`, written exactly as:

```go
req := r.Client.CoreV1().RESTClient().Post().Resource("pods").Namespace(r.Namespace).Name(pod).
    SubResource("exec").VersionedParams(&corev1.PodExecOptions{Container: Container, Command: argv,
    Stdin: stdin != nil, Stdout: stdout != nil, Stderr: stderr != nil}, scheme.ParameterCodec)
ws, err := remotecommand.NewWebSocketExecutor(r.Config, "GET", req.URL().String())
if err != nil { return 0, err }
spdy, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
if err != nil { return 0, err }
ex, err := remotecommand.NewFallbackExecutor(ws, spdy, func(err error) bool {
    return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err) })
if err != nil { return 0, err }
err = ex.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: stdin, Stdout: stdout, Stderr: stderr})
var ce utilexec.CodeExitError
if errors.As(err, &ce) { return ce.Code, nil }
return 0, err
```

Imports: `k8s.io/client-go/tools/remotecommand`, `k8s.io/client-go/kubernetes/scheme`,
`k8s.io/apimachinery/pkg/util/httpstream`, `utilexec "k8s.io/client-go/util/exec"`,
`corev1 "k8s.io/api/core/v1"`. It has no unit test beyond `TestRemoteExecutorImplementsExecutor`
(`var _ Executor = (*RemoteExecutor)(nil)`); S7's live acceptance exercises it.

Tests: `TestNewControllerRefuses` (empty namespace, `PaneBound` 0, nil executor, and each negative
duration error),
`TestExecutorRecorder` (the fake records argv and stdin).

### §7 podctl methods

Every pod is named after its tmux session (`crswd-<id>`), and so is its AgentSession. Each
method below lists what it does in order.

- `New(ctx, name, workDir)`:
  1. `rec, ok := c.describe(name)`. No describer or `!ok` is an error (`podctl: no session record for <name>`).
  2. `rec.LifetimeDisabled` is an error: `podctl: a session in kubernetes mode needs a finite lifetime`
     (FR-007: the pod's `activeDeadlineSeconds`).
  3. `Get` the object. If absent, `Create` one with name `name`, `spec{sessionName: rec.Name, owner: rec.Owner, workDir: workDir, startCommand: rec.StartCommand, conversation: rec.ConversationID, lifetime: <d>}`,
     where `d` is `rec.Deadline.Sub(now)` truncated **down** to a whole second
     (`.Truncate(time.Second)`) and formatted with `time.Duration.String()`. Down, because the
     server stamps `creationTimestamp` after `now`, and the pod's deadline must not land past the
     session's. `d <= 0` is an error before any create.
  4. Poll every `PollInterval` until `ReadyTimeout`: `Get` the object (its `status.phase`
     `Rejected` or `Failed` returns an error carrying `status.reason`); get pod `name`; when the pod
     phase is `Running`, exec `inPod(tmuxctl.ArgvHas(name))`; exit 0 returns nil. Timeout is an
     error naming the last phase seen.
- `SetOption(ctx, name, option, value)`: `SetAnnotation(name, annotationKey(option), value)` first
  (the object is the record), then exec `inPod(tmuxctl.ArgvSetOption(name, option, value))`; a
  non-zero exit is an error with stderr's first line. A failure between the two leaves the pod
  behind the record, and §9's repair fixes it on the next `List`.
- `SendKeys`: exec `inPod(tmuxctl.ArgvSendKeys(name, keys...))`.
- `Paste` / `PasteBracketed`: `buf, err := tmuxctl.NewBufferName()`, and an error returns before
  any exec; `load, paste := tmuxctl.ArgvPaste(buf, name)`
  (or `ArgvPasteBracketed`); exec `inPod(load)` with `stdin = bytes.NewReader(payload)`; then exec
  `inPod(paste)` with nil stdin; if that fails, exec `inPod(tmuxctl.ArgvDeleteBuffer(buf))` on
  `context.WithoutCancel(ctx)` and join both errors, as `tmuxctl.Exec.paste` does. The payload never
  appears in an argv and never in an error.
- `Resize`: exec `inPod(tmuxctl.ArgvResize(name, cols, rows))`.
- `PanePID`: exec `inPod(tmuxctl.ArgvPanePID(name))`; `strconv.Atoi(strings.TrimSpace(stdout))`;
  not a positive integer is `tmuxctl.ErrUnexpectedOutput`.
- `CaptureHistory`: exec `inPod(tmuxctl.ArgvCaptureHistory(name))` into a buffer capped at 4 MiB;
  past the cap, or more than `tmuxctl.HistoryLimit` lines, is `tmuxctl.ErrHistoryTooLarge`. Return
  `tmuxctl.Strip(stdout)`.
- `Has(ctx, name)` answers whether anything of the session is still alive, because
  `session.Manager` uses it to confirm a teardown:
  1. Pod `name` exists and is `Running`: exec `inPod(tmuxctl.ArgvHas(name))`; 0 is true,
     `tmuxctl.Absent(code, stderr)` is false, anything else is an error.
  2. Pod exists in any other phase, terminating included: `true, nil`.
  3. Pod absent and object present: `true, nil` (the reconciler is bringing its pod).
  4. Both absent: `false, nil`.
  An API error other than NotFound is an error.
- `Kill(ctx, name)`: stop the name's stream (§8); `Delete` the object (no grace override); poll the
  pod by name every `PollInterval` until NotFound or `KillTimeout`; timeout is an error
  `podctl: pod <name> is still there after the session was deleted`. Never delete the pod with
  `GracePeriodSeconds: 0` (spec 017 FR-010).
- `ReconcileServerEnvironment`: returns `tmuxctl.Reconciliation{}, nil`. Each pod's environment is
  fixed by the reconciler's pod template, so there is no shared server table to reconcile.

Tests (fake clientset `k8s.io/client-go/kubernetes/fake` for pods, dynamic fake for objects, the
recorder for exec; set pod phase by creating the pod object in the fake):
argv per method equals `inPod(tmuxctl.Argv*)`; Paste's payload reaches stdin and is in no argv;
a failed paste-buffer runs delete-buffer; `New` creates exactly one object with the spec above,
and a second `New` creates none; `New` with `LifetimeDisabled` creates nothing; `New` on a
`Rejected` object returns its reason; `Kill` errors while the pod object still exists and succeeds
once it is gone; `Has` maps exit 0/absent/other as above; `SetOption` writes the annotation key
`crswd.craigcloud.io/managed` for `@crswd-managed`.

### §8 `CapturePane`

One held exec of `[]string{Binary, "pane-loop", name}` per watched session (FR-011, D8b), with
`stdout` a frame splitter on `sessionpod.FrameSeparator`.

- `type stream struct { latest string; have bool; err error; lastUse time.Time; cancel context.CancelFunc; ready chan struct{} }`.
- `CapturePane(ctx, name)`: under `c.mu`, find or start the stream (start = a goroutine below,
  on a context derived from `context.Background()`, never the caller's). Set `lastUse = now`.
  Wait for `ready` (closed on the first frame or first error) or 3 s or `ctx.Done()`, whichever is
  first; 3 s with nothing is an error `podctl: no frame from <name> yet`. Return `latest, err`.
- The goroutine loops: `code, err := Exec(streamCtx, name, argv, nil, splitter, io.Discard)`;
  when it returns, a transport error or a non-zero code is recorded as `stream.err` only if no
  frame has arrived yet (a later reopen may still deliver one); if `streamCtx` is done, exit; else
  wait `PollInterval` and exec again (a cut stream is reopened).
- Idle is checked on every frame, not only when the exec returns, because a healthy stream never
  returns: the splitter, after storing a frame, cancels `streamCtx` and removes the stream from the
  map when `now - lastUse > StreamIdle`.
- Every read and write of a stream's fields happens under `c.mu`. `ready` is closed exactly once,
  through a `sync.Once` on the stream, on the first frame or the first recorded error.
- Run `go -C k8s test -race ./internal/podctl -run CapturePane` once in this task and record the
  result in PROGRESS.md.
- The splitter keeps a partial frame across writes. Each complete frame: `screen := tmuxctl.Strip(frame)`;
  more than `PaneBound` lines (count lines exactly as `countLines` in
  `internal/tmuxctl/exec.go` does; copy its body into an unexported podctl helper with a comment
  naming the source) sets `err = tmuxctl.ErrPaneTooLarge` and keeps the old
  `latest`; otherwise sets `latest = screen, err = nil`.
- `Kill` cancels and removes the stream.

Tests: frames written in pieces across several `Write` calls reassemble; ANSI in a frame is
stripped; an oversize frame returns `ErrPaneTooLarge`; the stream is reopened after the fake's
exec returns (count execs); a stream that keeps delivering frames but has had no `CapturePane`
for longer than `StreamIdle` (inject `Now`) is cancelled while its exec is still running, and the
next `CapturePane` starts a new one; an exec that exits non-zero before any frame makes
`CapturePane` return an error; `Kill` stops it.

### §9 `List`

`List(ctx) ([]tmuxctl.SessionInfo, error)`:

1. `sessions.List`. Skip objects in phase `Rejected` or `Failed`.
2. For each, the base row from annotations: `Name` = object name; `Created` = the object's
   `CreationTimestamp` (always, never the in-pod tmux time: a revived pod's tmux is younger than
   the session); `Managed` = annotation `managed` == `tmuxctl.OptionManagedValue`; `Label`, `StartCommand`,
   `Lifetime`, `Width`, `ConversationID` from their annotations; `WorkDir` from the `workdir`
   annotation base64-decoded with `base64.StdEncoding` (empty on a decode error); `Claude` =
   `tmuxctl.LivenessUnknown`.
3. If pod `name` is `Running`: exec `inPod(tmuxctl.ArgvList())`; `tmuxctl.ParseSessions`; find the
   row named `name`.
   - Repair when the base row is managed and the in-pod row disagrees with it: `Managed` false,
     `Claude == LivenessUnknown` (no `@crswd-binary`), or any of `Label`, `WorkDir`,
     `StartCommand`, `Lifetime`, `Width`, `ConversationID` different from the base row. That is a
     pod the reconciler recreated, or a `SetOption` whose in-pod half failed. Repair replays every
     annotation-backed option with exec `inPod(tmuxctl.ArgvSetOption(...))`: the options
     `session.markSession` writes (grep it for the list) plus `@crswd-width` when annotated, with
     `@crswd-managed` **last**, so a repair cut off part-way still reads as unmanaged and is
     retried in full. Then list once more and take that row.
   - Take `Claude` and, when non-empty, `ConversationID` from the in-pod row.
4. An exec error for one pod leaves that row with `LivenessUnknown` and does not fail `List`
   (the supervisor reads Unknown as alive). An error from `sessions.List` fails `List`.

Why the replay matters: the supervisor revives a session only when its row says
`LivenessStopped` (`supervisor.go` rule 3). A fresh pod runs a bare shell; once its tmux has
`@crswd-binary` again, tmux reports the shell as stopped and the supervisor types the resume line
(E1: the daemon owns the start command).

Tests: two objects, one pod Running and one Pending: two rows, the Pending one `LivenessUnknown`;
a Rejected object yields no row; a Running pod whose in-pod list lacks the managed option gets one
`set-option` exec per annotation with `@crswd-managed` the last of them, then a second
`list-sessions` exec; a managed in-pod row whose label differs from the annotation is repaired
too; `Created` equals the
object's timestamp; base64 workdir decodes.

### §10 Codex and transcript methods, assertion

- `func (c *Controller) CodexConversation(ctx context.Context, name string) (string, error)`: exec
  `[]string{Binary, "codex-conversation", name}`; exit 0 with empty stdout is `"", nil`; exit 0
  with stdout trimmed passed through `session.ValidateResume` (an invalid id is an error, never
  recorded); non-zero is an error with stderr's first line. This is the daemon half of spec 019
  T043; S7 wires it with `mgr.SetCodexConversationFinder(func(ctx, s) { return ctl.CodexConversation(ctx, s.TmuxName()) })`.
- `func (c *Controller) HasTranscript(ctx context.Context, name string, h harness.Name, id, workDir string) (bool, error)`:
  exec `[]string{Binary, "has-transcript", name, string(h), id, workDir}`; exit 0 is `true, nil`;
  exit 1 is `false, nil` (definitely absent); a transport error or any other exit is
  `false, err`. Only a definite answer may end a session (§4).
- `var _ tmuxctl.Controller = (*Controller)(nil)` in `podctl.go`.

S7 dispatches `codex-conversation <name>` as
`sessionpod.CodexConversation(ctx, <real exec>, name, "/proc", session.CodexHome(os.Environ()))`
and `has-transcript <name> <harness> <id> <workdir>` as
`sessionpod.HasTranscript(harness.Name(h), id, workdir, os.Environ())` (exit 0 when true, 1 when
false). Record this in PROGRESS.md for S7.

Tests: argv of each; a non-UUID stdout is an error; exit 1 for has-transcript is `false, nil`;
exit 2 and a transport error are errors.
