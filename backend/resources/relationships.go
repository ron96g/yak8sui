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
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Controller *bool  `json:"controller"`
}

type fieldReference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
	Namespace  string `json:"namespace"`
}

type relationshipAdapter interface {
	parents(context.Context, string, *Type, *Type, projectedObject, *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error)
	children(context.Context, string, *Type, projectedObject, *Type, *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error)
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
		adapter := r.adapter(&rule)
		matched, controllers, problem, resolveErr := adapter.parents(ctx, tenantID, &parentType, centerType, center, &rule)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		if problem != nil {
			problems = append(problems, *problem)
		}
		for _, parent := range matched {
			if seenParents[parentType.Key()+"/"+parent.UID] {
				continue
			}
			seenParents[parentType.Key()+"/"+parent.UID] = true
			result = append(result, relationshipNode(&parentType, parent, controllers[parent.UID]))
		}
	}
	return result, problems, nil
}

func (r *CompositeRelationshipResolver) adapter(rule *config.RelationshipConfig) relationshipAdapter {
	switch rule.Resolver {
	case "labelJoin":
		return labelJoinAdapter{resolver: r}
	case "fieldReference":
		return fieldReferenceAdapter{projection: r.projection}
	default:
		return ownerReferenceAdapter{projection: r.projection}
	}
}

type labelJoinAdapter struct {
	resolver *CompositeRelationshipResolver
}

func (a labelJoinAdapter) parents(ctx context.Context, tenantID string, parentType, childType *Type, child projectedObject, rule *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error) {
	parent, problem, err := a.resolver.resolveLabelJoinParent(ctx, tenantID, parentType, childType, child, rule)
	if parent == nil {
		return nil, nil, problem, err
	}
	return []projectedObject{*parent}, nil, problem, err
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
		children, controllers, problem, resolveErr := r.adapter(&rule).children(ctx, tenantID, centerType, center, &childType, &rule)
		if resolveErr != nil {
			return nil, nil, resolveErr
		}
		if problem != nil {
			problems = append(problems, *problem)
		}
		for _, child := range children {
			key := childType.Key() + "/" + child.UID
			if seenChildren[key] || !validOwnershipScope(centerType, center.Namespace, &childType, child.Namespace) {
				continue
			}
			seenChildren[key] = true
			result = append(result, relationshipNode(&childType, child, controllers[child.UID]))
		}
	}
	return result, problems, nil
}

func (a labelJoinAdapter) children(ctx context.Context, tenantID string, parentType *Type, parent projectedObject, childType *Type, rule *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error) {
	children, ambiguous, err := a.resolver.labelJoinChildren(ctx, tenantID, parentType, parent, childType, rule.LabelJoin.Labels)
	if ambiguous {
		problem := ambiguityProblem(rule)
		return nil, nil, &problem, err
	}
	return children, nil, nil, err
}

type ownerReferenceAdapter struct{ projection *Projection }

func (a ownerReferenceAdapter) parents(ctx context.Context, tenantID string, parentType, childType *Type, child projectedObject, _ *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error) {
	refs, err := decodeOwnerReferences(child.Contents)
	if err != nil {
		return nil, nil, nil, err
	}
	var result []projectedObject
	controllers := map[string]bool{}
	for _, ref := range refs {
		if !ownerReferenceMatches(ref, parentType) {
			continue
		}
		parent, getErr := a.projection.authorizedObject(ctx, tenantID, parentType.Key(), ref.UID)
		if errors.Is(getErr, sql.ErrNoRows) {
			continue
		}
		if getErr != nil {
			return nil, nil, nil, getErr
		}
		if validOwnershipScope(parentType, parent.Namespace, childType, child.Namespace) {
			result = append(result, parent)
			controllers[parent.UID] = controllers[parent.UID] || ref.Controller != nil && *ref.Controller
		}
	}
	return result, controllers, nil, nil
}

func (a ownerReferenceAdapter) children(ctx context.Context, tenantID string, parentType *Type, parent projectedObject, childType *Type, _ *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error) {
	children, controllers, err := a.projection.authorizedOwnerReferenceChildren(ctx, tenantID, childType.Key(), apiVersion(parentType), parentType.Kind, parent.UID)
	return children, controllers, nil, err
}

type fieldReferenceAdapter struct{ projection *Projection }

