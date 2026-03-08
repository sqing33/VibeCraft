import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Input, Skeleton, Switch } from '@heroui/react'
import { Plus, Trash2 } from 'lucide-react'

import {
  fetchMCPSettings,
  putMCPSettings,
  type MCPServerSetting,
  type MCPSettings,
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

function createEmptyServer(seed: number): MCPServerSetting {
  return {
    id: `mcp-${seed}`,
    label: '',
    enabled: true,
    enabled_cli_tool_ids: [],
    default_enabled_cli_tool_ids: [],
    config: {},
  }
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
        servers: prev.servers.map((server, serverIndex) => (
          serverIndex === index ? { ...server, ...patch } : server
        )),
      }
    })
  }, [])

  const onToggleToolEnabled = useCallback((index: number, toolId: string, selected: boolean) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        servers: prev.servers.map((server, serverIndex) => {
          if (serverIndex !== index) return server
          const enabledToolIDs = toggleStringList(server.enabled_cli_tool_ids ?? [], toolId, selected)
          const defaultToolIDs = selected
            ? normalizeStringList(server.default_enabled_cli_tool_ids ?? [])
            : normalizeStringList((server.default_enabled_cli_tool_ids ?? []).filter((id) => id !== toolId))
          return {
            ...server,
            enabled_cli_tool_ids: enabledToolIDs,
            default_enabled_cli_tool_ids: defaultToolIDs,
          }
        }),
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
          const defaultToolIDs = toggleStringList(server.default_enabled_cli_tool_ids ?? [], toolId, selected)
          const enabledToolIDs = selected
            ? toggleStringList(server.enabled_cli_tool_ids ?? [], toolId, true)
            : normalizeStringList(server.enabled_cli_tool_ids ?? [])
          return {
            ...server,
            enabled_cli_tool_ids: enabledToolIDs,
            default_enabled_cli_tool_ids: defaultToolIDs,
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
        servers: [...prev.servers, createEmptyServer(prev.servers.length + 1)],
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
          ...server,
          id: server.id.trim(),
          label: server.label?.trim() || '',
          enabled_cli_tool_ids: normalizeStringList(server.enabled_cli_tool_ids ?? []),
          default_enabled_cli_tool_ids: normalizeStringList(server.default_enabled_cli_tool_ids ?? []),
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
    <div className="space-y-4">
      <Alert
        color="primary"
        title="MCP 会按 CLI 工具注入到运行中的对话"
        description="“按工具启用”决定哪个 CLI 工具可以使用该 MCP；“默认启用”决定新建会话时会默认勾选哪些 MCP。"
      />

      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm text-muted-foreground">
          当前共配置 {data.servers.length} 个 MCP。
        </div>
        <Button variant="flat" startContent={<Plus className="h-4 w-4" />} onPress={onAddServer}>
          新增 MCP
        </Button>
      </div>

      {data.servers.length === 0 ? (
        <div className="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
          还没有 MCP 配置，点击右上角“新增 MCP”开始添加。
        </div>
      ) : null}

      <div className="space-y-4">
        {data.servers.map((server, index) => (
          <section key={`${server.id || 'mcp'}-${index}`} className="space-y-4 rounded-xl border bg-card p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="text-sm font-semibold">{server.label?.trim() || server.id || `MCP ${index + 1}`}</div>
                <div className="mt-1 text-xs text-muted-foreground">ID：{server.id || '未填写'}</div>
              </div>
              <div className="flex items-center gap-2">
                <Switch isSelected={server.enabled} onValueChange={(value) => updateServer(index, { enabled: value })}>
                  启用
                </Switch>
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
            </div>

            <div className="grid gap-3 lg:grid-cols-2">
              <Input
                label="显示名称"
                value={server.label ?? ''}
                onValueChange={(value) => updateServer(index, { label: value })}
                placeholder="例如：GitHub 文件系统"
              />
              <Input
                label="唯一 ID"
                value={server.id}
                onValueChange={(value) => updateServer(index, { id: value })}
                placeholder="例如：github-filesystem"
              />
            </div>

            <div className="rounded-xl border bg-background/40 p-3">
              <div className="text-sm font-medium">CLI 工具绑定</div>
              <div className="mt-1 text-xs text-muted-foreground">
                默认启用只影响“新建会话”的初始 MCP 选择；手动保存会话后会使用会话自己的 MCP 集。
              </div>
              {data.tools.length === 0 ? (
                <div className="mt-3 rounded-lg border border-dashed px-3 py-4 text-xs text-muted-foreground">
                  当前没有可用的 CLI 工具可供绑定。
                </div>
              ) : (
                <div className="mt-3 space-y-2">
                  {data.tools.map((tool) => {
                    const enabledForTool = (server.enabled_cli_tool_ids ?? []).includes(tool.id)
                    const defaultForTool = (server.default_enabled_cli_tool_ids ?? []).includes(tool.id)
                    return (
                      <div
                        key={tool.id}
                        className="grid gap-3 rounded-lg border bg-background/70 px-3 py-3 md:grid-cols-[minmax(0,1fr)_auto_auto] md:items-center"
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
                          onValueChange={(value) => onToggleToolEnabled(index, tool.id, value)}
                        >
                          按工具启用
                        </Switch>
                        <Switch
                          size="sm"
                          isSelected={defaultForTool}
                          onValueChange={(value) => onToggleToolDefault(index, tool.id, value)}
                        >
                          默认启用
                        </Switch>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
          </section>
        ))}
      </div>

      <div className="flex justify-end">
        <Button color="primary" isLoading={saving} onPress={() => void onSave()}>
          保存 MCP 设置
        </Button>
      </div>
    </div>
  )
}
