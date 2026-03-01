import { useEffect, useState } from 'react'

export type Route =
  | { name: 'workflows' }
  | { name: 'workflow_detail'; workflowId: string }

export function parseRouteFromHash(hash: string): Route {
  const raw = hash ?? ''
  const m = raw.match(/^#\/workflows\/([^/]+)$/)
  if (!m) return { name: 'workflows' }
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

export function goToWorkflow(workflowId: string) {
  window.location.hash = `#/workflows/${encodeURIComponent(workflowId)}`
}

export function goHome() {
  window.location.hash = ''
}

