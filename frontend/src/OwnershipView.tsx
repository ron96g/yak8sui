import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import HelpOutlineIcon from '@mui/icons-material/HelpOutline'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material'
import { lazy, Suspense, useState } from 'react'
import { technicalText } from './theme'
import { useResourcePreview } from './useResourcePreview'
import {
  type Completeness,
  type HierarchyOccurrence,
  type RelationshipNode,
  nodeIdentity,
  useOwnershipHierarchy,
} from './useOwnershipHierarchy'

export type { RelationshipNode } from './useOwnershipHierarchy'

type Props = {
  tenantID: string
  center: RelationshipNode
  revision: number
  onExplore: (node: RelationshipNode) => void
}

const YamlViewer = lazy(() => import('./YamlViewer'))

export default function OwnershipView(props: Props) {
  const contextKey = `${props.tenantID}/${nodeIdentity(props.center)}/${props.revision}`
  return (
    <OwnershipPresentation
      key={contextKey}
      {...props}
      initialPreview={props.center}
    />
  )
}

function OwnershipPresentation(
  props: Props & { initialPreview: RelationshipNode },
) {
  const [presentation, setPresentation] = useState<'graph' | 'table'>('table')
  const [preview, setPreview] = useState(props.initialPreview)
  const hierarchy = useOwnershipHierarchy(
    props.tenantID,
    props.center,
    props.revision,
  )
  const previewState = useResourcePreview(
    props.tenantID,
    nodeIdentity(props.center),
    preview,
  )
  return (
    <Stack spacing={2} minWidth={0}>
      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
        <ToggleButtonGroup
          size="small"
          exclusive
          value={presentation}
          onChange={(_, value: 'graph' | 'table' | null) => {
            if (value) setPresentation(value)
          }}
          aria-label="Ownership presentation"
        >
          <ToggleButton value="graph">Graph</ToggleButton>
          <ToggleButton value="table">Table</ToggleButton>
        </ToggleButtonGroup>
        <Button
          size="small"
          variant="outlined"
          onClick={hierarchy.expandEntireHierarchy}
        >
          Expand entire hierarchy
        </Button>
        <Button size="small" onClick={hierarchy.collapseBranches}>
          Collapse branches
        </Button>
      </Stack>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {
            xs: 'minmax(0, 1fr)',
            lg: 'minmax(0, 1.5fr) minmax(360px, .75fr)',
          },
          gap: 2,
          minWidth: 0,
        }}
      >
        <Box minWidth={0}>
          {presentation === 'graph' ? (
            <GraphOccurrence
              occurrence={hierarchy.root}
              onInspect={setPreview}
              onToggle={hierarchy.toggle}
              root
            />
          ) : (
            <OwnershipTable
              rows={hierarchy.rows}
              onInspect={setPreview}
              onToggle={hierarchy.toggle}
            />
          )}
        </Box>
        <Paper
          component="section"
          variant="outlined"
          aria-label="Ownership YAML preview"
          sx={{ p: 2, minWidth: 0 }}
        >
          <Stack
            direction="row"
            justifyContent="space-between"
            alignItems="center"
            gap={1}
            mb={1}
          >
            <Box minWidth={0}>
              <Typography variant="subtitle1" noWrap>
                {preview.name}
              </Typography>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={technicalText}
              >
                UID: {preview.uid}
              </Typography>
            </Box>
            <Button
              size="small"
              variant="outlined"
              onClick={() => props.onExplore(preview)}
            >
              Explore from this resource
            </Button>
          </Stack>
          {previewState.status === 'current' ? (
            <Box
              sx={{
                height: { xs: 360, md: 480, lg: '60vh' },
                maxHeight: 700,
                border: 1,
                borderColor: 'divider',
                overflow: 'hidden',
              }}
            >
              <Suspense
                fallback={
                  <CircularProgress aria-label="Loading preview YAML" />
                }
              >
                <YamlViewer value={previewState.detail.yaml} />
              </Suspense>
            </Box>
          ) : previewState.status === 'loading' ? (
            <Alert severity="info" role="status">
              Loading YAML preview…
            </Alert>
          ) : previewState.status === 'reconnecting' ? (
            <Alert severity="warning" role="status">
              Preview stream is reconnecting…
            </Alert>
          ) : 'message' in previewState ? (
            <Alert
              severity={previewState.status === 'revoked' ? 'error' : 'warning'}
              role="status"
            >
              {previewState.message}
            </Alert>
          ) : null}
        </Paper>
      </Box>
    </Stack>
  )
}

