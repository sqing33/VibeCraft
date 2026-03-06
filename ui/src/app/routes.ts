import { useEffect, useState } from 'react'

export type Route =
  | { name: 'orchestrations' }
  | { name: 'orchestration_detail'; orchestrationId: string }
  | { name: 'workflows' }
  | { name: 'chat' }
  | { name: 'workflow_detail'; workflowId: string }

export function parseRouteFromHash(hash: string): Route {
  const raw = hash ?? ''
  if (raw === '' || raw === '#' || raw === '#/') return { name: 'orchestrations' }
  if (/^#\/chat$/.test(raw)) return { name: 'chat' }
  const orchestrationMatch = raw.match(/^#\/orchestrations\/([^/]+)$/)
  if (orchestrationMatch) {
    return { name: 'orchestration_detail', orchestrationId: decodeURIComponent(orchestrationMatch[1]) }
  }
  if (/^#\/orchestrations$/.test(raw)) return { name: 'orchestrations' }
  if (/^#\/(legacy-workflows|workflows)$/.test(raw)) return { name: 'workflows' }
  const m = raw.match(/^#\/workflows\/([^/]+)$/)
  if (!m) return { name: 'orchestrations' }
  return { name: 'workflow_detail', workflowId: decodeURIComponent(m[1]) }
}

export function useHashRoute(): Route {
  const [route, setRoute] = useState<Route>(() =>
    parseRouteFromHash(window.location.hash),
  )

  useEffect(() => {
    const onHashChange = () => setRoute(parseRouteFromHash(window.location.hash))
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  return route
}

export function goToOrchestration(orchestrationId: string) {
  window.location.hash = `#/orchestrations/${encodeURIComponent(orchestrationId)}`
}

export function goToWorkflow(workflowId: string) {
  window.location.hash = `#/workflows/${encodeURIComponent(workflowId)}`
}

export function goHome() {
  window.location.hash = '#/orchestrations'
}

export function goToLegacyWorkflows() {
  window.location.hash = '#/legacy-workflows'
}

export function goToChat() {
  window.location.hash = '#/chat'
}
