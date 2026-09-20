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

// RelationshipResolver is the boundary for configured object-to-object edge resolution.
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
	Problems []RelationshipProblem    `json:"problems,omitempty"`
}

type RelationshipProblem struct {
	Parent   config.RelationshipType `json:"parent"`
	Child    config.RelationshipType `json:"child"`
	Resolver string                  `json:"resolver"`
	Reason   string                  `json:"reason"`
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

type CompositeRelationshipResolver struct{ projection *Projection }

func NewRelationshipResolver(projection *Projection) *CompositeRelationshipResolver {
	return &CompositeRelationshipResolver{projection: projection}
}

func (r *CompositeRelationshipResolver) Resolve(ctx context.Context, tenantID string, centerType *Type, centerUID string, rules []config.RelationshipConfig, catalog *Catalog) (Neighborhood, error) {
	center, err := r.projection.authorizedObject(ctx, tenantID, centerType.Key(), centerUID)
	if err != nil {
		return Neighborhood{}, err
	}
	result := Neighborhood{Center: relationshipNode(centerType, center, false)}
	upTypes, downTypes := relationshipTypes(centerType, rules, catalog)
	result.Upward = r.completeness(ctx, upTypes, len(upTypes) == 0)
	result.Downward = r.completeness(ctx, downTypes, len(downTypes) == 0)
	var upwardProblems, downwardProblems []RelationshipProblem
	result.Parents, upwardProblems, err = r.parents(ctx, tenantID, centerType, center, rules, catalog)
	if err != nil {
		return Neighborhood{}, err
	}
	result.Children, downwardProblems, err = r.children(ctx, tenantID, centerType, center, rules, catalog)
	if err != nil {
		return Neighborhood{}, err
	}
	sortNodes(result.Parents)
	sortNodes(result.Children)
	result.Upward.Problems = upwardProblems
	result.Downward.Problems = downwardProblems
	result.Upward.Complete = result.Upward.Complete && len(upwardProblems) == 0
	result.Downward.Complete = result.Downward.Complete && len(downwardProblems) == 0
	return result, nil
}

func (r *CompositeRelationshipResolver) parents(ctx context.Context, tenantID string, centerType *Type, center projectedObject, rules []config.RelationshipConfig, catalog *Catalog) ([]RelationshipNode, []RelationshipProblem, error) {
	refs, err := decodeOwnerReferences(center.Contents)
	if err != nil {
		return nil, nil, err
	}
	var result []RelationshipNode
	var problems []RelationshipProblem
	seenParents := map[string]bool{}
	for _, rule := range rules {
		if rule.Child.Group != centerType.Group || rule.Child.Kind != centerType.Kind {
			continue
		}
		parentType, ok := catalog.FindByGroupKind(rule.Parent.Group, rule.Parent.Kind)
		if !ok || !current(ctx, r.projection, &parentType) {
			continue
		}
		if rule.Resolver == "labelJoin" {
			matched, problem, listErr := r.resolveLabelJoinParent(ctx, tenantID, &parentType, centerType, center, &rule)
			if listErr != nil {
				return nil, nil, listErr
			}
			if problem != nil {
				problems = append(problems, *problem)
			}
			if matched != nil && !seenParents[parentType.Key()+"/"+matched.UID] {
				seenParents[parentType.Key()+"/"+matched.UID] = true
				result = append(result, relationshipNode(&parentType, *matched, false))
			}
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
				return nil, nil, getErr
			}
			if !validOwnershipScope(&parentType, parent.Namespace, centerType, center.Namespace) {
				continue
			}
			seenParents[parentType.Key()+"/"+ref.UID] = true
			result = append(result, relationshipNode(&parentType, parent, ref.Controller != nil && *ref.Controller))
		}
	}
	return result, problems, nil
}

func (r *CompositeRelationshipResolver) resolveLabelJoinParent(ctx context.Context, tenantID string, parentType, childType *Type, child projectedObject, rule *config.RelationshipConfig) (*projectedObject, *RelationshipProblem, error) {
	if childType.Scope == ScopeCluster && parentType.Scope == ScopeNamespaced {
		return nil, nil, nil
	}
	labels, ok := joinedLabels(child.Contents, rule.LabelJoin.Labels, false)
	if !ok {
		return nil, nil, nil
	}
	namespace := ""
	if parentType.Scope == ScopeNamespaced {
		namespace = child.Namespace
	}
	parents, err := r.projection.authorizedObjectsByLabels(ctx, tenantID, parentType.Key(), namespace, labels, 2)
	if err != nil {
		return nil, nil, err
	}
	if len(parents) > 1 {
		problem := ambiguityProblem(rule)
		return nil, &problem, nil
	}
	if len(parents) == 1 && validOwnershipScope(parentType, parents[0].Namespace, childType, child.Namespace) {
		return &parents[0], nil, nil
	}
	return nil, nil, nil
}

