package main

// What the execution mode does to this command (spec 017, FR-001, FR-003): the
// `unit` subcommands refuse in kubernetes mode, run() skips the two host-only
// startup questions, and until the cluster build exists the binary refuses to
// start in that mode with a sentence saying why.

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
)

// TestUnitRefusesInKubernetesMode is FR-003's subcommand half: every spelling of
// `unit` refuses, names the mode, exits 1 and writes nothing to stdout.
//
// The usage checks sit behind the refusal on purpose, so a mistyped
// `crswd unit adpot` in a pod is told about the mode rather than sent to look
// for a subcommand that would refuse anyway. The host-mode half is the control:
// the same typo there is the usage error it always was.
//
// **Must fail when** any spelling gets past the refusal in kubernetes mode, or
// when host mode stops reaching its own checks.
func TestUnitRefusesInKubernetesMode(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{
		"check":             {"unit", "check"},
		"adopt":             {"unit", "adopt"},
		"no subcommand":     {"unit"},
		"a subcommand typo": {"unit", "adpot"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			if code := runUnitCommandIn(config.ExecutionModeKubernetes, &out, &errOut, args); code != 1 {
				t.Errorf("runUnitCommandIn(kubernetes, %v) = %d; want 1, the failure exit", args, code)
			}
			if out.Len() != 0 {
				t.Errorf("a refused unit command wrote to stdout: %s", out.String())
			}
			for _, want := range []string{"unit", "disabled in kubernetes mode"} {
				if !strings.Contains(errOut.String(), want) {
					t.Errorf("the refusal does not say %q:\n%s", want, errOut.String())
				}
			}
		})
	}

	t.Run("host mode is not refused", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		if code := runUnitCommandIn(config.ExecutionModeHost, &out, &errOut, []string{"unit", "adpot"}); code != 2 {
			t.Errorf("runUnitCommandIn(host, unit adpot) = %d; want the usage exit 2 it has always returned", code)
		}
		if strings.Contains(errOut.String(), "kubernetes") {
			t.Errorf("host mode mentioned kubernetes:\n%s", errOut.String())
		}
	})
}

// TestUnitReadsTheModeTheWayTheDaemonDoes holds the half of the refusal that is
// easy to forget: runUnitCommand has no Config, since a full load demands a
// shared secret a shell on a quiet host does not carry, so it must find the mode
// itself. The environment first, then the file, the way every setting resolves,
// and an unknown word refused as it is at start.
//
// Not parallel: it sets process environment.
//
// **Must fail when** `crswd unit check` runs on a host whose environment or
// configuration file says kubernetes.
func TestUnitReadsTheModeTheWayTheDaemonDoes(t *testing.T) {
	file := func(t *testing.T, contents string) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config")
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatalf("write the configuration file: %v", err)
		}
		t.Setenv("CRSW_CONFIG_FILE", path)
	}

	cases := []struct {
		name     string
		mode     string // CRSW_EXECUTION_MODE
		file     string
		args     []string
		wantCode int
		wantErr  string
	}{
		{name: "the environment says kubernetes", mode: "kubernetes", args: []string{"unit", "check"}, wantCode: 1, wantErr: "disabled in kubernetes mode"},
		{name: "the file says kubernetes", file: "execution_mode = kubernetes\n", args: []string{"unit", "check"}, wantCode: 1, wantErr: "disabled in kubernetes mode"},
		{name: "the environment beats the file", mode: "host", file: "execution_mode = kubernetes\n", args: []string{"unit", "adpot"}, wantCode: 2, wantErr: "unknown unit subcommand"},
		{name: "nothing says anything", args: []string{"unit", "adpot"}, wantCode: 2, wantErr: "unknown unit subcommand"},
		{name: "an unknown word is refused", mode: "k8s", args: []string{"unit", "check"}, wantCode: 1, wantErr: "is not host or kubernetes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CRSW_EXECUTION_MODE", tc.mode)
			// A file that names nothing, so the operator's real one is never read.
			file(t, "")
			if tc.file != "" {
				file(t, tc.file)
			}

			var out, errOut bytes.Buffer
			if code := runUnitCommand(&out, &errOut, tc.args); code != tc.wantCode {
				t.Errorf("runUnitCommand(%v) = %d; want %d\n%s", tc.args, code, tc.wantCode, errOut.String())
			}
			if !strings.Contains(errOut.String(), tc.wantErr) {
				t.Errorf("stderr does not say %q:\n%s", tc.wantErr, errOut.String())
			}
			if out.Len() != 0 {
				t.Errorf("stdout was written to: %s", out.String())
			}
		})
	}
}

