package image

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + rel) //nolint:gosec // G304: fixed repo-relative paths named in this file.
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// Every base image is pinned to a digest, with the tag kept beside it.
func TestSessionImageBaseIsDigestPinned(t *testing.T) {
	t.Parallel()
	df := readRepoFile(t, "deploy/session-image/Dockerfile")
	re := regexp.MustCompile(`(?m)^FROM --platform=linux/amd64 \S+:\S+@sha256:[0-9a-f]{64}$`)
	if !re.MatchString(df) {
		t.Error("session image FROM is not image:tag@sha256:<digest>")
	}
}

// A rerun must not overwrite a version tag, and the path filter must cover what
// the cluster binary compiles from the root module.
func TestImagesWorkflowIsImmutableAndFiltersInputs(t *testing.T) {
	t.Parallel()
	wf := readRepoFile(t, ".github/workflows/images.yml")
	if !strings.Contains(wf, "if: github.ref == 'refs/heads/main'") {
		t.Error("images.yml has no github.ref == 'refs/heads/main' gate on the publish job")
	}
	for _, want := range []string{"imagetools inspect \"ghcr.io/craig-ai-tooling/$img:$VERSION\"", "steps.exists.outputs.crswd != 'true'", "steps.exists.outputs.session != 'true'", "helm show chart oci://ghcr.io/craig-ai-tooling/charts/crswd"} {
		if !strings.Contains(wf, want) {
			t.Errorf("images.yml never checks for an existing tag with %q", want)
		}
	}
	for _, p := range []string{"'web/**'", "'go.mod'", "'go.sum'", "'api/**'", "'internal/**'", "'deploy/k8s/**'", "'k8s/**'"} {
		if !strings.Contains(wf, "      - "+p) {
			t.Errorf("images.yml paths lack %s", p)
		}
	}
}
