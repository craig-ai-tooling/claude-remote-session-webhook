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
