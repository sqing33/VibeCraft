import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ChevronLeft, RefreshCcw } from 'lucide-react'
import {
  Alert,
  Button,
  Chip,
  Select,
  SelectItem,
  Skeleton,
} from '@heroui/react'

import { DAGView } from '@/components/DAGView'
import { TerminalPane, type TerminalPaneHandle } from '@/components/TerminalPane'
import { toast } from '@/lib/toast'
import {
  approveWorkflow,
  cancelNode,
  cancelWorkflow,
  fetchExecutionLogTail,
  fetchWorkflow,
  fetchWorkflowEdges,
  fetchWorkflowNodes,
  patchNode,
  patchWorkflow,
  retryNode,
  type Edge,
  type Node,
  type PublicExpert,
  type Workflow,
} from '@/lib/daemon'
import { onWsEnvelope } from '@/lib/wsBus'
import { goToLegacyWorkflows } from '@/app/routes'
import { useDaemonStore } from '@/stores/daemonStore'

type WorkflowDetailPageProps = {
  workflowId: string
}

function selectionToString(keys: unknown): string {
  if (keys === 'all') return ''
  if (keys instanceof Set) {
    const first = keys.values().next().value
    if (typeof first === 'string') return first
    if (typeof first === 'number') return String(first)
  }
  return ''
}

function formatMode(mode: string): string {
  return mode === 'auto' ? '自动' : '手动'
}

function formatStatus(status: string): string {
  if (status === 'todo') return '待开始'
  if (status === 'running') return '进行中'
  if (status === 'done') return '已完成'
  if (status === 'failed') return '失败'
  if (status === 'canceled') return '已取消'
  if (status === 'pending_approval') return '待审批'
  if (status === 'queued') return '排队中'
  if (status === 'draft') return '草稿'
  if (status === 'timeout') return '超时'
  if (status === 'succeeded') return '成功'
  if (status === 'skipped') return '已跳过'
  return status
}

function formatExpertOption(e: PublicExpert): string {
  const id = (e.id ?? '').trim()
  const label = (e.label ?? '').trim()
  if (!label || label === id) return id || '未命名'
  return `${label} (${id})`
}

function wsText(state: string): string {
  if (state === 'connected') return '已连接'
  if (state === 'connecting') return '连接中'
  return '未连接'
}

function canEditNode(n: Node): boolean {
  if (n.node_type === 'master') return false
  return (
    n.status === 'draft' ||
    n.status === 'pending_approval' ||
    n.status === 'queued' ||
    n.status === 'failed' ||
    n.status === 'canceled' ||
    n.status === 'timeout'
  )
}

function canRetryNode(n: Node): boolean {
  if (n.node_type === 'master') return false
  return n.status === 'failed' || n.status === 'canceled' || n.status === 'timeout'
}

function canCancelNode(n: Node): boolean {
  if (n.node_type === 'master') return false
  return n.status === 'running'
}

