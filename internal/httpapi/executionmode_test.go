package httpapi

// What the execution mode switches off in the server (spec 017, FR-003): the
// sign-in relay, the release feed, the update and restart routes, and the
// session journal. Each is asserted twice, once in kubernetes mode and once in
// host mode, because a check that only looked at the new mode would pass
// through a change that broke the old one, and host mode is what SC-003 says
// must not move.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
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

// TestKubernetesModeHasNoJournal is the journal half of FR-003's sibling rule,
// which the plan states as one sentence: the object is the record, so
// ReplayJournal and Adopt can never both revive one session.
//
// It reads where the manager's journal writes. A journal with no path keeps
// nothing and replays nothing, which is what "off" means to the manager; the
// host-mode half proves that the same call site does give a manager a path, so
// an empty answer is the mode's doing.
//
// Not parallel: the journal's path is derived from the process environment, and
// this pins CRSW_CONFIG_FILE so that host mode has somewhere to name.
//
// **Must fail when** kubernetes mode gives the manager a journal.
func TestKubernetesModeHasNoJournal(t *testing.T) {
	t.Setenv("CRSW_CONFIG_FILE", filepath.Join(t.TempDir(), "config"))

	build := func(mode config.ExecutionMode) *Server {
		t.Helper()

		fixture := newSessionFixture(t)
		srv, err := NewWith(modeConfig(mode), fixture.tmux, audit.NewTo(io.Discard, func() time.Time { return testTime }))
		if err != nil {
			t.Fatalf("NewWith(%q) = _, %v; want a server", mode, err)
		}
		return srv
	}

	if got := build(config.ExecutionModeKubernetes).sessions.JournalPath(); got != "" {
		t.Errorf("kubernetes mode journals to %q; want no journal, so the object is the only record", got)
	}
	got := build(config.ExecutionModeHost).sessions.JournalPath()
	if got == "" {
		t.Error("host mode has no journal; the mode switch has taken away the host's own memory")
	}
	if want := os.Getenv("CRSW_CONFIG_FILE"); got != "" && filepath.Dir(got) != filepath.Dir(want) {
		t.Errorf("host mode journals to %q, which is not beside the configuration file %q as it always was", got, want)
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
