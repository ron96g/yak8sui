package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/labels"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Identity    IdentityConfig    `yaml:"identity"`
	Observation ObservationConfig `yaml:"observation"`
	Tenants     []TenantConfig    `yaml:"tenants"`
}

type ObservationConfig struct {
	DatabasePath  string                `yaml:"databasePath"`
	Kubeconfig    string                `yaml:"kubeconfig"`
	Groups        []ResourceGroupConfig `yaml:"groups"`
	Relationships []RelationshipConfig  `yaml:"relationships"`
}

type RelationshipType struct {
	Group string `yaml:"group" json:"group"`
	Kind  string `yaml:"kind" json:"kind"`
}

type RelationshipConfig struct {
	Parent   RelationshipType `yaml:"parent" json:"parent"`
	Child    RelationshipType `yaml:"child" json:"child"`
	Resolver string           `yaml:"resolver" json:"resolver"`
}

type ResourceGroupConfig struct {
	Group   string                 `yaml:"group"`
	Include []string               `yaml:"include"`
	Exclude []string               `yaml:"exclude"`
	Types   []ResourceTypeOverride `yaml:"types"`
}

type ResourceTypeOverride struct {
	Resource      string `yaml:"resource"`
	Version       string `yaml:"version"`
	LabelSelector string `yaml:"labelSelector"`
}

type ServerConfig struct {
	Address       string        `yaml:"address"`
	PublicURL     string        `yaml:"publicURL"`
	WebDirectory  string        `yaml:"webDirectory"`
	SessionTTL    time.Duration `yaml:"-"`
	SessionTTLRaw string        `yaml:"sessionTTL"`
	CookieSecure  *bool         `yaml:"cookieSecure"`
}

