// Package podctl is the second tmuxctl.Controller: it drives the tmux server
// inside a session pod through the Kubernetes exec API, and keeps the
// session's metadata on the AgentSession object (k8s-20c-plan S5).
package podctl

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/sessionpod"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	"k8s.io/client-go/kubernetes"
)

const (
	// Container is the session container every exec targets.
	Container = "session"
	// Binary is where the session image carries crswd (deploy/session-image/Dockerfile).
	Binary = "/usr/local/bin/crswd"
	// AnnotationPrefix keys a tmux session option on the AgentSession.
	AnnotationPrefix = v1alpha1.Group + "/"
)

const (
	defaultReadyTimeout = 90 * time.Second // D8b cold start is 45 to 52 s
	defaultKillTimeout  = 60 * time.Second
	defaultPollInterval = time.Second
	defaultStreamIdle   = 30 * time.Second
)

// Executor runs argv in the session container of one pod. exitCode is the
// remote process's status; err is a transport or API failure, never a
// non-zero exit.
type Executor interface {
	Exec(ctx context.Context, pod string, argv []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)
}

// Describer returns the session whose tmux name is tmuxName.
type Describer func(tmuxName string) (session.PodRecord, bool)

// Config tunes a Controller. A zero duration takes its default.
type Config struct {
	Namespace    string
	PaneBound    int // config.Config.PaneBound; below 1 is an error, as tmuxctl.NewExec refuses it
	ReadyTimeout time.Duration
	KillTimeout  time.Duration
	PollInterval time.Duration
	StreamIdle   time.Duration
	Now          func() time.Time
}

var _ tmuxctl.Controller = (*Controller)(nil)

// Controller implements tmuxctl.Controller over pod exec.
type Controller struct {
	sessions *agentsession.Client
	pods     kubernetes.Interface
	exec     Executor
	describe Describer
	cfg      Config

	mu      sync.Mutex
	streams map[string]*stream
	closing map[string]int // names under Kill; CapturePane refuses them
}

// New refuses a negative duration, because a negative interval reaches
// time.NewTicker, which panics. A zero one takes its default.
func New(sessions *agentsession.Client, pods kubernetes.Interface, exec Executor, cfg Config) (*Controller, error) {
	switch {
	case sessions == nil:
		return nil, errors.New("podctl: nil AgentSession client")
	case pods == nil:
		return nil, errors.New("podctl: nil pod client")
	case exec == nil:
		return nil, errors.New("podctl: nil executor")
	case cfg.Namespace == "":
		return nil, errors.New("podctl: empty namespace")
	case cfg.PaneBound < 1:
		return nil, errors.New("podctl: PaneBound must be at least 1")
	case cfg.ReadyTimeout < 0 || cfg.KillTimeout < 0 || cfg.PollInterval < 0 || cfg.StreamIdle < 0:
		return nil, errors.New("podctl: negative duration")
	}
	if cfg.ReadyTimeout == 0 {
		cfg.ReadyTimeout = defaultReadyTimeout
	}
	if cfg.KillTimeout == 0 {
		cfg.KillTimeout = defaultKillTimeout
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = defaultPollInterval
	}
	if cfg.StreamIdle == 0 {
		cfg.StreamIdle = defaultStreamIdle
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Controller{sessions: sessions, pods: pods, exec: exec, cfg: cfg, streams: map[string]*stream{}, closing: map[string]int{}}, nil
}

// SetDescriber installs the lookup that finds a session's record.
func (c *Controller) SetDescriber(d Describer) { c.describe = d }

// annotationKey maps a tmux option such as @crswd-owner to its annotation.
func annotationKey(option string) string {
	return AnnotationPrefix + strings.TrimPrefix(option, "@crswd-")
}

// inPod points a tmuxctl.Argv* result at the session pod's tmux server. The
// wrappers carry no -L; sessionpod.Socket's comment says why.
func inPod(argv []string) []string {
	return append([]string{"tmux", "-L", sessionpod.Socket}, argv[1:]...)
}
