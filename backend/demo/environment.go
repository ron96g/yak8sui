// Package demo defines the controlled identity, tenant, and Kubernetes data used
// by non-production browser fixtures and the interactive local demo.
package demo

import (
	"fmt"

	"github.com/ron96g/yak8sui/backend/config"
	"github.com/ron96g/yak8sui/backend/testidp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	AdminGroup        = "yak8sui-demo-admins"
	UserName          = "demo"
	widgetResource    = "widgets"
	policyResource    = "clusterpolicies"
	excludedResource  = "internalwidgets"
	widgetGroup       = "widgets.example.test"
	policyGroup       = "policies.example.test"
	widgetGV          = "widgets.example.test/v1"
	policyGV          = "policies.example.test/v1"
	policyBetaGV      = "policies.example.test/v1beta1"
	policyVersion     = "v1beta1"
	policyKind        = "ClusterPolicy"
	configMapKind     = "ConfigMap"
	configMapResource = "configmaps"
	widgetKind        = "Widget"
	inspectSelector   = "yak8sui.io/inspect=true"
	fieldAPIVersion   = "apiVersion"
	inspectLabel      = "yak8sui.io/inspect"
	fieldCreatedAt    = "creationTimestamp"
	fieldKind         = "kind"
	fieldLabels       = "labels"
	fieldMetadata     = "metadata"
	stringTrue        = "true"
	tenantLabel       = "yak8sui.io/tenant"
	fieldUID          = "uid"
	fieldName         = "name"
	fieldType         = "type"
	fieldReplicas     = "replicas"
	fieldStatus       = "status"
	fieldMessage      = "message"
	fieldReason       = "reason"
	conditionReady    = "Ready"
	platformTenant    = "platform"
	conditionTrue     = "True"
	reasonAvailable   = "Available"
	fieldObservedGen  = "observedGeneration"
	ownerResolver     = "ownerReferences"
)

var (
	WidgetGVR    = schema.GroupVersionResource{Group: widgetGroup, Version: "v1", Resource: widgetResource}
	PolicyGVR    = schema.GroupVersionResource{Group: policyGroup, Version: policyVersion, Resource: policyResource}
	ConfigMapGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: configMapResource}
	ExcludedGVR  = schema.GroupVersionResource{Group: widgetGroup, Version: "v1", Resource: excludedResource}
)

func Users() map[string]testidp.User {
	return map[string]testidp.User{UserName: {
		Subject: "demo-operator", Name: "Demo Operator", Email: "demo@yak8sui.local", Groups: []string{AdminGroup},
	}}
}

func Config(appAddress, appURL, issuerURL, databasePath, webDirectory string) config.Config {
	cookieSecure := false
	return config.Config{
		Server:   config.ServerConfig{Address: appAddress, PublicURL: appURL, WebDirectory: webDirectory, SessionTTLRaw: "8h", CookieSecure: &cookieSecure},
		Identity: config.IdentityConfig{IssuerURL: issuerURL, ClientID: "yak8sui-local-demo", ClientSecret: "local-demo-only", RedirectURL: appURL + "/auth/callback", GroupsClaim: "groups"},
		Observation: config.ObservationConfig{DatabasePath: databasePath, Groups: []config.ResourceGroupConfig{
			{Group: widgetGroup, Include: []string{widgetResource, excludedResource}, Exclude: []string{excludedResource}, Types: []config.ResourceTypeOverride{{Resource: widgetResource, LabelSelector: inspectSelector}}},
			{Group: policyGroup, Include: []string{policyResource}, Types: []config.ResourceTypeOverride{{Resource: policyResource, Version: policyVersion, LabelSelector: inspectSelector}}},
			{Group: "", Include: []string{configMapResource}, Types: []config.ResourceTypeOverride{{Resource: configMapResource, Version: "v1", LabelSelector: inspectSelector}}},
		}, Relationships: []config.RelationshipConfig{
			{Parent: config.RelationshipType{Group: policyGroup, Kind: policyKind}, Child: config.RelationshipType{Kind: configMapKind}, Resolver: ownerResolver},
			{Parent: config.RelationshipType{Kind: configMapKind}, Child: config.RelationshipType{Group: widgetGroup, Kind: widgetKind}, Resolver: ownerResolver},
			{Parent: config.RelationshipType{Group: widgetGroup, Kind: widgetKind}, Child: config.RelationshipType{Kind: configMapKind}, Resolver: ownerResolver},
		}},
		Tenants: []config.TenantConfig{
			{ID: platformTenant, Name: "Platform", Groups: []string{AdminGroup}, LabelSelector: "yak8sui.io/tenant=" + platformTenant},
			{ID: "payments", Name: "Payments", Groups: []string{AdminGroup}, LabelSelector: "yak8sui.io/tenant=payments"},
		},
	}
}

