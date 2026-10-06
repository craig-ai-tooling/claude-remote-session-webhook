# Progress: k8s-20-chart

Notebook for k8s-20, the Kubernetes-native install. Newest entry at the bottom.

## Iteration 0 (planning, 10/6/26)

Plan written by an operator-session planner against origin/main `50d7e9c`. Facts checked:

- helm 3.16.3 is on the planning VM. A chart with a `.json` file in `crds/` lints, and
  `helm template --include-crds` prints it. `(.Files.Get "files/rules.json" | fromJson).daemon | toJson`
  renders a rules array. Both were checked in a scratch chart on 10/6/26.
- `.github/workflows/ci.yml`: job `project` ("Build / test / lint") runs on
  `[self-hosted, linux, x64]`. The k8s steps end with `Build (k8s)`. Actions elsewhere in the file
  are pinned by SHA (golangci-lint) or by major tag (checkout, setup-go).
- `release.yml` cuts `v0.$(git rev-list --count HEAD)` on every push to main, on `ubuntu-latest`.
  FR-004: it must not change.
- Action SHAs, looked up 10/6/26 with `gh api repos/<r>/commits/<tag>`:
  azure/setup-helm v5.0.1 `9bc31f4ebc9c6b171d7bfbaa5d006ae7abdb4310`,
  docker/setup-buildx-action v4.4.1 `f87e5991a6d7451dcb8d9637bfbc97413f497069`,
  docker/login-action v4.6.0 `dbcb813823bdd20940b903addbd779551569679f`,
  docker/build-push-action v7.4.0 `c3c9e263c25d99ce0380d002d59b67737d91b0dc`,
  actions/checkout v7.0.1 `3d3c42e5aac5ba805825da76410c181273ba90b1`,
  actions/setup-go v7.0.0 `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e`.
  cloudflared latest is `2026.10.0`.
- Merging a `.github/workflows/**` change: the gh OAuth token on the VM lacks `workflow` scope
  (scopes: admin:public_key, gist, read:org, repo). The craig-ai-tooling App installation now
  has `workflows=write` (`~/code/ai-lawnmower/loop/ralph-app-token.sh --describe`, 10/6/26). PR
  #194 was merged by `app/craig-ai-tooling`. The operator pushes and merges with
  `tok=$(~/code/ai-lawnmower/loop/ralph-app-token.sh)` and `GH_TOKEN=$tok gh pr merge ...`,
  pushing over `https://x-access-token:$tok@github.com/...`. The loop does neither.
- The VM's docker is logged in to ghcr.io and Docker Hub, but publishing here is the workflow's
  job, through `GITHUB_TOKEN`.
- The loop's `.claude/settings.json` (origin/main) allows go, gofmt, golangci-lint and a few git
  verbs, and no helm. T1 adds `helm lint` and `helm template`. The loop cannot `cd`, so every
  helm command runs from the repo root against `deploy/chart`.
- `README.md` headings: `## Install` (~137) is the host install, and `## The two doors` (~399)
  follows it. The new section goes directly before `## The two doors`.

## NEEDS CLARIFICATION

None open.

BLOCKED: T1 cannot edit .claude/settings.json, the Edit tool was denied write permission (iteration 1, 10/6/26). The operator must grant the write or add "Bash(helm lint:*)" and "Bash(helm template:*)" to permissions.allow, one per line.

## Operator (10/6/26): T1 done

The loop cannot write `.claude/`, so the operator added `helm lint`, `helm template`, `bash -n` and `shellcheck` to `.claude/settings.json` and ticked T1. Ignore the BLOCKED line above. Next: T2.

## Iteration 2 (T2, generator writes the chart's CRD and rules)

Done: `manifest.ChartFiles()` returns `crds/agentsessions.crswd.craigcloud.io.json` (same bytes as
`crd.json`) and `files/rules.json` (`daemon`, `lease`, `reconciler`). `gen` writes them under
`deploy/chart/`. `drift_test.go` compares them with disk. `chart_test.go` holds the SC-005 walker
checks over the chart rules, a match test against the three `rbac-*.json` files, and a walker
that can fail.

