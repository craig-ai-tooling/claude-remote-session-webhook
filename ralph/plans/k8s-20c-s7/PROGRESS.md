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

## Iteration 3 (T3, the cluster binary): stopped, needs a decision

Wrote `k8s/cmd/crswd/` (`main.go`, `daemon.go`, `reconcile.go`, `main_test.go`, `daemon_test.go`). Left UNCOMMITTED and untracked on disk so the next iteration can finish it. `go -C k8s test ./cmd/crswd/... -v` passes (9 tests) and `go -C k8s build -o /dev/null ./cmd/crswd` exits 0.

Failing first: `k8s/cmd/crswd/daemon_test.go:..: undefined: runDaemon` (the package did not compile).

Blocked at the root pre-commit step: `go test ./...` fails in `cmd/crswd`:

    --- FAIL: TestDiagnosticsGoToStderr (0.10s)
        main_test.go:121: ../../k8s/cmd/crswd/main.go:30:37: k8s/cmd/crswd/main.go writes to standard output, which carries the audit trail and nothing else.

That test (`cmd/crswd/main_test.go`, `parseTheDaemon`) walks the whole root module including `k8s/`, and allows `os.Stdout` only in `internal/audit/audit.go` and as an argument to `runConfigCommand`, `printVersion`, `runKeygen`, `runUnitCommand`. The plan forbids editing `cmd/crswd/` except the one new test, but Design §3 needs stdout for `--version`, `pane-loop` frames, and `codex-conversation`.

Also learned: `config.Load()` reads the real config file and ambient CRSW_ variables, so `daemon_test.go` sets `CRSW_CONFIG_FILE` empty and `XDG_CONFIG_HOME` to a temp dir, and blanks the Access variables.

## NEEDS CLARIFICATION

Task T3 ("The cluster binary `k8s/cmd/crswd` per Design §3"): `TestDiagnosticsGoToStderr` rejects `os.Stdout` in `k8s/cmd/crswd/main.go`, and `cmd/crswd/` is on the never-touch list. Which is intended? (a) allow editing `cmd/crswd/main_test.go` to exempt `k8s/cmd/crswd/main.go` (the in-pod subcommands legitimately own stdout; the daemon path writes the audit trail via `internal/audit`); (b) have it skip `k8s/` like `hostswitch_test.go` does; (c) a stdout writer built without the `os.Stdout` selector (`os.NewFile`), which evades the guard rather than satisfying it. Recommendation: (a).

## Operator decision (10/6/26): T3 and the stdout guard

Answer: neither (a) nor (c). `TestDiagnosticsGoToStderr` guards the host daemon, and `parseTheDaemon`
(cmd/crswd/main_test.go) walks every `.go` file under the root, which now includes a separate module.
Do this as part of T3, in this order:

1. In `parseTheDaemon`'s `WalkDir` callback, for a directory other than `moduleRoot`, return
   `fs.SkipDir` when `filepath.Join(path, "go.mod")` exists (`os.Stat` error nil). Comment: a nested
   module is not compiled into this daemon. This edit to `cmd/crswd/main_test.go` is allowed for
   this one purpose; touch nothing else under `cmd/crswd/`.
2. Add `k8s/cmd/crswd/stdout_test.go`: parse every non-test `.go` file in `k8s/cmd/crswd`, and fail on
   any `os.Stdout` selector outside the functions that run the in-pod subcommands `pane-loop`,
   `codex-conversation` and `has-transcript` (their protocol is stdout, read by podctl over exec).
   List those function names in one map in the test. Prove it can fail with a table case over a
   synthetic source string that uses `os.Stdout` in another function.
3. Verify: `go test ./cmd/crswd -run DiagnosticsGoToStderr` and `go -C k8s test ./cmd/crswd -run Stdout -v` pass.


## Iteration 4 (T3, the cluster binary): done

