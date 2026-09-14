import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import {
  Alert,
  Box,
  Button,
  Chip,
  Collapse,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  MenuItem,
  Typography,
} from '@mui/material'
import { useEffect, useRef, useState } from 'react'
import { space, technicalText } from './theme'

export type ResourceType = {
  group: string
  version: string
  resource: string
  kind: string
  scope: 'Namespaced' | 'Cluster' | 'Unknown'
  available: boolean
  reason?: string
  observation: ObservationStatus
}

export type ObservationStatus = {
  state: 'current' | 'stale' | 'resynchronizing' | 'unavailable'
  reason?: string
}

export type ResourceSummary = {
  uid: string
  name: string
  namespace?: string
  createdAt?: string
  health: ResourceHealth
}

export type ResourceHealth = {
  status: 'Healthy' | 'Unhealthy' | 'Unknown'
  stale: boolean
  reportedStatus?: string
}

export type ResourceQuery = {
  name: string
  namespace: string
  nameMode: 'fuzzy' | 'regex'
  namespaceMode: 'fuzzy' | 'regex'
  jsonPath: string
  jsonComparison: 'exists' | 'equals'
  jsonValue: string
  page: number
}

export type QueryErrors = Partial<
  Record<
    'name' | 'namespace' | 'jsonPath' | 'jsonComparison' | 'jsonValue' | 'page',
    string
  >
>

type Props = {
  resourceTypes: ResourceType[]
  resourceType: ResourceType | null
  resources: ResourceSummary[]
  query: ResourceQuery
  queryErrors: QueryErrors
  total: number
  pageCount: number
  loading: boolean
  connectionState: 'idle' | 'resynchronizing' | 'connected' | 'disconnected'
  observation: ObservationStatus | null
  onRefresh: () => void
  onSelectType: (resourceType: ResourceType) => void
  onOpen: (item: ResourceSummary) => void
  onQuery: (query: ResourceQuery) => void
}