func (a fieldReferenceAdapter) parents(ctx context.Context, tenantID string, parentType, childType *Type, child projectedObject, rule *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error) {
	ref, state := extractFieldReference(child.Contents, rule.FieldReference.Path)
	if state == referenceMissing {
		return nil, nil, nil, nil
	}
	if state == referenceMalformed || !fieldReferenceTypeMatches(&ref, parentType) {
		problem := malformedReferenceProblem(rule)
		return nil, nil, &problem, nil
	}
	if (parentType.Scope == ScopeNamespaced && ref.Namespace == "") || (parentType.Scope == ScopeCluster && ref.Namespace != "") {
		problem := malformedReferenceProblem(rule)
		return nil, nil, &problem, nil
	}
	parent, err := a.projection.authorizedObject(ctx, tenantID, parentType.Key(), ref.UID)
	if errors.Is(err, sql.ErrNoRows) {
		namespace := ""
		if parentType.Scope == ScopeNamespaced {
			namespace = ref.Namespace
		}
		_, nameErr := a.projection.authorizedObjectByName(ctx, tenantID, parentType.Key(), namespace, ref.Name)
		if nameErr == nil {
			problem := malformedReferenceProblem(rule)
			return nil, nil, &problem, nil
		}
		if errors.Is(nameErr, sql.ErrNoRows) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, nameErr
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if parent.Name != ref.Name || !validFieldReferenceScope(&ref, parentType, parent, childType, child) {
		problem := malformedReferenceProblem(rule)
		return nil, nil, &problem, nil
	}
	return []projectedObject{parent}, nil, nil, nil
}

func (a fieldReferenceAdapter) children(ctx context.Context, tenantID string, parentType *Type, parent projectedObject, childType *Type, rule *config.RelationshipConfig) ([]projectedObject, map[string]bool, *RelationshipProblem, error) {
	if childType.Scope == ScopeCluster && parentType.Scope == ScopeNamespaced {
		return nil, nil, nil, nil
	}
	ref := fieldReference{APIVersion: apiVersion(parentType), Kind: parentType.Kind, Name: parent.Name, UID: parent.UID}
	namespace := ""
	if parentType.Scope == ScopeNamespaced {
		ref.Namespace = parent.Namespace
		namespace = parent.Namespace
	}
	children, malformed, err := a.projection.authorizedFieldReferenceChildren(ctx, tenantID, childType.Key(), namespace, rule.FieldReference.Path, &ref)
	if malformed {
		problem := malformedReferenceProblem(rule)
		return children, nil, &problem, err
	}
	return children, nil, nil, err
}

type referenceState uint8

const (
	referenceMissing referenceState = iota
	referenceValid
	referenceMalformed
)

func extractFieldReference(contents []byte, path string) (fieldReference, referenceState) {
	var value any
	if json.Unmarshal(contents, &value) != nil {
		return fieldReference{}, referenceMalformed
	}
	current := value
	for _, field := range strings.Split(strings.TrimPrefix(path, "$."), ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return fieldReference{}, referenceMalformed
		}
		current, ok = object[field]
		if !ok {
			return fieldReference{}, referenceMissing
		}
	}
	encoded, err := json.Marshal(current)
	if err != nil {
		return fieldReference{}, referenceMalformed
	}
	var ref fieldReference
	if json.Unmarshal(encoded, &ref) != nil || ref.APIVersion == "" || ref.Kind == "" || ref.Name == "" || ref.UID == "" {
		return fieldReference{}, referenceMalformed
	}
	return ref, referenceValid
}

func fieldReferenceTypeMatches(ref *fieldReference, parentType *Type) bool {
	return ref.APIVersion == apiVersion(parentType) && ref.Kind == parentType.Kind
}

func validFieldReferenceScope(ref *fieldReference, parentType *Type, parent projectedObject, childType *Type, child projectedObject) bool {
	if !validOwnershipScope(parentType, parent.Namespace, childType, child.Namespace) {
		return false
	}
	if parentType.Scope == ScopeNamespaced {
		return ref.Namespace != "" && ref.Namespace == parent.Namespace
	}
	return ref.Namespace == ""
}

func apiVersion(resourceType *Type) string {
	if resourceType.Group == "" {
		return resourceType.Version
	}
	return resourceType.Group + "/" + resourceType.Version
}

func malformedReferenceProblem(rule *config.RelationshipConfig) RelationshipProblem {
	return RelationshipProblem{Parent: rule.Parent, Child: rule.Child, Resolver: rule.Resolver, Reason: "a present field reference is malformed or does not identify the configured parent"}
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
