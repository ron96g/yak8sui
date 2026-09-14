import LogoutIcon from '@mui/icons-material/Logout'
import {
  Alert,
  AppBar,
  Box,
  Button,
  CircularProgress,
  Container,
  Stack,
  Toolbar,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  IconButton,
  MenuItem,
  Tab,
  Tabs,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
} from '@mui/material'
import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'
import ResourceWorkspace, {
  type QueryErrors,
  type ResourceQuery,
  type ResourceType,
  type ResourceSummary,
  type ObservationStatus,
} from './ResourceWorkspace'
import OwnershipView, { type RelationshipNode } from './OwnershipView'
import { shellBackground, technicalText } from './theme'

const YamlViewer = lazy(() => import('./YamlViewer'))

type Tenant = { id: string; name: string }
type Session = {
  user: { name: string; email: string }
  expiresAt: string
  tenants: Tenant[]
}
type ResourceCondition = {
  type?: string
  status?: string
  reason?: string
  message?: string
  observedGeneration?: number
  stale: boolean
}
type ResourceDetail = ResourceSummary & {
  yaml: string
  conditions: ResourceCondition[]
}
type ConnectionState = 'idle' | 'resynchronizing' | 'connected' | 'disconnected'
type ResourcePage = {
  items: ResourceSummary[]
  total: number
  page: number
  pageCount: number
}

const emptyQuery: ResourceQuery = {
  name: '',
  namespace: '',
  nameMode: 'fuzzy',
  namespaceMode: 'fuzzy',
  jsonPath: '',
  jsonComparison: 'exists',
  jsonValue: '',
  page: 1,
}

