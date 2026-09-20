package server_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ron96g/yak8sui/backend/auth"
	"github.com/ron96g/yak8sui/backend/config"
	"github.com/ron96g/yak8sui/backend/demo"
	"github.com/ron96g/yak8sui/backend/resources"
	appserver "github.com/ron96g/yak8sui/backend/server"
	"github.com/ron96g/yak8sui/backend/testidp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

type environment struct {
	app, idp     *httptest.Server
	client       *http.Client
	kube         dynamic.Interface
	gvr          schema.GroupVersionResource
	policyGVR    schema.GroupVersionResource
	configMapGVR schema.GroupVersionResource
	store        *resources.Projection
	observation  *resources.Runtime
}

type streamSnapshot struct {
	Items     []resources.Summary `json:"items"`
	Total     int                 `json:"total"`
	Page      int                 `json:"page"`
	PageCount int                 `json:"pageCount"`
}

type staticDiscovery struct {
	groups    []*metav1.APIGroup
	resources []*metav1.APIResourceList
}

func (d staticDiscovery) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return d.groups, d.resources, nil
}

func newEnvironment(t *testing.T, ttl time.Duration) environment {
	return newEnvironmentWithFieldPath(t, ttl, "$.spec.policyRef")
}

func newEnvironmentWithFieldPath(t *testing.T, ttl time.Duration, fieldPath string) environment {
	return newEnvironmentConfigured(t, ttl, func(cfg *config.Config) {
		cfg.Observation.Relationships[2].FieldReference.Path = fieldPath
	})
}

func newEnvironmentConfigured(t *testing.T, ttl time.Duration, configure func(*config.Config)) environment {
	t.Helper()
	users := map[string]testidp.User{"allowed": {Subject: "1", Name: "Allowed User", Email: "allowed@example.test", Groups: []string{"platform-admins"}}, "denied": {Subject: "2", Name: "Denied User", Groups: []string{"unrelated"}}, "payments": {Subject: "3", Name: "Payments User", Groups: []string{"payments-viewers"}}}
	provider, err := testidp.New("", users, "allowed")
	if err != nil {
		t.Fatal(err)
	}
	idp := httptest.NewServer(provider.Handler())
	provider.SetIssuer(idp.URL)
	cfg := config.Config{Server: config.ServerConfig{PublicURL: "http://placeholder", WebDirectory: t.TempDir(), SessionTTLRaw: ttl.String()}, Identity: config.IdentityConfig{IssuerURL: idp.URL, ClientID: "test-client", ClientSecret: "test-secret", GroupsClaim: "groups"}, Observation: config.ObservationConfig{DatabasePath: t.TempDir() + "/resources.db", Groups: []config.ResourceGroupConfig{
		{Group: "widgets.example.test", Include: []string{"widgets", "internalwidgets"}, Exclude: []string{"internalwidgets"}, Types: []config.ResourceTypeOverride{{Resource: "widgets", LabelSelector: "inspect=true"}}},
		{Group: "policies.example.test", Include: []string{"clusterpolicies", "temporarilyabsent"}, Types: []config.ResourceTypeOverride{{Resource: "clusterpolicies", Version: "v1beta1", LabelSelector: "inspect=true"}, {Resource: "temporarilyabsent", Version: "v1"}}},
		{Group: "", Include: []string{"configmaps"}, Types: []config.ResourceTypeOverride{{Resource: "configmaps", Version: "v1", LabelSelector: "inspect=true"}}},
	}, Relationships: []config.RelationshipConfig{
		{Parent: config.RelationshipType{Group: "", Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "widgets.example.test", Kind: "Widget"}},
		{Parent: config.RelationshipType{Group: "policies.example.test", Kind: "ClusterPolicy"}, Child: config.RelationshipType{Group: "widgets.example.test", Kind: "Widget"}, Resolver: "labelJoin", LabelJoin: &config.LabelJoinConfig{Labels: []config.LabelJoinMapping{{ParentLabel: "owner", ChildLabel: "policy"}, {ParentLabel: "scope", ChildLabel: "policy-scope"}}}},
		{Parent: config.RelationshipType{Group: "policies.example.test", Kind: "ClusterPolicy"}, Child: config.RelationshipType{Group: "", Kind: "ConfigMap"}, Resolver: "fieldReference", FieldReference: &config.FieldReferenceConfig{Path: "$.spec.policyRef"}},
	}}, Tenants: []config.TenantConfig{{ID: "platform", Name: "Platform", Groups: []string{"platform-admins"}, LabelSelector: "tenant in (platform,shared)"}, {ID: "payments", Name: "Payments", Groups: []string{"payments-viewers", "platform-admins"}, LabelSelector: "tenant in (payments,shared)"}}}
	if configure != nil {
		configure(&cfg)
	}
	cfg.Identity.RedirectURL = "http://placeholder/auth/callback"
	if validateErr := cfg.Validate(); validateErr != nil {
		t.Fatal(validateErr)
	}
	app := httptest.NewUnstartedServer(nil)
	appURL := "http://" + app.Listener.Addr().String()
	cfg.Server.PublicURL = appURL
	cfg.Identity.RedirectURL = appURL + "/auth/callback"
	oidc, err := auth.NewOIDC(t.Context(), &cfg.Identity)
	if err != nil {
		t.Fatal(err)
	}
	store, err := resources.Open(cfg.Observation.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	gvr := schema.GroupVersionResource{Group: "widgets.example.test", Version: "v1", Resource: "widgets"}
	policyGVR := schema.GroupVersionResource{Group: "policies.example.test", Version: "v1beta1", Resource: "clusterpolicies"}
	configMapGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	kube := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "WidgetList", policyGVR: "ClusterPolicyList", configMapGVR: "ConfigMapList"})
	discovery := staticDiscovery{groups: demo.DiscoveryGroups(), resources: demo.DiscoveryResources()}
	observation, err := resources.NewRuntime(kube, discovery, &cfg, store)
	if err != nil {
		t.Fatal(err)
	}
	observation.SetRetryBounds(100*time.Millisecond, 200*time.Millisecond)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go func() { _ = observation.Run(ctx) }()
	app.Config.Handler = appserver.New(&cfg, oidc, auth.NewSessions(ttl), store, observation.Catalog()).Handler()
	app.Start()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	t.Cleanup(app.Close)
	t.Cleanup(idp.Close)
	return environment{app: app, idp: idp, client: client, kube: kube, gvr: gvr, policyGVR: policyGVR, configMapGVR: configMapGVR, store: store, observation: observation}
}

func TestPublicHTTPFieldReferenceSupportsStatusPath(t *testing.T) {
	env := newEnvironmentWithFieldPath(t, time.Hour, "$.status.policyRef")
	env.login(t, "allowed")
	parent := clusterPolicy("status-policy", "status-policy", "platform", true)
	child := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "status-child", "name": "status-child", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}, "status": map[string]any{"policyRef": map[string]any{"apiVersion": "policies.example.test/v1beta1", "kind": "ClusterPolicy", "name": "status-policy", "uid": "status-policy"}}}}
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), parent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "status-child")
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/policies.example.test/v1beta1/clusterpolicies", "status-policy")
	response := get(t, env.client, env.app.URL+"/api/v1/tenants/platform/relationships/_core/v1/configmaps/status-child")
	var result resources.Neighborhood
	decode(t, response, &result)
	if response.StatusCode != http.StatusOK || len(result.Parents) != 1 || result.Parents[0].UID != "status-policy" {
		t.Fatalf("status field-reference neighborhood = %#v", result)
	}
}