The files from Iteration 3 were already committed by the loop's sweep. This iteration applied the
operator decision: `parseTheDaemon` in `cmd/crswd/main_test.go` now returns `fs.SkipDir` for a
nested module (a directory other than the root that holds a `go.mod`), and the new
`k8s/cmd/crswd/stdout_test.go` guards `os.Stdout` in the cluster binary. Its table case
(`another function may not`) proves it can fail; `TestStdoutGuardOnThisPackage` runs it on the real sources.

Failing first: `main_test.go:121: ../../k8s/cmd/crswd/main.go:30:37: k8s/cmd/crswd/main.go writes to standard output` (`TestDiagnosticsGoToStderr`, recorded in Iteration 3, before the skip).

Full pre-commit list passed: gofmt, root build/vet/test, `-tags tmux`, `-tags quickstart`
(`127.0.0.1:8765` free, 55s), golangci-lint (0 issues), `go -C k8s` vet/test/build, no `go.sum`,
`grep -c require go.mod` prints 0. Task verify: 11 tests pass in `k8s/cmd/crswd`, build exits 0.

One judgement call: the decision says `os.Stdout` is allowed in the functions that run
`pane-loop`, `codex-conversation` and `has-transcript`. Those take a `stdout io.Writer` from
`dispatch`, so the only `os.Stdout` selector is in `main`, which hands it to `dispatch`. The
allow-map in `stdout_test.go` therefore holds `main`. Narrowing it would mean `main` avoiding
`os.Stdout`, which only evades the guard.

For the next iteration:
- T4 edits `internal/sessionpod/dockerfile_test.go`; read it before writing `deploy/image/dockerfile_test.go`.
- `go -C k8s` works for the cluster module; the sandbox refuses `(...)` groups, so run commands one per call.

Noticed, not fixed: none.

## Iteration 5 (T4, images): written, NOT ticked

Wrote `deploy/image/{Dockerfile,doc.go,dockerfile_test.go}`, `deploy/session-image/fetch-codex.sh`,
and edited `deploy/session-image/Dockerfile` (second COPY, new tag scheme, build block, CA note) and
`internal/sessionpod/dockerfile_test.go` (two COPY lines, new tag string).

Failing first: `internal/sessionpod/dockerfile_test.go:69: COPY = ["COPY --chmod=0755 crswd /usr/local/bin/crswd"], want exactly the crswd binary then the codex binary`, and for the new package `dockerfile_test.go:43: read Dockerfile: open Dockerfile: no such file or directory`.

Passing: `go test -run Dockerfile ./internal/sessionpod ./deploy/image -v`. Full pre-commit list passed
(`-tags quickstart` failed once on the known `TestDashboardQuickstartStory2Cap` flake, passed on the
rerun, 120s; `127.0.0.1:8765` was free).

