package httpapi

// What the execution mode switches off in the server (spec 017, FR-003): the
// sign-in relay, the release feed, the update and restart routes, and the
// session journal. Each is asserted twice, once in kubernetes mode and once in
// host mode, because a check that only looked at the new mode would pass
// through a change that broke the old one, and host mode is what SC-003 says
// must not move.

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/auth"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
)

// modeConfig is testConfig in the mode under test, with the one default start
// command the sign-in relay is built from. testConfig sets none, and a relay is
// only wired for a daemon that has something to sign in with.
func modeConfig(mode config.ExecutionMode) *config.Config {
	cfg := testConfig(loopbackListen)
	cfg.ExecutionMode = mode
	cfg.StartCommands = config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: "claude --dangerously-skip-permissions",
	})
	return cfg
}

// TestKubernetesModeWiresNoRelayAndNoReleaseFeed is FR-003 for the two
// collaborators New wires and NewWith does not: a relay drives `claude auth
// login` in a tmux window on this host, and the feed offers this binary a newer
// copy of itself. Neither means anything in a pod.
//
// The host-mode half is the control. It has to see both wired, or "nil in
// kubernetes mode" would hold on a build that never wired them anywhere.
//
// **Must fail when** kubernetes mode wires either, or host mode stops wiring
// either.
func TestKubernetesModeWiresNoRelayAndNoReleaseFeed(t *testing.T) {
	t.Parallel()

	t.Run("kubernetes mode", func(t *testing.T) {
		t.Parallel()

		srv, err := New(modeConfig(config.ExecutionModeKubernetes))
		if err != nil {
			t.Fatalf("New = _, %v; want a server", err)
		}
		if srv.signin != nil {
			t.Errorf("signin = %T; want nil, since there is no tmux window on this host to sign in from", srv.signin)
		}
		if srv.releaseFeed != nil {
			t.Error("releaseFeed is wired; want nil, since a pod's binary is an image and there is nothing to update")
		}
	})

	t.Run("host mode", func(t *testing.T) {
		t.Parallel()

		srv, err := New(modeConfig(config.ExecutionModeHost))
		if err != nil {
			t.Fatalf("New = _, %v; want a server", err)
		}
		if srv.signin == nil {
			t.Error("signin is nil in host mode; the mode switch has switched off the host's own sign-in")
		}
		if srv.releaseFeed == nil {
			t.Error("releaseFeed is nil in host mode; the mode switch has switched off the host's own updates")
		}
	})
}

// TestKubernetesModeHasNoJournal is the journal half of the rule the plan states
// as one sentence: the object is the record, so ReplayJournal and Adopt can never
// both revive one session.
//
// It creates a session through the manager NewWith built and looks for the file
// the journal would have written. Kubernetes mode goes first and must leave no
// file at the path; host mode then writes to that same path, which is what makes
// the absence the mode's doing and not a path that could never be written.
//
// Not parallel: the journal's path is derived from the process environment, and
// this pins CRSW_CONFIG_FILE so that both modes name the same place.
//
// **Must fail when** kubernetes mode gives the manager a journal, or host mode
// stops.
func TestKubernetesModeHasNoJournal(t *testing.T) {
	t.Setenv("CRSW_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))

	createOne := func(mode config.ExecutionMode) string {
		t.Helper()

		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("resolve the fixture root: %v", err)
		}
		repo := filepath.Join(root, "repo")
		if err := os.Mkdir(repo, 0o750); err != nil {
			t.Fatalf("create the working directory: %v", err)
		}
		cfg := modeConfig(mode)
		cfg.Roots = []config.ApprovedRoot{{Path: root}}

		srv, err := NewWith(cfg, newSessionFixture(t).tmux, audit.NewTo(io.Discard, func() time.Time { return testTime }))
		if err != nil {
			t.Fatalf("NewWith(%q) = _, %v; want a server", mode, err)
		}
		if _, _, err := srv.sessions.Create(context.Background(), session.CreateRequest{
			Owner: auth.CallerOperator, Name: "journal-" + string(mode), WorkDir: repo,
		}); err != nil {
			t.Fatalf("Create in %s mode = _, _, %v; want a session", mode, err)
		}
		return config.JournalPath(os.Getenv, cfg.Listen)
	}

	path := createOne(config.ExecutionModeKubernetes)
	if path == "" {
		t.Fatal("the journal path is empty, so this test would pass on any build")
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a create in kubernetes mode left a journal at %s (stat: %v); the object is the only record there", path, err)
	}

	if got := createOne(config.ExecutionModeHost); got != path {
		t.Fatalf("host mode names the journal %q and kubernetes mode %q", got, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("a create in host mode wrote no journal at %s: %v", path, err)
	}
}

// TestKubernetesModeWiresNoUpdatePath holds the wiring under the routes: with
// the mode set, NewWith leaves every update collaborator absent, so the routes'
// own refusal below is not the only thing between a kubernetes daemon and a
// download.
//
// **Must fail when** kubernetes mode builds the live update path, or host mode
// stops building it.
func TestKubernetesModeWiresNoUpdatePath(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		mode  config.ExecutionMode
		wired bool
	}{
		{config.ExecutionModeKubernetes, false},
		{config.ExecutionModeHost, true},
	} {
		t.Run(string(tc.mode), func(t *testing.T) {
			t.Parallel()

			srv, err := NewWith(modeConfig(tc.mode), newSessionFixture(t).tmux, audit.NewTo(io.Discard, func() time.Time { return testTime }))
			if err != nil {
				t.Fatalf("NewWith = _, %v; want a server", err)
			}
			if got := srv.updates.wired(); got != tc.wired {
				t.Errorf("the update path is wired = %t in %s mode; want %t", got, tc.mode, tc.wired)
			}
			if got := srv.updates.restartable(); got != tc.wired {
				t.Errorf("the restart is possible = %t in %s mode; want %t", got, tc.mode, tc.wired)
			}
		})
	}
}

