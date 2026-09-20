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

//go:generate go run ../../cmd/configschema -output ../../config/config.schema.json

type Config struct {
	Server      ServerConfig      `yaml:"server" jsonschema:"required" jsonschema_description:"HTTP server and browser-session settings."`
	Identity    IdentityConfig    `yaml:"identity" jsonschema:"required" jsonschema_description:"OpenID Connect identity-provider settings."`
	Observation ObservationConfig `yaml:"observation" jsonschema:"required" jsonschema_description:"Kubernetes discovery, observation, and relationship settings."`
	Tenants     []TenantConfig    `yaml:"tenants" jsonschema:"required,minItems=1" jsonschema_description:"Tenant security boundaries. At least one access or role group is required per tenant."`
}

type ObservationConfig struct {
	DatabasePath  string                `yaml:"databasePath" jsonschema:"required,minLength=1" jsonschema_description:"Writable SQLite database path."`
	Kubeconfig    string                `yaml:"kubeconfig" jsonschema_description:"Optional kubeconfig path; omit to use in-cluster credentials."`
	Groups        []ResourceGroupConfig `yaml:"groups" jsonschema:"required,minItems=1" jsonschema_description:"API groups and resources selected for observation."`
	Relationships []RelationshipConfig  `yaml:"relationships" jsonschema_description:"Configured directional ownership relationships."`
}

type RelationshipType struct {
	Group string `yaml:"group" json:"group" jsonschema_description:"API group; use an empty string for Kubernetes core resources."`
	Kind  string `yaml:"kind" json:"kind" jsonschema:"required,minLength=1" jsonschema_description:"Case-sensitive Kubernetes kind."`
}

type RelationshipConfig struct {
	Parent         RelationshipType      `yaml:"parent" json:"parent" jsonschema:"required"`
	Child          RelationshipType      `yaml:"child" json:"child" jsonschema:"required"`
	Resolver       string                `yaml:"resolver" json:"resolver" jsonschema:"enum=ownerReferences,enum=labelJoin,enum=fieldReference,default=ownerReferences" jsonschema_description:"Relationship resolver; defaults to ownerReferences."`
	LabelJoin      *LabelJoinConfig      `yaml:"labelJoin,omitempty" json:"labelJoin,omitempty" jsonschema_description:"Required options when resolver is labelJoin."`
	FieldReference *FieldReferenceConfig `yaml:"fieldReference,omitempty" json:"fieldReference,omitempty" jsonschema_description:"Required options when resolver is fieldReference."`
}

type FieldReferenceConfig struct {
	Path string `yaml:"path" json:"path" jsonschema:"required,pattern=^\\$\\.(spec|status)\\.[A-Za-z0-9_-]+(?:\\.[A-Za-z0-9_-]+)*$" jsonschema_description:"Field-only path below $.spec or $.status; arrays and wildcards are unsupported."`
}

type LabelJoinConfig struct {
	Labels []LabelJoinMapping `yaml:"labels" json:"labels" jsonschema:"required,minItems=1"`
}

type LabelJoinMapping struct {
	ParentLabel string `yaml:"parentLabel" json:"parentLabel" jsonschema:"required,minLength=1"`
	ChildLabel  string `yaml:"childLabel" json:"childLabel" jsonschema:"required,minLength=1"`
}

type ResourceGroupConfig struct {
	Group   string                 `yaml:"group" jsonschema_description:"API group; use an empty string for Kubernetes core resources."`
	Include []string               `yaml:"include" jsonschema:"uniqueItems=true" jsonschema_description:"Resource names to include; an empty list discovers the full API group."`
	Exclude []string               `yaml:"exclude" jsonschema:"uniqueItems=true" jsonschema_description:"Resource names to exclude; exclusions take precedence."`
	Types   []ResourceTypeOverride `yaml:"types" jsonschema_description:"Per-resource version and label-selector overrides."`
}

type ResourceTypeOverride struct {
	Resource      string `yaml:"resource" jsonschema:"required,minLength=1" jsonschema_description:"Plural Kubernetes API resource name."`
	Version       string `yaml:"version" jsonschema_description:"Served version override; omit to use the preferred version."`
	LabelSelector string `yaml:"labelSelector" jsonschema_description:"Additional Kubernetes label selector for this resource type."`
}

type ServerConfig struct {
	Address       string        `yaml:"address" jsonschema:"default=:8080" jsonschema_description:"HTTP listen address."`
	PublicURL     string        `yaml:"publicURL" jsonschema:"required,format=uri" jsonschema_description:"Externally visible absolute HTTP or HTTPS URL."`
	WebDirectory  string        `yaml:"webDirectory" jsonschema:"default=frontend/dist" jsonschema_description:"Directory containing the built frontend."`
	SessionTTL    time.Duration `yaml:"-"`
	SessionTTLRaw string        `yaml:"sessionTTL" jsonschema:"default=8h" jsonschema_description:"Positive Go duration for browser sessions, for example 8h or 30m. Syntax is validated at startup."`
	CookieSecure  *bool         `yaml:"cookieSecure" jsonschema_description:"Override secure-cookie behavior; defaults to true for HTTPS publicURL."`
}

