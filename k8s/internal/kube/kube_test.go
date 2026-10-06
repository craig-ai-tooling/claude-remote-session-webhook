package kube

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewElectorRefusesEmptyFields(t *testing.T) {
	t.Parallel()
	full := ElectorConfig{Namespace: "ns", Name: "lock", Identity: "me"}
	cases := map[string]func(c *ElectorConfig){
		"namespace": func(c *ElectorConfig) { c.Namespace = "" },
		"name":      func(c *ElectorConfig) { c.Name = "" },
		"identity":  func(c *ElectorConfig) { c.Identity = "" },
	}
	for field, blank := range cases {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			c := full
			blank(&c)
			// A nil client proves the check runs before the client is touched.
			if _, err := NewElector(nil, c, func(context.Context) {}, func() {}); err == nil {
				t.Fatalf("empty %s was accepted", field)
			}
		})
	}
}

func TestExactlyOneLeads(t *testing.T) {
	t.Parallel()
	client := fake.NewSimpleClientset()
	var started atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for _, id := range []string{"a", "b"} {
		e, err := NewElector(client, ElectorConfig{
			Namespace: "ns", Name: "lock", Identity: id,
			LeaseDuration: 2 * time.Second, RenewDeadline: time.Second, RetryPeriod: 200 * time.Millisecond,
		}, func(context.Context) { started.Add(1) }, func() {})
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() { defer wg.Done(); e.Run(ctx) }()
	}

	time.Sleep(3 * time.Second)
	if n := started.Load(); n != 1 {
		t.Fatalf("leaders started = %d, want exactly 1", n)
	}
	if _, err := client.CoordinationV1().Leases("ns").Get(context.Background(), "lock", metav1.GetOptions{}); err != nil {
		t.Fatalf("lease not created: %v", err)
	}
	cancel()
	wg.Wait()
}
