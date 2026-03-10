import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Alert, Button, Input, Select, SelectItem, Skeleton } from '@heroui/react'
import { Circle, LoaderCircle, Plus, RefreshCw, Save, Trash2 } from 'lucide-react'

import {
  fetchAPISourceSettings,
  fetchRuntimeModelSettings,
  postLLMTest,
  putRuntimeModelSettings,
  type APISource,
  type RuntimeModelProfile,
  type RuntimeModelRuntime,
  type RuntimeModelSettings,
} from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

import {
  SETTINGS_INPUT_CLASSNAMES,
  SETTINGS_PANEL_BUTTON_CLASS,
  SETTINGS_SELECT_CLASSNAMES,
  SettingsTabLayout,
  SETTINGS_TEXT_BUTTON_CLASS,
} from './settingsUi'

type RuntimeModelDraft = RuntimeModelProfile & {
  local_id: string
}

type RuntimeDraft = {
  id: string
  label: string
  kind: 'sdk' | 'cli'
  provider?: string
  cli_tool_id?: string
  default_model_id?: string
  models: RuntimeModelDraft[]
}

const RUNTIME_ORDER = ['sdk-openai', 'sdk-anthropic', 'codex', 'claude', 'iflow', 'opencode']

function newLocalID(): string {
  return `${Date.now()}_${Math.random().toString(16).slice(2)}`
}

function normalizeProvider(value: string): string {
  return value.trim().toLowerCase()
}

function normalizeModelID(value: string): string {
  return value.trim().toLowerCase()
}

function sourceOptionsForRuntime(sources: APISource[], _runtimeId: string): APISource[] {
  return [...(sources ?? [])]
}

function toRuntimeDraft(runtime: RuntimeModelRuntime): RuntimeDraft {
  return {
    id: runtime.id,
    label: runtime.label,
    kind: runtime.kind,
    provider: runtime.provider,
    cli_tool_id: runtime.cli_tool_id,
    default_model_id: runtime.default_model_id,
    models: (runtime.models ?? []).map((model) => ({ ...model, local_id: newLocalID() })),
  }
}

function sortRuntimes(runtimes: RuntimeModelRuntime[]): RuntimeModelRuntime[] {
  return [...runtimes].sort((a, b) => RUNTIME_ORDER.indexOf(a.id) - RUNTIME_ORDER.indexOf(b.id))
}

function firstAvailableModelID(models: RuntimeModelDraft[]): string {
  for (const model of models) {
    const id = normalizeModelID(model.id)
    if (id) {
      return id
    }
  }
  return ''
}

function FieldRow(props: { label: string; children: ReactNode }) {
  return (
    <div className="grid gap-2 sm:grid-cols-[88px_minmax(0,1fr)] sm:items-center">
      <div className="text-sm text-muted-foreground">{props.label}</div>
      <div className="min-w-0">{props.children}</div>
    </div>
  )
}

