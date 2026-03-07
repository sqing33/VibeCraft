import { useEffect, useMemo, useState } from 'react'
import { Alert, Button, Chip, Input, Skeleton, Textarea } from '@heroui/react'
import { FolderSearch, RefreshCcw } from 'lucide-react'

import { goToRepoLibraryRepository } from '@/app/routes'
import {
  fetchRepoLibraryRepositories,
  searchRepoLibrary,
  type RepoLibraryRepositorySummary,
  type RepoLibrarySearchRequest,
  type RepoLibrarySearchResult,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

import { RepoLibraryLayout } from './RepoLibraryLayout'

function formatScore(score?: number): string {
  if (typeof score !== 'number') return '—'
  return score <= 1 ? `${(score * 100).toFixed(1)}%` : score.toFixed(3)
}

function PatternSearchSkeleton() {
  return (
    <div className="space-y-3">
      <Skeleton className="h-28 w-full rounded-xl" />
      <Skeleton className="h-28 w-full rounded-xl" />
      <Skeleton className="h-28 w-full rounded-xl" />
    </div>
  )
}

/**
 * 功能：提供 Repo Library 的语义模式搜索，并支持按仓库过滤结果。
 * 参数/返回：无入参；返回模式搜索页面。
 * 失败场景：搜索或仓库过滤列表加载失败时展示错误提示，并允许用户重试。
 * 副作用：发起仓库列表与搜索请求；点击结果会跳转到相关仓库详情页。
 */
export function RepoLibraryPatternSearchPage() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const health = useDaemonStore((s) => s.health)

  const [repositories, setRepositories] = useState<RepoLibraryRepositorySummary[]>([])
  const [repositoriesLoading, setRepositoriesLoading] = useState(false)
  const [repositoriesError, setRepositoriesError] = useState<string | null>(null)

  const [query, setQuery] = useState('认证流程是如何落地的？')
  const [limit, setLimit] = useState('8')
  const [selectedRepositoryIds, setSelectedRepositoryIds] = useState<string[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [results, setResults] = useState<RepoLibrarySearchResult[]>([])
  const [searchError, setSearchError] = useState<string | null>(null)
  const [lastQueryAt, setLastQueryAt] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false

    const loadRepositories = async () => {
      setRepositoriesLoading(true)
      setRepositoriesError(null)
      try {
        const list = await fetchRepoLibraryRepositories(daemonUrl)
        if (cancelled) return
        setRepositories(list)
      } catch (err: unknown) {
        if (cancelled) return
        setRepositoriesError(err instanceof Error ? err.message : String(err))
      } finally {
        if (!cancelled) setRepositoriesLoading(false)
      }
    }

    void loadRepositories()
    return () => {
      cancelled = true
    }
  }, [daemonUrl])

  const repositoriesById = useMemo(
    () => new Map(repositories.map((item) => [item.repository_id, item])),
    [repositories],
  )

  const toggleRepositoryFilter = (repositoryId: string) => {
    setSelectedRepositoryIds((current) =>
      current.includes(repositoryId)
        ? current.filter((item) => item !== repositoryId)
        : [...current, repositoryId],
    )
  }

  const onSearch = async () => {
    if (!query.trim()) {
      toast({ variant: 'destructive', title: '请输入自然语言查询' })
      return
    }

    const req: RepoLibrarySearchRequest = {
      query: query.trim(),
      repository_ids: selectedRepositoryIds.length > 0 ? selectedRepositoryIds : undefined,
      limit: Number(limit) > 0 ? Number(limit) : 8,
    }

    setSubmitting(true)
    setSearchError(null)
    try {
      const next = await searchRepoLibrary(daemonUrl, req)
      setResults(next.results)
      setLastQueryAt(Date.now())
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setSearchError(message)
      setResults([])
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <RepoLibraryLayout
      activeNav="search"
      title="Repo Library 模式搜索"
      description="用自然语言跨仓库查找相似实现模式。搜索结果会保留仓库、快照、卡片和 evidence 上下文，方便快速回到源头。"
      meta={
        <Chip variant="flat" color={health.status === 'ok' ? 'success' : 'default'}>
          {health.status === 'ok' ? 'Daemon 已连接' : 'Daemon 未就绪'}
        </Chip>
      }
      actions={
        <Button
          variant="light"
          size="sm"
          startContent={<RefreshCcw className="h-4 w-4" />}
          onPress={() => window.location.reload()}
        >
          刷新页面
        </Button>
      }
    >
      <section className="rounded-2xl border bg-card p-5 shadow-sm">
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-lg font-semibold">
              <FolderSearch className="h-5 w-5" />
              Pattern Search
            </div>
            <div className="mt-1 text-sm text-muted-foreground">
              输入一个自然语言问题，例如“多租户权限如何建模”或“认证流程如何组织”，系统会返回带仓库上下文的相关卡片。
            </div>
          </div>
          <Chip variant="bordered" size="sm">
            支持语义检索
          </Chip>
        </div>

        <div className="grid gap-3 md:grid-cols-[1fr_180px]">
          <Textarea
            label="查询语句"
            minRows={3}
            placeholder="例如：认证流程是如何落地的？"
            value={query}
            onValueChange={setQuery}
          />
          <Input label="结果数" placeholder="8" value={limit} onValueChange={setLimit} />
        </div>

        <div className="mt-4 space-y-3">
          <div className="flex items-center justify-between gap-3">
            <div className="text-sm font-medium">仓库筛选</div>
            {selectedRepositoryIds.length > 0 ? (
              <Button variant="light" size="sm" onPress={() => setSelectedRepositoryIds([])}>
                清空筛选
              </Button>
            ) : null}
          </div>

          {repositoriesError ? (
            <Alert color="danger" title="加载仓库筛选失败" description={repositoriesError} />
          ) : repositoriesLoading ? (
            <div className="flex flex-wrap gap-2">
              <Skeleton className="h-8 w-28 rounded-full" />
              <Skeleton className="h-8 w-28 rounded-full" />
              <Skeleton className="h-8 w-28 rounded-full" />
            </div>
          ) : repositories.length === 0 ? (
            <div className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
              暂无可筛选的仓库。完成至少一次分析后，这里会出现仓库标签。
            </div>
          ) : (
            <div className="flex flex-wrap gap-2">
              {repositories.map((item) => {
                const active = selectedRepositoryIds.includes(item.repository_id)
                return (
                  <Button
                    key={item.repository_id}
                    variant={active ? 'flat' : 'light'}
                    size="sm"
                    onPress={() => toggleRepositoryFilter(item.repository_id)}
                  >
                    {item.full_name || item.repository_id}
                  </Button>
                )
              })}
            </div>
          )}
        </div>

        <div className="mt-4 flex justify-end">
          <Button color="primary" isLoading={submitting} onPress={onSearch}>
            开始搜索
          </Button>
        </div>
      </section>

      <section className="space-y-3">
        <div className="flex flex-wrap items-center gap-2 text-sm font-semibold text-muted-foreground">
          搜索结果
          {lastQueryAt ? (
            <Chip variant="bordered" size="sm">
              更新于 {formatRelativeTime(lastQueryAt)}
            </Chip>
          ) : null}
        </div>

        {searchError ? (
          <Alert color="danger" title="模式搜索失败" description={searchError} />
        ) : submitting ? (
          <PatternSearchSkeleton />
        ) : results.length === 0 ? (
          <div className="rounded-2xl border border-dashed p-6 text-sm text-muted-foreground">
            还没有搜索结果。先输入一个自然语言问题试试。
          </div>
        ) : (
          <div className="space-y-3">
            {results.map((item, index) => {
              const repository = item.repository ?? repositoriesById.get(item.repository_id)
              return (
                <div key={item.result_id || `${item.repository_id}-${item.card_id || index}`} className="rounded-2xl border bg-card p-5 shadow-sm">
                  <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div className="space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <div className="text-base font-semibold">{item.title || item.card?.title || '未命名结果'}</div>
                        <Chip variant="flat" size="sm">
                          相关度 {formatScore(item.score)}
                        </Chip>
                        {item.card?.card_type ? (
                          <Chip variant="bordered" size="sm">
                            {item.card.card_type}
                          </Chip>
                        ) : null}
                      </div>
                      <div className="text-sm text-muted-foreground">
                        {item.summary || item.card?.summary || item.rationale || '暂无摘要'}
                      </div>
                    </div>

                    <Button variant="flat" size="sm" onPress={() => goToRepoLibraryRepository(item.repository_id)}>
                      打开仓库详情
                    </Button>
                  </div>

                  <div className="mt-4 grid gap-3 text-sm md:grid-cols-3">
                    <div className="rounded-xl border bg-muted/20 p-3">
                      <div className="text-xs uppercase tracking-wide text-muted-foreground/80">仓库</div>
                      <div className="mt-1 font-medium text-foreground">
                        {repository?.full_name || item.repository_id}
                      </div>
                    </div>
                    <div className="rounded-xl border bg-muted/20 p-3">
                      <div className="text-xs uppercase tracking-wide text-muted-foreground/80">快照</div>
                      <div className="mt-1 font-medium text-foreground">
                        {item.snapshot?.resolved_ref || item.snapshot?.commit_sha?.slice(0, 12) || item.snapshot_id || '—'}
                      </div>
                    </div>
                    <div className="rounded-xl border bg-muted/20 p-3">
                      <div className="text-xs uppercase tracking-wide text-muted-foreground/80">卡片</div>
                      <div className="mt-1 font-medium text-foreground">{item.card?.title || item.card_id || '—'}</div>
                    </div>
                  </div>

                  {item.evidence_preview && item.evidence_preview.length > 0 ? (
                    <div className="mt-4 space-y-2">
                      <div className="text-sm font-medium">Evidence 预览</div>
                      {item.evidence_preview.slice(0, 2).map((evidence) => (
                        <div
                          key={evidence.evidence_id || `${evidence.source_path}-${evidence.start_line ?? 'na'}`}
                          className="rounded-xl border bg-muted/20 p-4"
                        >
                          <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                            <code>{evidence.source_path}</code>
                            {typeof evidence.start_line === 'number' ? (
                              <Chip variant="bordered" size="sm">
                                {evidence.start_line}
                                {typeof evidence.end_line === 'number' && evidence.end_line !== evidence.start_line ? `-${evidence.end_line}` : ''}
                              </Chip>
                            ) : null}
                            {evidence.label ? <Chip variant="flat" size="sm">{evidence.label}</Chip> : null}
                          </div>
                          {evidence.excerpt ? (
                            <pre className="mt-3 whitespace-pre-wrap rounded-lg border bg-background p-3 text-xs leading-6">
                              {evidence.excerpt}
                            </pre>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>
              )
            })}
          </div>
        )}
      </section>
    </RepoLibraryLayout>
  )
}
