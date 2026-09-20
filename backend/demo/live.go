package demo

import (
	"encoding/json"
	"net/http"

	"github.com/ron96g/yak8sui/backend/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const observationTypeKey = widgetGroup + "/v1/" + widgetResource

// ObservationSimulationHandler interrupts the real observer attempt. Changes
// requested while interrupted are applied to the controlled upstream, then the
// normal runtime retry performs the authoritative relist.
func ObservationSimulationHandler(runtime interface {
	Interrupt(string, resources.ObservationState, string) bool
}, client dynamic.Interface,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !runtime.Interrupt(observationTypeKey, resources.ObservationUnavailable, "demo-controlled Kubernetes API connection loss") {
			http.Error(w, "observation is already interrupted or restarting", http.StatusConflict)
			return
		}
		resource := client.Resource(WidgetGVR).Namespace(platformNamespace)
		object, err := resource.Get(r.Context(), "platform-api", metav1.GetOptions{})
		if err != nil {
			http.Error(w, "could not mutate controlled upstream", http.StatusInternalServerError)
			return
		}
		resourceLabels := object.GetLabels()
		if resourceLabels["yak8sui.io/tenant"] == platformTenant {
			resourceLabels["yak8sui.io/tenant"] = "payments"
		} else {
			resourceLabels["yak8sui.io/tenant"] = platformTenant
		}
		object.SetLabels(resourceLabels)
		if _, err = resource.Update(r.Context(), object, metav1.UpdateOptions{}); err != nil {
			http.Error(w, "could not update controlled upstream", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"state": "unavailable", "nextTenant": resourceLabels["yak8sui.io/tenant"]}); err != nil {
			return
		}
	}
}

// LiveSimulationHandler exposes a loopback-demo-only control for demonstrating
// an observed Kubernetes update. It must never be mounted by the production app.
func LiveSimulationHandler(client dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resource := client.Resource(WidgetGVR).Namespace("platform-system")
		object, err := resource.Get(r.Context(), "platform-api", metav1.GetOptions{})
		if err != nil {
			http.Error(w, "could not load simulated resource", http.StatusInternalServerError)
			return
		}
		spec, _, err := unstructured.NestedMap(object.Object, "spec")
		if err != nil {
			http.Error(w, "could not read simulated resource", http.StatusInternalServerError)
			return
		}
		if spec[fieldReplicas] == int64(4) {
			spec[fieldReplicas] = int64(3)
		} else {
			spec[fieldReplicas] = int64(4)
		}
		object.Object["spec"] = spec
		generation := object.GetGeneration() + 1
		object.SetGeneration(generation)
		annotations := object.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		state := annotations["demo.yak8sui.io/health-state"]
		condition := map[string]any{fieldType: conditionReady, fieldStatus: conditionTrue, fieldReason: reasonAvailable, fieldMessage: "All replicas are available", fieldObservedGen: generation}
		switch state {
		case "stale":
			annotations["demo.yak8sui.io/health-state"] = "unhealthy"
			condition = map[string]any{fieldType: conditionReady, fieldStatus: "False", fieldReason: "ProgressDeadlineExceeded", fieldMessage: "Deployment is below its requested capacity", fieldObservedGen: generation}
		case "unhealthy":
			delete(annotations, "demo.yak8sui.io/health-state")
		default:
			annotations["demo.yak8sui.io/health-state"] = "stale"
			condition = map[string]any{fieldType: conditionReady, fieldStatus: conditionTrue, fieldReason: reasonAvailable, fieldMessage: "The previous generation was available", fieldObservedGen: generation - 1}
		}
		object.SetAnnotations(annotations)
		object.Object[fieldStatus] = map[string]any{"conditions": []any{condition}}
		updated, err := resource.Update(r.Context(), object, metav1.UpdateOptions{})
		if err != nil {
			http.Error(w, "could not update simulated resource", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{fieldUID: updated.GetUID(), fieldReplicas: spec[fieldReplicas], "healthState": annotations["demo.yak8sui.io/health-state"]}); err != nil {
			return
		}
	}
}

// SearchSimulationHandler toggles one deterministic demo resource between a
// matching and non-matching name for exercising filtered live pages.
func SearchSimulationHandler(client dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resource := client.Resource(WidgetGVR).Namespace("search-lab")
		object, err := resource.Get(r.Context(), "platform-search-104", metav1.GetOptions{})
		if err != nil {
			object, err = resource.Get(r.Context(), "platform-hidden-104", metav1.GetOptions{})
		}
		if err != nil {
			http.Error(w, "could not load simulated search resource", http.StatusInternalServerError)
			return
		}
		oldName := object.GetName()
		if object.GetName() == "platform-search-104" {
			object.SetName("platform-hidden-104")
		} else {
			object.SetName("platform-search-104")
		}
		if deleteErr := resource.Delete(r.Context(), oldName, metav1.DeleteOptions{}); deleteErr != nil {
			http.Error(w, "could not remove simulated search resource", http.StatusInternalServerError)
			return
		}
		object.SetResourceVersion("")
		updated, err := resource.Create(r.Context(), object, metav1.CreateOptions{})
		if err != nil {
			http.Error(w, "could not update simulated search resource", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if encodeErr := json.NewEncoder(w).Encode(map[string]string{"name": updated.GetName()}); encodeErr != nil {
			return
		}
	}
}

// OwnershipSimulationHandler toggles the demo Widget's UID-verified owner link.
func OwnershipSimulationHandler(client dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resource := client.Resource(WidgetGVR).Namespace("platform-system")
		object, err := resource.Get(r.Context(), "platform-api", metav1.GetOptions{})
		if err != nil {
			http.Error(w, "could not load simulated resource", http.StatusInternalServerError)
			return
		}
		linked := len(object.GetOwnerReferences()) == 0
		if linked {
			controller := true
			object.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: coreAPIVersion, Kind: configMapKind, Name: platformSettings, UID: "platform-config", Controller: &controller}})
		} else {
			object.SetOwnerReferences(nil)
		}
		if _, err = resource.Update(r.Context(), object, metav1.UpdateOptions{}); err != nil {
			http.Error(w, "could not update simulated ownership", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]bool{"linked": linked}); err != nil {
			return
		}
	}
}

// LabelJoinSimulationHandler toggles one required composite-join label.
func LabelJoinSimulationHandler(client dynamic.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resource := client.Resource(ConfigMapGVR).Namespace(platformNamespace)
		object, err := resource.Get(r.Context(), "platform-feature-flags", metav1.GetOptions{})
		if err != nil {
			http.Error(w, "could not load simulated resource", http.StatusInternalServerError)
			return
		}
		labels := object.GetLabels()
		_, linked := labels[ownerScopeLabel]
		if linked {
			delete(labels, ownerScopeLabel)
		} else {
			labels[ownerScopeLabel] = platformNamespace
		}
		object.SetLabels(labels)
		if _, err = resource.Update(r.Context(), object, metav1.UpdateOptions{}); err != nil {
			http.Error(w, "could not update simulated label ownership", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]bool{"linked": !linked}); err != nil {
			return
		}
	}
}
