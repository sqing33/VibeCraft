import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button } from '@heroui/react'
import { Github, MessageSquare, Moon, Sun, Workflow } from 'lucide-react'

import { goHome, goToChat, goToRepoLibraryRepositories } from '@/app/routes'
import { useChatStore } from '@/stores/chatStore'
import { useThemeStore } from '@/stores/themeStore'

import { SettingsDialog } from './SettingsDialog'



type WorkspaceNavKey = 'chat' | 'orchestrations' | 'repo_library'

type WorkspacePortalTargetKey =
  | 'sidebarHeader'
  | 'sidebarBody'
  | 'headerMeta'
  | 'headerTitle'
  | 'headerActions'
  | 'content'

const WorkspacePortalContext = createContext<Record<WorkspacePortalTargetKey, HTMLElement | null> | null>(null)

type WorkspaceShellProps = {
  activeNav: WorkspaceNavKey
  children?: ReactNode
}

type WorkspacePortalProps = {
  target: WorkspacePortalTargetKey
  children: ReactNode
}

export function WorkspacePortal(props: WorkspacePortalProps) {
  const { target, children } = props
  const targets = useContext(WorkspacePortalContext)
  const element = targets?.[target] ?? null
  if (!element) return null
  return createPortal(children, element)
}

/**
 * 功能：提供顶级工作区共享壳层，并为左侧中段与右侧内容区暴露 portal 挂载点。
 * 参数/返回：接收当前激活导航项与 portal 子节点，返回固定左栏与右侧边框内容框。
 * 失败场景：无可用挂载点时 portal 子节点不会渲染，但壳层仍保持可见。
 * 副作用：读取 daemon 状态与主题状态，触发导航跳转与主题切换。
 */
export function WorkspaceShell(props: WorkspaceShellProps) {
  const { activeNav, children } = props
  const activeChatSessionId = useChatStore((s) => s.activeSessionId)
  const theme = useThemeStore((s) => s.theme)
  const toggleTheme = useThemeStore((s) => s.toggleTheme)

  const [sidebarHeaderEl, setSidebarHeaderEl] = useState<HTMLElement | null>(null)
  const [sidebarBodyEl, setSidebarBodyEl] = useState<HTMLElement | null>(null)
  const [headerMetaEl, setHeaderMetaEl] = useState<HTMLElement | null>(null)
  const [headerTitleEl, setHeaderTitleEl] = useState<HTMLElement | null>(null)
  const [headerActionsEl, setHeaderActionsEl] = useState<HTMLElement | null>(null)
  const [contentEl, setContentEl] = useState<HTMLElement | null>(null)

  const targets = useMemo(
    () => ({
      sidebarHeader: sidebarHeaderEl,
      sidebarBody: sidebarBodyEl,
      headerMeta: headerMetaEl,
      headerTitle: headerTitleEl,
      headerActions: headerActionsEl,
      content: contentEl,
    }),
    [contentEl, headerActionsEl, headerMetaEl, headerTitleEl, sidebarBodyEl, sidebarHeaderEl],
  )

  return (
    <WorkspacePortalContext.Provider value={targets}>
      <div className="grid h-full min-h-0 w-full grid-cols-1 lg:grid-cols-[292px_minmax(0,1fr)]">
        <section className="flex min-h-0 flex-col overflow-hidden px-1 py-2 md:px-2">
          <div className="shrink-0">
            <div className="mb-4 flex items-center justify-between gap-2 px-1">
              <div className="text-lg font-semibold tracking-tight">vibe-tree</div>
              <div className="flex items-center gap-2">
                <Button
                  variant="light"
                  size="sm"
                  isIconOnly
                  onPress={toggleTheme}
                  aria-label={theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'}
                  title={theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'}
                >
                  {theme === 'dark' ? (
                    <Sun className="h-4 w-4" aria-hidden="true" focusable="false" />
                  ) : (
                    <Moon className="h-4 w-4" aria-hidden="true" focusable="false" />
                  )}
                </Button>
                <SettingsDialog />
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Button
                color={activeNav === 'chat' ? 'primary' : 'default'}
                variant={activeNav === 'chat' ? 'flat' : 'light'}
                size="sm"
                className="justify-start rounded-2xl"
                startContent={<MessageSquare className="h-4 w-4" />}
                onPress={() => goToChat(activeChatSessionId ?? undefined)}
              >
                对话
              </Button>
              <Button
                color={activeNav === 'orchestrations' ? 'primary' : 'default'}
                variant={activeNav === 'orchestrations' ? 'flat' : 'light'}
                size="sm"
                className="justify-start rounded-2xl"
                startContent={<Workflow className="h-4 w-4" />}
                onPress={goHome}
              >
                工作流
              </Button>
              <Button
                color={activeNav === 'repo_library' ? 'primary' : 'default'}
                variant={activeNav === 'repo_library' ? 'flat' : 'light'}
                size="sm"
                className="justify-start rounded-2xl"
                startContent={<Github className="h-4 w-4" />}
                onPress={goToRepoLibraryRepositories}
              >
                Github 知识库
              </Button>
            </div>
          </div>

          <div className="mt-5 flex min-h-0 flex-1 flex-col overflow-hidden border-t border-default-200/70 pt-4">
            <div ref={setSidebarHeaderEl} className="shrink-0" />
            <div ref={setSidebarBodyEl} className="thin-scrollbar mt-3 min-h-0 flex-1 overflow-auto pr-0" />
          </div>
        </section>

        <section className="flex min-h-0 flex-col overflow-hidden rounded-[10px] border bg-card/70 shadow-sm">
          <div className="grid shrink-0 grid-cols-[minmax(0,1fr)_minmax(0,auto)_minmax(0,1fr)] items-center gap-3 border-b bg-background/60 p-[10px]">
            <div ref={setHeaderMetaEl} className="min-w-0 justify-self-start" />
            <div ref={setHeaderTitleEl} className="min-w-0 text-center" />
            <div ref={setHeaderActionsEl} className="min-w-0 justify-self-end" />
          </div>

          <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
            <div ref={setContentEl} className="flex min-h-0 flex-1 flex-col" />
          </div>
        </section>
      </div>
      {children}
    </WorkspacePortalContext.Provider>
  )
}
