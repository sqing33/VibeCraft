import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button } from '@heroui/react'
import { ChevronRight, Github, MessageSquare, Moon, Sun, Workflow } from 'lucide-react'

import { goHome, goToChat, goToRepoLibraryRepositories } from '@/app/routes'
import { cn } from '@/lib/utils'
import { AnimatedGradientText } from '@/registry/magicui/animated-gradient-text'
import { MorphingText } from '@/registry/magicui/morphing-text'
import { useChatStore } from '@/stores/chatStore'
import { useThemeStore } from '@/stores/themeStore'

import { SettingsDialog } from './SettingsDialog'



type WorkspaceNavKey = 'chat' | 'orchestrations' | 'repo_library'

const MORPHING_TITLE_TEXTS: string[] = [
  'VibeTree',
  'VibeCoding',
  'ChatGPT',
  'Gemini',
  'Claude',
  'GLM',
  'Qwen',
  'DeepSeek',
]

type SidebarNavItemProps = {
  active: boolean
  icon: ReactNode
  label: string
  onPress: () => void
}

function SidebarNavItem(props: SidebarNavItemProps) {
  const { active, icon, label, onPress } = props

  return (
    <button
      type="button"
      className={cn(
        'group relative flex w-full items-center justify-start gap-2 rounded-2xl px-3 py-2 text-left text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/60',
        active
          ? 'shadow-[inset_0_-8px_10px_#8fdfff1f] transition-shadow duration-500 ease-out hover:shadow-[inset_0_-5px_10px_#8fdfff3f]'
          : 'hover:bg-default-100/70',
      )}
      aria-current={active ? 'page' : undefined}
      onClick={onPress}
    >
      {active ? (
        <span
          className="animate-gradient pointer-events-none absolute inset-0 block h-full w-full rounded-[inherit] bg-gradient-to-r from-[#ffaa40]/50 via-[#9c40ff]/50 to-[#ffaa40]/50 bg-[length:300%_100%] p-[1px]"
          style={{
            WebkitMask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
            WebkitMaskComposite: 'destination-out',
            mask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
            maskComposite: 'subtract',
            WebkitClipPath: 'padding-box',
          }}
        />
      ) : null}
      <span className="relative flex min-w-0 flex-1 items-center gap-2">
        <span className="shrink-0 text-foreground/80 group-hover:text-foreground">{icon}</span>
        {active ? (
          <AnimatedGradientText className="min-w-0 truncate text-sm font-medium">{label}</AnimatedGradientText>
        ) : (
          <span className="min-w-0 truncate text-sm font-medium text-foreground/80 transition-colors group-hover:text-foreground">
            {label}
          </span>
        )}
      </span>
      <ChevronRight
        className={cn(
          'relative ml-1 size-4 shrink-0 stroke-neutral-500 transition-transform duration-300 ease-in-out group-hover:translate-x-0.5',
          active ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
        )}
        aria-hidden="true"
        focusable="false"
      />
    </button>
  )
}

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
            <MorphingText
              texts={MORPHING_TITLE_TEXTS}
              className="mb-4 h-9 w-full text-center text-xl leading-tight md:h-10 md:text-2xl"
            />
            <div className="flex flex-col gap-2">
              <SidebarNavItem
                active={activeNav === 'chat'}
                icon={<MessageSquare className="h-4 w-4" aria-hidden="true" focusable="false" />}
                label="对话"
                onPress={() => goToChat(activeChatSessionId ?? undefined)}
              />
              <SidebarNavItem
                active={activeNav === 'orchestrations'}
                icon={<Workflow className="h-4 w-4" aria-hidden="true" focusable="false" />}
                label="工作流"
                onPress={goHome}
              />
              <SidebarNavItem
                active={activeNav === 'repo_library'}
                icon={<Github className="h-4 w-4" aria-hidden="true" focusable="false" />}
                label="Github 知识库"
                onPress={goToRepoLibraryRepositories}
              />
            </div>
          </div>

          <div className="mt-5 flex min-h-0 flex-1 flex-col overflow-hidden border-t border-default-200/70 pt-4">
            <div ref={setSidebarHeaderEl} className="shrink-0" />
            <div ref={setSidebarBodyEl} className="thin-scrollbar mt-3 min-h-0 flex-1 overflow-auto pr-0" />
          </div>

          <div className="shrink-0 border-t border-default-200/70 pt-3 pb-2">
            <div className="flex items-center justify-center gap-2 px-1">
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