// TestTheUpdateRouteRefusesInKubernetesMode is FR-003's route half. The fake
// behind it is a complete, working update path, which is what makes the test
// mean something: the refusal has to come from the mode, since nothing else is
// missing.
//
// It answers with the ordinary update refusal and puts the mode's own reason on
// the record. The banner is the existing one on purpose — a new outcome code is
// a new sentence in a closed vocabulary, and this route's job is to refuse.
//
// **Must fail when** the route reaches any step of the update in kubernetes
// mode, when it is refused for another reason, or when a request that skipped
// the confirming step is told about the confirmation instead of the mode.
func TestTheUpdateRouteRefusesInKubernetesMode(t *testing.T) {
	t.Parallel()

	for name, confirmed := range map[string]bool{"a confirmed request": true, "an unconfirmed one": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			d := newUpdateDoor(t)
			d.cfg.ExecutionMode = config.ExecutionModeKubernetes
			form := d.confirmed(t)
			if !confirmed {
				form.Del(fieldConfirm)
			}

			w := d.post(t, form)

			wantOutcome(t, w, wantUpdateRefusedOutcome)
			if !d.steps.untouched() {
				t.Errorf("an update reached the update path in kubernetes mode: %s", d.steps.reached())
			}
			rec := d.record(t, audit.Deny)
			if got, want := rec["reason"], errUpdateDisabledInKubernetes.Error(); got != want {
				t.Errorf("reason = %v; want %q", got, want)
			}
		})
	}
}

// TestTheUpdateRouteStillUpdatesInHostMode is the control for the test above:
// the same door, the same fake, the mode set to the host. Without it a route
// that refused everything would pass.
func TestTheUpdateRouteStillUpdatesInHostMode(t *testing.T) {
	t.Parallel()

	d := newUpdateDoor(t)
	d.cfg.ExecutionMode = config.ExecutionModeHost

	d.post(t, d.confirmed(t))

	if len(d.steps.staged) == 0 {
		t.Errorf("a confirmed update in host mode never reached the update path: %s", d.steps.reached())
	}
	d.steps.waitForExit(t)
}

// TestTheRestartRouteRefusesInKubernetesMode is the restart's half of the same
// rule. A restart is the update path's tail: it ends the process and relies on a
// service manager to bring it back, and in a pod that is the cluster's job, done
// to the object rather than by a button.
//
// **Must fail when** the restart ends the process in kubernetes mode.
func TestTheRestartRouteRefusesInKubernetesMode(t *testing.T) {
	t.Parallel()

	d := newRestartDoor(t)
	d.cfg.ExecutionMode = config.ExecutionModeKubernetes

	w := d.post(t, d.confirmed(t))

	wantOutcome(t, w, wantRestartRefusedOutcome)
	if d.steps.exits != 0 {
		t.Errorf("the process was told to exit %d times in kubernetes mode", d.steps.exits)
	}
	rec := d.record(t, audit.Deny)
	if got, want := rec["reason"], errRestartDisabledInKubernetes.Error(); got != want {
		t.Errorf("reason = %v; want %q", got, want)
	}
}

// TestEveryKubernetesRefusalNamesTheMode is the sentence the task asks for, as
// a property of the strings: whoever reads the trail is told the mode and not
// merely that something was unwired.
func TestEveryKubernetesRefusalNamesTheMode(t *testing.T) {
	t.Parallel()

	for _, err := range []error{errUpdateDisabledInKubernetes, errRestartDisabledInKubernetes} {
		if got := err.Error(); !strings.Contains(got, "disabled in kubernetes mode") {
			t.Errorf("%q does not say it is disabled in kubernetes mode", got)
		}
	}
}

// TestEditRefusesToSaveKubernetesModeWhileTheBinaryCannotRunIt closes the path a
// row on the settings page opens: execution_mode is an ordinary non-secret key,
// so the page offers it, and a save of `kubernetes` followed by the restart
// button would leave a daemon that refuses to start and a dashboard that went
// with it. The FR-010 backup does not help, since the refusal comes after a load
// that succeeded.
//
// The edit is refused the way any value the daemon would not start on is, with
// the file left exactly as it was. The host value is the control: it saves.
//
// **Must fail when** the candidate is written because the loader accepts it.
func TestEditRefusesToSaveKubernetesModeWhileTheBinaryCannotRunIt(t *testing.T) {
	f := editable(t)
	before := readConfigFile(t, f.cfg.FilePath)

	w := editPost(t, f, editForm(t, f, "execution_mode", "kubernetes"))

	wantOutcome(t, w, outcome("setting-refused"))
	if after := readConfigFile(t, f.cfg.FilePath); after != before {
		t.Errorf("a mode the binary cannot run was saved anyway:\n%s", after)
	}

	// The control. The page renders what the running daemon is on, so a save of
	// `host` is a change only when the daemon is not already there; a fixture
	// that is would answer "unchanged" and prove nothing about the write.
	f.cfg.ExecutionMode = config.ExecutionModeKubernetes
	w = editPost(t, f, editForm(t, f, "execution_mode", "host"))

	wantOutcome(t, w, outcome("setting-written"))
	if after := readConfigFile(t, f.cfg.FilePath); !strings.Contains(after, "execution_mode = host") {
		t.Errorf("the host mode was not saved:\n%s", after)
	}
}
