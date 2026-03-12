import type { Selection } from '@react-types/shared'
import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Chip,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Select,
  SelectItem,
  Skeleton,
} from '@heroui/react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { FileSearch, Plus, RefreshCcw, ScrollText, Trash2 } from 'lucide-react'

import {
  goToChat,
  goToRepoLibraryPatternSearch,
  goToRepoLibraryRepositories,
  goToRepoLibraryRepository,
} from '@/app/routes'
import { RepoLibraryShell } from '@/app/components/RepoLibraryShell'
import { RepoLibrarySidebarRepositoryList } from '@/app/components/repo-library/RepoLibrarySidebarRepositoryList'
import {
  fetchRepoLibraryAnalysisReport,
  fetchRepoLibraryCard,
  fetchRepoLibraryCardEvidence,
  fetchRepoLibraryRepositoryView,
  deleteRepoLibraryAnalysis,
  syncRepoLibraryAnalysisChat,
  type RepoLibraryAnalysisResult,
  type RepoLibraryCard,
  type RepoLibraryCardEvidence,
  type RepoLibraryCardHydration,
  type RepoLibraryRepositorySummary,
  type RepoLibraryRepositoryViewFullResponse,
  type RepoLibraryRepositoryViewLiteResponse,
} from '@/lib/daemon'
import { formatAbsoluteTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'
import { getEmptyRepoLibraryDetailCache, useRepoLibraryUIStore } from '@/stores/repoLibraryUIStore'

type RepoLibraryRepositoryDetailPageProps = {
  repositoryId: string
  analysisId?: string
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

function parseISOToMs(value?: string | null): number | null {
  const text = (value || '').trim()
  if (!text) return null
  const ms = Date.parse(text)
  return Number.isFinite(ms) ? ms : null
}

function analysisGeneratedAtMs(analysis: RepoLibraryAnalysisResult): number {
  const reportGeneratedAt = parseISOToMs(analysis.report_context_summary?.generated_at ?? null)
  if (reportGeneratedAt) return reportGeneratedAt
  return analysis.ended_at || analysis.updated_at || analysis.created_at || 0
}

type AnalysisOptionParts = {
  ref: string
  sha: string
  model: string
  time: string
  status: string
}

function analysisOptionParts(analysis: RepoLibraryAnalysisResult): AnalysisOptionParts {
  return {
    ref: analysis.resolved_ref || analysis.requested_ref || analysis.analysis_id || '—',
    sha: analysis.commit_sha ? analysis.commit_sha.slice(0, 10) : '—',
    model: (analysis.model_id || analysis.cli_tool_id || analysis.runtime_kind || '—').trim(),
    time: formatAbsoluteTime(analysisGeneratedAtMs(analysis)),
    status: formatAnalysisStatus(analysis.status),
  }
}

function formatAnalysisOptionLabel(analysis: RepoLibraryAnalysisResult): string {
  const parts = analysisOptionParts(analysis)
  return `${parts.ref} @ ${parts.sha} · ${parts.time} · ${parts.status} · ${parts.model}`
}

function pickLatestAnalysisId(analyses: RepoLibraryAnalysisResult[]): string | null {
  if (!analyses || analyses.length === 0) return null
  let latest = analyses[0]
  for (const item of analyses) {
    if (!item.analysis_id) continue
    if ((item.updated_at ?? 0) > (latest.updated_at ?? 0)) {
      latest = item
    }
  }
  return latest.analysis_id || null
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

type ParsedCardNarrative = {
  conclusion?: string
  thinking?: string
  tradeoffs?: string
  rest: string
}

function parseCardNarrative(raw?: string | null): ParsedCardNarrative {
  const text = String(raw ?? '').trim()
  if (!text) return { rest: '' }

  const labels: Record<string, keyof Omit<ParsedCardNarrative, 'rest'>> = {
    结论: 'conclusion',
    思路: 'thinking',
    取舍: 'tradeoffs',
  }

  const lines = text.split(/\r?\n/u)
  const parts: Partial<Omit<ParsedCardNarrative, 'rest'>> = {}
  const restLines: string[] = []
  let active: keyof Omit<ParsedCardNarrative, 'rest'> | null = null

  for (const rawLine of lines) {
    const line = rawLine.trim()
    if (!line) continue

    const match = line.match(/^(\S+?)\s*[:：]\s*(.*)$/u)
    const key = match ? labels[match[1]] : undefined
    if (match && key) {
      active = key
      const value = (match[2] ?? '').trim()
      if (value) parts[key] = value
      continue
    }

    if (active) {
      const prev = String(parts[active] ?? '').trim()
      parts[active] = prev ? `${prev}\n${line}` : line
    } else {
      restLines.push(line)
    }
  }

  return {
    ...parts,
    rest: restLines.join('\n').trim(),
  }
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

function markdownWithHardBreaks(text: string): string {
  return text.replace(/\r\n/gu, '\n').replace(/\n/gu, '  \n')
}

function CardMarkdown(props: { text: string; className?: string }) {
  const normalized = props.text.trim()
  if (!normalized) return null
  return (
    <div className={`chat-markdown ${props.className ?? ''}`.trim()}>
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{markdownWithHardBreaks(normalized)}</ReactMarkdown>
    </div>
  )
}

function isRepositoryViewLiteResponse(
  body: RepoLibraryRepositoryViewLiteResponse | RepoLibraryRepositoryViewFullResponse,
): body is RepoLibraryRepositoryViewLiteResponse {
  return Boolean((body as RepoLibraryRepositoryViewLiteResponse).detail)
}

function isRepositoryViewFullResponse(
  body: RepoLibraryRepositoryViewLiteResponse | RepoLibraryRepositoryViewFullResponse,
): body is RepoLibraryRepositoryViewFullResponse {
  return typeof (body as RepoLibraryRepositoryViewFullResponse).analysis_id === 'string'
}

function shouldHideKnowledgeCard(card: RepoLibraryCard): boolean {
  if (!card) return false
  // Left panel already shows the tech stack context; avoid duplicate integration note.
  if (card.card_type === 'integration_note' && String(card.title || '').trim() === '技术栈与模块语言') return true
  // Risks are already included in the integration-note report section; do not flood the card list.
  if (card.card_type === 'risk_note') return true
  return false
}

type ReportContextSummary = {
  generated_at?: string | null
  stack_overview?: string | null
  backend_summary?: string | null
  frontend_summary?: string | null
  other_modules_summary?: string | null
}

function normalizeReportBulletValue(value: string): string {
  const trimmed = value.trim()
  if (!trimmed) return ''
  if (trimmed === '—' || trimmed === '-') return ''
  return trimmed
}

function extractH2Section(reportText: string, h2Title: string): string {
  const lines = reportText.split('\n')
  const target = h2Title.trim()

  const h2Start = lines.findIndex((line) => line.trim() === `## ${target}`)
  if (h2Start < 0) return ''

  let end = lines.length
  for (let i = h2Start + 1; i < lines.length; i += 1) {
    if (lines[i].trim().startsWith('## ')) {
      end = i
      break
    }
  }
  return lines.slice(h2Start + 1, end).join('\n').trim()
}

function extractReportContextSummary(reportText: string): ReportContextSummary | null {
  const part1 = extractH2Section(reportText, '第一部分：技术栈与模块语言')
  if (!part1) return null

  const fields: Record<string, string> = {}
  for (const raw of part1.split('\n')) {
    const line = raw.trim()
    if (!line.startsWith('- ')) continue
    const body = line.slice(2).trim()
    const sep = body.includes('：') ? '：' : ':'
    const idx = body.indexOf(sep)
    if (idx < 0) continue
    const key = body.slice(0, idx).trim()
    const value = body.slice(idx + 1).trim()
    if (!key) continue
    fields[key] = value
  }

  const summary: ReportContextSummary = {
    generated_at: normalizeReportBulletValue(fields['生成时间'] ?? ''),
    stack_overview: normalizeReportBulletValue(fields['主要语言/技术栈总览'] ?? ''),
    backend_summary: normalizeReportBulletValue(fields['后端'] ?? ''),
    frontend_summary: normalizeReportBulletValue(fields['前端'] ?? ''),
    other_modules_summary: normalizeReportBulletValue(fields['其它模块'] ?? ''),
  }

  if (
    !summary.generated_at &&
    !summary.stack_overview &&
    !summary.backend_summary &&
    !summary.frontend_summary &&
    !summary.other_modules_summary
  ) {
    return null
  }
  return summary
}

function isIntegrationSummaryCard(card: RepoLibraryCard | null): boolean {
  if (!card) return false
  if (card.card_type !== 'integration_note') return false
  return card.title === '技术栈与模块语言' || card.title === '项目用途与核心特点'
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
 * 失败场景：详情、卡片或证据请求失败时展示错误提示，并保留刷新入口。
 * 副作用：发起仓库详情/卡片/evidence 请求，并允许把分析 Chat 的最新回复同步回知识库。
 */
export function RepoLibraryRepositoryDetailPage(props: RepoLibraryRepositoryDetailPageProps) {
  const { repositoryId, analysisId } = props
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
  const setAnalysisDraft = useRepoLibraryUIStore((s) => s.setAnalysisDraft)

  const detailCache = currentDetailCache ?? fallbackDetailCache ?? getEmptyRepoLibraryDetailCache()
  const detail = detailCache.detail
  const cards = detailCache.cards
  const selectedAnalysisId = detailCache.selectedAnalysisId
  const selectedCardId = detailCache.selectedCardId
  const loading = detailCache.loading
  const error = detailCache.error
  const cardsLoading = detailCache.cardsLoading
  const cardsError = detailCache.cardsError
  const cardLoading = detailCache.cardLoading
  const cardError = detailCache.cardError
  const selectedCard = detailCache.selectedCard
  const selectedCardDetail = selectedCard?.detail ?? selectedCard?.mechanism ?? ''
  const narrative = useMemo(
    () => parseCardNarrative(selectedCardDetail),
    [selectedCardDetail],
  )
  const effectiveConclusion = useMemo(() => {
    const direct = selectedCard?.conclusion?.trim()
    if (direct) return direct
    const inferred = narrative.conclusion?.trim()
    return inferred || ''
  }, [narrative.conclusion, selectedCard?.conclusion])
  const mechanismRest = useMemo(() => {
    const rest = narrative.rest?.trim()
    if (rest) return rest
    const extracted = Boolean(narrative.conclusion || narrative.thinking || narrative.tradeoffs)
    if (extracted) return ''
    return selectedCardDetail.trim()
  }, [narrative.conclusion, narrative.rest, narrative.thinking, narrative.tradeoffs, selectedCardDetail])
  const selectedCardSummary = selectedCard
    ? displaySummary(selectedCard.summary, effectiveConclusion, mechanismRest)
    : ''
  const isProjectCharacteristicCard = selectedCard?.card_type === 'project_characteristic'
  const isIntegrationNoteCard = selectedCard?.card_type === 'integration_note'
  const evidence = detailCache.evidence
  const reportMarkdown = detailCache.reportMarkdown
  const reportExcerptMarkdown = detailCache.reportExcerptMarkdown

  const [syncingAnalysisId, setSyncingAnalysisId] = useState<string | null>(null)
  const [reportModalOpen, setReportModalOpen] = useState(false)
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const [deleteLastConfirmOpen, setDeleteLastConfirmOpen] = useState(false)
  const [deletingAnalysis, setDeletingAnalysis] = useState(false)
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

  const refresh = useCallback(async (options?: { force?: boolean; analysisId?: string | null }) => {
    const force = options?.force ?? false
    const state = useRepoLibraryUIStore.getState()
    const detailState = state.detailsByRepositoryId[repositoryId]
    const routePreferredAnalysisId = analysisId?.trim() || null

    if (!force && detailState?.loading) return

    setRepositoryDetailState(repositoryId, { loading: true, error: null })
    setRepositoriesState({ refreshing: true, error: null })
    try {
      const preferAnalysisId =
        (options?.analysisId && options.analysisId.trim() ? options.analysisId.trim() : null) ??
        (routePreferredAnalysisId && routePreferredAnalysisId.trim() ? routePreferredAnalysisId.trim() : null) ??
        null

      const view = await fetchRepoLibraryRepositoryView(daemonUrl, repositoryId, {
        mode: 'lite',
        analysis_id: preferAnalysisId || undefined,
      })
      if (!isRepositoryViewLiteResponse(view)) {
        throw new Error('unexpected repository view response')
      }

      const nextRepositories = (view.repositories ?? []) as RepoLibraryRepositorySummary[]
      setRepositoriesState({
        repositories: nextRepositories,
        loaded: true,
        refreshing: false,
        error: null,
      })

      const nextDetail = view.detail
      const analyses = nextDetail.analyses ?? []
      const resolvedSelection =
        (typeof view.selected_analysis_id === 'string' && view.selected_analysis_id.trim()) ||
        (preferAnalysisId && analyses.some((item) => item.analysis_id === preferAnalysisId) ? preferAnalysisId : null) ||
        pickLatestAnalysisId(analyses)

      const rawCards = Array.isArray(view.cards) ? view.cards : []
      const nextCards = rawCards.filter((item) => !shouldHideKnowledgeCard(item))
      const preferredCardId =
        typeof view.selected_card_id === 'string' && view.selected_card_id.trim() ? view.selected_card_id.trim() : null
      const resolvedCardId =
        (preferredCardId && nextCards.some((item) => item.card_id === preferredCardId) ? preferredCardId : null) ||
        nextCards[0]?.card_id ||
        null

      const nextCardsById = { ...(detailState?.cardsById ?? {}) }
      for (const card of rawCards) {
        if (!card?.card_id) continue
        nextCardsById[card.card_id] = card
      }
      if (view.selected_card?.card_id) {
        nextCardsById[view.selected_card.card_id] = view.selected_card
      }

      const nextEvidenceByCardId = { ...(detailState?.evidenceByCardId ?? {}) }
      if (resolvedCardId && Array.isArray(view.selected_evidence)) {
        nextEvidenceByCardId[resolvedCardId] = view.selected_evidence
      }

      setRepositoryDetailState(repositoryId, {
        detail: nextDetail,
        cards: nextCards,
        cardsById: nextCardsById,
        evidenceByCardId: nextEvidenceByCardId,
        selectedCardId: resolvedCardId,
        selectedCard: resolvedCardId ? nextCardsById[resolvedCardId] ?? null : null,
        evidence: resolvedCardId ? nextEvidenceByCardId[resolvedCardId] ?? [] : [],
        reportExcerptMarkdown: view.selected_integration_section_markdown ?? '',
        loading: false,
        loaded: true,
        error: null,
        selectedAnalysisId: resolvedSelection,
      })
      markLastViewedRepository(repositoryId)

      if (resolvedSelection) {
        void (async () => {
          try {
            const full = await fetchRepoLibraryRepositoryView(daemonUrl, repositoryId, {
              mode: 'full',
              analysis_id: resolvedSelection,
            })
            if (!isRepositoryViewFullResponse(full)) return
            const cardsFull = (full.cards_full ?? []) as RepoLibraryCardHydration[]
            const hydratedCards = cardsFull.map((item) => item.card)
            const hydratedCardsById: Record<string, RepoLibraryCard> = {}
            const hydratedEvidenceByCardId: Record<string, RepoLibraryCardEvidence[]> = {}
            for (const item of cardsFull) {
              if (!item.card?.card_id) continue
              hydratedCardsById[item.card.card_id] = item.card
              hydratedEvidenceByCardId[item.card.card_id] = item.evidence ?? []
            }
            // Merge caches best-effort without dropping any optimistic state.
            const current = useRepoLibraryUIStore.getState().detailsByRepositoryId[repositoryId]
            setRepositoryDetailState(repositoryId, {
              reportMarkdown: full.report_markdown ?? current?.reportMarkdown ?? '',
              cards: hydratedCards.filter((item) => !shouldHideKnowledgeCard(item)),
              cardsById: { ...(current?.cardsById ?? {}), ...hydratedCardsById },
              evidenceByCardId: { ...(current?.evidenceByCardId ?? {}), ...hydratedEvidenceByCardId },
            })
            const selected = useRepoLibraryUIStore.getState().detailsByRepositoryId[repositoryId]?.selectedCardId
            if (selected && hydratedCardsById[selected]) {
              setRepositoryDetailState(repositoryId, {
                selectedCard: hydratedCardsById[selected],
                evidence: hydratedEvidenceByCardId[selected] ?? [],
              })
            }
          } catch {
            // ignore hydration failures
          }
        })()
      }
    } catch (err: unknown) {
      setRepositoryDetailState(repositoryId, {
        loading: false,
        error: err instanceof Error ? err.message : String(err),
      })
      setRepositoriesState({ refreshing: false })
    }
  }, [analysisId, daemonUrl, markLastViewedRepository, repositoryId, setRepositoriesState, setRepositoryDetailState])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    const handler = (ev: Event) => {
      if (!(ev instanceof CustomEvent)) return
      const detail = (ev.detail ?? {}) as { repositoryId?: unknown }
      const target = typeof detail.repositoryId === 'string' ? detail.repositoryId : ''
      if (target && target !== repositoryId) return
      void refresh({ force: true })
    }
    window.addEventListener('repo-library:analysis-updated', handler)
    return () => window.removeEventListener('repo-library:analysis-updated', handler)
  }, [refresh, repositoryId])

  useEffect(() => {
    if (!selectedCardId) {
      setRepositoryDetailState(repositoryId, { selectedCard: null, evidence: [] })
      return
    }

    let cancelled = false

    const hydrateCard = async () => {
      const current = useRepoLibraryUIStore.getState().detailsByRepositoryId[repositoryId]
      const cachedCard = current?.cardsById?.[selectedCardId] ?? null
      const cachedEvidence = current?.evidenceByCardId?.[selectedCardId] ?? null
      if (cachedCard) {
        setRepositoryDetailState(repositoryId, {
          selectedCard: cachedCard,
          evidence: cachedEvidence ?? [],
          cardLoading: false,
          cardError: null,
        })
        if (cachedEvidence) return
      }

      setRepositoryDetailState(repositoryId, { cardLoading: true, cardError: null })
      try {
        const [nextCard, nextEvidence] = await Promise.all([
          fetchRepoLibraryCard(daemonUrl, selectedCardId),
          fetchRepoLibraryCardEvidence(daemonUrl, selectedCardId),
        ])
        if (cancelled) return
        const state = useRepoLibraryUIStore.getState().detailsByRepositoryId[repositoryId]
        setRepositoryDetailState(repositoryId, {
          selectedCard: nextCard,
          evidence: nextEvidence,
          cardsById: { ...(state?.cardsById ?? {}), [selectedCardId]: nextCard },
          evidenceByCardId: { ...(state?.evidenceByCardId ?? {}), [selectedCardId]: nextEvidence },
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

    void hydrateCard()
    return () => {
      cancelled = true
    }
  }, [daemonUrl, repositoryId, selectedCardId, setRepositoryDetailState])

  useEffect(() => {
    if (!reportModalOpen) return
    if (!selectedAnalysisId || reportMarkdown.trim()) return
    let cancelled = false
    void (async () => {
      try {
        const next = await fetchRepoLibraryAnalysisReport(daemonUrl, selectedAnalysisId)
        if (cancelled) return
        setRepositoryDetailState(repositoryId, { reportMarkdown: next.report_markdown })
      } catch {
        // ignore
      }
    })()
    return () => {
      cancelled = true
    }
  }, [daemonUrl, reportMarkdown, reportModalOpen, repositoryId, selectedAnalysisId, setRepositoryDetailState])

  const analyses = useMemo(() => detail?.analyses ?? [], [detail?.analyses])
  const repository = detail?.repository ?? null
  const selectedAnalysis = useMemo(
    () => analyses.find((item) => item.analysis_id === selectedAnalysisId) ?? analyses[0] ?? null,
    [analyses, selectedAnalysisId],
  )

  const onSyncLatestChatReply = useCallback(
    async (analysis: RepoLibraryAnalysisResult | null) => {
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

  const canDeleteSelectedAnalysis = useMemo(() => {
    if (!selectedAnalysis?.analysis_id) return false
    const status = (selectedAnalysis.status || '').trim()
    if (status === 'queued' || status === 'running') return false
    return true
  }, [selectedAnalysis?.analysis_id, selectedAnalysis?.status])

  const isDeletingLastAnalysis = useMemo(() => {
    if (!selectedAnalysis?.analysis_id) return false
    return analyses.length === 1
  }, [analyses.length, selectedAnalysis?.analysis_id])

  const onDeleteSelectedAnalysis = useCallback(async (opts?: { delete_repository_if_last?: boolean }) => {
    if (!selectedAnalysis?.analysis_id) return
    if (!canDeleteSelectedAnalysis) {
      toast({ variant: 'destructive', title: '分析进行中，暂不支持删除' })
      return
    }
    setDeletingAnalysis(true)
    try {
      const res = await deleteRepoLibraryAnalysis(daemonUrl, selectedAnalysis.analysis_id, {
        delete_repository_if_last: opts?.delete_repository_if_last ?? false,
      })
      if (res.deleted_repository) {
        toast({ title: '已删除最后一份分析并移除仓库' })
        goToRepoLibraryRepositories()
        return
      }
      toast({ title: '已删除分析报告' })
      await refresh({ force: true })
    } catch (err: unknown) {
      toast({ variant: 'destructive', title: '删除失败', description: err instanceof Error ? err.message : String(err) })
    } finally {
      setDeletingAnalysis(false)
    }
  }, [canDeleteSelectedAnalysis, daemonUrl, refresh, selectedAnalysis?.analysis_id])

  const sidebarContent = (
    <RepoLibrarySidebarRepositoryList
      repositories={repositories}
      loaded={repositoriesLoaded}
      loading={repositoriesLoading}
      error={repositoriesError}
      activeRepositoryId={repositoryId}
      emptyHint="暂无仓库列表。"
      onSelect={(id) => goToRepoLibraryRepository(id)}
    />
  )

  const reportAvailable = Boolean(reportMarkdown || selectedAnalysis?.report_path)
  const selectedRefLabel = selectedAnalysis?.resolved_ref || selectedAnalysis?.requested_ref || '未选择分析'
  const selectedAnalysisLabel = selectedAnalysis?.analysis_id || '未选择分析'
  const selectedCommitShort = selectedAnalysis?.commit_sha ? selectedAnalysis.commit_sha.slice(0, 10) : '—'
  const reportTextForUI = reportMarkdown || reportExcerptMarkdown
  const reportContext = selectedAnalysis?.report_context_summary ?? extractReportContextSummary(reportTextForUI)
  const generatedAtLabel = formatGeneratedAt(reportContext?.generated_at)
  const selectedIntegrationSectionMarkdown = selectedCard
    ? (selectedCard.title === '技术栈与模块语言'
        ? extractH2Section(reportTextForUI, '第一部分：技术栈与模块语言')
        : selectedCard.title === '项目用途与核心特点'
          ? extractH2Section(reportTextForUI, '第二部分：项目用途与核心特点')
          : '')
    : ''

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
                    <div className="grid gap-3">
                      <div className="min-w-0">
                        <div className="mb-1.5 pl-1 text-[11px] uppercase tracking-[0.16em] text-muted-foreground/75">分析</div>
                        <Select
                          aria-label="选择分析"
                          size="sm"
                          variant="bordered"
                          className="w-full"
                          selectedKeys={selectedAnalysisId ? new Set([selectedAnalysisId]) : new Set([])}
                          isDisabled={analyses.length === 0}
                          disallowEmptySelection
                          onSelectionChange={(keys) => {
                            const next = selectionToString(keys)
                            if (!next) return
                            void refresh({ force: true, analysisId: next })
                          }}
                        >
                          {analyses.map((analysis) => (
                            <SelectItem
                              key={analysis.analysis_id}
                              textValue={formatAnalysisOptionLabel(analysis)}
                            >
                              <span className="block whitespace-normal break-words leading-snug">
                                {formatAnalysisOptionLabel(analysis)}
                              </span>
                            </SelectItem>
                          ))}
                        </Select>
                      </div>
                    </div>

                    <div className="mt-3 flex flex-wrap items-center gap-2">
                      <Chip variant="flat" size="sm" className="bg-default-100/85">
                        {selectedRefLabel}
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
                              <span className="ml-2 text-foreground/90">{formatAbsoluteTime(selectedAnalysis.updated_at || selectedAnalysis.created_at || 0)}</span>
                            </div>
                          </div>
                        </div>

                        {selectedAnalysis.error_message ? (
                          <Alert color="danger" title="分析失败" description={selectedAnalysis.error_message} className="mt-3" />
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
                          <Button
                            variant="light"
                            size="sm"
                            onPress={() => {
                              const repoUrl = detail?.repository?.repo_url || ''
                              if (!repoUrl.trim()) {
                                toast({ variant: 'destructive', title: '缺少仓库地址，无法复用分析参数' })
                                return
                              }
                              setAnalysisDraft({
                                repo_url: repoUrl,
                                ref:
                                  (selectedAnalysis.resolved_ref || selectedAnalysis.requested_ref || 'HEAD').trim() ||
                                  'HEAD',
                                features: selectedAnalysis.features ?? [],
                                depth: (selectedAnalysis.depth as any) || 'standard',
                                language: (selectedAnalysis.language as any) || 'zh-CN',
                                analyzer_mode: (selectedAnalysis.agent_mode as any) || 'full',
                                cli_tool_id: selectedAnalysis.cli_tool_id || undefined,
                                model_id: selectedAnalysis.model_id || undefined,
                              })
                              goToRepoLibraryRepositories()
                            }}
                          >
                            用其他模型分析
                          </Button>
                          <Button
                            variant="light"
                            size="sm"
                            color="danger"
                            startContent={<Trash2 className="h-4 w-4" aria-hidden="true" focusable="false" />}
                            isDisabled={!selectedAnalysis?.analysis_id || !canDeleteSelectedAnalysis}
                            isLoading={deletingAnalysis}
                            onPress={() => setDeleteModalOpen(true)}
                          >
                            删除报告
                          </Button>
                        </div>
                      </>
                    ) : (
                      <div className="mt-4 rounded-[22px] border border-dashed border-default-300/80 px-4 py-5 text-sm text-muted-foreground">
                        当前仓库暂无可选分析结果。
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
                      {selectedAnalysis ? (
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
                          {isIntegrationSummaryCard(selectedCard) && selectedIntegrationSectionMarkdown ? (
                            <div className="chat-markdown mt-5 rounded-2xl border bg-background/80 p-4 text-sm leading-7 text-foreground">
                              <ReactMarkdown remarkPlugins={[remarkGfm]}>{selectedIntegrationSectionMarkdown}</ReactMarkdown>
                            </div>
                          ) : (
                            <>
                              {effectiveConclusion ? (
                                <div className="mt-5 rounded-[22px] border border-primary/20 bg-primary/5 px-4 py-3.5 text-[15px] font-medium leading-7 text-foreground">
                                  <CardMarkdown text={effectiveConclusion} className="text-[15px] font-medium leading-7 text-foreground" />
                                </div>
                              ) : null}
                              {selectedCardSummary && !isProjectCharacteristicCard && !isIntegrationNoteCard ? (
                                <CardMarkdown text={selectedCardSummary} className="mt-4 text-sm leading-7 text-muted-foreground" />
                              ) : null}
                              {narrative.thinking ? (
                                <div className="mt-5 space-y-2 border-t border-default-200/70 pt-4">
                                  {isProjectCharacteristicCard ? null : (
                                    <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground/75">思路</div>
                                  )}
                                  <CardMarkdown
                                    text={narrative.thinking}
                                    className={`text-sm leading-7 ${isProjectCharacteristicCard ? 'text-foreground' : 'text-foreground/85'}`}
                                  />
                                </div>
                              ) : null}
                              {narrative.tradeoffs ? (
                                <div className="mt-5 space-y-2 border-t border-default-200/70 pt-4">
                                  <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground/75">取舍</div>
                                  <CardMarkdown text={narrative.tradeoffs} className={`text-sm leading-7 ${isProjectCharacteristicCard ? 'text-foreground' : 'text-foreground/85'}`} />
                                </div>
                              ) : null}
                              {mechanismRest ? (
                                <div className="mt-5 space-y-2 border-t border-default-200/70 pt-4">
                                  <div className="text-xs uppercase tracking-[0.16em] text-muted-foreground/75">实现机制</div>
                                  <CardMarkdown text={mechanismRest} className={`text-sm leading-7 ${isProjectCharacteristicCard ? 'text-foreground' : 'text-foreground/85'}`} />
                                </div>
                              ) : !selectedCardSummary && !effectiveConclusion ? (
                                <div className="mt-4 text-sm text-muted-foreground">暂无详细说明</div>
                              ) : null}
                            </>
                          )}
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

                        {isIntegrationSummaryCard(selectedCard) ? null : (
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
                        )}
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
            ) : selectedAnalysis?.report_path ? (
              <div className="rounded-xl border bg-muted/20 p-4 text-sm text-muted-foreground">
                <div>当前分析已生成报告文件，但尚未加载到内存。</div>
                <code className="mt-2 block break-all text-xs text-foreground">{selectedAnalysis.report_path}</code>
              </div>
            ) : (
              <div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">当前分析暂无可展示的报告内容。</div>
            )}
          </ModalBody>
        </ModalContent>
      </Modal>

      <Modal isOpen={deleteModalOpen} onOpenChange={setDeleteModalOpen} size="lg">
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2">
                <Trash2 className="h-4 w-4" />
                删除分析报告
              </ModalHeader>
              <ModalBody className="space-y-2 text-sm text-muted-foreground">
                <div>将删除当前选中的这一次分析结果（包含报告、知识卡片与检索索引中的对应内容）。</div>
                <div className="text-foreground">
                  目标：<code className="text-xs">{selectedAnalysis?.analysis_id || '—'}</code>
                </div>
              </ModalBody>
              <ModalFooter>
                <Button variant="light" onPress={onClose} isDisabled={deletingAnalysis}>
                  取消
                </Button>
                <Button
                  color="danger"
                  variant="flat"
                  isLoading={deletingAnalysis}
                  onPress={async () => {
                    onClose()
                    if (isDeletingLastAnalysis) {
                      setDeleteLastConfirmOpen(true)
                      return
                    }
                    await onDeleteSelectedAnalysis({ delete_repository_if_last: false })
                  }}
                >
                  确认删除
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      <Modal isOpen={deleteLastConfirmOpen} onOpenChange={setDeleteLastConfirmOpen} size="lg">
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader className="flex items-center gap-2 text-danger-600">
                <Trash2 className="h-4 w-4" />
                最后一份分析报告
              </ModalHeader>
              <ModalBody className="space-y-2 text-sm text-muted-foreground">
                <div>当前仓库只有这一份分析报告。</div>
                <div className="text-foreground">继续删除将同时移除整个仓库记录与落盘目录。</div>
                <div className="text-foreground">
                  仓库：<code className="text-xs">{detail?.repository?.repo_url || '—'}</code>
                </div>
              </ModalBody>
              <ModalFooter>
                <Button variant="light" onPress={onClose} isDisabled={deletingAnalysis}>
                  取消
                </Button>
                <Button
                  color="danger"
                  variant="solid"
                  isLoading={deletingAnalysis}
                  onPress={async () => {
                    onClose()
                    await onDeleteSelectedAnalysis({ delete_repository_if_last: true })
                  }}
                >
                  删除仓库及报告
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </>
  )
}