func TestPublicHTTPFieldReferenceOwnershipIsExactAuthorizedAndLive(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	parent := clusterPolicy("policy-uid", "guardrails", "platform", true)
	hidden := clusterPolicy("hidden-uid", "hidden", "payments", true)
	child := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "field-child", "name": "settings", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}, "spec": map[string]any{"policyRef": map[string]any{"apiVersion": "policies.example.test/v1beta1", "kind": "ClusterPolicy", "name": "guardrails", "uid": "policy-uid"}}}}
	for _, item := range []*unstructured.Unstructured{parent, hidden} {
		if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "field-child")
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/policies.example.test/v1beta1/clusterpolicies", "policy-uid")
	childURL := env.app.URL + "/api/v1/tenants/platform/relationships/_core/v1/configmaps/field-child"
	parentURL := env.app.URL + "/api/v1/tenants/platform/relationships/policies.example.test/v1beta1/clusterpolicies/policy-uid"
	assert := func(url string) resources.Neighborhood {
		t.Helper()
		response := get(t, env.client, url)
		var result resources.Neighborhood
		decode(t, response, &result)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("relationship status = %d: %#v", response.StatusCode, result)
		}
		return result
	}
	if result := assert(childURL); len(result.Parents) != 1 || result.Parents[0].UID != "policy-uid" {
		t.Fatalf("field-reference parents = %#v", result)
	}
	if result := assert(parentURL); len(result.Children) != 1 || result.Children[0].UID != "field-child" {
		t.Fatalf("field-reference children = %#v", result)
	}

	stream, err := env.client.Get(strings.Replace(childURL, "/relationships/", "/relationship-streams/", 1))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	reader := bufio.NewReader(stream.Body)
	if _, snapshot := readRelationshipEvent(t, reader); len(snapshot.Parents) != 1 {
		t.Fatalf("initial field-reference stream = %#v", snapshot)
	}
	unstructured.SetNestedField(child.Object, map[string]any{"apiVersion": "policies.example.test/v1beta1", "kind": "ClusterPolicy", "name": "guardrails", "uid": "stale-uid"}, "spec", "policyRef")
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Update(t.Context(), child, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, snapshot := readRelationshipEvent(t, reader); len(snapshot.Parents) != 0 {
		t.Fatalf("stale UID rebound = %#v", snapshot)
	}
	unstructured.SetNestedField(child.Object, "not-an-object", "spec", "policyRef")
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Update(t.Context(), child, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	_, snapshot := readRelationshipEvent(t, reader)
	if snapshot.Upward.Complete || len(snapshot.Upward.Problems) != 1 || strings.Contains(snapshot.Upward.Problems[0].Reason, "guardrails") {
		t.Fatalf("malformed diagnostic = %#v", snapshot)
	}
	unstructured.RemoveNestedField(child.Object, "spec", "policyRef")
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Update(t.Context(), child, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	_, snapshot = readRelationshipEvent(t, reader)
	if !snapshot.Upward.Complete || len(snapshot.Upward.Problems) != 0 || len(snapshot.Parents) != 0 {
		t.Fatalf("missing reference was not authoritative absence = %#v", snapshot)
	}
}

func TestPublicHTTPFieldReferenceRejectsIdentityMismatchesAndUnauthorizedParent(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	parent := clusterPolicy("policy-uid", "guardrails", "platform", true)
	hidden := clusterPolicy("hidden-uid", "hidden", "payments", true)
	child := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "mismatch-child", "name": "mismatch-child", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}}}
	for _, item := range []*unstructured.Unstructured{parent, hidden} {
		if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	setRef := func(value map[string]any) {
		t.Helper()
		if err := unstructured.SetNestedField(child.Object, value, "spec", "policyRef"); err != nil {
			t.Fatal(err)
		}
		if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Update(t.Context(), child, metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	base := map[string]any{"apiVersion": "policies.example.test/v1beta1", "kind": "ClusterPolicy", "name": "guardrails", "uid": "policy-uid"}
	if err := unstructured.SetNestedField(child.Object, base, "spec", "policyRef"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "mismatch-child")
	waitForItemsAt(t, &env, policyPath(), "policy-uid")
	relationshipURL := env.app.URL + "/api/v1/tenants/platform/relationships/_core/v1/configmaps/mismatch-child"
	assertMalformed := func() {
		t.Helper()
		var last resources.Neighborhood
		for range 100 {
			decode(t, get(t, env.client, relationshipURL), &last)
			if len(last.Parents) == 0 && !last.Upward.Complete && len(last.Upward.Problems) == 1 {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("mismatched field reference = %#v", last)
	}
	for name, changes := range map[string]map[string]any{
		"group":   {"apiVersion": "other.example.test/v1beta1"},
		"version": {"apiVersion": "policies.example.test/v1"},
		"kind":    {"kind": "OtherPolicy"},
		"name":    {"name": "wrong"},
		"uid":     {"uid": "wrong"},
	} {
		t.Run(name, func(t *testing.T) {
			value := map[string]any{}
			for key, item := range base {
				value[key] = item
			}
			for key, item := range changes {
				value[key] = item
			}
			setRef(value)
			assertMalformed()
		})
	}
	setRef(map[string]any{"apiVersion": "policies.example.test/v1beta1", "kind": "ClusterPolicy", "name": "hidden", "uid": "hidden-uid"})
	var isolated resources.Neighborhood
	for range 100 {
		decode(t, get(t, env.client, relationshipURL), &isolated)
		if len(isolated.Parents) == 0 && isolated.Upward.Complete && len(isolated.Upward.Problems) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("unauthorized parent leaked through completeness: %#v", isolated)
}

func TestPublicHTTPFieldReferenceRejectsNamespacedParentForClusterChild(t *testing.T) {
	env := newEnvironmentConfigured(t, time.Hour, func(cfg *config.Config) {
		cfg.Observation.Relationships[2] = config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "policies.example.test", Kind: "ClusterPolicy"}, Resolver: "fieldReference", FieldReference: &config.FieldReferenceConfig{Path: "$.spec.configRef"}}
	})
	env.login(t, "allowed")
	parent := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "namespaced-parent", "name": "settings", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}}}
	child := clusterPolicy("cluster-child", "cluster-child", "platform", true)
	if err := unstructured.SetNestedField(child.Object, map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "name": "settings", "uid": "namespaced-parent", "namespace": "team-a"}, "spec", "configRef"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), parent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, policyPath(), "cluster-child")
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "namespaced-parent")
	response := get(t, env.client, env.app.URL+"/api/v1/tenants/platform/relationships/policies.example.test/v1beta1/clusterpolicies/cluster-child")
	var result resources.Neighborhood
	decode(t, response, &result)
	if response.StatusCode != http.StatusOK || len(result.Parents) != 0 || result.Upward.Complete || len(result.Upward.Problems) != 1 {
		t.Fatalf("cluster child accepted namespaced parent = %#v", result)
	}
}

func TestPublicHTTPFieldReferenceDoesNotRebindRecreatedParent(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	parent := clusterPolicy("old-policy", "guardrails", "platform", true)
	child := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "recreate-child", "name": "recreate-child", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}, "spec": map[string]any{"policyRef": map[string]any{"apiVersion": "policies.example.test/v1beta1", "kind": "ClusterPolicy", "name": "guardrails", "uid": "old-policy"}}}}
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), parent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "recreate-child")
	waitForItemsAt(t, &env, policyPath(), "old-policy")
	relationshipURL := env.app.URL + "/api/v1/tenants/platform/relationships/_core/v1/configmaps/recreate-child"
	if err := env.kube.Resource(env.policyGVR).Delete(t.Context(), "guardrails", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	replacement := clusterPolicy("new-policy", "guardrails", "platform", true)
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), replacement, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, policyPath(), "new-policy")
	var result resources.Neighborhood
	decode(t, get(t, env.client, relationshipURL), &result)
	if len(result.Parents) != 0 || result.Upward.Complete || len(result.Upward.Problems) != 1 {
		t.Fatalf("stale reference rebound after recreation = %#v", result)
	}
}

