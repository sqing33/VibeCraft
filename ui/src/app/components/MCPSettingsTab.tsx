import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Skeleton, Switch, Textarea } from '@heroui/react'
import { Plus, Save, Trash2 } from 'lucide-react'

import {
  fetchMCPSettings,
  putMCPSettings,
  type MCPServerSetting,
  type MCPSettings,
} from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

const CONTEXT7_MCP_PLACEHOLDER = `{
  "mcpServers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "bearer_token_env_var": "CONTEXT7_API_KEY"
    }
  }
}`

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

function createEmptyServer(): MCPServerSetting {
  return {
    id: '',
    raw_json: '',
    default_enabled_cli_tool_ids: [],
    config: {},
  }
}

function parseServerPreview(rawJSON: string): { id: string; config?: Record<string, unknown> } | null {
  const raw = rawJSON.trim()
  if (!raw) return null
  try {
    const payload = JSON.parse(raw) as Record<string, unknown>
    const registryValue = payload.mcpServers
    const registry = isRecord(registryValue) ? registryValue : payload
    const keys = Object.keys(registry).sort()
    if (keys.length === 0) return null
    const id = keys[0]?.trim()
    if (!id) return null
    const config = registry[id]
    if (!isRecord(config)) return null
    return { id, config }
  } catch {
    return null
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function readServerDescription(config?: Record<string, unknown>): string {
  const description = config?.description
  return typeof description === 'string' ? description.trim() : ''
}

function readServerTransport(config?: Record<string, unknown>): string {
  const command = config?.command
  if (typeof command === 'string' && command.trim()) {
    const rawArgs = Array.isArray(config?.args) ? config.args : []
    const args = rawArgs.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    return [command.trim(), ...args].join(' ')
  }
  const url = config?.url
  if (typeof url === 'string' && url.trim()) {
    return url.trim()
  }
  return ''
}

export function MCPSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [data, setData] = useState<MCPSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fetchMCPSettings(daemonUrl)
      setData({
        ...res,
        servers: res.servers ?? [],
        tools: res.tools ?? [],
      })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '加载 MCP 设置失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load()
  }, [load])

  const updateServer = useCallback((index: number, patch: Partial<MCPServerSetting>) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        servers: prev.servers.map((server, serverIndex) => (serverIndex === index ? { ...server, ...patch } : server)),
      }
    })
  }, [])

  const onToggleToolDefault = useCallback((index: number, toolId: string, selected: boolean) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        servers: prev.servers.map((server, serverIndex) => {
          if (serverIndex !== index) return server
          return {
            ...server,
            default_enabled_cli_tool_ids: toggleStringList(server.default_enabled_cli_tool_ids ?? [], toolId, selected),
          }
        }),
      }
    })
  }, [])

  const onAddServer = useCallback(() => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        servers: [...prev.servers, createEmptyServer()],
      }
    })
  }, [])

  const onRemoveServer = useCallback((index: number) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        servers: prev.servers.filter((_, serverIndex) => serverIndex !== index),
      }
    })
  }, [])

  const onSave = useCallback(async () => {
    if (!data) return
    setSaving(true)
    try {
      const res = await putMCPSettings(daemonUrl, {
        servers: data.servers.map((server) => ({
          id: server.id.trim(),
          raw_json: server.raw_json,
          default_enabled_cli_tool_ids: normalizeStringList(server.default_enabled_cli_tool_ids ?? []),
          config: server.config,
        })),
      })
      setData({
        ...res,
        servers: res.servers ?? [],
        tools: res.tools ?? [],
      })
      toast({ title: 'MCP 设置已保存' })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '保存 MCP 设置失败',
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
    return <Alert color="danger" title="未能加载 MCP 设置" />
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4">
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
        <div className="text-sm text-muted-foreground">当前共配置 {data.servers.length} 个 MCP。</div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="flat" startContent={<Plus className="h-4 w-4" />} onPress={onAddServer}>
            新增 MCP JSON
          </Button>
          <Button color="primary" isLoading={saving} startContent={<Save className="h-4 w-4" />} onPress={() => void onSave()}>
            保存 MCP 设置
          </Button>
        </div>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto pr-1">
        {data.servers.length === 0 ? (
          <div className="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
            还没有 MCP 配置，点击右上角“新增 MCP JSON”开始添加。
          </div>
        ) : (
          <div className="grid gap-4 lg:grid-cols-2">
            {data.servers.map((server, index) => {
              const preview = parseServerPreview(server.raw_json)
              const previewID = preview?.id || server.id || `MCP ${index + 1}`
              const previewConfig = preview?.config ?? server.config
              const description = readServerDescription(previewConfig)
              const transport = readServerTransport(previewConfig)
              return (
                <section key={`${server.id || 'mcp'}-${index}`} className="space-y-4 rounded-xl border bg-card p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-semibold">{previewID}</div>
                      {description ? <div className="mt-1 text-sm text-muted-foreground">{description}</div> : null}
                      {transport ? <div className="mt-1 break-all text-xs text-muted-foreground">{transport}</div> : null}
                    </div>
                    <Button
                      isIconOnly
                      size="sm"
                      variant="light"
                      color="danger"
                      onPress={() => onRemoveServer(index)}
                      aria-label="删除 MCP"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>

                  <Textarea
                    label="MCP JSON"
                    minRows={10}
                    value={server.raw_json}
                    onValueChange={(value) => updateServer(index, { raw_json: value })}
                    placeholder={CONTEXT7_MCP_PLACEHOLDER}
                  />

                  <div className="rounded-xl border bg-background/40 p-3">
                    {data.tools.length === 0 ? (
                      <div className="rounded-lg border border-dashed px-3 py-4 text-xs text-muted-foreground">
                        当前没有可用的 CLI 工具可供设置默认值。
                      </div>
                    ) : (
                      <div className="grid gap-2 sm:grid-cols-2">
                        {data.tools.map((tool) => {
                          const defaultForTool = (server.default_enabled_cli_tool_ids ?? []).includes(tool.id)
                          return (
                            <div
                              key={tool.id}
                              className="flex items-center justify-between gap-3 rounded-lg border bg-background/70 px-3 py-2"
                            >
                              <div className="min-w-0">
                                <div className="truncate text-sm font-medium">{tool.label}</div>
                                <div className="text-[11px] text-muted-foreground">{tool.id}</div>
                              </div>
                              <Switch
                                size="sm"
                                isSelected={defaultForTool}
                                onValueChange={(value) => onToggleToolDefault(index, tool.id, value)}
                              />
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
      </div>
    </div>
  )
}