export default function App() {
  const [loading, setLoading] = useState(true)
  const [session, setSession] = useState<Session | null>(null)
  const [tenant, setTenant] = useState<Tenant | null>(null)
  const [resources, setResources] = useState<ResourceSummary[]>([])
  const [query, setQuery] = useState<ResourceQuery>(emptyQuery)
  const [queryErrors, setQueryErrors] = useState<QueryErrors>({})
  const [total, setTotal] = useState(0)
  const [pageCount, setPageCount] = useState(0)
  const [detail, setDetail] = useState<ResourceDetail | null>(null)
  const [resourceTypes, setResourceTypes] = useState<ResourceType[]>([])
  const [resourceType, setResourceType] = useState<ResourceType | null>(null)
  const [resourcesLoading, setResourcesLoading] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailTab, setDetailTab] = useState(0)
  const [detailType, setDetailType] = useState<ResourceType | null>(null)
  const hierarchyRevision = useRef(0)
  const [connectionState, setConnectionState] =
    useState<ConnectionState>('idle')
  const [observation, setObservation] = useState<ObservationStatus | null>(null)
  const contextRevision = useRef(0)
  const detailRevision = useRef(0)
  const streamRevision = useRef(0)
  const stream = useRef<EventSource | null>(null)
  const selectedUID = useRef<string | null>(null)
  const selectedDetailType = useRef<ResourceType | null>(null)
  const [error, setError] = useState<string | null>(() =>
    new URLSearchParams(location.search).get('authError'),
  )

  const clearTenantData = useCallback(() => {
    stream.current?.close()
    stream.current = null
    selectedUID.current = null
    selectedDetailType.current = null
    contextRevision.current += 1
    detailRevision.current += 1
    streamRevision.current += 1
    setTenant(null)
    setResources([])
    setQuery(emptyQuery)
    setQueryErrors({})
    setTotal(0)
    setPageCount(0)
    setDetail(null)
    setDetailType(null)
    hierarchyRevision.current += 1
    setResourceType(null)
    setResourceTypes([])
    setResourcesLoading(false)
    setDetailLoading(false)
    setConnectionState('idle')
    setObservation(null)
  }, [])
  useEffect(() => {
    fetch('/api/session')
      .then(async (response) => {
        if (response.status === 401) {
          setSession(null)
          clearTenantData()
          return
        }
        if (!response.ok) throw new Error('Could not load your session.')
        setSession((await response.json()) as Session)
      })
      .catch((cause: unknown) =>
        setError(
          cause instanceof Error
            ? cause.message
            : 'Could not load your session.',
        ),
      )
      .finally(() => setLoading(false))
  }, [clearTenantData])

  useEffect(() => {
    if (!session) return
    const remaining = new Date(session.expiresAt).getTime() - Date.now()
    if (remaining <= 0) {
      clearTenantData()
      setSession(null)
      setError('Your session expired. Sign in again.')
      return
    }
    const timer = window.setTimeout(() => {
      clearTenantData()
      setSession(null)
      setError('Your session expired. Sign in again.')
    }, remaining)
    return () => window.clearTimeout(timer)
  }, [session, clearTenantData])

  async function selectTenant(selected: Tenant) {
    setError(null)
    clearTenantData()
    const revision = contextRevision.current
    setTenant(selected)
    setResourcesLoading(true)
    try {
      const response = await fetch(
        `/api/tenants/${encodeURIComponent(selected.id)}/context`,
      )
      if (revision !== contextRevision.current) return
      if (response.status === 401) {
        clearTenantData()
        setSession(null)
        setError('Your session expired. Sign in again.')
        return
      }
      if (!response.ok)
        throw new Error('You are not authorized to use that tenant.')
      const configurationResponse = await fetch('/api/v1/configuration')
      if (revision !== contextRevision.current) return
      if (configurationResponse.status === 401) {
        clearTenantData()
        setSession(null)
        setError('Your session expired. Sign in again.')
        return
      }
      if (!configurationResponse.ok)
        throw new Error('Could not load configured resource types.')
      const configuration = (await configurationResponse.json()) as {
        resourceTypes: ResourceType[]
      }
      if (revision !== contextRevision.current) return
      setResourceTypes(configuration.resourceTypes)
      const initial =
        configuration.resourceTypes.find((item) => item.available) ??
        configuration.resourceTypes[0]
      if (initial) await selectResourceType(selected, initial, revision)
    } catch (cause) {
      if (revision === contextRevision.current)
        setError(
          cause instanceof Error ? cause.message : 'Could not load resources.',
        )
    } finally {
      if (revision === contextRevision.current) setResourcesLoading(false)
    }
  }

  async function selectResourceType(
    selectedTenant: Tenant,
    selectedType: ResourceType,
    context = contextRevision.current,
    selectedQuery: ResourceQuery = emptyQuery,
    preserveSelection = false,
  ) {
    setError(null)
    detailRevision.current += 1
    const request = ++streamRevision.current
    stream.current?.close()
    stream.current = null
    if (!preserveSelection) selectedUID.current = null
    setResourceType(selectedType)
    setObservation(selectedType.observation)
    setQuery(selectedQuery)
    setQueryErrors({})
    setResources([])
    if (!preserveSelection) setDetail(null)
    if (!selectedType.available) {
      setResourcesLoading(false)
      setConnectionState('idle')
      return
    }
    setResourcesLoading(true)
    setConnectionState('resynchronizing')
    try {
      const listResponse = await fetch(
        resourcePath(selectedTenant, selectedType) + queryString(selectedQuery),
      )
      if (
        context !== contextRevision.current ||
        request !== streamRevision.current
      )
        return
      if (!listResponse.ok) {
        const problem = (await listResponse.json().catch(() => ({}))) as {
          error?: string
          fields?: QueryErrors
          observation?: ObservationStatus
        }
        if (
          context !== contextRevision.current ||
          request !== streamRevision.current
        )
          return
        setQueryErrors(problem.fields ?? {})
        if (problem.observation) setObservation(problem.observation)
        setError(problem.error ?? 'Could not query resources.')
        setConnectionState('idle')
        return
      }
      const initialPage = (await listResponse.json()) as ResourcePage
      if (
        context !== contextRevision.current ||
        request !== streamRevision.current
      )
        return
      setResources(initialPage.items)
      setObservation({ state: 'current' })
      setTotal(initialPage.total)
      setPageCount(initialPage.pageCount)
      const source = new EventSource(
        resourceStreamPath(selectedTenant, selectedType) +
          queryString(selectedQuery),
      )
      stream.current = source
      source.onopen = () => {
        if (
          context === contextRevision.current &&
          request === streamRevision.current &&
          stream.current === source
        )
          setConnectionState('resynchronizing')
      }
      source.addEventListener('snapshot', (event) => {
        if (
          context !== contextRevision.current ||
          request !== streamRevision.current ||
          stream.current !== source
        )
          return
        let payload: ResourcePage
        try {
          payload = JSON.parse(
            (event as MessageEvent<string>).data,
          ) as ResourcePage
        } catch {
          source.close()
          setConnectionState('disconnected')
          setError('The live resource stream returned invalid data.')
          return
        }
        setResources(payload.items)
        setQuery((current) => ({ ...current, page: payload.page }))
        setTotal(payload.total)
        setPageCount(payload.pageCount)
        setResourcesLoading(false)
        setConnectionState('connected')
        setObservation({ state: 'current' })
        const uid = selectedUID.current
        if (!uid) return
        void loadResourceDetail(
          selectedTenant,
          selectedDetailType.current ?? selectedType,
          uid,
          context,
        )
      })
      source.addEventListener('observation', (event) => {
        if (
          context !== contextRevision.current ||
          request !== streamRevision.current ||
          stream.current !== source
        )
          return
        try {
          setObservation(
            JSON.parse(
              (event as MessageEvent<string>).data,
            ) as ObservationStatus,
          )
        } catch {
          setObservation({
            state: 'stale',
            reason: 'The observer reported an invalid status.',
          })
        }
      })
      source.onerror = () => {
        if (
          context === contextRevision.current &&
          request === streamRevision.current &&
          stream.current === source
        ) {
          setConnectionState('disconnected')
          setResourcesLoading(false)
          void refreshObservation(selectedType, context, request)
        }
      }
    } catch (cause) {
      if (
        context === contextRevision.current &&
        request === streamRevision.current
      ) {
        setError(
          cause instanceof Error ? cause.message : 'Could not query resources.',
        )
        setConnectionState('idle')
      }
    } finally {
      if (
        context === contextRevision.current &&
        request === streamRevision.current
      )
        setResourcesLoading(false)
    }
  }

  async function refreshObservation(
    selectedType: ResourceType,
    context: number,
    request: number,
  ) {
    try {
      const response = await fetch('/api/v1/configuration')
      if (
        !response.ok ||
        context !== contextRevision.current ||
        request !== streamRevision.current
      )
        return
      const payload = (await response.json()) as {
        resourceTypes: ResourceType[]
      }
      const current = payload.resourceTypes.find(
        (item) =>
          item.group === selectedType.group &&
          item.version === selectedType.version &&
          item.resource === selectedType.resource,
      )
      if (current) {
        setObservation(current.observation)
        setResourceTypes(payload.resourceTypes)
        setResourceType(current)
      }
    } catch {
      // EventSource remains the transport retry authority. This status refresh
      // is best effort and never fetches resource contents.
    }
  }

  async function openResource(item: ResourceSummary) {
    if (!tenant || !resourceType) return
    setError(null)
    const revision = contextRevision.current
    selectedUID.current = item.uid
    setDetail(null)
    setDetailLoading(true)
    setDetailTab(0)
    setDetailType(resourceType)
    selectedDetailType.current = resourceType
    await loadResourceDetail(tenant, resourceType, item.uid, revision)
  }

  async function loadResourceDetail(
    selectedTenant: Tenant,
    selectedType: ResourceType,
    uid: string,
    revision: number,
  ) {
    const request = ++detailRevision.current
    try {
      const response = await fetch(
        `${resourcePath(selectedTenant, selectedType)}/${encodeURIComponent(uid)}`,
      )
      if (
        revision !== contextRevision.current ||
        request !== detailRevision.current
      )
        return
      if (response.status === 401) {
        clearTenantData()
        setSession(null)
        return
      }
      if (response.status === 404) {
        selectedUID.current = null
        setDetail(null)
        setDetailType(null)
        selectedDetailType.current = null
        setError('The selected resource is no longer visible.')
        return
      }
      if (!response.ok) throw new Error('Could not load that resource.')
      const payload = (await response.json()) as ResourceDetail
      if (
        revision === contextRevision.current &&
        request === detailRevision.current
      )
        setDetail(payload)
    } catch (cause) {
      if (
        revision === contextRevision.current &&
        request === detailRevision.current
      )
        setError(
          cause instanceof Error
            ? cause.message
            : 'Could not load that resource.',
        )
    } finally {
      if (
        revision === contextRevision.current &&
        request === detailRevision.current
      )
        setDetailLoading(false)
    }
  }

  function closeDetail() {
    selectedUID.current = null
    detailRevision.current += 1
    setDetail(null)
    setDetailType(null)
    selectedDetailType.current = null
    setDetailLoading(false)
  }

  async function logout() {
    clearTenantData()
    setSession(null)
    await fetch('/api/logout', { method: 'POST' }).catch(() => undefined)
  }

  if (loading)
    return (
      <Box
        minHeight="100vh"
        display="grid"
        sx={{ ...shellBackground, placeItems: 'center' }}
      >
        <CircularProgress aria-label="Loading session" />
      </Box>
    )
  if (!session)
    return (
      <Box sx={shellBackground}>
        <Container maxWidth="sm">
          <Stack minHeight="100vh" justifyContent="center" spacing={4}>
            <Box>
              <Typography variant="overline" color="primary">
                Kubernetes resource inspection
              </Typography>
              <Typography variant="h2">YAK8SUI</Typography>
              <Typography color="text.secondary" mt={2}>
                Sign in with your organization identity to inspect authorized
                tenant resources.
              </Typography>
            </Box>
            {error && <Alert severity="error">{error}</Alert>}
            <Button href="/auth/login" variant="contained" size="large">
              Sign in with OIDC
            </Button>
          </Stack>
        </Container>
      </Box>
    )

  return (
    <Box
      sx={{
        ...shellBackground,
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <AppBar position="static" color="transparent" elevation={0}>
        <Toolbar sx={{ gap: 1 }}>
          <Typography variant="h6" color="primary" sx={{ flexGrow: 1 }}>
            YAK8SUI
          </Typography>
          <TextField
            select
            label="Tenant"
            size="small"
            value={tenant?.id || ''}
            onChange={(event) => {
              const selected = session.tenants.find(
                (item) => item.id === event.target.value,
              )
              if (selected) void selectTenant(selected)
            }}
            sx={{ minWidth: { xs: 136, sm: 160 } }}
          >
            {session.tenants.map((item) => (
              <MenuItem key={item.id} value={item.id}>
                {item.name}
              </MenuItem>
            ))}
          </TextField>
          <Typography
            color="text.secondary"
            mr={2}
            sx={{ display: { xs: 'none', sm: 'block' } }}
          >
            {session.user.name || session.user.email}
          </Typography>
          <IconButton
            aria-label="Log out"
            color="inherit"
            onClick={logout}
            sx={{ display: { xs: 'inline-flex', sm: 'none' } }}
          >
            <LogoutIcon />
          </IconButton>
          <Button
            color="inherit"
            startIcon={<LogoutIcon />}
            onClick={logout}
            sx={{ display: { xs: 'none', sm: 'inline-flex' } }}
          >
            Log out
          </Button>
        </Toolbar>
      </AppBar>
      {error && (
        <Alert severity="error" sx={{ m: 2 }}>
          {error}
        </Alert>
      )}
      {tenant ? (
        <ResourceWorkspace
          key={`${tenant.id}/${resourceType?.group}/${resourceType?.version}/${resourceType?.resource}`}
          resourceTypes={resourceTypes}
          resourceType={resourceType}
          resources={resources}
          query={query}
          queryErrors={queryErrors}
          total={total}
          pageCount={pageCount}
          loading={resourcesLoading}
          connectionState={connectionState}
          observation={observation}
          onRefresh={() =>
            resourceType &&
            void selectResourceType(
              tenant,
              resourceType,
              contextRevision.current,
              query,
              true,
            )
          }
          onSelectType={(selectedType) =>
            void selectResourceType(
              tenant,
              selectedType,
              contextRevision.current,
              selectedType === resourceType ? query : emptyQuery,
              selectedType === resourceType,
            )
          }
          onOpen={(item) => void openResource(item)}
          onQuery={(selectedQuery) =>
            resourceType &&
            void selectResourceType(
              tenant,
              resourceType,
              contextRevision.current,
              selectedQuery,
              true,
            )
          }
        />
      ) : (
        <Container sx={{ py: 4 }}>
          <Typography variant="h5" component="h1">
            Choose where to inspect
          </Typography>
          <Typography color="text.secondary" mt={1}>
            Select a tenant from the toolbar to browse its resources.
          </Typography>
          {session.tenants.length === 0 && (
            <Alert severity="warning">
              Your account has no authorized tenants.
            </Alert>
          )}
        </Container>
      )}
      <Dialog
        open={detailLoading || detail !== null}
        onClose={closeDetail}
        maxWidth="lg"
        fullWidth
        aria-labelledby="resource-detail-title"
      >
        <DialogTitle id="resource-detail-title">
          {detail?.name || 'Loading resource'} — Resource details
        </DialogTitle>
        <DialogContent>
          {observation && observation.state !== 'current' && (
            <Alert
              severity={
                observation.state === 'unavailable' ? 'error' : 'warning'
              }
              sx={{ mb: 2 }}
            >
              Observation is {observation.state}. This retained detail is stale
              and will not update until recovery.
            </Alert>
          )}
          {detail && (
            <Typography
              variant="body2"
              color="text.secondary"
              mb={2}
              sx={technicalText}
            >
              {detail.namespace || 'Cluster scoped'} · {detail.uid} · Read-only
            </Typography>
          )}
          {detail ? (
            <>
              <Tabs
                value={detailTab}
                onChange={(_, value: number) => setDetailTab(value)}
                aria-label="Resource detail views"
                sx={{ mb: 2 }}
              >
                <Tab label="Conditions" />
                <Tab label="YAML" />
                <Tab label="Ownership" />
              </Tabs>
              {detailTab === 0 ? (
                <ConditionsView detail={detail} />
              ) : detailTab === 1 ? (
                <Box
                  height="60vh"
                  border={1}
                  borderColor="divider"
                  borderRadius={1}
                  overflow="hidden"
                >
                  <Suspense
                    fallback={<CircularProgress aria-label="Loading YAML" />}
                  >
                    <YamlViewer value={detail.yaml} />
                  </Suspense>
                </Box>
              ) : detailType && tenant ? (
                <OwnershipView
                  tenantID={tenant.id}
                  revision={hierarchyRevision.current}
                  center={{ ...detail, type: detailType }}
                  onInspect={(node: RelationshipNode) => {
                    selectedUID.current = node.uid
                    setDetailType(node.type)
                    selectedDetailType.current = node.type
                    setDetailTab(0)
                    setDetailLoading(true)
                    void loadResourceDetail(
                      tenant,
                      node.type,
                      node.uid,
                      contextRevision.current,
                    )
                  }}
                />
              ) : null}
            </>
          ) : (
            <Box height="60vh" display="grid" sx={{ placeItems: 'center' }}>
              <CircularProgress aria-label="Loading resource" />
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={closeDetail}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

function ConditionsView({
  detail,
}: {
  detail: ResourceDetail & { conditions?: ResourceCondition[] }
}) {
  const conditions = detail.conditions ?? []
  return (
    <Stack spacing={2} minHeight="40vh">
      <Alert
        severity={
          detail.health.stale
            ? 'warning'
            : detail.health.status === 'Healthy'
              ? 'success'
              : detail.health.status === 'Unhealthy'
                ? 'error'
                : 'info'
        }
      >
        {detail.health.stale
          ? `Stale — Ready last reported ${detail.health.status}. This is not confirmation of the current generation.`
          : `${detail.health.status} — determined only from the Ready condition.`}
      </Alert>
      {conditions.length ? (
        <TableContainer>
          <Table size="small" aria-label="Resource conditions">
            <TableHead>
              <TableRow>
                <TableCell>Type</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Reason</TableCell>
                <TableCell>Message</TableCell>
                <TableCell>Observed generation</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {conditions.map((condition) => (
                <TableRow
                  key={`${condition.type}:${condition.status}:${condition.reason}:${condition.message}:${condition.observedGeneration}`}
                >
                  <TableCell sx={technicalText}>
                    {condition.type || 'Not reported'}
                  </TableCell>
                  <TableCell>
                    {condition.status || 'Not reported'}
                    {condition.stale && (
                      <Typography
                        color="warning.main"
                        variant="caption"
                        display="block"
                      >
                        Stale
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell sx={technicalText}>
                    {condition.reason || 'Not reported'}
                  </TableCell>
                  <TableCell sx={{ whiteSpace: 'pre-wrap' }}>
                    {condition.message || 'Not reported'}
                  </TableCell>
                  <TableCell sx={technicalText}>
                    {condition.observedGeneration ?? 'Not reported'}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      ) : (
        <Alert severity="info">
          No readable conditions were reported. Full status remains available in
          YAML.
        </Alert>
      )}
    </Stack>
  )
}

function resourcePath(tenant: Tenant, resourceType: ResourceType) {
  return `/api/v1/tenants/${encodeURIComponent(tenant.id)}/resources/${encodeURIComponent(resourceType.group || '_core')}/${encodeURIComponent(resourceType.version)}/${encodeURIComponent(resourceType.resource)}`
}

function resourceStreamPath(tenant: Tenant, resourceType: ResourceType) {
  return `/api/v1/tenants/${encodeURIComponent(tenant.id)}/resource-streams/${encodeURIComponent(resourceType.group || '_core')}/${encodeURIComponent(resourceType.version)}/${encodeURIComponent(resourceType.resource)}`
}

function queryString(query: ResourceQuery) {
  const parameters = new URLSearchParams({
    name: query.name,
    namespace: query.namespace,
    nameMode: query.nameMode,
    namespaceMode: query.namespaceMode,
    jsonPath: query.jsonPath,
    jsonComparison: query.jsonComparison,
    jsonValue: query.jsonValue,
    page: String(query.page),
  })
  return `?${parameters.toString()}`
}
