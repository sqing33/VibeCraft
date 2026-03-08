import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Skeleton, Switch, Textarea } from '@heroui/react'
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
    id: `new-mcp-${seed}`,
    raw_json: `{
  "mcpServers": {
    "mcp-router": {
      "command": "npx",
      "args": ["-y", "@mcp_router/cli@latest", "connect"],
      "env": {
        "MCPR_TOKEN": "your-token"
      }
    }
  }
}`,
    default_enabled_cli_tool_ids: [],
    config: {},
  }
}

function readServerDescription(server: MCPServerSetting): string {
  const description = server.config?.['description']
  return typeof description === 'string' ? description.trim() : ''
}

function readServerTransport(server: MCPServerSetting): string {
  const command = server.config?.['command']
  if (typeof command === 'string' && command.trim()) {
    const rawArgs = Array.isArray(server.config?.['args']) ? server.config?.['args'] : []
    const args = rawArgs.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    return [command.trim(), ...args].join(' ')
  }
  const url = server.config?.['url']
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
    <div className="space-y-4">
      <Alert
        color="primary"
        title="MCP 直接使用 JSON 配置，并按会话注入到 Codex"
        description={'这里接受两种 JSON 形态：{"mcpServers": {...}} 或直接 {...}。页面只保留“默认启用”，用于决定新建会话时每个 CLI 工具默认勾选哪些 MCP。'}
      />

      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm text-muted-foreground">当前共配置 {data.servers.length} 个 MCP。</div>
        <Button variant="flat" startContent={<Plus className="h-4 w-4" />} onPress={onAddServer}>
          新增 MCP JSON
        </Button>
      </div>

      {data.servers.length === 0 ? (
        <div className="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
          还没有 MCP 配置，点击右上角“新增 MCP JSON”开始添加。
        </div>
      ) : null}

      <div className="space-y-4">
        {data.servers.map((server, index) => {
          const description = readServerDescription(server)
          const transport = readServerTransport(server)
          return (
            <section key={`${server.id || 'mcp'}-${index}`} className="space-y-4 rounded-xl border bg-card p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="truncate text-sm font-semibold">{server.id || `MCP ${index + 1}`}</div>
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
                minRows={12}
                value={server.raw_json}
                onValueChange={(value) => updateServer(index, { raw_json: value })}
                placeholder={'粘贴 {"mcpServers": {...}} 或直接 { ... }'}
              />

              <div className="rounded-xl border bg-background/40 p-3">
                <div className="text-sm font-medium">默认启用</div>
                <div className="mt-1 text-xs text-muted-foreground">
                  这里只影响“新建会话”的默认 MCP 选择。当前会话仍然可以单独勾选任意已保存的 MCP。
                </div>
                {data.tools.length === 0 ? (
                  <div className="mt-3 rounded-lg border border-dashed px-3 py-4 text-xs text-muted-foreground">
                    当前没有可用的 CLI 工具可供设置默认值。
                  </div>
                ) : (
                  <div className="mt-3 space-y-2">
                    {data.tools.map((tool) => {
                      const defaultForTool = (server.default_enabled_cli_tool_ids ?? []).includes(tool.id)
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
          )
        })}
      </div>

      <div className="flex justify-end">
        <Button color="primary" isLoading={saving} onPress={() => void onSave()}>
          保存 MCP 设置
        </Button>
      </div>
    </div>
  )
}
