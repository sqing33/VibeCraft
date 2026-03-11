import type { Selection } from '@react-types/shared'
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Chip,
  Modal,
  ModalBody,
  ModalContent,
  ModalHeader,
  Select,
  SelectItem,
  Skeleton,
} from '@heroui/react'
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
import {
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

function selectionToString(keys: Selection): string {
  if (keys === 'all') return ''
  const first = keys.values().next()
  return typeof first.value === 'string' ? first.value : ''
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

function normalizeCardText(text?: string): string {
  return (text ?? '')
    .replace(/^思路[:：]\s*/u, '')
    .replace(/\s+/gu, ' ')
    .trim()
}

function displaySummary(summary?: string, conclusion?: string, mechanism?: string): string {
  const normalizedSummary = normalizeCardText(summary)
  if (!normalizedSummary) return ''

  const normalizedConclusion = normalizeCardText(conclusion)
  if (normalizedConclusion && normalizedSummary === normalizedConclusion) {
    return ''
  }

  const normalizedMechanism = normalizeCardText(mechanism)
  const mechanismPrefix = normalizedMechanism.replace(/^思路[:：]\s*/u, '').trim()
  const summaryPrefix = normalizedSummary.replace(/\.{3}$/u, '').trim()
  if (summaryPrefix && mechanismPrefix && mechanismPrefix.startsWith(summaryPrefix)) {
    return ''
  }

  return summary?.trim() ?? ''
}

function formatGeneratedAt(value?: string | null): string {
  if (!value) return '—'
  return value.replace(/\s+[+-]\d{4}$/u, '')
}

function DetailSkeleton() {
  return (
    <div className="space-y-4">
      <Skeleton className="h-20 w-full rounded-2xl" />
      <Skeleton className="h-[72vh] w-full rounded-2xl" />
    </div>
  )
}

function ContextField(props: { label: string; value?: string | null; multiline?: boolean }) {
  const { label, value, multiline = false } = props
  return (
    <div className="border-b border-default-200/70 pb-3 last:border-b-0 last:pb-0">
      <div className="text-[11px] uppercase tracking-[0.16em] text-muted-foreground/75">{label}</div>
      <div className={`mt-1 text-sm text-foreground/85 ${multiline ? 'whitespace-pre-wrap leading-6' : 'font-medium'}`}>{value?.trim() || '—'}</div>
    </div>
  )
}

/**
 * 功能：展示单个仓库的双区域知识工作台，包括左侧上下文和右侧卡片阅读区。
 * 参数/返回：接收 repositoryId；返回仓库详情页。
 * 失败场景：详情、快照、卡片或证据请求失败时展示错误提示，并保留刷新入口。
 * 副作用：发起仓库详情/快照/卡片/evidence 请求，并允许把分析 Chat 的最新回复同步回知识库。
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
  const selectedCardSummary = selectedCard
    ? displaySummary(selectedCard.summary, selectedCard.conclusion, selectedCard.mechanism)
    : ''
  const evidence = detailCache.evidence
  const reportMarkdown = detailCache.reportMarkdown

  const [syncingAnalysisId, setSyncingAnalysisId] = useState<string | null>(null)
  const [reportModalOpen, setReportModalOpen] = useState(false)
  const [detailReloadToken, setDetailReloadToken] = useState(0)
  const cardScrollRef = useCallback((el: HTMLDivElement | null) => {
    if (!el) return
    const handler = (e: WheelEvent) => {
      if (e.deltaY === 0) return
      e.preventDefault()
      el.scrollLeft += e.deltaY * 0.3
    }
    el.addEventListener('wheel', handler, { passive: false })
  }, [])

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
      setDetailReloadToken((value) => value + 1)
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
  }, [daemonUrl, detailReloadToken, repositoryId, selectedSnapshotId, setRepositoryDetailState])

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
  }, [daemonUrl, detailReloadToken, repositoryId, selectedSnapshotId, setRepositoryDetailState])

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
  }, [daemonUrl, detailReloadToken, repositoryId, selectedCardId, setRepositoryDetailState])

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

  const reportAvailable = Boolean(reportMarkdown || selectedSnapshot?.report_path)
  const selectedSnapshotLabel = selectedSnapshot?.resolved_ref || selectedSnapshot?.ref || '未选择快照'
  const selectedAnalysisLabel = selectedAnalysis?.analysis_id || '未选择分析'
  const selectedCommitShort = selectedSnapshot?.commit_sha ? selectedSnapshot.commit_sha.slice(0, 10) : '—'
  const reportContext = selectedSnapshot?.report_context_summary
  const generatedAtLabel = formatGeneratedAt(reportContext?.generated_at)

  return (
    <>
      <RepoLibraryShell
        title={
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <div className="truncate">{repository?.full_name || '仓库详情'}</div>
            {selectedAnalysis ? (
              <Chip color={analysisStatusColor(selectedAnalysis.status)} variant="flat" size="sm">
                {formatAnalysisStatus(selectedAnalysis.status)}
              </Chip>
            ) : null}
            <span className="text-xs font-normal text-muted-foreground">
              生成时间：{generatedAtLabel}
            </span>
          </div>
        }
        headerMeta={
          <div className="flex min-w-0 flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <span className="truncate">{repository?.repo_url || '查看仓库详情'}</span>
          </div>
        }
        headerActions={
          <>
            <Button variant="flat" size="sm" startContent={<ScrollText className="h-4 w-4" />} isDisabled={!reportAvailable} onPress={() => setReportModalOpen(true)}>
              查看报告
            </Button>
            <Button variant="flat" size="sm" startContent={<FileSearch className="h-4 w-4" />} onPress={() => goToRepoLibraryPatternSearch()}>
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
        contentMaxWidthClassName="max-w-[1680px]"
        contentPaddingClassName="gap-4 pb-4 pl-2 pr-4 pt-4 md:gap-4 md:pb-5 md:pl-3 md:pr-5 md:pt-5"
      >
        <div className="relative">
          {repositoriesError && hasRepositoryCache ? <Alert color="danger" title="刷新失败，已保留上次仓库列表" description={repositoriesError} className="mb-4" /> : null}
          {error && hasDetailCache ? <Alert color="danger" title="刷新详情失败，已保留上次内容" description={error} className="mb-4" /> : null}

          {!hasDetailCache && loading ? (
            <DetailSkeleton />
          ) : (
            <div className="space-y-4">
              <section className="grid gap-4 xl:h-[calc(100vh-6.25rem)] xl:min-h-[680px] xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] xl:overflow-hidden">
                <div className="grid gap-4 xl:min-h-0 xl:grid-rows-[auto_minmax(0,1fr)]">
                  <div className="rounded-[28px] border border-default-200/80 bg-gradient-to-br from-background to-default-50/65 p-4 shadow-sm md:p-5">
                    <div className="grid gap-3 md:grid-cols-2">
                      <div className="min-w-0">
                        <div className="mb-1.5 pl-1 text-[11px] uppercase tracking-[0.16em] text-muted-foreground/75">快照</div>
                        <Select
                          aria-label="选择快照"
                          size="sm"
                          variant="bordered"
                          className="w-full"
                          selectedKeys={selectedSnapshotId ? new Set([selectedSnapshotId]) : new Set([])}
                          isDisabled={snapshots.length === 0}
                          disallowEmptySelection
                          onSelectionChange={(keys) => {
                            const next = selectionToString(keys)
                            if (!next) return
                            setRepositoryDetailState(repositoryId, { selectedSnapshotId: next })
                          }}
                        >
                          {snapshots.map((snapshot) => (
                            <SelectItem key={snapshot.snapshot_id} textValue={snapshot.resolved_ref || snapshot.ref || snapshot.snapshot_id}>
                              {(snapshot.resolved_ref || snapshot.ref || snapshot.snapshot_id) + ' · ' + formatRelativeTime(snapshot.created_at || snapshot.updated_at || 0)}
                            </SelectItem>
                          ))}
                        </Select>
                      </div>
                      <div className="min-w-0">
                        <div className="mb-1.5 pl-1 text-[11px] uppercase tracking-[0.16em] text-muted-foreground/75">分析运行</div>
                        <Select
                          aria-label="选择分析运行"
                          size="sm"
                          variant="bordered"
                          className="w-full"
                          selectedKeys={selectedAnalysisId ? new Set([selectedAnalysisId]) : new Set([])}
                          isDisabled={analysisRuns.length === 0}
                          disallowEmptySelection
                          onSelectionChange={(keys) => {
                            const next = selectionToString(keys)
                            if (!next) return
                            setRepositoryDetailState(repositoryId, { selectedAnalysisId: next })
                          }}
                        >
                          {analysisRuns.map((analysis) => (
                            <SelectItem key={analysis.analysis_id} textValue={analysis.analysis_id}>
                              {analysis.analysis_id + ' · ' + formatAnalysisStatus(analysis.status)}
                            </SelectItem>
                          ))}
                        </Select>
                      </div>
                    </div>

                    <div className="mt-3 flex flex-wrap items-center gap-2">
                      <Chip variant="flat" size="sm" className="bg-default-100/85">
                        {selectedSnapshotLabel}
                      </Chip>
                      <Chip variant="bordered" size="sm">
                        Commit {selectedCommitShort}
                      </Chip>
                      {selectedAnalysis ? (
                        <Chip color={analysisStatusColor(selectedAnalysis.status)} variant="flat" size="sm">
                          {formatAnalysisStatus(selectedAnalysis.status)}
                        </Chip>
                      ) : null}
                      <Chip variant="bordered" size="sm" className="max-w-full">
                        {selectedAnalysisLabel}
                      </Chip>
                    </div>

                    {selectedAnalysis ? (
                      <>
                        <div className="mt-3 rounded-[24px] border border-default-200/75 bg-background/75 px-4 py-3">
                          <div className="grid gap-x-6 gap-y-2 text-xs text-muted-foreground md:grid-cols-2">
                            <div>
                              <span className="uppercase tracking-[0.14em] text-muted-foreground/70">运行时</span>
                              <span className="ml-2 text-foreground/90">{selectedAnalysis.runtime_kind || '—'}</span>
                            </div>
                            <div>
                              <span className="uppercase tracking-[0.14em] text-muted-foreground/70">CLI 工具</span>
                              <span className="ml-2 text-foreground/90">{selectedAnalysis.cli_tool_id || '—'}</span>
                            </div>
                            <div className="md:col-span-2">
                              <span className="uppercase tracking-[0.14em] text-muted-foreground/70">Chat</span>
                              <span className="ml-2 break-all text-foreground/90">{selectedAnalysis.chat_session_id || '—'}</span>
                            </div>
                            <div>
                              <span className="uppercase tracking-[0.14em] text-muted-foreground/70">模型</span>
                              <span className="ml-2 break-all text-foreground/90">{selectedAnalysis.model_id || '—'}</span>
                            </div>
                            <div>
                              <span className="uppercase tracking-[0.14em] text-muted-foreground/70">更新时间</span>
                              <span className="ml-2 text-foreground/90">{formatRelativeTime(selectedAnalysis.updated_at || selectedAnalysis.created_at || 0)}</span>
                            </div>
                          </div>
                        </div>

                        {selectedAnalysis.failure_message ? (
                          <Alert color="danger" title="分析失败" description={selectedAnalysis.failure_message} className="mt-3" />
                        ) : null}

                        <div className="mt-3 flex flex-wrap gap-2">
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
                            同步最新回复
                          </Button>
                        </div>
                      </>
                    ) : (
                      <div className="mt-4 rounded-[22px] border border-dashed border-default-300/80 px-4 py-5 text-sm text-muted-foreground">
                        当前仓库暂无可选分析运行。
                      </div>
                    )}
                  </div>

                  <div className="min-h-0 rounded-[28px] border border-default-200/70 bg-background/90 p-4 shadow-sm md:p-5 xl:flex xl:flex-col">
                    <div className="thin-scrollbar min-h-0 space-y-3 overflow-x-hidden xl:flex-1 xl:overflow-y-auto xl:pr-1">
                      <ContextField label="技术栈总览" value={reportContext?.stack_overview} multiline />
                      <ContextField label="后端" value={reportContext?.backend_summary} multiline />
                      <ContextField label="前端" value={reportContext?.frontend_summary} multiline />
                      <ContextField label="其它模块" value={reportContext?.other_modules_summary} multiline />
                    </div>
                  </div>
                </div>

                <div className="min-h-0 rounded-[32px] border border-default-200/85 bg-background/95 shadow-[0_28px_72px_-42px_rgba(15,23,42,0.35)] xl:flex xl:flex-col xl:overflow-hidden">
                  <div className="shrink-0 px-4 pb-4 pt-4 md:px-5 md:pb-5 md:pt-5">
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div className="text-sm font-semibold">知识卡片</div>
                      {selectedSnapshot ? (
                        <Chip variant="bordered" size="sm">
                          {cards.length} 张
                        </Chip>
                      ) : null}
                    </div>

                    {cardsError ? (
                      <Alert color="danger" title="加载卡片失败" description={cardsError} />
                    ) : cardsLoading && cards.length === 0 ? (
                      <div className="flex gap-3 overflow-hidden">
                        <Skeleton className="h-[88px] min-w-[220px] rounded-[22px]" />
                        <Skeleton className="h-[88px] min-w-[220px] rounded-[22px]" />
                        <Skeleton className="h-[88px] min-w-[220px] rounded-[22px]" />
                      </div>
                    ) : cards.length === 0 ? (
                      <div className="rounded-[22px] border border-dashed p-4 text-sm text-muted-foreground">当前筛选条件下暂无知识卡片。</div>
                    ) : (
                      <div className="relative">
                        <div
                          ref={cardScrollRef}
                          className="grid grid-flow-col auto-cols-[220px] gap-3 overflow-x-auto pb-[5px] [&::-webkit-scrollbar]:h-[3px] [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-default-200/60 [&::-webkit-scrollbar-track]:bg-transparent"
                        >
                          {cards.map((card: RepoLibraryCard) => {
                            const active = card.card_id === selectedCardId
                            return (
                              <button
                                key={card.card_id}
                                type="button"
                                className={`flex min-h-[88px] flex-col justify-between rounded-[22px] border px-4 py-3 text-left transition ${
                                  active
                                    ? 'border-primary/40 bg-background shadow-[0_18px_34px_-28px_rgba(14,165,233,0.65)] ring-1 ring-primary/20'
                                    : 'border-default-200/70 bg-background/70 hover:border-default-300 hover:bg-background'
                                }`}
                                onClick={() => setRepositoryDetailState(repositoryId, { selectedCardId: card.card_id })}
                              >
                                <div className="line-clamp-2 text-[15px] font-medium leading-6 text-foreground/95">{card.title || card.card_id}</div>
                                <div className="flex items-center justify-between gap-2">
                                  <Chip variant="flat" size="sm">
                                    {formatCardType(card.card_type)}
                                  </Chip>
                                  {active ? <span className="text-[11px] font-medium text-primary">当前</span> : null}
                                </div>
                              </button>
                            )
                          })}
                        </div>
                        <LoadingVeil visible={cardsLoading && cards.length > 0} compact label="正在刷新卡片列表…" />
                      </div>
                    )}
                  </div>

                  <div className="min-h-0 border-t border-default-200/70 bg-default-50/25 px-4 pb-4 pt-4 md:px-5 md:pb-5 xl:flex-1 xl:overflow-hidden">
                    {cardError ? (
                      <Alert color="danger" title="加载卡片详情失败" description={cardError} />
                    ) : cardLoading && !selectedCard ? (
                      <div className="space-y-2">
                        <Skeleton className="h-24 w-full rounded-xl" />
                        <Skeleton className="h-24 w-full rounded-xl" />
                      </div>
                    ) : selectedCard ? (
                      <div className="thin-scrollbar relative space-y-4 xl:h-full xl:overflow-y-auto xl:pr-1">
                        <div className="rounded-[26px] border border-default-200/70 bg-default-50/45 p-5">
                          <div className="flex flex-wrap items-center gap-2">
                            <div className="text-xl font-semibold leading-8">{selectedCard.title}</div>
                            {typeof selectedCard.confidence === 'number' ? (
                              <Chip variant="bordered" size="sm">
                                置信度 {(selectedCard.confidence * 100).toFixed(0)}%
                              </Chip>
                            ) : null}
                            {selectedCard.section_title ? (
                              <Chip variant="flat" size="sm">
                                {selectedCard.section_title}
                              </Chip>
                            ) : null}
                          </div>
                          {selectedCard.conclusion ? (
                            <div className="mt-5 rounded-[22px] border border-primary/20 bg-primary/5 px-4 py-3.5 text-[15px] font-medium leading-7 text-foreground">
                              {selectedCard.conclusion}
                            </div>
                          ) : null}
                          {selectedCardSummary ? (
                            <div className="mt-4 whitespace-pre-wrap text-sm leading-7 text-muted-foreground">{selectedCardSummary}</div>
                          ) : null}
                          {selectedCard.mechanism ? (
                            <div className="mt-5 space-y-2 border-t border-default-200/70 pt-4">
                              <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground/75">实现机制</div>
                              <div className="whitespace-pre-wrap text-sm leading-7 text-foreground/85">{selectedCard.mechanism}</div>
                            </div>
                          ) : !selectedCardSummary && !selectedCard.conclusion ? (
                            <div className="mt-4 text-sm text-muted-foreground">暂无详细说明</div>
                          ) : null}
                          {selectedCard.tags && selectedCard.tags.length > 0 ? (
                            <div className="mt-5 flex flex-wrap gap-2">
                              {selectedCard.tags.map((tag) => (
                                <Chip key={tag} variant="bordered" size="sm">
                                  {tag}
                                </Chip>
                              ))}
                            </div>
                          ) : null}
                        </div>

                        <div className="rounded-[26px] border border-default-200/65 bg-default-50/30 p-4">
                          <div className="mb-3 flex items-start justify-between gap-2">
                            <div>
                              <div className="text-sm font-medium">实现证据</div>
                              <div className="mt-1 text-xs text-muted-foreground">支撑当前卡片结论的代码定位与摘录。</div>
                            </div>
                            <Chip variant="flat" size="sm">
                              {evidence.length}
                            </Chip>
                          </div>
                          {evidence.length === 0 ? (
                            <div className="rounded-2xl border border-dashed p-4 text-sm text-muted-foreground">当前卡片暂无独立 evidence 记录。</div>
                          ) : (
                            <div className="space-y-3 overflow-x-hidden">
                              {evidence.map((item: RepoLibraryCardEvidence) => (
                                <div key={item.evidence_id} className="rounded-[20px] border border-default-200/65 bg-background/85 p-3.5">
                                  <div className="flex flex-wrap items-center gap-2 text-xs">
                                    <code className="rounded-full bg-default-100/80 px-2.5 py-1 text-[11px] text-foreground/80">{item.source_path}</code>
                                    {item.label ? <Chip variant="flat" size="sm">{item.label}</Chip> : null}
                                    {typeof item.start_line === 'number' || typeof item.end_line === 'number' ? (
                                      <Chip variant="bordered" size="sm">
                                        {typeof item.start_line === 'number' ? item.start_line : '?'}
                                        {typeof item.end_line === 'number' && item.end_line !== item.start_line ? `-${item.end_line}` : ''}
                                      </Chip>
                                    ) : null}
                                  </div>
                                  {item.excerpt ? (
                                    <pre className="mt-3 whitespace-pre-wrap rounded-[18px] bg-default-100/45 px-3 py-3 text-xs leading-6 text-foreground/85">
                                      {item.excerpt}
                                    </pre>
                                  ) : null}
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                        <LoadingVeil visible={cardLoading && Boolean(selectedCard)} compact label="正在刷新卡片详情…" />
                      </div>
                    ) : (
                      <div className="flex h-full min-h-[240px] items-center">
                        <div className="w-full rounded-[26px] border border-dashed border-default-300/80 bg-default-50/25 p-8 text-sm text-muted-foreground">
                          先在上方选择一张知识卡片，这里会显示完整结论、实现机制和实现证据。
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </section>
            </div>
          )}

          <LoadingVeil visible={loading && hasDetailCache} label="正在刷新仓库详情…" />
        </div>
      </RepoLibraryShell>

      <Modal isOpen={reportModalOpen} onOpenChange={setReportModalOpen} size="5xl" scrollBehavior="inside">
        <ModalContent className="h-[85vh] min-h-0 overflow-hidden">
          <ModalHeader className="flex items-center gap-2">
            <ScrollText className="h-4 w-4" />
            查看报告
          </ModalHeader>
          <ModalBody className="min-h-0 overflow-y-auto">
            {reportMarkdown ? (
              <div className="chat-markdown rounded-xl border bg-muted/20 p-4 text-sm leading-7">
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{reportMarkdown}</ReactMarkdown>
              </div>
            ) : selectedSnapshot?.report_path ? (
              <div className="rounded-xl border bg-muted/20 p-4 text-sm text-muted-foreground">
                <div>当前快照已生成报告文件，但尚未加载到内存。</div>
                <code className="mt-2 block break-all text-xs text-foreground">{selectedSnapshot.report_path}</code>
              </div>
            ) : (
              <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">当前快照暂无可展示的报告内容。</div>
            )}
          </ModalBody>
        </ModalContent>
      </Modal>
    </>
  )
}
