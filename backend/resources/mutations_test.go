package resources

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestMutatorUpdatesTriggersAndDeletesSelectedResource(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "example.test", Version: "v1", Resource: "widgets"}
	resourceType := Type{Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource, Kind: "Widget", Scope: ScopeNamespaced}
	original := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.test/v1", "kind": "Widget",
		"metadata": map[string]any{"name": "demo", "namespace": "team-a", "uid": "uid-1"},
		"spec":     map[string]any{"size": "small"},
	}}
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), original)
	mutator := NewMutator(client)
	mutator.now = func() time.Time { return time.Date(2026, 9, 20, 12, 34, 56, 0, time.UTC) }
	target := MutationTarget{UID: "uid-1", Name: "demo", Namespace: "team-a"}

	updated := []byte("apiVersion: example.test/v1\nkind: Widget\nmetadata:\n  name: demo\n  namespace: team-a\n  uid: uid-1\nspec:\n  size: large\n")
	if err := mutator.Update(context.Background(), &resourceType, target, updated); err != nil {
		t.Fatal(err)
	}
	if err := mutator.Trigger(context.Background(), &resourceType, target, "operator@example.test"); err != nil {
		t.Fatal(err)
	}
	object, err := client.Resource(gvr).Namespace("team-a").Get(context.Background(), "demo", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if size, _, _ := unstructured.NestedString(object.Object, "spec", "size"); size != "large" {
		t.Fatalf("updated size = %q", size)
	}
	if got := object.GetAnnotations()[TriggeredByAnnotation]; got != "operator@example.test" {
		t.Fatalf("triggered-by = %q", got)
	}
	if got := object.GetAnnotations()[TriggeredAtAnnotation]; got != "2026-09-20T12:34:56Z" {
		t.Fatalf("triggered-at = %q", got)
	}
	if err := mutator.Delete(context.Background(), &resourceType, target); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Resource(gvr).Namespace("team-a").Get(context.Background(), "demo", metav1.GetOptions{}); err == nil {
		t.Fatal("deleted resource still exists")
	}
}

func TestMutatorRejectsChangedIdentity(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "example.test", Version: "v1", Resource: "widgets"}
	resourceType := Type{Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource, Kind: "Widget", Scope: ScopeNamespaced}
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	mutator := NewMutator(client)
	target := MutationTarget{UID: "uid-1", Name: "demo", Namespace: "team-a"}
	changedName := []byte("apiVersion: example.test/v1\nkind: Widget\nmetadata:\n  name: another\n  namespace: team-a\n  uid: uid-1\n")
	if err := mutator.Update(context.Background(), &resourceType, target, changedName); !errors.Is(err, ErrResourceIdentityChanged) {
		t.Fatalf("identity error = %v", err)
	}
	missingUID := []byte("apiVersion: example.test/v1\nkind: Widget\nmetadata:\n  name: demo\n  namespace: team-a\n")
	if err := mutator.Update(context.Background(), &resourceType, target, missingUID); !errors.Is(err, ErrResourceIdentityChanged) {
		t.Fatalf("missing UID error = %v", err)
	}
}
