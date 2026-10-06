package main

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	"github.com/nctiggy/claude-remote-session-webhook/internal/httpapi"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/kube"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/podctl"
)

const shutdownBudget = 30 * time.Second

// runDaemon is the cluster daemon: the dashboard and API over a pod-backed
// controller. It starts in the order cmd/crswd/main.go run() does, from
// Reconcile on, and ends the same way (FR-040).
func runDaemon(ctx context.Context, stderr io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !cfg.ExecutionMode.Kubernetes() {
		return errors.New("this is the cluster build of crswd and runs only with CRSW_EXECUTION_MODE=kubernetes; on a host, install the release binary (install.sh)")
	}
	// Proves the switch is on: main flips it before anything else.
	if err := cfg.ExecutionMode.Runnable(); err != nil {
		return err
	}
	if err := cfg.CheckDependencies(stderr); err != nil {
		return err
	}
	// The variable the reconciler reads too, so one name is one namespace.
	ns := os.Getenv("CRSW_SESSION_NAMESPACE")
	if ns == "" {
		return errors.New("CRSW_SESSION_NAMESPACE is empty")
	}

	rc, err := kube.InCluster()
	if err != nil {
		return err
	}
	kc, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return err
	}
	dyn, err := dynamic.NewForConfig(rc)
	if err != nil {
		return err
	}
	sessions, err := agentsession.New(dyn, ns)
	if err != nil {
		return err
	}
	exec := &podctl.RemoteExecutor{Config: rc, Client: kc, Namespace: ns}
	podCfg := podctl.Config{Namespace: ns, PaneBound: cfg.PaneBound}
	ctl, err := podctl.New(sessions, kc, exec, podCfg)
	if err != nil {
		return err
	}
	srv, err := httpapi.NewForCluster(cfg, ctl, httpapi.ClusterHooks{
		StartDeadline: startDeadline(podCfg.ReadyTimeout),
		CodexConversation: func(ctx context.Context, s session.Session) (string, error) {
			return ctl.CodexConversation(ctx, s.TmuxName())
		},
		HasTranscript: func(ctx context.Context, s session.Session, h harness.Name, id string) (bool, error) {
			return ctl.HasTranscript(ctx, s.TmuxName(), h, id, s.WorkDir)
		},
	})
	if err != nil {
		return err
	}
	ctl.SetDescriber(srv.PodRecord)

	if err := srv.Reconcile(ctx); err != nil {
		return err
	}
	if err := srv.StartReaper(ctx); err != nil {
		return err
	}
	if err := srv.StartSupervisor(ctx); err != nil {
		return err
	}
	if err := srv.Listen(); err != nil {
		return err
	}

	serving := make(chan error, 1)
	go func() { serving <- srv.Serve() }()

	var serveErr error
	stoppedOnItsOwn := false
	select {
	case serveErr = <-serving:
		stoppedOnItsOwn = true
	case <-ctx.Done():
	}

	// Not derived from ctx: it is already done, and a teardown on a cancelled
	// context would reach no pod.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownBudget)
	defer cancel()
	shutdownErr := srv.Shutdown(shutdownCtx)
	if !stoppedOnItsOwn {
		serveErr = <-serving
	}
	return errors.Join(serveErr, shutdownErr)
}

// startDeadline is how long a create may take to answer: the time New waits for
// the pod, plus a minute for the API calls around it. A zero ready timeout is
// podctl's default, because that is what podctl.New applied.
func startDeadline(ready time.Duration) time.Duration {
	if ready == 0 {
		ready = podctl.DefaultReadyTimeout
	}
	return ready + time.Minute
}
