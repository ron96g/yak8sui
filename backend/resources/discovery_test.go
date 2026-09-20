package resources_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ron96g/yak8sui/backend/config"
	"github.com/ron96g/yak8sui/backend/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

type discoveryFixture struct {
	groups    []*metav1.APIGroup
	resources []*metav1.APIResourceList
	err       error
}

func (f discoveryFixture) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return f.groups, f.resources, f.err
}

func TestConfiguredDiscoverySelectsOneVersionScopeAndExclusions(t *testing.T) {
	fixture := discoveryFixture{
		groups: []*metav1.APIGroup{
			{Name: "apps.example.test", Versions: []metav1.GroupVersionForDiscovery{{Version: "v1"}, {Version: "v2"}}, PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v2"}},
			{Name: "policy.example.test", Versions: []metav1.GroupVersionForDiscovery{{Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"}},
		},
		resources: []*metav1.APIResourceList{
			{GroupVersion: "apps.example.test/v1", APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: true}}},
			{GroupVersion: "apps.example.test/v2", APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: true}, {Name: "secrets", Kind: "SecretWidget", Namespaced: true}}},
			{GroupVersion: "policy.example.test/v1", APIResources: []metav1.APIResource{{Name: "policies", Kind: "Policy"}}},
		},
	}
	catalog := resources.Discover(fixture, []config.ResourceGroupConfig{
		{Group: "apps.example.test", Exclude: []string{"secrets"}, Types: []config.ResourceTypeOverride{{Resource: "widgets", Version: "v1", LabelSelector: "inspect=true"}}},
		{Group: "policy.example.test", Include: []string{"policies", "missing"}},
	})
	types := catalog.Types()
	if len(types) != 3 {
		t.Fatalf("selected types = %#v", types)
	}
	widget, ok := catalog.Find("apps.example.test", "v1", "widgets")
	if !ok || widget.Kind != "Widget" || widget.Scope != "Namespaced" || widget.LabelSelector != "inspect=true" {
		t.Fatalf("widget type = %#v", widget)
	}
	policy, ok := catalog.Find("policy.example.test", "v1", "policies")
	if !ok || policy.Scope != "Cluster" || !policy.Available {
		t.Fatalf("policy type = %#v", policy)
	}
	missing, ok := catalog.Find("policy.example.test", "v1", "missing")
	if !ok || missing.Available || missing.Scope != "Unknown" || missing.Reason == "" {
		t.Fatalf("missing type = %#v", missing)
	}
	if _, ok := catalog.Find("apps.example.test", "v2", "secrets"); ok {
		t.Fatal("excluded resource was selected")
	}
}

func TestPartialDiscoveryStillReportsExplicitUnavailableType(t *testing.T) {
	failure := &discovery.ErrGroupDiscoveryFailed{Groups: map[schema.GroupVersion]error{{Group: "broken.example.test", Version: "v1"}: errors.New("forbidden")}}
	catalog := resources.Discover(discoveryFixture{err: failure}, []config.ResourceGroupConfig{{Group: "broken.example.test", Include: []string{"things"}, Types: []config.ResourceTypeOverride{{Resource: "things", Version: "v1"}}}})
	types := catalog.Types()
	if len(types) != 1 || types[0].Available || types[0].Version != "v1" {
		t.Fatalf("unavailable types = %#v", types)
	}
}

func TestRuntimeRejectsRelationshipTypeOutsideObservation(t *testing.T) {
	store, err := resources.Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := config.Config{Observation: config.ObservationConfig{
		Groups: []config.ResourceGroupConfig{{Group: "apps.example.test", Include: []string{"widgets"}}},
		Relationships: []config.RelationshipConfig{{
			Parent: config.RelationshipType{Group: "apps.example.test", Kind: "Widget"},
			Child:  config.RelationshipType{Group: "apps.example.test", Kind: "Unselected"},
		}},
	}}
	fixture := discoveryFixture{groups: []*metav1.APIGroup{{Name: "apps.example.test", Versions: []metav1.GroupVersionForDiscovery{{Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"}}}, resources: []*metav1.APIResourceList{{GroupVersion: "apps.example.test/v1", APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: true}, {Name: "unselected", Kind: "Unselected", Namespaced: true}}}}}
	if _, err := resources.NewRuntime(nil, fixture, &cfg, store); err == nil || !strings.Contains(err.Error(), "child apps.example.test/Unselected is not selected") {
		t.Fatalf("relationship endpoint error = %v", err)
	}
}

func TestRuntimeAllowsExplicitUnavailableRelationshipTypeWithIrregularKind(t *testing.T) {
	store, err := resources.Open(t.TempDir() + "/projection.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := config.Config{Observation: config.ObservationConfig{
		Groups: []config.ResourceGroupConfig{{Group: "apps.example.test", Include: []string{"widgets", "people"}, Types: []config.ResourceTypeOverride{{Resource: "people", Version: "v1"}}}},
		Relationships: []config.RelationshipConfig{{
			Parent: config.RelationshipType{Group: "apps.example.test", Kind: "Person"},
			Child:  config.RelationshipType{Group: "apps.example.test", Kind: "Widget"},
		}},
	}}
	fixture := discoveryFixture{groups: []*metav1.APIGroup{{Name: "apps.example.test", Versions: []metav1.GroupVersionForDiscovery{{Version: "v1"}}, PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"}}}, resources: []*metav1.APIResourceList{{GroupVersion: "apps.example.test/v1", APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: true}}}}}
	if _, err := resources.NewRuntime(nil, fixture, &cfg, store); err != nil {
		t.Fatalf("explicit unavailable relationship endpoint: %v", err)
	}
}
