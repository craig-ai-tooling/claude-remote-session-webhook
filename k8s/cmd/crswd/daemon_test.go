package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
)

// No test in this package calls t.Parallel, and TestClusterBuildIsRunnable stays
// first in the first file: it flips a package variable in internal/config that
// nothing synchronises, and it asserts the state from before the flip.

func TestClusterBuildIsRunnable(t *testing.T) {
	if err := config.ExecutionModeKubernetes.Runnable(); !errors.Is(err, config.ErrKubernetesModeUnbuilt) {
		t.Fatalf("before BuildKubernetesMode, Runnable() = %v; want %v", err, config.ErrKubernetesModeUnbuilt)
	}
	config.BuildKubernetesMode()
	if err := config.ExecutionModeKubernetes.Runnable(); err != nil {
		t.Fatalf("after BuildKubernetesMode, Runnable() = %v; want nil", err)
	}
}

// setLoadableEnv is the least a config needs to load, with the dashboard
// password as the browser door so no Cloudflare variable is needed.
func setLoadableEnv(t *testing.T) {
	t.Helper()
	t.Setenv(config.EnvSharedSecret, "test-only-shared-secret-32-bytes")
	t.Setenv(config.EnvAllowedRoots, t.TempDir())
	t.Setenv(config.EnvDashboardPassword, strings.Repeat("p", config.MinDashboardPasswordLen))
	// A host's own config file or environment may name the other door, and a
	// daemon has one. An empty config directory keeps the real file out.
	t.Setenv("CRSW_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv(config.EnvAccessTeamDomain, "")
	t.Setenv(config.EnvAccessAUD, "")
	t.Setenv(config.EnvAccessAllowedEmails, "")
	t.Setenv(config.EnvAccessEnabled, "")
}

func TestDaemonRefusesHostConfig(t *testing.T) {
	setLoadableEnv(t)
	t.Setenv(config.EnvExecutionMode, "")
	err := runDaemon(context.Background(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "CRSW_EXECUTION_MODE=kubernetes") {
		t.Fatalf("runDaemon = %v; want the cluster-build refusal", err)
	}
}

func TestDaemonNeedsSessionNamespace(t *testing.T) {
	config.BuildKubernetesMode()
	setLoadableEnv(t)
	t.Setenv(config.EnvExecutionMode, "kubernetes")
	t.Setenv("CRSW_SESSION_NAMESPACE", "")
	err := runDaemon(context.Background(), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "CRSW_SESSION_NAMESPACE") {
		t.Fatalf("runDaemon = %v; want an error naming CRSW_SESSION_NAMESPACE", err)
	}
}

func TestReconcileRefusesBadConfig(t *testing.T) {
	t.Setenv("CRSW_SESSION_NAMESPACE", "")
	err := runReconciler(context.Background())
	if err == nil || !strings.Contains(err.Error(), "CRSW_SESSION_NAMESPACE") {
		t.Fatalf("runReconciler = %v; want ConfigFromEnv's error", err)
	}
}
