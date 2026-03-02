import { useCallback, useEffect, useMemo, useState } from 'react'
import { ChevronRight, Plus, Play } from 'lucide-react'
import {
  Alert,
  Button,
  Chip,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Select,
  SelectItem,
  Skeleton,
} from '@heroui/react'

import { toast } from '@/lib/toast'
import {
  createWorkflow,
  fetchWorkflows,
  startWorkflow,
  type PublicExpert,
  type Workflow,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { goToWorkflow } from '@/app/routes'
import { useDaemonStore } from '@/stores/daemonStore'
import { onWsEnvelope } from '@/lib/wsBus'

type WorkflowMode = 'manual' | 'auto'

function formatMode(mode: string): string {
  return mode === 'auto' ? '自动' : '手动'
}

function formatWorkflowStatus(status: string): string {
  if (status === 'todo') return '待开始'
  if (status === 'running') return '进行中'
  if (status === 'done') return '已完成'
  if (status === 'failed') return '失败'
  if (status === 'canceled') return '已取消'
  if (status === 'timeout') return '超时'
  return status
}

function formatExpertOption(e: PublicExpert): string {
  const id = (e.id ?? '').trim()
  const label = (e.label ?? '').trim()
  if (!label || label === id) return id || '未命名'
  return `${label} (${id})`
}

function columnKeyForStatus(status: string): 'todo' | 'running' | 'done' | 'failed' {
  if (status === 'running') return 'running'
  if (status === 'done') return 'done'
  if (status === 'failed' || status === 'canceled') return 'failed'
  return 'todo'
}

const KANBAN_COLUMNS: Array<{
  key: 'todo' | 'running' | 'done' | 'failed'
  title: string
}> = [
  { key: 'todo', title: '待开始' },
  { key: 'running', title: '进行中' },
  { key: 'done', title: '已完成' },
  { key: 'failed', title: '失败/已取消' },
]

type StartAdvancedState = {
  open: boolean
  workflowId: string | null
  expertId: string
  prompt: string
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

function EmptyKanban() {
  return (
    <div className="grid gap-3 md:grid-cols-4">
      {KANBAN_COLUMNS.map((c) => (
        <div key={c.key} className="space-y-3 rounded-xl border bg-card p-3">
          <div className="flex items-center justify-between">
            <div className="text-sm font-semibold">{c.title}</div>
            <Skeleton className="h-4 w-8 rounded-md" />
          </div>
          <div className="space-y-2">
            <Skeleton className="h-16 w-full rounded-md" />
            <Skeleton className="h-16 w-full rounded-md" />
          </div>
        </div>
      ))}
    </div>
  )
}

export function WorkflowsPage() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const health = useDaemonStore((s) => s.health)
  const experts = useDaemonStore((s) => s.experts)

  const [workflows, setWorkflows] = useState<Workflow[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [newOpen, setNewOpen] = useState(false)
  const [newTitle, setNewTitle] = useState('')
  const [newWorkspace, setNewWorkspace] = useState('.')
  const [newMode, setNewMode] = useState<WorkflowMode>('manual')
  const [creating, setCreating] = useState(false)

  const [startingId, setStartingId] = useState<string | null>(null)
  const [advanced, setAdvanced] = useState<StartAdvancedState>({
    open: false,
    workflowId: null,
    expertId: 'master',
    prompt: '',
  })

  const grouped = useMemo(() => {
    const map: Record<'todo' | 'running' | 'done' | 'failed', Workflow[]> = {
      todo: [],
      running: [],
      done: [],
      failed: [],
    }
    for (const wf of workflows) {
      map[columnKeyForStatus(wf.status)].push(wf)
    }
    return map
  }, [workflows])

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const list = await fetchWorkflows(daemonUrl)
      setWorkflows(list)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    return onWsEnvelope((env) => {
      if (env.type !== 'workflow.updated') return
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
    })
  }, [])

  const onCreate = async () => {
    setCreating(true)
    try {
      const created = await createWorkflow(daemonUrl, {
        title: newTitle.trim() ? newTitle.trim() : undefined,
        workspace_path: newWorkspace.trim() || '.',
        mode: newMode,
      })
      toast({ title: '工作流已创建', description: created.workflow_id })
      setNewTitle('')
      setNewWorkspace(created.workspace_path)
      setNewMode((created.mode as WorkflowMode) ?? 'manual')
      setNewOpen(false)
      await refresh()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '创建失败', description: message })
    } finally {
      setCreating(false)
    }
  }

  const onStart = async (workflowId: string, req?: { expert_id?: string; prompt?: string }) => {
    setStartingId(workflowId)
    try {
      const started = await startWorkflow(daemonUrl, workflowId, req)
      setWorkflows((prev) =>
        prev.map((w) =>
          w.workflow_id === started.workflow.workflow_id ? started.workflow : w,
        ),
      )
      toast({ title: '已启动', description: started.execution.execution_id })
      goToWorkflow(workflowId)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '启动失败', description: message })
    } finally {
      setStartingId(null)
    }
  }

  const openAdvanced = (workflowId: string) => {
    setAdvanced({
      open: true,
      workflowId,
      expertId: experts.some((e) => e.id === 'master')
        ? 'master'
        : experts[0]?.id ?? 'master',
      prompt: '',
    })
  }

  const onStartAdvanced = async () => {
    if (!advanced.workflowId) return
    const workflowId = advanced.workflowId
    setAdvanced((s) => ({ ...s, open: false }))
    await onStart(workflowId, {
      expert_id: advanced.expertId.trim() || undefined,
      prompt: advanced.prompt.trim() || undefined,
    })
  }

  const content =
    health.status === 'error' ? (
      <Alert color="danger" title="无法连接守护进程" description={health.message} />
    ) : error ? (
      <Alert color="danger" title="加载工作流失败" description={error} />
    ) : loading && workflows.length === 0 ? (
      <EmptyKanban />
    ) : (
      <div className="grid gap-3 md:grid-cols-4">
        {KANBAN_COLUMNS.map((col) => {
          const list = grouped[col.key]
          return (
            <div key={col.key} className="space-y-3 rounded-xl border bg-card p-3">
              <div className="flex items-center justify-between">
                <div className="text-sm font-semibold">{col.title}</div>
                <Chip variant="flat" size="sm">
                  {list.length}
                </Chip>
              </div>
              <div className="space-y-2">
                {list.length === 0 ? (
                  <div className="text-xs text-muted-foreground">暂无内容</div>
                ) : (
                  list.map((wf) => (
                    <div
                      key={wf.workflow_id}
                      className="group rounded-lg border bg-background/40 p-3 hover:bg-background/60"
                    >
                      <button
                        className="block w-full text-left"
                        onClick={() => goToWorkflow(wf.workflow_id)}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0">
                            <div className="truncate text-sm font-semibold">
                              {wf.title || '未命名'}
                            </div>
                            <div className="truncate text-xs text-muted-foreground">
                              {wf.workspace_path}
                            </div>
                          </div>
                          <div className="flex shrink-0 flex-col items-end gap-1">
                            <Chip variant="flat" size="sm">
                              {formatWorkflowStatus(wf.status)}
                            </Chip>
                            <Chip variant="bordered" size="sm">
                              {formatMode(wf.mode)}
                            </Chip>
                          </div>
                        </div>
                        <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                          <span>{formatRelativeTime(wf.updated_at)}</span>
                          <span className="inline-flex items-center gap-2">
                            <Chip variant="flat" size="sm">
                              {(typeof wf.running_nodes_count === 'number'
                                ? wf.running_nodes_count
                                : 0) + ' 运行中'}
                            </Chip>
                            <span className="inline-flex items-center gap-1">
                              <span className="hidden sm:inline">查看</span>
                              <ChevronRight className="h-4 w-4 opacity-70" />
                            </span>
                          </span>
                        </div>
                      </button>

                      {wf.status === 'todo' ? (
                        <div className="mt-3 flex flex-wrap gap-2">
                          <Button
                            color="primary"
                            size="sm"
                            isDisabled={startingId === wf.workflow_id}
                            onPress={() => void onStart(wf.workflow_id)}
                            startContent={
                              <Play className="h-4 w-4" aria-hidden="true" focusable="false" />
                            }
                          >
                            {startingId === wf.workflow_id ? '启动中…' : '启动'}
                          </Button>
                          <Button
                            size="sm"
                            variant="flat"
                            isDisabled={startingId === wf.workflow_id}
                            onPress={() => openAdvanced(wf.workflow_id)}
                          >
                            高级
                          </Button>
                        </div>
                      ) : null}
                    </div>
                  ))
                )}
              </div>
            </div>
          )
        })}
      </div>
    )

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-lg font-semibold">工作流</div>
          <div className="text-sm text-muted-foreground">
            看板概览（待开始 / 进行中 / 已完成 / 失败）
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            color="primary"
            onPress={() => setNewOpen(true)}
            startContent={<Plus className="h-4 w-4" aria-hidden="true" focusable="false" />}
          >
            新建
          </Button>

          <Modal isOpen={newOpen} onOpenChange={setNewOpen} size="lg">
            <ModalContent>
              {() => (
                <>
                  <ModalHeader>创建工作流</ModalHeader>
                  <ModalBody className="space-y-4">
                    <div className="space-y-2">
                      <div className="text-sm font-medium">标题</div>
                      <Input
                        value={newTitle}
                        onValueChange={setNewTitle}
                        placeholder="未命名"
                      />
                    </div>
                    <div className="space-y-2">
                      <div className="text-sm font-medium">工作目录</div>
                      <Input
                        value={newWorkspace}
                        onValueChange={setNewWorkspace}
                        placeholder="."
                      />
                    </div>
                    <div className="space-y-2">
                      <div className="text-sm font-medium">模式</div>
                      <Select
                        aria-label="模式"
                        placeholder="选择模式"
                        selectionMode="single"
                        disallowEmptySelection
                        selectedKeys={new Set([newMode])}
                        onSelectionChange={(keys) =>
                          setNewMode(selectionToString(keys) === 'auto' ? 'auto' : 'manual')
                        }
                      >
                        <SelectItem key="manual">手动</SelectItem>
                        <SelectItem key="auto">自动</SelectItem>
                      </Select>
                    </div>
                  </ModalBody>
                  <ModalFooter>
                    <Button
                      color="primary"
                      onPress={() => void onCreate()}
                      isDisabled={creating}
                    >
                      {creating ? '创建中…' : '创建'}
                    </Button>
                  </ModalFooter>
                </>
              )}
            </ModalContent>
          </Modal>

          <Button color="secondary" variant="flat" onPress={() => void refresh()}>
            刷新
          </Button>
        </div>
      </div>

      {content}

      <Modal
        isOpen={advanced.open}
        onOpenChange={(o) => setAdvanced((s) => ({ ...s, open: o }))}
        size="xl"
      >
        <ModalContent>
          {() => (
            <>
              <ModalHeader>高级启动</ModalHeader>
              <ModalBody className="space-y-4">
                <div className="space-y-2">
                  <div className="text-sm font-medium">主控专家</div>
                  <Select
                    aria-label="主控专家"
                    placeholder="选择专家"
                    selectionMode="single"
                    selectedKeys={advanced.expertId ? new Set([advanced.expertId]) : new Set([])}
                    onSelectionChange={(keys) =>
                      setAdvanced((s) => ({ ...s, expertId: selectionToString(keys) }))
                    }
                  >
                    {experts.length > 0 ? (
                      experts.map((e) => (
                        <SelectItem key={e.id}>{formatExpertOption(e)}</SelectItem>
                      ))
                    ) : (
                      <SelectItem key="master">主控专家（master）</SelectItem>
                    )}
                  </Select>
                </div>

                <div className="space-y-2">
                  <div className="text-sm font-medium">主控提示词（可选）</div>
                  <textarea
                    className="min-h-[120px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                    value={advanced.prompt}
                    onChange={(e) => setAdvanced((s) => ({ ...s, prompt: e.target.value }))}
                    placeholder="留空则使用默认模板"
                    spellCheck={false}
                  />
                </div>
              </ModalBody>
              <ModalFooter>
                <Button color="primary" onPress={() => void onStartAdvanced()}>
                  启动
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </div>
  )
}
