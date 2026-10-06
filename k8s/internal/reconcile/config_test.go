package reconcile

import (
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
)

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func baseEnv() map[string]string {
	return map[string]string{
		"CRSW_SESSION_NAMESPACE":    "sessions",
		"CRSW_RECONCILER_NAMESPACE": "crswd",
		"CRSW_SESSION_IMAGE":        "registry.example/crswd-session:1.2.3",
	}
}

func TestConfigDefaults(t *testing.T) {
	t.Parallel()
	cfg, err := ConfigFromEnv(envOf(baseEnv()))
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if cfg.SessionNamespace != "sessions" || cfg.LeaseNamespace != "crswd" {
		t.Errorf("namespaces = %q, %q", cfg.SessionNamespace, cfg.LeaseNamespace)
	}
	if cfg.Node != "" {
		t.Errorf("Node = %q, want none", cfg.Node)
	}
	if cfg.Claim != "crswd-sessions" {
		t.Errorf("Claim = %q", cfg.Claim)
	}
	if cfg.Cap != config.DefaultMaxSessions {
		t.Errorf("Cap = %d, want %d", cfg.Cap, config.DefaultMaxSessions)
	}
	if cfg.LifetimeMax != session.AbsoluteLifetime {
		t.Errorf("LifetimeMax = %v, want %v", cfg.LifetimeMax, session.AbsoluteLifetime)
	}
	if cfg.ClaudeSecret != "claude-credentials" || cfg.CodexSecret != "codex-auth" {
		t.Errorf("secrets = %q, %q", cfg.ClaudeSecret, cfg.CodexSecret)
	}
	if cfg.Home != "/home/ralph" || cfg.UID != 10001 {
		t.Errorf("Home, UID = %q, %d", cfg.Home, cfg.UID)
	}
}

func TestConfigOverrides(t *testing.T) {
	t.Parallel()
	env := baseEnv()
	env["CRSW_SESSION_NODE"] = "node-a"
	env["CRSW_SESSION_CLAIM"] = "other-claim"
	env[config.EnvMaxSessions] = "2"
	env[config.EnvSessionLifetimeMax] = "6h"
	env["CRSW_CLAUDE_SECRET"] = "my-claude"
	env["CRSW_CODEX_SECRET"] = "my-codex"
	env["CRSW_SESSION_HOME"] = "/home/other"
	cfg, err := ConfigFromEnv(envOf(env))
	if err != nil {
		t.Fatalf("ConfigFromEnv: %v", err)
	}
	if cfg.Node != "node-a" || cfg.Claim != "other-claim" || cfg.Cap != 2 ||
		cfg.LifetimeMax != 6*time.Hour || cfg.ClaudeSecret != "my-claude" ||
		cfg.CodexSecret != "my-codex" || cfg.Home != "/home/other" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestConfigLifetimeNever(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"never", "NEVER", "Never"} {
		env := baseEnv()
		env[config.EnvSessionLifetimeMax] = v
		cfg, err := ConfigFromEnv(envOf(env))
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		if cfg.LifetimeMax != 0 {
			t.Errorf("%q: LifetimeMax = %v, want 0", v, cfg.LifetimeMax)
		}
	}
}

func TestConfigRefusals(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		set     map[string]string
		wantVar string
	}{
		{"empty session namespace", map[string]string{"CRSW_SESSION_NAMESPACE": ""}, "CRSW_SESSION_NAMESPACE"},
		{"empty reconciler namespace", map[string]string{"CRSW_RECONCILER_NAMESPACE": ""}, "CRSW_RECONCILER_NAMESPACE"},
		{"same namespace", map[string]string{"CRSW_RECONCILER_NAMESPACE": "sessions"}, "CRSW_RECONCILER_NAMESPACE"},
		{"empty image", map[string]string{"CRSW_SESSION_IMAGE": ""}, "CRSW_SESSION_IMAGE"},
		{"latest image", map[string]string{"CRSW_SESSION_IMAGE": "registry.example/crswd:latest"}, "CRSW_SESSION_IMAGE"},
		{"untagged image", map[string]string{"CRSW_SESSION_IMAGE": "registry.example/crswd"}, "CRSW_SESSION_IMAGE"},
		{"registry port is not a tag", map[string]string{"CRSW_SESSION_IMAGE": "registry.example:5000/crswd"}, "CRSW_SESSION_IMAGE"},
		{"cap not a number", map[string]string{config.EnvMaxSessions: "many"}, config.EnvMaxSessions},
		{"cap zero", map[string]string{config.EnvMaxSessions: "0"}, config.EnvMaxSessions},
		{"lifetime unparsable", map[string]string{config.EnvSessionLifetimeMax: "soon"}, config.EnvSessionLifetimeMax},
		{"lifetime zero", map[string]string{config.EnvSessionLifetimeMax: "0s"}, config.EnvSessionLifetimeMax},
		{"lifetime negative", map[string]string{config.EnvSessionLifetimeMax: "-1h"}, config.EnvSessionLifetimeMax},
		{"relative home", map[string]string{"CRSW_SESSION_HOME": "home/ralph"}, "CRSW_SESSION_HOME"},
		{"bad claim", map[string]string{"CRSW_SESSION_CLAIM": "Bad_Name"}, "CRSW_SESSION_CLAIM"},
		{"bad claude secret", map[string]string{"CRSW_CLAUDE_SECRET": "Bad_Name"}, "CRSW_CLAUDE_SECRET"},
		{"bad codex secret", map[string]string{"CRSW_CODEX_SECRET": "Bad_Name"}, "CRSW_CODEX_SECRET"},
		{"dotted session namespace", map[string]string{"CRSW_SESSION_NAMESPACE": "a.b"}, "CRSW_SESSION_NAMESPACE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := baseEnv()
			for k, v := range tc.set {
				env[k] = v
			}
			_, err := ConfigFromEnv(envOf(env))
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.wantVar) {
				t.Errorf("error %q does not name %s", err, tc.wantVar)
			}
		})
	}
}

func TestConfigDigestImageAccepted(t *testing.T) {
	t.Parallel()
	env := baseEnv()
	env["CRSW_SESSION_IMAGE"] = "registry.example/crswd@sha256:" + strings.Repeat("a", 64)
	if _, err := ConfigFromEnv(envOf(env)); err != nil {
		t.Fatalf("digest image refused: %v", err)
	}
}

func TestConfigRoots(t *testing.T) {
	t.Parallel()
	roots := Config{}.Roots()
	if len(roots) != 1 || roots[0].Path != sessionpod.WorkRoot || sessionpod.WorkRoot != "/work" {
		t.Errorf("Roots() = %+v, want exactly /work", roots)
	}
}

func TestConfigNow(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	if got := (Config{Now: func() time.Time { return fixed }}).now(); !got.Equal(fixed) {
		t.Errorf("now() = %v, want %v", got, fixed)
	}
	if got := (Config{}).now(); time.Since(got) > time.Minute {
		t.Errorf("nil Now returned %v", got)
	}
}