type IdentityConfig struct {
	IssuerURL    string `yaml:"issuerURL" jsonschema:"required,format=uri" jsonschema_description:"Absolute HTTP or HTTPS OpenID Connect issuer URL."`
	ClientID     string `yaml:"clientID" jsonschema:"required,minLength=1"`
	ClientSecret string `yaml:"clientSecret" jsonschema:"required,minLength=1" jsonschema_description:"OIDC client secret; inject through a secret manager."`
	RedirectURL  string `yaml:"redirectURL" jsonschema:"required,format=uri" jsonschema_description:"OIDC callback URL using the publicURL origin and /auth/callback path."`
	GroupsClaim  string `yaml:"groupsClaim" jsonschema:"default=groups,minLength=1" jsonschema_description:"OIDC claim containing identity-provider group names."`
}

type TenantConfig struct {
	ID               string   `yaml:"id" jsonschema:"required,minLength=1,pattern=^[^/?#]+$"`
	Name             string   `yaml:"name" jsonschema:"required,minLength=1"`
	Groups           []string `yaml:"groups" jsonschema_description:"Identity-provider groups granted read-only access."`
	SupervisorGroups []string `yaml:"supervisorGroups" jsonschema_description:"Identity-provider groups granted read and trigger access."`
	AdminGroups      []string `yaml:"adminGroups" jsonschema_description:"Identity-provider groups granted read, trigger, update, and delete access."`
	LabelSelector    string   `yaml:"labelSelector" jsonschema:"required,minLength=1" jsonschema_description:"Kubernetes label selector defining this tenant's resources."`
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
		switch relationship.Resolver {
		case "ownerReferences":
			if relationship.LabelJoin != nil {
				*problems = append(*problems, prefix+".labelJoin is only valid for resolver labelJoin")
			}
			if relationship.FieldReference != nil {
				*problems = append(*problems, prefix+".fieldReference is only valid for resolver fieldReference")
			}
		case "labelJoin":
			validateLabelJoin(prefix, relationship.LabelJoin, problems)
			if relationship.FieldReference != nil {
				*problems = append(*problems, prefix+".fieldReference is only valid for resolver fieldReference")
			}
		case "fieldReference":
			validateFieldReference(prefix, relationship.FieldReference, problems)
			if relationship.LabelJoin != nil {
				*problems = append(*problems, prefix+".labelJoin is only valid for resolver labelJoin")
			}
		default:
			*problems = append(*problems, prefix+".resolver must be ownerReferences, labelJoin, or fieldReference")
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

func validateFieldReference(prefix string, options *FieldReferenceConfig, problems *[]string) {
	if options == nil || strings.TrimSpace(options.Path) == "" {
		*problems = append(*problems, prefix+".fieldReference.path is required")
		return
	}
	path := options.Path
	if strings.TrimSpace(path) != path {
		*problems = append(*problems, prefix+".fieldReference.path must not have surrounding whitespace")
		return
	}
	if !strings.HasPrefix(path, "$.spec.") && !strings.HasPrefix(path, "$.status.") {
		*problems = append(*problems, prefix+".fieldReference.path must select a child field under $.spec or $.status")
		return
	}
	for _, segment := range strings.Split(path[2:], ".") {
		if !isFieldReferenceSegment(segment) {
			*problems = append(*problems, prefix+".fieldReference.path must contain field access only; arrays and wildcards are not supported")
			return
		}
	}
}

func isFieldReferenceSegment(segment string) bool {
	if segment == "" {
		return false
	}
	for _, character := range segment {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func validateLabelJoin(prefix string, options *LabelJoinConfig, problems *[]string) {
	if options == nil || len(options.Labels) == 0 {
		*problems = append(*problems, prefix+".labelJoin.labels must contain at least one mapping")
		return
	}
	seenMappings := map[string]bool{}
	for i, mapping := range options.Labels {
		mappingPrefix := fmt.Sprintf("%s.labelJoin.labels[%d]", prefix, i)
		parentLabel := strings.TrimSpace(mapping.ParentLabel)
		childLabel := strings.TrimSpace(mapping.ChildLabel)
		if parentLabel == "" {
			*problems = append(*problems, mappingPrefix+".parentLabel is required")
		} else if parentLabel != mapping.ParentLabel {
			*problems = append(*problems, mappingPrefix+".parentLabel must not have surrounding whitespace")
		}
		if childLabel == "" {
			*problems = append(*problems, mappingPrefix+".childLabel is required")
		} else if childLabel != mapping.ChildLabel {
			*problems = append(*problems, mappingPrefix+".childLabel must not have surrounding whitespace")
		}
		key := parentLabel + "\x00" + childLabel
		if seenMappings[key] {
			*problems = append(*problems, mappingPrefix+" duplicates an earlier mapping")
		}
		seenMappings[key] = true
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
		if len(tenant.Groups)+len(tenant.SupervisorGroups)+len(tenant.AdminGroups) == 0 {
			*problems = append(*problems, prefix+" must contain at least one identity-provider group across groups, supervisorGroups, or adminGroups")
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
