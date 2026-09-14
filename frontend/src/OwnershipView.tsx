import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward'
import {
  Alert,
  Box,
  Button,
  IconButton,
  Tooltip,
  CircularProgress,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { useEffect, useState } from 'react'
import type { ResourceType, ResourceSummary } from './ResourceWorkspace'
import { technicalText } from './theme'

export type RelationshipNode = ResourceSummary & {
  type: ResourceType
  controller?: boolean
}
type Completeness = {
  complete: boolean
  configurationLimited: boolean
  unavailableScopes?: {
    group: string
    kind: string
    observation: { state: string; reason?: string }
  }[]
}
type Neighborhood = {
  center: RelationshipNode
  parents: RelationshipNode[]
  children: RelationshipNode[]
  upward: Completeness
  downward: Completeness
}

type Props = {
  tenantID: string
  center: RelationshipNode
  revision: number
  onInspect: (node: RelationshipNode) => void
}

export default function OwnershipView(props: Props) {
  const [expansion, setExpansion] = useState({ all: false, generation: 0 })
  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        <Button
          size="small"
          variant="outlined"
          onClick={() =>
            setExpansion((current) => ({
              all: true,
              generation: current.generation + 1,
            }))
          }
        >
          Expand entire hierarchy
        </Button>
        <Button
          size="small"
          onClick={() =>
            setExpansion((current) => ({
              all: false,
              generation: current.generation + 1,
            }))
          }
        >
          Collapse branches
        </Button>
      </Stack>
      <NeighborhoodView
        key={`${props.tenantID}/${nodeIdentity(props.center)}/${expansion.generation}`}
        {...props}
        expandAll={expansion.all}
        ancestry={new Set()}
        root
        direction="center"
      />
    </Stack>
  )
}