function GraphOccurrence({
  occurrence,
  onInspect,
  onToggle,
  root = false,
}: {
  occurrence: HierarchyOccurrence
  onInspect: (node: RelationshipNode) => void
  onToggle: (key: string) => void
  root?: boolean
}) {
  const { node, neighborhood, direction } = occurrence
  const parents = occurrence.children.filter(
    (child) => child.direction === 'parent',
  )
  const children = occurrence.children.filter(
    (child) => child.direction === 'child',
  )
  return (
    <Stack spacing={0.5} alignItems="center" width="100%" minWidth={0}>
      {occurrence.expanded && neighborhood && direction !== 'child' && (
        <CompletenessNotice
          label={`Owners of ${node.name}`}
          value={neighborhood.upward}
          empty={!neighborhood.parents.length}
        />
      )}
      {parents.length > 0 && (
        <GraphList
          nodes={parents}
          anchor={node}
          onInspect={onInspect}
          onToggle={onToggle}
        />
      )}
      <ResourceCard
        occurrence={occurrence}
        root={root}
        onInspect={onInspect}
        onToggle={onToggle}
      />
      {occurrence.expanded &&
        !occurrence.cycle &&
        !neighborhood &&
        !occurrence.problem && (
          <CircularProgress size={24} aria-label="Loading ownership" />
        )}
      {occurrence.problem && (
        <Alert severity="warning" sx={{ width: 'min(100%, 620px)' }}>
          {occurrence.problem}
        </Alert>
      )}
      {occurrence.expanded && neighborhood && direction !== 'parent' && (
        <CompletenessNotice
          label={`Resources owned by ${node.name}`}
          value={neighborhood.downward}
          empty={!neighborhood.children.length}
        />
      )}
      {children.length > 0 && (
        <GraphList
          nodes={children}
          anchor={node}
          onInspect={onInspect}
          onToggle={onToggle}
        />
      )}
    </Stack>
  )
}

function ResourceCard({
  occurrence,
  root,
  onInspect,
  onToggle,
}: {
  occurrence: HierarchyOccurrence
  root: boolean
  onInspect: (node: RelationshipNode) => void
  onToggle: (key: string) => void
}) {
  const { node, cycle, expanded, direction } = occurrence
  return (
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
      aria-label={`${root ? 'Selected resource' : 'Resource'} ${node.name}`}
    >
      <Stack direction="row" gap={0.5} alignItems="center">
        <Box flex={1} minWidth={0}>
          <Tooltip title={node.name}>
            {root ? (
              <Button
                size="small"
                aria-label={`Preview ${node.name}`}
                onClick={() => onInspect(node)}
                sx={{ p: 0, minWidth: 0, color: 'primary.main' }}
              >
                {node.name}
              </Button>
            ) : (
              <Button
                size="small"
                aria-label={`Preview ${node.name}`}
                onClick={() => onInspect(node)}
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
                  {node.name}
                </Box>
              </Button>
            )}
          </Tooltip>
          <Tooltip
            title={`${node.type.group || 'core'}/${node.type.version} · ${node.namespace || 'Cluster scoped'} · ${node.uid}`}
          >
            <Typography
              variant="caption"
              color="text.secondary"
              noWrap
              display="block"
              tabIndex={0}
              sx={technicalText}
            >
              {node.type.kind}
              {root ? ' · Selected' : ''}
            </Typography>
          </Tooltip>
        </Box>
        <HealthIndicator node={node} compact />
        {!root && <BranchButton occurrence={occurrence} onToggle={onToggle} />}
      </Stack>
    </Paper>
  )
}