type IdentityConfig struct {
	IssuerURL    string `yaml:"issuerURL"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
	RedirectURL  string `yaml:"redirectURL"`
	GroupsClaim  string `yaml:"groupsClaim"`
}

type TenantConfig struct {
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	Groups        []string `yaml:"groups"`
	LabelSelector string   `yaml:"labelSelector"`
}

func Load(path string) (Config, error) {
	// #nosec G304 -- the configuration path is an intentional command-line input.
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read configuration %q: %w", path, err)
	}
	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse configuration %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid configuration %q: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	var problems []string
	publicURL := c.validateServer(&problems)
	c.validateIdentity(publicURL, &problems)
	c.validateTenants(&problems)
	c.validateObservation(&problems)
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (c *Config) validateObservation(problems *[]string) {
	if strings.TrimSpace(c.Observation.DatabasePath) == "" {
		*problems = append(*problems, "observation.databasePath is required")
	}
	if len(c.Observation.Groups) == 0 {
		*problems = append(*problems, "observation.groups must contain at least one API group")
	}
	seenGroups := map[string]bool{}
	for i, group := range c.Observation.Groups {
		prefix := fmt.Sprintf("observation.groups[%d]", i)
		// The empty API group deliberately denotes Kubernetes core/v1 resources.
		if seenGroups[group.Group] {
			*problems = append(*problems, prefix+".group duplicates "+group.Group)
		}
		seenGroups[group.Group] = true
		includes := validateResourceNames(prefix+".include", group.Include, problems)
		excludes := validateResourceNames(prefix+".exclude", group.Exclude, problems)
		seenTypes := map[string]bool{}
		for j, override := range group.Types {
			typePrefix := fmt.Sprintf("%s.types[%d]", prefix, j)
			if strings.TrimSpace(override.Resource) == "" {
				*problems = append(*problems, typePrefix+".resource is required")
			} else if seenTypes[override.Resource] {
				*problems = append(*problems, typePrefix+".resource duplicates "+override.Resource)
			}
			seenTypes[override.Resource] = true
			if len(includes) > 0 && !includes[override.Resource] {
				*problems = append(*problems, typePrefix+" configures "+override.Resource+" which is not included")
			}
			if excludes[override.Resource] {
				*problems = append(*problems, typePrefix+" configures "+override.Resource+" which is excluded")
			}
			if override.LabelSelector != "" {
				if _, err := labels.Parse(override.LabelSelector); err != nil {
					*problems = append(*problems, fmt.Sprintf("%s.labelSelector %q is invalid: %v", typePrefix, override.LabelSelector, err))
				}
			}
		}
	}
	c.validateRelationships(seenGroups, problems)
}

func (c *Config) validateRelationships(seenGroups map[string]bool, problems *[]string) {
	seenRelationships := map[string]bool{}
	for i := range c.Observation.Relationships {
		relationship := &c.Observation.Relationships[i]
		prefix := fmt.Sprintf("observation.relationships[%d]", i)
		if strings.TrimSpace(relationship.Parent.Kind) == "" {
			*problems = append(*problems, prefix+".parent.kind is required")
		}
		if strings.TrimSpace(relationship.Child.Kind) == "" {
			*problems = append(*problems, prefix+".child.kind is required")
		}
		if relationship.Resolver == "" {
			relationship.Resolver = "ownerReferences"
		}
		if relationship.Resolver != "ownerReferences" {
			*problems = append(*problems, prefix+".resolver must be ownerReferences")
		}
		if !seenGroups[relationship.Parent.Group] {
			*problems = append(*problems, prefix+".parent.group is not configured for observation")
		}
		if !seenGroups[relationship.Child.Group] {
			*problems = append(*problems, prefix+".child.group is not configured for observation")
		}
		key := relationship.Parent.Group + "/" + relationship.Parent.Kind + "->" + relationship.Child.Group + "/" + relationship.Child.Kind
		if seenRelationships[key] {
			*problems = append(*problems, prefix+" duplicates "+key)
		}
		seenRelationships[key] = true
	}
}

func validateResourceNames(path string, values []string, problems *[]string) map[string]bool {
	seen := map[string]bool{}
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			*problems = append(*problems, fmt.Sprintf("%s[%d] must not be empty", path, i))
		} else if seen[value] {
			*problems = append(*problems, fmt.Sprintf("%s[%d] duplicates %s", path, i, value))
		}
		seen[value] = true
	}
	return seen
}

func (c *Config) validateServer(problems *[]string) *url.URL {
	if c.Server.Address == "" {
		c.Server.Address = ":8080"
	}
	if c.Server.WebDirectory == "" {
		c.Server.WebDirectory = "frontend/dist"
	}
	if c.Server.SessionTTLRaw == "" {
		c.Server.SessionTTLRaw = "8h"
	}
	ttl, err := time.ParseDuration(c.Server.SessionTTLRaw)
	if err != nil || ttl <= 0 {
		*problems = append(*problems, "server.sessionTTL must be a positive duration (for example 8h)")
	} else {
		c.Server.SessionTTL = ttl
	}
	if c.Server.PublicURL == "" {
		*problems = append(*problems, "server.publicURL is required")
		return nil
	}
	publicURL, ok := absoluteHTTPURL(c.Server.PublicURL)
	if !ok {
		*problems = append(*problems, "server.publicURL must be an absolute http(s) URL")
		return nil
	}
	return publicURL
}

func (c *Config) validateIdentity(publicURL *url.URL, problems *[]string) {
	for field, value := range map[string]string{"identity.issuerURL": c.Identity.IssuerURL, "identity.clientID": c.Identity.ClientID, "identity.clientSecret": c.Identity.ClientSecret, "identity.redirectURL": c.Identity.RedirectURL} {
		if strings.TrimSpace(value) == "" {
			*problems = append(*problems, field+" is required")
		}
	}
	if c.Identity.GroupsClaim == "" {
		c.Identity.GroupsClaim = "groups"
	}
	if c.Identity.GroupsClaim == "sub" || c.Identity.GroupsClaim == "iss" || c.Identity.GroupsClaim == "aud" || c.Identity.GroupsClaim == "exp" || c.Identity.GroupsClaim == "iat" || c.Identity.GroupsClaim == "nonce" {
		*problems = append(*problems, "identity.groupsClaim must not use a reserved OIDC claim")
	}
	if c.Identity.IssuerURL != "" {
		if _, ok := absoluteHTTPURL(c.Identity.IssuerURL); !ok {
			*problems = append(*problems, "identity.issuerURL must be an absolute http(s) URL")
		}
	}
	if c.Identity.RedirectURL != "" {
		u, ok := absoluteHTTPURL(c.Identity.RedirectURL)
		if !ok {
			*problems = append(*problems, "identity.redirectURL must be an absolute http(s) URL")
		} else if publicURL != nil && (u.Scheme != publicURL.Scheme || u.Host != publicURL.Host || u.Path != "/auth/callback") {
			*problems = append(*problems, "identity.redirectURL must use server.publicURL origin and path /auth/callback")
		}
	}
}

func (c *Config) validateTenants(problems *[]string) {
	if len(c.Tenants) == 0 {
		*problems = append(*problems, "tenants must contain at least one tenant")
	}
	seen := map[string]bool{}
	for i, tenant := range c.Tenants {
		prefix := fmt.Sprintf("tenants[%d]", i)
		switch {
		case tenant.ID == "":
			*problems = append(*problems, prefix+".id is required")
		case strings.ContainsAny(tenant.ID, "/?#"):
			*problems = append(*problems, prefix+".id must not contain '/', '?' or '#'")
		case seen[tenant.ID]:
			*problems = append(*problems, prefix+".id duplicates tenant "+tenant.ID)
		}
		seen[tenant.ID] = true
		if tenant.Name == "" {
			*problems = append(*problems, prefix+".name is required")
		}
		if len(tenant.Groups) == 0 {
			*problems = append(*problems, prefix+".groups must contain at least one identity-provider group")
		}
		if tenant.LabelSelector == "" {
			*problems = append(*problems, prefix+".labelSelector is required")
		} else if _, err := labels.Parse(tenant.LabelSelector); err != nil {
			*problems = append(*problems, fmt.Sprintf("%s.labelSelector %q is invalid: %v", prefix, tenant.LabelSelector, err))
		}
	}
}

func absoluteHTTPURL(raw string) (*url.URL, bool) {
	u, err := url.ParseRequestURI(raw)
	return u, err == nil && u.Scheme != "" && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func (c *Config) SecureCookie() bool {
	if c.Server.CookieSecure != nil {
		return *c.Server.CookieSecure
	}
	return strings.HasPrefix(c.Server.PublicURL, "https://")
}
