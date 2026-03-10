import type { ReactNode } from 'react'

import { WorkspacePortal } from './WorkspaceShell'

type OrchestrationsShellProps = {
  title: string
  headerMeta?: ReactNode
  headerActions?: ReactNode
  sidebarTitle: string
  sidebarCount: number
  sidebarAction?: ReactNode
  sidebarContent: ReactNode
  children: ReactNode
  contentMaxWidthClassName?: string
}

/**
 * 功能：为编排页面把侧栏与右侧主体挂载到共享工作区壳层。
 * 参数/返回：接收编排页头、侧栏与内容节点，返回一组 portal 片段。
 * 失败场景：共享壳层未挂载时 portal 不显示，但页面组件可继续完成渲染流程。
 * 副作用：无额外副作用，仅向共享壳层挂载节点。
 */
export function OrchestrationsShell(props: OrchestrationsShellProps) {
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
        <div className="truncate text-base font-semibold">{title}</div>
      </WorkspacePortal>
      <WorkspacePortal target="headerActions">
        <div className="flex items-center justify-end gap-2">{headerActions}</div>
      </WorkspacePortal>
      <WorkspacePortal target="content">
        <div className="thin-scrollbar min-h-0 flex-1 overflow-y-auto">
          <div className={`mx-auto flex w-full flex-col gap-5 p-4 md:p-6 ${contentMaxWidthClassName}`}>
            {children}
          </div>
        </div>
      </WorkspacePortal>
    </>
  )
}
