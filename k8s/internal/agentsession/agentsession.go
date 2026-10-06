// Package agentsession is the one place AgentSession objects cross the dynamic
// client. They are converted to and from api/v1alpha1 by JSON round-trip, so
// there is no generated clientset (k8s-20c-plan S4).
package agentsession

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// GVR addresses the AgentSession resource.
var GVR = schema.GroupVersionResource{Group: v1alpha1.Group, Version: v1alpha1.Version, Resource: v1alpha1.Plural}

// Client is a namespaced, typed view over the dynamic client.
type Client struct {
	dyn dynamic.Interface
	ns  string
}

// New refuses an empty namespace: a dynamic client with none addresses the
// cluster-wide collection, which a namespaced resource cannot be created in.
func New(dyn dynamic.Interface, namespace string) (*Client, error) {
	if dyn == nil {
		return nil, errors.New("agentsession: nil dynamic client")
	}
	if namespace == "" {
		return nil, errors.New("agentsession: empty namespace")
	}
	return &Client{dyn: dyn, ns: namespace}, nil
}

func (c *Client) res() dynamic.ResourceInterface {
	return c.dyn.Resource(GVR).Namespace(c.ns)
}

// Get returns false, nil when the object does not exist.
func (c *Client) Get(ctx context.Context, name string) (v1alpha1.AgentSession, bool, error) {
	u, err := c.res().Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return v1alpha1.AgentSession{}, false, nil
	}
	if err != nil {
		return v1alpha1.AgentSession{}, false, fmt.Errorf("agentsession: get %s: %w", name, err)
	}
	obj, err := ToObject(u)
	if err != nil {
		return v1alpha1.AgentSession{}, false, fmt.Errorf("agentsession: get %s: %w", name, err)
	}
	return obj, true, nil
}

// List returns every AgentSession in the namespace.
func (c *Client) List(ctx context.Context) ([]v1alpha1.AgentSession, error) {
	l, err := c.res().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("agentsession: list: %w", err)
	}
	out := make([]v1alpha1.AgentSession, 0, len(l.Items))
	for i := range l.Items {
		obj, err := ToObject(&l.Items[i])
		if err != nil {
			return nil, fmt.Errorf("agentsession: list: %w", err)
		}
		out = append(out, obj)
	}
	return out, nil
}

// Create sends obj and returns what the server stored.
func (c *Client) Create(ctx context.Context, obj v1alpha1.AgentSession) (v1alpha1.AgentSession, error) {
	u, err := FromObject(obj)
	if err != nil {
		return v1alpha1.AgentSession{}, fmt.Errorf("agentsession: create %s: %w", obj.Metadata.Name, err)
	}
	made, err := c.res().Create(ctx, u, metav1.CreateOptions{})
	if err != nil {
		return v1alpha1.AgentSession{}, fmt.Errorf("agentsession: create %s: %w", obj.Metadata.Name, err)
	}
	return ToObject(made)
}

// Delete treats NotFound as success and never overrides the grace period, so
// the pod's own termination grace applies (spec 017 FR-010).
func (c *Client) Delete(ctx context.Context, name string) error {
	err := c.res().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("agentsession: delete %s: %w", name, err)
	}
	return nil
}

// SetAnnotation is a merge patch, so it needs no resourceVersion and keeps
// every other annotation.
func (c *Client) SetAnnotation(ctx context.Context, name, key, value string) error {
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{"annotations": map[string]string{key: value}},
	})
	if err != nil {
		return fmt.Errorf("agentsession: annotate %s: %w", name, err)
	}
	if _, err := c.res().Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("agentsession: annotate %s: %w", name, err)
	}
	return nil
}

