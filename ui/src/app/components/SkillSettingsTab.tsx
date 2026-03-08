import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Chip, Skeleton, Switch } from '@heroui/react'

import {
  fetchSkillSettings,
  putSkillSettings,
  type SkillBindingSetting,
  type SkillSettings,
} from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

function normalizeStringList(values: string[]): string[] {
  const next: string[] = []
  const seen = new Set<string>()
  for (const value of values) {
    const trimmed = value.trim()
    if (!trimmed || seen.has(trimmed)) continue
    seen.add(trimmed)
    next.push(trimmed)
  }
  return next
}

function toggleStringList(values: string[], target: string, selected: boolean): string[] {
  const normalizedTarget = target.trim()
  if (!normalizedTarget) return normalizeStringList(values)
  const next = new Set(normalizeStringList(values))
  if (selected) next.add(normalizedTarget)
  else next.delete(normalizedTarget)
  return Array.from(next)
}

function skillBindingSummary(skill: SkillBindingSetting, toolCount: number): string {
  if (!skill.enabled) return '当前已停用'
  const boundCount = normalizeStringList(skill.enabled_cli_tool_ids ?? []).length
  if (toolCount > 0 && boundCount >= toolCount) return '默认对全部 CLI 工具启用'
  if (boundCount === 0) return '当前未绑定任何 CLI 工具'
  return `已绑定 ${boundCount} 个 CLI 工具`
}

export function SkillSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [data, setData] = useState<SkillSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fetchSkillSettings(daemonUrl)
      setData({
        ...res,
        skills: res.skills ?? [],
        tools: res.tools ?? [],
      })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '加载 Skill 设置失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load()
  }, [load])

  const toolCount = useMemo(() => data?.tools.length ?? 0, [data?.tools.length])

  const updateSkill = useCallback((skillId: string, patch: Partial<SkillBindingSetting>) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        skills: prev.skills.map((skill) => (skill.id === skillId ? { ...skill, ...patch } : skill)),
      }
    })
  }, [])

  const onToggleTool = useCallback((skillId: string, toolId: string, selected: boolean) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        skills: prev.skills.map((skill) => {
          if (skill.id !== skillId) return skill
          return {
            ...skill,
            enabled_cli_tool_ids: toggleStringList(skill.enabled_cli_tool_ids ?? [], toolId, selected),
          }
        }),
      }
    })
  }, [])

  const onSave = useCallback(async () => {
    if (!data) return
    setSaving(true)
    try {
      const res = await putSkillSettings(daemonUrl, {
        skills: data.skills.map((skill) => ({
          ...skill,
          enabled_cli_tool_ids: normalizeStringList(skill.enabled_cli_tool_ids ?? []),
        })),
      })
      setData({
        ...res,
        skills: res.skills ?? [],
        tools: res.tools ?? [],
      })
      toast({ title: 'Skill 设置已保存' })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '保存 Skill 设置失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setSaving(false)
    }
  }, [daemonUrl, data])

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
        title="这里管理可发现的 Skill 与 CLI 工具绑定"
        description="发现到的 Skill 会自动列出来；你可以统一停用，也可以细化到具体 CLI 工具。未做特殊限制时，默认会对全部 CLI 工具启用。"
      />

      {data.skills.length === 0 ? (
        <div className="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
          当前没有发现任何 Skill。
        </div>
      ) : (
        <div className="space-y-4">
          {data.skills.map((skill) => {
            const summary = skillBindingSummary(skill, toolCount)
            return (
              <section key={skill.id} className="space-y-4 rounded-xl border bg-card p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="text-sm font-semibold">{skill.id}</div>
                      {skill.source ? <Chip size="sm" variant="bordered">{skill.source}</Chip> : null}
                    </div>
                    {skill.description ? (
                      <div className="mt-1 text-sm text-muted-foreground">{skill.description}</div>
                    ) : null}
                    {skill.path ? (
                      <div className="mt-2 break-all text-xs text-muted-foreground">{skill.path}</div>
                    ) : null}
                    <div className="mt-2 text-xs text-muted-foreground">{summary}</div>
                  </div>
                  <Switch isSelected={skill.enabled} onValueChange={(value) => updateSkill(skill.id, { enabled: value })}>
                    启用
                  </Switch>
                </div>

                <div className="rounded-xl border bg-background/40 p-3">
                  <div className="text-sm font-medium">CLI 工具绑定</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    如果保留为“默认对全部 CLI 工具启用”，通常不需要额外调整；只有想细分时再按工具收紧即可。
                  </div>
                  {data.tools.length === 0 ? (
                    <div className="mt-3 rounded-lg border border-dashed px-3 py-4 text-xs text-muted-foreground">
                      当前没有可用的 CLI 工具可供绑定。
                    </div>
                  ) : (
                    <div className="mt-3 space-y-2">
                      {data.tools.map((tool) => {
                        const enabledForTool = (skill.enabled_cli_tool_ids ?? []).includes(tool.id)
                        return (
                          <div
                            key={tool.id}
                            className="flex items-center justify-between gap-3 rounded-lg border bg-background/70 px-3 py-3"
                          >
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium">{tool.label}</div>
                              <div className="mt-1 text-xs text-muted-foreground">
                                {tool.id} · {tool.cli_family}
                              </div>
                            </div>
                            <Switch
                              size="sm"
                              isSelected={enabledForTool}
                              onValueChange={(value) => onToggleTool(skill.id, tool.id, value)}
                            >
                              按工具启用
                            </Switch>
                          </div>
                        )
                      })}
                    </div>
                  )}
                </div>
              </section>
            )
          })}
        </div>
      )}

      <div className="flex justify-end">
        <Button color="primary" isLoading={saving} onPress={() => void onSave()}>
          保存 Skill 设置
        </Button>
      </div>
    </div>
  )
}