function NeighborhoodView({
  tenantID,
  center,
  revision,
  onInspect,
  ancestry,
  root = false,
  direction,
  expandAll,
}: Props & {
  expandAll: boolean
  ancestry: Set<string>
  root?: boolean
  direction: 'center' | 'parent' | 'child'
}) {
  const [expanded, setExpanded] = useState(root || expandAll)
  const [neighborhood, setNeighborhood] = useState<Neighborhood | null>(null)
  const [problem, setProblem] = useState<string | null>(null)
  const identity = nodeIdentity(center)
  const cycle = ancestry.has(identity)
  useEffect(() => {
    if (!expanded || cycle) return
    let active = true
    setNeighborhood(null)
    setProblem(null)
    const source = new EventSource(relationshipStreamPath(tenantID, center))
    source.addEventListener('snapshot', (event) => {
      if (!active) return
      try {
        setNeighborhood(JSON.parse((event as MessageEvent<string>).data))
        setProblem(null)
      } catch {
        setProblem('Ownership data could not be read.')
        source.close()
      }
    })
    source.addEventListener('observation', () => {
      if (!active) return
      setProblem('This ownership branch is not current.')
      source.close()
    })
    source.addEventListener('revoked', () => {
      if (!active) return
      setNeighborhood(null)
      setProblem('This resource is no longer visible in the active tenant.')
      source.close()
    })
    source.onerror = () => {
      if (active) setProblem('Ownership updates are reconnecting…')
    }
    return () => {
      active = false
      source.close()
    }
  }, [tenantID, identity, revision, expanded, cycle])

  const nextAncestry = new Set(ancestry).add(identity)
  const parents = neighborhood?.parents ?? []
  const children = neighborhood?.children ?? []
  return (
    <Stack spacing={0.5} alignItems="center" width="100%" minWidth={0}>
      {expanded && neighborhood && direction !== 'child' && (
        <CompletenessNotice
          label={`Owners of ${center.name}`}
          value={neighborhood.upward}
          empty={parents.length === 0}
        />
      )}
      {expanded && neighborhood && direction !== 'child' && (
        <RelationshipList
          anchor={center}
          expandAll={expandAll}
          nodes={parents}
          direction="parent"
          tenantID={tenantID}
          revision={revision}
          ancestry={nextAncestry}
          onInspect={onInspect}
        />
      )}
      <Paper
        variant="outlined"
        sx={{
          px: 1,
          py: 0.5,
          width: 260,
          maxWidth: '100%',
          borderColor: root ? 'primary.main' : 'divider',
          borderWidth: root ? 2 : 1,
        }}
        role="group"
        aria-label={`${root ? 'Selected resource' : 'Resource'} ${center.name}`}
      >
        <Stack direction="row" gap={0.5} alignItems="center">
          <Box flex={1} minWidth={0}>
            <Tooltip title={center.name}>
              {root ? (
                <Typography fontWeight={600} noWrap color="primary">
                  {center.name}
                </Typography>
              ) : (
                <Button
                  size="small"
                  aria-label={`Inspect ${center.name}`}
                  onClick={() => onInspect(center)}
                  sx={{
                    p: 0,
                    minWidth: 0,
                    maxWidth: '100%',
                    justifyContent: 'flex-start',
                  }}
                >
                  <Box
                    component="span"
                    sx={{
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {center.name}
                  </Box>
                </Button>
              )}
            </Tooltip>
            <Tooltip
              title={`${center.type.group || 'core'}/${center.type.version} · ${center.namespace || 'Cluster scoped'} · ${center.uid}`}
            >
              <Typography
                variant="caption"
                color="text.secondary"
                noWrap
                display="block"
                tabIndex={0}
                sx={technicalText}
              >
                {center.type.kind}
                {root ? ' · Selected' : ''}
              </Typography>
            </Tooltip>
          </Box>
          {!root && (
            <Tooltip
              title={
                cycle
                  ? 'Cycle — already in this chain'
                  : expanded
                    ? 'Collapse branch'
                    : direction === 'parent'
                      ? 'Show owners'
                      : 'Show owned resources'
              }
            >
              <span>
                <IconButton
                  size="small"
                  disabled={cycle}
                  aria-expanded={expanded}
                  aria-label={`${expanded ? 'Hide' : 'Show'} ${direction === 'parent' ? 'owners of' : 'resources owned by'} ${center.name}`}
                  onClick={() => setExpanded((value) => !value)}
                >
                  <ExpandMoreIcon
                    sx={{
                      transform:
                        expanded || direction === 'parent'
                          ? 'rotate(180deg)'
                          : undefined,
                    }}
                  />
                </IconButton>
              </span>
            </Tooltip>
          )}
        </Stack>
      </Paper>
      {expanded && !cycle && !neighborhood && !problem && (
        <CircularProgress size={24} aria-label="Loading ownership" />
      )}
      {problem && (
        <Alert severity="warning" sx={{ width: 'min(100%, 620px)' }}>
          {problem}
        </Alert>
      )}
      {expanded && neighborhood && direction !== 'parent' && (
        <CompletenessNotice
          label={`Resources owned by ${center.name}`}
          value={neighborhood.downward}
          empty={children.length === 0}
        />
      )}
      {expanded && neighborhood && direction !== 'parent' && (
        <RelationshipList
          anchor={center}
          expandAll={expandAll}
          nodes={children}
          direction="child"
          tenantID={tenantID}
          revision={revision}
          ancestry={nextAncestry}
          onInspect={onInspect}
        />
      )}
    </Stack>
  )
}

function RelationshipList({
  nodes,
  anchor,
  ...props
}: { nodes: RelationshipNode[]; anchor: RelationshipNode } & Omit<
  Parameters<typeof NeighborhoodView>[0],
  'center' | 'root'
>) {
  return nodes.length ? (
    <Box
      role="group"
      aria-label={`${props.direction === 'parent' ? 'Owners of' : 'Resources owned by'} ${anchor.name}`}
      sx={{ width: '100%', minWidth: 0, overflowX: 'auto', pb: 1 }}
    >
      <Box
        sx={{
          display: 'flex',
          gap: 1,
          width: 'max-content',
          minWidth: '100%',
          justifyContent: 'center',
          alignItems: props.direction === 'parent' ? 'flex-end' : 'flex-start',
        }}
      >
        {nodes.map((node) => {
          const edge = (
            <OwnershipEdge
              owner={props.direction === 'parent' ? node : anchor}
              child={props.direction === 'parent' ? anchor : node}
              controller={node.controller}
            />
          )
          return (
            <Stack
              key={nodeIdentity(node)}
              spacing={0.5}
              alignItems="center"
              sx={{
                flex: '0 0 260px',
                width: 260,
                minWidth: 0,
              }}
            >
              {props.direction === 'child' && edge}
              <NeighborhoodView {...props} center={node} />
              {props.direction === 'parent' && edge}
            </Stack>
          )
        })}
      </Box>
    </Box>
  ) : null
}

function OwnershipEdge({
  owner,
  child,
  controller,
}: {
  owner: RelationshipNode
  child: RelationshipNode
  controller?: boolean
}) {
  return (
    <Stack
      role="group"
      aria-label={`${owner.name} owns ${child.name}`}
      direction="row"
      alignItems="center"
      justifyContent="center"
      spacing={0.5}
      sx={{ maxWidth: '100%', height: 28 }}
    >
      <ArrowDownwardIcon color="primary" sx={{ fontSize: 16 }} />
      <Typography
        variant="caption"
        textAlign="center"
        sx={{ overflowWrap: 'anywhere' }}
      >
        {controller ? 'owns · controller' : 'owns'}
      </Typography>
    </Stack>
  )
}

function CompletenessNotice({
  label,
  value,
  empty,
}: {
  label: string
  value: Completeness
  empty: boolean
}) {
  if (!empty && value.complete && !value.configurationLimited) return null
  const unavailable = value.unavailableScopes
    ?.map(
      (scope) =>
        `${scope.group || 'core'}/${scope.kind}: ${scope.observation.state}`,
    )
    .join(', ')
  return (
    <Typography
      variant="caption"
      color={value.complete ? 'text.secondary' : 'warning.main'}
    >
      {label}:{' '}
      {value.complete
        ? empty
          ? 'none observed'
          : 'current'
        : `incomplete (${unavailable || 'observation unavailable'})`}
      {value.configurationLimited
        ? ' · limited to configured ownership types'
        : ''}
    </Typography>
  )
}

function nodeIdentity(node: RelationshipNode) {
  return `${node.type.group}/${node.type.version}/${node.type.resource}/${node.uid}`
}

function relationshipStreamPath(tenantID: string, node: RelationshipNode) {
  const group = node.type.group || '_core'
  return `/api/v1/tenants/${encodeURIComponent(tenantID)}/relationship-streams/${encodeURIComponent(group)}/${encodeURIComponent(node.type.version)}/${encodeURIComponent(node.type.resource)}/${encodeURIComponent(node.uid)}`
}
