import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Input, Select, SelectItem, Skeleton, Switch } from '@heroui/react'

import { fetchCLIToolSettings, putCLIToolSettings, type CLIToolSettings } from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

export function CLIToolSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [data, setData] = useState<CLIToolSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fetchCLIToolSettings(daemonUrl)
      setData(res)
    } catch (err: unknown) {
      toast({ variant: 'destructive', title: '加载 CLI 工具失败', description: err instanceof Error ? err.message : String(err) })
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load()
  }, [load])

  const onSave = useCallback(async () => {
    if (!data) return
    setSaving(true)
    try {
      const res = await putCLIToolSettings(daemonUrl, data)
      setData(res)
      toast({ title: 'CLI 工具设置已保存' })
    } catch (err: unknown) {
      toast({ variant: 'destructive', title: '保存 CLI 工具失败', description: err instanceof Error ? err.message : String(err) })
    } finally {
      setSaving(false)
    }
  }, [daemonUrl, data])

  const modelsByProvider = useMemo(() => {
    const map = new Map<string, CLIToolSettings["models"]>()
    for (const model of data?.models ?? []) {
      const provider = (model.provider || '').trim()
      const list = map.get(provider) ?? []
      list.push(model)
      map.set(provider, list)
    }
    return map
  }, [data?.models])

  const updateTool = useCallback((toolId: string, patch: Record<string, unknown>) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        tools: prev.tools.map((tool) => (tool.id === toolId ? { ...tool, ...patch } : tool)),
      }
    })
  }, [])

  if (loading) {
    return <Skeleton className="h-64 w-full rounded-xl" />
  }
  if (!data) {
    return <Alert color="danger" title="未能加载 CLI 工具设置" />
  }

  return (
    <div className="space-y-4">
      <Alert color="primary" title="这里配置主执行器" description="聊天与主开发执行流优先选择 CLI 工具；模型选择从对应协议族的模型池中完成。" />
      <div className="grid gap-4 lg:grid-cols-2">
        {data.tools.map((tool) => {
          const models = modelsByProvider.get((tool.protocol_family || '').trim()) ?? []
          return (
            <section key={tool.id} className="space-y-3 rounded-xl border bg-card p-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-sm font-semibold">{tool.label}</div>
                  <div className="text-xs text-muted-foreground">协议族：{tool.protocol_family} · CLI family：{tool.cli_family}</div>
                </div>
                <Switch isSelected={tool.enabled} onValueChange={(value) => updateTool(tool.id, { enabled: value })}>启用</Switch>
              </div>
              <Input label="命令路径（可选）" value={tool.command_path ?? ''} onValueChange={(value) => updateTool(tool.id, { command_path: value })} placeholder="留空则使用 PATH 中的默认命令" />
              <Select
                label="默认模型"
                selectedKeys={tool.default_model_id ? new Set([tool.default_model_id]) : new Set()}
                onSelectionChange={(keys) => {
                  if (keys === 'all') return
                  const first = keys.values().next().value
                  updateTool(tool.id, { default_model_id: typeof first === 'string' ? first : '' })
                }}
                disallowEmptySelection={false}
              >
                {models.map((model) => (
                  <SelectItem key={model.id}>{model.label || model.id} · {model.model}</SelectItem>
                ))}
              </Select>
            </section>
          )
        })}
      </div>
      <div className="flex justify-end">
        <Button color="primary" isLoading={saving} onPress={() => void onSave()}>保存 CLI 工具</Button>
      </div>
    </div>
  )
}
