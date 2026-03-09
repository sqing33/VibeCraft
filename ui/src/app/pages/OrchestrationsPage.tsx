import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Chip, Input, Skeleton, Textarea } from '@heroui/react'
import { Play, Plus, RefreshCcw, Sparkles } from 'lucide-react'

import { goToOrchestration } from '@/app/routes'
import { LoadingVeil } from '@/app/components/LoadingVeil'
import { OrchestrationsShell } from '@/app/components/OrchestrationsShell'
import {
  createOrchestration,
  fetchOrchestrations,
  type Orchestration,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { onWsEnvelope } from '@/lib/wsBus'
import { useDaemonStore } from '@/stores/daemonStore'
import { useOrchestrationUIStore } from '@/stores/orchestrationUIStore'

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

/**
 * 功能：展示 Orchestrations 首页，并允许用户发起新的编排任务。
 * 参数/返回：无入参；返回编排首页、创建表单与最近编排列表。
 * 失败场景：列表加载或创建 orchestration 失败时展示错误提示，并允许用户重试。
 * 副作用：发起 orchestration 列表查询、创建请求，并订阅 orchestration.updated 事件更新列表。
 */
export function OrchestrationsPage() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const health = useDaemonStore((s) => s.health)

  const items = useOrchestrationUIStore((s) => s.recentItems)
  const recentLoaded = useOrchestrationUIStore((s) => s.recentLoaded)
  const recentRefreshing = useOrchestrationUIStore((s) => s.recentRefreshing)
  const error = useOrchestrationUIStore((s) => s.recentError)
  const setRecentState = useOrchestrationUIStore((s) => s.setRecentState)

  const [goal, setGoal] = useState('')
  const [title, setTitle] = useState('')
  const [workspace, setWorkspace] = useState('.')
  const [creating, setCreating] = useState(false)

  const hasRecentCache = useMemo(() => recentLoaded || items.length > 0, [items.length, recentLoaded])

  const refresh = useCallback(async () => {
    setRecentState({ refreshing: true, error: null })
    try {
      setRecentState({ items: await fetchOrchestrations(daemonUrl), loaded: true, refreshing: false, error: null })
    } catch (err: unknown) {
      setRecentState({ refreshing: false, error: err instanceof Error ? err.message : String(err) })
    }
  }, [daemonUrl, setRecentState])

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
      setRecentState({
        items: (() => {
          const merged = items.some((item) => item.orchestration_id === next.orchestration_id)
            ? items.map((item) => (item.orchestration_id === next.orchestration_id ? next : item))
            : [next, ...items]
          return merged.sort((a, b) => b.updated_at - a.updated_at)
        })(),
        loaded: true,
      })
    })
  }, [items, setRecentState])

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
      setRecentState({
        items: [detail.orchestration, ...items.filter((item) => item.orchestration_id !== detail.orchestration.orchestration_id)].sort(
          (a, b) => b.updated_at - a.updated_at,
        ),
        loaded: true,
      })
      toast({ title: 'Orchestration 已启动', description: detail.orchestration.orchestration_id })
      goToOrchestration(detail.orchestration.orchestration_id)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '启动失败', description: message })
    } finally {
      setCreating(false)
    }
  }

  const sidebarContent = (
    <div className="relative min-h-[120px]">
      {error && !hasRecentCache ? <Alert color="danger" title="加载失败" description={error} /> : null}
      {!hasRecentCache && recentRefreshing ? (
        <div className="space-y-2">
          <Skeleton className="h-[58px] w-full rounded-[22px]" />
          <Skeleton className="h-[58px] w-full rounded-[22px]" />
          <Skeleton className="h-[58px] w-full rounded-[22px]" />
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">
          还没有编排，先在右侧创建一个新的工作流任务。
        </div>
      ) : (
        <div className="space-y-2">
          {items.map((item) => (
            <button
              key={item.orchestration_id}
              type="button"
              className="w-full rounded-[22px] border px-3 py-3 text-left transition hover:border-default-200 hover:bg-background/80"
              onClick={() => goToOrchestration(item.orchestration_id)}
            >
              <div className="flex items-center justify-between gap-2">
                <div className="min-w-0 truncate text-sm font-medium">
                  {item.title || `编排 ${item.orchestration_id.slice(0, 8)}`}
                </div>
                <span className="shrink-0 text-xs text-muted-foreground">{formatRelativeTime(item.updated_at)}</span>
              </div>
              <div className="mt-1 truncate text-xs text-muted-foreground">
                {item.goal || item.workspace_path || item.orchestration_id}
              </div>
            </button>
          ))}
        </div>
      )}
      <LoadingVeil visible={recentRefreshing && hasRecentCache} compact label="正在刷新编排列表…" />
    </div>
  )

  return (
    <OrchestrationsShell
      title="工作流"
      headerMeta={
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span>Orchestrations</span>
          <span>·</span>
          <span>{items.length} 个编排</span>
          <Chip variant="flat" color={health.status === 'ok' ? 'success' : 'default'} size="sm">
            {health.status === 'ok' ? 'Daemon 正常' : 'Daemon 未就绪'}
          </Chip>
        </div>
      }
      headerActions={
        <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void refresh()}>
          刷新
        </Button>
      }
      sidebarTitle="最近编排"
      sidebarCount={items.length}
      sidebarAction={
        <Button
          color="primary"
          size="sm"
          className="w-[25%] min-w-[86px] rounded-2xl"
          startContent={<Plus className="h-4 w-4 shrink-0 stroke-[3]" />}
          onPress={() => {
            document.getElementById('orchestrations-create-form')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
            window.requestAnimationFrame(() => {
              const input = document.getElementById('orchestration-goal') as HTMLTextAreaElement | null
              input?.focus()
            })
          }}
        >
          新建编排
        </Button>
      }
      sidebarContent={sidebarContent}
    >
      <div className="relative">
        {error && hasRecentCache ? <Alert color="danger" title="刷新失败，已保留上次内容" description={error} className="mb-4" /> : null}

        <section id="orchestrations-create-form" className="rounded-2xl border bg-card p-5 shadow-sm">
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
          </div>

          <div className="grid gap-3 md:grid-cols-[1fr_220px]">
            <Input label="可选标题" placeholder="例如：登录页优化 + 设置页重构" value={title} onValueChange={setTitle} />
            <Input label="Workspace" placeholder="." value={workspace} onValueChange={setWorkspace} />
          </div>

          <div className="mt-3">
            <Textarea
              id="orchestration-goal"
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

        <section className="mt-5 space-y-3">
          <div className="text-sm font-semibold text-muted-foreground">最近编排</div>
          {!hasRecentCache && recentRefreshing ? (
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

        <LoadingVeil visible={recentRefreshing && hasRecentCache} label="正在同步编排内容…" />
      </div>
    </OrchestrationsShell>
  )
}
