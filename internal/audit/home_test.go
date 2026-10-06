package audit_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain moves the process's own home, Claude and Codex directories into a
// temporary one. The leak suite runs the real daemon, and httpapi.New takes the
// session environment from os.Environ(), so every session it creates seeds
// workspace trust into whatever ~/.claude.json and ~/.codex/config.toml that
// environment names. Without this the suite wrote its temp directories into the
// operator's real files on every run.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "audit-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for k, v := range map[string]string{
		"HOME":              home,
		"CLAUDE_CONFIG_DIR": filepath.Join(home, ".claude"),
		"CODEX_HOME":        filepath.Join(home, ".codex"),
	} {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	if err := os.RemoveAll(home); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
