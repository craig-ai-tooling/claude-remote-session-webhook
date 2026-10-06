package manifest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Built from parts so this file does not contain what it forbids and need not
// be exempt from its own walk.
var forbiddenInFixtures = []string{"law" + "nmower", "." + "claude"}

// forbiddenHits reports every file under root that names a forbidden string,
// and how many files it read, so a walk that found nothing cannot pass.
func forbiddenHits(root string) (hits []string, read int, err error) {
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}
		b, rerr := os.ReadFile(p) //nolint:gosec // G304: p comes from walking fixed repo directories
		if rerr != nil {
			return rerr
		}
		read++
		low := strings.ToLower(string(b))
		for _, bad := range forbiddenInFixtures {
			if strings.Contains(low, bad) {
				hits = append(hits, p+" names "+bad)
			}
		}
		return nil
	})
	return hits, read, err
}

// SC-004: nothing the cluster build ships may point at the lab's namespace, its
// Secrets, or the operator's Claude home.
func TestFixturesNameNoLabNamespaceOrClaudeHome(t *testing.T) {
	t.Parallel()
	read := 0
	for _, root := range []string{"..", filepath.Join("..", "..", "..", "api"), filepath.Join("..", "..", "..", "internal", "admit")} {
		hits, n, err := forbiddenHits(root)
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
		read += n
		for _, h := range hits {
			t.Error(h)
		}
	}
	if read < 4 {
		t.Fatalf("walk read %d files, want at least 4", read)
	}
}

func TestForbiddenHitsCanFail(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	planted := filepath.Join(dir, "x.json")
	if err := os.WriteFile(planted, []byte(`{"namespace":"`+strings.ToUpper(forbiddenInFixtures[0])+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, read, err := forbiddenHits(dir)
	if err != nil {
		t.Fatal(err)
	}
	if read != 1 || len(hits) != 1 {
		t.Fatalf("read %d files with %d hits, want 1 and 1", read, len(hits))
	}
}