func TestPublicHTTPFieldReferenceCoreParentEnforcesNamespaceScope(t *testing.T) {
	env := newEnvironmentConfigured(t, time.Hour, func(cfg *config.Config) {
		cfg.Observation.Relationships[0] = config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "widgets.example.test", Kind: "Widget"}, Resolver: "fieldReference", FieldReference: &config.FieldReferenceConfig{Path: "$.spec.configRef"}}
	})
	env.login(t, "allowed")
	parent := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "core-parent", "name": "settings", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}}}
	child := widget("core-child", "core-child", "platform", "Ready", 1)
	child.SetNamespace("team-a")
	if err := unstructured.SetNestedField(child.Object, map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "name": "settings", "uid": "core-parent", "namespace": "team-a"}, "spec", "configRef"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), parent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, widgetPath("platform"), "core-child")
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "core-parent")
	relationshipURL := env.app.URL + "/api/v1/tenants/platform/relationships/widgets.example.test/v1/widgets/core-child"
	var result resources.Neighborhood
	decode(t, get(t, env.client, relationshipURL), &result)
	if len(result.Parents) != 1 || result.Parents[0].UID != "core-parent" {
		t.Fatalf("core parent field reference = %#v", result)
	}
	if err := unstructured.SetNestedField(child.Object, map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "name": "missing", "uid": "missing"}, "spec", "configRef"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), child, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		decode(t, get(t, env.client, relationshipURL), &result)
		if len(result.Parents) == 0 && !result.Upward.Complete {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if result.Upward.Complete || len(result.Upward.Problems) != 1 {
		t.Fatalf("missing namespace was not malformed without a current parent: %#v", result)
	}
	if err := unstructured.SetNestedField(child.Object, map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "name": "settings", "uid": "core-parent", "namespace": "team-b"}, "spec", "configRef"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), child, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		decode(t, get(t, env.client, relationshipURL), &result)
		if len(result.Parents) == 0 && !result.Upward.Complete {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("namespace mismatch remained resolved: %#v", result)
}

func TestPublicHTTPFieldReferenceUnavailableEndpointIsIncomplete(t *testing.T) {
	env := newEnvironmentConfigured(t, time.Hour, func(cfg *config.Config) {
		cfg.Observation.Relationships[2].Parent = config.RelationshipType{Group: "policies.example.test", Kind: "TemporarilyAbsent"}
	})
	env.login(t, "allowed")
	child := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "unavailable-child", "name": "unavailable-child", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}, "spec": map[string]any{"policyRef": map[string]any{"apiVersion": "policies.example.test/v1", "kind": "TemporarilyAbsent", "name": "missing", "uid": "missing"}}}}
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "unavailable-child")
	response := get(t, env.client, env.app.URL+"/api/v1/tenants/platform/relationships/_core/v1/configmaps/unavailable-child")
	var result resources.Neighborhood
	decode(t, response, &result)
	if response.StatusCode != http.StatusOK || result.Upward.Complete || len(result.Upward.Scopes) != 1 || result.Upward.Scopes[0].Kind != "TemporarilyAbsent" {
		t.Fatalf("unavailable endpoint completeness = %#v", result)
	}
	stream, err := env.client.Get(env.app.URL + "/api/v1/tenants/platform/relationship-streams/_core/v1/configmaps/unavailable-child")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	event, snapshot := readRelationshipEvent(t, bufio.NewReader(stream.Body))
	if event != "snapshot" || snapshot.Upward.Complete || len(snapshot.Upward.Scopes) != 1 || snapshot.Upward.Scopes[0].Kind != "TemporarilyAbsent" {
		t.Fatalf("unavailable endpoint stream event %q = %#v", event, snapshot)
	}
}

func TestPublicHTTPOwnershipNeighborhoodIsUIDVerifiedTenantScopedAndLive(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	parent := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"uid": "parent-uid", "name": "settings", "namespace": "team-a", "labels": map[string]any{"tenant": "platform", "inspect": "true"}}}}
	other := parent.DeepCopy()
	other.SetUID("wrong-uid")
	other.SetName("same-name")
	child := widget("child-uid", "child", "platform", "Ready", 1)
	controller := true
	child.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "wrong-name", UID: "parent-uid", Controller: &controller}})
	for _, item := range []*unstructured.Unstructured{parent, other} {
		if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), child, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "child-uid")
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/_core/v1/configmaps", "wrong-uid", "parent-uid")
	path := env.app.URL + "/api/v1/tenants/platform/relationships/widgets.example.test/v1/widgets/child-uid"
	response := get(t, env.client, path)
	var neighborhood resources.Neighborhood
	decode(t, response, &neighborhood)
	if response.StatusCode != http.StatusOK || len(neighborhood.Parents) != 1 || neighborhood.Parents[0].UID != "parent-uid" || !neighborhood.Parents[0].Controller {
		t.Fatalf("neighborhood = %#v", neighborhood)
	}
	parentPath := env.app.URL + "/api/v1/tenants/platform/relationships/_core/v1/configmaps/parent-uid"
	response = get(t, env.client, parentPath)
	decode(t, response, &neighborhood)
	if len(neighborhood.Children) != 1 || neighborhood.Children[0].UID != "child-uid" {
		t.Fatalf("downward neighborhood = %#v", neighborhood)
	}

	parent.SetLabels(map[string]string{"tenant": "payments", "inspect": "true"})
	if _, err := env.kube.Resource(env.configMapGVR).Namespace("team-a").Update(t.Context(), parent, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		response = get(t, env.client, path)
		decode(t, response, &neighborhood)
		if len(neighborhood.Parents) == 0 {
			response = get(t, env.client, parentPath)
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("inaccessible center status = %d: %s", response.StatusCode, body(response))
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("inaccessible parent remained in ownership neighborhood")
}

func TestPublicHTTPLabelJoinOwnershipIsCompositeAuthorizedUnambiguousAndLive(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	parent := clusterPolicy("logical-parent", "logical-parent", "platform", true)
	parent.SetLabels(map[string]string{"tenant": "platform", "inspect": "true", "owner": "api", "scope": "production"})
	hiddenParent := clusterPolicy("hidden-parent", "hidden-parent", "payments", true)
	hiddenParent.SetLabels(map[string]string{"tenant": "payments", "inspect": "true", "owner": "api", "scope": "production"})
	child := widget("logical-child", "logical-child", "platform", "Ready", 1)
	child.SetLabels(map[string]string{"tenant": "platform", "inspect": "true", "policy": "api", "policy-scope": "production"})
	secondChild := widget("second-child", "second-child", "platform", "Ready", 1)
	secondChild.SetLabels(map[string]string{"tenant": "platform", "inspect": "true", "policy": "api", "policy-scope": "production"})
	missingLabel := widget("missing-label", "missing-label", "platform", "Ready", 1)
	missingLabel.SetLabels(map[string]string{"tenant": "platform", "inspect": "true", "policy": "api"})
	for _, item := range []*unstructured.Unstructured{parent, hiddenParent} {
		if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []*unstructured.Unstructured{child, secondChild, missingLabel} {
		if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	waitForItemsAt(t, &env, widgetPath("platform"), "logical-child", "missing-label", "second-child")
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/policies.example.test/v1beta1/clusterpolicies", "logical-parent")

	childPath := env.app.URL + "/api/v1/tenants/platform/relationships/widgets.example.test/v1/widgets/logical-child"
	parentPath := env.app.URL + "/api/v1/tenants/platform/relationships/policies.example.test/v1beta1/clusterpolicies/logical-parent"
	assertNeighborhood := func(path string) resources.Neighborhood {
		t.Helper()
		response := get(t, env.client, path)
		var neighborhood resources.Neighborhood
		decode(t, response, &neighborhood)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("neighborhood status %d: %#v", response.StatusCode, neighborhood)
		}
		return neighborhood
	}
	upward := assertNeighborhood(childPath)
	if len(upward.Parents) != 1 || upward.Parents[0].UID != "logical-parent" || upward.Parents[0].Controller || !upward.Upward.Complete {
		t.Fatalf("label-joined parents = %#v", upward)
	}
	downward := assertNeighborhood(parentPath)
	if len(downward.Children) != 2 || downward.Children[0].UID != "logical-child" || downward.Children[1].UID != "second-child" {
		t.Fatalf("label-joined children = %#v", downward)
	}
	stream, err := env.client.Get(strings.Replace(childPath, "/relationships/", "/relationship-streams/", 1))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	reader := bufio.NewReader(stream.Body)
	if event, snapshot := readRelationshipEvent(t, reader); event != "snapshot" || len(snapshot.Parents) != 1 {
		t.Fatalf("initial relationship event %q = %#v", event, snapshot)
	}

	ambiguous := clusterPolicy("ambiguous-parent", "ambiguous-parent", "platform", true)
	ambiguous.SetLabels(map[string]string{"tenant": "platform", "inspect": "true", "owner": "api", "scope": "production"})
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), ambiguous, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, "/api/v1/tenants/platform/resources/policies.example.test/v1beta1/clusterpolicies", "ambiguous-parent", "logical-parent")
	event, upward := readRelationshipEvent(t, reader)
	if event != "snapshot" {
		t.Fatalf("ambiguity event = %q", event)
	}
	if len(upward.Parents) != 0 || upward.Upward.Complete || len(upward.Upward.Problems) != 1 || strings.Contains(upward.Upward.Problems[0].Reason, "logical-parent") {
		t.Fatalf("ambiguous upward neighborhood = %#v", upward)
	}
	downward = assertNeighborhood(parentPath)
	if len(downward.Children) != 0 || downward.Downward.Complete || len(downward.Downward.Problems) != 1 {
		t.Fatalf("ambiguous downward neighborhood = %#v", downward)
	}

	if err := env.kube.Resource(env.policyGVR).Delete(t.Context(), "logical-parent", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	event, upward = readRelationshipEvent(t, reader)
	if event != "snapshot" || len(upward.Parents) != 1 || upward.Parents[0].UID != "ambiguous-parent" {
		t.Fatalf("logical ownership did not rebind in %q event: %#v", event, upward)
	}
}

func readRelationshipEvent(t *testing.T, reader *bufio.Reader) (string, resources.Neighborhood) {
	t.Helper()
	eventLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	dataLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	_, _ = reader.ReadString('\n')
	var neighborhood resources.Neighborhood
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")), &neighborhood); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(strings.TrimPrefix(eventLine, "event: ")), neighborhood
}

func (e *environment) login(t *testing.T, user string) {
	t.Helper()
	idpURL, _ := http.NewRequest("GET", e.idp.URL, http.NoBody)
	e.client.Jar.SetCookies(idpURL.URL, []*http.Cookie{{Name: "testidp_user", Value: user, Path: "/"}})
	response, err := e.client.Get(e.app.URL + "/auth/login")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.Request.URL.Path != "/" || response.Request.URL.Query().Get("authError") != "" {
		t.Fatalf("login ended at %s with %d (auth error %s)", response.Request.URL.Path, response.StatusCode, response.Request.URL.Query().Get("authError"))
	}
}

func TestPublicHTTPAuthorizationAndTenantSwitching(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	response := get(t, env.client, env.app.URL+"/api/session")
	if response.StatusCode != 200 {
		t.Fatalf("session status %d: %s", response.StatusCode, body(response))
	}
	var payload struct {
		Tenants []struct {
			ID string `json:"id"`
		} `json:"tenants"`
	}
	decode(t, response, &payload)
	if len(payload.Tenants) != 2 {
		t.Fatalf("authorized tenants = %#v", payload.Tenants)
	}
	for _, tenant := range []string{"platform", "payments"} {
		response = get(t, env.client, env.app.URL+"/api/tenants/"+tenant+"/context")
		if response.StatusCode != 200 {
			t.Fatalf("tenant %s status %d", tenant, response.StatusCode)
		}
	}
	response = get(t, env.client, env.app.URL+"/api/tenants/arbitrary/context")
	if response.StatusCode != 403 {
		t.Fatalf("arbitrary tenant status %d", response.StatusCode)
	}
}

func TestPublicHTTPResourceInspectionTracksCurrentUIDAndTenant(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")

	platform := widget("old-uid", "shared-name", "platform", "Pending", 1)
	payment := widget("payment-uid", "invoice", "payments", "Ready", 1)
	hidden := widget("hidden-uid", "not-selected", "platform", "Ready", 1)
	hidden.SetLabels(map[string]string{"tenant": "platform"})
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), platform, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), payment, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), hidden, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "old-uid")

	response := get(t, env.client, env.app.URL+widgetPath("platform")+"/payment-uid")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant detail status %d: %s", response.StatusCode, body(response))
	}
	platform.Object["status"] = map[string]any{"phase": "Ready", "arbitrary": map[string]any{"answer": int64(42)}}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), platform, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForYAML(t, &env, "platform", "old-uid", "answer: 42")

	if err := env.kube.Resource(env.gvr).Namespace("team-a").Delete(t.Context(), "shared-name", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	recreated := widget("new-uid", "shared-name", "platform", "Recreated", 2)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), recreated, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "new-uid")
	response = get(t, env.client, env.app.URL+widgetPath("platform")+"/old-uid")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted UID status %d", response.StatusCode)
	}
}

