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
