// Package reconcile keeps one pod per AgentSession and holds every object to
// spec 001's containment whoever wrote it (spec 017 FR-006 to FR-010).
package reconcile

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	LabelManagedBy = "app.kubernetes.io/managed-by"
	ManagedByValue = "crswd-reconciler"
	LabelSession   = v1alpha1.Group + "/session"
	LeaseName      = "crswd-reconciler"
	// SessionContainer names the one container in a session pod. podctl.Container
	// is the same string; S7's test holds the two equal.
	SessionContainer = "session"
)

const (
	envSessionNamespace    = "CRSW_SESSION_NAMESPACE"
	envReconcilerNamespace = "CRSW_RECONCILER_NAMESPACE"
	envSessionImage        = "CRSW_SESSION_IMAGE"
	envSessionNode         = "CRSW_SESSION_NODE"
	envSessionClaim        = "CRSW_SESSION_CLAIM"
	envClaudeSecret        = "CRSW_CLAUDE_SECRET"
	envCodexSecret         = "CRSW_CODEX_SECRET"
	envSessionHome         = "CRSW_SESSION_HOME"
)

const defaultUID int64 = 10001

// Config is everything the pod template and admission read. Nothing on an
// AgentSession object shapes a pod, so nothing here comes from one.
type Config struct {
	SessionNamespace string        // where AgentSessions and session pods live
	LeaseNamespace   string        // the reconciler's own namespace (FR-009)
	Image            string        // the session image, an immutable tag
	Node             string        // kubernetes.io/hostname for nodeSelector; "" sets none
	Claim            string        // the shared ReadWriteOnce claim (FR-010)
	Cap              int           // the concurrency cap Admit enforces
	LifetimeMax      time.Duration // the lifetime ceiling Admit enforces; 0 is none
	ClaudeSecret     string        // the keeper's access-token Secret (FR-015)
	CodexSecret      string        // the namespace's own Codex login (FR-022)
	Home             string        // HOME inside the session image
	UID              int64         // runAsUser, runAsGroup and fsGroup
	Now              func() time.Time
}

// Roots is fixed: every pod's one root is where the claim's work directory is
// mounted (k8s-20c-plan E3), so an operator cannot widen it.
func (c Config) Roots() []config.ApprovedRoot {
	return []config.ApprovedRoot{{Path: sessionpod.WorkRoot}}
}

func (c Config) now() time.Time {
	if c.Now == nil {
		return time.Now()
	}
	return c.Now()
}

// ConfigFromEnv returns the first error it meets. Every error names the
// variable and never its value.
func ConfigFromEnv(lookup func(string) string) (Config, error) {
	cfg := Config{UID: defaultUID}

	cfg.SessionNamespace = lookup(envSessionNamespace)
	if cfg.SessionNamespace == "" {
		return Config{}, fmt.Errorf("reconcile: %s is empty", envSessionNamespace)
	}
	cfg.LeaseNamespace = lookup(envReconcilerNamespace)
	if cfg.LeaseNamespace == "" {
		return Config{}, fmt.Errorf("reconcile: %s is empty", envReconcilerNamespace)
	}
	// The daemon holds exec in the session namespace, so the Lease must live
	// where it does not (FR-009).
	if cfg.LeaseNamespace == cfg.SessionNamespace {
		return Config{}, fmt.Errorf("reconcile: %s must differ from %s", envReconcilerNamespace, envSessionNamespace)
	}

	cfg.Image = lookup(envSessionImage)
	if err := checkImage(cfg.Image); err != nil {
		return Config{}, err
	}

	cfg.Node = lookup(envSessionNode)
	cfg.Claim = withDefault(lookup(envSessionClaim), "crswd-sessions")

	cfg.Cap = config.DefaultMaxSessions
	if v := lookup(config.EnvMaxSessions); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("reconcile: %s must be an integer of at least 1", config.EnvMaxSessions)
		}
		cfg.Cap = n
	}

	cfg.LifetimeMax = session.AbsoluteLifetime
	if v := lookup(config.EnvSessionLifetimeMax); v != "" {
		if strings.EqualFold(v, config.NeverLifetime) {
			cfg.LifetimeMax = 0
		} else {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return Config{}, fmt.Errorf("reconcile: %s must be a positive duration or %q", config.EnvSessionLifetimeMax, config.NeverLifetime)
			}
			cfg.LifetimeMax = d
		}
	}

	cfg.ClaudeSecret = withDefault(lookup(envClaudeSecret), "claude-credentials")
	cfg.CodexSecret = withDefault(lookup(envCodexSecret), "codex-auth")

	cfg.Home = withDefault(lookup(envSessionHome), "/home/ralph")
	if !path.IsAbs(cfg.Home) {
		return Config{}, fmt.Errorf("reconcile: %s must be an absolute path", envSessionHome)
	}

	for _, n := range []struct{ env, val string }{
		{envSessionNamespace, cfg.SessionNamespace},
		{envReconcilerNamespace, cfg.LeaseNamespace},
	} {
		if len(validation.IsDNS1123Label(n.val)) != 0 {
			return Config{}, fmt.Errorf("reconcile: %s is not a valid namespace name", n.env)
		}
	}
	for _, n := range []struct{ env, val string }{
		{envSessionClaim, cfg.Claim},
		{envClaudeSecret, cfg.ClaudeSecret},
		{envCodexSecret, cfg.CodexSecret},
	} {
		if len(validation.IsDNS1123Subdomain(n.val)) != 0 {
			return Config{}, fmt.Errorf("reconcile: %s is not a valid object name", n.env)
		}
	}
	return cfg, nil
}

// checkImage refuses only the two spellings that are certainly mutable: the
// operator is trusted to pin a tag that is never re-pushed, or a digest.
func checkImage(img string) error {
	if img == "" {
		return fmt.Errorf("reconcile: %s is empty", envSessionImage)
	}
	if strings.HasSuffix(img, ":latest") {
		return fmt.Errorf("reconcile: %s must not be :latest", envSessionImage)
	}
	last := img[strings.LastIndex(img, "/")+1:]
	if !strings.ContainsAny(last, ":@") {
		return fmt.Errorf("reconcile: %s needs a tag or a digest", envSessionImage)
	}
	return nil
}

func withDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
