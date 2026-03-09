import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Chip, Skeleton } from '@heroui/react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { FileSearch, Plus, RefreshCcw, ScrollText } from 'lucide-react'

import {
  goToChat,
  goToRepoLibraryPatternSearch,
  goToRepoLibraryRepositories,
  goToRepoLibraryRepository,
} from '@/app/routes'
import { LoadingVeil } from '@/app/components/LoadingVeil'
import { RepoLibraryShell, RepoLibrarySidebarRepositoryItem } from '@/app/components/RepoLibraryShell'
import { TerminalPane, type TerminalPaneHandle } from '@/components/TerminalPane'
import {
  fetchExecutionLogTail,
  fetchRepoLibraryCard,
  fetchRepoLibraryCardEvidence,
  fetchRepoLibraryCards,
  fetchRepoLibraryRepositories,
  fetchRepoLibraryRepository,
  fetchRepoLibrarySnapshotReport,
  fetchRepoLibrarySnapshots,
  syncRepoLibraryAnalysisChat,
  type RepoLibraryAnalysisRun,
  type RepoLibraryCard,
  type RepoLibraryCardEvidence,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'
import { getEmptyRepoLibraryDetailCache, useRepoLibraryUIStore } from '@/stores/repoLibraryUIStore'

type RepoLibraryRepositoryDetailPageProps = {
  repositoryId: string
}

function formatAnalysisStatus(status: string): string {
  if (status === 'queued') return '排队中'
  if (status === 'running') return '分析中'
  if (status === 'succeeded') return '已完成'
  if (status === 'failed') return '失败'
  return status || '未知'
}

function analysisStatusColor(status: string): 'default' | 'success' | 'danger' | 'warning' {
  if (status === 'succeeded') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'queued' || status === 'running') return 'warning'
  return 'default'
}

function formatCardType(type: string): string {
  if (type === 'project_characteristic') return '项目特征'
  if (type === 'feature_pattern') return '功能模式'
  if (type === 'risk_note') return '风险提示'
  if (type === 'integration_note') return '集成提示'
  return type || '卡片'
}

function DetailSkeleton() {
  return (
    <div className="space-y-4">
      <Skeleton className="h-36 w-full rounded-2xl" />
      <Skeleton className="h-72 w-full rounded-2xl" />
      <Skeleton className="h-72 w-full rounded-2xl" />
    </div>
  )
}

/**
 * 功能：展示单个仓库的快照、分析运行、报告、知识卡片与证据明细。
 * 参数/返回：接收 repositoryId；返回仓库详情页。
 * 失败场景：详情、快照、卡片或证据请求失败时展示错误提示，并保留刷新入口。
 * 副作用：发起仓库详情/快照/卡片/evidence 请求，并在选择分析运行时读取 execution 日志尾部。
 */
