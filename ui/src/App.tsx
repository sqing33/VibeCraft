import { useEffect } from 'react'

import { useHashRoute } from '@/app/routes'
import { Topbar } from '@/app/components/Topbar'
import { WorkflowsPage } from '@/app/pages/WorkflowsPage'
import { WorkflowDetailPage } from '@/app/pages/WorkflowDetailPage'
import { fetchExperts, fetchHealth, fetchInfo } from '@/lib/daemon'
import { emitWsEnvelope } from '@/lib/wsBus'
import { parseWsEnvelope } from '@/lib/ws'
import { useDaemonStore } from '@/stores/daemonStore'

/**
 * 功能：应用入口（App Shell + 路由），并集中维护 daemon health 与 WS 连接状态。
 * 参数/返回：无入参；返回 React 组件树。
 * 失败场景：daemon 不可达时 health 进入 error，并由页面展示可恢复提示。
 * 副作用：发起 health/info/experts 请求；建立 WebSocket 连接并断线重连。
 */
export default function App() {
  const route = useHashRoute()

  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const wsUrl = useDaemonStore((s) => s.wsUrl)
  const setHealth = useDaemonStore((s) => s.setHealth)
  const setWsState = useDaemonStore((s) => s.setWsState)
  const setInfo = useDaemonStore((s) => s.setInfo)
  const setInfoError = useDaemonStore((s) => s.setInfoError)
  const setExperts = useDaemonStore((s) => s.setExperts)
  const setExpertsError = useDaemonStore((s) => s.setExpertsError)

  useEffect(() => {
    const abortController = new AbortController()
    let cancelled = false

    setHealth({ status: 'checking' })
    setInfo(null)
    setInfoError(null)
    setExperts([])
    setExpertsError(null)

    fetchHealth(daemonUrl, abortController.signal)
      .then(() => {
        if (cancelled) return
        setHealth({ status: 'ok' })

        fetchInfo(daemonUrl)
          .then((res) => {
            if (cancelled) return
            setInfo(res)
          })
          .catch((err: unknown) => {
            if (cancelled) return
            const message = err instanceof Error ? err.message : String(err)
            setInfoError(message)
          })

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
  }, [
    daemonUrl,
    setExperts,
    setExpertsError,
    setHealth,
    setInfo,
    setInfoError,
  ])

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
    <div className="min-h-screen">
      <Topbar />
      <main className="mx-auto max-w-6xl p-4">
        {route.name === 'workflows' ? (
          <WorkflowsPage />
        ) : (
          <WorkflowDetailPage workflowId={route.workflowId} />
        )}
      </main>
    </div>
  )
}
