package httpapi

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/audit"
	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

const clusterTestOwner = "operator@example.com"

// clusterConfig is testConfig in kubernetes mode over a real directory. The
// root is resolved for the reason newSessionFixture's is.
func clusterConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve the root: %v", err)
	}
	cfg := testConfig("127.0.0.1:0")
	cfg.ExecutionMode = config.ExecutionModeKubernetes
	cfg.Roots = []config.ApprovedRoot{{Path: root}}
	cfg.StartCommands = config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: "claude --dangerously-skip-permissions",
		"codex":                        "codex --dangerously-bypass-approvals-and-sandbox",
	})
	return cfg, root
}

func TestNewForClusterRefusesHostMode(t *testing.T) {
	t.Parallel()

	cfg := testConfig("127.0.0.1:0")
	_, err := NewForCluster(cfg, tmuxctl.NewFake(), ClusterHooks{})
	if err == nil || !strings.Contains(err.Error(), "New") {
		t.Fatalf("NewForCluster(host config) = %v, want an error naming New", err)
	}
}

func TestNewForClusterRefusesNilArguments(t *testing.T) {
	t.Parallel()

	cfg, _ := clusterConfig(t)
	if _, err := NewForCluster(nil, tmuxctl.NewFake(), ClusterHooks{}); err == nil {
		t.Error("NewForCluster(nil config) = nil error")
	}
	if _, err := NewForCluster(cfg, nil, ClusterHooks{}); err == nil {
		t.Error("NewForCluster(nil controller) = nil error")
	}
}

func TestNewForClusterChecksWorkDirLexically(t *testing.T) {
	t.Parallel()

	cfg, root := clusterConfig(t)
	absent := filepath.Join(root, "absent")
	req := session.CreateRequest{Owner: clusterTestOwner, Name: "probe", WorkDir: absent}

	srv, err := NewForCluster(cfg, tmuxctl.NewFake(), ClusterHooks{})
	if err != nil {
		t.Fatalf("NewForCluster() = %v", err)
	}
	if _, _, err := srv.sessions.Create(context.Background(), req); err != nil {
		t.Errorf("Create(%q) through NewForCluster = %v, want success: the pod owns the filesystem", absent, err)
	}

	plain, err := NewWith(cfg, tmuxctl.NewFake(), audit.New())
	if err != nil {
		t.Fatalf("NewWith() = %v", err)
	}
	if _, _, err := plain.sessions.Create(context.Background(), req); !errors.Is(err, session.ErrInvalidWorkDir) {
		t.Errorf("Create(%q) through NewWith = %v, want ErrInvalidWorkDir", absent, err)
	}
}

func TestNewForClusterHasNoRelayOrFeed(t *testing.T) {
	t.Parallel()

	cfg, _ := clusterConfig(t)
	srv, err := NewForCluster(cfg, tmuxctl.NewFake(), ClusterHooks{})
	if err != nil {
		t.Fatalf("NewForCluster() = %v", err)
	}
	if len(srv.signins) != 0 {
		t.Errorf("sign-in relays = %d, want none (FR-003)", len(srv.signins))
	}
	if srv.releaseFeed != nil {
		t.Error("release feed is wired, want none (FR-003)")
	}
}

func TestNewForClusterWiresHooks(t *testing.T) {
	t.Parallel()

	cfg, root := clusterConfig(t)
	fake := tmuxctl.NewFake()
	const id = "3f1b2c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"

	var found int
	var asked struct {
		harness harness.Name
		id      string
	}
	srv, err := NewForCluster(cfg, fake, ClusterHooks{
		CodexConversation: func(context.Context, session.Session) (string, error) {
			found++
			return id, nil
		},
		HasTranscript: func(_ context.Context, _ session.Session, h harness.Name, convo string) (bool, error) {
			asked.harness, asked.id = h, convo
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("NewForCluster() = %v", err)
	}
	sup, err := session.NewSupervisor(srv.sessions, audit.NewTo(&bytes.Buffer{}, time.Now))
	if err != nil {
		t.Fatalf("NewSupervisor() = %v", err)
	}

	cx, _, err := srv.sessions.Create(context.Background(), session.CreateRequest{
		Owner: clusterTestOwner, Name: "codex-one", WorkDir: filepath.Join(root, "a"), StartCommand: "codex",
	})
	if err != nil {
		t.Fatalf("Create(codex) = %v", err)
	}
	fake.SetPaneCommand(cx.TmuxName(), "codex")
	if err := sup.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep() = %v", err)
	}
	if found != 1 {
		t.Errorf("the conversation finder ran %d times, want 1", found)
	}

	cl, _, err := srv.sessions.Create(context.Background(), session.CreateRequest{
		Owner: clusterTestOwner, Name: "claude-one", WorkDir: filepath.Join(root, "b"),
	})
	if err != nil {
		t.Fatalf("Create(claude) = %v", err)
	}
	fake.SetPaneCommand(cl.TmuxName(), "bash")
	if err := sup.Sweep(context.Background()); err != nil {
		t.Logf("second Sweep() = %v; only who the hook was asked matters here", err)
	}
	if asked.harness != harness.Claude || asked.id != cl.ConversationID || asked.id == "" {
		t.Errorf("the transcript hook saw (%q, %q), want (%q, %q)", asked.harness, asked.id, harness.Claude, cl.ConversationID)
	}
}
