package resources

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ron96g/yak8sui/backend/config"
)

// RelationshipResolver is the boundary for configured object-to-object edge
// resolution. OwnerReferencesResolver is the only implementation currently
// accepted by configuration.
type RelationshipResolver interface {
	Resolve(context.Context, string, *Type, string, []config.RelationshipConfig, *Catalog) (Neighborhood, error)
}

type RelationshipTypeStatus struct {
	Group       string            `json:"group"`
	Kind        string            `json:"kind"`
	Observation ObservationStatus `json:"observation"`
}

type RelationshipNode struct {
	Type       Type   `json:"type"`
	UID        string `json:"uid"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	Controller bool   `json:"controller,omitempty"`
	Presentation
}

type RelationshipCompleteness struct {
	Complete bool                     `json:"complete"`
	Limited  bool                     `json:"configurationLimited"`
	Scopes   []RelationshipTypeStatus `json:"unavailableScopes,omitempty"`
}

type Neighborhood struct {
	Center   RelationshipNode         `json:"center"`
	Parents  []RelationshipNode       `json:"parents"`
	Children []RelationshipNode       `json:"children"`
	Upward   RelationshipCompleteness `json:"upward"`
	Downward RelationshipCompleteness `json:"downward"`
}

type ownerReference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	UID        string `json:"uid"`
	Controller *bool  `json:"controller"`
}

type projectedObject struct {
	UID, Namespace, Name string
	Contents             []byte
}

type OwnerReferencesResolver struct{ projection *Projection }

func NewOwnerReferencesResolver(projection *Projection) *OwnerReferencesResolver {
	return &OwnerReferencesResolver{projection: projection}
}

func (r *OwnerReferencesResolver) Resolve(ctx context.Context, tenantID string, centerType *Type, centerUID string, rules []config.RelationshipConfig, catalog *Catalog) (Neighborhood, error) {
	center, err := r.projection.authorizedObject(ctx, tenantID, centerType.Key(), centerUID)
	if err != nil {
		return Neighborhood{}, err
	}
	result := Neighborhood{Center: relationshipNode(centerType, center, false)}
	upTypes, downTypes := relationshipTypes(centerType, rules, catalog)
	result.Upward = r.completeness(ctx, upTypes, len(upTypes) == 0)
	result.Downward = r.completeness(ctx, downTypes, len(downTypes) == 0)
	result.Parents, err = r.parents(ctx, tenantID, centerType, center, rules, catalog)
	if err != nil {
		return Neighborhood{}, err
	}
	result.Children, err = r.children(ctx, tenantID, centerType, center, rules, catalog)
	if err != nil {
		return Neighborhood{}, err
	}
	sortNodes(result.Parents)
	sortNodes(result.Children)
	return result, nil
}

func (r *OwnerReferencesResolver) parents(ctx context.Context, tenantID string, centerType *Type, center projectedObject, rules []config.RelationshipConfig, catalog *Catalog) ([]RelationshipNode, error) {
	refs, err := decodeOwnerReferences(center.Contents)
	if err != nil {
		return nil, err
	}
	var result []RelationshipNode
	seenParents := map[string]bool{}
	for _, rule := range rules {
		if rule.Child.Group != centerType.Group || rule.Child.Kind != centerType.Kind {
			continue
		}
		parentType, ok := catalog.FindByGroupKind(rule.Parent.Group, rule.Parent.Kind)
		if !ok || !current(ctx, r.projection, &parentType) {
			continue
		}
		for _, ref := range refs {
			if !ownerReferenceMatches(ref, &parentType) || seenParents[parentType.Key()+"/"+ref.UID] {
				continue
			}
			parent, getErr := r.projection.authorizedObject(ctx, tenantID, parentType.Key(), ref.UID)
			if errors.Is(getErr, sql.ErrNoRows) {
				continue
			}
			if getErr != nil {
				return nil, getErr
			}
			if !validOwnershipScope(&parentType, parent.Namespace, centerType, center.Namespace) {
				continue
			}
			seenParents[parentType.Key()+"/"+ref.UID] = true
			result = append(result, relationshipNode(&parentType, parent, ref.Controller != nil && *ref.Controller))
		}
	}
	return result, nil
}

func (r *OwnerReferencesResolver) children(ctx context.Context, tenantID string, centerType *Type, center projectedObject, rules []config.RelationshipConfig, catalog *Catalog) ([]RelationshipNode, error) {
	var result []RelationshipNode
	seenChildren := map[string]bool{}
	for _, rule := range rules {
		if rule.Parent.Group != centerType.Group || rule.Parent.Kind != centerType.Kind {
			continue
		}
		childType, ok := catalog.FindByGroupKind(rule.Child.Group, rule.Child.Kind)
		if !ok || !current(ctx, r.projection, &childType) {
			continue
		}
		children, listErr := r.projection.authorizedObjects(ctx, tenantID, childType.Key())
		if listErr != nil {
			return nil, listErr
		}
		for _, child := range children {
			for _, ref := range mustOwnerReferences(child.Contents) {
				key := childType.Key() + "/" + child.UID
				if ref.UID != center.UID || !ownerReferenceMatches(ref, centerType) || seenChildren[key] || !validOwnershipScope(centerType, center.Namespace, &childType, child.Namespace) {
					continue
				}
				seenChildren[key] = true
				result = append(result, relationshipNode(&childType, child, ref.Controller != nil && *ref.Controller))
			}
		}
	}
	return result, nil
}

func relationshipTypes(center *Type, rules []config.RelationshipConfig, catalog *Catalog) (parents, children []Type) {
	for _, rule := range rules {
		if rule.Child.Group == center.Group && rule.Child.Kind == center.Kind {
			if item, ok := catalog.FindByGroupKind(rule.Parent.Group, rule.Parent.Kind); ok {
				parents = appendType(parents, &item)
			} else {
				parents = appendType(parents, unresolvedRelationshipType(rule.Parent))
			}
		}
		if rule.Parent.Group == center.Group && rule.Parent.Kind == center.Kind {
			if item, ok := catalog.FindByGroupKind(rule.Child.Group, rule.Child.Kind); ok {
				children = appendType(children, &item)
			} else {
				children = appendType(children, unresolvedRelationshipType(rule.Child))
			}
		}
	}
	return parents, children
}

func unresolvedRelationshipType(configured config.RelationshipType) *Type {
	reason := "configured relationship type is not currently discovered"
	return &Type{Group: configured.Group, Kind: configured.Kind, Available: false, Observation: ObservationStatus{State: ObservationUnavailable, Reason: reason}}
}

func appendType(items []Type, item *Type) []Type {
	for i := range items {
		if items[i].Key() == item.Key() {
			return items
		}
	}
	return append(items, *item)
}

func (r *OwnerReferencesResolver) completeness(ctx context.Context, types []Type, limited bool) RelationshipCompleteness {
	result := RelationshipCompleteness{Complete: true, Limited: limited}
	for i := range types {
		item := &types[i]
		if !item.Available && item.Key() == "//" {
			result.Complete = false
			result.Scopes = append(result.Scopes, RelationshipTypeStatus{Group: item.Group, Kind: item.Kind, Observation: item.Observation})
			continue
		}
		status, err := r.projection.ObservationStatus(ctx, item.Key())
		if err != nil {
			status = ObservationStatus{State: ObservationUnavailable, Reason: "observation status could not be loaded"}
		}
		if !item.Available && status.State == "" {
			status = item.Observation
		}
		if !status.Synchronized() {
			result.Complete = false
			result.Scopes = append(result.Scopes, RelationshipTypeStatus{Group: item.Group, Kind: item.Kind, Observation: status})
		}
	}
	return result
}

func current(ctx context.Context, projection *Projection, item *Type) bool {
	status, err := projection.ObservationStatus(ctx, item.Key())
	return err == nil && item.Available && status.Synchronized()
}

func ownerReferenceMatches(ref ownerReference, owner *Type) bool {
	gv := strings.SplitN(ref.APIVersion, "/", 2)
	group, version := "", gv[0]
	if len(gv) == 2 {
		group, version = gv[0], gv[1]
	}
	return ref.UID != "" && group == owner.Group && version == owner.Version && ref.Kind == owner.Kind
}

func validOwnershipScope(parent *Type, parentNamespace string, child *Type, childNamespace string) bool {
	if child.Scope == ScopeCluster {
		return parent.Scope == ScopeCluster
	}
	return parent.Scope == ScopeCluster || (parent.Scope == ScopeNamespaced && parentNamespace == childNamespace)
}

func decodeOwnerReferences(contents []byte) ([]ownerReference, error) {
	var object struct {
		Metadata struct {
			OwnerReferences []ownerReference `json:"ownerReferences"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(contents, &object); err != nil {
		return nil, fmt.Errorf("decode owner references: %w", err)
	}
	return object.Metadata.OwnerReferences, nil
}

func mustOwnerReferences(contents []byte) []ownerReference {
	refs, err := decodeOwnerReferences(contents)
	if err != nil {
		return nil
	}
	return refs
}

func relationshipNode(resourceType *Type, object projectedObject, controller bool) RelationshipNode {
	return RelationshipNode{Type: *resourceType, UID: object.UID, Name: object.Name, Namespace: object.Namespace, Controller: controller, Presentation: present(object.Contents)}
}

func sortNodes(nodes []RelationshipNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Namespace != nodes[j].Namespace {
			return nodes[i].Namespace < nodes[j].Namespace
		}
		if nodes[i].Name != nodes[j].Name {
			return nodes[i].Name < nodes[j].Name
		}
		return nodes[i].UID < nodes[j].UID
	})
}
