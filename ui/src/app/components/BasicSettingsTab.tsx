import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Select, SelectItem, Skeleton } from '@heroui/react'

import {
  fetchBasicSettings,
  fetchRuntimeModelSettings,
  putBasicSettings,
  type BasicSettings,
  type RuntimeModelSettings,
} from '@/lib/daemon'
import { flattenRuntimeModels } from '@/lib/runtimeModels'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

import {
  SETTINGS_PANEL_BUTTON_CLASS,
  SETTINGS_SELECT_CLASSNAMES,
  SettingsTabLayout,
  SETTINGS_TEXT_BUTTON_CLASS,
} from './settingsUi'

type ThinkingTranslationDraft = {
  model_id: string
  target_model_ids: string[]
}

function draftFromSettings(settings: BasicSettings): ThinkingTranslationDraft {
  return {
    model_id: settings.thinking_translation?.model_id?.trim() ?? '',
    target_model_ids: settings.thinking_translation?.target_model_ids ?? [],
  }
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

function selectionToString(keys: 'all' | Set<React.Key>): string {
  if (keys === 'all') return ''
  return Array.from(keys)[0]?.toString() ?? ''
}

export function BasicSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [runtimeSettings, setRuntimeSettings] = useState<RuntimeModelSettings | null>(null)
  const [draft, setDraft] = useState<ThinkingTranslationDraft>({ model_id: '', target_model_ids: [] })

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [basic, runtimeModels] = await Promise.all([
        fetchBasicSettings(daemonUrl),
        fetchRuntimeModelSettings(daemonUrl),
      ])
      setDraft(draftFromSettings(basic))
      setRuntimeSettings(runtimeModels)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      setRuntimeSettings(null)
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load()
  }, [load])

  const allModels = useMemo(() => flattenRuntimeModels(runtimeSettings), [runtimeSettings])
  const translationModels = useMemo(
    () => allModels.filter((model) => model.kind === 'sdk' && (model.provider === 'openai' || model.provider === 'anthropic')),
    [allModels],
  )
  const groupedModels = useMemo(() => {
    const groups = new Map<string, { label: string; items: typeof allModels }>()
    for (const model of allModels) {
      const key = model.runtime_id
      const existing = groups.get(key) ?? { label: model.runtime_label, items: [] as typeof allModels }
      existing.items.push(model)
      groups.set(key, existing)
    }
    return Array.from(groups.entries()).map(([id, group]) => ({
      runtime_id: id,
      label: group.label,
      items: group.items.sort((a, b) => (a.label || a.id).localeCompare(b.label || b.id, 'zh-CN')),
    }))
  }, [allModels])

  const hasRuntimeModels = groupedModels.length > 0
  const hasTranslationModels = translationModels.length > 0

  const toggleTargetModel = (modelID: string) => {
    setDraft((prev) => ({
      ...prev,
      target_model_ids: normalizeTargetModelIDs(
        prev.target_model_ids.includes(modelID)
          ? prev.target_model_ids.filter((item) => item !== modelID)
          : [...prev.target_model_ids, modelID],
      ),
    }))
  }

  const onSave = async () => {
    if (!draft.model_id.trim()) {
      toast({ variant: 'destructive', title: '请先选择翻译模型' })
      return
    }
    if (draft.target_model_ids.length === 0) {
      toast({ variant: 'destructive', title: '请至少选择一个需要翻译的模型' })
      return
    }
    setSaving(true)
    setError(null)
    try {
      const saved = await putBasicSettings(daemonUrl, {
        thinking_translation: {
          model_id: draft.model_id.trim(),
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
    <SettingsTabLayout
      footer={
        <>
          <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} variant="flat" onPress={() => void load()}>
            重新加载
          </Button>
          <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} color="primary" isLoading={saving} isDisabled={!hasTranslationModels || !hasRuntimeModels} onPress={() => void onSave()}>
            保存
          </Button>
        </>
      }
    >
      {error ? <Alert color="danger" title="加载或保存失败" description={error} /> : null}

      {loading ? (
        <div className="space-y-3 rounded-xl border bg-background/40 p-4">
          <Skeleton className="h-20 rounded-xl" />
          <Skeleton className="h-32 rounded-xl" />
        </div>
      ) : (
        <div className="space-y-4 rounded-xl border bg-background/40 p-4">
          {!hasTranslationModels ? (
            <Alert
              color="warning"
              title="请先配置 SDK 翻译模型"
              description="请先到“模型设置”页为 OpenAI SDK 或 Anthropic SDK 添加至少一个模型，然后再配置思考过程翻译。"
            />
          ) : null}
          {!hasRuntimeModels ? (
            <Alert
              color="warning"
              title="当前还没有可翻译的模型"
              description="请先到“模型设置”页配置至少一个 runtime 模型。"
            />
          ) : null}

          <div className="flex flex-col gap-4 xl:flex-row xl:items-start">
            <div className="min-w-0 flex-1 space-y-2">
              <div className="text-sm font-semibold">思考过程翻译</div>
              <div className="text-xs text-muted-foreground">
                选择一个 SDK 模型负责翻译思考过程，并指定哪些 runtime 模型需要启用翻译展示。
              </div>
            </div>

            <div className="w-full shrink-0 xl:w-[262px] xl:max-w-[262px]">
              <div className="grid grid-cols-[50px_minmax(0,1fr)] items-center gap-3 xl:grid-cols-[50px_200px]">
                <div className="text-xs font-medium text-muted-foreground">翻译模型</div>
                <Select
                  radius="full"
                  size="sm"
                  classNames={SETTINGS_SELECT_CLASSNAMES}
                  aria-label="翻译模型"
                  placeholder={hasTranslationModels ? '请选择用于翻译的 SDK 模型' : '请先到模型设置页配置 SDK 模型'}
                  selectedKeys={draft.model_id ? new Set([draft.model_id]) : new Set([])}
                  selectionMode="single"
                  isDisabled={!hasTranslationModels}
                  onSelectionChange={(keys) => setDraft((prev) => ({ ...prev, model_id: selectionToString(keys) }))}
                >
                  {translationModels.map((model) => (
                    <SelectItem key={model.id} textValue={model.label || model.id}>
                      <div className="flex flex-col">
                        <span>{model.label || model.id}</span>
                        <span className="text-xs text-muted-foreground">{model.runtime_label}</span>
                      </div>
                    </SelectItem>
                  ))}
                </Select>
              </div>
            </div>
          </div>

          <div className="space-y-3 border-t border-default-200/70 pt-4">
            <div className="flex items-center justify-between gap-2">
              <div className="text-sm font-medium">需要翻译的模型</div>
              <div className="text-xs text-muted-foreground">已选择 {draft.target_model_ids.length} 个</div>
            </div>
            {groupedModels.length === 0 ? (
              <div className="rounded-lg border border-dashed px-3 py-4 text-sm text-muted-foreground">
                当前还没有任何 runtime 模型，请先到“模型设置”页完成配置。
              </div>
            ) : (
              <div className="space-y-3">
                {groupedModels.map((group) => (
                  <div key={group.runtime_id} className="space-y-2">
                    <div className="text-xs font-medium text-muted-foreground">{group.label}</div>
                    <div className="flex flex-wrap gap-2">
                      {group.items.map((model) => {
                        const selected = draft.target_model_ids.includes(model.id)
                        return (
                          <Button
                            key={`${group.runtime_id}:${model.id}`}
                            size="sm"
                            radius="full"
                            className={SETTINGS_TEXT_BUTTON_CLASS}
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
        </div>
      )}
    </SettingsTabLayout>
  )
}
