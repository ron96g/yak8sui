package resources

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

const (
	TriggeredByAnnotation = "triggered-by"
	TriggeredAtAnnotation = "triggered-at"
)

var ErrResourceIdentityChanged = errors.New("resource identity does not match the selected resource")

type MutationTarget struct {
	UID       string
	Name      string
	Namespace string
}

type Mutator struct {
	client dynamic.Interface
	now    func() time.Time
}

func NewMutator(client dynamic.Interface) *Mutator {
	return &Mutator{client: client, now: time.Now}
}

func (m *Mutator) Update(ctx context.Context, resourceType *Type, target MutationTarget, contents []byte) (string, error) {
	jsonContents, err := yaml.YAMLToJSON(contents)
	if err != nil {
		return "", fmt.Errorf("decode YAML: %w", err)
	}
	object := &unstructured.Unstructured{}
	if unmarshalErr := object.UnmarshalJSON(jsonContents); unmarshalErr != nil {
		return "", fmt.Errorf("decode resource: %w", unmarshalErr)
	}
	if identityErr := validateMutationIdentity(resourceType, target, object); identityErr != nil {
		return "", identityErr
	}
	live, err := m.resource(resourceType, target.Namespace).Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if string(live.GetUID()) != target.UID {
		return "", ErrResourceIdentityChanged
	}
	updated, err := m.resource(resourceType, target.Namespace).Update(ctx, object, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}
	return updated.GetResourceVersion(), nil
}

func (m *Mutator) Delete(ctx context.Context, resourceType *Type, target MutationTarget) error {
	uid := types.UID(target.UID)
	return m.resource(resourceType, target.Namespace).Delete(ctx, target.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}})
}

func (m *Mutator) Trigger(ctx context.Context, resourceType *Type, target MutationTarget, actor string) (string, error) {
	resource := m.resource(resourceType, target.Namespace)
	object, err := resource.Get(ctx, target.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if string(object.GetUID()) != target.UID {
		return "", ErrResourceIdentityChanged
	}
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[TriggeredByAnnotation] = actor
	triggeredAt := m.now().UTC().Format(time.RFC3339Nano)
	annotations[TriggeredAtAnnotation] = triggeredAt
	object.SetAnnotations(annotations)
	_, err = resource.Update(ctx, object, metav1.UpdateOptions{})
	if err != nil {
		return "", err
	}
	return triggeredAt, nil
}

func (m *Mutator) resource(resourceType *Type, namespace string) dynamic.ResourceInterface {
	resource := m.client.Resource(resourceType.GVR())
	if resourceType.Scope == ScopeNamespaced {
		return resource.Namespace(namespace)
	}
	return resource
}

func validateMutationIdentity(resourceType *Type, target MutationTarget, object *unstructured.Unstructured) error {
	if object.GetAPIVersion() != resourceType.GVR().GroupVersion().String() || object.GetKind() != resourceType.Kind || object.GetName() != target.Name || object.GetNamespace() != target.Namespace {
		return ErrResourceIdentityChanged
	}
	if string(object.GetUID()) != target.UID {
		return ErrResourceIdentityChanged
	}
	return nil
}
