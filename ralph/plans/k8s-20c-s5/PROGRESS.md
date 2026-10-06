# Progress: k8s-20c-s5

Notebook for the S5 slice of k8s-20c. Newest entry at the bottom.

## Iteration 0 (planning, 10/6/26)

Plan written by the operator's planner against main `50d7e9c` (#205 merged). S4 merged as #207:
`kube.InCluster() (*rest.Config, error)` and
`kube.NewElector(client kubernetes.Interface, c kube.ElectorConfig, onStart func(context.Context), onStop func())`.
S2 was still in its loop. Facts checked on disk, so no iteration has to rediscover them:

- `tmuxctl.Controller` has 13 methods (`internal/tmuxctl/controller.go`), including
  `PasteBracketed`, `PanePID` and `CaptureHistory`, which the source plan predates.
- The exported argv wrappers already exist (`internal/tmuxctl/argv.go`, S3). They carry no `-L`;
  `sessionpod.Socket` (`crswd-session`) is the pod's server name.
- `parseSessions`, `newBufferName`, `noServer`, `countLines` are unexported in
  `internal/tmuxctl/exec.go`; T1 exports what podctl needs.
- The session image's entrypoint is `tini -- /usr/local/bin/crswd` (`deploy/session-image/Dockerfile`),
  so in-pod subcommands are exec'd as `/usr/local/bin/crswd <subcommand> ...`.
- The daemon types the start command (E1). In a recreated pod tmux has no `@crswd-*` options, so
  its List row is unmanaged with unknown liveness, and the supervisor (rule 3 in `supervisor.go`)
  would leave the bare shell alone forever. T9's replay closes that.
- `supervisor.go` rule 6 calls `ResolveWorkDir` directly, which S2's `SetWorkDirResolver` does not
  cover, so a pod session would always give up with the workdir reason. T4 routes it.
- `hasTranscriptFor` reads the daemon's own disk; in kubernetes mode transcripts are on the pod's
  claim. T3, T4 and T10 move the question into the pod.
- Codex conversation discovery reads `/proc` under the pane, which is in another pod. T3 and T10
  move it into the pod (spec 019 T042, T043).
- A pod starts with an empty `CLAUDE_CONFIG_DIR`: research D8a's probe seeded
  `hasCompletedOnboarding`, workspace trust and the bypass acceptance. On this host the bypass
  acceptance is `skipDangerousModePermissionPrompt` in `settings.json`. T2's `Seed` writes them.
- Lint of the cluster module runs in CI, not here: the loop cannot `cd` into `k8s/`.

## Notes for S7 (do not act on these here)

- Dispatch `session-pod <name> <workdir>` as `sessionpod.Run(name, workdir, []config.ApprovedRoot{{Path: sessionpod.WorkRoot}})`.
- Dispatch `codex-conversation` and `has-transcript` as Design §10 states.
- The transcript hook is `func(ctx context.Context, s session.Session, id string) (bool, error)`
  (Design §4, after the critic pass): S7's `ClusterHooks.HasTranscript` must take that type and
  call `ctl.HasTranscript(ctx, s.TmuxName(), h, id, s.WorkDir)`, which returns `(bool, error)`.
- `sessionpod.NewInPodExec()` is the in-pod tmux controller for `codex-conversation`.
- Wire `mgr.SetCodexConversationFinder`, `mgr.SetTranscriptChecker`,
  `mgr.SetWorkDirResolver(session.LexicalWorkDir)` and `ctl.SetDescriber(mgr.PodRecord)` in
  kubernetes mode. Leave `SetClaudeConfig` and `SetCodexHome` unset there: trust is seeded in the
  pod (T2), and the daemon's own files are not the pod's.

## NEEDS CLARIFICATION

None open.

## Iteration 1: T1 (tmuxctl exports)

Added `internal/tmuxctl/export.go` (`ParseSessions`, `NewBufferName`, `Absent`) and `export_test.go`.

