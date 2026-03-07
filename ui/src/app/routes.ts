import { useEffect, useState } from 'react'

export type Route =
  | { name: 'orchestrations' }
  | { name: 'orchestration_detail'; orchestrationId: string }
  | { name: 'workflows' }
  | { name: 'chat' }
  | { name: 'repo_library_repositories' }
  | { name: 'repo_library_repository_detail'; repositoryId: string }
  | { name: 'repo_library_pattern_search' }
  | { name: 'workflow_detail'; workflowId: string }

export function parseRouteFromHash(hash: string): Route {
  const raw = hash ?? ''
  if (raw === '' || raw === '#' || raw === '#/') return { name: 'orchestrations' }
  if (/^#\/chat$/.test(raw)) return { name: 'chat' }
  if (/^#\/repo-library(?:\/repositories)?$/.test(raw)) {
    return { name: 'repo_library_repositories' }
  }
  if (/^#\/repo-library\/search$/.test(raw)) {
    return { name: 'repo_library_pattern_search' }
  }
  const repositoryMatch = raw.match(/^#\/repo-library\/repositories\/([^/]+)$/)
  if (repositoryMatch) {
    return {
      name: 'repo_library_repository_detail',
      repositoryId: decodeURIComponent(repositoryMatch[1]),
    }
  }
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

export function goToRepoLibraryRepositories() {
  window.location.hash = '#/repo-library/repositories'
}

export function goToRepoLibraryRepository(repositoryId: string) {
  window.location.hash = `#/repo-library/repositories/${encodeURIComponent(repositoryId)}`
}

export function goToRepoLibraryPatternSearch() {
  window.location.hash = '#/repo-library/search'
}
