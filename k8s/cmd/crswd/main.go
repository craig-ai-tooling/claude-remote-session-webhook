// Command crswd, the cluster build: the daemon and the reconciler for
// kubernetes mode, and the in-pod subcommands the session image runs (spec
// 017). It runs no other mode. A host is served by the release binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
)

// version is set by -ldflags "-X main.version=...".
var version = "dev"

func main() {
	// First, so that Runnable agrees with this binary everywhere it is asked.
	// The host binary never calls this, which is why it keeps refusing kubernetes.
	config.BuildKubernetesMode()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	code := dispatch(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// dispatch runs one command and returns the process's exit code.
func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return report(stderr, runDaemon(ctx, stderr))
	}
	rest := args[1:]
	switch args[0] {
	case "--version":
		say(stdout, "crswd %s (cluster build)\n", version)
		return 0
	case "session-pod":
		name, workdir, roots, err := parseSessionPodArgs(rest)
		if err != nil {
			say(stderr, "crswd: %v\n", err)
			return 2
		}
		return report(stderr, sessionpod.Run(name, workdir, roots))
	case "pane-loop":
		if len(rest) != 1 {
			sayln(stderr, "crswd: pane-loop takes one argument: the session name")
			return 2
		}
		return report(stderr, sessionpod.PaneLoop(rest[0], stdout))
	case "codex-conversation":
		if len(rest) != 1 {
			sayln(stderr, "crswd: codex-conversation takes one argument: the session name")
			return 2
		}
		return report(stderr, codexConversation(ctx, rest[0], stdout))
	case "has-transcript":
		if len(rest) != 4 {
			sayln(stderr, "crswd: has-transcript takes four arguments: name, harness, id, workdir")
			return 2
		}
		if sessionpod.HasTranscript(harness.Name(rest[1]), rest[2], rest[3], os.Environ()) {
			return 0
		}
		return 1
	case "unit":
		// FR-003: no systemd unit runs this build. The Helm chart installs and
		// upgrades it.
		sayln(stderr, "crswd: unit is refused in kubernetes mode: the Helm chart installs and upgrades this daemon")
		return 2
	case "reconcile":
		return report(stderr, runReconciler(ctx))
	default:
		say(stderr, "crswd: unknown command %q\n", args[0])
		return 2
	}
}

// report turns an error into an exit code, with the error on stderr.
func report(stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	say(stderr, "crswd: %v\n", err)
	return 1
}

// parseSessionPodArgs reads the arguments the reconciler's pod template writes:
// the session name and its working directory. The one root is where the claim's
// work directory is mounted, so no argument can widen it.
func parseSessionPodArgs(args []string) (name, workdir string, roots []config.ApprovedRoot, err error) {
	if len(args) != 2 {
		return "", "", nil, errors.New("session-pod takes two arguments: the session name and its working directory")
	}
	return args[0], args[1], []config.ApprovedRoot{{Path: sessionpod.WorkRoot}}, nil
}

// codexConversation prints the id of the Codex conversation the pane is in, or
// nothing when there is none.
func codexConversation(ctx context.Context, name string, stdout io.Writer) error {
	tmux, err := sessionpod.NewInPodExec()
	if err != nil {
		return err
	}
	id, err := sessionpod.CodexConversation(ctx, tmux, name, "/proc", session.CodexHome(os.Environ()))
	if err != nil {
		return err
	}
	if id != "" {
		_, err = fmt.Fprintln(stdout, id)
	}
	return err
}

// say and sayln write a report to stdout or stderr. The dropped error mirrors
// cmd/crswd/config_cmd.go: there is no answer to a failed write to the stream
// you would report the failure on, and check-blank flags `_, _ =` at each site.
func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) //nolint:errcheck // see the comment above.
}

func sayln(w io.Writer, line string) {
	_, _ = fmt.Fprintln(w, line) //nolint:errcheck // see the comment above.
}