func TestPublicHTTPPresentsReadyHealthConditionsAndCompleteYAML(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	tests := []struct {
		uid, status, health string
		generation          int64
		observedGeneration  any
		stale               bool
		includeReady        bool
	}{
		{uid: "healthy", status: "True", health: "Healthy", generation: 3, observedGeneration: int64(3), includeReady: true},
		{uid: "unhealthy", status: "False", health: "Unhealthy", generation: 3, observedGeneration: int64(3), includeReady: true},
		{uid: "unknown", status: "Unknown", health: "Unknown", generation: 3, observedGeneration: int64(3), includeReady: true},
		{uid: "stale", status: "True", health: "Healthy", generation: 4, observedGeneration: int64(3), stale: true, includeReady: true},
		{uid: "missing-generation", status: "False", health: "Unhealthy", generation: 3, includeReady: true},
		{uid: "non-ready", health: "Unknown", generation: 3, observedGeneration: int64(3)},
		{uid: "malformed-status", health: "Unknown", generation: 3},
	}
	for _, test := range tests {
		item := widget(test.uid, test.uid, "platform", "ignored", test.generation)
		condition := map[string]any{"type": "Progressing", "status": "True", "reason": "Reconciling", "message": "Work remains visible"}
		if test.includeReady {
			condition = map[string]any{"type": "Ready", "status": test.status, "reason": "ControllerReport", "message": "Controller evidence retained"}
		}
		if test.observedGeneration != nil {
			condition["observedGeneration"] = test.observedGeneration
		}
		if test.uid == "malformed-status" {
			item.Object["status"] = []any{"controller-specific", map[string]any{"conditions": "not-a-list"}}
		} else {
			item.Object["status"] = map[string]any{"conditions": []any{condition}}
		}
		if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	waitForItemsAt(t, &env, widgetPath("platform"), "healthy", "malformed-status", "missing-generation", "non-ready", "stale", "unhealthy", "unknown")

	response := get(t, env.client, env.app.URL+widgetPath("platform"))
	var list struct {
		Items []resources.Summary `json:"items"`
	}
	decode(t, response, &list)
	byUID := map[string]resources.Summary{}
	for _, item := range list.Items {
		byUID[item.UID] = item
	}
	for _, test := range tests {
		actual := byUID[test.uid]
		if actual.Health.Status != test.health || actual.Health.Stale != test.stale {
			t.Errorf("%s health = %#v, want status %s stale %v", test.uid, actual.Health, test.health, test.stale)
		}
	}

	response = get(t, env.client, env.app.URL+widgetPath("platform")+"/stale")
	var detail resources.Detail
	decode(t, response, &detail)
	if len(detail.Conditions) != 1 || detail.Conditions[0].Reason != "ControllerReport" || detail.Conditions[0].Message != "Controller evidence retained" || !detail.Conditions[0].Stale {
		t.Fatalf("stale detail conditions = %#v", detail.Conditions)
	}
	if detail.Conditions[0].ObservedGeneration == nil || *detail.Conditions[0].ObservedGeneration != 3 || !strings.Contains(detail.YAML, "size: large") || !strings.Contains(detail.YAML, "Controller evidence retained") {
		t.Fatalf("detail did not preserve generation and complete YAML: %#v\n%s", detail.Conditions[0], detail.YAML)
	}

	response = get(t, env.client, env.app.URL+widgetPath("platform")+"/missing-generation")
	detail = resources.Detail{}
	decode(t, response, &detail)
	if detail.Health.Stale || detail.Conditions[0].ObservedGeneration != nil {
		t.Fatalf("missing observed generation asserted freshness: %#v", detail)
	}
	response = get(t, env.client, env.app.URL+widgetPath("platform")+"/malformed-status")
	detail = resources.Detail{}
	decode(t, response, &detail)
	if detail.Health.Status != "Unknown" || len(detail.Conditions) != 0 || !strings.Contains(detail.YAML, "controller-specific") {
		t.Fatalf("malformed status was not safely inspectable: %#v\n%s", detail, detail.YAML)
	}
}

func TestResourceStreamUpdatesReadyHealthAcrossCurrentStaleAndUnhealthyStates(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	item := widget("live-health", "live-health", "platform", "ignored", 1)
	setReadyCondition(item, "True", 1)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "live-health")
	stream, snapshots := openStream(t, env.client, env.app.URL+widgetStreamPath("platform"))
	defer stream.Body.Close()
	assertSnapshotHealth(t, snapshots, "Healthy", false)
	item.SetGeneration(2)
	setReadyCondition(item, "True", 1)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), item, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	assertSnapshotHealth(t, snapshots, "Healthy", true)
	setReadyCondition(item, "False", 2)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), item, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	assertSnapshotHealth(t, snapshots, "Unhealthy", false)
}

