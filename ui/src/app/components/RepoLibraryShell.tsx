import type { ReactNode } from 'react'

import { WorkspacePortal } from './WorkspaceShell'

type RepoLibraryShellProps = {
  title: ReactNode
  headerMeta?: ReactNode
  headerActions?: ReactNode
  sidebarTitle: string
  sidebarCount: number
  sidebarAction?: ReactNode
  sidebarContent: ReactNode
  children: ReactNode
  contentMaxWidthClassName?: string
  contentPaddingClassName?: string
}

type RepoLibrarySidebarRepositoryItemProps = {
  title: string
  subtitle: string
  meta: string
  active?: boolean
  onPress: () => void
}

export function RepoLibrarySidebarRepositoryItem(props: RepoLibrarySidebarRepositoryItemProps) {
  const { title, subtitle, meta, active = false, onPress } = props

  return (
    <button
      type="button"
      className={`w-full rounded-[22px] border px-3 py-3 text-left transition ${
        active
          ? 'border-primary/50 bg-primary/5 shadow-sm'
          : 'border-transparent bg-background/40 hover:border-default-200 hover:bg-background/80'
      }`}
      onClick={onPress}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0 truncate text-sm font-medium">{title}</div>
        <span className="shrink-0 text-xs text-muted-foreground">{meta}</span>
      </div>
      <div className="mt-1 truncate text-xs text-muted-foreground">{subtitle}</div>
    </button>
  )
}

/**
 * 功能：为 Repo Library 页面把侧栏与右侧内容投递到共享工作区壳层。
 * 参数/返回：接收知识库页头、侧栏与内容节点，返回一组 portal 片段。
 * 失败场景：共享壳层未挂载时 portal 不显示，但页面本身不会抛出同步错误。
 * 副作用：无额外副作用，仅向共享壳层挂载节点。
 */
export function RepoLibraryShell(props: RepoLibraryShellProps) {
  const {
    title,
    headerMeta,
    headerActions,
    sidebarTitle,
    sidebarCount,
    sidebarAction,
    sidebarContent,
    children,
    contentMaxWidthClassName = 'max-w-[980px]',
    contentPaddingClassName = 'gap-5 p-4 md:p-6',
  } = props

  return (
    <>
      <WorkspacePortal target="sidebarHeader">
        <div className="mb-3 flex items-center justify-between gap-3 px-1">
          <div className="flex min-w-0 items-center gap-2">
            <div className="text-sm font-semibold">{sidebarTitle}</div>
            <span className="flex h-6 min-w-6 items-center justify-center rounded-full bg-default-100 px-2 text-xs font-medium text-muted-foreground">
              {sidebarCount}
            </span>
          </div>
          {sidebarAction}
        </div>
      </WorkspacePortal>

      <WorkspacePortal target="sidebarBody">{sidebarContent}</WorkspacePortal>
      <WorkspacePortal target="headerMeta">{headerMeta ?? <div />}</WorkspacePortal>
      <WorkspacePortal target="headerTitle">
        <div className="min-w-0 text-base font-semibold">{title}</div>
      </WorkspacePortal>
      <WorkspacePortal target="headerActions">
        <div className="flex items-center justify-end gap-2">{headerActions}</div>
      </WorkspacePortal>
      <WorkspacePortal target="content">
        <div className="flex min-h-0 flex-1 flex-col">
          <div className={`mx-auto flex w-full flex-col ${contentPaddingClassName} ${contentMaxWidthClassName}`}>
            {children}
          </div>
        </div>
      </WorkspacePortal>
    </>
  )
}
