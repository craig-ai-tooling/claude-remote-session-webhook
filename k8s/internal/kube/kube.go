// Package kube holds the cluster-only helpers shared by the daemon and the
// reconciler in kubernetes mode (spec 017). It lives in the nested k8s module so
// client-go never enters the root module's dependency graph.
package kube

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// Defaults match client-go's own guidance: the lease must outlast the renew
// deadline, which must outlast several retry periods.
const (
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
)

// InCluster returns the pod's service-account config. It never falls back to a
// kubeconfig: kubernetes mode runs in a pod, and a stray ~/.kube/config on a
// host must not be picked up by accident.
func InCluster() (*rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("kube: in-cluster config: %w", err)
	}
	return cfg, nil
}

// ElectorConfig names the Lease and the candidate. Zero durations take defaults.
type ElectorConfig struct {
	Namespace, Name, Identity                 string
	LeaseDuration, RenewDeadline, RetryPeriod time.Duration
}

// NewElector builds a Lease-backed elector. ReleaseOnCancel hands the lease back
// on shutdown so a rolling update does not wait out LeaseDuration.
func NewElector(client kubernetes.Interface, c ElectorConfig, onStart func(context.Context), onStop func()) (*leaderelection.LeaderElector, error) {
	// Checked before the client is touched so a misconfigured pod fails at start.
	switch {
	case c.Namespace == "":
		return nil, errors.New("kube: elector namespace is empty")
	case c.Name == "":
		return nil, errors.New("kube: elector lease name is empty")
	case c.Identity == "":
		return nil, errors.New("kube: elector identity is empty")
	}
	if c.LeaseDuration == 0 {
		c.LeaseDuration = defaultLeaseDuration
	}
	if c.RenewDeadline == 0 {
		c.RenewDeadline = defaultRenewDeadline
	}
	if c.RetryPeriod == 0 {
		c.RetryPeriod = defaultRetryPeriod
	}
	e, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta:  metav1.ObjectMeta{Namespace: c.Namespace, Name: c.Name},
			Client:     client.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{Identity: c.Identity},
		},
		ReleaseOnCancel: true,
		LeaseDuration:   c.LeaseDuration,
		RenewDeadline:   c.RenewDeadline,
		RetryPeriod:     c.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: onStart,
			OnStoppedLeading: onStop,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("kube: new elector: %w", err)
	}
	return e, nil
}