func TestDetailStreamIsUIDScopedLiveAndTerminalOnRevocation(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	item := widget("preview-uid", "preview", "platform", "Ready", 1)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "preview-uid")
	path := env.app.URL + "/api/v1/tenants/platform/detail-streams/widgets.example.test/v1/widgets/preview-uid"
	response, err := env.client.Get(path)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("detail stream response %d %q", response.StatusCode, response.Header.Get("Content-Type"))
	}
	reader := bufio.NewReader(response.Body)
	event, detail := readDetailEvent(t, reader)
	if event != "snapshot" || detail.UID != "preview-uid" || !strings.Contains(detail.YAML, "generation: 1") {
		t.Fatalf("initial detail event %q = %#v", event, detail)
	}
	item.SetGeneration(2)
	if _, updateErr := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), item, metav1.UpdateOptions{}); updateErr != nil {
		t.Fatal(updateErr)
	}
	event, detail = readDetailEvent(t, reader)
	if event != "snapshot" || detail.UID != "preview-uid" || !strings.Contains(detail.YAML, "generation: 2") {
		t.Fatalf("updated detail event %q = %#v", event, detail)
	}
	item.SetLabels(map[string]string{"tenant": "payments", "inspect": "true"})
	if _, updateErr := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), item, metav1.UpdateOptions{}); updateErr != nil {
		t.Fatal(updateErr)
	}
	line, err := reader.ReadString('\n')
	if err != nil || line != "event: revoked\n" {
		t.Fatalf("revocation event = %q, %v", line, err)
	}

	if deleteErr := env.kube.Resource(env.gvr).Namespace("team-a").Delete(t.Context(), "preview", metav1.DeleteOptions{}); deleteErr != nil {
		t.Fatal(deleteErr)
	}
	recreated := widget("replacement-uid", "preview", "platform", "Ready", 3)
	if _, createErr := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), recreated, metav1.CreateOptions{}); createErr != nil {
		t.Fatal(createErr)
	}
	waitForItems(t, &env, "platform", "replacement-uid")
	oldResponse := get(t, env.client, path)
	if oldResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("old UID detail stream status = %d: %s", oldResponse.StatusCode, body(oldResponse))
	}
}

func readDetailEvent(t *testing.T, reader *bufio.Reader) (string, resources.Detail) {
	t.Helper()
	eventLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	dataLine, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	_, _ = reader.ReadString('\n')
	var detail resources.Detail
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(dataLine), "data: ")), &detail); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(strings.TrimPrefix(eventLine, "event: ")), detail
}

func TestResourceStreamDeliversCommittedTenantScopedReplacementSnapshots(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	shared := widget("shared-uid", "shared", "platform", "Pending", 1)
	shared.SetLabels(map[string]string{"tenant": "shared", "inspect": "true"})
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), shared, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "shared-uid")

	stream, snapshots := openStream(t, env.client, env.app.URL+widgetStreamPath("platform"))
	defer stream.Body.Close()
	assertSnapshotUIDs(t, snapshots, "shared-uid")
	paymentStream, paymentSnapshots := openStream(t, env.client, env.app.URL+widgetStreamPath("payments"))
	defer paymentStream.Body.Close()
	assertSnapshotUIDs(t, paymentSnapshots, "shared-uid")

	shared.Object["status"] = map[string]any{"phase": "Ready"}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), shared, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	assertSnapshotUIDs(t, snapshots, "shared-uid")

	shared.SetLabels(map[string]string{"tenant": "payments", "inspect": "true"})
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), shared, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	assertSnapshotUIDs(t, snapshots)
	assertSnapshotUIDs(t, paymentSnapshots, "shared-uid")
	waitForItems(t, &env, "payments", "shared-uid")
}

func TestResourceStreamRejectsUnauthorizedTenant(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "payments")
	response := get(t, env.client, env.app.URL+widgetStreamPath("platform"))
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized stream status %d: %s", response.StatusCode, body(response))
	}
}

func TestResourceStreamsAreTypeScopedReconnectAndPreserveUIDIdentity(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	waitForReady(t, &env, widgetPath("platform"))
	old := widget("old-uid", "replace-me", "platform", "Pending", 1)
	policy := clusterPolicy("policy-uid", "guardrails", "platform", true)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), old, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), policy, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "old-uid")
	widgetResponse, widgetSnapshots := openStream(t, env.client, env.app.URL+widgetStreamPath("platform"))
	assertSnapshotUIDs(t, widgetSnapshots, "old-uid")
	widgetResponse.Body.Close()

	if err := env.kube.Resource(env.gvr).Namespace("team-a").Delete(t.Context(), "replace-me", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	recreated := widget("new-uid", "replace-me", "platform", "Ready", 2)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), recreated, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "new-uid")
	reconnected, replacement := openStream(t, env.client, env.app.URL+widgetStreamPath("platform"))
	defer reconnected.Body.Close()
	assertSnapshotUIDs(t, replacement, "new-uid")
	policyResponse, policySnapshots := openStream(t, env.client, env.app.URL+policyStreamPath("platform"))
	defer policyResponse.Body.Close()
	assertSnapshotUIDs(t, policySnapshots, "policy-uid")
}

func TestLogoutAndExpiryCancelResourceStreams(t *testing.T) {
	for name, ttl := range map[string]time.Duration{"logout": time.Hour, "expiry": 2 * time.Second} {
		t.Run(name, func(t *testing.T) {
			env := newEnvironment(t, ttl)
			env.login(t, "allowed")
			waitForReady(t, &env, widgetPath("platform"))
			stream, snapshots := openStream(t, env.client, env.app.URL+widgetStreamPath("platform"))
			assertSnapshotUIDs(t, snapshots)
			if name == "logout" {
				request, _ := http.NewRequest(http.MethodPost, env.app.URL+"/api/logout", http.NoBody)
				request.Header.Set("Origin", env.app.URL)
				response, err := env.client.Do(request)
				if err != nil || response.StatusCode != http.StatusNoContent {
					t.Fatalf("logout response: %v, %v", response, err)
				}
				response.Body.Close()
			}
			closed := make(chan error, 1)
			go func() {
				_, err := bufio.NewReader(stream.Body).ReadByte()
				closed <- err
			}()
			select {
			case err := <-closed:
				if err == nil {
					t.Fatal("stream remained readable after session ended")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stream was not canceled with session")
			}
			stream.Body.Close()
		})
	}
}

func TestResourceReadsAreGatedUntilSynchronization(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	if err := env.store.SetSynchronized(t.Context(), "widgets.example.test/v1/widgets", false); err != nil {
		t.Fatal(err)
	}
	response := get(t, env.client, env.app.URL+widgetPath("platform"))
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unsynchronized list status %d: %s", response.StatusCode, body(response))
	}
}

