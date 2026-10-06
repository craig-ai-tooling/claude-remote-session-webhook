package reconcile

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const resync = 30 * time.Second

// leaseTimings is a variable so a test can shorten it.
var leaseTimings = kube.ElectorConfig{LeaseDuration: 15 * time.Second, RenewDeadline: 10 * time.Second, RetryPeriod: 2 * time.Second}

// runLoop is a variable so a test can make the loop fail.
var runLoop = func(r *Reconciler, ctx context.Context) error { return r.Run(ctx) }

// Run watches AgentSessions and their pods and reconciles one name at a time
// until ctx ends. The 30 s resync re-runs every object, which turns a pod
// deleted while the reconciler was down into a recreate.
func (r *Reconciler) Run(ctx context.Context) error {
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	defer queue.ShutDown()

	enqueueObj := func(obj any) {
		if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			obj = d.Obj
		}
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			return
		}
		if _, name, err := cache.SplitMetaNamespaceKey(key); err == nil && name != "" {
			queue.Add(name)
		}
	}
	enqueuePod := func(obj any) {
		if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			obj = d.Obj
		}
		if p, ok := obj.(*corev1.Pod); ok && p.Labels[LabelSession] != "" {
			queue.Add(p.Labels[LabelSession])
		}
	}
	handler := func(f func(any)) cache.ResourceEventHandlerFuncs {
		return cache.ResourceEventHandlerFuncs{
			AddFunc:    f,
			UpdateFunc: func(_, n any) { f(n) },
			DeleteFunc: f,
		}
	}

	dynFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(r.dyn, resync, r.cfg.SessionNamespace, nil)
	objInformer := dynFactory.ForResource(agentsession.GVR).Informer()
	if _, err := objInformer.AddEventHandler(handler(enqueueObj)); err != nil {
		return fmt.Errorf("reconcile: object handler: %w", err)
	}

	podFactory := informers.NewSharedInformerFactoryWithOptions(r.kube, resync,
		informers.WithNamespace(r.cfg.SessionNamespace),
		informers.WithTweakListOptions(func(o *metav1.ListOptions) {
			o.LabelSelector = LabelManagedBy + "=" + ManagedByValue
		}))
	podInformer := podFactory.Core().V1().Pods().Informer()
	if _, err := podInformer.AddEventHandler(handler(enqueuePod)); err != nil {
		return fmt.Errorf("reconcile: pod handler: %w", err)
	}

	dynFactory.Start(ctx.Done())
	podFactory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), objInformer.HasSynced, podInformer.HasSynced) {
		return ctx.Err()
	}

	go func() {
		<-ctx.Done()
		queue.ShutDown()
	}()
	for {
		name, quit := queue.Get()
		if quit {
			return ctx.Err()
		}
		if err := r.ReconcileOne(ctx, name); err != nil {
			queue.AddRateLimited(name)
		} else {
			queue.Forget(name)
		}
		queue.Done(name)
	}
}

// RunWithLease runs the loop only while holding the reconciler's Lease. A loop
// that dies ends the call, so a reconciler that cannot act never keeps renewing.
//
// The worker is cancelled and joined before the Lease is released. The elector
// runs under its own context, which is cancelled only after the join, so
// ReleaseOnCancel hands the Lease back after the last write the worker could
// make. Cancelling the caller's context straight into the elector would release
// the Lease while the worker was still reconciling, and a standby could act
// beside it.
func RunWithLease(ctx context.Context, kc kubernetes.Interface, identity string, r *Reconciler) error {
	electorCtx, stopElector := context.WithCancel(context.Background())
	defer stopElector()
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()

	var (
		mu       sync.Mutex
		stopping bool
		workers  sync.WaitGroup
	)
	runErr := make(chan error, 1)

	c := leaseTimings
	c.Namespace, c.Name, c.Identity = r.cfg.LeaseNamespace, LeaseName, identity
	elector, err := kube.NewElector(kc, c, func(lctx context.Context) {
		mu.Lock()
		if stopping {
			mu.Unlock()
			return
		}
		workers.Add(1)
		mu.Unlock()
		defer workers.Done()

		// Cancelled by the elector on lease loss, or by shutdown below.
		wctx, cancel := context.WithCancel(lctx)
		defer cancel()
		defer context.AfterFunc(workerCtx, cancel)()
		if err := runLoop(r, wctx); err != nil && wctx.Err() == nil {
			select {
			case runErr <- err:
			default:
			}
		}
	}, stopWorker)
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}

	electorDone := make(chan struct{})
	go func() {
		defer close(electorDone)
		elector.Run(electorCtx)
	}()

	var result error
	select {
	case <-ctx.Done():
		result = ctx.Err()
	case result = <-runErr:
	case <-electorDone:
		// The Lease was lost or never won. The worker may still be running.
		result = ctx.Err()
	}

	mu.Lock()
	stopping = true
	mu.Unlock()
	stopWorker()
	workers.Wait()
	stopElector()
	<-electorDone
	return result
}