Not done, why: the sandbox refused `bash -n deploy/session-image/fetch-codex.sh` (the other half of
T4's verify) and `chmod +x deploy/session-image/fetch-codex.sh`. I did not try another interpreter to
get around it. So the script's syntax is unchecked and the file is mode 0644, and the Dockerfile
build block calls it as `deploy/session-image/fetch-codex.sh "$BIN"`. T4 stays `- [ ]`.

For the next iteration (an operator or a session with those two permitted):
- Run `bash -n deploy/session-image/fetch-codex.sh`, then `chmod +x` it (or `git update-index --chmod=+x`). If both pass, tick T4; nothing else is left in it.
- Do not rewrite the Dockerfiles or tests; they are done and green.

Noticed, not fixed: the quickstart cap flake again (2nd time, Iterations 2 and 5).

BLOCKED: T4 needs `bash -n deploy/session-image/fetch-codex.sh` and `chmod +x` on it, and the sandbox refuses both; an operator must run them, then tick T4.

## Operator (10/6/26): T4 closed

`bash -n` and shellcheck pass on fetch-codex.sh; it is now mode 0755; a real run fetched the asset, the sha matched, and the binary printed `codex-cli 0.153.4`. T4 ticked. Ignore the BLOCKED line above. Next: T5.

## Iteration 6 (T5, crswd-next manifests): done

Wrote `deploy/k8s/manifest/crswdnext.go` and `crswdnext_test.go`; `Files()` merges the six
`crswd-next/*.json` entries (a new `render` helper shared with the S2 objects); `gen/main.go`
creates the directory; `drift_test.go` globs `crswd-next/` too. Generated with `go run ./deploy/k8s/gen`.

Failing first: `crswdnext_test.go:143:18: undefined: DaemonImage` (does not compile before the change).

Task verify `go test ./deploy/k8s/...` passes. Full pre-commit list passed (`-tags quickstart` failed
once on the known `TestDashboardQuickstartStory2Cap` flake, passed on rerun; `127.0.0.1:8765` free).

For the next iteration:
- `fixtures_test.go` bans the literal `.claude` and `lawnmower` in every file under `deploy/k8s`,
  tests included; build such strings from two parts.
- gosec G101 fires on a map literal that pairs `*_SECRET` env names with Secret names; nolint is in the test.
- Bash refuses `$?` and `$VAR` in a command line here; write literal paths.

Noticed, not fixed: the quickstart cap flake (3rd time, Iterations 2, 5, 6).

## Iteration 7 (T6, docs/k8s-mode.md): done

Wrote `docs/k8s-mode.md` with the seven Design §6 sections, in order. No em-dash. The deployment table is copied from spec 017. The build commands match the two Dockerfile headers.

Failing first: `grep -c "^## " docs/k8s-mode.md` printed `grep: docs/k8s-mode.md: No such file or directory` before the file existed (a doc has no unit test; the verify is the heading count, now 7).

Verify: heading count 7, `go test ./...` exits 0. Full pre-commit list passed (`-tags quickstart` passed first time, 44s, port free).

Deviation from Design §6 item 4: the plan lists the secret before `kubectl apply`, but the namespace must exist first, so step 1 applies `crswd-next/namespaces.json`. The doc also names `placeholder-credentials.json` (empty `claude-credentials`) in step 4.

For the next iteration (T7):
- Read VALIDATION_CONTRACT.md and run every command in it; the pre-commit list above is the same set, plus whatever the contract adds.
- The quickstart cap flake (`TestDashboardQuickstartStory2Cap`) did not fire this time; if it does, rerun alone.

Noticed, not fixed: none.

## Iteration 8 (T7, validation contract): done

Ran every command in VALIDATION_CONTRACT.md on the branch. Results:
- `test ! -e go.sum` ok; `grep -c require go.mod` printed `0`.
- Root gate: gofmt clean, build, vet, `go test ./...`, `-tags tmux`, `-tags quickstart ./cmd/crswd` (48s, port free), `golangci-lint run` (0 issues): all exit 0.
- Cluster module: `go -C k8s vet`, `test`, `build` exit 0.
- `Kubernetes|HostNeverBuildsKubernetes` pass in `./cmd/crswd` and `./internal/config`.
- `ClusterBuildIsRunnable` and `DaemonRefusesHostConfig` pass in `k8s/cmd/crswd`.
- `NewForCluster` tests pass (5).
- `grep -c '^COPY'` session Dockerfile printed `2`; the Codex sha256 appears `1` time in fetch-codex.sh.
- `-run Dockerfile ./deploy/image ./internal/sessionpod` pass.
- `go run ./deploy/k8s/gen` then `git status --porcelain deploy/k8s` printed nothing.
- `grep -c REPLACE deploy/k8s/crswd-next/daemon.json` printed `1`.
- `grep -rniE 'lawnmower|\.claude' deploy/k8s/` printed nothing, exit 1.
- `grep -c '^## ' docs/k8s-mode.md` printed `7`.

Not run: images, cluster acceptance (operator-run per the plan). The cluster module is not linted here (CI does it).

RALPH_COMPLETE
