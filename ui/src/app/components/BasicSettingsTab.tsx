import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Input, Skeleton, Select, SelectItem } from '@heroui/react'

import {
  fetchBasicSettings,
  fetchLLMSettings,
  putBasicSettings,
  type BasicSettings,
  type LLMModelProfile,
  type LLMSource,
} from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

type ThinkingTranslationDraft = {
  source_id: string
  model: string
  target_model_ids: string[]
}

function selectionToString(keys: unknown): string {
  if (keys === 'all') return ''
  if (keys instanceof Set) {
    const first = keys.values().next().value
    if (typeof first === 'string') return first
  }
  return ''
}

function normalizeTargetModelIDs(values: string[]): string[] {
  const seen = new Set<string>()
  const next: string[] = []
  for (const value of values) {
    const normalized = value.trim().toLowerCase()
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    next.push(normalized)
  }
  return next
}

function draftFromSettings(settings: BasicSettings): ThinkingTranslationDraft {
  return {
    source_id: settings.thinking_translation?.source_id ?? '',
    model: settings.thinking_translation?.model ?? '',
    target_model_ids: normalizeTargetModelIDs(settings.thinking_translation?.target_model_ids ?? []),
  }
}

export function BasicSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)

  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sources, setSources] = useState<LLMSource[]>([])
  const [models, setModels] = useState<LLMModelProfile[]>([])
  const [draft, setDraft] = useState<ThinkingTranslationDraft>({
    source_id: '',
    model: '',
    target_model_ids: [],
  })

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [basicSettings, llmSettings] = await Promise.all([
        fetchBasicSettings(daemonUrl),
        fetchLLMSettings(daemonUrl),
      ])
      setDraft(draftFromSettings(basicSettings))
      setSources(llmSettings.sources ?? [])
      setModels(llmSettings.models ?? [])
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load()
  }, [load])

  const groupedModels = useMemo(() => {
    const sourceLabelByID = new Map<string, string>()
    for (const source of sources) {
      sourceLabelByID.set(source.id, source.label?.trim() || source.id)
    }
    const groups = new Map<string, { label: string; items: LLMModelProfile[] }>()
    for (const model of models) {
      const sourceID = model.source_id?.trim() || 'unknown'
      const group = groups.get(sourceID) ?? {
        label: sourceLabelByID.get(sourceID) ?? sourceID,
        items: [],
      }
      group.items.push(model)
      groups.set(sourceID, group)
    }
    return Array.from(groups.entries())
      .sort((a, b) => a[1].label.localeCompare(b[1].label, 'zh-CN'))
      .map(([sourceID, group]) => ({
        source_id: sourceID,
        label: group.label,
        items: [...group.items].sort((a, b) =>
          (a.label || a.id).localeCompare(b.label || b.id, 'zh-CN'),
        ),
      }))
  }, [models, sources])

  const hasSources = sources.length > 0
  const hasModels = models.length > 0

  const toggleTargetModel = (modelID: string) => {
    setDraft((prev) => {
      const normalized = modelID.trim().toLowerCase()
      const next = prev.target_model_ids.includes(normalized)
        ? prev.target_model_ids.filter((item) => item !== normalized)
        : [...prev.target_model_ids, normalized]
      return {
        ...prev,
        target_model_ids: normalizeTargetModelIDs(next),
      }
    })
  }

  const onSave = async () => {
    if (!draft.source_id.trim()) {
      toast({ variant: 'destructive', title: '请先选择 API 源' })
      return
    }
    if (!draft.model.trim()) {
      toast({ variant: 'destructive', title: '请先填写翻译模型' })
      return
    }
    if (draft.target_model_ids.length === 0) {
      toast({ variant: 'destructive', title: '请至少选择一个需要翻译的 AI 模型' })
      return
    }
    setSaving(true)
    setError(null)
    try {
      const saved = await putBasicSettings(daemonUrl, {
        thinking_translation: {
          source_id: draft.source_id.trim(),
          model: draft.model.trim(),
          target_model_ids: normalizeTargetModelIDs(draft.target_model_ids),
        },
      })
      setDraft(draftFromSettings(saved))
      toast({ title: '基本设置已保存' })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      toast({ variant: 'destructive', title: '保存失败', description: message })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <section className="space-y-3 rounded-xl border bg-background/40 p-4">
        <div>
          <div className="text-sm font-semibold">思考过程翻译</div>
          <div className="text-xs text-muted-foreground">
            为指定 AI 模型的思考过程启用中文翻译展示。翻译成功时只展示中文，失败时回退为原文。
          </div>
        </div>

        {error ? <Alert color="danger" title="加载或保存失败" description={error} /> : null}

        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-12 rounded-lg" />
            <Skeleton className="h-12 rounded-lg" />
            <Skeleton className="h-32 rounded-lg" />
          </div>
        ) : (
          <>
            {!hasSources ? (
              <Alert
                color="warning"
                title="请先配置 API 源"
                description="当前还没有可用的 API 源，请先到“模型”标签页添加 Source 和模型。"
              />
            ) : null}

            <Select
              aria-label="翻译 API 源"
              label="API 源"
              placeholder={hasSources ? '请选择翻译 API 源' : '请先到模型页配置 API 源'}
              selectedKeys={draft.source_id ? new Set([draft.source_id]) : new Set([])}
              selectionMode="single"
              isDisabled={!hasSources}
              onSelectionChange={(keys) =>
                setDraft((prev) => ({
                  ...prev,
                  source_id: selectionToString(keys),
                }))
              }
            >
              {sources.map((source) => (
                <SelectItem key={source.id}>{source.label?.trim() || source.id}</SelectItem>
              ))}
            </Select>

            <Input
              label="翻译模型"
              placeholder="例如：gpt-4.1-mini-translator"
              value={draft.model}
              isDisabled={!hasSources}
              onValueChange={(value) => setDraft((prev) => ({ ...prev, model: value }))}
            />

            <div className="space-y-2">
              <div className="flex items-center justify-between gap-2">
                <div className="text-sm font-medium">需要翻译的 AI 模型</div>
                <div className="text-xs text-muted-foreground">已选择 {draft.target_model_ids.length} 个</div>
              </div>

              {!hasModels ? (
                <Alert
                  color="warning"
                  title="暂无可选模型"
                  description="请先到“模型”标签页添加至少一个模型，然后再回来选择需要翻译的 AI 模型。"
                />
              ) : (
                <div className="space-y-3 rounded-lg border bg-muted/20 p-3">
                  {groupedModels.map((group) => (
                    <div key={group.source_id} className="space-y-2">
                      <div className="text-xs font-medium text-muted-foreground">{group.label}</div>
                      <div className="flex flex-wrap gap-2">
                        {group.items.map((model) => {
                          const selected = draft.target_model_ids.includes(model.id)
                          return (
                            <Button
                              key={model.id}
                              size="sm"
                              variant={selected ? 'solid' : 'bordered'}
                              color={selected ? 'primary' : 'default'}
                              onPress={() => toggleTargetModel(model.id)}
                            >
                              {model.label || model.id}
                            </Button>
                          )
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="flex justify-end gap-2">
              <Button variant="flat" onPress={() => void load()}>
                重新加载
              </Button>
              <Button
                color="primary"
                isLoading={saving}
                isDisabled={!hasSources}
                onPress={() => void onSave()}
              >
                保存
              </Button>
            </div>
          </>
        )}
      </section>
    </div>
  )
}