// TestTheBinaryRefusesToStartInKubernetesMode is the sentence the task asks for,
// driven through run() itself: the configuration loads, the mode says
// kubernetes, and the next thing that happens is a refusal in words.
//
// Everything the process could touch is isolated, because a failure of this test
// is a daemon that started. HOME and every XDG directory are temporary; the
// listen address is one this test is already holding, so a run that got past the
// refusal would fail to bind rather than serve; and the context is short.
//
// It also asserts that nothing was left behind in the directories the startup
// sequence writes to, which is what "before anything touches this host" means.
//
// Not parallel: it sets process environment, which the loader reads.
//
// **Must fail when** the binary starts in kubernetes mode, or refuses without
// saying why.
func TestTheBinaryRefusesToStartInKubernetesMode(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold a port: %v", err)
	}
	t.Cleanup(func() {
		if err := taken.Close(); err != nil {
			t.Logf("release the held port: %v", err)
		}
	})

	for name, value := range map[string]string{
		"HOME":                       home,
		"XDG_CONFIG_HOME":            filepath.Join(home, "config"),
		"XDG_STATE_HOME":             filepath.Join(home, "state"),
		"XDG_CACHE_HOME":             filepath.Join(home, "cache"),
		"CRSW_CONFIG_FILE":           filepath.Join(home, "config", "absent"),
		"CRSW_SHARED_SECRET":         strings.Repeat("k", config.MinSecretBytes),
		"CRSW_DASHBOARD_PASSWORD":    strings.Repeat("p", config.MinDashboardPasswordLen),
		"CRSW_ALLOWED_ROOTS":         root,
		"CRSW_LISTEN":                taken.Addr().String(),
		"CRSW_EXECUTION_MODE":        "kubernetes",
		"CRSW_DESTROY_ON_SHUTDOWN":   "",
		"CRSW_SESSION_ENVIRONMENT":   "",
		"CRSW_ACCESS_ENABLED":        "",
		"CRSW_ACCESS_TEAM_DOMAIN":    "",
		"CRSW_ACCESS_AUD":            "",
		"CRSW_ACCESS_ALLOWED_EMAILS": "",
	} {
		t.Setenv(name, value)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = run(ctx)

	if err == nil {
		t.Fatal("run returned nil in kubernetes mode; want a refusal")
	}
	if !errors.Is(err, config.ErrKubernetesModeUnbuilt) {
		t.Fatalf("run = %v; want %v", err, config.ErrKubernetesModeUnbuilt)
	}
	sentence := err.Error()
	for _, want := range []string{`"kubernetes"`, "not built into this binary", "refuses to start"} {
		if !strings.Contains(sentence, want) {
			t.Errorf("the refusal does not say %q: %s", want, sentence)
		}
	}
	if strings.Contains(sentence, "\n") {
		t.Errorf("the refusal is more than one line: %q", sentence)
	}

	left, readErr := os.ReadDir(home)
	if readErr != nil {
		t.Fatalf("read the isolated home: %v", readErr)
	}
	if len(left) != 0 {
		t.Errorf("a refused start left %d entries in the directories a start writes to", len(left))
	}
}

// TestKubernetesModeSkipsTheHostStartupQuestions is FR-003's startup half, as a
// structural assertion in the shape this package's other main.go tests take.
//
// run() refuses in kubernetes mode before it reaches them today, so a behaviour
// test cannot see the skip: it is the code that becomes reachable when the
// cluster build removes the refusal, and the moment to find out that the update
// staging sweep or the unit report still runs in a pod is not that one. What
// this reads is the syntax: each of the two calls sits inside an `if` whose
// condition is that the mode is not kubernetes.
//
// The dependency probe is not among them, and that is deliberate: it is the one
// call main.go must make exactly once (TestStartupDiagnosticsGoToStderr), and
// the skip lives inside it, where internal/config asserts it.
//
// **Must fail when** either call is made outside such a guard, or when the sweep
// or the report is dropped from run() and this test would be asserting about
// nothing.
func TestKubernetesModeSkipsTheHostStartupQuestions(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var run *ast.FuncDecl
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "run" {
			run = fn
		}
	}
	if run == nil {
		t.Fatal("main.go has no run()")
	}

	v := &guardVisitor{fset: fset, found: map[string]int{}}
	ast.Walk(v, run.Body)

	for _, name := range []string{"Sweep", "sayWhatBecameOfTheUnit"} {
		if v.found[name] == 0 {
			t.Errorf("run() no longer calls %s, so this test is asserting about nothing", name)
		}
	}
	for _, at := range v.unguarded {
		t.Errorf("%s runs in kubernetes mode; it belongs inside an `if !cfg.ExecutionMode.Kubernetes()`", at)
	}
}

// guardVisitor walks run() carrying whether the node under it is inside an
// `if !….Kubernetes() { }` body.
type guardVisitor struct {
	fset      *token.FileSet
	guarded   bool
	found     map[string]int
	unguarded []string
}

func (v *guardVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.IfStmt:
		if !notKubernetes(n.Cond) {
			return v
		}
		inside := *v
		inside.guarded = true
		ast.Walk(&inside, n.Body)
		if n.Else != nil {
			ast.Walk(v, n.Else)
		}
		return nil
	case *ast.CallExpr:
		name := ""
		switch fn := n.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		}
		if name == "Sweep" || name == "sayWhatBecameOfTheUnit" {
			v.found[name]++
			if !v.guarded {
				v.unguarded = append(v.unguarded, name+" at "+v.fset.Position(n.Pos()).String())
			}
		}
	}
	return v
}

// notKubernetes recognises `!<anything>.Kubernetes()`.
func notKubernetes(cond ast.Expr) bool {
	not, ok := cond.(*ast.UnaryExpr)
	if !ok || not.Op != token.NOT {
		return false
	}
	call, ok := not.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Kubernetes"
}