export default function ResourceWorkspace({
  resourceTypes,
  resourceType,
  resources,
  query,
  queryErrors,
  total,
  pageCount,
  loading,
  connectionState,
  observation,
  onRefresh,
  onSelectType,
  onOpen,
  onQuery,
}: Props) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({})
  const [draft, setDraft] = useState(query)
  const queryKey = `${query.name}\u0000${query.namespace}\u0000${query.nameMode}\u0000${query.namespaceMode}\u0000${query.jsonPath}\u0000${query.jsonComparison}\u0000${query.jsonValue}`
  const draftKey = `${draft.name}\u0000${draft.namespace}\u0000${draft.nameMode}\u0000${draft.namespaceMode}\u0000${draft.jsonPath}\u0000${draft.jsonComparison}\u0000${draft.jsonValue}`
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(
    () => () => {
      if (searchTimer.current !== null) clearTimeout(searchTimer.current)
    },
    [],
  )

  function updateDraft(change: Partial<ResourceQuery>) {
    const next = { ...draft, ...change, page: 1 }
    setDraft(next)
    if (searchTimer.current !== null) clearTimeout(searchTimer.current)
    searchTimer.current = setTimeout(() => {
      searchTimer.current = null
      onQuery(next)
    }, 300)
  }
  const groupedTypes = resourceTypes.reduce<Record<string, ResourceType[]>>(
    (groups, item) => {
      const key = `${item.group || 'core'}/${item.version || 'unavailable'}`
      ;(groups[key] ??= []).push(item)
      return groups
    },
    {},
  )
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: { xs: 'column', md: 'row' },
        flex: 1,
      }}
    >
      <Box
        component="nav"
        aria-label="Resource types"
        sx={{
          width: { md: 260 },
          flexShrink: 0,
          borderRight: { md: 1 },
          borderBottom: { xs: 1, md: 0 },
          borderColor: 'divider',
          backgroundColor: 'background.paper',
          p: 2,
        }}
      >
        <Typography variant="overline">Resources</Typography>
        {resourceTypes.length ? (
          <List dense>
            {Object.entries(groupedTypes).map(([groupVersion, types]) => (
              <Box key={groupVersion}>
                <ListItemButton
                  onClick={() =>
                    setCollapsed((current) => ({
                      ...current,
                      [groupVersion]: !current[groupVersion],
                    }))
                  }
                  aria-expanded={!collapsed[groupVersion]}
                >
                  <ListItemText
                    primary={groupVersion}
                    primaryTypographyProps={{ sx: technicalText }}
                  />
                  {!collapsed[groupVersion] ? (
                    <ExpandLessIcon />
                  ) : (
                    <ExpandMoreIcon />
                  )}
                </ListItemButton>
                <Collapse in={!collapsed[groupVersion]}>
                  <List disablePadding>
                    {types.map((item) => (
                      <ListItemButton
                        key={`${item.group}/${item.version}/${item.resource}`}
                        selected={
                          resourceType?.resource === item.resource &&
                          resourceType?.group === item.group
                        }
                        aria-current={
                          resourceType === item ? 'page' : undefined
                        }
                        onClick={() => onSelectType(item)}
                        sx={{ pl: 4, opacity: item.available ? 1 : 0.7 }}
                      >
                        <ListItemText
                          primary={item.kind || item.resource}
                          secondary={
                            item.available ? item.scope : 'Unavailable'
                          }
                        />
                      </ListItemButton>
                    ))}
                  </List>
                </Collapse>
              </Box>
            ))}
          </List>
        ) : (
          <Typography variant="body2" color="text.secondary">
            Select a tenant to load its configured resource type.
          </Typography>
        )}
      </Box>
      <Stack
        component="main"
        spacing={3}
        sx={{ p: { xs: 2, md: 3 }, flex: 1, minWidth: 0 }}
      >
        <Stack
          direction="row"
          justifyContent="space-between"
          alignItems="center"
        >
          <Box>
            <Typography variant="h5" component="h1">
              {resourceType?.kind || 'Resources'}
            </Typography>
            {resourceType && (
              <Typography variant="body2" color="text.secondary">
                {resourceType.scope === 'Namespaced'
                  ? 'Namespaced'
                  : resourceType.scope === 'Cluster'
                    ? 'Cluster-scoped'
                    : 'Scope unavailable'}{' '}
                · Read-only inspection
              </Typography>
            )}
          </Box>
          <Stack alignItems="flex-end" spacing={0.5}>
            <Chip
              size="small"
              role="status"
              variant="outlined"
              label={
                connectionState === 'connected'
                  ? 'Stream: Live'
                  : connectionState === 'resynchronizing'
                    ? 'Stream: Resynchronizing…'
                    : connectionState === 'disconnected'
                      ? 'Stream: Disconnected — reconnecting…'
                      : 'Stream: Not connected'
              }
              sx={{
                color:
                  connectionState === 'connected'
                    ? space.primary
                    : connectionState === 'disconnected'
                      ? space.error
                      : space.muted,
                borderColor: 'currentColor',
              }}
            />
            <Button onClick={onRefresh} disabled={loading}>
              Refresh
            </Button>
          </Stack>
        </Stack>
        {resourceType?.available && observation?.state !== 'current' && (
          <Alert
            severity={
              observation?.state === 'unavailable' ? 'error' : 'warning'
            }
            role="status"
          >
            Observation: {observation?.state ?? 'resynchronizing'}
            {observation?.reason ? ` — ${observation.reason}` : ''}. Displayed
            contents are retained as stale and will not update until an
            authoritative relist completes.
          </Alert>
        )}
        {resourceType && !resourceType.available && (
          <Alert severity="warning">
            {resourceType.reason || 'This configured type is unavailable.'}
          </Alert>
        )}
        <Paper
          variant="outlined"
          component="section"
          aria-labelledby="filters-title"
          sx={{ p: 2 }}
        >
          <Typography id="filters-title" variant="subtitle2" mb={2}>
            Filters
          </Typography>
          <Stack
            direction={{ xs: 'column', lg: 'row' }}
            spacing={2}
            useFlexGap
            flexWrap="wrap"
          >
            <TextField
              label="Name"
              value={draft.name}
              onChange={(event) =>
                updateDraft({
                  name: event.target.value,
                })
              }
              error={draftKey === queryKey && Boolean(queryErrors.name)}
              helperText={draftKey === queryKey ? queryErrors.name : undefined}
              disabled={!resourceType?.available}
              fullWidth
              size="small"
            />
            <TextField
              select
              label="Name mode"
              value={draft.nameMode}
              onChange={(event) =>
                updateDraft({
                  nameMode: event.target.value as 'fuzzy' | 'regex',
                })
              }
              disabled={!resourceType?.available}
              sx={{ minWidth: 120 }}
              size="small"
            >
              <MenuItem value="fuzzy">Fuzzy</MenuItem>
              <MenuItem value="regex">Regex</MenuItem>
            </TextField>
            <TextField
              label="Namespace"
              value={draft.namespace}
              onChange={(event) =>
                updateDraft({
                  namespace: event.target.value,
                })
              }
              error={draftKey === queryKey && Boolean(queryErrors.namespace)}
              disabled={
                !resourceType?.available || resourceType.scope !== 'Namespaced'
              }
              fullWidth
              size="small"
              helperText={
                (draftKey === queryKey ? queryErrors.namespace : undefined) ??
                (resourceType && resourceType.scope === 'Cluster'
                  ? 'Not applicable to cluster-scoped resources'
                  : undefined)
              }
            />
            <TextField
              select
              label="Namespace mode"
              value={draft.namespaceMode}
              onChange={(event) =>
                updateDraft({
                  namespaceMode: event.target.value as 'fuzzy' | 'regex',
                })
              }
              disabled={
                !resourceType?.available || resourceType.scope !== 'Namespaced'
              }
              sx={{ minWidth: 120 }}
              size="small"
            >
              <MenuItem value="fuzzy">Fuzzy</MenuItem>
              <MenuItem value="regex">Regex</MenuItem>
            </TextField>
            <TextField
              label="JSONPath"
              placeholder="$.status.conditions[*].type"
              value={draft.jsonPath}
              onChange={(event) =>
                updateDraft({ jsonPath: event.target.value })
              }
              error={draftKey === queryKey && Boolean(queryErrors.jsonPath)}
              helperText={
                draftKey === queryKey ? queryErrors.jsonPath : undefined
              }
              disabled={!resourceType?.available}
              fullWidth
              size="small"
              sx={{ minWidth: { sm: 280 }, flexBasis: { lg: 320 } }}
            />
            <TextField
              select
              label="JSON comparison"
              value={draft.jsonComparison}
              onChange={(event) =>
                updateDraft({
                  jsonComparison: event.target.value as 'exists' | 'equals',
                })
              }
              error={
                draftKey === queryKey && Boolean(queryErrors.jsonComparison)
              }
              helperText={
                draftKey === queryKey ? queryErrors.jsonComparison : undefined
              }
              disabled={!resourceType?.available}
              sx={{ minWidth: 150 }}
              size="small"
            >
              <MenuItem value="exists">Exists</MenuItem>
              <MenuItem value="equals">Equals</MenuItem>
            </TextField>
            <TextField
              label="JSON value"
              placeholder={'e.g. 3, true, null, or "Ready"'}
              value={draft.jsonValue}
              onChange={(event) =>
                updateDraft({ jsonValue: event.target.value })
              }
              error={draftKey === queryKey && Boolean(queryErrors.jsonValue)}
              helperText={
                draftKey === queryKey
                  ? queryErrors.jsonValue
                  : draft.jsonComparison === 'equals'
                    ? 'Enter a typed JSON scalar; use "" for an empty string.'
                    : 'Not used in Exists mode.'
              }
              disabled={
                !resourceType?.available || draft.jsonComparison !== 'equals'
              }
              fullWidth
              size="small"
              sx={{ minWidth: { sm: 260 }, flexBasis: { lg: 300 } }}
            />
            <Button
              onClick={() =>
                updateDraft({
                  jsonPath: '',
                  jsonComparison: 'exists',
                  jsonValue: '',
                })
              }
              disabled={
                !resourceType?.available ||
                (!draft.jsonPath &&
                  draft.jsonComparison === 'exists' &&
                  !draft.jsonValue)
              }
            >
              Clear JSON filter
            </Button>
          </Stack>
          <Typography
            variant="caption"
            color="text.secondary"
            display="block"
            mt={2}
          >
            Results update as you type after a brief pause. Fuzzy mode matches
            ordered characters without regard to case. Regex uses Go syntax and
            is case-sensitive unless you add (?i). JSONPath supports fields,
            quoted field names, array indexes, and array wildcards. Exists also
            matches a present null; Equals requires a typed JSON scalar.
          </Typography>
        </Paper>
        {loading ? (
          <Alert severity="info" role="status">
            Loading resources…
          </Alert>
        ) : (
          <TableContainer component={Paper} variant="outlined">
            <Table
              size="small"
              aria-label={`${resourceType?.kind || 'Configured'} resources`}
            >
              <TableHead>
                <TableRow>
                  <TableCell>Status (condition)</TableCell>
                  <TableCell>Name</TableCell>
                  <TableCell>Namespace</TableCell>
                  <TableCell>Created at</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {resources.map((item) => (
                  <TableRow
                    hover
                    key={item.uid}
                    onClick={() => onOpen(item)}
                    sx={{ cursor: 'pointer' }}
                  >
                    <TableCell>
                      <HealthChip health={item.health} />
                    </TableCell>
                    <TableCell>
                      <Button
                        size="small"
                        onClick={(event) => {
                          event.stopPropagation()
                          onOpen(item)
                        }}
                        sx={{ ...technicalText, textAlign: 'left' }}
                      >
                        {item.name}
                      </Button>
                    </TableCell>
                    <TableCell sx={technicalText}>
                      {item.namespace || 'Cluster scoped'}
                    </TableCell>
                    <TableCell sx={technicalText}>
                      {item.createdAt ? (
                        <time dateTime={item.createdAt}>{item.createdAt}</time>
                      ) : (
                        'Not reported'
                      )}
                    </TableCell>
                    <TableCell align="right">
                      <Button
                        size="small"
                        aria-label={`Inspect ${item.name}`}
                        onClick={(event) => {
                          event.stopPropagation()
                          onOpen(item)
                        }}
                      >
                        Inspect
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
                {resources.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5}>
                      {query.name || query.namespace || query.jsonPath
                        ? 'No resources match these filters.'
                        : 'No resources are visible for this tenant.'}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
            <Stack
              direction={{ xs: 'column', sm: 'row' }}
              alignItems="center"
              justifyContent="space-between"
              spacing={1}
              sx={{ p: 1.5, borderTop: 1, borderColor: 'divider' }}
            >
              <Typography variant="body2" role="status">
                {total} {total === 1 ? 'resource' : 'resources'} · Page{' '}
                {query.page} of {Math.max(pageCount, 1)}
              </Typography>
              <Stack direction="row" spacing={1}>
                <Button
                  disabled={loading || draftKey !== queryKey || query.page <= 1}
                  onClick={() => onQuery({ ...query, page: query.page - 1 })}
                >
                  Previous
                </Button>
                <Button
                  disabled={
                    loading || draftKey !== queryKey || query.page >= pageCount
                  }
                  onClick={() => onQuery({ ...query, page: query.page + 1 })}
                >
                  Next
                </Button>
              </Stack>
            </Stack>
          </TableContainer>
        )}
      </Stack>
    </Box>
  )
}

function HealthChip({ health }: { health: ResourceHealth }) {
  const label = health.stale
    ? `Stale (reported ${health.status})`
    : health.status
  const color = health.stale
    ? 'warning'
    : health.status === 'Healthy'
      ? 'success'
      : health.status === 'Unhealthy'
        ? 'error'
        : 'default'
  return (
    <Chip
      size="small"
      label={`Ready: ${label}`}
      color={color}
      variant="outlined"
    />
  )
}
