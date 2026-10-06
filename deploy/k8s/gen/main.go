// Command gen writes the manifests in deploy/k8s from the Go values in
// deploy/k8s/manifest. Run it from the repository root:
//
//	go run ./deploy/k8s/gen
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nctiggy/claude-remote-session-webhook/deploy/k8s/manifest"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run() error {
	files, err := manifest.Files()
	if err != nil {
		return fmt.Errorf("render manifests: %w", err)
	}
	for name, b := range files {
		// The name comes from manifest.Files, a closed set of constants, never
		// from input; 0600 matches what the drift test reads back.
		p := filepath.Join("deploy", "k8s", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, b, 0o600); err != nil { //nolint:gosec // G306/G304: constant names, repo-local output
			return fmt.Errorf("write %s: %w", p, err)
		}
	}
	chart, err := manifest.ChartFiles()
	if err != nil {
		return fmt.Errorf("render chart files: %w", err)
	}
	for name, b := range chart {
		p := filepath.Join("deploy", "chart", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, b, 0o600); err != nil { //nolint:gosec // G306/G304: constant names, repo-local output
			return fmt.Errorf("write %s: %w", p, err)
		}
	}
	return nil
}