export function RepoLibraryRepositoryDetailPage(props: RepoLibraryRepositoryDetailPageProps) {
  const { repositoryId } = props
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)

  const repositories = useRepoLibraryUIStore((s) => s.repositories)
  const repositoriesLoaded = useRepoLibraryUIStore((s) => s.repositoriesLoaded)
  const repositoriesLoading = useRepoLibraryUIStore((s) => s.repositoriesRefreshing)
  const repositoriesError = useRepoLibraryUIStore((s) => s.repositoriesError)
  const setRepositoriesState = useRepoLibraryUIStore((s) => s.setRepositoriesState)
  const currentDetailCache = useRepoLibraryUIStore((s) => s.detailsByRepositoryId[repositoryId])
  const fallbackDetailCache = useRepoLibraryUIStore((s) =>
    s.lastViewedRepositoryId ? s.detailsByRepositoryId[s.lastViewedRepositoryId] : undefined,
  )
  const setRepositoryDetailState = useRepoLibraryUIStore((s) => s.setRepositoryDetailState)
  const markLastViewedRepository = useRepoLibraryUIStore((s) => s.markLastViewedRepository)

  const detailCache = currentDetailCache ?? fallbackDetailCache ?? getEmptyRepoLibraryDetailCache()
  const detail = detailCache.detail
  const snapshots = detailCache.snapshots
  const cards = detailCache.cards
  const selectedSnapshotId = detailCache.selectedSnapshotId
  const selectedAnalysisId = detailCache.selectedAnalysisId
  const selectedCardId = detailCache.selectedCardId
  const loading = detailCache.loading
  const error = detailCache.error
  const cardsLoading = detailCache.cardsLoading
  const cardsError = detailCache.cardsError
  const cardLoading = detailCache.cardLoading
  const cardError = detailCache.cardError
  const selectedCard = detailCache.selectedCard
  const evidence = detailCache.evidence
  const reportMarkdown = detailCache.reportMarkdown

  const [syncingAnalysisId, setSyncingAnalysisId] = useState<string | null>(null)

  const terminalRef = useRef<TerminalPaneHandle | null>(null)

  const hasRepositoryCache = useMemo(
    () => repositoriesLoaded || repositories.length > 0,
    [repositories.length, repositoriesLoaded],
  )
  const hasDetailCache = useMemo(
    () => detailCache.loaded || Boolean(detail),
    [detail, detailCache.loaded],
  )

  const refresh = useCallback(async (options?: { force?: boolean }) => {
    const force = options?.force ?? false
    const state = useRepoLibraryUIStore.getState()
    const detailState = state.detailsByRepositoryId[repositoryId]
    const fallbackRepositories = state.repositories
    const previousSnapshotId = detailState?.selectedSnapshotId ?? null
    const previousAnalysisId = detailState?.selectedAnalysisId ?? null

    if (!force && detailState?.loading) return

    setRepositoryDetailState(repositoryId, { loading: true, error: null })
    setRepositoriesState({ refreshing: true, error: null })
    try {
      const [nextDetail, nextSnapshots, nextRepositories] = await Promise.all([
        fetchRepoLibraryRepository(daemonUrl, repositoryId),
        fetchRepoLibrarySnapshots(daemonUrl, repositoryId),
        fetchRepoLibraryRepositories(daemonUrl).catch(() => fallbackRepositories),
      ])

      setRepositoriesState({
        repositories: nextRepositories,
        loaded: true,
        refreshing: false,
        error: null,
      })

      const analysisRuns = nextDetail.analysis_runs ?? []
      setRepositoryDetailState(repositoryId, {
        detail: nextDetail,
        snapshots: nextSnapshots,
        loading: false,
        loaded: true,
        error: null,
        selectedSnapshotId:
          previousSnapshotId && nextSnapshots.some((item) => item.snapshot_id === previousSnapshotId)
            ? previousSnapshotId
            : nextDetail.latest_snapshot?.snapshot_id || nextSnapshots[0]?.snapshot_id || null,
        selectedAnalysisId:
          previousAnalysisId && analysisRuns.some((item) => item.analysis_id === previousAnalysisId)
            ? previousAnalysisId
            : nextDetail.latest_analysis?.analysis_id || analysisRuns[0]?.analysis_id || null,
      })
      markLastViewedRepository(repositoryId)
    } catch (err: unknown) {
      setRepositoryDetailState(repositoryId, {
        loading: false,
        error: err instanceof Error ? err.message : String(err),
      })
      setRepositoriesState({ refreshing: false })
    }
  }, [daemonUrl, markLastViewedRepository, repositoryId, setRepositoriesState, setRepositoryDetailState])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    let cancelled = false

    const loadCards = async () => {
      if (!selectedSnapshotId) return
      setRepositoryDetailState(repositoryId, { cardsLoading: true, cardsError: null })
      try {
        const next = await fetchRepoLibraryCards(daemonUrl, {
          repository_id: repositoryId,
          snapshot_id: selectedSnapshotId,
          limit: 100,
        })
        if (cancelled) return
        const currentSelectedCardId =
          useRepoLibraryUIStore.getState().detailsByRepositoryId[repositoryId]?.selectedCardId ?? null
        setRepositoryDetailState(repositoryId, {
          cards: next,
          cardsLoading: false,
          cardsError: null,
          selectedCardId:
            currentSelectedCardId && next.some((item) => item.card_id === currentSelectedCardId)
              ? currentSelectedCardId
              : next[0]?.card_id ?? null,
        })
      } catch (err: unknown) {
        if (cancelled) return
        setRepositoryDetailState(repositoryId, {
          cardsLoading: false,
          cardsError: err instanceof Error ? err.message : String(err),
        })
      }
    }

    void loadCards()
    return () => {
      cancelled = true
    }
  }, [daemonUrl, repositoryId, selectedSnapshotId, setRepositoryDetailState])

  useEffect(() => {
    let cancelled = false

    const loadReport = async () => {
      if (!selectedSnapshotId) {
        setRepositoryDetailState(repositoryId, { reportMarkdown: '' })
        return
      }
      try {
        const next = await fetchRepoLibrarySnapshotReport(daemonUrl, selectedSnapshotId)
        if (cancelled) return
        setRepositoryDetailState(repositoryId, { reportMarkdown: next.report_markdown })
      } catch {
        if (cancelled) return
        setRepositoryDetailState(repositoryId, { reportMarkdown: '' })
      }
    }

    void loadReport()
    return () => {
      cancelled = true
    }
  }, [daemonUrl, repositoryId, selectedSnapshotId, setRepositoryDetailState])

  useEffect(() => {
    if (!selectedCardId) {
      setRepositoryDetailState(repositoryId, { selectedCard: null, evidence: [] })
      return
    }

    let cancelled = false

    const loadCard = async () => {
      setRepositoryDetailState(repositoryId, { cardLoading: true, cardError: null })
      try {
        const [nextCard, nextEvidence] = await Promise.all([
          fetchRepoLibraryCard(daemonUrl, selectedCardId),
          fetchRepoLibraryCardEvidence(daemonUrl, selectedCardId),
        ])
        if (cancelled) return
        setRepositoryDetailState(repositoryId, {
          selectedCard: nextCard,
          evidence: nextEvidence,
          cardLoading: false,
          cardError: null,
        })
      } catch (err: unknown) {
        if (cancelled) return
        setRepositoryDetailState(repositoryId, {
          selectedCard: null,
          evidence: [],
          cardLoading: false,
          cardError: err instanceof Error ? err.message : String(err),
        })
      }
    }

    void loadCard()
    return () => {
      cancelled = true
    }
  }, [daemonUrl, repositoryId, selectedCardId, setRepositoryDetailState])

  const analysisRuns = useMemo(() => detail?.analysis_runs ?? [], [detail?.analysis_runs])
  const repository = detail?.repository ?? null
  const selectedSnapshot = useMemo(
    () => snapshots.find((item) => item.snapshot_id === selectedSnapshotId) ?? detail?.latest_snapshot ?? null,
    [detail?.latest_snapshot, selectedSnapshotId, snapshots],
  )
  const selectedAnalysis = useMemo(
    () => analysisRuns.find((item) => item.analysis_id === selectedAnalysisId) ?? detail?.latest_analysis ?? null,
    [analysisRuns, detail?.latest_analysis, selectedAnalysisId],
  )

  const onSyncLatestChatReply = useCallback(
    async (analysis: RepoLibraryAnalysisRun | null) => {
      if (!analysis?.analysis_id || !analysis.chat_session_id) {
        toast({ variant: 'destructive', title: '当前分析未关联 Chat 会话' })
        return
      }
      setSyncingAnalysisId(analysis.analysis_id)
      try {
        await syncRepoLibraryAnalysisChat(daemonUrl, analysis.analysis_id)
        await refresh()
        toast({ title: '已同步最新 Chat 回复', description: analysis.chat_session_id })
      } catch (err: unknown) {
        toast({
          variant: 'destructive',
          title: '同步 Chat 回复失败',
          description: err instanceof Error ? err.message : String(err),
        })
      } finally {
        setSyncingAnalysisId(null)
      }
    },
    [daemonUrl, refresh],
  )

  const loadTailIntoTerminal = useCallback(
    async (analysis: RepoLibraryAnalysisRun | null) => {
      const executionId = analysis?.execution_id
      if (!executionId) {
        terminalRef.current?.reset('当前分析暂无 execution 日志。\r\n')
        return
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

  useEffect(() => {
    void loadTailIntoTerminal(selectedAnalysis)
  }, [loadTailIntoTerminal, selectedAnalysis])

  const reportText =
    reportMarkdown ||
    selectedSnapshot?.report_markdown ||
    selectedSnapshot?.report_excerpt ||
    detail?.report_markdown ||
    detail?.report_excerpt ||
    ''

  const sidebarContent = (
    <div className="relative min-h-[120px]">
      {repositoriesError && !hasRepositoryCache ? <Alert color="danger" title="加载仓库失败" description={repositoriesError} /> : null}
      {!hasRepositoryCache && repositoriesLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-[58px] w-full rounded-[22px]" />
          <Skeleton className="h-[58px] w-full rounded-[22px]" />
          <Skeleton className="h-[58px] w-full rounded-[22px]" />
        </div>
      ) : repositories.length === 0 ? (
        <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">
          暂无仓库列表。
        </div>
      ) : (
        <div className="space-y-2">
          {repositories.map((item) => (
            <RepoLibrarySidebarRepositoryItem
              key={item.repository_id}
              title={item.full_name || item.name || item.repo_url}
              subtitle={item.repo_url}
              meta={formatRelativeTime(item.created_at || item.updated_at || 0)}
              active={item.repository_id === repositoryId}
              onPress={() => goToRepoLibraryRepository(item.repository_id)}
            />
          ))}
        </div>
      )}
      <LoadingVeil visible={repositoriesLoading && hasRepositoryCache} compact label="正在刷新仓库列表…" />
    </div>
  )

  return (
    <RepoLibraryShell
      title={repository?.full_name || '仓库详情'}
      headerMeta={
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span className="truncate">{repository?.repo_url || '查看仓库详情'}</span>
          {selectedAnalysis ? (
            <Chip color={analysisStatusColor(selectedAnalysis.status)} variant="flat" size="sm">
              {formatAnalysisStatus(selectedAnalysis.status)}
            </Chip>
          ) : null}
        </div>
      }
      headerActions={
        <>
          <Button variant="flat" size="sm" startContent={<FileSearch className="h-4 w-4" />} onPress={goToRepoLibraryPatternSearch}>
            知识库检索
          </Button>
          <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void refresh({ force: true })}>
            刷新详情
          </Button>
        </>
      }
      sidebarTitle="仓库"
      sidebarCount={repositories.length}
      sidebarAction={
        <Button color="primary" size="sm" className="w-[25%] min-w-[86px] rounded-2xl" startContent={<Plus className="h-4 w-4 shrink-0 stroke-[3]" />} onPress={goToRepoLibraryRepositories}>
          添加仓库
        </Button>
      }
      sidebarContent={sidebarContent}
      contentMaxWidthClassName="max-w-[1200px]"
    >
      <div className="relative">
        {repositoriesError && hasRepositoryCache ? <Alert color="danger" title="刷新失败，已保留上次仓库列表" description={repositoriesError} className="mb-4" /> : null}
        {error && hasDetailCache ? <Alert color="danger" title="刷新详情失败，已保留上次内容" description={error} className="mb-4" /> : null}

        {!hasDetailCache && loading ? (
          <DetailSkeleton />
        ) : (
          <>
            <section className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
              <div className="space-y-4">
                <div className="rounded-2xl border bg-card p-5 shadow-sm">
                  <div className="mb-4 flex flex-wrap items-center gap-2">
                    <div className="text-sm font-semibold">仓库元信息</div>
                    {repository?.default_branch ? (
                      <Chip variant="bordered" size="sm">
                        默认分支 {repository.default_branch}
                      </Chip>
                    ) : null}
                    {typeof repository?.snapshot_count === 'number' ? (
                      <Chip variant="bordered" size="sm">
                        快照 {repository.snapshot_count}
                      </Chip>
                    ) : null}
                    {typeof repository?.card_count === 'number' ? (
                      <Chip variant="bordered" size="sm">
                        卡片 {repository.card_count}
                      </Chip>
                    ) : null}
                  </div>

                  <div className="grid gap-3 text-sm md:grid-cols-2">
                    <div className="rounded-xl border bg-muted/20 p-3">
                      <div className="text-xs uppercase tracking-wide text-muted-foreground/80">仓库地址</div>
                      <div className="mt-1 break-all font-medium">{repository?.repo_url || '—'}</div>
                    </div>
                    <div className="rounded-xl border bg-muted/20 p-3">
                      <div className="text-xs uppercase tracking-wide text-muted-foreground/80">最近活跃</div>
                      <div className="mt-1 font-medium">
                        {repository?.updated_at ? formatRelativeTime(repository.updated_at) : '—'}
                      </div>
                    </div>
                    <div className="rounded-xl border bg-muted/20 p-3">
                      <div className="text-xs uppercase tracking-wide text-muted-foreground/80">最新 Ref</div>
                      <div className="mt-1 font-medium">{selectedSnapshot?.resolved_ref || selectedSnapshot?.ref || '—'}</div>
                    </div>
                    <div className="rounded-xl border bg-muted/20 p-3">
                      <div className="text-xs uppercase tracking-wide text-muted-foreground/80">最新 Commit</div>
                      <code className="mt-1 block font-medium">{selectedSnapshot?.commit_sha || '—'}</code>
                    </div>
                  </div>
                </div>

                <div className="rounded-2xl border bg-card p-5 shadow-sm">
                  <div className="mb-4 flex flex-wrap items-center gap-2">
                    <ScrollText className="h-4 w-4" />
                    <div className="text-sm font-semibold">报告</div>
                    {selectedSnapshot?.report_url ? (
                      <Button
                        variant="light"
                        size="sm"
                        onPress={() => window.open(selectedSnapshot.report_url, '_blank', 'noopener,noreferrer')}
                      >
                        打开完整报告
                      </Button>
                    ) : null}
                  </div>

                  {reportText ? (
                    <div className="chat-markdown max-h-80 overflow-auto rounded-xl border bg-muted/20 p-4 text-sm leading-7">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{reportText}</ReactMarkdown>
                    </div>
                  ) : selectedSnapshot?.report_path ? (
                    <div className="rounded-xl border bg-muted/20 p-4 text-sm text-muted-foreground">
                      <div>当前快照已生成报告文件。</div>
                      <code className="mt-2 block break-all text-xs text-foreground">{selectedSnapshot.report_path}</code>
                    </div>
                  ) : (
                    <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
                      当前快照暂无可展示的报告正文，后端补齐 `report_markdown` / `report_excerpt` 后会直接显示在这里。
                    </div>
                  )}
                </div>
              </div>

              <div className="space-y-4">
                <div className="rounded-2xl border bg-card p-5 shadow-sm">
                  <div className="mb-3 text-sm font-semibold">快照</div>
                  {snapshots.length === 0 ? (
                    <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                      该仓库还没有快照。
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {snapshots.map((snapshot) => {
                        const active = snapshot.snapshot_id === selectedSnapshotId
                        return (
                          <button
                            key={snapshot.snapshot_id}
                            type="button"
                            className={`w-full rounded-xl border p-3 text-left transition ${
                              active ? 'border-primary bg-primary/5' : 'bg-muted/10 hover:border-primary/40'
                            }`}
                            onClick={() => setRepositoryDetailState(repositoryId, { selectedSnapshotId: snapshot.snapshot_id })}
                          >
                            <div className="flex items-center justify-between gap-2">
                              <div className="font-medium">
                                {snapshot.resolved_ref || snapshot.ref || snapshot.snapshot_id}
                              </div>
                              <div className="text-xs text-muted-foreground">
                                {formatRelativeTime(snapshot.created_at || snapshot.updated_at || 0)}
                              </div>
                            </div>
                            <code className="mt-2 block text-xs text-muted-foreground">{snapshot.commit_sha || '未记录 commit'}</code>
                          </button>
                        )
                      })}
                    </div>
                  )}
                </div>

                <div className="rounded-2xl border bg-card p-5 shadow-sm">
                  <div className="mb-3 text-sm font-semibold">分析运行</div>
                  {analysisRuns.length === 0 ? (
                    <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                      当前仓库暂无分析运行记录。
                    </div>
                  ) : (
                    <div className="space-y-2">
                      {analysisRuns.map((analysis) => {
                        const active = analysis.analysis_id === selectedAnalysisId
                        return (
                          <button
                            key={analysis.analysis_id}
                            type="button"
                            className={`w-full rounded-xl border p-3 text-left transition ${
                              active ? 'border-primary bg-primary/5' : 'bg-muted/10 hover:border-primary/40'
                            }`}
                            onClick={() => setRepositoryDetailState(repositoryId, { selectedAnalysisId: analysis.analysis_id })}
                          >
                            <div className="flex flex-wrap items-center gap-2">
                              <div className="font-medium">{analysis.analysis_id}</div>
                              <Chip color={analysisStatusColor(analysis.status)} variant="flat" size="sm">
                                {formatAnalysisStatus(analysis.status)}
                              </Chip>
                            </div>
                            <div className="mt-2 text-xs text-muted-foreground">
                              Execution: {analysis.execution_id || '待分配'}
                            </div>
                            {analysis.chat_session_id ? (
                              <div className="mt-1 truncate text-xs text-muted-foreground">
                                Chat: {analysis.chat_session_id}
                              </div>
                            ) : null}
                            <div className="mt-1 text-xs text-muted-foreground">
                              {formatRelativeTime(analysis.updated_at || analysis.created_at || 0)}
                            </div>
                            {analysis.failure_message ? (
                              <div className="mt-2 text-xs text-danger">{analysis.failure_message}</div>
                            ) : null}
                          </button>
                        )
                      })}
                    </div>
                  )}
                  {selectedAnalysis ? (
                    <div className="mt-3 rounded-xl border bg-muted/20 p-3">
                      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                        <div className="space-y-2">
                          <div>
                            <div className="text-sm font-medium">关联 Chat</div>
                            <div className="mt-1 text-xs text-muted-foreground">
                              {selectedAnalysis.chat_session_id
                                ? '可直接打开分析会话，或将最新 assistant 回复同步回 Repo Library。'
                                : '当前分析尚未关联可打开的 Chat 会话。'}
                            </div>
                          </div>
                          <code className="block break-all text-xs text-foreground">
                            {selectedAnalysis.chat_session_id || '—'}
                          </code>
                          <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-3">
                            <div>
                              <div className="text-muted-foreground/80">CLI 工具</div>
                              <div className="text-foreground">{selectedAnalysis.cli_tool_id || '—'}</div>
                            </div>
                            <div>
                              <div className="text-muted-foreground/80">模型</div>
                              <div className="break-all text-foreground">{selectedAnalysis.model_id || '—'}</div>
                            </div>
                            <div>
                              <div className="text-muted-foreground/80">运行时</div>
                              <div className="text-foreground">{selectedAnalysis.runtime_kind || '—'}</div>
                            </div>
                          </div>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            variant="flat"
                            size="sm"
                            isDisabled={!selectedAnalysis.chat_session_id}
                            onPress={() => {
                              if (!selectedAnalysis.chat_session_id) return
                              goToChat(selectedAnalysis.chat_session_id)
                            }}
                          >
                            打开分析 Chat
                          </Button>
                          <Button
                            variant="light"
                            size="sm"
                            isDisabled={!selectedAnalysis.chat_session_id}
                            isLoading={syncingAnalysisId === selectedAnalysis.analysis_id}
                            onPress={() => void onSyncLatestChatReply(selectedAnalysis)}
                          >
                            同步最新 Chat 回复
                          </Button>
                        </div>
                      </div>
                    </div>
                  ) : null}
                </div>
              </div>
            </section>

            <section className="mt-4 grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
              <div className="rounded-2xl border bg-card p-5 shadow-sm">
                <div className="mb-3 flex items-center justify-between gap-2">
                  <div className="text-sm font-semibold">知识卡片</div>
                  {selectedSnapshot ? (
                    <Chip variant="bordered" size="sm">
                      当前快照 {selectedSnapshot.resolved_ref || selectedSnapshot.ref || selectedSnapshot.snapshot_id}
                    </Chip>
                  ) : null}
                </div>

                {cardsError ? (
                  <Alert color="danger" title="加载卡片失败" description={cardsError} />
                ) : cardsLoading && cards.length === 0 ? (
                  <div className="space-y-2">
                    <Skeleton className="h-24 w-full rounded-xl" />
                    <Skeleton className="h-24 w-full rounded-xl" />
                  </div>
                ) : cards.length === 0 ? (
                  <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                    当前筛选条件下暂无知识卡片。
                  </div>
                ) : (
                  <div className="relative space-y-2">
                    {cards.map((card: RepoLibraryCard) => {
                      const active = card.card_id === selectedCardId
                      return (
                        <button
                          key={card.card_id}
                          type="button"
                          className={`w-full rounded-xl border p-3 text-left transition ${
                            active ? 'border-primary bg-primary/5' : 'bg-muted/10 hover:border-primary/40'
                          }`}
                          onClick={() => setRepositoryDetailState(repositoryId, { selectedCardId: card.card_id })}
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <div className="font-medium">{card.title || card.card_id}</div>
                            <Chip variant="flat" size="sm">
                              {formatCardType(card.card_type)}
                            </Chip>
                          </div>
                          <div className="mt-2 text-sm text-muted-foreground">{card.summary || '暂无摘要'}</div>
                        </button>
                      )
                    })}
                    <LoadingVeil visible={cardsLoading && cards.length > 0} compact label="正在刷新卡片列表…" />
                  </div>
                )}
              </div>

              <div className="rounded-2xl border bg-card p-5 shadow-sm">
                <div className="mb-3 text-sm font-semibold">卡片详情与证据</div>
                {cardError ? (
                  <Alert color="danger" title="加载卡片详情失败" description={cardError} />
                ) : cardLoading && !selectedCard ? (
                  <div className="space-y-2">
                    <Skeleton className="h-24 w-full rounded-xl" />
                    <Skeleton className="h-24 w-full rounded-xl" />
                  </div>
                ) : selectedCard ? (
                  <div className="relative space-y-4">
                    <div className="rounded-xl border bg-muted/20 p-4">
                      <div className="flex flex-wrap items-center gap-2">
                        <div className="text-base font-semibold">{selectedCard.title}</div>
                        <Chip variant="flat" size="sm">
                          {formatCardType(selectedCard.card_type)}
                        </Chip>
                        {typeof selectedCard.confidence === 'number' ? (
                          <Chip variant="bordered" size="sm">
                            置信度 {(selectedCard.confidence * 100).toFixed(0)}%
                          </Chip>
                        ) : null}
                      </div>
                      <div className="mt-3 whitespace-pre-wrap text-sm text-muted-foreground">
                        {selectedCard.detail || selectedCard.summary || '暂无详细说明'}
                      </div>
                      {selectedCard.tags && selectedCard.tags.length > 0 ? (
                        <div className="mt-3 flex flex-wrap gap-2">
                          {selectedCard.tags.map((tag) => (
                            <Chip key={tag} variant="bordered" size="sm">
                              {tag}
                            </Chip>
                          ))}
                        </div>
                      ) : null}
                    </div>

                    <div className="space-y-2">
                      <div className="text-sm font-medium">Evidence</div>
                      {evidence.length === 0 ? (
                        <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                          当前卡片暂无独立 evidence 记录。
                        </div>
                      ) : (
                        evidence.map((item: RepoLibraryCardEvidence) => (
                          <div key={item.evidence_id} className="rounded-xl border bg-muted/20 p-4">
                            <div className="flex flex-wrap items-center gap-2">
                              <code className="text-xs">{item.source_path}</code>
                              {item.label ? <Chip variant="flat" size="sm">{item.label}</Chip> : null}
                              {(typeof item.start_line === 'number' || typeof item.end_line === 'number') ? (
                                <Chip variant="bordered" size="sm">
                                  {typeof item.start_line === 'number' ? item.start_line : '?'}
                                  {typeof item.end_line === 'number' && item.end_line !== item.start_line ? `-${item.end_line}` : ''}
                                </Chip>
                              ) : null}
                            </div>
                            {item.excerpt ? (
                              <pre className="mt-3 whitespace-pre-wrap rounded-lg border bg-background p-3 text-xs leading-6">
                                {item.excerpt}
                              </pre>
                            ) : null}
                          </div>
                        ))
                      )}
                    </div>
                    <LoadingVeil visible={cardLoading && Boolean(selectedCard)} compact label="正在刷新卡片详情…" />
                  </div>
                ) : (
                  <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
                    选择左侧卡片后，这里会显示摘要、详细说明和 evidence 链接。
                  </div>
                )}
              </div>
            </section>

            <section className="mt-4 rounded-2xl border bg-card p-5 shadow-sm">
              <div className="mb-3 text-sm font-semibold">分析日志</div>
              {selectedAnalysis?.execution_id ? (
                <div className="mb-3 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                  <code>{selectedAnalysis.execution_id}</code>
                  <Button
                    variant="light"
                    size="sm"
                    onPress={() => {
                      void loadTailIntoTerminal(selectedAnalysis)
                      toast({ title: '已刷新日志', description: selectedAnalysis.execution_id })
                    }}
                  >
                    刷新日志
                  </Button>
                </div>
              ) : null}
              <div className="h-[420px] overflow-hidden rounded-xl border bg-black">
                <TerminalPane ref={terminalRef} />
              </div>
            </section>
          </>
        )}

        <LoadingVeil visible={loading && hasDetailCache} label="正在刷新仓库详情…" />
      </div>
    </RepoLibraryShell>
  )
}
