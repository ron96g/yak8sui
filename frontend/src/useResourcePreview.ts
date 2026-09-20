import { useEffect, useRef, useState } from 'react'
import type { ResourceHealth } from './ResourceWorkspace'
import type { RelationshipNode } from './useOwnershipHierarchy'

export type ResourcePreviewDetail = {
  uid: string
  name: string
  namespace?: string
  yaml: string
  health: ResourceHealth
}

export type PreviewState =
  | { status: 'loading' | 'reconnecting'; detail: null }
  | { status: 'current'; detail: ResourcePreviewDetail }
  | {
      status: 'unavailable' | 'revoked' | 'malformed'
      detail: null
      message: string
    }

export function useResourcePreview(
  tenantID: string,
  centerIdentity: string,
  node: RelationshipNode,
): PreviewState {
  const [state, setState] = useState<PreviewState>({
    status: 'loading',
    detail: null,
  })
  const generation = useRef(0)

  useEffect(() => {
    const request = ++generation.current
    setState({ status: 'loading', detail: null })
    let source: EventSource | null = null
    let terminal = false
    let probe = 0
    let probeController: AbortController | null = null
    const initialController = new AbortController()
    const current = () => generation.current === request
    const finish = (next: PreviewState) => {
      if (!current()) return
      terminal = true
      probeController?.abort()
      source?.close()
      setState(next)
    }
    const acceptDetail = (detail: ResourcePreviewDetail) => {
      if (
        detail.uid !== node.uid ||
        typeof detail.yaml !== 'string' ||
        typeof detail.name !== 'string' ||
        !detail.health
      )
        throw new Error('invalid detail')
      setState({ status: 'current', detail })
    }
    const openStream = () => {
      if (!current() || terminal) return
      const nextSource = new EventSource(detailStreamPath(tenantID, node))
      source = nextSource
      nextSource.addEventListener('snapshot', (event) => {
        if (!current() || source !== nextSource) return
        try {
          const detail = JSON.parse(
            (event as MessageEvent<string>).data,
          ) as ResourcePreviewDetail
          acceptDetail(detail)
          probe += 1
          probeController?.abort()
        } catch {
          finish({
            status: 'malformed',
            detail: null,
            message: 'The preview stream returned malformed data.',
          })
        }
      })
      nextSource.addEventListener('observation', () => {
        if (!current() || terminal || source !== nextSource) return
        probe += 1
        probeController?.abort()
        setState({
          status: 'unavailable',
          detail: null,
          message: 'Current observation is unavailable. YAML has been cleared.',
        })
      })
      nextSource.addEventListener('revoked', () =>
        finish({
          status: 'revoked',
          detail: null,
          message: 'This resource is no longer visible in the active tenant.',
        }),
      )
      nextSource.onerror = () => {
        if (!current() || terminal || source !== nextSource) return
        setState((currentState) =>
          currentState.status === 'unavailable'
            ? currentState
            : { status: 'reconnecting', detail: null },
        )
        const currentProbe = ++probe
        probeController?.abort()
        probeController = new AbortController()
        void fetch(detailPath(tenantID, node), {
          signal: probeController.signal,
        })
          .then((response) => {
            if (!current() || terminal || currentProbe !== probe) return
            if (response.status === 503)
              setState({
                status: 'unavailable',
                detail: null,
                message:
                  'Current observation is unavailable. YAML has been cleared.',
              })
            else if ([401, 403, 404].includes(response.status))
              finish({
                status: 'revoked',
                detail: null,
                message:
                  'This resource is no longer visible in the active tenant.',
              })
          })
          .catch(() => undefined)
      }
    }
    void fetch(detailPath(tenantID, node), {
      signal: initialController.signal,
    })
      .then(async (response) => {
        if (!current() || terminal) return
        if (response.status === 503) {
          setState({
            status: 'unavailable',
            detail: null,
            message:
              'Current observation is unavailable. YAML has been cleared.',
          })
          openStream()
          return
        }
        if ([401, 403, 404].includes(response.status)) {
          finish({
            status: 'revoked',
            detail: null,
            message: 'This resource is no longer visible in the active tenant.',
          })
          return
        }
        if (!response.ok) throw new Error('preview request failed')
        const detail = (await response.json()) as ResourcePreviewDetail
        acceptDetail(detail)
        openStream()
      })
      .catch((cause: unknown) => {
        if (
          current() &&
          !terminal &&
          !(cause instanceof DOMException && cause.name === 'AbortError')
        )
          finish({
            status: 'malformed',
            detail: null,
            message: 'The preview response could not be read.',
          })
      })
    return () => {
      ++generation.current
      initialController.abort()
      probeController?.abort()
      source?.close()
    }
  }, [tenantID, centerIdentity, node])

  return state
}

function detailStreamPath(tenantID: string, node: RelationshipNode) {
  const group = node.type.group || '_core'
  return `/api/v1/tenants/${encodeURIComponent(tenantID)}/detail-streams/${encodeURIComponent(group)}/${encodeURIComponent(node.type.version)}/${encodeURIComponent(node.type.resource)}/${encodeURIComponent(node.uid)}`
}

function detailPath(tenantID: string, node: RelationshipNode) {
  const group = node.type.group || '_core'
  return `/api/v1/tenants/${encodeURIComponent(tenantID)}/resources/${encodeURIComponent(group)}/${encodeURIComponent(node.type.version)}/${encodeURIComponent(node.type.resource)}/${encodeURIComponent(node.uid)}`
}
