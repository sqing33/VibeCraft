import type { ReactNode } from 'react'
import { Alert, Skeleton } from '@heroui/react'
import { LoaderCircle, X } from 'lucide-react'

import { RepoLibrarySidebarRepositoryItem } from '@/app/components/RepoLibraryShell'
import type { RepoLibraryRepositorySummary } from '@/lib/daemon'

type RepoLibrarySidebarRepositoryListProps = {
  repositories: RepoLibraryRepositorySummary[]
  loaded: boolean
  loading: boolean
  error: string | null
  activeRepositoryId?: string
  emptyHint?: string
  onSelect: (repositoryId: string) => void
  renderAction?: (item: RepoLibraryRepositorySummary) => ReactNode
}

function LoadingSkeleton() {
  return (
    <div className="space-y-1">
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
      <Skeleton className="h-[58px] w-full rounded-[22px]" />
    </div>
  )
}

function StatusIndicator(props: { status?: string | null }) {
  const status = (props.status ?? '').trim()
  if (status === 'queued' || status === 'running') {
    return (
      <span title={status === 'queued' ? '排队中' : '分析中'} className="flex items-center">
        <LoaderCircle className="h-4 w-4 animate-spin text-primary/80" aria-hidden="true" focusable="false" />
      </span>
    )
  }
  if (status === 'failed') {
    return (
      <span title="失败" className="flex items-center">
        <X className="h-4 w-4 text-danger-500" aria-hidden="true" focusable="false" />
      </span>
    )
  }
  return null
}

/**
 * 功能：Repo Library 左侧仓库列表的统一组件，保证行高/间距一致。
 * 参数/返回：接收仓库列表状态与点击回调；返回侧栏列表内容节点。
 * 失败场景：加载失败展示错误提示；无数据展示 emptyHint。
 * 副作用：无。
 */
export function RepoLibrarySidebarRepositoryList(props: RepoLibrarySidebarRepositoryListProps) {
  const {
    repositories,
    loaded,
    loading,
    error,
    activeRepositoryId,
    emptyHint = '暂无仓库列表。',
    onSelect,
    renderAction,
  } = props

  const hasCache = loaded || repositories.length > 0

  return (
    <div className="relative min-h-[120px]">
      {error && !hasCache ? <Alert color="danger" title="加载仓库失败" description={error} /> : null}
      {!hasCache && loading ? (
        <LoadingSkeleton />
      ) : repositories.length === 0 ? (
        <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">{emptyHint}</div>
      ) : (
        <div className="space-y-1">
          {repositories.map((item) => (
            <RepoLibrarySidebarRepositoryItem
              key={item.repository_id}
              title={item.full_name || item.name || item.repo_url}
              subtitle=""
              meta=""
              active={activeRepositoryId ? item.repository_id === activeRepositoryId : false}
              onPress={() => onSelect(item.repository_id)}
              action={
                item.latest_analysis?.status || renderAction ? (
                  <div className="flex items-center gap-2">
                    <StatusIndicator status={item.latest_analysis?.status} />
                    {renderAction ? renderAction(item) : null}
                  </div>
                ) : undefined
              }
            />
          ))}
        </div>
      )}
    </div>
  )
}