func TestObservationLossWithholdsNewReadsAndInterruptsExistingStream(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	item := widget("stale-membership", "stale-membership", "platform", "Ready", 1)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItems(t, &env, "platform", "stale-membership")
	stream, err := env.client.Get(env.app.URL + widgetStreamPath("platform"))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	reader := bufio.NewReader(stream.Body)
	if event, _ := reader.ReadString('\n'); event != "event: snapshot\n" {
		t.Fatalf("initial stream event = %q", event)
	}
	if data, _ := reader.ReadString('\n'); !strings.Contains(data, "stale-membership") {
		t.Fatalf("initial stream payload = %q", data)
	}
	_, _ = reader.ReadString('\n')

	if !env.observation.Interrupt("widgets.example.test/v1/widgets", resources.ObservationUnavailable, "watch denied") {
		t.Fatal("observer did not accept controlled interruption")
	}
	response := get(t, env.client, env.app.URL+widgetPath("platform"))
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("stale list status = %d: %s", response.StatusCode, body(response))
	}
	var problem struct {
		Observation resources.ObservationStatus `json:"observation"`
	}
	decode(t, response, &problem)
	if problem.Observation.State != resources.ObservationUnavailable || problem.Observation.Reason != "watch denied" {
		t.Fatalf("observation problem = %#v", problem)
	}
	line, err := reader.ReadString('\n')
	if err != nil || line != "event: observation\n" {
		t.Fatalf("stream status line = %q, %v", line, err)
	}
	line, err = reader.ReadString('\n')
	if err != nil || !strings.Contains(line, `"state":"unavailable"`) || !strings.Contains(line, "watch denied") {
		t.Fatalf("stream status payload = %q, %v", line, err)
	}
	current, err := env.kube.Resource(env.gvr).Namespace("team-a").Get(t.Context(), "stale-membership", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	current.SetLabels(map[string]string{"tenant": "payments", "inspect": "true"})
	if _, err = env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), current, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, widgetPath("platform"))
	waitForItems(t, &env, "payments", "stale-membership")
}

func TestPublicHTTPConfiguredDiscoveryScopeVersionAvailabilityAndIsolation(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")

	response := get(t, env.client, env.app.URL+"/api/v1/configuration")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("configuration status %d: %s", response.StatusCode, body(response))
	}
	var configuration struct {
		APIVersion    string           `json:"apiVersion"`
		ResourceTypes []resources.Type `json:"resourceTypes"`
	}
	decode(t, response, &configuration)
	if configuration.APIVersion != "yak8sui.io/v1" || len(configuration.ResourceTypes) != 4 {
		t.Fatalf("configuration = %#v", configuration)
	}
	if configuration.ResourceTypes[0].Resource == "internalwidgets" || configuration.ResourceTypes[1].Resource == "internalwidgets" {
		t.Fatal("excluded type exposed")
	}

	platform := clusterPolicy("policy-platform", "platform-policy", "platform", true)
	payment := clusterPolicy("policy-payment", "payment-policy", "payments", true)
	unselected := clusterPolicy("policy-unselected", "unselected-policy", "platform", false)
	for _, item := range []*unstructured.Unstructured{platform, payment, unselected} {
		if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	waitForItemsAt(t, &env, policyPath(), "policy-platform")
	response = get(t, env.client, env.app.URL+policyPath()+"/policy-payment")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant cluster detail status %d: %s", response.StatusCode, body(response))
	}
	response = get(t, env.client, env.app.URL+"/api/v1/tenants/platform/resources/policies.example.test/v1/temporarilyabsent")
	if response.StatusCode != http.StatusServiceUnavailable || !strings.Contains(body(response), "not currently served") {
		t.Fatalf("unavailable type response %d", response.StatusCode)
	}
}

func TestPublicHTTPFiltersAllScopedResourcesBeforeCountingAndPagination(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	for index := range 205 {
		name := fmt.Sprintf("ordinary-%03d", index)
		if index%2 == 0 {
			name = fmt.Sprintf("Alpha-target-%03d", index)
		}
		item := widget(fmt.Sprintf("bulk-%03d", index), name, "platform", "Ready", 1)
		item.SetNamespace(map[bool]string{true: "Team-East", false: "team-west"}[index%4 == 0])
		if _, err := env.kube.Resource(env.gvr).Namespace(item.GetNamespace()).Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	unicodeItem := widget("unicode-uid", "Ångström", "platform", "Ready", 1)
	unicodeItem.SetNamespace("Tést-Lab")
	if _, err := env.kube.Resource(env.gvr).Namespace(unicodeItem.GetNamespace()).Create(t.Context(), unicodeItem, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForTotal(t, &env, widgetPath("platform"), 206)
	response := get(t, env.client, env.app.URL+widgetPath("platform")+"?name=åö&namespace=éa")
	unicodePage := decodePage(t, response)
	if unicodePage.Total != 1 || unicodePage.Items[0].UID != "unicode-uid" {
		t.Fatalf("Unicode fuzzy page = %#v", unicodePage)
	}

	response = get(t, env.client, env.app.URL+widgetPath("platform")+"?name=At1&namespace=meas&nameMode=fuzzy&namespaceMode=fuzzy&page=1")
	page := decodePage(t, response)
	if page.Total != 27 || len(page.Items) != 27 || page.PageCount != 1 {
		t.Fatalf("combined fuzzy page = total %d count %d pages %d", page.Total, len(page.Items), page.PageCount)
	}
	for _, item := range page.Items {
		if item.Namespace != "Team-East" {
			t.Fatalf("namespace filter returned %#v", item)
		}
	}

	response = get(t, env.client, env.app.URL+widgetPath("platform")+"?name=%5EA.*-(19%5B0-9%5D%7C20%5B0-4%5D)%24&nameMode=regex&page=1")
	page = decodePage(t, response)
	if page.Total != 8 {
		t.Fatalf("Go regex total = %d, want 8", page.Total)
	}
	response = get(t, env.client, env.app.URL+widgetPath("platform")+"?name=(%3Fi)%5Ealpha&nameMode=regex&page=2")
	page = decodePage(t, response)
	if page.Total != 103 || len(page.Items) != 3 || page.PageCount != 2 || page.Page != 2 {
		t.Fatalf("second filtered page = %#v", page)
	}
	if page.Items[0].Name != "Alpha-target-194" {
		t.Fatalf("stable second page begins with %q", page.Items[0].Name)
	}

	response = get(t, env.client, env.app.URL+widgetPath("platform")+"?name=%28&nameMode=regex")
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid regex status = %d", response.StatusCode)
	}
	var problem struct {
		Fields map[string]string `json:"fields"`
	}
	decode(t, response, &problem)
	if !strings.Contains(problem.Fields["name"], "invalid Go regular expression") {
		t.Fatalf("invalid regex response = %#v", problem)
	}
	response = get(t, env.client, env.app.URL+widgetPath("platform")+"?page=4")
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("out-of-range page status = %d", response.StatusCode)
	}
	response = get(t, env.client, env.app.URL+widgetPath("platform")+"?name=does-not-exist")
	page = decodePage(t, response)
	if page.Total != 0 || len(page.Items) != 0 || page.Page != 1 || page.PageCount != 0 {
		t.Fatalf("empty filtered page = %#v", page)
	}
	response = get(t, env.client, env.app.URL+widgetPath("platform")+"?nameMode=glob")
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("unsupported mode status = %d", response.StatusCode)
	}
}