// fresh reads the object and refuses it unless it is the very object, at the
// very version, the caller computed its write from. A name alone is not an
// identity: the object may have been deleted and recreated under it, and a
// write computed from the old one would land on the new one. The refusal is a
// Conflict, which the caller requeues rather than reports.
func (c *Client) fresh(ctx context.Context, op string, from v1alpha1.AgentSession) (*unstructured.Unstructured, error) {
	name := from.Metadata.Name
	u, err := c.res().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("agentsession: %s %s: %w", op, name, err)
	}
	if string(u.GetUID()) != from.Metadata.UID {
		return nil, fmt.Errorf("agentsession: %s %s: %w", op, name,
			apierrors.NewConflict(GVR.GroupResource(), name, errors.New("the object was replaced since it was read")))
	}
	if u.GetResourceVersion() != from.Metadata.ResourceVersion {
		return nil, fmt.Errorf("agentsession: %s %s: %w", op, name,
			apierrors.NewConflict(GVR.GroupResource(), name, errors.New("the object changed since it was read")))
	}
	return u, nil
}

// UpdateStatus and Update never send a typed object back: they re-read the
// unstructured object and change only their own part. Both take the object the
// caller read and write only if the live one has the same UID and
// resourceVersion; otherwise they return a Conflict and write nothing.
func (c *Client) UpdateStatus(ctx context.Context, from v1alpha1.AgentSession, status v1alpha1.AgentSessionStatus) error {
	name := from.Metadata.Name
	u, err := c.fresh(ctx, "update status", from)
	if err != nil {
		return err
	}
	m, err := toMap(status)
	if err != nil {
		return fmt.Errorf("agentsession: update status %s: %w", name, err)
	}
	if err := unstructured.SetNestedField(u.Object, m, "status"); err != nil {
		return fmt.Errorf("agentsession: update status %s: %w", name, err)
	}
	if _, err := c.res().UpdateStatus(ctx, u, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("agentsession: update status %s: %w", name, err)
	}
	return nil
}

// Update lets mutate change metadata only. Spec is never written back, so a
// caller cannot reshape what the reconciler will run. It returns the stored
// object, so a later write in the same pass carries the new resourceVersion.
func (c *Client) Update(ctx context.Context, from v1alpha1.AgentSession, mutate func(*v1alpha1.AgentSession)) (v1alpha1.AgentSession, error) {
	name := from.Metadata.Name
	u, err := c.fresh(ctx, "update", from)
	if err != nil {
		return v1alpha1.AgentSession{}, err
	}
	obj, err := ToObject(u)
	if err != nil {
		return v1alpha1.AgentSession{}, fmt.Errorf("agentsession: update %s: %w", name, err)
	}
	mutate(&obj)
	u.SetAnnotations(obj.Metadata.Annotations)
	u.SetLabels(obj.Metadata.Labels)
	stored, err := c.res().Update(ctx, u, metav1.UpdateOptions{})
	if err != nil {
		return v1alpha1.AgentSession{}, fmt.Errorf("agentsession: update %s: %w", name, err)
	}
	out, err := ToObject(stored)
	if err != nil {
		return v1alpha1.AgentSession{}, fmt.Errorf("agentsession: update %s: %w", name, err)
	}
	return out, nil
}

// ToObject converts through JSON.
func ToObject(u *unstructured.Unstructured) (v1alpha1.AgentSession, error) {
	var obj v1alpha1.AgentSession
	raw, err := json.Marshal(u.Object)
	if err != nil {
		return obj, fmt.Errorf("agentsession: encode object: %w", err)
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return obj, fmt.Errorf("agentsession: decode object: %w", err)
	}
	return obj, nil
}

// FromObject drops a zero creationTimestamp: time.Time marshals it as year 1,
// which the API server would store as a real date.
func FromObject(obj v1alpha1.AgentSession) (*unstructured.Unstructured, error) {
	obj.APIVersion = v1alpha1.APIVersion
	obj.Kind = v1alpha1.Kind
	m, err := toMap(obj)
	if err != nil {
		return nil, err
	}
	if obj.Metadata.CreationTimestamp.IsZero() {
		if meta, ok := m["metadata"].(map[string]any); ok {
			delete(meta, "creationTimestamp")
		}
	}
	return &unstructured.Unstructured{Object: m}, nil
}

func toMap(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("agentsession: encode: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("agentsession: decode: %w", err)
	}
	return m, nil
}
