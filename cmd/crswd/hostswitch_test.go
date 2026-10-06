package main

// The host binary never builds kubernetes mode (spec 017, FR-004). The switch is a
// setter only k8s/cmd/crswd calls, so a call anywhere else in the root module would
// let the host binary start sessions in pods it cannot reach. A compiler cannot
// say that; a walk of the source can.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostNeverBuildsKubernetesMode(t *testing.T) {
	t.Parallel()

	// Two parts, so this file does not match its own search.
	call := "BuildKubernetes" + "Mode("
	definition := "func " + call + ")"
	const definer = "internal/config/config.go"

	root := filepath.Join("..", "..")
	var read, defined int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "testdata":
				return filepath.SkipDir
			}
			if rel == "k8s" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path) //nolint:gosec // G304: the path comes from walking this repo's own tree.
		if err != nil {
			return err
		}
		read++
		for _, line := range strings.Split(string(b), "\n") {
			if !strings.Contains(line, call) {
				continue
			}
			if rel == definer && strings.Contains(line, definition) {
				defined++
				continue
			}
			t.Errorf("%s calls the cluster switch: %q", rel, strings.TrimSpace(line))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if read < 50 {
		t.Fatalf("read %d Go files, want at least 50: the walk is not finding the tree", read)
	}
	if defined != 1 {
		t.Fatalf("%s defines the setter %d times, want exactly 1", definer, defined)
	}
}
