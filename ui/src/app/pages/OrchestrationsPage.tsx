import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Chip, Input, Skeleton, Textarea } from '@heroui/react'
import { Play, Sparkles } from 'lucide-react'

import { goToOrchestration } from '@/app/routes'
import {
  createOrchestration,
  fetchOrchestrations,
  type Orchestration,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { onWsEnvelope } from '@/lib/wsBus'
import { useDaemonStore } from '@/stores/daemonStore'

function formatStatus(status: string): string {
  if (status === 'planning') return '规划中'
  if (status === 'running') return '进行中'
  if (status === 'done') return '已完成'
  if (status === 'failed') return '失败'
  if (status === 'canceled') return '已取消'
  if (status === 'waiting_continue') return '待继续'
  return status
}

function EmptyList() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-24 w-full rounded-xl" />
      <Skeleton className="h-24 w-full rounded-xl" />
      <Skeleton className="h-24 w-full rounded-xl" />
    </div>
  )
}

export function OrchestrationsPage() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const health = useDaemonStore((s) => s.health)

  const [items, setItems] = useState<Orchestration[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [goal, setGoal] = useState('')
  const [title, setTitle] = useState('')
  const [workspace, setWorkspace] = useState('.')
  const [creating, setCreating] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      setItems(await fetchOrchestrations(daemonUrl))
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    return onWsEnvelope((env) => {
      if (env.type !== 'orchestration.updated') return
      const payload = env.payload as Partial<Orchestration> | undefined
      const orchestrationId =
        payload && typeof payload === 'object'
          ? (payload as { orchestration_id?: unknown }).orchestration_id
          : undefined
      if (typeof orchestrationId !== 'string') return
      const next = payload as Orchestration
      setItems((prev) => {
        const merged = prev.some((item) => item.orchestration_id === next.orchestration_id)
          ? prev.map((item) => (item.orchestration_id === next.orchestration_id ? next : item))
          : [next, ...prev]
        return merged.sort((a, b) => b.updated_at - a.updated_at)
      })
    })
  }, [])

  const onCreate = async () => {
    const trimmedGoal = goal.trim()
    if (!trimmedGoal) {
      toast({ variant: 'destructive', title: '请输入任务目标' })
      return
    }

    setCreating(true)
    try {
      const detail = await createOrchestration(daemonUrl, {
        title: title.trim() || undefined,
        goal: trimmedGoal,
        workspace_path: workspace.trim() || '.',
      })
      setGoal('')
      setTitle('')
      setWorkspace(detail.orchestration.workspace_path)
      toast({ title: 'Orchestration 已启动', description: detail.orchestration.orchestration_id })
      goToOrchestration(detail.orchestration.orchestration_id)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '启动失败', description: message })
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border bg-card p-5 shadow-sm">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-lg font-semibold">
              <Sparkles className="h-5 w-5" />
              项目编排 Orchestrations
            </div>
            <div className="mt-1 text-sm text-muted-foreground">
              直接输入一个项目开发目标，系统会生成第一轮并行 agent 并开始执行。
            </div>
          </div>
          <Chip variant="flat" color={health.status === 'ok' ? 'success' : 'default'}>
            {health.status === 'ok' ? 'Daemon 正常' : 'Daemon 未就绪'}
          </Chip>
        </div>

        <div className="grid gap-3 md:grid-cols-[1fr_220px]">
          <Input label="可选标题" placeholder="例如：登录页优化 + 设置页重构" value={title} onValueChange={setTitle} />
          <Input label="Workspace" placeholder="." value={workspace} onValueChange={setWorkspace} />
        </div>

        <div className="mt-3">
          <Textarea
            label="任务目标"
            placeholder="例如：帮我同时做登录页优化和设置页重构"
            minRows={4}
            value={goal}
            onValueChange={setGoal}
          />
        </div>

        <div className="mt-4 flex justify-end">
          <Button color="primary" startContent={<Play className="h-4 w-4" />} isLoading={creating} onPress={onCreate}>
            启动 Orchestration
          </Button>
        </div>
      </section>

      <section className="space-y-3">
        <div className="text-sm font-semibold text-muted-foreground">最近编排</div>
        {error ? (
          <Alert color="danger" title="加载失败" description={error} />
        ) : loading && items.length === 0 ? (
          <EmptyList />
        ) : items.length === 0 ? (
          <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
            还没有 orchestration，先从上面的输入区发起一次项目开发任务。
          </div>
        ) : (
          <div className="space-y-3">
            {items.map((item) => (
              <button
                key={item.orchestration_id}
                type="button"
                className="w-full rounded-2xl border bg-card p-4 text-left transition hover:border-primary/50 hover:bg-muted/20"
                onClick={() => goToOrchestration(item.orchestration_id)}
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="text-base font-semibold">{item.title}</div>
                    <div className="mt-1 line-clamp-2 text-sm text-muted-foreground">{item.goal}</div>
                  </div>
                  <Chip variant="flat" size="sm">{formatStatus(item.status)}</Chip>
                </div>
                <div className="mt-3 flex flex-wrap gap-3 text-xs text-muted-foreground">
                  <span>Workspace：{item.workspace_path}</span>
                  <span>轮次：{item.current_round}</span>
                  <span>运行中 Agent：{item.running_agent_runs_count ?? 0}</span>
                  <span>更新于：{formatRelativeTime(item.updated_at)}</span>
                </div>
                {item.summary ? <div className="mt-3 line-clamp-2 text-xs text-muted-foreground">{item.summary}</div> : null}
              </button>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
