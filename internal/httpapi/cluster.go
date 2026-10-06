package httpapi

import (
	"context"
	"errors"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

// ClusterHooks are the session.Manager hooks for sessions that live in pods.
// Either may be nil, which leaves the host's own lookup in place.
type ClusterHooks struct {
	CodexConversation func(ctx context.Context, s session.Session) (string, error)

	// HasTranscript is handed the session's runtime by NewForCluster, which reads
	// it from the manager: a Session carries no runtime field of its own.
	HasTranscript func(ctx context.Context, s session.Session, h harness.Name, id string) (bool, error)

	// StartDeadline is how long a create route may take to answer. A pod-backed
	// create waits for its pod, so it has to outlast the controller's ready
	// timeout. Zero takes defaultStartDeadline.
	StartDeadline time.Duration
}

// defaultStartDeadline covers podctl's 90 second ready timeout with room for a
// cold image pull and the API round trips around it.
const defaultStartDeadline = 3 * time.Minute

// NewForCluster is the cluster build's constructor (spec 017). It builds on
// NewWith, which wires no journal, no sign-in relay and no release feed: the
// object that describes a session is its record, and the image is replaced
// rather than self-updated (FR-003).
//
// The working directory is checked lexically because the daemon cannot see a
// pod's filesystem; the pod re-checks the path with symlinks resolved.
func NewForCluster(cfg *config.Config, ctl tmuxctl.Controller, hooks ClusterHooks) (*Server, error) {
	switch {
	case cfg == nil:
		return nil, errors.New("httpapi: no configuration provided; refusing to start")
	case !cfg.ExecutionMode.Kubernetes():
		return nil, errors.New("httpapi: NewForCluster is for kubernetes mode; a host daemon is built by New")
	case ctl == nil:
		return nil, errors.New("httpapi: no session controller provided; refusing to start")
	}
	srv, err := NewWith(cfg, ctl, audit.New())
	if err != nil {
		return nil, err
	}
	srv.slowStartDeadline = hooks.StartDeadline
	if srv.slowStartDeadline == 0 {
		srv.slowStartDeadline = defaultStartDeadline
	}
	srv.sessions.SetWorkDirResolver(session.LexicalWorkDir)
	if hooks.CodexConversation != nil {
		srv.sessions.SetCodexConversationFinder(hooks.CodexConversation)
	}
	if hooks.HasTranscript != nil {
		srv.sessions.SetTranscriptChecker(func(ctx context.Context, s session.Session, id string) (bool, error) {
			return hooks.HasTranscript(ctx, s, srv.sessions.SpecOf(s).Name, id)
		})
	}
	return srv, nil
}

// PodRecord is what a pod's object needs to know about a session, by its tmux
// name. The cluster daemon hands it to the pod-backed controller.
func (s *Server) PodRecord(name string) (session.PodRecord, bool) {
	return s.sessions.PodRecord(name)
}
