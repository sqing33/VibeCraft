import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Chip, Skeleton } from '@heroui/react'

import { fetchSkillSettings, type SkillSettings } from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

export function SkillSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [data, setData] = useState<SkillSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const load = useCallback(async (mode: 'initial' | 'refresh' = 'initial') => {
    if (mode === 'initial') setLoading(true)
    else setRefreshing(true)
    try {
      const res = await fetchSkillSettings(daemonUrl)
      setData({ skills: res.skills ?? [] })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '加载 Skill 设置失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      if (mode === 'initial') setLoading(false)
      else setRefreshing(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load('initial')
  }, [load])

  if (loading) {
    return <Skeleton className="h-64 w-full rounded-xl" />
  }

  if (!data) {
    return <Alert color="danger" title="未能加载 Skill 设置" />
  }

  return (
    <div className="space-y-4">
      <Alert
        color="primary"
        title="Skill 来自项目目录和用户目录中的 SKILL.md"
        description="这里展示当前已发现的 Skill。运行时默认会把这些 Skill 作为可用目录注入给 Codex；如果 expert 声明了 enabled_skills，才会在运行时进一步收窄。"
      />

      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm text-muted-foreground">当前共发现 {data.skills.length} 个 Skill。</div>
        <Button variant="flat" isLoading={refreshing} onPress={() => void load('refresh')}>
          刷新列表
        </Button>
      </div>

      {data.skills.length === 0 ? (
        <div className="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
          当前没有发现任何 Skill。请检查项目目录或用户目录下是否存在 `SKILL.md`。
        </div>
      ) : (
        <div className="space-y-4">
          {data.skills.map((skill) => (
            <section key={skill.id} className="space-y-2 rounded-xl border bg-card p-4">
              <div className="flex flex-wrap items-center gap-2">
                <div className="text-sm font-semibold">{skill.id}</div>
                {skill.source ? <Chip size="sm" variant="bordered">{skill.source}</Chip> : null}
              </div>
              {skill.description ? <div className="text-sm text-muted-foreground">{skill.description}</div> : null}
              {skill.path ? <div className="break-all text-xs text-muted-foreground">{skill.path}</div> : null}
            </section>
          ))}
        </div>
      )}
    </div>
  )
}