function BranchButton({
  occurrence,
  onToggle,
}: {
  occurrence: HierarchyOccurrence
  onToggle: (key: string) => void
}) {
  const { node, cycle, expanded, direction } = occurrence
  const relationship =
    direction === 'parent' ? 'owners of' : 'resources owned by'
  return (
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
          aria-label={`${expanded ? 'Hide' : 'Show'} ${relationship} ${node.name}`}
          onKeyDown={(event) => event.stopPropagation()}
          onClick={(event) => {
            event.stopPropagation()
            onToggle(occurrence.key)
          }}
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
  )
}

function GraphList({
  nodes,
  anchor,
  onInspect,
  onToggle,
}: {
  nodes: HierarchyOccurrence[]
  anchor: RelationshipNode
  onInspect: (node: RelationshipNode) => void
  onToggle: (key: string) => void
}) {
  const direction = nodes[0].direction
  return (
    <Box
      role="group"
      aria-label={`${direction === 'parent' ? 'Owners of' : 'Resources owned by'} ${anchor.name}`}
      sx={{ width: '100%', minWidth: 0, overflowX: 'auto', pb: 1 }}
    >
      <Box
        sx={{
          display: 'flex',
          gap: 1,
          width: 'max-content',
          minWidth: '100%',
          justifyContent: 'center',
          alignItems: direction === 'parent' ? 'flex-end' : 'flex-start',
        }}
      >
        {nodes.map((occurrence) => {
          const edge = (
            <OwnershipEdge
              owner={direction === 'parent' ? occurrence.node : anchor}
              child={direction === 'parent' ? anchor : occurrence.node}
              controller={occurrence.controller}
            />
          )
          return (
            <Stack
              key={occurrence.key}
              spacing={0.5}
              alignItems="center"
              sx={{ flex: '0 0 260px', width: 260, minWidth: 0 }}
            >
              {direction === 'child' && edge}
              <GraphOccurrence
                occurrence={occurrence}
                onInspect={onInspect}
                onToggle={onToggle}
              />
              {direction === 'parent' && edge}
            </Stack>
          )
        })}
      </Box>
    </Box>
  )
}

