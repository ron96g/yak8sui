import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ResourceSummary, ResourceType } from './ResourceWorkspace'

export type RelationshipNode = ResourceSummary & {
  type: ResourceType
  controller?: boolean
}

export type Completeness = {
  complete: boolean
  configurationLimited: boolean
  unavailableScopes?: {
    group: string
    kind: string
    observation: { state: string; reason?: string }
  }[]
  problems?: {
    parent: { group: string; kind: string }
    child: { group: string; kind: string }
    resolver: string
    reason: string
  }[]
}

export type Neighborhood = {
  center: RelationshipNode
  parents: RelationshipNode[]
  children: RelationshipNode[]
  upward: Completeness
  downward: Completeness
}

export type HierarchyDirection = 'center' | 'parent' | 'child'

export type HierarchyOccurrence = {
  key: string
  node: RelationshipNode
  direction: HierarchyDirection
  depth: number
  predecessor?: RelationshipNode
  controller?: boolean
  cycle: boolean
  expanded: boolean
  neighborhood: Neighborhood | null
  problem: string | null
  children: HierarchyOccurrence[]
}

export type OwnershipHierarchy = {
  root: HierarchyOccurrence
  rows: HierarchyOccurrence[]
  expandAll: boolean
  toggle: (occurrenceKey: string) => void
  expandEntireHierarchy: () => void
  collapseBranches: () => void
}

export function useOwnershipHierarchy(
  tenantID: string,
  center: RelationshipNode,
  revision: number,
): OwnershipHierarchy {
  const [expandedKeys, setExpandedKeys] = useState<Set<string>>(new Set())
  const [collapsedKeys, setCollapsedKeys] = useState<Set<string>>(new Set())
  const [expandAll, setExpandAll] = useState(false)
  const [snapshots, setSnapshots] = useState<Map<string, Neighborhood>>(
    new Map(),
  )
  const [problems, setProblems] = useState<Map<string, string>>(new Map())
  const sources = useRef(new Map<string, EventSource>())

  const hierarchy = useMemo(
    () =>
      buildHierarchy(
        center,
        snapshots,
        problems,
        expandedKeys,
        collapsedKeys,
        expandAll,
      ),
    [center, snapshots, problems, expandedKeys, collapsedKeys, expandAll],
  )
  const requiredNodes = useMemo(() => {
    const nodes = new Map<string, RelationshipNode>()
    for (const occurrence of hierarchy.rows) {
      if (occurrence.expanded && !occurrence.cycle)
        nodes.set(nodeIdentity(occurrence.node), occurrence.node)
    }
    return nodes
  }, [hierarchy.rows])
  const visibleOccurrenceKeys = hierarchy.rows.map(({ key }) => key)
  const visibleOccurrenceKeyList = visibleOccurrenceKeys.join('|')
  const visibleNodeIdentities = new Set(
    hierarchy.rows.map(({ node }) => nodeIdentity(node)),
  )
  const visibleNodeIdentityList = [...visibleNodeIdentities].sort().join('|')
  useEffect(() => {
    const activeSources = sources.current
    const nodes = requiredNodes
    for (const [identity, source] of activeSources) {
      if (!nodes.has(identity)) {
        source.close()
        activeSources.delete(identity)
      }
    }
    for (const [identity, node] of nodes) {
      if (activeSources.has(identity)) continue
      const source = new EventSource(relationshipStreamPath(tenantID, node))
      activeSources.set(identity, source)
      const isCurrent = () => activeSources.get(identity) === source
      const closeTerminal = () => {
        source.close()
        if (isCurrent()) activeSources.delete(identity)
      }
      source.addEventListener('snapshot', (event) => {
        if (!isCurrent()) return
        try {
          const snapshot = JSON.parse(
            (event as MessageEvent<string>).data,
          ) as Neighborhood
          snapshot.parents ??= []
          snapshot.children ??= []
          setSnapshots((current) => new Map(current).set(identity, snapshot))
          setProblems((current) => {
            const next = new Map(current)
            next.delete(identity)
            return next
          })
        } catch {
          setProblems((current) =>
            new Map(current).set(identity, 'Ownership data could not be read.'),
          )
          closeTerminal()
        }
      })
      source.addEventListener('observation', () => {
        if (!isCurrent()) return
        setProblems((current) =>
          new Map(current).set(
            identity,
            'This ownership branch is not current.',
          ),
        )
        closeTerminal()
      })
      source.addEventListener('revoked', () => {
        if (!isCurrent()) return
        setSnapshots((current) => {
          const next = new Map(current)
          next.delete(identity)
          return next
        })
        setProblems((current) =>
          new Map(current).set(
            identity,
            'This resource is no longer visible in the active tenant.',
          ),
        )
        closeTerminal()
      })
      source.onerror = () => {
        if (isCurrent()) {
          setProblems((current) =>
            new Map(current).set(
              identity,
              'Ownership updates are reconnecting…',
            ),
          )
        }
      }
    }
  }, [tenantID, revision, requiredNodes])

  useEffect(() => {
    const occurrenceKeys = new Set(visibleOccurrenceKeys)
    setExpandedKeys((current) => retainSetKeys(current, occurrenceKeys))
    setCollapsedKeys((current) => retainSetKeys(current, occurrenceKeys))
    setSnapshots((current) => retainMapKeys(current, visibleNodeIdentities))
    setProblems((current) => retainMapKeys(current, visibleNodeIdentities))
  }, [visibleOccurrenceKeyList, visibleNodeIdentityList])

  useEffect(
    () => () => {
      for (const source of sources.current.values()) source.close()
      sources.current.clear()
    },
    [],
  )

  const toggle = useCallback(
    (key: string) => {
      if (expandAll) {
        setCollapsedKeys((current) => {
          const next = new Set(current)
          if (next.has(key)) next.delete(key)
          else next.add(key)
          return next
        })
      } else {
        setExpandedKeys((current) => {
          const next = new Set(current)
          if (next.has(key)) next.delete(key)
          else next.add(key)
          return next
        })
      }
    },
    [expandAll],
  )

  return {
    ...hierarchy,
    expandAll,
    toggle,
    expandEntireHierarchy: () => {
      setExpandAll(true)
      setCollapsedKeys(new Set())
    },
    collapseBranches: () => {
      setExpandAll(false)
      setExpandedKeys(new Set())
      setCollapsedKeys(new Set())
    },
  }
}

