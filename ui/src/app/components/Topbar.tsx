import { Moon, Sun } from 'lucide-react'
import { Button, Chip } from '@heroui/react'

import { goHome, goToChat, useHashRoute } from '@/app/routes'
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
  const route = useHashRoute()
  const health = useDaemonStore((s) => s.health)
  const wsState = useDaemonStore((s) => s.wsState)
  const theme = useThemeStore((s) => s.theme)
  const toggleTheme = useThemeStore((s) => s.toggleTheme)

  const healthBadge =
    health.status === 'ok' ? (
      <Chip color="success" variant="flat" size="sm">
        健康状态：{healthText(health)}
      </Chip>
    ) : health.status === 'error' ? (
      <Chip color="danger" variant="flat" size="sm">
        健康状态：{healthText(health)}
      </Chip>
    ) : (
      <Chip variant="flat" size="sm">
        健康状态：{healthText(health)}
      </Chip>
    )

  return (
    <header className="sticky top-0 z-40 border-b bg-background/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4">
        <div className="flex items-center gap-2">
          <div className="text-sm font-semibold tracking-tight">vibe-tree</div>
          <Chip variant="bordered" size="sm">
            前端
          </Chip>
          <Button
            variant={route.name === 'chat' ? 'flat' : 'light'}
            size="sm"
            onPress={goToChat}
          >
            Chat
          </Button>
          <Button
            variant={route.name === 'orchestrations' || route.name === 'orchestration_detail' ? 'flat' : 'light'}
            size="sm"
            onPress={goHome}
          >
            Orchestrations
          </Button>
        </div>

        <div className="flex items-center gap-2">
          {healthBadge}
          <Chip variant="flat" size="sm">
            连接：{wsText(wsState)}
          </Chip>
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
          <DevToolsDialog />
          <SettingsDialog />
        </div>
      </div>
    </header>
  )
}
