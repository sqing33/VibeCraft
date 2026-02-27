import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import './App.css'
import {
  cancelExecution,
  createWorkflow,
  daemonUrlFromEnv,
  fetchExecutionLogTail,
  fetchHealth,
  fetchWorkflowNodes,
  fetchWorkflows,
  startWorkflow,
  startExecution,
  wsUrlFromDaemonUrl,
  type Execution,
  type Node,
  type Workflow,
} from './lib/daemon'
import { TerminalPane, type TerminalPaneHandle } from './components/TerminalPane'

type HealthState =
  | { status: 'checking' }
  | { status: 'ok' }
  | { status: 'error'; message: string }

type WsState = 'connecting' | 'connected' | 'disconnected'

type WsEnvelope = {
  type: string
  ts: number
  workflow_id?: string
  node_id?: string
  execution_id?: string
  payload?: unknown
}

function workflowIdFromHash(): string | null {
  const raw = window.location.hash ?? ''
  const m = raw.match(/^#\/workflows\/([^/]+)$/)
  if (!m) return null
  return decodeURIComponent(m[1])
}

/**
 * 功能：MVP 首页，提供 daemon health、execution 列表与实时终端（WS + xterm）。
 * 参数/返回：无入参；返回 React 组件。
 * 失败场景：daemon 不可达时展示错误；WS 断线时自动重连并通过 log tail 补齐。
 * 副作用：发起 HTTP/WS 请求、维护本地状态、向终端写入输出。
 */
function App() {
  const daemonUrl = useMemo(() => daemonUrlFromEnv(), [])
  const wsUrl = useMemo(() => wsUrlFromDaemonUrl(daemonUrl), [daemonUrl])
  const [health, setHealth] = useState<HealthState>({ status: 'checking' })
  const [wsState, setWsState] = useState<WsState>('connecting')
  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [wfTitle, setWfTitle] = useState('')
  const [wfWorkspace, setWfWorkspace] = useState('.')
  const [wfMode, setWfMode] = useState<'manual' | 'auto'>('manual')
  const [wfError, setWfError] = useState<string | null>(null)
  const [wfStartingId, setWfStartingId] = useState<string | null>(null)
  const [selectedWorkflowId, setSelectedWorkflowId] = useState<string | null>(
    () => workflowIdFromHash(),
  )
  const [nodes, setNodes] = useState<Node[]>([])
  const [nodesLoading, setNodesLoading] = useState(false)
  const [nodesError, setNodesError] = useState<string | null>(null)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [executions, setExecutions] = useState<Execution[]>([])
  const [selectedExecutionId, setSelectedExecutionId] = useState<string | null>(
    null,
  )
  const [execError, setExecError] = useState<string | null>(null)

  const selectedWorkflow = useMemo(() => {
    if (!selectedWorkflowId) return null
    return workflows.find((w) => w.workflow_id === selectedWorkflowId) ?? null
  }, [workflows, selectedWorkflowId])

  const selectedNode = useMemo(() => {
    if (!selectedNodeId) return null
    return nodes.find((n) => n.node_id === selectedNodeId) ?? null
  }, [nodes, selectedNodeId])

  const terminalRef = useRef<TerminalPaneHandle | null>(null)
  const selectedExecutionIdRef = useRef<string | null>(null)
  const selectedWorkflowIdRef = useRef<string | null>(null)
  const selectedNodeIdRef = useRef<string | null>(null)

  useEffect(() => {
    selectedExecutionIdRef.current = selectedExecutionId
  }, [selectedExecutionId])

  useEffect(() => {
    selectedWorkflowIdRef.current = selectedWorkflowId
  }, [selectedWorkflowId])

  useEffect(() => {
    selectedNodeIdRef.current = selectedNodeId
  }, [selectedNodeId])

  useEffect(() => {
    const onHashChange = () => setSelectedWorkflowId(workflowIdFromHash())
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  useEffect(() => {
    const abortController = new AbortController()

    fetchHealth(daemonUrl, abortController.signal)
      .then(() => setHealth({ status: 'ok' }))
      .catch((err: unknown) => {
        if (abortController.signal.aborted) return
        const message = err instanceof Error ? err.message : String(err)
        setHealth({ status: 'error', message })
      })

    return () => abortController.abort()
  }, [daemonUrl])

  const loadWorkflows = useCallback(async () => {
    setWfError(null)
    try {
      const wfs = await fetchWorkflows(daemonUrl)
      setWorkflows(wfs)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setWfError(message)
    }
  }, [daemonUrl])

  const openWorkflow = useCallback((workflowId: string) => {
    window.location.hash = `#/workflows/${encodeURIComponent(workflowId)}`
  }, [])

  const closeWorkflow = useCallback(() => {
    window.location.hash = ''
  }, [])

  useEffect(() => {
    let cancelled = false
    fetchWorkflows(daemonUrl)
      .then((wfs) => {
        if (cancelled) return
        setWorkflows(wfs)
        setWfError(null)
      })
      .catch((err: unknown) => {
        if (cancelled) return
        const message = err instanceof Error ? err.message : String(err)
        setWfError(message)
      })

    return () => {
      cancelled = true
    }
  }, [daemonUrl])

  const refreshNodes = useCallback(async () => {
    if (!selectedWorkflowId) return
    setNodesError(null)
    setNodesLoading(true)
    try {
      const ns = await fetchWorkflowNodes(daemonUrl, selectedWorkflowId)
      setNodes(ns)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setNodesError(message)
    } finally {
      setNodesLoading(false)
    }
  }, [daemonUrl, selectedWorkflowId])

  useEffect(() => {
    if (!selectedWorkflowId) {
      setNodes([])
      setNodesLoading(false)
      setNodesError(null)
      setSelectedNodeId(null)
      return
    }

    let cancelled = false
    setNodesLoading(true)
    setNodesError(null)
    fetchWorkflowNodes(daemonUrl, selectedWorkflowId)
      .then((ns) => {
        if (cancelled) return
        setNodes(ns)
        setNodesLoading(false)

        const preferred =
          ns.find((n) => n.node_type === 'master') ?? ns[0] ?? null
        setSelectedNodeId(preferred?.node_id ?? null)
        if (preferred?.last_execution_id) {
          setSelectedExecutionId(preferred.last_execution_id)
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return
        const message = err instanceof Error ? err.message : String(err)
        setNodesError(message)
        setNodesLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [daemonUrl, selectedWorkflowId])

  const onSelectNode = useCallback((node: Node) => {
    setSelectedNodeId(node.node_id)
    if (node.last_execution_id) {
      setSelectedExecutionId(node.last_execution_id)
    } else {
      setSelectedExecutionId(null)
      terminalRef.current?.reset('No execution yet.\r\n')
    }
  }, [])

  const loadTailIntoTerminal = useCallback(
    async (executionId: string) => {
      terminalRef.current?.reset('Loading log…\r\n')
      try {
        const text = await fetchExecutionLogTail(daemonUrl, executionId, 200000)
        terminalRef.current?.reset(text)
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : String(err)
        terminalRef.current?.reset(`Failed to load log: ${message}\r\n`)
      }
    },
    [daemonUrl],
  )

  useEffect(() => {
    if (!selectedExecutionId) return
    void loadTailIntoTerminal(selectedExecutionId)
  }, [selectedExecutionId, loadTailIntoTerminal])

  useEffect(() => {
    // 功能：建立 WS 连接并将 node.log 增量路由到当前选中的 execution；断线自动重连。
    // 参数/返回：依赖 wsUrl 与 loadTailIntoTerminal；无返回值。
    // 失败场景：WS 握手失败或异常断开时进入重连循环（UI 仍可通过 tail 回放）。
    // 副作用：创建 WebSocket、注册事件回调、更新本地状态与终端输出。
    let stopped = false
    let socket: WebSocket | null = null
    let reconnectTimer: number | undefined

    const connect = () => {
      if (stopped) return
      setWsState('connecting')
      socket = new WebSocket(wsUrl)

      socket.onopen = () => {
        setWsState('connected')
        const exId = selectedExecutionIdRef.current
        if (exId) void loadTailIntoTerminal(exId)
      }

      socket.onmessage = (ev) => {
        let env: WsEnvelope | null = null
        try {
          env = JSON.parse(ev.data) as WsEnvelope
        } catch {
          return
        }

        if (env.type === 'workflow.updated') {
          const payload = env.payload as Partial<Workflow> | undefined
          const wfId =
            payload && typeof payload === 'object'
              ? (payload as { workflow_id?: unknown }).workflow_id
              : undefined
          if (typeof wfId !== 'string') return
          const wf = payload as Workflow
          setWorkflows((prev) => {
            const next = prev.some((w) => w.workflow_id === wf.workflow_id)
              ? prev.map((w) => (w.workflow_id === wf.workflow_id ? wf : w))
              : [wf, ...prev]
            return next.sort((a, b) => b.updated_at - a.updated_at)
          })
          return
        }

        if (env.type === 'node.updated') {
          const payload = env.payload as Partial<Node> | undefined
          const nodeId =
            payload && typeof payload === 'object'
              ? (payload as { node_id?: unknown }).node_id
              : undefined
          if (typeof nodeId !== 'string') return
          const node = payload as Node

          const currentWorkflowId = selectedWorkflowIdRef.current
          if (currentWorkflowId && node.workflow_id === currentWorkflowId) {
            setNodes((prev) => {
              if (prev.some((n) => n.node_id === node.node_id)) {
                return prev.map((n) => (n.node_id === node.node_id ? node : n))
              }
              return [...prev, node]
            })
            if (selectedNodeIdRef.current === node.node_id && node.last_execution_id) {
              setSelectedExecutionId(node.last_execution_id)
            }
          }
          return
        }

        const exId = env.execution_id
        if (!exId) return

        if (env.type === 'node.log') {
          const payload = env.payload as { chunk?: unknown } | undefined
          const chunk = typeof payload?.chunk === 'string' ? payload.chunk : ''
          if (exId === selectedExecutionIdRef.current) {
            terminalRef.current?.write(chunk)
          }
          return
        }

        if (env.type === 'execution.exited') {
          const payload = env.payload as { status?: unknown } | undefined
          const status =
            typeof payload?.status === 'string' ? payload.status : 'failed'
          setExecutions((prev) =>
            prev.map((e) =>
              e.execution_id === exId ? { ...e, status } : e,
            ),
          )
          return
        }

        if (env.type === 'execution.started') {
          const payload =
            env.payload as
              | { command?: unknown; args?: unknown; cwd?: unknown }
              | undefined
          const command =
            typeof payload?.command === 'string' ? payload.command : 'unknown'
          const args = Array.isArray(payload?.args)
            ? payload?.args.filter((a): a is string => typeof a === 'string')
            : []
          const cwd = typeof payload?.cwd === 'string' ? payload.cwd : undefined

          setExecutions((prev) => {
            if (prev.some((e) => e.execution_id === exId)) return prev
            const startedAt = new Date().toISOString()
            return [
              { execution_id: exId, status: 'running', command, args, cwd, started_at: startedAt },
              ...prev,
            ]
          })
        }
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
  }, [wsUrl, loadTailIntoTerminal])

  const onRunDemo = async () => {
    setExecError(null)
    try {
      const exec = await startExecution(daemonUrl)
      setExecutions((prev) => [exec, ...prev])
      setSelectedExecutionId(exec.execution_id)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setExecError(message)
    }
  }

  const onCancelSelected = async () => {
    if (!selectedExecutionId) return
    setExecError(null)
    try {
      await cancelExecution(daemonUrl, selectedExecutionId)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setExecError(message)
    }
  }

  const onCreateWorkflow = async () => {
    setWfError(null)
    try {
      const created = await createWorkflow(daemonUrl, {
        title: wfTitle.trim() ? wfTitle.trim() : undefined,
        workspace_path: wfWorkspace.trim(),
        mode: wfMode,
      })
      setWfTitle('')
      setWfWorkspace(created.workspace_path)
      await loadWorkflows()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setWfError(message)
    }
  }

  const onStartWorkflow = async (workflowId: string) => {
    setWfError(null)
    setWfStartingId(workflowId)
    openWorkflow(workflowId)
    try {
      const started = await startWorkflow(daemonUrl, workflowId)
      setWorkflows((prev) =>
        prev.map((wf) =>
          wf.workflow_id === started.workflow.workflow_id ? started.workflow : wf,
        ),
      )
      setExecutions((prev) => {
        if (prev.some((e) => e.execution_id === started.execution.execution_id)) {
          return prev
        }
        return [started.execution, ...prev]
      })
      setSelectedExecutionId(started.execution.execution_id)
      void refreshNodes()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setWfError(message)
    } finally {
      setWfStartingId(null)
    }
  }

  const kanbanColumns = useMemo(
    () => [
      { key: 'todo', title: 'Todo', statuses: ['todo'] },
      { key: 'running', title: 'Running', statuses: ['running'] },
      { key: 'done', title: 'Done', statuses: ['done'] },
      { key: 'failed', title: 'Failed', statuses: ['failed', 'canceled'] },
    ],
    [],
  )

  return (
    <div className="page">
      <header className="header">
        <h1 className="title">vibe-tree</h1>
        <div className="subtitle">MVP</div>
      </header>

      <section className="panel">
        <div className="panelTitle">Daemon</div>
        <div className="row">
          <div className="label">URL</div>
          <div className="value">
            <code>{daemonUrl}</code>
          </div>
        </div>
        <div className="row">
          <div className="label">Health</div>
          <div className="value">
            {health.status === 'checking' && <span>Checking…</span>}
            {health.status === 'ok' && <span className="ok">OK</span>}
            {health.status === 'error' && (
              <div className="errorBox">
                <div className="errorTitle">无法连接到 daemon</div>
                <div className="errorMsg">{health.message}</div>
                <div className="hint">
                  请确认后端已启动，且端口配置一致（默认 7777）。
                </div>
              </div>
            )}
          </div>
        </div>
      </section>

      <section className="panel">
        <div className="panelTitle">Workflows</div>

        <div className="row">
          <div className="label">Title</div>
          <div className="value">
            <input
              className="input"
              placeholder="Untitled"
              value={wfTitle}
              onChange={(e) => setWfTitle(e.target.value)}
            />
          </div>
        </div>
        <div className="row">
          <div className="label">Workspace</div>
          <div className="value">
            <input
              className="input"
              placeholder="."
              value={wfWorkspace}
              onChange={(e) => setWfWorkspace(e.target.value)}
            />
          </div>
        </div>
        <div className="row">
          <div className="label">Mode</div>
          <div className="value">
            <select
              className="select"
              value={wfMode}
              onChange={(e) =>
                setWfMode(e.target.value === 'auto' ? 'auto' : 'manual')
              }
            >
              <option value="manual">manual</option>
              <option value="auto">auto</option>
            </select>
          </div>
        </div>
        <div className="row">
          <div className="label">Actions</div>
          <div className="value">
            <div className="actionsRow">
              <button className="primaryBtnInline" onClick={onCreateWorkflow}>
                Create
              </button>
              <button className="ghostBtn" onClick={loadWorkflows}>
                Refresh
              </button>
            </div>
          </div>
        </div>

        {wfError && (
          <div className="errorBox" style={{ marginTop: 10 }}>
            <div className="errorTitle">加载/创建 workflow 失败</div>
            <div className="errorMsg">{wfError}</div>
          </div>
        )}

        <div className="kanban">
          {kanbanColumns.map((col) => {
            const items = workflows.filter((w) => col.statuses.includes(w.status))
            return (
              <div key={col.key} className="kanbanCol">
                <div className="kanbanColHeader">
                  <div className="kanbanColTitle">{col.title}</div>
                  <div className="kanbanColCount">{items.length}</div>
                </div>
                <div className="kanbanColList">
                  {items.length === 0 ? (
                    <div className="emptyHint">-</div>
                  ) : (
                    items.map((wf) => (
                      <div
                        key={wf.workflow_id}
                        className={
                          wf.workflow_id === selectedWorkflowId
                            ? 'wfItem selected'
                            : 'wfItem'
                        }
                        onClick={() => openWorkflow(wf.workflow_id)}
                        role="button"
                        tabIndex={0}
                      >
                        <div className="wfItemTop">
                          <span className="wfStatus">{wf.status}</span>
                          <span className="wfMode">{wf.mode}</span>
                        </div>
                        <div className="wfTitleRow">
                          <span className="wfTitleText">{wf.title}</span>
                          <span className="wfId">{wf.workflow_id}</span>
                        </div>
                        <div className="wfMetaRow">
                          <div className="wfMeta">{wf.workspace_path}</div>
                          {wf.status === 'todo' && (
                            <button
                              className="ghostBtn"
                              disabled={
                                wfStartingId === wf.workflow_id ||
                                wf.status !== 'todo'
                              }
                              onClick={(e) => {
                                e.stopPropagation()
                                void onStartWorkflow(wf.workflow_id)
                              }}
                            >
                              {wfStartingId === wf.workflow_id ? 'Starting…' : 'Start'}
                            </button>
                          )}
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </section>

      <section className="panel">
        <div className="panelTitle">Executions</div>
        <div className="execLayout">
          <div className="execSidebar">
            <button onClick={onRunDemo} className="primaryBtn">
              Run demo
            </button>
            <div className="wsRow">
              <div className="label">WS</div>
              <div className="value">
                <span
                  className={
                    wsState === 'connected'
                      ? 'ok'
                      : wsState === 'connecting'
                        ? 'muted'
                        : 'warn'
                  }
                >
                  {wsState}
                </span>
              </div>
            </div>
            {execError && (
              <div className="errorBox">
                <div className="errorTitle">启动 execution 失败</div>
                <div className="errorMsg">{execError}</div>
              </div>
            )}

            <div className="detailBox">
              <div className="detailBoxHeader">
                <div className="detailBoxTitle">
                  <div className="detailBoxTitleText">
                    {selectedWorkflow?.title ?? 'Workflow'}
                  </div>
                  <div className="detailBoxTitleSub">
                    {selectedWorkflowId ? (
                      <code>{selectedWorkflowId}</code>
                    ) : (
                      <span className="muted">未选择 workflow</span>
                    )}
                  </div>
                </div>
                <div className="detailBoxActions">
                  <button
                    className="ghostBtn"
                    disabled={!selectedWorkflowId || nodesLoading}
                    onClick={refreshNodes}
                  >
                    Refresh nodes
                  </button>
                  <button
                    className="ghostBtn"
                    disabled={!selectedWorkflowId}
                    onClick={closeWorkflow}
                  >
                    Close
                  </button>
                </div>
              </div>

              {nodesError && (
                <div className="errorBox" style={{ marginTop: 10 }}>
                  <div className="errorTitle">加载 nodes 失败</div>
                  <div className="errorMsg">{nodesError}</div>
                </div>
              )}

              <div className="nodeList">
                {!selectedWorkflowId ? (
                  <div className="emptyHint">点击 Kanban 卡片打开详情。</div>
                ) : nodesLoading ? (
                  <div className="emptyHint">Loading nodes…</div>
                ) : nodes.length === 0 ? (
                  <div className="emptyHint">暂无 nodes。</div>
                ) : (
                  nodes.map((n) => (
                    <button
                      key={n.node_id}
                      className={
                        n.node_id === selectedNodeId
                          ? 'nodeItem selected'
                          : 'nodeItem'
                      }
                      onClick={() => onSelectNode(n)}
                    >
                      <div className="nodeItemTop">
                        <span className="nodeStatus">{n.status}</span>
                        <span className="nodeId">{n.node_id}</span>
                      </div>
                      <div className="nodeTitle">{n.title}</div>
                    </button>
                  ))
                )}
              </div>
            </div>

            <div className="execList">
              {executions.length === 0 ? (
                <div className="emptyHint">点击 “Run demo” 生成一个 execution。</div>
              ) : (
                executions.map((e) => (
                  <button
                    key={e.execution_id}
                    className={
                      e.execution_id === selectedExecutionId
                        ? 'execItem selected'
                        : 'execItem'
                    }
                    onClick={() => setSelectedExecutionId(e.execution_id)}
                  >
                    <div className="execItemTop">
                      <span className="execStatus">{e.status}</span>
                      <span className="execId">{e.execution_id}</span>
                    </div>
                    <div className="execCmd">{e.command}</div>
                  </button>
                ))
              )}
            </div>
          </div>

          <div className="execMain">
            <div className="execMainHeader">
              <div className="label">Selected</div>
              <div className="value">
                <div className="selectedRow">
                  <code>{selectedExecutionId ?? '-'}</code>
                  {selectedNode && (
                    <span className="muted">
                      {selectedNode.title} ({selectedNode.node_id})
                    </span>
                  )}
                  <button
                    className="ghostBtn"
                    disabled={!selectedExecutionId}
                    onClick={onCancelSelected}
                  >
                    Cancel
                  </button>
                </div>
              </div>
            </div>
            <TerminalPane ref={terminalRef} />
          </div>
        </div>
      </section>
    </div>
  )
}

export default App