func TestPublicHTTPJSONPathFilteringUsesTypedAnyMatchBeforePagination(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	for index := range 125 {
		item := widget(fmt.Sprintf("json-%03d", index), fmt.Sprintf("json-target-%03d", index), "platform", "Ready", 1)
		item.Object["spec"] = map[string]any{
			"replicas": int64(3), "enabled": true, "nullable": nil,
			"quoted.field": "safe' OR 1=1 --",
		}
		item.Object["status"] = map[string]any{"conditions": []any{
			map[string]any{"type": "Progressing"}, map[string]any{"type": "Ready"}, map[string]any{"type": "Ready"},
		}}
		if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	waitForTotal(t, &env, widgetPath("platform"), 125)

	query := func(path, comparison, value string, extras url.Values) streamSnapshot {
		t.Helper()
		values := url.Values{"jsonPath": {path}, "jsonComparison": {comparison}}
		if comparison == "equals" {
			values.Set("jsonValue", value)
		}
		for key, entries := range extras {
			values[key] = entries
		}
		return decodePage(t, get(t, env.client, env.app.URL+widgetPath("platform")+"?"+values.Encode()))
	}

	page := query("$.spec.nullable", "exists", "", nil)
	if page.Total != 125 || len(page.Items) != 100 || page.PageCount != 2 {
		t.Fatalf("present-null existence page = %#v", page)
	}
	page = query("$.spec.missing", "exists", "", nil)
	if page.Total != 0 {
		t.Fatalf("missing existence total = %d", page.Total)
	}
	for _, test := range []struct {
		path, value string
	}{
		{"$.spec.replicas", "3"},
		{"$.spec.enabled", "true"},
		{"$.spec.nullable", "null"},
		{"$.status.conditions[*].type", `"Ready"`},
		{"$['spec']['quoted.field']", `"safe' OR 1=1 --"`},
	} {
		page = query(test.path, "equals", test.value, nil)
		if page.Total != 125 {
			t.Fatalf("%s equals %s total = %d", test.path, test.value, page.Total)
		}
	}
	page = query("$.spec.enabled", "equals", `"true"`, nil)
	if page.Total != 0 {
		t.Fatalf("boolean matched string: %d", page.Total)
	}
	page = query("$.spec.replicas", "equals", "3.0", url.Values{"page": {"2"}})
	if page.Total != 125 || len(page.Items) != 25 || page.Page != 2 {
		t.Fatalf("numeric fallback second page = %#v", page)
	}
	page = query("$.status.conditions[1].type", "equals", `"Ready"`, url.Values{"name": {"jt12"}, "namespace": {"ta"}})
	if page.Total != 8 {
		t.Fatalf("combined JSON/name/namespace total = %d", page.Total)
	}
	page = query("$.spec.replicas", "equals", "3", nil)
	for _, item := range page.Items {
		if item.UID == "widget-platform" {
			t.Fatal("other typed value entered JSON result")
		}
	}
	page = query("$.spec.replicas", "equals", "3", url.Values{})
	if page.Total != 125 {
		t.Fatalf("tenant-scoped JSON total = %d", page.Total)
	}
}

func TestPublicHTTPJSONPathValidationIsFieldSpecific(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	waitForReady(t, &env, widgetPath("platform"))
	for _, test := range []struct {
		query, field string
	}{
		{"jsonPath=%24..status", "jsonPath"},
		{"jsonPath=%24.%2A", "jsonPath"},
		{"jsonPath=%24.status.conditions%5B%3F%28%40.type%3D%3D%27Ready%27%29%5D", "jsonPath"},
		{"jsonPath=%24.spec.replicas&jsonComparison=equals&jsonValue=%7B%7D", "jsonValue"},
		{"jsonPath=%24.spec.replicas&jsonComparison=equals&jsonValue=not-json", "jsonValue"},
		{"jsonPath=%24.spec.replicas&jsonComparison=contains", "jsonComparison"},
		{"jsonComparison=equals&jsonValue=null", "jsonPath"},
	} {
		response := get(t, env.client, env.app.URL+widgetPath("platform")+"?"+test.query)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s status = %d", test.query, response.StatusCode)
		}
		var problem struct {
			Fields map[string]string `json:"fields"`
		}
		decode(t, response, &problem)
		if problem.Fields[test.field] == "" {
			t.Fatalf("%s response fields = %#v", test.query, problem.Fields)
		}
	}
}

func TestClusterScopedResourcesSupportJSONPathAndIgnoreNamespace(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	policy := clusterPolicy("json-policy", "json-policy", "platform", true)
	policy.Object["spec"] = map[string]any{"enforcement": "required"}
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), policy, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, policyPath(), "json-policy")
	response := get(t, env.client, env.app.URL+policyPath()+"?namespace=%28&namespaceMode=unsupported&jsonPath=%24.spec.enforcement&jsonComparison=equals&jsonValue=%22required%22")
	page := decodePage(t, response)
	if page.Total != 1 || page.Items[0].UID != "json-policy" {
		t.Fatalf("cluster JSON query = %#v", page)
	}
}

func TestJSONPathFilteredStreamTracksFieldEntryAndExit(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	item := widget("live-json", "live-json", "platform", "Ready", 1)
	item.Object["spec"] = map[string]any{"replicas": int64(2)}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, widgetPath("platform"), "live-json")
	stream, snapshots := openStream(t, env.client, env.app.URL+widgetStreamPath("platform")+"?jsonPath=%24.spec.replicas&jsonComparison=equals&jsonValue=3")
	defer stream.Body.Close()
	if initial := receiveSnapshot(t, snapshots); initial.Total != 0 {
		t.Fatalf("initial JSON stream total = %d", initial.Total)
	}
	item.Object["spec"] = map[string]any{"replicas": int64(3)}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), item, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if entered := receiveSnapshot(t, snapshots); entered.Total != 1 || entered.Items[0].UID != "live-json" {
		t.Fatalf("entered JSON stream = %#v", entered)
	}
	item.Object["spec"] = map[string]any{"replicas": int64(4)}
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), item, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if exited := receiveSnapshot(t, snapshots); exited.Total != 0 {
		t.Fatalf("exited JSON stream = %#v", exited)
	}
}

func TestFilteredStreamRefreshesPageAndCountWithoutRevokingOffPageDetail(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	for index := range 101 {
		item := widget(fmt.Sprintf("match-%03d", index), fmt.Sprintf("match-%03d", index), "platform", "Ready", 1)
		if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), item, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	waitForTotal(t, &env, widgetPath("platform")+"?name=match", 101)
	stream, snapshots := openStream(t, env.client, env.app.URL+widgetStreamPath("platform")+"?name=match&page=1")
	defer stream.Body.Close()
	initial := receiveSnapshot(t, snapshots)
	if initial.Total != 101 || len(initial.Items) != 100 || initial.PageCount != 2 {
		t.Fatalf("initial filtered stream = %#v", initial)
	}

	entering := widget("match-new", "match-050a", "platform", "Ready", 1)
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Create(t.Context(), entering, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	updated := receiveSnapshot(t, snapshots)
	if updated.Total != 102 || len(updated.Items) != 100 {
		t.Fatalf("inserted filtered stream = %#v", updated)
	}
	response := get(t, env.client, env.app.URL+widgetPath("platform")+"/match-100")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("off-page authorized detail status = %d", response.StatusCode)
	}
	response.Body.Close()

	entering.SetLabels(map[string]string{"tenant": "payments", "inspect": "true"})
	if _, err := env.kube.Resource(env.gvr).Namespace("team-a").Update(t.Context(), entering, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	updated = receiveSnapshot(t, snapshots)
	if updated.Total != 101 {
		t.Fatalf("membership removal total = %d", updated.Total)
	}
	response = get(t, env.client, env.app.URL+widgetPath("platform")+"/match-new")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("lost-membership detail status = %d", response.StatusCode)
	}
}

func TestClusterScopedQueryIgnoresNamespacePredicateAndMode(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "allowed")
	policy := clusterPolicy("policy-uid", "GuardRail", "platform", true)
	if _, err := env.kube.Resource(env.policyGVR).Create(t.Context(), policy, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitForItemsAt(t, &env, policyPath(), "policy-uid")
	response := get(t, env.client, env.app.URL+policyPath()+"?name=gr&namespace=%28&namespaceMode=unsupported")
	page := decodePage(t, response)
	if page.Total != 1 || page.Items[0].UID != "policy-uid" {
		t.Fatalf("cluster query = %#v", page)
	}
}

func widget(uid, name, tenant, phase string, generation int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "widgets.example.test/v1", "kind": "Widget",
		"metadata": map[string]any{"uid": uid, "name": name, "namespace": "team-a", "labels": map[string]any{"tenant": tenant, "inspect": "true"}, "generation": generation},
		"spec":     map[string]any{"size": "large"}, "status": map[string]any{"phase": phase},
	}}
}

func clusterPolicy(uid, name, tenant string, selected bool) *unstructured.Unstructured {
	labels := map[string]any{"tenant": tenant}
	if selected {
		labels["inspect"] = "true"
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "policies.example.test/v1beta1", "kind": "ClusterPolicy",
		"metadata": map[string]any{"uid": uid, "name": name, "labels": labels},
	}}
}

func setReadyCondition(item *unstructured.Unstructured, status string, observedGeneration int64) {
	item.Object["status"] = map[string]any{"conditions": []any{map[string]any{
		"type": "Ready", "status": status, "reason": "TestReport", "message": "Test evidence", "observedGeneration": observedGeneration,
	}}}
}

func widgetPath(tenant string) string {
	return "/api/v1/tenants/" + tenant + "/resources/widgets.example.test/v1/widgets"
}

func policyPath() string {
	return "/api/v1/tenants/platform/resources/policies.example.test/v1beta1/clusterpolicies"
}

