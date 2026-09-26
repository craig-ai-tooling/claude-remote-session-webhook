package config_test

// The execution mode (spec 017, FR-001, FR-003): where a session runs, and what
// that switches off. Every case here is about one of two things that must not
// drift: an operator's word is read exactly or refused, and a daemon that never
// said anything is the host daemon it always was.

import (
	"io"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
)

// modeVar and modeKey are spelled as literals on purpose. The constants are
// what the daemon reads, and a test that named them would pass through a rename
// that broke every operator's file.
const (
	modeVar = "CRSW_EXECUTION_MODE"
	modeKey = "execution_mode"
)

// TestExecutionModeAbsentMeansHost is FR-001's sentence, "absent means host,
// which is v0's behaviour byte for byte", as a property of the loader: no
// variable and no file key is the host, and nothing was written into the
// provenance record that says otherwise.
//
// **Must fail when** the loader defaults to anything but the host, or when it
// starts demanding the setting.
func TestExecutionModeAbsentMeansHost(t *testing.T) {
	t.Parallel()

	pairs, _ := baseEnv(t)
	cfg := mustLoad(t, pairs)

	if cfg.ExecutionMode != config.ExecutionModeHost {
		t.Errorf("ExecutionMode = %q with nothing configured; want %q", cfg.ExecutionMode, config.ExecutionModeHost)
	}
	if cfg.ExecutionMode.Kubernetes() {
		t.Error("a daemon that was never told a mode reports kubernetes")
	}
	if got := cfg.Sources[modeVar]; got != config.SourceDefault {
		t.Errorf("Sources[%s] = %v; want the default, since nothing supplied it", modeVar, got)
	}
}

// TestExecutionModeParses covers the two words and the two places they may come
// from, and the precedence between the places, which is the one every other
// setting has: the environment beats the file.
//
// The file case is the one the old code failed for a plain reason: it did not
// know the key, so it refused `execution_mode` as unknown before it read a
// value.
//
// **Must fail when** either word is refused, the file key is not accepted, or
// the file beats the environment.
func TestExecutionModeParses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		env  string
		file string
		want config.ExecutionMode
		from config.Source
	}{
		{name: "environment host", env: "host", want: config.ExecutionModeHost, from: config.SourceEnv},
		{name: "environment kubernetes", env: "kubernetes", want: config.ExecutionModeKubernetes, from: config.SourceEnv},
		{name: "environment with the space a unit file leaves", env: " kubernetes ", want: config.ExecutionModeKubernetes, from: config.SourceEnv},
		{name: "file kubernetes", file: "execution_mode = kubernetes\n", want: config.ExecutionModeKubernetes, from: config.SourceFile},
		{name: "file host", file: "execution_mode = host\n", want: config.ExecutionModeHost, from: config.SourceFile},
		{name: "the environment beats the file", env: "host", file: "execution_mode = kubernetes\n", want: config.ExecutionModeHost, from: config.SourceEnv},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pairs, _ := baseEnv(t)
			if tc.env != "" {
				pairs[modeVar] = tc.env
			}
			if tc.file != "" {
				pairs["CRSW_CONFIG_FILE"] = writeConfig(t, tc.file, 0o600)
			}

			cfg, err := config.LoadFrom(env(pairs), io.Discard)
			if err != nil {
				t.Fatalf("LoadFrom = _, %v; want the mode read", err)
			}
			if cfg.ExecutionMode != tc.want {
				t.Errorf("ExecutionMode = %q; want %q", cfg.ExecutionMode, tc.want)
			}
			if got := cfg.Sources[modeVar]; got != tc.from {
				t.Errorf("Sources[%s] = %v; want %v", modeVar, got, tc.from)
			}
		})
	}
}

