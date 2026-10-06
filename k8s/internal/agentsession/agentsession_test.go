package agentsession

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const testNS = "crswd-sessions"

func newClient(t *testing.T, objs ...runtime.Object) *Client {
	t.Helper()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{GVR: v1alpha1.ListKind}, objs...)
	c, err := New(dyn, testNS)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func sample(name string) v1alpha1.AgentSession {
	return v1alpha1.AgentSession{
		Metadata: v1alpha1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{"keep": "me"},
		},
		Spec: v1alpha1.AgentSessionSpec{
			SessionName:  "build",
			Owner:        "craig",
			WorkDir:      "/work/repo",
			StartCommand: "claude",
			Conversation: "0b5f4c1e-0000-4000-8000-000000000001",
			Lifetime:     "8h0m0s",
		},
	}
}

func TestNewRefusesEmptyNamespace(t *testing.T) {
	t.Parallel()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{GVR: v1alpha1.ListKind})
	if _, err := New(dyn, ""); err == nil {
		t.Fatal("New accepted an empty namespace")
	}
	if _, err := New(nil, testNS); err == nil {
		t.Fatal("New accepted a nil dynamic client")
	}
}

func TestCreateGetRoundTripsSpec(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	ctx := context.Background()
	in := sample("crswd-abc")
	if _, err := c.Create(ctx, in); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, ok, err := c.Get(ctx, "crswd-abc")
	if err != nil || !ok {
		t.Fatalf("Get = ok %v err %v", ok, err)
	}
	if !reflect.DeepEqual(got.Spec, in.Spec) {
		t.Fatalf("spec = %+v, want %+v", got.Spec, in.Spec)
	}
	if got.Metadata.Annotations["keep"] != "me" {
		t.Fatalf("annotations = %v", got.Metadata.Annotations)
	}
	if got.APIVersion != v1alpha1.APIVersion || got.Kind != v1alpha1.Kind {
		t.Fatalf("type meta = %q %q", got.APIVersion, got.Kind)
	}
}

func TestGetMissingIsFalseNil(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	_, ok, err := c.Get(context.Background(), "nope")
	if err != nil || ok {
		t.Fatalf("Get missing = ok %v err %v, want false, nil", ok, err)
	}
}

func TestListReturnsEveryObject(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	ctx := context.Background()
	for _, n := range []string{"crswd-a", "crswd-b"} {
		if _, err := c.Create(ctx, sample(n)); err != nil {
			t.Fatalf("Create %s: %v", n, err)
		}
	}
	got, err := c.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List returned %d objects, want 2", len(got))
	}
}

func TestDeleteMissingIsNil(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	if err := c.Delete(context.Background(), "nope"); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestDeleteRemovesObject(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	ctx := context.Background()
	if _, err := c.Create(ctx, sample("crswd-x")); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, "crswd-x"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := c.Get(ctx, "crswd-x"); ok {
		t.Fatal("object still there after Delete")
	}
}

func TestSetAnnotationKeepsExisting(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	ctx := context.Background()
	if _, err := c.Create(ctx, sample("crswd-x")); err != nil {
		t.Fatal(err)
	}
	if err := c.SetAnnotation(ctx, "crswd-x", "crswd.craigcloud.io/managed", "1"); err != nil {
		t.Fatalf("SetAnnotation: %v", err)
	}
	got, _, _ := c.Get(ctx, "crswd-x")
	want := map[string]string{"keep": "me", "crswd.craigcloud.io/managed": "1"}
	if !reflect.DeepEqual(got.Metadata.Annotations, want) {
		t.Fatalf("annotations = %v, want %v", got.Metadata.Annotations, want)
	}
}

func TestFromObjectZeroTimestampIsOmitted(t *testing.T) {
	t.Parallel()
	u, err := FromObject(sample("crswd-x"))
	if err != nil {
		t.Fatal(err)
	}
	meta, _ := u.Object["metadata"].(map[string]any)
	if _, has := meta["creationTimestamp"]; has {
		t.Fatalf("zero creationTimestamp was serialised: %v", meta["creationTimestamp"])
	}
	stamped := sample("crswd-x")
	stamped.Metadata.CreationTimestamp = time.Date(2026, 10, 6, 1, 2, 3, 0, time.UTC)
	u, err = FromObject(stamped)
	if err != nil {
		t.Fatal(err)
	}
	meta, _ = u.Object["metadata"].(map[string]any)
	if _, has := meta["creationTimestamp"]; !has {
		t.Fatal("a set creationTimestamp was dropped")
	}
}

func TestUpdateStatusLeavesSpecAndAnnotations(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	ctx := context.Background()
	in := sample("crswd-x")
	if _, err := c.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	st := v1alpha1.AgentSessionStatus{Phase: v1alpha1.PhaseRunning, Reason: "up", Conversation: "c1"}
	if err := c.UpdateStatus(ctx, "crswd-x", st); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _, _ := c.Get(ctx, "crswd-x")
	if got.Status != st {
		t.Fatalf("status = %+v, want %+v", got.Status, st)
	}
	if !reflect.DeepEqual(got.Spec, in.Spec) || got.Metadata.Annotations["keep"] != "me" {
		t.Fatalf("UpdateStatus disturbed spec or annotations: %+v", got)
	}
}

func TestUpdateChangesAnnotationsNotSpec(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	ctx := context.Background()
	in := sample("crswd-x")
	if _, err := c.Create(ctx, in); err != nil {
		t.Fatal(err)
	}
	err := c.Update(ctx, "crswd-x", func(o *v1alpha1.AgentSession) {
		o.Metadata.Annotations["added"] = "yes"
		o.Spec.Owner = "someone-else" // must never reach the server
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _, _ := c.Get(ctx, "crswd-x")
	if got.Metadata.Annotations["added"] != "yes" || got.Metadata.Annotations["keep"] != "me" {
		t.Fatalf("annotations = %v", got.Metadata.Annotations)
	}
	if !reflect.DeepEqual(got.Spec, in.Spec) {
		t.Fatalf("Update wrote spec: %+v", got.Spec)
	}
}

func TestUpdateMissingIsError(t *testing.T) {
	t.Parallel()
	c := newClient(t)
	if err := c.Update(context.Background(), "nope", func(*v1alpha1.AgentSession) {}); err == nil {
		t.Fatal("Update of a missing object returned nil")
	}
	if err := c.UpdateStatus(context.Background(), "nope", v1alpha1.AgentSessionStatus{}); err == nil {
		t.Fatal("UpdateStatus of a missing object returned nil")
	}
}
