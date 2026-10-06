package kube

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const k8sModulePath = "github.com/nctiggy/claude-remote-session-webhook/k8s"

// importsK8s parses only the import block, so a file that does not compile
// still gets checked. The prefix match also catches the module root itself.
func importsK8s(name string, src []byte) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
	if err != nil {
		return false, fmt.Errorf("parse imports of %s: %w", name, err)
	}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return false, fmt.Errorf("unquote import in %s: %w", name, err)
		}
		if strings.HasPrefix(path, k8sModulePath) {
			return true, nil
		}
	}
	return false, nil
}

func TestBoundaryDetectsK8sImport(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		src  string
		want bool
	}{
		"module root": {
			src:  "package x\nimport _ \"" + k8sModulePath + "\"\n",
			want: true,
		},
		"subpackage": {
			src:  "package x\nimport k \"" + k8sModulePath + "/internal/kube\"\n",
			want: true,
		},
		"grouped": {
			src:  "package x\nimport (\n\t\"fmt\"\n\t\"" + k8sModulePath + "/internal/kube\"\n)\n",
			want: true,
		},
		"stdlib only": {
			src:  "package x\nimport \"fmt\"\n",
			want: false,
		},
		"root module package": {
			src:  "package x\nimport _ \"github.com/nctiggy/claude-remote-session-webhook/internal/session\"\n",
			want: false,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := importsK8s("x.go", []byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("importsK8s = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBoundaryRootImportsNothingFromK8s(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	var read int
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			switch {
			case rel == "k8s", d.Name() == ".git", d.Name() == ".claude", d.Name() == "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		src, err := os.ReadFile(path) //nolint:gosec // path comes from walking this repo's own tree, not from input
		if err != nil {
			return err
		}
		read++
		bad, err := importsK8s(path, src)
		if err != nil {
			return err
		}
		if bad {
			t.Errorf("%s imports the cluster module; the host module must stay standard library only", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// An empty walk would pass vacuously, so a wrong root fails here.
	if read < 50 {
		t.Fatalf("read %d Go files under %s, want at least 50", read, root)
	}
	if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
		t.Fatal("root go.sum exists; only k8s/go.sum may carry dependencies")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
