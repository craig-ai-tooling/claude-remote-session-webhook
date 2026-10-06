# Validation contract: k8s-20-chart

k8s-20: the Helm chart, published images, and the README's Kubernetes install. Run every command
from the repository root on the milestone branch.

- The root gate is green: `go build ./... && go vet ./... && go test ./... && golangci-lint run` exits 0, `test ! -e go.sum` exits 0, and `grep -c require go.mod` prints `0`.
- The chart lints: `helm lint deploy/chart --set sharedSecret.existingSecret=s --set sessionNode=n` exits 0.
- The chart renders in the three shapes CI renders: `helm template t deploy/chart -n crswd --include-crds --set sharedSecret.existingSecret=s --set sessionNode=n` exits 0, and the same with `--set service.enabled=true --set dashboardPassword.existingSecret=p --set cloudflared.enabled=true --set cloudflared.tokenSecret=t` exits 0.
- The chart refuses an install without the shared secret: `helm template t deploy/chart -n crswd` exits non-zero and its stderr names `sharedSecret.existingSecret`.
- The chart's CRD and rules are the generator's: `go run ./deploy/k8s/gen && git status --porcelain deploy/k8s deploy/chart` prints nothing, and `cmp deploy/k8s/crd.json deploy/chart/crds/agentsessions.crswd.craigcloud.io.json` exits 0.
- No rule grants secrets, and nothing is cluster-wide but the CRD: `helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s --set sessionNode=n | grep -v secretKeyRef | grep -c -E 'ClusterRole|"secrets"'` prints `0`.
- A Service needs a browser door: `helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s --set sessionNode=n --set service.enabled=true` exits non-zero.
- Both runtimes are offered by default: `helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s --set sessionNode=n | grep -c 'codex=/usr/local/bin/codex'` prints `1`.
- The reconciler lives in its own namespace: `helm template t deploy/chart -n crswd --set sharedSecret.existingSecret=s --set sessionNode=n | grep -c 'namespace: crswd-reconciler'` prints at least `3`.
- The claim survives an uninstall: `grep -c 'helm.sh/resource-policy: keep' deploy/chart/templates/claim.yaml` prints `1`.
- CI renders the chart and a workflow publishes images and the chart: `grep -c 'helm lint deploy/chart' .github/workflows/ci.yml` prints `1`, `grep -c 'packages: write' .github/workflows/images.yml` prints `1`, and `git diff --name-only origin/main...HEAD -- .github/workflows/release.yml` prints nothing.
- Both installs are documented as first class: `grep -c '^## Install on Kubernetes' README.md` prints `1`, and `grep -c '^## Install with Helm' docs/k8s-mode.md` prints `1`.
