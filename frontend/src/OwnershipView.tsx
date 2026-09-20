import ArrowDownwardIcon from '@mui/icons-material/ArrowDownward'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
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
import { useState } from 'react'
import { technicalText } from './theme'
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
  onInspect: (node: RelationshipNode) => void
}

export default function OwnershipView(props: Props) {
  const contextKey = `${props.tenantID}/${nodeIdentity(props.center)}/${props.revision}`
  return <OwnershipPresentation key={contextKey} {...props} />
}

function OwnershipPresentation(props: Props) {
  const [presentation, setPresentation] = useState<'graph' | 'table'>('graph')
  const hierarchy = useOwnershipHierarchy(
    props.tenantID,
    props.center,
    props.revision,
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
      {presentation === 'graph' ? (
        <GraphOccurrence
          occurrence={hierarchy.root}
          onInspect={props.onInspect}
          onToggle={hierarchy.toggle}
          root
        />
      ) : (
        <OwnershipTable
          rows={hierarchy.rows}
          onInspect={props.onInspect}
          onToggle={hierarchy.toggle}
        />
      )}
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
  onInspect: Props['onInspect']
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
  onInspect: Props['onInspect']
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
              <Typography fontWeight={600} noWrap color="primary">
                {node.name}
              </Typography>
            ) : (
              <Button
                size="small"
                aria-label={`Inspect ${node.name}`}
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
          onClick={() => onToggle(occurrence.key)}
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
  onInspect: Props['onInspect']
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
  onInspect: Props['onInspect']
  onToggle: (key: string) => void
}) {
  return (
    <TableContainer sx={{ width: '100%', maxWidth: '100%', overflowX: 'auto' }}>
      <Table
        size="small"
        aria-label="Loaded ownership hierarchy"
        sx={{ minWidth: 980 }}
      >
        <TableHead>
          <TableRow>
            <TableCell>Direction / depth</TableCell>
            <TableCell>Predecessor / relationship</TableCell>
            <TableCell>Name</TableCell>
            <TableCell>Kind</TableCell>
            <TableCell>Namespace</TableCell>
            <TableCell>Health</TableCell>
            <TableCell>Controller</TableCell>
            <TableCell>Branch state</TableCell>
            <TableCell>Actions</TableCell>
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
  onInspect: Props['onInspect']
  onToggle: (key: string) => void
}) {
  const { node, predecessor, direction } = occurrence
  const relationship = !predecessor
    ? 'Selected center'
    : direction === 'parent'
      ? `${node.name} owns ${predecessor.name}`
      : `${predecessor.name} owns ${node.name}`
  return (
    <TableRow
      data-node-identity={nodeIdentity(node)}
      aria-selected={direction === 'center'}
      sx={
        direction === 'center'
          ? {
              '& > *': {
                backgroundColor: 'action.selected',
                borderBlockColor: 'primary.main',
              },
              '& > :first-of-type': {
                borderLeft: 2,
                borderLeftColor: 'primary.main',
              },
              '& > :last-child': {
                borderRight: 2,
                borderRightColor: 'primary.main',
              },
            }
          : undefined
      }
    >
      <TableCell>
        {direction === 'center'
          ? 'Center'
          : direction === 'parent'
            ? 'Owner'
            : 'Owned'}{' '}
        · {occurrence.depth}
      </TableCell>
      <TableCell>
        {predecessor?.name ?? '—'}
        <Typography variant="caption" display="block">
          {relationship}
        </Typography>
      </TableCell>
      <TableCell>
        {node.name}
        <Typography variant="caption" display="block" sx={technicalText}>
          UID: {node.uid}
        </Typography>
      </TableCell>
      <TableCell>
        {node.type.kind}
        <Typography variant="caption" display="block" sx={technicalText}>
          {node.type.group || 'core'}/{node.type.version}/{node.type.resource}
        </Typography>
      </TableCell>
      <TableCell>{node.namespace || 'Cluster scoped'}</TableCell>
      <TableCell>{healthLabel(node)}</TableCell>
      <TableCell>{occurrence.controller ? 'Controller' : '—'}</TableCell>
      <TableCell>{branchState(occurrence)}</TableCell>
      <TableCell>
        <Stack direction="row" alignItems="center">
          {direction !== 'center' && (
            <BranchButton occurrence={occurrence} onToggle={onToggle} />
          )}
          {direction !== 'center' && (
            <Button
              size="small"
              aria-label={`Inspect ${node.name}`}
              onClick={() => onInspect(node)}
            >
              Inspect
            </Button>
          )}
        </Stack>
      </TableCell>
    </TableRow>
  )
}

function branchState(occurrence: HierarchyOccurrence) {
  if (occurrence.cycle) return 'Cycle — path stopped'
  if (occurrence.problem) return occurrence.problem
  if (!occurrence.expanded) {
    if (!occurrence.neighborhood) return 'Collapsed · not loaded'
    const completeness =
      occurrence.direction === 'parent'
        ? occurrence.neighborhood.upward
        : occurrence.neighborhood.downward
    return `Collapsed · loaded · ${completenessState(completeness)}`
  }
  if (!occurrence.neighborhood) return 'Loading'
  if (occurrence.direction === 'center') {
    return `Owners: ${completenessState(occurrence.neighborhood.upward)} · Owned: ${completenessState(occurrence.neighborhood.downward)}`
  }
  return completenessState(
    occurrence.direction === 'parent'
      ? occurrence.neighborhood.upward
      : occurrence.neighborhood.downward,
  )
}

function completenessState(completeness: Completeness) {
  const labels = [completeness.complete ? 'Expanded' : 'Incomplete']
  if (completeness.configurationLimited) labels.push('configuration-limited')
  if (completeness.unavailableScopes?.length) labels.push('unavailable')
  return labels.join(' · ')
}

function healthLabel(node: RelationshipNode) {
  return `${node.health.status}${node.health.stale ? ' (stale)' : ''}`
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
