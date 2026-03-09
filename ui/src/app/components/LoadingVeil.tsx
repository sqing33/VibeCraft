import { Skeleton } from '@heroui/react'

import { cn } from '@/lib/utils'

type LoadingVeilProps = {
  visible: boolean
  className?: string
  compact?: boolean
  label?: string
}

/**
 * 功能：在保留旧内容的前提下，为正在后台刷新的区域提供轻量遮罩反馈。
 * 参数/返回：接收是否可见、紧凑模式与文案；返回覆盖层节点或 null。
 * 失败场景：不可见时不渲染任何节点。
 * 副作用：无副作用，仅渲染视觉反馈。
 */
export function LoadingVeil(props: LoadingVeilProps) {
  const { visible, className, compact = false, label = '正在更新…' } = props
  if (!visible) return null

  return (
    <div
      className={cn(
        'pointer-events-none absolute inset-0 z-10 flex items-start justify-center rounded-inherit bg-background/55 backdrop-blur-[1.5px] transition-opacity',
        compact ? 'p-3' : 'p-4',
        className,
      )}
      aria-hidden="true"
    >
      <div className="w-full max-w-sm rounded-2xl border border-default-200/70 bg-background/85 px-3 py-3 shadow-sm">
        <div className="mb-2 text-xs font-medium text-muted-foreground">{label}</div>
        <div className="space-y-2">
          <Skeleton className={compact ? 'h-3 w-2/3 rounded-full' : 'h-3.5 w-2/3 rounded-full'} />
          <Skeleton className={compact ? 'h-3 w-full rounded-full' : 'h-3.5 w-full rounded-full'} />
          <Skeleton className={compact ? 'h-3 w-4/5 rounded-full' : 'h-3.5 w-4/5 rounded-full'} />
        </div>
      </div>
    </div>
  )
}
