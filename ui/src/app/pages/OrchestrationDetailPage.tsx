import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Chip, Skeleton } from '@heroui/react'
import { Plus, RefreshCcw, RotateCcw, Square } from 'lucide-react'

import { goHome, goToOrchestration } from '@/app/routes'
import { LoadingVeil } from '@/app/components/LoadingVeil'
import { OrchestrationsShell } from '@/app/components/OrchestrationsShell'
import { TerminalPane, type TerminalPaneHandle } from '@/components/TerminalPane'
import {
  cancelOrchestration,
  continueOrchestration,
  fetchExecutionLogTail,
  fetchOrchestrationDetail,
  fetchOrchestrations,
  retryAgentRun,
  type AgentRun,
  type Orchestration,
  type OrchestrationArtifact,
  type SynthesisStep,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { onWsEnvelope } from '@/lib/wsBus'
import { useDaemonStore } from '@/stores/daemonStore'
import { getEmptyOrchestrationDetailCache, useOrchestrationUIStore } from '@/stores/orchestrationUIStore'

type OrchestrationDetailPageProps = {
  orchestrationId: string
}

function formatStatus(status: string): string {
  if (status === 'planning') return '规划中'
  if (status === 'running') return '进行中'
  if (status === 'done') return '已完成'
  if (status === 'failed') return '失败'
  if (status === 'canceled') return '已取消'
  if (status === 'waiting_continue') return '待继续'
  if (status === 'queued') return '排队中'
  if (status === 'succeeded') return '成功'
  if (status === 'timeout') return '超时'
  if (status === 'retryable') return '待重试'
  return status
}

function canRetry(status: string): boolean {
  return status === 'failed' || status === 'canceled' || status === 'timeout'
}

function RecentListSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
    </div>
  )
}

/**
 * 功能：展示单个编排详情，并在共享工作区壳层内保留最近编排侧栏。
 * 参数/返回：接收 orchestrationId，返回详情页与右侧日志/agent 面板。
 * 失败场景：详情加载、继续、取消或重试失败时展示错误提示。
 * 副作用：请求详情与最近编排列表、订阅 orchestration/execution 事件、拉取日志尾部。
 */