// TestExecutionModeRefusesWhatItDoesNotKnow is the refusal the task names: `k8s`
// is not `kubernetes`, and a daemon that guessed would put an operator's
// sessions on the machine they meant to leave.
//
// The refusal names the variable and the two words, and quotes the value the
// operator wrote. It is the same refusal from the environment and from the file,
// because the file is a source behind the same seam.
//
// **Must fail when** a value outside the two words loads, is read as a
// near-miss, or is refused with a message that does not say what was wrong.
func TestExecutionModeRefusesWhatItDoesNotKnow(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"k8s", "Kubernetes", "KUBERNETES", "kube", "docker", "host,kubernetes", "true"} {
		for _, source := range []string{"environment", "file"} {
			t.Run(source+" "+value, func(t *testing.T) {
				t.Parallel()

				pairs, _ := baseEnv(t)
				if source == "environment" {
					pairs[modeVar] = value
				} else {
					pairs["CRSW_CONFIG_FILE"] = writeConfig(t, "execution_mode = "+value+"\n", 0o600)
				}

				cfg, err := config.LoadFrom(env(pairs), io.Discard)
				if err == nil {
					t.Fatalf("LoadFrom accepted %s %q as %q; want a refusal", modeVar, value, cfg.ExecutionMode)
				}
				for _, want := range []string{modeVar, `"` + value + `"`, "host", "kubernetes", "refusing to start"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("the refusal does not mention %q: %v", want, err)
					}
				}
			})
		}
	}
}

// TestParseExecutionMode is the same rule at the function the loader and the
// `unit` command both call, so the two cannot read one word differently.
func TestParseExecutionMode(t *testing.T) {
	t.Parallel()

	for word, want := range map[string]config.ExecutionMode{
		"":           config.ExecutionModeHost,
		"host":       config.ExecutionModeHost,
		"kubernetes": config.ExecutionModeKubernetes,
	} {
		got, err := config.ParseExecutionMode(word)
		if err != nil || got != want {
			t.Errorf("ParseExecutionMode(%q) = %q, %v; want %q", word, got, err, want)
		}
	}
	if got, err := config.ParseExecutionMode("k8s"); err == nil {
		t.Errorf("ParseExecutionMode(\"k8s\") = %q, nil; want a refusal", got)
	}
}

// TestAConfigNobodySetTheModeOnIsTheHost holds the zero value, which every hand
// built Config in this repository's tests is. If the zero value were anything
// else, every branch that asks Kubernetes() would need a fixture updated to keep
// the host behaviour it was written to assert.
func TestAConfigNobodySetTheModeOnIsTheHost(t *testing.T) {
	t.Parallel()

	var cfg config.Config
	if cfg.ExecutionMode.Kubernetes() {
		t.Error("the zero ExecutionMode is kubernetes")
	}
	if got := cfg.ExecutionMode.String(); got != "host" {
		t.Errorf("the zero ExecutionMode renders as %q; want %q, which is what it does", got, "host")
	}
}

// TestKubernetesModeSkipsTheDependencyProbe is FR-003's tmux half. On a host
// with no tmux, host mode refuses to start and kubernetes mode does not look:
// its tmux is in the pod, so a daemon image without one is a correct
// deployment.
//
// The counts are asserted beside the answer. A probe that ran and was ignored
// would still have asked the operator's login shell for its PATH, which runs
// their profile.
//
// **Must fail when** the probe runs in kubernetes mode, or stops running in
// host mode.
func TestKubernetesModeSkipsTheDependencyProbe(t *testing.T) {
	t.Parallel()

	commands := config.NewStartCommands(map[string]string{"default": "claude --dangerously-skip-permissions"})

	t.Run("kubernetes mode does not probe", func(t *testing.T) {
		t.Parallel()

		host := newHostTools(t) // nothing installed: no tmux, no claude
		var warn warnBuffer

		err := config.CheckDependenciesWith(
			config.Config{StartCommands: commands, ExecutionMode: config.ExecutionModeKubernetes},
			host.lookPath, host.loginShellPATH, host.osRelease, &warn)

		if err != nil {
			t.Errorf("a kubernetes-mode daemon on a host with no tmux was refused: %v", err)
		}
		if len(host.asked) != 0 || host.loginAsks != 0 {
			t.Errorf("the probe asked the host about %v and ran the login shell %d times; want neither", host.asked, host.loginAsks)
		}
		if warn.String() != "" {
			t.Errorf("the probe warned in kubernetes mode:\n%s", warn.String())
		}
	})

	t.Run("host mode still refuses without tmux", func(t *testing.T) {
		t.Parallel()

		host := newHostTools(t, "claude")
		var warn warnBuffer

		err := config.CheckDependenciesWith(
			config.Config{StartCommands: commands, ExecutionMode: config.ExecutionModeHost},
			host.lookPath, host.loginShellPATH, host.osRelease, &warn)

		if err == nil || !strings.Contains(err.Error(), "tmux") {
			t.Errorf("a host-mode daemon with no tmux started or was refused for another reason: %v", err)
		}
	})
}