func Resources() []*unstructured.Unstructured {
	items := make([]*unstructured.Unstructured, 0, 110)
	items = append(items,
		widget("platform-api", "platform-api", "platform-system", "platform", map[string]any{fieldReplicas: int64(3), "image": "example/platform-api:v2"}, true, "All replicas are available"),
		widget("payments-worker", "payments-worker", "payments", "payments", map[string]any{fieldReplicas: int64(2), "queue": "settlements"}, false, "Deployment is below its requested capacity"),
		widget("internal-only", "internal-only", "internal", "internal", map[string]any{fieldMessage: "must not be visible in configured tenants"}, true, "Internal resource is ready"),
		clusterPolicy("platform-policy", "platform-guardrails", "platform"),
		clusterPolicy("payments-policy", "payments-guardrails", "payments"),
		configMap("platform-config", "platform-settings", "platform-system", "platform"),
	)
	items[0].SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: configMapKind, Name: "platform-settings", UID: "platform-config", Controller: pointer(true)}})
	items[5].SetOwnerReferences([]metav1.OwnerReference{{APIVersion: policyBetaGV, Kind: policyKind, Name: "platform-guardrails", UID: "platform-policy", Controller: pointer(true)}})
	for _, name := range []string{"platform-runtime", "platform-routes"} {
		child := configMap(name, name, "platform-system", platformTenant)
		child.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: widgetGV, Kind: widgetKind, Name: "platform-api", UID: "platform-api", Controller: pointer(true)}})
		items = append(items, child)
	}
	for index := range 105 {
		name := fmt.Sprintf("platform-search-%03d", index)
		items = append(items, widget(name, name, "search-lab", "platform", map[string]any{fieldReplicas: int64(1), "purpose": "pagination demo"}, true, "Search fixture is ready"))
	}
	return items
}

func ListKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{WidgetGVR: "WidgetList", PolicyGVR: "ClusterPolicyList", ConfigMapGVR: "ConfigMapList", ExcludedGVR: "InternalWidgetList"}
}

func DiscoveryResources() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{
		{GroupVersion: widgetGV, APIResources: []metav1.APIResource{{Name: widgetResource, Kind: widgetKind, Namespaced: true}, {Name: excludedResource, Kind: "InternalWidget", Namespaced: true}}},
		{GroupVersion: policyGV, APIResources: []metav1.APIResource{{Name: policyResource, Kind: policyKind}}},
		{GroupVersion: policyBetaGV, APIResources: []metav1.APIResource{{Name: policyResource, Kind: policyKind}}},
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: configMapResource, Kind: configMapKind, Namespaced: true}}},
	}
}

func configMap(uid, name, namespace, tenant string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{fieldAPIVersion: "v1", fieldKind: configMapKind, fieldMetadata: map[string]any{fieldUID: uid, fieldName: name, "namespace": namespace, fieldCreatedAt: "2026-09-01T11:00:00Z", fieldLabels: map[string]any{inspectLabel: stringTrue, tenantLabel: tenant}}, "data": map[string]any{"mode": "safe"}}}
}

func pointer[T any](value T) *T { return &value }

func DiscoveryGroups() []*metav1.APIGroup {
	return []*metav1.APIGroup{
		{Name: widgetGroup, Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: widgetGV, Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: widgetGV, Version: "v1"}},
		{Name: policyGroup, Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: policyGV, Version: "v1"}, {GroupVersion: policyBetaGV, Version: policyVersion}}, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: policyGV, Version: "v1"}},
	}
}

type Discovery struct{}

func (Discovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return DiscoveryGroups(), DiscoveryResources(), nil
}

var _ interface {
	ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error)
} = Discovery{}

func RuntimeObjects() []runtime.Object {
	items := Resources()
	result := make([]runtime.Object, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

func clusterPolicy(uid, name, tenant string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		fieldAPIVersion: policyBetaGV, fieldKind: policyKind,
		fieldMetadata: map[string]any{fieldUID: uid, fieldName: name, fieldCreatedAt: "2026-09-02T12:00:00Z", fieldLabels: map[string]any{inspectLabel: stringTrue, tenantLabel: tenant}},
		"spec":        map[string]any{"enforcement": "required"},
	}}
}

func widget(uid, name, namespace, tenant string, spec map[string]any, ready bool, message string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		fieldAPIVersion: widgetGV,
		fieldKind:       widgetKind,
		fieldMetadata: map[string]any{
			fieldUID: uid, fieldName: name, "namespace": namespace,
			"generation":   int64(3),
			fieldCreatedAt: "2026-09-01T12:00:00Z",
			fieldLabels:    map[string]any{inspectLabel: stringTrue, tenantLabel: tenant},
		},
		"spec": spec,
		fieldStatus: map[string]any{
			"conditions": []any{map[string]any{fieldType: conditionReady, fieldStatus: conditionStatus(ready), fieldReason: conditionReason(ready), fieldMessage: message, fieldObservedGen: int64(3)}},
		},
	}}
}

func conditionStatus(ready bool) string {
	if ready {
		return conditionTrue
	}
	return "False"
}

func conditionReason(ready bool) string {
	if ready {
		return reasonAvailable
	}
	return "ProgressDeadlineExceeded"
}