export function OrchestrationDetailPage(props: OrchestrationDetailPageProps) {
  const { orchestrationId } = props
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const wsState = useDaemonStore((s) => s.wsState)

  const recentItems = useOrchestrationUIStore((s) => s.recentItems)
  const recentLoaded = useOrchestrationUIStore((s) => s.recentLoaded)
  const recentRefreshing = useOrchestrationUIStore((s) => s.recentRefreshing)
  const recentError = useOrchestrationUIStore((s) => s.recentError)
  const setRecentState = useOrchestrationUIStore((s) => s.setRecentState)
  const currentDetailCache = useOrchestrationUIStore((s) => s.detailById[orchestrationId])
  const fallbackDetailCache = useOrchestrationUIStore((s) =>
    s.lastViewedOrchestrationId ? s.detailById[s.lastViewedOrchestrationId] : undefined,
  )
  const setDetailState = useOrchestrationUIStore((s) => s.setDetailState)
  const markLastViewedOrchestration = useOrchestrationUIStore((s) => s.markLastViewedOrchestration)

  const detailCache = currentDetailCache ?? fallbackDetailCache ?? getEmptyOrchestrationDetailCache()
  const detail = detailCache.detail
  const loading = detailCache.loading
  const error = detailCache.error

  const [canceling, setCanceling] = useState(false)
  const [continuing, setContinuing] = useState(false)
  const [retrying, setRetrying] = useState(false)
  const [selectedAgentRunId, setSelectedAgentRunId] = useState<string | null>(null)

  const terminalRef = useRef<TerminalPaneHandle | null>(null)
  const terminalPendingRef = useRef('')
  const terminalFlushRafRef = useRef<number | null>(null)
  const selectedExecutionIdRef = useRef<string | null>(null)

  const hasRecentCache = useMemo(() => recentLoaded || recentItems.length > 0, [recentItems.length, recentLoaded])
  const hasDetailCache = useMemo(() => detailCache.loaded || Boolean(detail), [detail, detailCache.loaded])

  const orchestration = detail?.orchestration ?? null
  const rounds = useMemo(() => detail?.rounds ?? [], [detail])
  const agentRuns = useMemo(() => detail?.agent_runs ?? [], [detail])
  const synthesisSteps = useMemo(() => detail?.synthesis_steps ?? [], [detail])
  const artifacts = useMemo(() => detail?.artifacts ?? [], [detail])

  const selectedAgentRun = useMemo(
    () => (selectedAgentRunId ? agentRuns.find((run) => run.agent_run_id === selectedAgentRunId) : null) ?? null,
    [agentRuns, selectedAgentRunId],
  )
  const selectedArtifacts = useMemo(
    () => artifacts.filter((artifact) => artifact.agent_run_id === selectedAgentRun?.agent_run_id),
    [artifacts, selectedAgentRun?.agent_run_id],
  )

  const refreshRecent = useCallback(async () => {
    setRecentState({ refreshing: true, error: null })
    try {
      setRecentState({ items: await fetchOrchestrations(daemonUrl), loaded: true, refreshing: false, error: null })
    } catch (err: unknown) {
      setRecentState({ refreshing: false, error: err instanceof Error ? err.message : String(err) })
    }
  }, [daemonUrl, setRecentState])

  const refresh = useCallback(async () => {
    setDetailState(orchestrationId, { loading: true, error: null })
    try {
      const next = await fetchOrchestrationDetail(daemonUrl, orchestrationId)
      setDetailState(orchestrationId, { detail: next, loading: false, loaded: true, error: null })
      markLastViewedOrchestration(orchestrationId)
      setSelectedAgentRunId((current) => current ?? next.agent_runs[0]?.agent_run_id ?? null)
    } catch (err: unknown) {
      setDetailState(orchestrationId, { loading: false, error: err instanceof Error ? err.message : String(err) })
    }
  }, [daemonUrl, markLastViewedOrchestration, orchestrationId, setDetailState])

  useEffect(() => {
    void refresh()
    void refreshRecent()
  }, [refresh, refreshRecent])

  const loadTailIntoTerminal = useCallback(
    async (executionId: string) => {
      terminalPendingRef.current = ''
      if (terminalFlushRafRef.current != null) {
        window.cancelAnimationFrame(terminalFlushRafRef.current)
        terminalFlushRafRef.current = null
      }
      terminalRef.current?.reset('正在加载日志…\r\n')
      try {
        terminalRef.current?.reset(await fetchExecutionLogTail(daemonUrl, executionId))
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : String(err)
        terminalRef.current?.reset(`日志加载失败：${message}\r\n`)
      }
    },
    [daemonUrl],
  )

  useEffect(() => {
    const nextExecutionId = selectedAgentRun?.last_execution_id ?? null
    selectedExecutionIdRef.current = nextExecutionId
    if (nextExecutionId) {
      void loadTailIntoTerminal(nextExecutionId)
      return
    }
    terminalRef.current?.reset('选择一个 agent 查看日志…\r\n')
  }, [loadTailIntoTerminal, selectedAgentRun?.last_execution_id])

  useEffect(() => {
    return onWsEnvelope((env) => {
      if (env.type.startsWith('orchestration.') && env.orchestration_id === orchestrationId) {
        void refresh()
        void refreshRecent()
        return
      }
      if (env.type === 'orchestration.updated') {
        const payload = env.payload as Partial<Orchestration> | undefined
        const nextId = payload && typeof payload === 'object' ? (payload as { orchestration_id?: unknown }).orchestration_id : undefined
        if (typeof nextId === 'string') {
          const next = payload as Orchestration
          const currentItems = useOrchestrationUIStore.getState().recentItems
          const merged = currentItems.some((item) => item.orchestration_id === next.orchestration_id)
            ? currentItems.map((item) => (item.orchestration_id === next.orchestration_id ? next : item))
            : [next, ...currentItems]
          setRecentState({ items: merged.sort((a, b) => b.updated_at - a.updated_at), loaded: true })
        }
        return
      }
      if (env.type === 'node.log' && env.execution_id === selectedExecutionIdRef.current) {
        const payload = env.payload as { chunk?: unknown } | undefined
        const chunk = typeof payload?.chunk === 'string' ? payload.chunk : ''
        if (!chunk) return
        terminalPendingRef.current += chunk
        if (terminalFlushRafRef.current != null) return
        terminalFlushRafRef.current = window.requestAnimationFrame(() => {
          terminalFlushRafRef.current = null
          const data = terminalPendingRef.current
          if (!data) return
          terminalPendingRef.current = ''
          terminalRef.current?.write(data)
        })
        return
      }
      if (env.type === 'execution.exited' && env.execution_id === selectedExecutionIdRef.current) {
        void refresh()
      }
    })
  }, [orchestrationId, refresh, refreshRecent, setRecentState])

  useEffect(() => {
    return () => {
      terminalPendingRef.current = ''
      if (terminalFlushRafRef.current != null) {
        window.cancelAnimationFrame(terminalFlushRafRef.current)
      }
    }
  }, [])

  useEffect(() => {
    if (wsState !== 'connected') return
    const executionId = selectedExecutionIdRef.current
    if (!executionId) return
    void loadTailIntoTerminal(executionId)
  }, [loadTailIntoTerminal, wsState])

  const onCancel = async () => {
    if (!orchestration) return
    setCanceling(true)
    try {
      await cancelOrchestration(daemonUrl, orchestration.orchestration_id)
      toast({ title: 'Orchestration 已取消', description: orchestration.title })
      await refresh()
      await refreshRecent()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '取消失败', description: message })
    } finally {
      setCanceling(false)
    }
  }

  const onRetry = async () => {
    if (!selectedAgentRun) return
    setRetrying(true)
    try {
      await retryAgentRun(daemonUrl, selectedAgentRun.agent_run_id)
      toast({ title: 'Agent 已重试', description: selectedAgentRun.title })
      await refresh()
      await refreshRecent()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '重试失败', description: message })
    } finally {
      setRetrying(false)
    }
  }

  const onContinue = async () => {
    if (!orchestration) return
    setContinuing(true)
    try {
      const next = await continueOrchestration(daemonUrl, orchestration.orchestration_id)
      setDetailState(orchestrationId, { detail: next, loaded: true, loading: false, error: null })
      markLastViewedOrchestration(orchestrationId)
      toast({ title: '已进入下一轮', description: `Round ${next.orchestration.current_round}` })
      await refreshRecent()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '继续失败', description: message })
    } finally {
      setContinuing(false)
    }
  }

  const synthesisByRoundId = useMemo(() => {
    const map = new Map<string, SynthesisStep>()
    for (const item of synthesisSteps) map.set(item.round_id, item)
    return map
  }, [synthesisSteps])

  const runsByRoundId = useMemo(() => {
    const map = new Map<string, AgentRun[]>()
    for (const run of agentRuns) {
      const list = map.get(run.round_id) ?? []
      list.push(run)
      map.set(run.round_id, list)
    }
    return map
  }, [agentRuns])

  const sidebarContent = (
    <div className="relative min-h-[120px]">
      {recentError && !hasRecentCache ? <Alert color="danger" title="加载失败" description={recentError} /> : null}
      {!hasRecentCache && recentRefreshing ? (
        <RecentListSkeleton />
      ) : recentItems.length === 0 ? (
        <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">
          还没有编排，先返回首页创建一个新的工作流任务。
        </div>
      ) : (
        <div className="space-y-2">
          {recentItems.map((item) => (
            <button
              key={item.orchestration_id}
              type="button"
              className={`w-full rounded-[22px] border px-3 py-3 text-left transition ${
                item.orchestration_id === orchestrationId
                  ? 'border-primary/50 bg-primary/5 shadow-sm'
                  : 'border-transparent bg-background/40 hover:border-default-200 hover:bg-background/80'
              }`}
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
      title={orchestration?.title || '工作流详情'}
      headerMeta={
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span>Orchestration</span>
          <span>·</span>
          <span>{orchestration?.orchestration_id || orchestrationId}</span>
          {orchestration ? <Chip variant="flat" size="sm">{formatStatus(orchestration.status)}</Chip> : null}
        </div>
      }
      headerActions={
        <>
          <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void refresh()}>
            刷新
          </Button>
          {orchestration?.status === 'waiting_continue' ? (
            <Button color="primary" variant="flat" size="sm" isLoading={continuing} onPress={onContinue}>
              继续下一轮
            </Button>
          ) : null}
          {orchestration?.status === 'running' ? (
            <Button color="danger" variant="flat" size="sm" startContent={<Square className="h-4 w-4" />} isLoading={canceling} onPress={onCancel}>
              取消
            </Button>
          ) : null}
        </>
      }
      sidebarTitle="最近编排"
      sidebarCount={recentItems.length}
      sidebarAction={
        <Button
          color="primary"
          size="sm"
          className="w-[25%] min-w-[86px] rounded-2xl"
          startContent={<Plus className="h-4 w-4 shrink-0 stroke-[3]" />}
          onPress={goHome}
        >
          新建编排
        </Button>
      }
      sidebarContent={sidebarContent}
      contentMaxWidthClassName="max-w-[1200px]"
    >
      <div className="relative">
        {recentError && hasRecentCache ? <Alert color="danger" title="刷新失败，已保留上次编排列表" description={recentError} className="mb-4" /> : null}
        {error && hasDetailCache ? <Alert color="danger" title="刷新失败，已保留上次详情内容" description={error} className="mb-4" /> : null}

        {!hasDetailCache && loading ? (
          <div className="space-y-3">
            <Skeleton className="h-28 w-full rounded-xl" />
            <Skeleton className="h-80 w-full rounded-xl" />
          </div>
        ) : !detail ? null : (
          <div className="grid gap-4 xl:grid-cols-[1.5fr_1fr]">
            <div className="space-y-4">
              <section className="rounded-2xl border bg-card p-5 shadow-sm">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="text-lg font-semibold">{orchestration?.title}</div>
                    <div className="mt-1 text-sm text-muted-foreground">{orchestration?.goal}</div>
                  </div>
                  <Chip variant="flat">{formatStatus(orchestration?.status ?? '')}</Chip>
                </div>
                <div className="mt-3 flex flex-wrap gap-3 text-xs text-muted-foreground">
                  <span>Workspace：{orchestration?.workspace_path}</span>
                  <span>当前轮次：{orchestration?.current_round}</span>
                  <span>运行中 Agent：{orchestration?.running_agent_runs_count ?? 0}</span>
                </div>
                {orchestration?.summary ? (
                  <div className="mt-3 rounded-xl bg-muted/20 p-3 text-sm text-muted-foreground whitespace-pre-wrap">{orchestration.summary}</div>
                ) : null}
              </section>

              {rounds.map((round) => {
                const roundRuns = runsByRoundId.get(round.round_id) ?? []
                const synthesis = synthesisByRoundId.get(round.round_id)
                return (
                  <section key={round.round_id} className="rounded-2xl border bg-card p-4 shadow-sm">
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div>
                        <div className="font-semibold">Round {round.round_index}</div>
                        <div className="text-sm text-muted-foreground">{round.goal}</div>
                      </div>
                      <Chip variant="flat" size="sm">{formatStatus(round.status)}</Chip>
                    </div>

                    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                      {roundRuns.map((run) => (
                        <button
                          key={run.agent_run_id}
                          type="button"
                          className={`rounded-xl border p-3 text-left transition ${selectedAgentRunId === run.agent_run_id ? 'border-primary bg-primary/5' : 'hover:border-primary/40 hover:bg-muted/20'}`}
                          onClick={() => setSelectedAgentRunId(run.agent_run_id)}
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div>
                              <div className="text-sm font-semibold">{run.title}</div>
                              <div className="mt-1 text-xs text-muted-foreground">{run.role} · {run.expert_id}</div>
                            </div>
                            <Chip size="sm" variant="flat">{formatStatus(run.status)}</Chip>
                          </div>
                          <div className="mt-3 line-clamp-3 text-sm text-muted-foreground">{run.result_summary || run.goal}</div>
                          <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
                            <span>{run.intent}</span>
                            <span>{run.workspace_mode}</span>
                            <span>{run.modified_code ? '改过代码' : '未改代码'}</span>
                          </div>
                        </button>
                      ))}
                    </div>

                    {synthesis ? (
                      <div className="mt-4 rounded-xl border bg-muted/10 p-3">
                        <div className="text-sm font-semibold">Synthesis · {synthesis.decision}</div>
                        <div className="mt-2 whitespace-pre-wrap text-sm text-muted-foreground">{synthesis.summary}</div>
                      </div>
                    ) : null}
                  </section>
                )
              })}
            </div>

            <aside className="space-y-4">
              <section className="rounded-2xl border bg-card p-4 shadow-sm">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="font-semibold">Agent 详情</div>
                    <div className="text-sm text-muted-foreground">选择左侧 agent 查看摘要、artifact 和日志。</div>
                  </div>
                  {selectedAgentRun && canRetry(selectedAgentRun.status) ? (
                    <Button size="sm" variant="flat" startContent={<RotateCcw className="h-4 w-4" />} isLoading={retrying} onPress={onRetry}>
                      重试
                    </Button>
                  ) : null}
                </div>

                {selectedAgentRun ? (
                  <div className="mt-4 space-y-3 text-sm">
                    <div>
                      <div className="font-semibold">{selectedAgentRun.title}</div>
                      <div className="text-muted-foreground">{selectedAgentRun.goal}</div>
                    </div>
                    <div className="grid gap-2 text-xs text-muted-foreground">
                      <div>角色：{selectedAgentRun.role}</div>
                      <div>状态：{formatStatus(selectedAgentRun.status)}</div>
                      <div>Expert：{selectedAgentRun.expert_id}</div>
                      <div>Intent：{selectedAgentRun.intent}</div>
                      <div>Workspace：{selectedAgentRun.workspace_mode} · {selectedAgentRun.workspace_path}</div>
                      {selectedAgentRun.branch_name ? <div>Branch：{selectedAgentRun.branch_name}</div> : null}
                      {selectedAgentRun.base_ref ? <div>Base Ref：{selectedAgentRun.base_ref}</div> : null}
                      {selectedAgentRun.worktree_path ? <div>Worktree：{selectedAgentRun.worktree_path}</div> : null}
                      <div>代码变更：{selectedAgentRun.modified_code ? '是' : '否'}</div>
                    </div>
                    {selectedAgentRun.result_summary ? (
                      <div className="rounded-xl bg-muted/20 p-3 whitespace-pre-wrap text-xs text-muted-foreground">{selectedAgentRun.result_summary}</div>
                    ) : null}
                    {selectedAgentRun.error_message ? (
                      <Alert color="danger" title="执行错误" description={selectedAgentRun.error_message} />
                    ) : null}
                    {selectedArtifacts.length > 0 ? (
                      <div className="space-y-2">
                        <div className="text-xs font-semibold text-muted-foreground">Artifacts</div>
                        {selectedArtifacts.map((artifact: OrchestrationArtifact) => (
                          <div key={artifact.artifact_id} className="rounded-xl border p-3 text-xs">
                            <div className="font-semibold">{artifact.title}</div>
                            <div className="mt-1 text-muted-foreground">{artifact.kind}</div>
                            {artifact.summary ? <div className="mt-2 whitespace-pre-wrap text-muted-foreground">{artifact.summary}</div> : null}
                          </div>
                        ))}
                      </div>
                    ) : null}
                  </div>
                ) : (
                  <div className="mt-4 text-sm text-muted-foreground">请选择一个 agent。</div>
                )}
              </section>

              <section className="rounded-2xl border bg-card p-4 shadow-sm">
                <div className="mb-3 text-sm font-semibold">日志</div>
                <TerminalPane ref={terminalRef} />
              </section>
            </aside>
          </div>
        )}

        <LoadingVeil visible={loading && hasDetailCache} label="正在刷新编排详情…" />
      </div>
    </OrchestrationsShell>
  )
}
