package sessionpod

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestMain moves the process's own home, Claude and Codex directories into a
// temporary one. Pod.Env nil means os.Environ(), so every older Run test that
// leaves Env unset now seeds from it, and without this it would write into the
// operator's real configuration.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "sessionpod-home-")
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

// seedDirs makes the three directories a seed run touches, each with its symlinks
// already followed so the trust key a test reads back is the one Seed wrote.
func seedDirs(t *testing.T) (claudeDir, codexHome, workDir string) {
	t.Helper()

	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve the temporary directory: %v", err)
	}
	// The Claude and Codex directories do not exist yet: a pod starts without them.
	return filepath.Join(base, "claude"), filepath.Join(base, "codex"), mkdir(t, filepath.Join(base, "work"))
}

func readJSON(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()

	raw, err := os.ReadFile(path) //nolint:gosec // G304: a path the test just built under t.TempDir.
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return doc
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

func trustedIn(t *testing.T, doc map[string]json.RawMessage, workDir string) bool {
	t.Helper()

	var projects map[string]struct {
		Trusted bool `json:"hasTrustDialogAccepted"`
	}
	if err := json.Unmarshal(doc["projects"], &projects); err != nil {
		t.Fatalf("decode projects: %v", err)
	}
	return projects[workDir].Trusted
}

func TestSeedWritesFreshClaudeConfig(t *testing.T) {
	t.Parallel()

	claudeDir, codexHome, workDir := seedDirs(t)
	env := []string{"CLAUDE_CONFIG_DIR=" + claudeDir, "CODEX_HOME=" + codexHome}

	if err := Seed(env, workDir); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	config := filepath.Join(claudeDir, ".claude.json")
	doc := readJSON(t, config)
	if string(doc["hasCompletedOnboarding"]) != "true" {
		t.Errorf("hasCompletedOnboarding = %s, want true", doc["hasCompletedOnboarding"])
	}
	if !trustedIn(t, doc, workDir) {
		t.Errorf("projects[%s].hasTrustDialogAccepted is not true", workDir)
	}

	settings := filepath.Join(claudeDir, "settings.json")
	if got := string(readJSON(t, settings)["skipDangerousModePermissionPrompt"]); got != "true" {
		t.Errorf("skipDangerousModePermissionPrompt = %s, want true", got)
	}
	for _, p := range []string{config, settings} {
		if m := mode(t, p); m != 0o600 {
			t.Errorf("%s mode = %o, want 600", p, m)
		}
	}
}

func TestSeedKeepsExistingClaudeConfig(t *testing.T) {
	t.Parallel()

	claudeDir, _, workDir := seedDirs(t)
	mkdir(t, claudeDir)
	config := filepath.Join(claudeDir, ".claude.json")
	settings := filepath.Join(claudeDir, "settings.json")
	const kept = `{"theme":"dark"}`
	if err := os.WriteFile(config, []byte(`{"keep":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Seed([]string{"CLAUDE_CONFIG_DIR=" + claudeDir}, workDir); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	doc := readJSON(t, config)
	if string(doc["keep"]) != "1" {
		t.Errorf("keep = %s, want 1: an existing key was lost", doc["keep"])
	}
	if !trustedIn(t, doc, workDir) {
		t.Errorf("the existing file did not gain the trust entry")
	}
	got, err := os.ReadFile(settings) //nolint:gosec // G304: a path the test just built under t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != kept {
		t.Errorf("settings.json = %q, want it untouched (%q)", got, kept)
	}
}

func TestSeedCodexTrust(t *testing.T) {
	t.Parallel()

	_, codexHome, workDir := seedDirs(t)

	if err := Seed([]string{"CODEX_HOME=" + codexHome}, workDir); err != nil {
		t.Fatalf("Seed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(codexHome, "config.toml")) //nolint:gosec // G304: a path the test just built under t.TempDir.
	if err != nil {
		t.Fatalf("config.toml was not written: %v", err)
	}
	text := string(raw)
	header := `[projects."` + workDir + `"]`
	at := strings.Index(text, header)
	if at < 0 {
		t.Fatalf("no table for the working directory in:\n%s", text)
	}
	if !strings.Contains(text[at:], `trust_level = "trusted"`) {
		t.Errorf("the working directory's table does not hold trust_level = \"trusted\"")
	}
}

func TestSeedNoConfigDirs(t *testing.T) {
	t.Parallel()

	_, _, workDir := seedDirs(t)
	before, err := os.ReadDir(filepath.Dir(workDir))
	if err != nil {
		t.Fatal(err)
	}

	if err := Seed([]string{"PATH=/bin"}, workDir); err != nil {
		t.Fatalf("Seed = %v, want nil with nothing to seed", err)
	}

	after, err := os.ReadDir(filepath.Dir(workDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("Seed created %d entries beside the working directory, want none", len(after)-len(before))
	}
}

// Fails before T2: CODEX_HOME was dropped from the session's environment, so Codex
// in a pod would have looked in $HOME/.codex and found no login.
func TestPassThroughCarriesCodexHome(t *testing.T) {
	t.Parallel()

	got := environment([]string{"CODEX_HOME=/c", "CLAUDE_CONFIG_DIR=/d", "PATH=/bin", "HOME=/h"})

	for _, want := range []string{"CODEX_HOME=/c", "CLAUDE_CONFIG_DIR=/d"} {
		if !slices.Contains(got, want) {
			t.Errorf("environment is missing %s: %v", want, got)
		}
	}
}

func TestRunSeedsBeforeNew(t *testing.T) {
	t.Parallel()

	p, f := newPod()
	claudeDir, codexHome, _ := seedDirs(t)
	p.Env = []string{"CLAUDE_CONFIG_DIR=" + claudeDir, "CODEX_HOME=" + codexHome}
	root, roots := approvedRoot(t)
	workdir := mkdir(t, filepath.Join(root, "repo"))

	done := runAsync(context.Background(), p, podName, workdir, roots)
	waitUntil(t, "the session to exist", func() bool { return hasSession(f, podName) })

	doc := readJSON(t, filepath.Join(claudeDir, ".claude.json"))
	if !trustedIn(t, doc, workdir) {
		t.Errorf("Run started the session before trusting %s", workdir)
	}

	f.Vanish(podName)
	if err := result(t, done, "Run after the session vanished"); err != nil {
		t.Errorf("Run = %v, want nil once the session is gone", err)
	}
}
