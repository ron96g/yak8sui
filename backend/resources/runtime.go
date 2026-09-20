package resources

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ron96g/yak8sui/backend/config"
	"k8s.io/client-go/dynamic"
)

const (
	initialRetry = 250 * time.Millisecond
	maximumRetry = 30 * time.Second
)

type Runtime struct {
	catalog      *Catalog
	client       dynamic.Interface
	discovery    Discovery
	cfg          *config.Config
	store        *Projection
	controlsMu   sync.Mutex
	controls     map[string]chan ObservationStatus
	retryInitial time.Duration
	retryMaximum time.Duration
}

func NewRuntime(dynamicClient dynamic.Interface, discoveryClient Discovery, cfg *config.Config, store *Projection) (*Runtime, error) {
	catalog := Discover(discoveryClient, cfg.Observation.Groups)
	if err := validateRelationshipTypes(cfg.Observation.Relationships, catalog); err != nil {
		return nil, err
	}
	types := catalog.Types()
	for i := range types {
		status := ObservationStatus{State: ObservationResynchronizing, Reason: "backend started; waiting for authoritative relist"}
		if !types[i].Available {
			status = types[i].Observation
		}
		if err := store.SetObservationStatus(context.Background(), types[i].Key(), status); err != nil {
			return nil, fmt.Errorf("initialize observation status for %s: %w", types[i].Key(), err)
		}
	}
	return &Runtime{catalog: catalog, client: dynamicClient, discovery: discoveryClient, cfg: cfg, store: store, controls: map[string]chan ObservationStatus{}, retryInitial: initialRetry, retryMaximum: maximumRetry}, nil
}

func validateRelationshipTypes(relationships []config.RelationshipConfig, catalog *Catalog) error {
	types := catalog.Types()
	for i, relationship := range relationships {
		for role, endpoint := range map[string]config.RelationshipType{"parent": relationship.Parent, "child": relationship.Child} {
			if _, ok := catalog.FindByGroupKind(endpoint.Group, endpoint.Kind); ok {
				continue
			}
			endpointMayBeUnavailable := false
			for i := range types {
				if types[i].Group == endpoint.Group && !types[i].Available {
					endpointMayBeUnavailable = true
					break
				}
			}
			if catalog.KindDiscovered(endpoint.Group, endpoint.Kind) || !endpointMayBeUnavailable {
				return fmt.Errorf("observation.relationships[%d].%s %s/%s is not selected for observation", i, role, endpoint.Group, endpoint.Kind)
			}
		}
	}
	return nil
}

func (r *Runtime) Catalog() *Catalog { return r.catalog }
func (r *Runtime) Mutator() *Mutator { return NewMutator(r.client) }

func (r *Runtime) Run(ctx context.Context) error {
	types := r.catalog.Types()
	for i := range types {
		go r.runType(ctx, &types[i]) // #nosec G118 -- runType exits with the runtime context.
	}
	<-ctx.Done()
	return nil
}

func (r *Runtime) runType(ctx context.Context, initial *Type) {
	selected := *initial
	backoff := r.retryInitial
	rediscover := !selected.Available
	for ctx.Err() == nil {
		if rediscover {
			resolved := r.resolve(&selected)
			previousKey := selected.Key()
			if selected.Version != "" && resolved.Version != selected.Version {
				resolved = selected
				resolved.Available = false
				resolved.Reason = fmt.Sprintf("configured selection remains on %s while it is not served", selected.Version)
			}
			selected = resolved
			r.catalog.replaceType(previousKey, &selected)
		}
		if !selected.Available {
			status := ObservationStatus{State: ObservationUnavailable, Reason: selected.Reason}
			r.setStatus(selected.Key(), status)
			if !waitRetry(ctx, backoff) {
				return
			}
			backoff = min(backoff*2, r.retryMaximum)
			rediscover = true
			continue
		}
		observer, err := NewObserver(r.client, &selected, r.cfg.Tenants, r.store)
		if err == nil {
			var synchronized atomic.Bool
			observer.loss = r.control(selected.Key())
			observer.onSynchronized = func() {
				synchronized.Store(true)
				r.catalog.setObservation(selected.Key(), ObservationStatus{State: ObservationCurrent})
			}
			err = observer.RunOnce(ctx)
			if synchronized.Load() {
				backoff = r.retryInitial
			}
		}
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			rediscover = true
			status, statusErr := r.store.ObservationStatus(context.Background(), selected.Key())
			if statusErr != nil || status.State == ObservationCurrent || status.State == ObservationResynchronizing {
				status = ObservationStatus{State: ObservationStale, Reason: err.Error()}
				r.setStatus(selected.Key(), status)
			} else {
				r.catalog.setObservation(selected.Key(), status)
			}
			slog.Warn("resource observation will retry", "type", selected.Key(), "error", err, "after", backoff)
		}
		if !waitRetry(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, r.retryMaximum)
	}
}

// SetRetryBounds configures deterministic short retries for controlled tests
// and demos. Production keeps the bounded defaults.
func (r *Runtime) SetRetryBounds(initial, maximum time.Duration) {
	if initial > 0 {
		r.retryInitial = initial
	}
	if maximum >= initial {
		r.retryMaximum = maximum
	}
}

// Interrupt is a controlled lifecycle seam for deterministic tests and the
// loopback-only demo. Production observation loss enters through the same
// per-attempt cancellation path from client-go's watch error handler.
func (r *Runtime) Interrupt(typeKey string, state ObservationState, reason string) bool {
	r.controlsMu.Lock()
	control, ok := r.controls[typeKey]
	r.controlsMu.Unlock()
	if !ok {
		return false
	}
	select {
	case control <- ObservationStatus{State: state, Reason: reason}:
		return true
	default:
		return false
	}
}

func (r *Runtime) control(typeKey string) chan ObservationStatus {
	r.controlsMu.Lock()
	defer r.controlsMu.Unlock()
	control := make(chan ObservationStatus, 1)
	r.controls[typeKey] = control
	return control
}

func (r *Runtime) resolve(selected *Type) Type {
	latest := Discover(r.discovery, r.cfg.Observation.Groups)
	types := latest.Types()
	for i := range types {
		if types[i].Group == selected.Group && types[i].Resource == selected.Resource {
			return types[i]
		}
	}
	result := *selected
	result.Available = false
	result.Reason = "configured resource type is not currently selected by discovery"
	return result
}

func (r *Runtime) setStatus(key string, status ObservationStatus) {
	if err := r.store.SetObservationStatus(context.Background(), key, status); err != nil {
		slog.Error("persist observation status", "type", key, "error", err)
	}
	r.catalog.setObservation(key, status)
}

func waitRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
