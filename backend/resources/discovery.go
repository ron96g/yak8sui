package resources

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ron96g/yak8sui/backend/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

const (
	ScopeNamespaced = "Namespaced"
	ScopeCluster    = "Cluster"
)

type Type struct {
	Group         string            `json:"group"`
	Version       string            `json:"version,omitempty"`
	Resource      string            `json:"resource"`
	Kind          string            `json:"kind,omitempty"`
	Scope         string            `json:"scope"`
	Available     bool              `json:"available"`
	Reason        string            `json:"reason,omitempty"`
	Observation   ObservationStatus `json:"observation"`
	LabelSelector string            `json:"-"`
}

func (t *Type) Key() string { return t.Group + "/" + t.Version + "/" + t.Resource }
func (t *Type) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: t.Group, Version: t.Version, Resource: t.Resource}
}

type Catalog struct {
	mu              sync.RWMutex
	types           []Type
	byKey           map[string]Type
	discoveredKinds map[string]bool
}

func (c *Catalog) Types() []Type {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Type(nil), c.types...)
}

func (c *Catalog) Find(group, version, resource string) (Type, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.byKey[group+"/"+version+"/"+resource]
	return t, ok
}

func (c *Catalog) FindByGroupKind(group, kind string) (Type, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.types {
		if c.types[i].Group == group && c.types[i].Kind == kind {
			return c.types[i], true
		}
	}
	return Type{}, false
}

func (c *Catalog) KindDiscovered(group, kind string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.discoveredKinds[group+"/"+kind]
}

func (c *Catalog) replaceType(previousKey string, current *Type) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.byKey, previousKey)
	c.byKey[current.Key()] = *current
	for i := range c.types {
		if c.types[i].Key() == previousKey || (c.types[i].Group == current.Group && c.types[i].Resource == current.Resource) {
			c.types[i] = *current
			return
		}
	}
}

func (c *Catalog) setObservation(key string, status ObservationStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.byKey[key]
	if !ok {
		return
	}
	item.Observation = status
	c.byKey[key] = item
	for i := range c.types {
		if c.types[i].Key() == key {
			c.types[i] = item
			return
		}
	}
}

type Discovery interface {
	ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error)
}

func Discover(client Discovery, configured []config.ResourceGroupConfig) *Catalog {
	groups, lists, discoveryErr := client.ServerGroupsAndResources()
	preferred, versions, available := indexDiscovery(groups, lists)
	selected := selectTypes(configured, preferred, versions, available, discoveryErr)
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Group != selected[j].Group {
			return selected[i].Group < selected[j].Group
		}
		if selected[i].Version != selected[j].Version {
			return selected[i].Version < selected[j].Version
		}
		return selected[i].Kind < selected[j].Kind
	})
	byKey := make(map[string]Type, len(selected))
	for i := range selected {
		if selected[i].Available {
			selected[i].Observation = ObservationStatus{State: ObservationResynchronizing, Reason: "waiting for initial synchronization"}
		} else {
			selected[i].Observation = ObservationStatus{State: ObservationUnavailable, Reason: selected[i].Reason}
		}
		byKey[selected[i].Key()] = selected[i]
	}
	discoveredKinds := map[string]bool{}
	for group, resources := range available {
		for resourceKey := range resources {
			discoveredKinds[group+"/"+resources[resourceKey].Kind] = true
		}
	}
	return &Catalog{types: selected, byKey: byKey, discoveredKinds: discoveredKinds}
}

func indexDiscovery(groups []*metav1.APIGroup, lists []*metav1.APIResourceList) (map[string]string, map[string][]string, map[string]map[string]metav1.APIResource) {
	preferred := map[string]string{}
	versions := map[string][]string{}
	for _, group := range groups {
		preferred[group.Name] = group.PreferredVersion.Version
		for _, version := range group.Versions {
			versions[group.Name] = append(versions[group.Name], version.Version)
		}
	}
	available := map[string]map[string]metav1.APIResource{}
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		if available[gv.Group] == nil {
			available[gv.Group] = map[string]metav1.APIResource{}
		}
		for i := range list.APIResources {
			resource := &list.APIResources[i]
			if strings.Contains(resource.Name, "/") || resource.Kind == "" {
				continue
			}
			available[gv.Group][gv.Version+"/"+resource.Name] = *resource
			if !contains(versions[gv.Group], gv.Version) {
				versions[gv.Group] = append(versions[gv.Group], gv.Version)
			}
		}
	}
	return preferred, versions, available
}

func selectTypes(configured []config.ResourceGroupConfig, preferred map[string]string, versions map[string][]string, available map[string]map[string]metav1.APIResource, discoveryErr error) []Type {
	selected := []Type{}
	for _, rule := range configured {
		overrides := map[string]config.ResourceTypeOverride{}
		for _, override := range rule.Types {
			overrides[override.Resource] = override
		}
		excluded := stringSet(rule.Exclude)
		resourceNames := append([]string(nil), rule.Include...)
		if len(resourceNames) == 0 {
			seen := map[string]bool{}
			for key := range available[rule.Group] {
				resource := strings.SplitN(key, "/", 2)[1]
				if !seen[resource] {
					resourceNames = append(resourceNames, resource)
					seen[resource] = true
				}
			}
		}
		sort.Strings(resourceNames)
		for _, resourceName := range resourceNames {
			if excluded[resourceName] {
				continue
			}
			override := overrides[resourceName]
			version := override.Version
			if version == "" {
				version = preferred[rule.Group]
			}
			apiResource, ok := available[rule.Group][version+"/"+resourceName]
			if !ok && override.Version == "" {
				for _, candidate := range versions[rule.Group] {
					if found, exists := available[rule.Group][candidate+"/"+resourceName]; exists {
						version, apiResource, ok = candidate, found, true
						break
					}
				}
			}
			item := Type{Group: rule.Group, Version: version, Resource: resourceName, Scope: healthUnknown, Available: ok, LabelSelector: override.LabelSelector}
			if ok {
				item.Kind = apiResource.Kind
				if apiResource.Namespaced {
					item.Scope = ScopeNamespaced
				} else {
					item.Scope = ScopeCluster
				}
			} else {
				item.Reason = unavailableReason(rule.Group, resourceName, version, discoveryErr)
			}
			selected = append(selected, item)
		}
	}
	return selected
}

func unavailableReason(group, resource, version string, err error) string {
	target := group + "/" + version + " resource " + resource
	if version == "" {
		target = group + " resource " + resource
	}
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return fmt.Sprintf("%s is unavailable: %v", target, err)
	}
	return target + " is not currently served"
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
