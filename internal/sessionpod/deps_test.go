package sessionpod

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	goModPath = "../../go.mod"
	goSumPath = "../../go.sum"
)

// modulePath reads this repository's module path out of go.mod, so the import
// check below does not carry a second copy of it.
func modulePath(t *testing.T, mod []byte) string {
	t.Helper()

	for _, line := range strings.Split(string(mod), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest)
		}
	}
	t.Fatalf("%s names no module", goModPath)
	return ""
}

// TestNoDependenciesOutsideTheStandardLibrary holds the constraint this slice was
// written under: the in-pod side is standard library and this repository, and
// go.sum does not exist (docs/security.md §5).
//
// It reads two things, because each alone can be true of a package that is one
// edit from a dependency. go.mod having no require block is what a `go get` would
// break, and it covers everything transitively. The imports are read from the
// source, so the failure names the file and the path that brought a dependency
// in, rather than a require line somebody has to trace back.
func TestNoDependenciesOutsideTheStandardLibrary(t *testing.T) {
	t.Parallel()

	if _, err := os.Stat(goSumPath); err == nil {
		t.Errorf("%s exists, so something outside the standard library was imported", goSumPath)
	} else if !os.IsNotExist(err) {
		t.Errorf("stat %s: %v", goSumPath, err)
	}

	mod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read %s: %v", goModPath, err)
	}
	if strings.Contains(string(mod), "require") {
		t.Errorf("%s carries a require block:\n%s", goModPath, mod)
	}
	own := modulePath(t, mod)

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package's files: %v", err)
	}
	var sources int
	fset := token.NewFileSet()
	for _, file := range files {
		parsed, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		isTest := strings.HasSuffix(file, "_test.go")
		if !isTest {
			sources++
		}
		for _, spec := range parsed.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Fatalf("%s: unreadable import %s", file, spec.Path.Value)
			}
			first, _, _ := strings.Cut(path, "/")
			switch {
			case !strings.Contains(first, "."):
				// The standard library: no path in it has a dot in its first element.
			case path == own || strings.HasPrefix(path, own+"/"):
				// This repository.
			default:
				t.Errorf("%s imports %s, which is neither the standard library nor this repository", file, path)
			}
			// tmuxctl is the only place in the daemon that executes anything. This
			// package reaches tmux through it and never around it.
			if path == "os/exec" && !isTest {
				t.Errorf("%s imports os/exec; the pod's tmux commands go through internal/tmuxctl", file)
			}
		}
	}

	// A check over no files is a check that cannot fail. Before this package
	// exists it would find nothing to object to, and report that as a pass.
	if sources == 0 {
		t.Fatal("found no non-test source in this package, so nothing was checked")
	}
}
