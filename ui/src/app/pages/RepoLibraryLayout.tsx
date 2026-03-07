import type { ReactNode } from 'react'
import { Button, Chip } from '@heroui/react'
import { LibraryBig, Search } from 'lucide-react'

import {
  goToRepoLibraryPatternSearch,
  goToRepoLibraryRepositories,
} from '@/app/routes'

type RepoLibraryLayoutProps = {
  activeNav: 'repositories' | 'search'
  title: string
  description: string
  meta?: ReactNode
  actions?: ReactNode
  children: ReactNode
}

/**
 * 功能：提供 Repo Library 页面统一页头与产品线内导航。
 * 参数/返回：接收当前激活导航、标题、副标题、可选元信息与操作区；返回包裹后的页面结构。
 * 失败场景：无显式失败；仅在路由 helper 缺失时无法完成页内跳转。
 * 副作用：点击顶部按钮时会修改 hash 路由。
 */
export function RepoLibraryLayout(props: RepoLibraryLayoutProps) {
  const { activeNav, title, description, meta, actions, children } = props

  return (
    <div className="space-y-6">
      <section className="rounded-2xl border bg-card p-5 shadow-sm">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <div className="flex items-center gap-2 text-lg font-semibold">
                <LibraryBig className="h-5 w-5" />
                {title}
              </div>
              <Chip variant="flat" color="secondary">
                Repo Library
              </Chip>
              {meta}
            </div>
            <div className="max-w-3xl text-sm text-muted-foreground">{description}</div>
            <div className="flex flex-wrap gap-2">
              <Button
                variant={activeNav === 'repositories' ? 'flat' : 'light'}
                size="sm"
                onPress={goToRepoLibraryRepositories}
              >
                仓库
              </Button>
              <Button
                variant={activeNav === 'search' ? 'flat' : 'light'}
                size="sm"
                startContent={<Search className="h-4 w-4" />}
                onPress={goToRepoLibraryPatternSearch}
              >
                模式搜索
              </Button>
            </div>
          </div>

          {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
        </div>
      </section>

      {children}
    </div>
  )
}