func widgetStreamPath(tenant string) string {
	return "/api/v1/tenants/" + tenant + "/resource-streams/widgets.example.test/v1/widgets"
}

func policyStreamPath(tenant string) string {
	return "/api/v1/tenants/" + tenant + "/resource-streams/policies.example.test/v1beta1/clusterpolicies"
}

func openStream(t *testing.T, client *http.Client, target string) (*http.Response, <-chan streamSnapshot) {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("stream response %d %q: %s", response.StatusCode, response.Header.Get("Content-Type"), body(response))
	}
	snapshots := make(chan streamSnapshot, 8)
	go func() {
		defer close(snapshots)
		scanner := bufio.NewScanner(response.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			var snapshot streamSnapshot
			if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &snapshot) == nil {
				snapshots <- snapshot
			}
		}
	}()
	return response, snapshots
}

func assertSnapshotUIDs(t *testing.T, snapshots <-chan streamSnapshot, expected ...string) {
	t.Helper()
	snapshot := receiveSnapshot(t, snapshots)
	{
		actual := make([]string, len(snapshot.Items))
		for i := range snapshot.Items {
			actual[i] = snapshot.Items[i].UID
		}
		if strings.Join(actual, ",") != strings.Join(expected, ",") {
			t.Fatalf("snapshot UIDs = %v, want %v", actual, expected)
		}
	}
}

func receiveSnapshot(t *testing.T, snapshots <-chan streamSnapshot) streamSnapshot {
	t.Helper()
	select {
	case snapshot, ok := <-snapshots:
		if !ok {
			t.Fatal("stream closed before snapshot")
		}
		return snapshot
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stream snapshot")
		return streamSnapshot{}
	}
}

func assertSnapshotHealth(t *testing.T, snapshots <-chan streamSnapshot, status string, stale bool) {
	t.Helper()
	select {
	case snapshot, ok := <-snapshots:
		if !ok || len(snapshot.Items) != 1 {
			t.Fatalf("health snapshot = %#v, open %v", snapshot, ok)
		}
		if snapshot.Items[0].Health.Status != status || snapshot.Items[0].Health.Stale != stale {
			t.Fatalf("snapshot health = %#v, want status %s stale %v", snapshot.Items[0].Health, status, stale)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for health snapshot")
	}
}

func waitForItems(t *testing.T, env *environment, tenant, expectedUID string) {
	waitForItemsAt(t, env, widgetPath(tenant), expectedUID)
}

func waitForItemsAt(t *testing.T, env *environment, path string, expectedUIDs ...string) {
	t.Helper()
	for range 100 {
		response := get(t, env.client, env.app.URL+path)
		if response.StatusCode == http.StatusOK {
			var payload struct {
				Items []resources.Summary `json:"items"`
			}
			decode(t, response, &payload)
			actual := make([]string, len(payload.Items))
			for i := range payload.Items {
				actual[i] = payload.Items[i].UID
			}
			if strings.Join(actual, ",") == strings.Join(expectedUIDs, ",") {
				return
			}
		} else {
			response.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("resources %v did not become current at %s", expectedUIDs, path)
}

func decodePage(t *testing.T, response *http.Response) streamSnapshot {
	t.Helper()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("query status %d: %s", response.StatusCode, body(response))
	}
	var page streamSnapshot
	decode(t, response, &page)
	return page
}

func waitForTotal(t *testing.T, env *environment, path string, expected int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response := get(t, env.client, env.app.URL+path)
		if response.StatusCode == http.StatusOK {
			page := decodePage(t, response)
			if page.Total == expected {
				return
			}
		} else {
			response.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("resource total did not become %d at %s", expected, path)
}

func waitForReady(t *testing.T, env *environment, path string) {
	t.Helper()
	for range 100 {
		response := get(t, env.client, env.app.URL+path)
		if response.StatusCode == http.StatusOK {
			response.Body.Close()
			return
		}
		response.Body.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("resource endpoint did not become ready at %s", path)
}

func waitForYAML(t *testing.T, env *environment, tenant, uid, expected string) {
	t.Helper()
	for range 100 {
		response := get(t, env.client, env.app.URL+widgetPath(tenant)+"/"+uid)
		if response.StatusCode == http.StatusOK {
			var detail resources.Detail
			decode(t, response, &detail)
			if strings.Contains(detail.YAML, expected) {
				return
			}
		} else {
			response.Body.Close()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("resource %s YAML did not contain %q", uid, expected)
}

func TestDeniedGroupsSeeNoTenantsAndCannotSupplyTenantID(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	env.login(t, "denied")
	response := get(t, env.client, env.app.URL+"/api/session")
	var payload struct {
		Tenants []any `json:"tenants"`
	}
	decode(t, response, &payload)
	if len(payload.Tenants) != 0 {
		t.Fatalf("tenants = %#v", payload.Tenants)
	}
	response = get(t, env.client, env.app.URL+"/api/tenants/platform/context")
	if response.StatusCode != 403 {
		t.Fatalf("status %d", response.StatusCode)
	}
}

func TestInvalidSessionIsUnauthenticated(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	request, _ := http.NewRequest("GET", env.app.URL+"/api/session", http.NoBody)
	request.AddCookie(&http.Cookie{Name: "yak8sui_session", Value: "invalid"})
	response, err := env.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 401 {
		t.Fatalf("status %d", response.StatusCode)
	}
}

func TestAuthenticationFailureReturnsUsableBrowserError(t *testing.T) {
	env := newEnvironment(t, time.Hour)
	response := get(t, env.client, env.app.URL+"/auth/callback?state=forged&code=forged")
	if response.Request.URL.Query().Get("authError") == "" {
		t.Fatalf("expected browser error, ended at %s", response.Request.URL)
	}
	response = get(t, env.client, env.app.URL+"/api/session")
	if response.StatusCode != 401 {
		t.Fatalf("failed sign-in established access: %d", response.StatusCode)
	}
}

func TestLoginUsesSecureHTTPOnlyCookieInHTTPSConfiguration(t *testing.T) {
	// Cookie policy itself is covered here without requiring a TLS OIDC round trip.
	cfg := config.Config{Server: config.ServerConfig{PublicURL: "https://yak8sui.example.test", SessionTTLRaw: "1h", WebDirectory: t.TempDir()}, Identity: config.IdentityConfig{IssuerURL: "https://id.example.test", ClientID: "test", ClientSecret: "secret", RedirectURL: "https://yak8sui.example.test/auth/callback"}, Observation: config.ObservationConfig{DatabasePath: "resources.db", Groups: []config.ResourceGroupConfig{{Group: "widgets.example.test", Include: []string{"widgets"}}}}, Tenants: []config.TenantConfig{{ID: "one", Name: "One", Groups: []string{"one"}, LabelSelector: "tenant=one"}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.SecureCookie() {
		t.Fatal("HTTPS public URL must default to secure cookies")
	}
}

func TestExpiryAndLogoutInvalidateAccess(t *testing.T) {
	t.Run("expiry", func(t *testing.T) {
		env := newEnvironment(t, 15*time.Millisecond)
		env.login(t, "allowed")
		time.Sleep(25 * time.Millisecond)
		response := get(t, env.client, env.app.URL+"/api/session")
		if response.StatusCode != 401 {
			t.Fatalf("status %d", response.StatusCode)
		}
	})
	t.Run("logout", func(t *testing.T) {
		env := newEnvironment(t, time.Hour)
		env.login(t, "allowed")
		request, _ := http.NewRequest("POST", env.app.URL+"/api/logout", http.NoBody)
		request.Header.Set("Origin", env.app.URL)
		response, err := env.client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 204 {
			t.Fatalf("logout %d", response.StatusCode)
		}
		response = get(t, env.client, env.app.URL+"/api/session")
		if response.StatusCode != 401 {
			t.Fatalf("session %d", response.StatusCode)
		}
	})
}

func get(t *testing.T, client *http.Client, target string) *http.Response {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decode(t *testing.T, response *http.Response, value any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(value); err != nil {
		t.Fatal(err)
	}
}

func body(response *http.Response) string {
	defer response.Body.Close()
	b, _ := io.ReadAll(response.Body)
	return string(b)
}