Failing first: `go vet ./deploy/k8s/...` printed `chart_test.go:17:16: undefined: ChartFiles` (does
not compile). After the code, before `gen` ran, `TestCommittedChartFilesMatchChartFiles` failed with
`open ../../chart/files/rules.json: no such file or directory`.

Next iteration would waste time on:
- `rbac.go` is not in "Files touched", so the rules are not extracted into helpers there. `rulesOf`
  in `chart.go` reads item 0 of each builder's List and returns an error, because errcheck rejects
  `_` on the type assertions.
- `deploy/chart/` has only `crds/` and `files/` so far. T3 adds Chart.yaml and the rest.
- Gate steps 1 to 6, 8 and 9 passed. Step 7 (helm lint) starts at T3.

Noticed, not fixed: one `go test ./...` run failed `TestCodexSignedIn` in `internal/loginrelay` with
`text file busy` (fork/exec of a just-written fake codex). It passed on the next run. Outside this
plan's files.

## Iteration 3 (T3, chart skeleton)

Done: `Chart.yaml`, `values.yaml` (the Design §3 keys and defaults, one comment each), `.helmignore`,
`doc.go`, `templates/_helpers.tpl`, `templates/NOTES.txt`, and a placeholder `templates/daemon.yaml`
holding only the two guards.

Failing first: `helm lint deploy/chart --set sharedSecret.existingSecret=s` printed
`Error unable to check Chart.yaml file in chart: stat deploy/chart/Chart.yaml: no such file or directory`.
After: lint exits 0 (only `icon is recommended`). `helm template` without the secret fails with
`sharedSecret.existingSecret is required`; with `service.enabled=true` and no door it fails with
`service.enabled needs a browser door`; with `dashboardPassword.existingSecret=p` it renders (empty,
no workloads yet).

Next iteration would waste time on:
- `crswd.selectorLabels` and `crswd.labels` take a dict, not a bare component string: call them as
  `include "crswd.selectorLabels" (dict "root" . "component" "daemon")`. A Helm `include` takes one
  argument, and the helper needs both the release and the component.
- `daemon.yaml` renders nothing until T5. `helm template` of a valid install prints no output, and
  that is expected, not a bug. T4 adds the first rendered objects.
- No Go test was added in T3 (the plan's T3 verify is helm lint). `deploy/chart/doc.go` is the only
  Go file there, and `chart_test.go` arrives in T4.
- Gate steps 1 to 9 passed, including `helm lint` with the secret set.

## Iteration 4 (T4, namespaces, ServiceAccounts, RBAC)

Done: `templates/namespace.yaml` (reconciler namespace when `reconciler.createNamespace`) and
`templates/rbac.yaml` (SAs `crswd` and `crswd-reconciler`, Roles `crswd`, `crswd-reconciler`,
`crswd-reconciler-lease`, three RoleBindings). Every `rules:` line is a `.Files.Get "files/rules.json"`
expression. No ClusterRole. `deploy/chart/chart_test.go` holds `TestChartRBACReadsGeneratedRules`.

Failing first: `go test -run ChartRBAC ./deploy/chart` printed
`chart_test.go:24: read template rbac.yaml: open templates/rbac.yaml: no such file or directory`.
After: passes, and `helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s` renders
the Namespace, 2 SAs, 3 Roles and 3 RoleBindings.

Next iteration would waste time on:
- The test bans the substring `secrets` in every file under `templates/`, including NOTES.txt and
  comments. T5 must not write that word in a template (say "Secret" singular, or put the prose in values).
- `readTemplate` in `chart_test.go` is the helper T5's `TestChartWorkloadsAreLockedDown` should reuse.
- The Bash sandbox refuses heredocs and commands containing braces plus quotes. Write template files
  with the Write tool, and avoid `${PIPESTATUS[0]}` style expansions.
- Gate steps 1 to 9 passed (gosec G304 on the test's ReadFile needed a `//nolint:gosec` with a reason).
