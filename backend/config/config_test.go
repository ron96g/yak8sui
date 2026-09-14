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

func TestValidateDefaultsOwnerReferenceResolverAndRejectsUnsupportedResolver(t *testing.T) {
	cfg := config.Config{Server: config.ServerConfig{PublicURL: "https://app.example.test", SessionTTLRaw: "1h"}, Identity: config.IdentityConfig{IssuerURL: "https://id.example.test", ClientID: "client", ClientSecret: "secret", RedirectURL: "https://app.example.test/auth/callback"}, Observation: config.ObservationConfig{DatabasePath: "resources.db", Groups: []config.ResourceGroupConfig{{Group: "", Include: []string{"configmaps"}}, {Group: "example.test", Include: []string{"widgets"}}}, Relationships: []config.RelationshipConfig{{Parent: config.RelationshipType{Kind: "ConfigMap"}, Child: config.RelationshipType{Group: "example.test", Kind: "Widget"}}}}, Tenants: []config.TenantConfig{{ID: "one", Name: "One", Groups: []string{"one"}, LabelSelector: "tenant=one"}}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Observation.Relationships[0].Resolver != "ownerReferences" {
		t.Fatal("resolver default was not applied")
	}
	cfg.Observation.Relationships[0].Resolver = "labelSelector"
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "resolver must be ownerReferences") {
		t.Fatalf("unsupported resolver error = %v", err)
	}
}
