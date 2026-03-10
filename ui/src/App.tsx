import { useEffect } from 'react'

import { type Route, useHashRoute } from '@/app/routes'
import { Topbar } from '@/app/components/Topbar'
import { WorkspaceShell } from '@/app/components/WorkspaceShell'
import { ChatSessionsPage } from '@/app/pages/ChatSessionsPage'
import { OrchestrationDetailPage } from '@/app/pages/OrchestrationDetailPage'
import { OrchestrationsPage } from '@/app/pages/OrchestrationsPage'
import { RepoLibraryPatternSearchPage } from '@/app/pages/RepoLibraryPatternSearchPage'
import { RepoLibraryRepositoriesPage } from '@/app/pages/RepoLibraryRepositoriesPage'
import { RepoLibraryRepositoryDetailPage } from '@/app/pages/RepoLibraryRepositoryDetailPage'
import { WorkflowsPage } from '@/app/pages/WorkflowsPage'
import { WorkflowDetailPage } from '@/app/pages/WorkflowDetailPage'
import { fetchExperts, fetchHealth } from '@/lib/daemon'
import { emitWsEnvelope } from '@/lib/wsBus'
import { parseWsEnvelope } from '@/lib/ws'
import { useDaemonStore } from '@/stores/daemonStore'

function isWorkspaceRoute(route: Route): boolean {
  return (
    route.name === 'chat' ||
    route.name === 'orchestrations' ||
    route.name === 'orchestration_detail' ||
    route.name === 'repo_library_repositories' ||
    route.name === 'repo_library_repository_detail' ||
    route.name === 'repo_library_pattern_search'
  )
}

function workspaceNavForRoute(route: Route): 'chat' | 'orchestrations' | 'repo_library' {
  if (route.name === 'chat') return 'chat'
  if (
    route.name === 'repo_library_repositories' ||
    route.name === 'repo_library_repository_detail' ||
    route.name === 'repo_library_pattern_search'
  ) {
    return 'repo_library'
  }
  return 'orchestrations'
}

function renderRoute(route: Route) {
  if (route.name === 'orchestrations') return <OrchestrationsPage />
  if (route.name === 'orchestration_detail') {
    return <OrchestrationDetailPage orchestrationId={route.orchestrationId} />
  }
  if (route.name === 'repo_library_repositories') return <RepoLibraryRepositoriesPage />
  if (route.name === 'repo_library_pattern_search') return <RepoLibraryPatternSearchPage />
  if (route.name === 'repo_library_repository_detail') {
    return <RepoLibraryRepositoryDetailPage repositoryId={route.repositoryId} />
  }
  if (route.name === 'workflows') return <WorkflowsPage />
  if (route.name === 'chat') return <ChatSessionsPage sessionId={route.sessionId} />
  return <WorkflowDetailPage workflowId={route.workflowId} />
}

/**
 * 功能：应用入口（App Shell + 路由），并集中维护 daemon health 与 WS 连接状态。
 * 参数/返回：无入参；返回 React 组件树。
 * 失败场景：daemon 不可达时 health 进入 error，并由页面展示可恢复提示。
 * 副作用：发起 health/experts 请求；建立 WebSocket 连接并断线重连。
 */
export default function App() {
  const route = useHashRoute()
  const workspaceRoute = isWorkspaceRoute(route)

  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const wsUrl = useDaemonStore((s) => s.wsUrl)
  const setHealth = useDaemonStore((s) => s.setHealth)
  const setWsState = useDaemonStore((s) => s.setWsState)
  const setExperts = useDaemonStore((s) => s.setExperts)
  const setExpertsError = useDaemonStore((s) => s.setExpertsError)

  useEffect(() => {
    const abortController = new AbortController()
    let cancelled = false

    setHealth({ status: 'checking' })
    setExperts([])
    setExpertsError(null)

    fetchHealth(daemonUrl, abortController.signal)
      .then(() => {
        if (cancelled) return
        setHealth({ status: 'ok' })

        fetchExperts(daemonUrl)
          .then((list) => {
            if (cancelled) return
            setExperts(list)
          })
          .catch((err: unknown) => {
            if (cancelled) return
            const message = err instanceof Error ? err.message : String(err)
            setExpertsError(message)
          })
      })
      .catch((err: unknown) => {
        if (abortController.signal.aborted) return
        const message = err instanceof Error ? err.message : String(err)
        setHealth({ status: 'error', message })
      })

    return () => {
      cancelled = true
      abortController.abort()
    }
  }, [daemonUrl, setExperts, setExpertsError, setHealth])

  useEffect(() => {
    if (!wsUrl) {
      setWsState('disconnected')
      return
    }

    let stopped = false
    let socket: WebSocket | null = null
    let reconnectTimer: number | undefined

    const connect = () => {
      if (stopped) return
      setWsState('connecting')
      socket = new WebSocket(wsUrl)

      socket.onopen = () => {
        setWsState('connected')
      }

      socket.onmessage = (ev) => {
        const env = parseWsEnvelope(String(ev.data ?? ''))
        if (!env) return
        emitWsEnvelope(env)
      }

      socket.onclose = () => {
        setWsState('disconnected')
        if (stopped) return
        reconnectTimer = window.setTimeout(connect, 1000)
      }

      socket.onerror = () => {
        socket?.close()
      }
    }

    connect()

    return () => {
      stopped = true
      if (reconnectTimer) window.clearTimeout(reconnectTimer)
      socket?.close()
    }
  }, [wsUrl, setWsState])

  return (
    <div className={workspaceRoute ? 'h-screen overflow-hidden' : 'min-h-screen'}>
      {workspaceRoute ? null : <Topbar />}
      <main className={workspaceRoute ? 'h-full w-full overflow-hidden p-[5px]' : 'mx-auto max-w-6xl p-4'}>
        {workspaceRoute ? (
          <WorkspaceShell activeNav={workspaceNavForRoute(route)}>
            {renderRoute(route)}
          </WorkspaceShell>
        ) : (
          renderRoute(route)
        )}
      </main>
    </div>
  )
}
