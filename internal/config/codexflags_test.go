package config

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestCodexUpdateCheckValues(t *testing.T) {
	t.Parallel()

	const key = "check_for_update_on_startup"
	tests := []struct {
		name    string
		command string
		want    []string
		wantErr bool
	}{
		{"bare codex", "codex", nil, false},
		{"false", "codex -c " + key + "=false", []string{"false"}, false},
		{"true", "codex -c " + key + "=true", []string{"true"}, true},
		{"long form with equals", "codex --config=" + key + "=true", []string{"true"}, true},
		{"long form with next token", "codex --config " + key + "=true", []string{"true"}, true},
		{"attached short form, quoted", `codex -c` + key + `="true"`, []string{"true"}, true},
		{"single quotes", "codex -c " + key + "='false'", []string{"false"}, false},
		{"last one wins, both are read", "codex -c " + key + "=false -c " + key + "=true", []string{"false", "true"}, true},
		{"after the double dash is a prompt", "codex -- -c " + key + "=true", nil, false},
		{"not codex", "claude -c " + key + "=true", []string{"true"}, false},
		{"absolute path, numeric", "/abs/sf-cli/bin/codex -c " + key + "=1", []string{"1"}, true},
		{"another key", "codex -c model=o3", nil, false},
		{"key prefix only", "codex -c " + key + "x=true", nil, false},
		{"trailing -c with nothing", "codex -c", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := codexUpdateCheckValues(tc.command); !slices.Equal(got, tc.want) {
				t.Errorf("codexUpdateCheckValues(%q) = %q, want %q", tc.command, got, tc.want)
			}
			err := validateCodexUpdateCheck("VAR", "n", tc.command)
			if tc.command != "" && strings.ContainsAny(tc.command, "'\"\\") && strings.HasPrefix(tc.command, "codex") {
				if !errors.Is(err, ErrCodexQuoting) {
					t.Fatalf("validateCodexUpdateCheck(%q) = %v, want ErrCodexQuoting", tc.command, err)
				}
				return
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateCodexUpdateCheck(%q) = %v, wantErr %v", tc.command, err, tc.wantErr)
			}
			if err != nil {
				if !errors.Is(err, ErrCodexUpdateCheck) {
					t.Errorf("error %v does not wrap ErrCodexUpdateCheck", err)
				}
				if strings.Contains(err.Error(), "check_for_update") {
					t.Errorf("error spells the command line: %v", err)
				}
			}
		})
	}
}

func TestLoadStartCommandsRefusesCodexUpdateCheck(t *testing.T) {
	t.Parallel()

	pairs := map[string]string{
		EnvSharedSecret:        "test-only-shared-secret-32-bytes",
		EnvAllowedRoots:        t.TempDir(),
		EnvAccessTeamDomain:    "example-team.cloudflareaccess.com",
		EnvAccessAUD:           "test-only-audience-tag",
		EnvAccessAllowedEmails: "operator@example.com",
		EnvStartCommands:       "codex=codex -c check_for_update_on_startup=true",
	}
	_, err := LoadFrom(func(k string) string { return pairs[k] }, io.Discard)
	if !errors.Is(err, ErrCodexUpdateCheck) {
		t.Fatalf("LoadFrom() = %v, want ErrCodexUpdateCheck", err)
	}
}

func TestValidateCodexQuoting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		command string
	}{
		{"double quote", `codex -c model="o3"`},
		{"single quote", "codex -c model='o3'"},
		{"backslash", `codex -c model=o\3`},
		{"quoted whole assignment", `codex -c "check_for_update_on_startup=true"`},
		{"single-quoted whole assignment", `codex -c 'check_for_update_on_startup=true'`},
		{"quoted key", `codex -c "check_for_update_on_startup"=true`},
		{"escaped key", `codex -c check_for_update_on_st\artup=true`},
		{"absolute path", `/abs/bin/codex -c "check_for_update_on_startup=true"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateCodexUpdateCheck("VAR", "n", tc.command)
			if !errors.Is(err, ErrCodexQuoting) {
				t.Fatalf("validateCodexUpdateCheck(%q) = %v, want ErrCodexQuoting", tc.command, err)
			}
			if strings.Contains(err.Error(), "check_for_update") && !strings.Contains(err.Error(), "may not contain") {
				t.Errorf("unexpected error text: %v", err)
			}
		})
	}
	// Claude and other harnesses keep their quotes.
	for _, c := range []string{`claude --name "x"`, `sh -c 'echo hi'`} {
		if err := validateCodexUpdateCheck("VAR", "n", c); err != nil {
			t.Errorf("validateCodexUpdateCheck(%q) = %v, want nil", c, err)
		}
	}
}

func TestLoadRefusesCodexRemoteControlCommand(t *testing.T) {
	t.Parallel()

	for name, env := range map[string]map[string]string{
		"named": {
			EnvStartCommands:        "rc=codex --dangerously-bypass-approvals-and-sandbox",
			EnvRemoteControlCommand: "rc",
		},
		"defaulted to rc": {
			EnvStartCommands: "rc=/usr/bin/codex",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pairs := map[string]string{
				EnvSharedSecret:        "test-only-shared-secret-32-bytes",
				EnvAllowedRoots:        t.TempDir(),
				EnvAccessTeamDomain:    "example-team.cloudflareaccess.com",
				EnvAccessAUD:           "test-only-audience-tag",
				EnvAccessAllowedEmails: "operator@example.com",
			}
			for k, v := range env {
				pairs[k] = v
			}
			_, err := LoadFrom(func(k string) string { return pairs[k] }, io.Discard)
			if !errors.Is(err, ErrCodexRemoteControl) {
				t.Fatalf("LoadFrom() = %v, want ErrCodexRemoteControl", err)
			}
		})
	}
}

func TestCodexExecutableMustBeShellSafe(t *testing.T) {
	t.Parallel()

	bad := []string{
		"/tmp/$(touch${IFS}/tmp/pwn)/codex",
		"/tmp/`id`/codex",
		"/tmp/a$HOME/codex",
		"/tmp/a&b/codex",
		"/tmp/a|b/codex",
		"/tmp/a(b/codex",
		"/tmp/a;b/codex",
		"/tmp/a>b/codex",
	}
	for _, command := range bad {
		err := validateCodexUpdateCheck("VAR", "n", command)
		if !errors.Is(err, ErrCodexExecutable) {
			t.Errorf("validateCodexUpdateCheck(%q) = %v, want ErrCodexExecutable", command, err)
		}
	}
	// A space ends the token, so the executable itself is still safe.
	for _, command := range []string{"codex", "/opt/sf-cli/bin/codex --yolo", "/usr/local/bin/codex-1.2+x"} {
		if err := validateCodexUpdateCheck("VAR", "n", command); err != nil {
			t.Errorf("validateCodexUpdateCheck(%q) = %v, want nil", command, err)
		}
	}
}
