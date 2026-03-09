import { useCallback, useEffect, useMemo } from 'react'
import { Alert, Button, Chip, Input, Skeleton, Textarea } from '@heroui/react'
import { FolderSearch, Plus, RefreshCcw } from 'lucide-react'

import { goToRepoLibraryRepositories, goToRepoLibraryRepository } from '@/app/routes'
import { LoadingVeil } from '@/app/components/LoadingVeil'
import { RepoLibraryShell, RepoLibrarySidebarRepositoryItem } from '@/app/components/RepoLibraryShell'
import {
  fetchRepoLibraryRepositories,
  searchRepoLibrary,
  type RepoLibrarySearchRequest,
} from '@/lib/daemon'
import { formatRelativeTime } from '@/lib/time'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'
import { useRepoLibraryUIStore } from '@/stores/repoLibraryUIStore'

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
 * 功能：提供 Repo Library 的语义知识库检索，并支持按仓库跳转结果。
 * 参数/返回：无入参；返回知识库检索页面。
 * 失败场景：搜索或仓库列表加载失败时展示错误提示，并允许用户重试。
 * 副作用：发起仓库列表与搜索请求；点击结果会跳转到相关仓库详情页。
 */
export function RepoLibraryPatternSearchPage() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)

  const repositories = useRepoLibraryUIStore((s) => s.repositories)
  const repositoriesLoaded = useRepoLibraryUIStore((s) => s.repositoriesLoaded)
  const repositoriesLoading = useRepoLibraryUIStore((s) => s.repositoriesRefreshing)
  const repositoriesError = useRepoLibraryUIStore((s) => s.repositoriesError)
  const setRepositoriesState = useRepoLibraryUIStore((s) => s.setRepositoriesState)

  const query = useRepoLibraryUIStore((s) => s.searchQuery)
  const limit = useRepoLibraryUIStore((s) => s.searchLimit)
  const results = useRepoLibraryUIStore((s) => s.searchResults)
  const searchLoaded = useRepoLibraryUIStore((s) => s.searchLoaded)
  const submitting = useRepoLibraryUIStore((s) => s.searchSubmitting)
  const searchError = useRepoLibraryUIStore((s) => s.searchError)
  const lastQueryAt = useRepoLibraryUIStore((s) => s.searchLastQueryAt)
  const setSearchDraft = useRepoLibraryUIStore((s) => s.setSearchDraft)
  const setSearchState = useRepoLibraryUIStore((s) => s.setSearchState)

  const hasRepositoryCache = useMemo(
    () => repositoriesLoaded || repositories.length > 0,
    [repositories.length, repositoriesLoaded],
  )
  const hasSearchCache = useMemo(
    () => searchLoaded || results.length > 0,
    [results.length, searchLoaded],
  )

  const loadRepositories = useCallback(async (options?: { force?: boolean }) => {
    const force = options?.force ?? false
    if (!force && useRepoLibraryUIStore.getState().repositoriesRefreshing) return

    setRepositoriesState({ refreshing: true, error: null })
    try {
      setRepositoriesState({
        repositories: await fetchRepoLibraryRepositories(daemonUrl),
        loaded: true,
        refreshing: false,
        error: null,
      })
    } catch (err: unknown) {
      setRepositoriesState({ refreshing: false, error: err instanceof Error ? err.message : String(err) })
    }
  }, [daemonUrl, setRepositoriesState])

  useEffect(() => {
    void loadRepositories()
  }, [loadRepositories])

  const repositoriesById = useMemo(
    () => new Map(repositories.map((item) => [item.repository_id, item])),
    [repositories],
  )

  const onSearch = async () => {
    if (!query.trim()) {
      toast({ variant: 'destructive', title: '请输入自然语言查询' })
      return
    }

    const req: RepoLibrarySearchRequest = {
      query: query.trim(),
      limit: Number(limit) > 0 ? Number(limit) : 8,
    }

    setSearchState({ submitting: true, error: null })
    try {
      const next = await searchRepoLibrary(daemonUrl, req)
      setSearchState({
        results: next.results,
        loaded: true,
        submitting: false,
        error: null,
        lastQueryAt: Date.now(),
      })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setSearchState({ submitting: false, error: message })
    }
  }

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
          暂无仓库。完成至少一次分析后，这里会出现仓库列表。
        </div>
      ) : (
        <div className="space-y-2">
          {repositories.map((item) => (
            <RepoLibrarySidebarRepositoryItem
              key={item.repository_id}
              title={item.full_name || item.name || item.repo_url}
              subtitle={item.repo_url}
              meta={formatRelativeTime(item.created_at || item.updated_at || 0)}
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
      title="知识库检索"
      headerMeta={
        <div className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
          <span>知识库检索</span>
          <span>·</span>
          <span>全仓库范围</span>
        </div>
      }
      headerActions={
        <>
          <Button variant="flat" size="sm" onPress={goToRepoLibraryRepositories}>
            添加仓库
          </Button>
          <Button variant="light" size="sm" startContent={<RefreshCcw className="h-4 w-4" />} onPress={() => void loadRepositories({ force: true })}>
            刷新仓库
          </Button>
        </>
      }
      sidebarTitle="仓库"
      sidebarCount={repositories.length}
      sidebarAction={
        <Button
          color="primary"
          size="sm"
          className="w-[25%] min-w-[86px] rounded-2xl"
          startContent={<Plus className="h-4 w-4 shrink-0 stroke-[3]" />}
          onPress={goToRepoLibraryRepositories}
        >
          添加仓库
        </Button>
      }
      sidebarContent={sidebarContent}
    >
      <div className="relative">
        {repositoriesError && hasRepositoryCache ? <Alert color="danger" title="刷新失败，已保留上次内容" description={repositoriesError} className="mb-4" /> : null}

        <section className="rounded-2xl border bg-card p-5 shadow-sm">
          <div className="mb-4 flex items-start justify-between gap-3">
            <div>
              <div className="flex items-center gap-2 text-lg font-semibold">
                <FolderSearch className="h-5 w-5" />
                知识库检索
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
              onValueChange={(value) => setSearchDraft({ query: value })}
            />
            <Input label="结果数" placeholder="8" value={limit} onValueChange={(value) => setSearchDraft({ limit: value })} />
          </div>
          <div className="mt-4 flex justify-end">
            <Button color="primary" isLoading={submitting} onPress={onSearch}>
              开始搜索
            </Button>
          </div>
        </section>

        <section className="mt-5 space-y-3">
          <div className="flex flex-wrap items-center gap-2 text-sm font-semibold text-muted-foreground">
            搜索结果
            {lastQueryAt ? (
              <Chip variant="bordered" size="sm">
                更新于 {formatRelativeTime(lastQueryAt)}
              </Chip>
            ) : null}
          </div>

          {searchError && !hasSearchCache ? (
            <Alert color="danger" title="知识库检索失败" description={searchError} />
          ) : !hasSearchCache && submitting ? (
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
                  </div>
                )
              })}
            </div>
          )}
        </section>

        <LoadingVeil visible={submitting && hasSearchCache} label="正在刷新检索结果…" />
      </div>
    </RepoLibraryShell>
  )
}