function OwnershipTable({
  rows,
  onInspect,
  onToggle,
}: {
  rows: HierarchyOccurrence[]
  onInspect: (node: RelationshipNode) => void
  onToggle: (key: string) => void
}) {
  return (
    <TableContainer sx={{ width: '100%', maxWidth: '100%', overflowX: 'auto' }}>
      <Table
        size="small"
        aria-label="Loaded ownership hierarchy"
        sx={{ minWidth: 760 }}
      >
        <TableHead>
          <TableRow>
            <TableCell>Resource</TableCell>
            <TableCell>Kind</TableCell>
            <TableCell>Namespace</TableCell>
            <TableCell>Health</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((occurrence) => (
            <TableOccurrence
              key={occurrence.key}
              occurrence={occurrence}
              onInspect={onInspect}
              onToggle={onToggle}
            />
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  )
}

function TableOccurrence({
  occurrence,
  onInspect,
  onToggle,
}: {
  occurrence: HierarchyOccurrence
  onInspect: (node: RelationshipNode) => void
  onToggle: (key: string) => void
}) {
  const { node, direction } = occurrence
  const directionLabel =
    direction === 'center'
      ? 'Selected center'
      : direction === 'parent'
        ? 'Owner'
        : 'Owned resource'
  return (
    <TableRow
      data-node-identity={nodeIdentity(node)}
      data-depth={occurrence.depth}
      aria-selected={direction === 'center'}
      tabIndex={0}
      onClick={() => onInspect(node)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onInspect(node)
        }
      }}
      sx={{
        cursor: 'pointer',
        '& > *': {
          transition: (theme) =>
            theme.transitions.create('background-color', {
              duration: theme.transitions.duration.shortest,
            }),
          ...(direction === 'center'
            ? {
                backgroundColor: 'action.selected',
                borderBlockColor: 'primary.main',
              }
            : {}),
        },
        '&:focus-visible': {
          outline: 2,
          outlineColor: 'primary.main',
          outlineOffset: -2,
        },
        ...(direction === 'center'
          ? {
              '& > :first-of-type': {
                borderLeft: 2,
                borderLeftColor: 'primary.main',
              },
              '& > :last-child': {
                borderRight: 2,
                borderRightColor: 'primary.main',
              },
            }
          : {}),
        '&:hover > *': {
          backgroundColor: 'action.hover',
        },
        '&:hover .ownership-resource-name': {
          color: 'primary.main',
          textDecoration: 'underline',
        },
      }}
    >
      <TableCell sx={{ pl: 1 + occurrence.depth * 3 }}>
        <Stack direction="row" alignItems="center" spacing={0.5}>
          {direction !== 'center' ? (
            <BranchButton occurrence={occurrence} onToggle={onToggle} />
          ) : (
            <Box sx={{ width: 34, flex: '0 0 34px' }} />
          )}
          <Box minWidth={0}>
            <Typography
              className="ownership-resource-name"
              fontWeight={direction === 'center' ? 600 : 400}
            >
              {node.name}
            </Typography>
            <Typography
              variant="caption"
              display="block"
              color="text.secondary"
            >
              {directionLabel}
            </Typography>
            <Typography variant="caption" display="block" sx={technicalText}>
              UID: {node.uid}
            </Typography>
          </Box>
        </Stack>
      </TableCell>
      <TableCell>
        {node.type.kind}
        <Typography variant="caption" display="block" sx={technicalText}>
          {node.type.group || 'core'}/{node.type.version}/{node.type.resource}
        </Typography>
      </TableCell>
      <TableCell>{node.namespace || 'Cluster scoped'}</TableCell>
      <TableCell>
        <HealthIndicator node={node} />
      </TableCell>
    </TableRow>
  )
}

function healthLabel(node: RelationshipNode) {
  return node.health.stale
    ? `Stale (reported ${node.health.status})`
    : node.health.status
}

function HealthIndicator({
  node,
  compact = false,
}: {
  node: RelationshipNode
  compact?: boolean
}) {
  const label = healthLabel(node)
  const isHealthy = node.health.status === 'Healthy' && !node.health.stale
  const isUnhealthy = node.health.status === 'Unhealthy' && !node.health.stale
  const Icon = isHealthy
    ? CheckCircleOutlineIcon
    : isUnhealthy
      ? ErrorOutlineIcon
      : HelpOutlineIcon
  const color = isHealthy
    ? 'success.main'
    : isUnhealthy
      ? 'error.main'
      : 'warning.main'
  return (
    <Tooltip title={label}>
      <Stack
        component="span"
        role="img"
        direction="row"
        spacing={0.5}
        alignItems="center"
        aria-label={`Health: ${label}`}
        data-health-icon={
          isHealthy ? 'check' : isUnhealthy ? 'error' : 'question'
        }
        data-health-state={
          isHealthy ? 'healthy' : isUnhealthy ? 'unhealthy' : 'unknown'
        }
        sx={{ color }}
      >
        <Icon sx={{ fontSize: compact ? 19 : 20, flex: '0 0 auto' }} />
        {!compact && (
          <Typography component="span" variant="body2" color="text.primary">
            {label}
          </Typography>
        )}
      </Stack>
    </Tooltip>
  )
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
  const problems = value.problems
    ?.map(
      (problem) =>
        `${problem.parent.group || 'core'}/${problem.parent.kind} → ${problem.child.group || 'core'}/${problem.child.kind}: ${problem.reason}`,
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
        : `incomplete (${problems || unavailable || 'observation unavailable'})`}
      {value.configurationLimited
        ? ' · limited to configured ownership types'
        : ''}
    </Typography>
  )
}
