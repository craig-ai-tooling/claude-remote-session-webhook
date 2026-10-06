package sessionpod

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
)

// Seed writes the first-run answers a session pod has no person to give. A pod
// starts with an empty Claude config directory and a Codex home that may be empty,
// and the pane would otherwise sit on an onboarding, trust or bypass screen for
// ever (research D8a: the probe pod seeded these three facts and saw no prompt).
//
// It only ever adds. A file that exists is kept as it is, apart from the trust
// entry session.SeedTrust and session.SeedCodexTrust add, so a claim that already
// holds the operator's own configuration is not reset by a pod restart.
//
// Errors name the file and never its contents.
func Seed(env []string, workDir string) error {
	if claudeDir := envValue(env, "CLAUDE_CONFIG_DIR"); filepath.IsAbs(claudeDir) {
		if err := seedClaude(claudeDir, workDir); err != nil {
			return err
		}
	}
	if codexHome := session.CodexHome(env); codexHome != "" {
		if err := os.MkdirAll(codexHome, 0o700); err != nil {
			return fmt.Errorf("seed codex home %s: %w", codexHome, err)
		}
		if err := session.SeedCodexTrust(codexHome, workDir); err != nil {
			return fmt.Errorf("seed %s: %w", filepath.Join(codexHome, "config.toml"), err)
		}
	}
	return nil
}

func seedClaude(claudeDir, workDir string) error {
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		return fmt.Errorf("seed claude config directory %s: %w", claudeDir, err)
	}

	config := filepath.Join(claudeDir, ".claude.json")
	switch _, err := os.Stat(config); {
	case errors.Is(err, fs.ErrNotExist):
		// Built with encoding/json: workDir is a path a caller chose, so it must
		// never be spliced into JSON as text.
		doc := map[string]any{
			"hasCompletedOnboarding": true,
			"projects":               map[string]any{workDir: map[string]any{"hasTrustDialogAccepted": true}},
		}
		if err := writeNew(config, doc); err != nil {
			return err
		}
	case err != nil:
		return fmt.Errorf("seed %s: %w", config, err)
	default:
		if err := session.SeedTrust(config, workDir); err != nil {
			return fmt.Errorf("seed %s: %w", config, err)
		}
	}

	// On this host the bypass acceptance lives in settings.json. A file that is
	// there is the operator's and is left alone.
	settings := filepath.Join(claudeDir, "settings.json")
	switch _, err := os.Stat(settings); {
	case errors.Is(err, fs.ErrNotExist):
		return writeNew(settings, map[string]any{"skipDangerousModePermissionPrompt": true})
	case err != nil:
		return fmt.Errorf("seed %s: %w", settings, err)
	}
	return nil
}

// writeNew writes doc as the file at path, mode 0600, by renaming a temporary file
// from the same directory so a reader never sees half of it.
func writeNew(path string, doc map[string]any) (err error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("seed %s: %w", path, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".crswd-*")
	if err != nil {
		return fmt.Errorf("seed %s: %w", path, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.Remove(tmp.Name()))
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return errors.Join(fmt.Errorf("seed %s: %w", path, err), tmp.Close())
	}
	if _, err := tmp.Write(raw); err != nil {
		return errors.Join(fmt.Errorf("seed %s: %w", path, err), tmp.Close())
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("seed %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("seed %s: %w", path, err)
	}
	return nil
}

// envValue is the last value env gives key, trimmed, which is how a process's
// environment resolves a repeated name.
func envValue(env []string, key string) string {
	v := ""
	for _, kv := range env {
		if k, val, ok := strings.Cut(kv, "="); ok && k == key {
			v = val
		}
	}
	return strings.TrimSpace(v)
}
