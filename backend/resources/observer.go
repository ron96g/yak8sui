package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ron96g/yak8sui/backend/config"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
	dynamicinformer "k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type Observer struct {
	resource       Type
	typeRule       labels.Selector
	tenants        []tenantSelector
	store          *Projection
	client         dynamic.Interface
	loss           <-chan ObservationStatus
	onSynchronized func()
}

type tenantSelector struct {
	id       string
	selector labels.Selector
}

func NewObserver(client dynamic.Interface, resource *Type, tenants []config.TenantConfig, store *Projection) (*Observer, error) {
	tenantSelectors := make([]tenantSelector, 0, len(tenants))
	for _, tenant := range tenants {
		selector, err := labels.Parse(tenant.LabelSelector)
		if err != nil {
			return nil, fmt.Errorf("parse tenant selector %q: %w", tenant.ID, err)
		}
		tenantSelectors = append(tenantSelectors, tenantSelector{id: tenant.ID, selector: selector})
	}
	typeRule, err := labels.Parse(resource.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("parse resource type selector: %w", err)
	}
	return &Observer{resource: *resource, typeRule: typeRule, tenants: tenantSelectors, store: store, client: client}, nil
}

// RunOnce establishes a new list/watch boundary. A prior HasSynced value is
// never reused: recovery always creates a fresh informer and authoritative list.
func (o *Observer) RunOnce(ctx context.Context) error {
	attempt, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	if err := o.store.SetObservationStatus(ctx, o.resource.Key(), ObservationStatus{State: ObservationResynchronizing, Reason: "relisting configured resource type"}); err != nil {
		return fmt.Errorf("mark observation resynchronizing: %w", err)
	}
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(o.client, 0, "", func(options *metav1.ListOptions) {
		options.LabelSelector = o.resource.LabelSelector
	})
	informer := factory.ForResource(o.resource.GVR()).Informer()
	var gate sync.Mutex
	live := false
	project := func(value any, deleted bool) {
		gate.Lock()
		defer gate.Unlock()
		if !live || attempt.Err() != nil {
			return
		}
		object, ok := observedObject(value)
		if !ok {
			o.fail(cancel, errors.New("observer received an unexpected object"))
			return
		}
		var err error
		if deleted {
			err = o.store.Delete(attempt, o.resource.Key(), string(object.GetUID()))
		} else {
			var observed ObservedResource
			observed, err = o.project(object)
			if err == nil {
				err = o.store.Replace(attempt, o.resource.Key(), observed.UID, observed.Namespace, observed.Name, observed.ResourceVersion, observed.Contents, observed.TenantIDs)
			}
		}
		if err != nil && attempt.Err() == nil {
			o.fail(cancel, fmt.Errorf("update latest-state projection: %w", err))
		}
	}
	if err := o.configureInformer(informer, project, cancel); err != nil {
		return err
	}
	go informer.Run(attempt.Done())
	o.listenForControlledLoss(attempt, cancel)
	if !cache.WaitForCacheSync(attempt.Done(), informer.HasSynced) {
		if cause := context.Cause(attempt); cause != nil {
			return cause
		}
		return fmt.Errorf("synchronize observation for %s", o.resource.Key())
	}
	gate.Lock()
	snapshot, err := o.snapshot(informer.GetStore().List())
	if err != nil {
		gate.Unlock()
		return err
	}
	if err := o.store.ReplaceSnapshot(attempt, o.resource.Key(), snapshot); err != nil {
		gate.Unlock()
		return fmt.Errorf("commit authoritative observation snapshot: %w", err)
	}
	live = true
	if o.onSynchronized != nil {
		o.onSynchronized()
	}
	gate.Unlock()
	<-attempt.Done()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return context.Cause(attempt)
}

func (o *Observer) listenForControlledLoss(ctx context.Context, cancel context.CancelCauseFunc) {
	if o.loss == nil {
		return
	}
	go func() { // #nosec G118 -- goroutine exits with the attempt context.
		select {
		case status := <-o.loss:
			if err := o.store.SetObservationStatus(ctx, o.resource.Key(), status); err != nil && ctx.Err() == nil {
				cancel(fmt.Errorf("persist controlled observation loss: %w", err))
				return
			}
			cancel(errors.New(status.Reason))
		case <-ctx.Done():
		}
	}()
}

func (o *Observer) snapshot(values []any) ([]ObservedResource, error) {
	snapshot := make([]ObservedResource, 0, len(values))
	for _, value := range values {
		object, ok := value.(*unstructured.Unstructured)
		if !ok {
			return nil, errors.New("initial observation contained an unexpected object")
		}
		observed, err := o.project(object)
		if err != nil {
			return nil, err
		}
		snapshot = append(snapshot, observed)
	}
	return snapshot, nil
}

func (o *Observer) configureInformer(informer cache.SharedIndexInformer, project func(any, bool), cancel context.CancelCauseFunc) error {
	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(value any) { project(value, false) },
		UpdateFunc: func(_, value any) { project(value, false) },
		DeleteFunc: func(value any) { project(value, true) },
	}); err != nil {
		return fmt.Errorf("register resource observer: %w", err)
	}
	if err := informer.SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
		if !isExpectedWatchClosure(err) {
			o.fail(cancel, fmt.Errorf("kubernetes list/watch failed: %w", err))
		}
	}); err != nil {
		return fmt.Errorf("register observation error handler: %w", err)
	}
	return nil
}

func (o *Observer) fail(cancel context.CancelCauseFunc, err error) {
	status := ObservationStale
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) {
		status = ObservationUnavailable
	}
	if statusErr := o.store.SetObservationStatus(context.Background(), o.resource.Key(), ObservationStatus{State: status, Reason: err.Error()}); statusErr != nil {
		err = errors.Join(err, fmt.Errorf("persist observation failure: %w", statusErr))
	}
	cancel(err)
}

func (o *Observer) project(object *unstructured.Unstructured) (ObservedResource, error) {
	contents, err := json.Marshal(object.Object)
	if err != nil {
		return ObservedResource{}, fmt.Errorf("encode observed resource: %w", err)
	}
	matching := []string{}
	resourceLabels := labels.Set(object.GetLabels())
	if o.typeRule.Matches(resourceLabels) {
		for _, tenant := range o.tenants {
			if tenant.selector.Matches(resourceLabels) {
				matching = append(matching, tenant.id)
			}
		}
	}
	return ObservedResource{UID: string(object.GetUID()), Namespace: object.GetNamespace(), Name: object.GetName(), ResourceVersion: object.GetResourceVersion(), Contents: contents, TenantIDs: matching}, nil
}

func observedObject(value any) (*unstructured.Unstructured, bool) {
	if object, ok := value.(*unstructured.Unstructured); ok {
		return object, true
	}
	if tombstone, ok := value.(cache.DeletedFinalStateUnknown); ok {
		object, objectOK := tombstone.Obj.(*unstructured.Unstructured)
		return object, objectOK
	}
	return nil, false
}

func isExpectedWatchClosure(err error) bool {
	status, ok := err.(apierrors.APIStatus)
	if !ok {
		return false
	}
	details := status.Status()
	return details.Reason == metav1.StatusReasonTimeout || details.Code == 504
}