export function WorkflowDetailPage(props: WorkflowDetailPageProps) {
  const { workflowId } = props

  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const health = useDaemonStore((s) => s.health)
  const wsState = useDaemonStore((s) => s.wsState)
  const experts = useDaemonStore((s) => s.experts)

  const [workflow, setWorkflow] = useState<Workflow | null>(null)
  const [workflowLoading, setWorkflowLoading] = useState(false)
  const [workflowError, setWorkflowError] = useState<string | null>(null)

  const [nodes, setNodes] = useState<Node[]>([])
  const [edges, setEdges] = useState<Edge[]>([])
  const [graphLoading, setGraphLoading] = useState(false)
  const [graphError, setGraphError] = useState<string | null>(null)

  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const selectedNode = useMemo(
    () => (selectedNodeId ? nodes.find((n) => n.node_id === selectedNodeId) : null) ?? null,
    [nodes, selectedNodeId],
  )

  const [selectedExecutionId, setSelectedExecutionId] = useState<string | null>(null)

  const [modeSwitching, setModeSwitching] = useState(false)
  const [approving, setApproving] = useState(false)
  const [cancelingWorkflow, setCancelingWorkflow] = useState(false)

  const [nodeEditPrompt, setNodeEditPrompt] = useState('')
  const [nodeEditExpert, setNodeEditExpert] = useState('master')
  const [nodeEditSaving, setNodeEditSaving] = useState(false)

  const [nodeRetrying, setNodeRetrying] = useState(false)
  const [nodeCanceling, setNodeCanceling] = useState(false)

  const terminalRef = useRef<TerminalPaneHandle | null>(null)
  const terminalPendingRef = useRef<string>('')
  const terminalFlushRafRef = useRef<number | null>(null)
  const selectedExecutionIdRef = useRef<string | null>(null)

  const flushTerminalPending = useCallback(() => {
    if (terminalFlushRafRef.current != null) {
      window.cancelAnimationFrame(terminalFlushRafRef.current)
      terminalFlushRafRef.current = null
    }
    const chunk = terminalPendingRef.current
    if (!chunk) return
    terminalPendingRef.current = ''
    terminalRef.current?.write(chunk)
  }, [])

  const enqueueTerminalWrite = useCallback(
    (chunk: string) => {
      if (!chunk) return
      terminalPendingRef.current += chunk

      if (terminalPendingRef.current.length >= 512 * 1024) {
        flushTerminalPending()
        return
      }

      if (terminalFlushRafRef.current != null) return
      terminalFlushRafRef.current = window.requestAnimationFrame(() => {
        terminalFlushRafRef.current = null
        const data = terminalPendingRef.current
        if (!data) return
        terminalPendingRef.current = ''
        terminalRef.current?.write(data)
      })
    },
    [flushTerminalPending],
  )

  const loadTailIntoTerminal = useCallback(
    async (executionId: string) => {
      terminalPendingRef.current = ''
      if (terminalFlushRafRef.current != null) {
        window.cancelAnimationFrame(terminalFlushRafRef.current)
        terminalFlushRafRef.current = null
      }
      terminalRef.current?.reset('正在加载日志…\r\n')
      try {
        const text = await fetchExecutionLogTail(daemonUrl, executionId, 200000)
        terminalRef.current?.reset(text)
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : String(err)
        terminalRef.current?.reset(`日志加载失败：${message}\r\n`)
      }
    },
    [daemonUrl],
  )

  const refreshWorkflow = useCallback(async () => {
    setWorkflowLoading(true)
    setWorkflowError(null)
    try {
      const wf = await fetchWorkflow(daemonUrl, workflowId)
      setWorkflow(wf)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setWorkflowError(message)
    } finally {
      setWorkflowLoading(false)
    }
  }, [daemonUrl, workflowId])

  const refreshGraph = useCallback(async () => {
    setGraphLoading(true)
    setGraphError(null)
    try {
      const [ns, es] = await Promise.all([
        fetchWorkflowNodes(daemonUrl, workflowId),
        fetchWorkflowEdges(daemonUrl, workflowId),
      ])
      setNodes(ns)
      setEdges(es)

      setSelectedNodeId((prev) => {
        const still = prev ? ns.find((n) => n.node_id === prev) : null
        const next =
          still ??
          ns.find((n) => n.node_type === 'master') ??
          ns[0] ??
          null
        return next?.node_id ?? null
      })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setGraphError(message)
    } finally {
      setGraphLoading(false)
    }
  }, [daemonUrl, workflowId])

  const refreshAll = useCallback(async () => {
    await Promise.all([refreshWorkflow(), refreshGraph()])
  }, [refreshGraph, refreshWorkflow])

  useEffect(() => {
    void refreshAll()
  }, [refreshAll])

  useEffect(() => {
    if (!selectedNode) {
      setSelectedExecutionId(null)
      terminalRef.current?.reset('未选择节点。\r\n')
      return
    }

    if (selectedNode.last_execution_id) {
      setSelectedExecutionId(selectedNode.last_execution_id)
    } else {
      setSelectedExecutionId(null)
      terminalRef.current?.reset('当前节点暂无执行记录。\r\n')
    }
  }, [selectedNode])

  useEffect(() => {
    selectedExecutionIdRef.current = selectedExecutionId
  }, [selectedExecutionId])

  useEffect(() => {
    if (!selectedNode || !canEditNode(selectedNode)) {
      setNodeEditPrompt('')
      setNodeEditExpert(experts.some((e) => e.id === 'master') ? 'master' : experts[0]?.id ?? 'master')
      return
    }
    setNodeEditPrompt(selectedNode.prompt)
    setNodeEditExpert(selectedNode.expert_id || (experts.some((e) => e.id === 'master') ? 'master' : experts[0]?.id ?? 'master'))
  }, [experts, selectedNode])

  useEffect(() => {
    if (!selectedExecutionId) return
    void loadTailIntoTerminal(selectedExecutionId)
  }, [selectedExecutionId, loadTailIntoTerminal])

  useEffect(() => {
    if (wsState !== 'connected') return
    const exId = selectedExecutionIdRef.current
    if (!exId) return
    void loadTailIntoTerminal(exId)
  }, [wsState, loadTailIntoTerminal])

  useEffect(() => {
    return () => {
      terminalPendingRef.current = ''
      if (terminalFlushRafRef.current != null) {
        window.cancelAnimationFrame(terminalFlushRafRef.current)
        terminalFlushRafRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    return onWsEnvelope((env) => {
      if (env.type === 'workflow.updated') {
        const payload = env.payload as Partial<Workflow> | undefined
        const wfId =
          payload && typeof payload === 'object'
            ? (payload as { workflow_id?: unknown }).workflow_id
            : undefined
        if (typeof wfId !== 'string' || wfId !== workflowId) return
        setWorkflow(payload as Workflow)
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
        if (node.workflow_id !== workflowId) return

        setNodes((prev) => {
          if (prev.some((n) => n.node_id === node.node_id)) {
            return prev.map((n) => (n.node_id === node.node_id ? node : n))
          }
          return [...prev, node]
        })

        if (selectedNodeId === node.node_id && node.last_execution_id) {
          setSelectedExecutionId(node.last_execution_id)
        }
        return
      }

      if (env.type === 'dag.generated') {
        if (env.workflow_id === workflowId) {
          void refreshGraph()
        }
        return
      }

      if (env.type === 'node.log') {
        const exId = env.execution_id
        if (!exId || exId !== selectedExecutionIdRef.current) return
        const payload = env.payload as { chunk?: unknown } | undefined
        const chunk = typeof payload?.chunk === 'string' ? payload.chunk : ''
        enqueueTerminalWrite(chunk)
      }
    })
  }, [enqueueTerminalWrite, refreshGraph, selectedNodeId, workflowId])

  const onSwitchMode = async (nextMode: string) => {
    if (!workflow) return
    const mode = nextMode === 'auto' ? 'auto' : 'manual'
    setModeSwitching(true)
    try {
      const updated = await patchWorkflow(daemonUrl, workflow.workflow_id, { mode })
      setWorkflow(updated)
      toast({ title: '模式已更新', description: formatMode(mode) })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '模式更新失败', description: message })
    } finally {
      setModeSwitching(false)
    }
  }

  const onApproveRunnable = async () => {
    if (!workflow) return
    setApproving(true)
    try {
      const res = await approveWorkflow(daemonUrl, workflow.workflow_id)
      setWorkflow(res.workflow)
      setNodes((prev) => {
        const map = new Map(prev.map((n) => [n.node_id, n]))
        for (const n of res.nodes ?? []) map.set(n.node_id, n)
        return Array.from(map.values()).sort((a, b) => a.created_at - b.created_at)
      })
      toast({ title: '已批准可运行节点', description: `${res.nodes.length} 个节点` })
      await refreshGraph()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '审批失败', description: message })
    } finally {
      setApproving(false)
    }
  }

  const onCancelWorkflow = async () => {
    if (!workflow) return
    setCancelingWorkflow(true)
    try {
      await cancelWorkflow(daemonUrl, workflow.workflow_id)
      toast({ title: '已发起取消工作流' })
      await refreshAll()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '取消失败', description: message })
    } finally {
      setCancelingWorkflow(false)
    }
  }

  const onSaveNodeEdit = async () => {
    if (!selectedNode || !canEditNode(selectedNode)) return
    setNodeEditSaving(true)
    try {
      const updated = await patchNode(daemonUrl, selectedNode.node_id, {
        prompt: nodeEditPrompt,
        expert_id: nodeEditExpert,
      })
      setNodes((prev) => prev.map((n) => (n.node_id === updated.node_id ? updated : n)))
      toast({ title: '节点已更新', description: updated.node_id })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '保存失败', description: message })
    } finally {
      setNodeEditSaving(false)
    }
  }

  const onRetryNode = async () => {
    if (!selectedNode || !canRetryNode(selectedNode)) return
    setNodeRetrying(true)
    try {
      const res = await retryNode(daemonUrl, selectedNode.node_id)
      setWorkflow(res.workflow)
      setNodes((prev) => {
        const map = new Map(prev.map((n) => [n.node_id, n]))
        for (const n of res.nodes ?? []) map.set(n.node_id, n)
        return Array.from(map.values()).sort((a, b) => a.created_at - b.created_at)
      })
      toast({ title: '已发起重试', description: selectedNode.node_id })
      await refreshGraph()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '重试失败', description: message })
    } finally {
      setNodeRetrying(false)
    }
  }

  const onCancelNode = async () => {
    if (!selectedNode || !canCancelNode(selectedNode)) return
    setNodeCanceling(true)
    try {
      await cancelNode(daemonUrl, selectedNode.node_id)
      toast({ title: '已发起取消', description: selectedNode.node_id })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '取消节点失败', description: message })
    } finally {
      setNodeCanceling(false)
    }
  }

  if (health.status === 'error') {
    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <Button
            color="secondary"
            variant="flat"
            onPress={goToLegacyWorkflows}
            startContent={
              <ChevronLeft
                className="h-4 w-4"
                aria-hidden="true"
                focusable="false"
              />
            }
          >
            返回
          </Button>
        </div>
        <Alert
          color="danger"
          title="无法连接守护进程"
          description={health.message}
        />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="text-lg font-semibold">工作流详情</div>
          <div className="truncate text-sm text-muted-foreground">{workflowId}</div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            color="secondary"
            variant="flat"
            onPress={goToLegacyWorkflows}
            startContent={
              <ChevronLeft
                className="h-4 w-4"
                aria-hidden="true"
                focusable="false"
              />
            }
          >
            返回
          </Button>
          <Button
            color="secondary"
            variant="flat"
            onPress={() => void refreshAll()}
            isDisabled={workflowLoading || graphLoading}
            startContent={
              <RefreshCcw
                className="h-4 w-4"
                aria-hidden="true"
                focusable="false"
              />
            }
          >
            刷新
          </Button>
        </div>
      </div>

      {workflowError ? (
        <Alert
          color="danger"
          title="加载工作流失败"
          description={workflowError}
        />
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[1fr_440px]">
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border bg-card p-3">
            <div className="flex flex-wrap items-center gap-2">
              <Chip variant="flat" size="sm">
                {workflow ? formatStatus(workflow.status) : '...'}
              </Chip>
              {workflow ? (
                <Chip variant="bordered" size="sm">
                  {formatMode(workflow.mode)}
                </Chip>
              ) : null}
              <Chip variant="flat" size="sm">
                连接：{wsText(wsState)}
              </Chip>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              {workflow ? (
                <Select
                  aria-label="模式"
                  className="w-[160px]"
                  placeholder="选择模式"
                  selectionMode="single"
                  disallowEmptySelection
                  selectedKeys={
                    new Set([workflow.mode === 'auto' ? 'auto' : 'manual'])
                  }
                  onSelectionChange={(keys) =>
                    void onSwitchMode(selectionToString(keys))
                  }
                  isDisabled={modeSwitching}
                >
                  <SelectItem key="manual">手动</SelectItem>
                  <SelectItem key="auto">自动</SelectItem>
                </Select>
              ) : (
                <Skeleton className="h-9 w-[160px] rounded-md" />
              )}

              {workflow?.mode === 'manual' ? (
                <Button
                  color="primary"
                  onPress={() => void onApproveRunnable()}
                  isDisabled={approving}
                >
                  {approving ? '审批中…' : '批准可运行节点'}
                </Button>
              ) : null}

              {workflow?.status === 'running' ? (
                <Button
                  color="danger"
                  onPress={() => void onCancelWorkflow()}
                  isDisabled={cancelingWorkflow}
                >
                  {cancelingWorkflow ? '取消中…' : '取消工作流'}
                </Button>
              ) : null}
            </div>
          </div>

      {graphError ? (
            <Alert color="danger" title="加载 DAG 失败" description={graphError} />
          ) : null}

          {graphLoading && nodes.length === 0 ? (
            <Skeleton className="h-[320px] w-full" />
          ) : nodes.length === 0 ? (
            <div className="rounded-xl border bg-card p-4 text-sm text-muted-foreground">
              暂无节点。
            </div>
          ) : (
            <DAGView
              nodes={nodes}
              edges={edges}
              selectedNodeId={selectedNodeId}
              onSelectNodeId={(id) => setSelectedNodeId(id)}
            />
          )}
        </div>

        <div className="space-y-3">
          <div className="rounded-xl border bg-card p-3">
            <div className="flex items-center justify-between gap-2">
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold">
                  {selectedNode ? selectedNode.title : '节点'}
                </div>
                <div className="truncate text-xs text-muted-foreground">
                  {selectedNode ? selectedNode.node_id : '-'}
                </div>
              </div>
              {selectedNode ? (
                <Chip variant="flat" size="sm">
                  {formatStatus(selectedNode.status)}
                </Chip>
              ) : null}
            </div>

            {selectedNode ? (
              <div className="mt-3 space-y-3">
                <div className="grid grid-cols-2 gap-2 text-xs text-muted-foreground">
                  <div className="truncate">类型={selectedNode.node_type}</div>
                  <div className="truncate text-right">专家={selectedNode.expert_id}</div>
                </div>

                {canEditNode(selectedNode) ? (
                  <div className="space-y-3 rounded-lg border bg-muted/20 p-3">
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-muted-foreground">专家</div>
                      <Select
                        aria-label="专家"
                        placeholder="选择专家"
                        selectionMode="single"
                        disallowEmptySelection
                        selectedKeys={
                          nodeEditExpert ? new Set([nodeEditExpert]) : new Set([])
                        }
                        onSelectionChange={(keys) =>
                          setNodeEditExpert(selectionToString(keys))
                        }
                        isDisabled={nodeEditSaving}
                      >
                        {experts.length > 0 ? (
                          experts.map((e) => (
                            <SelectItem key={e.id}>
                              {formatExpertOption(e)}
                            </SelectItem>
                          ))
                        ) : (
                          <SelectItem key="master">主控专家（master）</SelectItem>
                        )}
                      </Select>
                    </div>

                    <div className="space-y-2">
                      <div className="text-xs font-medium text-muted-foreground">提示词</div>
                      <textarea
                        className="min-h-[120px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                        value={nodeEditPrompt}
                        onChange={(e) => setNodeEditPrompt(e.target.value)}
                        spellCheck={false}
                        disabled={nodeEditSaving}
                      />
                    </div>

                    <div className="flex items-center justify-end">
                      <Button
                        color="primary"
                        onPress={() => void onSaveNodeEdit()}
                        isDisabled={nodeEditSaving}
                      >
                        {nodeEditSaving ? '保存中…' : '保存'}
                      </Button>
                    </div>
                  </div>
                ) : null}

                <div className="flex flex-wrap gap-2">
                  {selectedNode && canCancelNode(selectedNode) ? (
                    <Button
                      color="danger"
                      onPress={() => void onCancelNode()}
                      isDisabled={nodeCanceling}
                    >
                      {nodeCanceling ? '取消中…' : '取消节点'}
                    </Button>
                  ) : null}

                  {selectedNode && canRetryNode(selectedNode) ? (
                    <Button
                      color="secondary"
                      variant="flat"
                      onPress={() => void onRetryNode()}
                      isDisabled={nodeRetrying}
                    >
                      {nodeRetrying ? '重试中…' : '重试'}
                    </Button>
                  ) : null}
                </div>

                {selectedNode.result_summary ? (
                  <div className="rounded-lg border bg-muted/20 p-3">
                    <div className="text-xs font-medium text-muted-foreground">
                      结果摘要
                    </div>
                    <pre className="mt-2 max-h-[160px] overflow-auto whitespace-pre-wrap break-words text-xs">
                      {selectedNode.result_summary}
                    </pre>
                  </div>
                ) : null}
              </div>
            ) : (
              <div className="mt-3 text-sm text-muted-foreground">
                请选择一个节点查看详情。
              </div>
            )}
          </div>

          <div className="rounded-xl border bg-card p-3">
            <div className="flex items-center justify-between gap-2">
              <div className="text-sm font-semibold">终端</div>
              <Chip variant="bordered" size="sm">
                {selectedExecutionId ?? '无执行记录'}
              </Chip>
            </div>
            <div className="mt-3">
              <TerminalPane ref={terminalRef} />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
