package manifest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const regenerate = "run `go run ./deploy/k8s/gen` from the repo root and commit the result"

// The committed JSON is what kubectl applies, so it is the file that must match
// the Go value, not the other way round.
func TestCommittedManifestsMatchFiles(t *testing.T) {
	t.Parallel()
	files, err := Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("..", name)) //nolint:gosec // G304: name comes from Files(), not input
		if err != nil {
			t.Errorf("read %s: %v; %s", name, err, regenerate)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("deploy/k8s/%s differs from Files(); %s", name, regenerate)
		}
	}
}

func TestCommittedChartFilesMatchChartFiles(t *testing.T) {
	t.Parallel()
	files, err := ChartFiles()
	if err != nil {
		t.Fatalf("ChartFiles: %v", err)
	}
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("..", "..", "chart", filepath.FromSlash(name))) //nolint:gosec // G304: name comes from ChartFiles(), not input
		if err != nil {
			t.Errorf("read deploy/chart/%s: %v; %s", name, err, regenerate)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("deploy/chart/%s differs from ChartFiles(); %s", name, regenerate)
		}
	}
}

// A JSON file nothing generates is a manifest nobody reviews as Go.
func TestNoStrayManifests(t *testing.T) {
	t.Parallel()
	files, err := Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	for _, pattern := range []string{"*.json", filepath.Join("crswd-next", "*.json")} {
		found, err := filepath.Glob(filepath.Join("..", pattern))
		if err != nil {
			t.Fatalf("glob: %v", err)
		}
		for _, p := range found {
			name := filepath.ToSlash(strings.TrimPrefix(p, ".."+string(filepath.Separator)))
			if _, ok := files[name]; !ok {
				t.Errorf("deploy/k8s/%s is not produced by Files(); delete it or add it to Files()", name)
			}
		}
	}
}