function retainSetKeys(current: Set<string>, retained: Set<string>) {
  if ([...current].every((key) => retained.has(key))) return current
  return new Set([...current].filter((key) => retained.has(key)))
}

function retainMapKeys<T>(
  current: Map<string, T>,
  retained: Set<string>,
): Map<string, T> {
  if ([...current.keys()].every((key) => retained.has(key))) return current
  return new Map([...current].filter(([key]) => retained.has(key)))
}

function buildHierarchy(
  center: RelationshipNode,
  snapshots: Map<string, Neighborhood>,
  problems: Map<string, string>,
  expandedKeys: Set<string>,
  collapsedKeys: Set<string>,
  expandAll: boolean,
) {
  const rows: HierarchyOccurrence[] = []
  const visit = (
    node: RelationshipNode,
    direction: HierarchyDirection,
    key: string,
    ancestry: Set<string>,
    depth: number,
    predecessor?: RelationshipNode,
  ): HierarchyOccurrence => {
    const identity = nodeIdentity(node)
    const cycle = ancestry.has(identity)
    const expanded =
      direction === 'center' ||
      (!cycle && (expandAll ? !collapsedKeys.has(key) : expandedKeys.has(key)))
    const neighborhood = snapshots.get(identity) ?? null
    const occurrence: HierarchyOccurrence = {
      key,
      node,
      direction,
      depth,
      predecessor,
      controller: node.controller,
      cycle,
      expanded,
      neighborhood,
      problem: problems.get(identity) ?? null,
      children: [],
    }
    rows.push(occurrence)
    if (!expanded || cycle || !neighborhood) return occurrence
    const nextAncestry = new Set(ancestry).add(identity)
    const related =
      direction === 'parent'
        ? neighborhood.parents
        : direction === 'child'
          ? neighborhood.children
          : [...neighborhood.parents, ...neighborhood.children]
    occurrence.children = related.map((relatedNode, index) => {
      const nextDirection =
        direction === 'center'
          ? index < neighborhood.parents.length
            ? 'parent'
            : 'child'
          : direction
      return visit(
        relatedNode,
        nextDirection,
        `${key}/${nextDirection}:${nodeIdentity(relatedNode)}`,
        nextAncestry,
        depth + 1,
        node,
      )
    })
    return occurrence
  }
  const root = visit(
    center,
    'center',
    `center:${nodeIdentity(center)}`,
    new Set(),
    0,
  )
  return { root, rows }
}

export function nodeIdentity(node: RelationshipNode) {
  return `${node.type.group}/${node.type.version}/${node.type.resource}/${node.uid}`
}

function relationshipStreamPath(tenantID: string, node: RelationshipNode) {
  const group = node.type.group || '_core'
  return `/api/v1/tenants/${encodeURIComponent(tenantID)}/relationship-streams/${encodeURIComponent(group)}/${encodeURIComponent(node.type.version)}/${encodeURIComponent(node.type.resource)}/${encodeURIComponent(node.uid)}`
}
