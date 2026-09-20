package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ron96g/yak8sui/backend/config"
)

func TestLoadRejectsActionableInvalidTenantConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `server:
  publicURL: http://localhost:8080
identity:
  issuerURL: http://identity.test
  clientID: test
  redirectURL: http://localhost:8080/auth/callback
observation:
  databasePath: resources.db
  groups:
    - group: widgets.example.test
      include: [widgets]
      types:
        - resource: widgets
          labelSelector: inspect=true
tenants:
  - id: duplicate
    name: First
    groups: [admins]
    labelSelector: app=first
  - id: duplicate
    name: Second
    groups: []
    labelSelector: "broken in ("
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected invalid configuration")
	}
	for _, expected := range []string{"duplicates tenant duplicate", "groups must contain", "labelSelector"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not contain %q", err, expected)
		}
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	_ = os.WriteFile(path, []byte("unexpected: true\n"), 0o600)
	_, err := config.Load(path)
	if err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}
}

func TestValidateRejectsContradictoryAndMalformedResourceRules(t *testing.T) {
	cfg := config.Config{
		Server:   config.ServerConfig{PublicURL: "https://app.example.test", SessionTTLRaw: "1h"},
		Identity: config.IdentityConfig{IssuerURL: "https://id.example.test", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://app.example.test/auth/callback"},
		Observation: config.ObservationConfig{DatabasePath: "resources.db", Groups: []config.ResourceGroupConfig{{
			Group: "example.test", Include: []string{"widgets"}, Types: []config.ResourceTypeOverride{{Resource: "other", LabelSelector: "broken in ("}},
		}}},
		Tenants: []config.TenantConfig{{ID: "one", Name: "One", Groups: []string{"one"}, LabelSelector: "tenant=one"}},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected invalid resource rules")
	}
	for _, expected := range []string{"other which is not included", "labelSelector"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not contain %q", err, expected)
		}
	}
}

func TestValidateDefaultsOwnerReferenceResolverAndAcceptsCustomResolvers(t *testing.T) {
	cfg := config.Config{Server: config.ServerConfig{PublicURL: "https://app.example.test", SessionTTLRaw: "1h"}, Identity: config.IdentityConfig{IssuerURL: "https://id.example.test", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://app.example.test/auth/callback"}, Observation: config.ObservationConfig{DatabasePath: "resources.db", Groups: []config.ResourceGroupConfig{{Group: "", Include: []string{"configmaps"}}, {Group: "example.test", Include: []string{"widgets"}}}, Relationships: []config.RelationshipConfig{{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}}}}, Tenants: []config.TenantConfig{{ID: "one", Name: "One", Groups: []string{"one"}, LabelSelector: "tenant=one"}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Observation.Relationships[0].Resolver != "ownerReferences" {
		t.Fatal("resolver default was not applied")
	}
	cfg.Observation.Relationships[0] = config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "labelJoin", LabelJoin: &config.LabelJoinConfig{Labels: []config.LabelJoinMapping{{ParentLabel: "app.kubernetes.io/name", ChildLabel: "owner.example.test/name"}}}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid label join: %v", err)
	}
	cfg.Observation.Relationships[0] = config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "fieldReference", FieldReference: &config.FieldReferenceConfig{Path: "$.spec.owner"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid field reference: %v", err)
	}
	cfg.Observation.Relationships[0].Resolver = "labelSelector"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "resolver must be ownerReferences, labelJoin, or fieldReference") {
		t.Fatalf("unsupported resolver error = %v", err)
	}
}

func TestValidateRejectsInvalidFieldReferenceOptions(t *testing.T) {
	base := config.Config{Server: config.ServerConfig{PublicURL: "https://app.example.test", SessionTTLRaw: "1h"}, Identity: config.IdentityConfig{IssuerURL: "https://id.example.test", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://app.example.test/auth/callback"}, Observation: config.ObservationConfig{DatabasePath: "resources.db", Groups: []config.ResourceGroupConfig{{Group: "", Include: []string{"configmaps"}}, {Group: "example.test", Include: []string{"widgets"}}}}, Tenants: []config.TenantConfig{{ID: "one", Name: "One", Groups: []string{"one"}, LabelSelector: "tenant=one"}}}
	tests := []struct {
		name, path, want string
		options          *config.FieldReferenceConfig
	}{
		{name: "missing options", want: "fieldReference.path is required"},
		{name: "metadata", options: &config.FieldReferenceConfig{Path: "$.metadata.owner"}, want: "under $.spec or $.status"},
		{name: "root only", options: &config.FieldReferenceConfig{Path: "$.spec"}, want: "under $.spec or $.status"},
		{name: "array", options: &config.FieldReferenceConfig{Path: "$.status.owners[0]"}, want: "field access only"},
		{name: "wildcard", options: &config.FieldReferenceConfig{Path: "$.spec.*"}, want: "field access only"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			cfg.Observation.Relationships = []config.RelationshipConfig{{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "fieldReference", FieldReference: test.options}}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v, want %q", err, test.want)
			}
		})
	}
	base.Observation.Relationships = []config.RelationshipConfig{{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "fieldReference", FieldReference: &config.FieldReferenceConfig{Path: "$.status.owner"}, LabelJoin: &config.LabelJoinConfig{Labels: []config.LabelJoinMapping{{ParentLabel: "a", ChildLabel: "b"}}}}}
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "labelJoin is only valid") {
		t.Fatalf("cross-resolver validation error = %v", err)
	}
}

func TestValidateRejectsInvalidLabelJoinOptions(t *testing.T) {
	base := config.Config{Server: config.ServerConfig{PublicURL: "https://app.example.test", SessionTTLRaw: "1h"}, Identity: config.IdentityConfig{IssuerURL: "https://id.example.test", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://app.example.test/auth/callback"}, Observation: config.ObservationConfig{DatabasePath: "resources.db", Groups: []config.ResourceGroupConfig{{Group: "", Include: []string{"configmaps"}}, {Group: "example.test", Include: []string{"widgets"}}}}, Tenants: []config.TenantConfig{{ID: "one", Name: "One", Groups: []string{"one"}, LabelSelector: "tenant=one"}}}
	tests := []struct {
		name         string
		relationship config.RelationshipConfig
		want         string
	}{
		{name: "missing options", relationship: config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "labelJoin"}, want: "labelJoin.labels must contain"},
		{name: "empty mapping", relationship: config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "labelJoin", LabelJoin: &config.LabelJoinConfig{Labels: []config.LabelJoinMapping{{}}}}, want: "parentLabel is required"},
		{name: "duplicate mapping", relationship: config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "labelJoin", LabelJoin: &config.LabelJoinConfig{Labels: []config.LabelJoinMapping{{ParentLabel: "parent", ChildLabel: "child"}, {ParentLabel: "parent", ChildLabel: "child"}}}}, want: "duplicates an earlier mapping"},
		{name: "surrounding whitespace", relationship: config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, Resolver: "labelJoin", LabelJoin: &config.LabelJoinConfig{Labels: []config.LabelJoinMapping{{ParentLabel: " parent", ChildLabel: "child"}}}}, want: "must not have surrounding whitespace"},
		{name: "options on owner references", relationship: config.RelationshipConfig{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}, LabelJoin: &config.LabelJoinConfig{Labels: []config.LabelJoinMapping{{ParentLabel: "parent", ChildLabel: "child"}}}}, want: "only valid for resolver labelJoin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			cfg.Observation.Relationships = []config.RelationshipConfig{test.relationship}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v, want %q", err, test.want)
			}
		})
	}
}
