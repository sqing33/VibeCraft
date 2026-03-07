import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Chip, Skeleton } from '@heroui/react'
import { ArrowLeft, FileSearch, RefreshCcw, ScrollText } from 'lucide-react'

import {
  goToRepoLibraryPatternSearch,
  goToRepoLibraryRepositories,
} from '@/app/routes'
import { TerminalPane, type TerminalPaneHandle } from '@/components/TerminalPane'
import {
  fetchExecutionLogTail,
  fetchRepoLibraryCard,
  fetchRepoLibraryCardEvidence,
  fetchRepoLibraryCards,
  fetchRepoLibraryRepository,
  fetchRepoLibrarySnapshotReport,
  fetchRepoLibrarySnapshots,
  type RepoLibraryAnalysisRun,
  type RepoLibraryCard,
  type RepoLibraryCardEvidence,
  type RepoLibraryRepositoryDetail,
  type RepoLibrarySnapshot,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

import { RepoLibraryLayout } from './RepoLibraryLayout'

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

  const [detail, setDetail] = useState<RepoLibraryRepositoryDetail | null>(null)
  const [snapshots, setSnapshots] = useState<RepoLibrarySnapshot[]>([])
  const [cards, setCards] = useState<RepoLibraryCard[]>([])
  const [selectedSnapshotId, setSelectedSnapshotId] = useState<string | null>(null)
  const [selectedAnalysisId, setSelectedAnalysisId] = useState<string | null>(null)
  const [selectedCardId, setSelectedCardId] = useState<string | null>(null)

  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [cardsLoading, setCardsLoading] = useState(false)
  const [cardsError, setCardsError] = useState<string | null>(null)
  const [cardLoading, setCardLoading] = useState(false)
  const [cardError, setCardError] = useState<string | null>(null)
  const [selectedCard, setSelectedCard] = useState<RepoLibraryCard | null>(null)
  const [evidence, setEvidence] = useState<RepoLibraryCardEvidence[]>([])
  const [reportMarkdown, setReportMarkdown] = useState<string>('')

  const terminalRef = useRef<TerminalPaneHandle | null>(null)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [nextDetail, nextSnapshots] = await Promise.all([
        fetchRepoLibraryRepository(daemonUrl, repositoryId),
        fetchRepoLibrarySnapshots(daemonUrl, repositoryId),
      ])
      setDetail(nextDetail)
      setSnapshots(nextSnapshots)

      setSelectedSnapshotId((current) => {
        if (current && nextSnapshots.some((item) => item.snapshot_id === current)) return current
        if (nextDetail.latest_snapshot?.snapshot_id) return nextDetail.latest_snapshot.snapshot_id
        return nextSnapshots[0]?.snapshot_id ?? null
      })

      const analysisRuns = nextDetail.analysis_runs ?? []
      setSelectedAnalysisId((current) => {
        if (current && analysisRuns.some((item) => item.analysis_id === current)) return current
        if (nextDetail.latest_analysis?.analysis_id) return nextDetail.latest_analysis.analysis_id
        return analysisRuns[0]?.analysis_id ?? null
      })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [daemonUrl, repositoryId])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    let cancelled = false

    const loadCards = async () => {
      setCardsLoading(true)
      setCardsError(null)
      try {
        const next = await fetchRepoLibraryCards(daemonUrl, {
          repository_id: repositoryId,
          snapshot_id: selectedSnapshotId ?? undefined,
          limit: 100,
        })
        if (cancelled) return
        setCards(next)
        setSelectedCardId((current) => {
          if (current && next.some((item) => item.card_id === current)) return current
          return next[0]?.card_id ?? null
        })
      } catch (err: unknown) {
        if (cancelled) return
        setCardsError(err instanceof Error ? err.message : String(err))
        setCards([])
      } finally {
        if (!cancelled) setCardsLoading(false)
      }
    }

    void loadCards()
    return () => {
      cancelled = true
    }
  }, [daemonUrl, repositoryId, selectedSnapshotId])

  useEffect(() => {
		let cancelled = false

		const loadReport = async () => {
			if (!selectedSnapshotId) {
				setReportMarkdown('')
				return
			}
			try {
				const next = await fetchRepoLibrarySnapshotReport(daemonUrl, selectedSnapshotId)
				if (cancelled) return
				setReportMarkdown(next.report_markdown)
			} catch {
				if (cancelled) return
				setReportMarkdown('')
			}
		}

		void loadReport()
		return () => {
			cancelled = true
		}
	}, [daemonUrl, selectedSnapshotId])

  useEffect(() => {
    if (!selectedCardId) {
      setSelectedCard(null)
      setEvidence([])
      return
    }

    let cancelled = false

    const loadCard = async () => {
      setCardLoading(true)
      setCardError(null)
      try {
        const [nextCard, nextEvidence] = await Promise.all([
          fetchRepoLibraryCard(daemonUrl, selectedCardId),
          fetchRepoLibraryCardEvidence(daemonUrl, selectedCardId),
        ])
        if (cancelled) return
        setSelectedCard(nextCard)
        setEvidence(nextEvidence)
      } catch (err: unknown) {
        if (cancelled) return
        setCardError(err instanceof Error ? err.message : String(err))
        setSelectedCard(null)
        setEvidence([])
      } finally {
        if (!cancelled) setCardLoading(false)
      }
    }

    void loadCard()
    return () => {
      cancelled = true
    }
  }, [daemonUrl, selectedCardId])

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

  if (loading && !detail) {
    return (
      <RepoLibraryLayout
        activeNav="repositories"
        title="仓库详情"
        description="正在加载仓库详情、快照、分析运行与知识卡片。"
        actions={
          <Button variant="light" size="sm" startContent={<ArrowLeft className="h-4 w-4" />} onPress={goToRepoLibraryRepositories}>
            返回仓库列表
          </Button>
        }
      >
        <DetailSkeleton />
      </RepoLibraryLayout>
    )
  }

  return (
    <RepoLibraryLayout
      activeNav="repositories"
      title={repository?.full_name || '仓库详情'}
      description={repository?.description || repository?.repo_url || '查看仓库快照、分析运行、报告与知识卡片。'}
      meta={
        selectedAnalysis ? (
          <Chip color={analysisStatusColor(selectedAnalysis.status)} variant="flat" size="sm">
            {formatAnalysisStatus(selectedAnalysis.status)}
          </Chip>
        ) : null
      }
      actions={
        <>
          <Button variant="light" size="sm" startContent={<ArrowLeft className="h-4 w-4" />} onPress={goToRepoLibraryRepositories}>
            返回仓库列表
          </Button>
          <Button variant="flat" size="sm" startContent={<FileSearch className="h-4 w-4" />} onPress={goToRepoLibraryPatternSearch}>
            去模式搜索
          </Button>
          <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void refresh()}>
            刷新详情
          </Button>
        </>
      }
    >
      {error ? <Alert color="danger" title="加载仓库详情失败" description={error} /> : null}

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
              <pre className="max-h-80 overflow-auto whitespace-pre-wrap rounded-xl border bg-muted/20 p-4 text-xs leading-6">
                {reportText}
              </pre>
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
                      onClick={() => setSelectedSnapshotId(snapshot.snapshot_id)}
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
                      onClick={() => setSelectedAnalysisId(analysis.analysis_id)}
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
          </div>
        </div>
      </section>

      <section className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
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
          ) : cardsLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-24 w-full rounded-xl" />
              <Skeleton className="h-24 w-full rounded-xl" />
            </div>
          ) : cards.length === 0 ? (
            <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
              当前筛选条件下暂无知识卡片。
            </div>
          ) : (
            <div className="space-y-2">
              {cards.map((card) => {
                const active = card.card_id === selectedCardId
                return (
                  <button
                    key={card.card_id}
                    type="button"
                    className={`w-full rounded-xl border p-3 text-left transition ${
                      active ? 'border-primary bg-primary/5' : 'bg-muted/10 hover:border-primary/40'
                    }`}
                    onClick={() => setSelectedCardId(card.card_id)}
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
            </div>
          )}
        </div>

        <div className="rounded-2xl border bg-card p-5 shadow-sm">
          <div className="mb-3 text-sm font-semibold">卡片详情与证据</div>
          {cardError ? (
            <Alert color="danger" title="加载卡片详情失败" description={cardError} />
          ) : cardLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-24 w-full rounded-xl" />
              <Skeleton className="h-24 w-full rounded-xl" />
            </div>
          ) : selectedCard ? (
            <div className="space-y-4">
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
                  evidence.map((item) => (
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
            </div>
          ) : (
            <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
              选择左侧卡片后，这里会显示摘要、详细说明和 evidence 链接。
            </div>
          )}
        </div>
      </section>

      <section className="rounded-2xl border bg-card p-5 shadow-sm">
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
    </RepoLibraryLayout>
  )
}