- Failing first: `go test ./internal/tmuxctl -run Export` failed by not compiling:
  `export_test.go:19:17: undefined: ParseSessions` (also `NewBufferName`, `Absent`).
- Gate: build, vet, test, `-tags tmux`, `-tags quickstart ./cmd/crswd` (port 8765 was free, passed),
  golangci-lint (0 issues), `go -C k8s` vet/test/build all green. No root go.sum, `grep -c require go.mod` is 0.
- Rediscovery savings: Bash here refuses commands containing `$?`; run the checks one per line or with `;`.
  A parse fixture row is `listFieldCount` pipe-joined empty fields with field 2 numeric (see `TestListFormatFieldCount`).
- Nothing noticed to fix.

## Iteration 2: T2 (sessionpod pass-through, WorkRoot, Seed)

Added `internal/sessionpod/seed.go` (`Seed`), `seed_test.go`; `sessionpod.go` gains `WorkRoot`, `Pod.Env`, `Pod.seed`, `CODEX_HOME` in `passThrough`, and `Run` seeds after `ResolveWorkDir` and before `Tmux.New`.

- Failing first: `go test ./internal/sessionpod -run 'PassThrough|Seed|RunSeeds'` failed by not compiling: `seed_test.go:68:12: undefined: Seed` and `p.Env undefined (type *Pod has no field or method Env)`. `TestPassThroughCarriesCodexHome` would also fail on the old `passThrough` once it compiles.
- Gate: build, vet, test, `-tags tmux`, `-tags quickstart ./cmd/crswd` (port free, passed), golangci-lint 0 issues, `go -C k8s` vet/test/build green, no root go.sum, `grep -c require go.mod` is 0.
- **Existing Run tests leave `Pod.Env` nil, so they now seed from `os.Environ()`.** That is the real `HOME`, `CLAUDE_CONFIG_DIR` and `CODEX_HOME` of whoever runs the suite. I added a package `TestMain` in `seed_test.go` that points all three at a temp dir (no existing test file edited). The first `go test ./internal/sessionpod` run before that TestMain existed may have created `~/.codex` and trust entries in the real home; this sandbox cannot list `~`, so I could not check. The operator should look for `~/.codex/config.toml` and a `.claude.json` trust entry for a `t.TempDir` path.
- Rediscovery savings: Bash refuses `$?` and `;`-joined commands that mix operations; one command per call. golangci-lint has errcheck check-blank on, so `_ = os.RemoveAll(x)` fails. The format hook already adds imports (goimports).

## Iteration 3: T3 (TranscriptExists, in-pod helpers)

Added `session.TranscriptExists` (conversation.go; `(*Manager).HasTranscript` now delegates), `internal/sessionpod/inpod.go` (`CodexConversation`, `HasTranscript`, `hasTranscriptIn`, `NewInPodExec`), and the tests `internal/session/transcript_test.go`, `internal/sessionpod/inpod_test.go`.

- Failing first: `go test ./internal/session ./internal/sessionpod -run 'TranscriptExists|CodexConversation|PodHasTranscript'` failed by not compiling: `undefined: TranscriptExists`, `undefined: CodexConversation`, `undefined: hasTranscriptIn`.
- Gate: build, vet, test, `-tags tmux`, `-tags quickstart ./cmd/crswd` (port free, passed), golangci-lint 0 issues, `go -C k8s` vet/test/build green, no root go.sum, `grep -c require go.mod` is 0.
- Behaviour to know: the Codex branch of `TranscriptExists` compares against the resolved workdir (Design §3). `(*Manager).hasTranscriptFor` is untouched and still uses `m.codexHome` and the unresolved `s.WorkDir`.
- Rediscovery savings: use the Write tool for test files; a bash heredoc containing a brace next to a quote is refused. `writeRollout`, `codexMetaLine` and `codexTestID` are reusable from any new file in package `session`. `claudeProjectDir` in `inpod_test.go` copies the unexported `projectDirFor`.
- Nothing noticed to fix.