func (r *CompositeRelationshipResolver) children(ctx context.Context, tenantID string, centerType *Type, center projectedObject, rules []config.RelationshipConfig, catalog *Catalog) ([]RelationshipNode, []RelationshipProblem, error) {
	var result []RelationshipNode
	var problems []RelationshipProblem
	seenChildren := map[string]bool{}
	for _, rule := range rules {
		if rule.Parent.Group != centerType.Group || rule.Parent.Kind != centerType.Kind {
			continue
		}
		childType, ok := catalog.FindByGroupKind(rule.Child.Group, rule.Child.Kind)
		if !ok || !current(ctx, r.projection, &childType) {
			continue
		}
		if rule.Resolver == "labelJoin" {
			matched, ambiguous, parentErr := r.labelJoinChildren(ctx, tenantID, centerType, center, &childType, rule.LabelJoin.Labels)
			if parentErr != nil {
				return nil, nil, parentErr
			}
			for _, child := range matched {
				key := childType.Key() + "/" + child.UID
				if seenChildren[key] {
					continue
				}
				seenChildren[key] = true
				result = append(result, relationshipNode(&childType, child, false))
			}
			if ambiguous {
				problems = append(problems, ambiguityProblem(&rule))
			}
			continue
		}
		children, listErr := r.projection.authorizedObjects(ctx, tenantID, childType.Key())
		if listErr != nil {
			return nil, nil, listErr
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
	return result, problems, nil
}

func (r *CompositeRelationshipResolver) labelJoinChildren(ctx context.Context, tenantID string, parentType *Type, center projectedObject, childType *Type, mappings []config.LabelJoinMapping) ([]projectedObject, bool, error) {
	if childType.Scope == ScopeCluster && parentType.Scope == ScopeNamespaced {
		return nil, false, nil
	}
	childLabels, ok := joinedLabels(center.Contents, mappings, true)
	if !ok {
		return nil, false, nil
	}
	childNamespace := ""
	if parentType.Scope == ScopeNamespaced {
		childNamespace = center.Namespace
	}
	result, err := r.projection.authorizedObjectsByLabels(ctx, tenantID, childType.Key(), childNamespace, childLabels, 0)
	if err != nil || len(result) == 0 {
		return result, false, err
	}
	parentNamespace := ""
	if parentType.Scope == ScopeNamespaced {
		parentNamespace = center.Namespace
	}
	parentLabels := make([]labelMatch, 0, len(mappings))
	for _, mapping := range mappings {
		for _, childLabel := range childLabels {
			if childLabel.key == mapping.ChildLabel {
				parentLabels = append(parentLabels, labelMatch{key: mapping.ParentLabel, value: childLabel.value})
				break
			}
		}
	}
	parents, err := r.projection.authorizedObjectsByLabels(ctx, tenantID, parentType.Key(), parentNamespace, parentLabels, 2)
	if err != nil {
		return nil, false, err
	}
	if len(parents) > 1 {
		return nil, true, nil
	}
	return result, false, nil
}

func joinedLabels(contents []byte, mappings []config.LabelJoinMapping, sourceParent bool) ([]labelMatch, bool) {
	var object struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if json.Unmarshal(contents, &object) != nil {
		return nil, false
	}
	result := make([]labelMatch, 0, len(mappings))
	for _, mapping := range mappings {
		sourceKey, targetKey := mapping.ChildLabel, mapping.ParentLabel
		if sourceParent {
			sourceKey, targetKey = mapping.ParentLabel, mapping.ChildLabel
		}
		value, present := object.Metadata.Labels[sourceKey]
		if !present {
			return nil, false
		}
		result = append(result, labelMatch{key: targetKey, value: value})
	}
	return result, true
}

func ambiguityProblem(rule *config.RelationshipConfig) RelationshipProblem {
	return RelationshipProblem{Parent: rule.Parent, Child: rule.Child, Resolver: rule.Resolver, Reason: "multiple authorized parents match this relationship"}
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

func (r *CompositeRelationshipResolver) completeness(ctx context.Context, types []Type, limited bool) RelationshipCompleteness {
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
