import { Moon, Sun } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { type HealthState, useDaemonStore } from '@/stores/daemonStore'
import { useThemeStore } from '@/stores/themeStore'

import { SettingsDialog } from './SettingsDialog'
import { DevToolsDialog } from './DevToolsDialog'

function healthText(health: HealthState): string {
  if (health.status === 'checking') return '检查中'
  if (health.status === 'ok') return '正常'
  return '异常'
}

function wsText(state: string): string {
  if (state === 'connected') return '已连接'
  if (state === 'connecting') return '连接中'
  return '未连接'
}

export function Topbar() {
  const health = useDaemonStore((s) => s.health)
  const wsState = useDaemonStore((s) => s.wsState)
  const theme = useThemeStore((s) => s.theme)
  const toggleTheme = useThemeStore((s) => s.toggleTheme)

  const healthBadge =
    health.status === 'ok' ? (
      <Badge className="bg-emerald-500/15 text-emerald-700 hover:bg-emerald-500/15 dark:text-emerald-200">
        健康状态：{healthText(health)}
      </Badge>
    ) : health.status === 'error' ? (
      <Badge className="bg-red-500/15 text-red-700 hover:bg-red-500/15 dark:text-red-200">
        健康状态：{healthText(health)}
      </Badge>
    ) : (
      <Badge variant="secondary">健康状态：{healthText(health)}</Badge>
    )

  return (
    <header className="sticky top-0 z-40 border-b bg-background/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        <div className="flex items-center gap-2">
          <div className="text-sm font-semibold tracking-tight">vibe-tree</div>
          <Badge variant="outline">前端</Badge>
        </div>

        <div className="flex items-center gap-2">
          {healthBadge}
          <Badge variant="secondary">连接：{wsText(wsState)}</Badge>
          <Button
            variant="ghost"
            size="icon"
            onClick={toggleTheme}
            aria-label={theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'}
            title={theme === 'dark' ? '切换为浅色主题' : '切换为深色主题'}
          >
            {theme === 'dark' ? (
              <Sun className="h-4 w-4" />
            ) : (
              <Moon className="h-4 w-4" />
            )}
          </Button>
          <DevToolsDialog />
          <SettingsDialog />
        </div>
      </div>
    </header>
  )
}
