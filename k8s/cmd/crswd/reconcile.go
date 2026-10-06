package main

import (
	"context"
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/kube"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/reconcile"
)

// runReconciler is the `reconcile` subcommand: the loop, run only while this
// pod holds the Lease. The configuration is read first so a bad one refuses
// before the cluster is touched.
func runReconciler(ctx context.Context) error {
	cfg, err := reconcile.ConfigFromEnv(os.Getenv)
	if err != nil {
		return err
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
	r, err := reconcile.New(kc, dyn, cfg)
	if err != nil {
		return err
	}
	// The pod name, unique per pod, so two reconcilers never share an identity.
	identity, err := os.Hostname()
	if err != nil {
		return err
	}
	return reconcile.RunWithLease(ctx, kc, identity, r)
}