export function RuntimeModelSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testingId, setTestingId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [sources, setSources] = useState<APISource[]>([])
  const [drafts, setDrafts] = useState<RuntimeDraft[]>([])

  const updateRuntime = useCallback((runtimeId: string, updater: (runtime: RuntimeDraft) => RuntimeDraft) => {
    setDrafts((prev) => prev.map((runtime) => (runtime.id === runtimeId ? updater(runtime) : runtime)))
  }, [])

  const updateModel = useCallback(
    (runtimeId: string, localId: string, updater: (model: RuntimeModelDraft) => RuntimeModelDraft) => {
      updateRuntime(runtimeId, (runtime) => ({
        ...runtime,
        models: runtime.models.map((model) => (model.local_id === localId ? updater(model) : model)),
      }))
    },
    [updateRuntime],
  )

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [sourceRes, runtimeRes] = await Promise.all([
        fetchAPISourceSettings(daemonUrl),
        fetchRuntimeModelSettings(daemonUrl),
      ])
      setSources(sourceRes.sources ?? [])
      setDrafts(sortRuntimes(runtimeRes.runtimes ?? []).map(toRuntimeDraft))
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      setSources([])
      setDrafts([])
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load()
  }, [load])

  const sourceById = useMemo(() => {
    const map = new Map<string, APISource>()
    for (const source of sources) {
      map.set((source.id ?? '').trim(), source)
    }
    return map
  }, [sources])

  const onSave = async () => {
    setSaving(true)
    setError(null)
    try {
      const payload: RuntimeModelSettings = await putRuntimeModelSettings(daemonUrl, {
        runtimes: drafts.map((runtime) => ({
          id: runtime.id,
          default_model_id: normalizeModelID(runtime.default_model_id || '') || undefined,
          models: runtime.models.map((model) => ({
            id: model.id.trim(),
            label: model.label.trim(),
            provider: normalizeProvider(model.provider || '') || undefined,
            source_id: model.source_id.trim(),
          })),
        })),
      })
      setDrafts(sortRuntimes(payload.runtimes ?? []).map(toRuntimeDraft))
      toast({ title: '模型设置已保存' })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      toast({ variant: 'destructive', title: '保存失败', description: message })
    } finally {
      setSaving(false)
    }
  }

  const onTestModel = async (model: RuntimeModelDraft) => {
    if (testingId) return

    const source = sourceById.get(model.source_id.trim())
    if (!source) {
      toast({
        variant: 'destructive',
        title: '请先选择 API 来源',
        description: '当前模型还没有绑定可用来源。',
      })
      return
    }

    const provider = normalizeProvider(model.provider || '')
    if (provider !== 'openai' && provider !== 'anthropic') {
      toast({
        variant: 'destructive',
        title: '当前模型不支持测试',
        description: '只有 OpenAI / Anthropic 协议的模型支持 SDK 测试。',
      })
      return
    }
    if (!source.has_key) {
      toast({
        variant: 'destructive',
        title: '来源缺少 API Key',
        description: '请先到“API 来源”页为该来源保存 API Key，再执行测试。',
      })
      return
    }

    const effectiveModel = normalizeModelID(model.id)
    if (!effectiveModel) {
      toast({
        variant: 'destructive',
        title: '请先填写模型',
        description: '需要先提供模型 ID 才能发起测试。',
      })
      return
    }

    setTestingId(model.local_id)
    try {
      const res = await postLLMTest(daemonUrl, {
        provider,
        model: effectiveModel,
        source_id: source.id.trim(),
        base_url: source.base_url?.trim() || undefined,
        prompt: 'Reply with a single word: OK',
      })
      toast({
        title: '测试成功',
        description: `${res.output || 'OK'}（${res.latency_ms}ms）`,
      })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '测试失败', description: message })
    } finally {
      setTestingId(null)
    }
  }

  const canSave = useMemo(
    () =>
      drafts.every((runtime) =>
        runtime.models.every(
          (model) => normalizeModelID(model.id) && model.source_id.trim() && sourceById.has(model.source_id.trim()),
        ),
      ),
    [drafts, sourceById],
  )

  return (
    <SettingsTabLayout
      footer={
        <>
          <Button radius="full" size="sm" variant="flat" className={SETTINGS_PANEL_BUTTON_CLASS} startContent={<RefreshCw className="h-4 w-4" />} onPress={() => void load()}>
            重新加载
          </Button>
          <Button
            radius="full"
            size="sm"
            className={SETTINGS_PANEL_BUTTON_CLASS}
            color="primary"
            startContent={<Save className="h-4 w-4" />}
            isLoading={saving}
            isDisabled={!canSave}
            onPress={() => void onSave()}
          >
            保存
          </Button>
        </>
      }
    >
          {error ? <Alert color="danger" title="加载或保存失败" description={error} /> : null}

          {loading ? (
            <div className="space-y-3">
              <Skeleton className="h-32 rounded-xl" />
              <Skeleton className="h-32 rounded-xl" />
              <Skeleton className="h-32 rounded-xl" />
            </div>
          ) : (
            <div className="space-y-4">
              {drafts.map((runtime) => {
                const compatibleSources = sourceOptionsForRuntime(sources, runtime.id)
                return (
                  <section key={runtime.id} className="space-y-3 rounded-xl border bg-background/40 p-4">
                    <div className="flex items-center justify-between gap-2">
                      <div className="text-sm font-medium">{runtime.label}</div>
                      <Button
                        size="sm"
                        radius="full"
                        className={SETTINGS_PANEL_BUTTON_CLASS}
                        variant="flat"
                        startContent={<Plus className="h-4 w-4" />}
                        onPress={() => {
                          const nextSource = compatibleSources[0]
                          updateRuntime(runtime.id, (item) => ({
                            ...item,
                            models: [
                              ...item.models,
                              {
                                local_id: newLocalID(),
                                id: '',
                                label: '',
                                provider: normalizeProvider(item.provider || runtime.provider || ''),
                                model: '',
                                source_id: nextSource?.id ?? '',
                              },
                            ],
                          }))
                        }}
                      >
                        添加模型
                      </Button>
                    </div>

                    {runtime.models.length === 0 ? (
                      <div className="rounded-lg border border-dashed px-3 py-4 text-sm text-muted-foreground">
                        当前 runtime 还没有模型。
                      </div>
                    ) : (
                      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                        {runtime.models.map((model) => {
                          const selectedSource = sourceById.get(model.source_id.trim())
                          const selectedProvider = normalizeProvider(model.provider || runtime.provider || '')
                          const supportsProbe = selectedProvider === 'openai' || selectedProvider === 'anthropic'
                          const isDefault =
                            normalizeModelID(runtime.default_model_id || '') !== '' &&
                            normalizeModelID(runtime.default_model_id || '') === normalizeModelID(model.id)
                          const canTest = Boolean(selectedSource) && Boolean(normalizeModelID(model.id))

                          return (
                            <div key={model.local_id} className="space-y-3 rounded-lg border bg-background/60 p-3">
                              <div className="flex items-center justify-between gap-3">
                                <div className="flex items-center gap-2">
                                  {isDefault ? (
                                    <span className="text-sm font-medium text-primary">默认</span>
                                  ) : (
                                    <Button
                                      size="sm"
                                      radius="full"
                                      variant="light"
                                      color="primary"
                                      className={SETTINGS_TEXT_BUTTON_CLASS}
                                      isDisabled={!normalizeModelID(model.id)}
                                      onPress={() =>
                                        updateRuntime(runtime.id, (item) => ({
                                          ...item,
                                          default_model_id: normalizeModelID(model.id),
                                        }))
                                      }
                                    >
                                      设为默认
                                    </Button>
                                  )}
                                </div>
                                <div className="flex items-center gap-2">
                                  {supportsProbe ? (
                                    <Button
                                      size="sm"
                                      radius="full"
                                      variant="light"
                                      className={SETTINGS_TEXT_BUTTON_CLASS}
                                      startContent={
                                        testingId === model.local_id ? (
                                          <LoaderCircle className="h-4 w-4 animate-spin" />
                                        ) : (
                                          <Circle className="h-3.5 w-3.5" />
                                        )
                                      }
                                      isDisabled={!canTest || testingId !== null}
                                      onPress={() => void onTestModel(model)}
                                    >
                                      测试
                                    </Button>
                                  ) : null}
                                  <Button
                                    size="sm"
                                    radius="full"
                                    variant="light"
                                    color="danger"
                                    className={SETTINGS_TEXT_BUTTON_CLASS}
                                    startContent={<Trash2 className="h-4 w-4" />}
                                    onPress={() => {
                                      updateRuntime(runtime.id, (item) => {
                                        const remaining = item.models.filter((entry) => entry.local_id !== model.local_id)
                                        const wasDefault = normalizeModelID(item.default_model_id || '') === normalizeModelID(model.id)
                                        return {
                                          ...item,
                                          models: remaining,
                                          default_model_id: wasDefault ? firstAvailableModelID(remaining) : item.default_model_id,
                                        }
                                      })
                                    }}
                                  >
                                    删除
                                  </Button>
                                </div>
                              </div>

                              <div className="space-y-3">
                                <FieldRow label="模型">
                                  <Input
                                    radius="full"
                                    size="sm"
                                    classNames={SETTINGS_INPUT_CLASSNAMES}
                                    aria-label="模型"
                                    value={model.id}
                                    onValueChange={(value) => {
                                      updateRuntime(runtime.id, (item) => {
                                        const previousID = normalizeModelID(model.id)
                                        const nextID = normalizeModelID(value)
                                        return {
                                          ...item,
                                          default_model_id:
                                            normalizeModelID(item.default_model_id || '') === previousID
                                              ? nextID
                                              : item.default_model_id,
                                          models: item.models.map((entry) =>
                                            entry.local_id === model.local_id
                                              ? {
                                                  ...entry,
                                                  id: value,
                                                  model: value,
                                                }
                                              : entry,
                                          ),
                                        }
                                      })
                                    }}
                                    placeholder="例如 gpt-5-codex"
                                  />
                                </FieldRow>

                                <FieldRow label="显示名称">
                                  <Input
                                    radius="full"
                                    size="sm"
                                    classNames={SETTINGS_INPUT_CLASSNAMES}
                                    aria-label="显示名称"
                                    value={model.label}
                                    onValueChange={(value) =>
                                      updateModel(runtime.id, model.local_id, (entry) => ({
                                        ...entry,
                                        label: value,
                                      }))
                                    }
                                    placeholder="例如 GPT-5 Codex"
                                  />
                                </FieldRow>

                                <FieldRow label="API 来源">
                                  <Select
                                    radius="full"
                                    size="sm"
                                    classNames={SETTINGS_SELECT_CLASSNAMES}
                                    aria-label="API 来源"
                                    selectedKeys={model.source_id ? new Set([model.source_id]) : new Set([])}
                                    selectionMode="single"
                                    placeholder={compatibleSources.length > 0 ? '选择来源' : '请先到 API 来源页添加兼容来源'}
                                    onSelectionChange={(keys) => {
                                      const value = Array.from(keys)[0]?.toString() ?? ''
                                      updateModel(runtime.id, model.local_id, (entry) => ({
                                        ...entry,
                                        source_id: value,
                                      }))
                                    }}
                                  >
                                    {compatibleSources.map((source) => (
                                      <SelectItem key={source.id} textValue={source.label || source.id}>
                                        {source.label || source.id}
                                      </SelectItem>
                                    ))}
                                  </Select>
                                </FieldRow>
                              </div>
                            </div>
                          )
                        })}
                      </div>
                    )}
                  </section>
                )
              })}
            </div>
          )}
    </SettingsTabLayout>
  )
}
