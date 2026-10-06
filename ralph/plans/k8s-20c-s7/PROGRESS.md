# Progress: k8s-20c-s7

Notebook for the S7 slice of k8s-20c. Newest entry at the bottom.

## Iteration 0 (planning, 10/6/26)

Plan written by an operator-session planner against origin/main `50d7e9c`. Facts checked:

- `cmd/crswd/main.go` `run()` (~line 160) is the run sequence to mirror: `loadConfig`
  (`config.Load()`), `Runnable`, `CheckDependencies`, the host-only block, `newDaemon`,
  `Reconcile`, `StartReaper`, `StartSupervisor`, `Listen`, `Serve`, `Shutdown` with
  `shutdownBudget = 30 * time.Second` (main.go:56).
- `internal/config/config.go` ~line 450: `const kubernetesModeBuilt = false`, read only by
  `ExecutionMode.Runnable` (~455). `Runnable` is called by `cmd/crswd/main.go:183` and
  `internal/config/write.go:310`. Tests that pin the refusal: `cmd/crswd/executionmode_test.go:190`,
  `internal/config/executionmode_test.go:241,271`. They stay green under §1.
- `internal/httpapi/server.go`: `New` (~337) returns early in kubernetes mode before the relays and
  the release feed; `NewWith(cfg, tmux, trail)` (~449) wires none of them. `Server.sessions` is a
  `*session.Manager` field (~194).
- `internal/sessionpod/sessionpod.go`: `Run(name, workdir string, roots []config.ApprovedRoot) error`
  (~289) and `PaneLoop(name string, w io.Writer) error` (~302).
- `internal/sessionpod/dockerfile_test.go` pins exactly one COPY and the tag scheme; T4 edits
  those two assertions only.
- Base image `docker.io/nctiggy/ralph-runner:2.1.246-ci10`: Debian 13, has node, claude 2.1.246,
  tmux, tini, curl, tar and `/etc/ssl/certs/ca-certificates.crt` (ca-certificates 20250419). No
  codex, no zstd (hence the `.tar.gz` asset).
- Codex asset `codex-x86_64-unknown-linux-musl.tar.gz` from `rust-v0.153.4`, sha256
  `f479424eca092484dc40d87ae28c44f4cc40234a60045d6131e493800d814a30`, one member
  `codex-x86_64-unknown-linux-musl` (258,659,424 bytes) that printed `codex-cli 0.153.4` when run
  inside the base image.
- The loop's `.claude/settings.json` allows `go -C k8s:*` once S4 merges. It cannot `cd`, so the
  cluster module is linted by CI only.

- Critic pass (Codex, 10/6/26) folded in: explicit cluster wiring (§3), `/work` emptyDir and
  ServiceAccounts and selector labels in the manifests, `CRSW_SESSION_NODE=lm-amd64-1`, a `codex=`
  start command, `openssl rand -hex 32` for the shared secret (`crswd keygen` is the release-signing
  key pair), the host-switch test made failure-first, `fetch-codex.sh` creates its directory.
- S5 hands S7: dispatch `codex-conversation <name>` and `has-transcript <name> <harness> <id> <workdir>`,
  wire `SetCodexConversationFinder` and `SetTranscriptChecker`, and `ctl.SetDescriber`. S6 hands
  S7: hold `podctl.Container == reconcile.SessionContainer` in a test. `ralph/plans/codex-k8s` is
  retired by S5.

## NEEDS CLARIFICATION

None open.

## Iteration 1 (T1, the cluster switch)

Done: `kubernetesModeBuilt` is now `var kubernetesModeBuilt bool` in `internal/config/config.go`,
with `BuildKubernetesMode()` beside it and the comment rewritten. New
`cmd/crswd/hostswitch_test.go` (`TestHostNeverBuildsKubernetesMode`) walks the root module,
skipping `k8s/`, `.git/`, `.claude/`, `testdata`, and fails on any non-test `.go` file that calls
the setter, unless it is the definition line in `config.go`. It needs the definition exactly once
and at least 50 files read.

Failing first: `hostswitch_test.go:72: internal/config/config.go defines the setter 0 times, want exactly 1`.

Full pre-commit list passed: gofmt, root build/vet/test, `-tags tmux`, `-tags quickstart`
(`127.0.0.1:8765` was free, 61s), golangci-lint, `go -C k8s` vet/test/build, no `go.sum`,
`grep -c require go.mod` prints 0.

For the next iteration:
- golangci-lint's gosec flags a bare `os.ReadFile(path)` (G304) even in tests. The repo's style is a
  trailing `//nolint:gosec // G304: ...` comment (see `cmd/crswd/config_cmd_test.go`).
- The variable is not synchronised. T3's `TestClusterBuildIsRunnable` must not use `t.Parallel()`,
  as the plan says, or it races every other test in that package.
- The shell sandbox refuses `$?` in a command ("a variable can't be checked"); use `&& echo OK || echo FAIL`.

Noticed, not fixed: none.

## Iteration 2 (T2, httpapi.NewForCluster)

Done: new `internal/httpapi/cluster.go` (`ClusterHooks`, `NewForCluster`, and `Server.PodRecord`,
which T3 needs, so T3 does not have to add it) and `cluster_test.go` with the five Design §2 tests.

Failing first: `internal/httpapi/cluster_test.go:41:12: undefined: NewForCluster` (the package did
not compile).

Full pre-commit list passed. `-tags quickstart` failed once on
`TestDashboardQuickstartStory2Cap` ("a stream opened after one closed = 429, want 200"), passed
alone and passed on the full rerun (46s). A flake under load; nothing in this change touches streams.

For the next iteration:
- `session.CreateRequest` needs a `Name` or `Create` fails with "a name is required".
- The `TestNewForClusterWiresHooks` transcript half drives `Supervisor.Sweep` on a claude session
  whose pane died (`fake.SetPaneCommand(name, "bash")`); a nil journal is safe there.
- golangci-lint (errcheck and gosec) rejects both a bare and a `_ =` discarded `Sweep` error; the
  test logs it instead.
- `Server.PodRecord` already exists in `cluster.go`, so Design §3 step 11's "otherwise add" branch
  is done. `ctl.SetDescriber` itself was not checked: grep `podctl` for it first.
- Heredocs with `cat >> file <<'EOF'` chained after `&&` tripped the shell parser; use Edit.

Noticed, not fixed: the `TestDashboardQuickstartStory2Cap` flake above.
