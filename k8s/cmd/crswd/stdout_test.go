package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stdoutOwners are the functions allowed to name os.Stdout. The in-pod
// subcommands pane-loop, codex-conversation and has-transcript speak on stdout
// (podctl reads it over exec); main hands that one writer to dispatch, which
// passes it to them and to --version. The daemon path writes the audit trail
// through internal/audit and everything else to stderr.
var stdoutOwners = map[string]bool{
	"main": true,
}

// stdoutUses lists the functions in src that select os.Stdout and are not in
// owners.
func stdoutUses(t *testing.T, filename, src string, owners map[string]bool) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), filename, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	var bad []string
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || owners[fn.Name.Name] {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Stdout" {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "os" {
				bad = append(bad, fn.Name.Name)
			}
			return true
		})
	}
	return bad
}

func TestStdoutOnlyInTheInPodSubcommands(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"owner may use it", "package p\nimport \"os\"\nfunc main() { _ = os.Stdout }\n", nil},
		{"another function may not", "package p\nimport \"os\"\nfunc runDaemon() { _ = os.Stdout }\n", []string{"runDaemon"}},
		{"stderr is fine", "package p\nimport \"os\"\nfunc runDaemon() { _ = os.Stderr }\n", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stdoutUses(t, "x.go", tc.src, stdoutOwners)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("stdoutUses = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStdoutGuardOnThisPackage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	read := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(name)) //nolint:gosec // G304: a fixed directory listing of this package
		if err != nil {
			t.Fatal(err)
		}
		read++
		if bad := stdoutUses(t, name, string(raw), stdoutOwners); len(bad) > 0 {
			t.Errorf("%s names os.Stdout in %v; stdout carries the in-pod protocol and nothing else", name, bad)
		}
	}
	if read < 3 {
		t.Errorf("read %d source files, want at least 3", read)
	}
}
