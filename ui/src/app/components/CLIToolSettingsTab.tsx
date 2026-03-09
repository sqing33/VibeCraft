import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Chip, Input, Select, SelectItem, Skeleton, Switch, Textarea } from '@heroui/react'

import {
  cancelIFLOWBrowserAuth,
  cliToolProtocolFamilies,
  fetchCLIToolSettings,
  fetchIFLOWBrowserAuth,
  putCLIToolSettings,
  startIFLOWBrowserAuth,
  submitIFLOWBrowserAuthCode,
  type CLIToolSettings,
  type IFLOWBrowserAuthSession,
  type PutCLITool,
  type PutCLIToolSettingsRequest,
} from '@/lib/daemon'
import { buildCLIToolModelProfiles, cliToolDefaultModelID } from '@/lib/cliToolModels'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

function parseModelListInput(raw: string): string[] {
  const next: string[] = []
  const seen = new Set<string>()
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim()
    if (!trimmed || seen.has(trimmed)) continue
    seen.add(trimmed)
    next.push(trimmed)
  }
  return next
}

function authStatusLabel(status?: string): string {
  switch ((status || '').trim()) {
    case 'starting':
      return '正在拉起 iFlow 登录'
    case 'awaiting_code':
      return '等待授权码'
    case 'verifying':
      return '正在验证授权码'
    case 'succeeded':
      return '网页登录完成'
    case 'failed':
      return '网页登录失败'
    case 'canceled':
      return '网页登录已取消'
    default:
      return '未启动网页登录'
  }
}

function authStatusColor(status?: string): 'default' | 'primary' | 'success' | 'danger' | 'warning' {
  switch ((status || '').trim()) {
    case 'awaiting_code':
      return 'primary'
    case 'succeeded':
      return 'success'
    case 'failed':
      return 'danger'
    case 'verifying':
      return 'warning'
    default:
      return 'default'
  }
}

function firstSelection(keys: 'all' | Set<unknown>): string {
  if (keys === 'all') return ''
  const first = keys.values().next().value
  return typeof first === 'string' ? first : ''
}

export function CLIToolSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [data, setData] = useState<CLIToolSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [iflowApiKeyDrafts, setIFLOWApiKeyDrafts] = useState<Record<string, string>>({})
  const [iflowClearKeyFlags, setIFLOWClearKeyFlags] = useState<Record<string, boolean>>({})
  const [iflowAuthSession, setIFLOWAuthSession] = useState<IFLOWBrowserAuthSession | null>(null)
  const [iflowAuthBusy, setIFLOWAuthBusy] = useState(false)
  const [iflowAuthCode, setIFLOWAuthCode] = useState('')

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

  useEffect(() => {
    if (!iflowAuthSession?.session_id) return
    const status = (iflowAuthSession.status || '').trim()
    if (!['starting', 'awaiting_code', 'verifying'].includes(status)) return
    let cancelled = false
    const timer = window.setInterval(() => {
      void fetchIFLOWBrowserAuth(daemonUrl, iflowAuthSession.session_id)
        .then((next) => {
          if (cancelled) return
          setIFLOWAuthSession(next)
          if (next.status === 'succeeded') {
            toast({ title: 'iFlow 网页登录完成' })
            void load()
          }
        })
        .catch(() => {
          if (cancelled) return
        })
    }, 1500)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [daemonUrl, iflowAuthSession?.session_id, iflowAuthSession?.status, load])

  const onSave = useCallback(async () => {
    if (!data) return
    setSaving(true)
    try {
      const req: PutCLIToolSettingsRequest = {
        tools: data.tools.map((tool) => {
          const next: PutCLITool = {
            id: tool.id,
            label: tool.label,
            protocol_family: tool.protocol_family,
            protocol_families: tool.protocol_families ?? [],
            cli_family: tool.cli_family,
            default_model_id: tool.default_model_id ?? '',
            command_path: tool.command_path ?? '',
            enabled: tool.enabled,
          }
          if (tool.id === 'iflow' || tool.cli_family === 'iflow') {
            next.iflow_auth_mode = tool.iflow_auth_mode ?? 'browser'
            next.iflow_base_url = tool.iflow_base_url ?? 'https://apis.iflow.cn/v1'
            next.iflow_models = tool.iflow_models ?? []
            next.iflow_default_model = tool.iflow_default_model ?? ''
            if (iflowClearKeyFlags[tool.id]) {
              next.iflow_api_key = ''
            } else {
              const apiKeyDraft = (iflowApiKeyDrafts[tool.id] ?? '').trim()
              if (apiKeyDraft) next.iflow_api_key = apiKeyDraft
            }
          }
          return next
        }),
      }
      const res = await putCLIToolSettings(daemonUrl, req)
      setData(res)
      setIFLOWApiKeyDrafts({})
      setIFLOWClearKeyFlags({})
      toast({ title: 'CLI 工具设置已保存' })
    } catch (err: unknown) {
      toast({ variant: 'destructive', title: '保存 CLI 工具失败', description: err instanceof Error ? err.message : String(err) })
    } finally {
      setSaving(false)
    }
  }, [daemonUrl, data, iflowApiKeyDrafts, iflowClearKeyFlags])

  const updateTool = useCallback((toolId: string, patch: Record<string, unknown>) => {
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        tools: prev.tools.map((tool) => (tool.id === toolId ? { ...tool, ...patch } : tool)),
      }
    })
  }, [])

  const updateIFLOWModelList = useCallback((toolId: string, raw: string) => {
    const models = parseModelListInput(raw)
    setData((prev) => {
      if (!prev) return prev
      return {
        ...prev,
        tools: prev.tools.map((tool) => {
          if (tool.id !== toolId) return tool
          const currentDefault = (tool.iflow_default_model || '').trim()
          const nextDefault = currentDefault && models.includes(currentDefault) ? currentDefault : models[0] ?? ''
          return { ...tool, iflow_models: models, iflow_default_model: nextDefault }
        }),
      }
    })
  }, [])

  const startBrowserLogin = useCallback(async (commandPath?: string) => {
    setIFLOWAuthBusy(true)
    try {
      const session = await startIFLOWBrowserAuth(daemonUrl, { command_path: commandPath })
      setIFLOWAuthSession(session)
      setIFLOWAuthCode('')
      toast({ title: 'iFlow 网页登录已启动' })
    } catch (err: unknown) {
      toast({ variant: 'destructive', title: '启动 iFlow 登录失败', description: err instanceof Error ? err.message : String(err) })
    } finally {
      setIFLOWAuthBusy(false)
    }
  }, [daemonUrl])

  const submitAuthCode = useCallback(async () => {
    if (!iflowAuthSession?.session_id) return
    setIFLOWAuthBusy(true)
    try {
      const next = await submitIFLOWBrowserAuthCode(daemonUrl, iflowAuthSession.session_id, {
        authorization_code: iflowAuthCode.trim(),
      })
      setIFLOWAuthSession(next)
      toast({ title: '授权码已提交' })
    } catch (err: unknown) {
      toast({ variant: 'destructive', title: '提交授权码失败', description: err instanceof Error ? err.message : String(err) })
    } finally {
      setIFLOWAuthBusy(false)
    }
  }, [daemonUrl, iflowAuthCode, iflowAuthSession?.session_id])

  const cancelBrowserLogin = useCallback(async () => {
    if (!iflowAuthSession?.session_id) return
    setIFLOWAuthBusy(true)
    try {
      const next = await cancelIFLOWBrowserAuth(daemonUrl, iflowAuthSession.session_id)
      setIFLOWAuthSession(next)
    } catch (err: unknown) {
      toast({ variant: 'destructive', title: '取消网页登录失败', description: err instanceof Error ? err.message : String(err) })
    } finally {
      setIFLOWAuthBusy(false)
    }
  }, [daemonUrl, iflowAuthSession?.session_id])

  if (loading) {
    return <Skeleton className="h-64 w-full rounded-xl" />
  }
  if (!data) {
    return <Alert color="danger" title="未能加载 CLI 工具设置" />
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-2">
        {data.tools.map((tool) => {
          const isIFLOW = tool.id === 'iflow' || tool.cli_family === 'iflow'
          const protocolFamilies = cliToolProtocolFamilies(tool)
          const models = buildCLIToolModelProfiles(tool, data.models ?? [])
          const effectiveDefaultModelId = cliToolDefaultModelID(tool, data.models ?? [])
          const iflowModelText = (tool.iflow_models ?? []).join('\n')
          return (
            <section key={tool.id} className="space-y-3 rounded-xl border bg-card p-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="text-sm font-semibold">{tool.label}</div>
                  <div className="text-xs text-muted-foreground">
                    协议族：{protocolFamilies.join(' / ') || '未配置'} · CLI family：{tool.cli_family}
                  </div>
                </div>
                <Switch isSelected={tool.enabled} onValueChange={(value) => updateTool(tool.id, { enabled: value })}>启用</Switch>
              </div>
              <Input label="命令路径（可选）" value={tool.command_path ?? ''} onValueChange={(value) => updateTool(tool.id, { command_path: value })} placeholder="留空则使用 PATH 中的默认命令" />
              {isIFLOW ? (
                <div className="space-y-3">
                  <Alert
                    color="primary"
                    title="iFlow 使用官方认证与官方模型"
                    description="这里不复用系统设置里的共享 LLM 模型池。网页登录和 API Key 都走 iFlow 官方链路。"
                  />
                  <Select
                    label="认证方式"
                    selectedKeys={tool.iflow_auth_mode ? new Set([tool.iflow_auth_mode]) : new Set(['browser'])}
                    onSelectionChange={(keys) => updateTool(tool.id, { iflow_auth_mode: firstSelection(keys) || 'browser' })}
                    disallowEmptySelection
                  >
                    <SelectItem key="browser">网页登录（官方 OAuth）</SelectItem>
                    <SelectItem key="api_key">API Key</SelectItem>
                  </Select>
                  <div className="rounded-xl border bg-muted/20 p-3 space-y-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="text-sm font-medium">网页登录状态</div>
                      <Chip size="sm" color={authStatusColor(iflowAuthSession?.status)} variant="flat">
                        {iflowAuthSession ? authStatusLabel(iflowAuthSession.status) : (tool.iflow_browser_authenticated ? '已完成网页登录' : '尚未登录')}
                      </Chip>
                      {tool.iflow_browser_authenticated ? <Chip size="sm" color="success" variant="flat">已检测到 managed HOME 登录态</Chip> : null}
                    </div>
                    {tool.iflow_browser_model ? <div className="text-xs text-muted-foreground">网页登录当前模型：{tool.iflow_browser_model}</div> : null}
                    <div className="flex flex-wrap gap-2">
                      <Button color="primary" variant="flat" isLoading={iflowAuthBusy && !iflowAuthSession} onPress={() => void startBrowserLogin(tool.command_path)}>
                        启动网页登录
                      </Button>
                      <Button
                        variant="light"
                        isDisabled={!iflowAuthSession?.auth_url}
                        onPress={() => {
                          if (!iflowAuthSession?.auth_url) return
                          window.open(iflowAuthSession.auth_url, '_blank', 'noopener,noreferrer')
                        }}
                      >
                        打开授权链接
                      </Button>
                      <Button variant="light" color="danger" isDisabled={!iflowAuthSession?.session_id} onPress={() => void cancelBrowserLogin()}>
                        取消网页登录
                      </Button>
                    </div>
                    {iflowAuthSession?.auth_url ? (
                      <Textarea
                        label="终端生成的授权链接"
                        value={iflowAuthSession.auth_url}
                        minRows={2}
                        isReadOnly
                      />
                    ) : null}
                    <div className="grid gap-3 md:grid-cols-[1fr_auto]">
                      <Input
                        label="授权码"
                        value={iflowAuthCode}
                        onValueChange={setIFLOWAuthCode}
                        placeholder="从 iFlow 授权页复制 authorization code"
                      />
                      <Button color="primary" isDisabled={!iflowAuthCode.trim() || !iflowAuthSession?.session_id} isLoading={iflowAuthBusy && !!iflowAuthSession?.session_id} onPress={() => void submitAuthCode()}>
                        提交授权码
                      </Button>
                    </div>
                    {iflowAuthSession?.last_output ? (
                      <Textarea
                        label="最近终端输出"
                        value={iflowAuthSession.last_output}
                        minRows={4}
                        isReadOnly
                      />
                    ) : null}
                    {iflowAuthSession?.error ? <Alert color="danger" title="iFlow 登录失败" description={iflowAuthSession.error} /> : null}
                  </div>
                  <Input
                    label="官方 Base URL"
                    value={tool.iflow_base_url ?? 'https://apis.iflow.cn/v1'}
                    onValueChange={(value) => updateTool(tool.id, { iflow_base_url: value })}
                    placeholder="https://apis.iflow.cn/v1"
                  />
                  <Input
                    type="password"
                    label="官方 API Key"
                    value={iflowApiKeyDrafts[tool.id] ?? ''}
                    onValueChange={(value) => {
                      setIFLOWApiKeyDrafts((prev) => ({ ...prev, [tool.id]: value }))
                      setIFLOWClearKeyFlags((prev) => ({ ...prev, [tool.id]: false }))
                    }}
                    description={tool.iflow_has_key && !iflowClearKeyFlags[tool.id] ? `已保存：${tool.iflow_masked_key || '****'}` : '留空则保留当前 Key；点击清除可移除已保存 Key'}
                    placeholder="输入 iFlow 官方 API Key"
                  />
                  <div className="flex justify-end">
                    <Button
                      variant="light"
                      color="danger"
                      isDisabled={!tool.iflow_has_key && !iflowApiKeyDrafts[tool.id]}
                      onPress={() => {
                        setIFLOWApiKeyDrafts((prev) => ({ ...prev, [tool.id]: '' }))
                        setIFLOWClearKeyFlags((prev) => ({ ...prev, [tool.id]: true }))
                      }}
                    >
                      清除已保存 Key
                    </Button>
                  </div>
                  <Textarea
                    label="iFlow 模型列表"
                    value={iflowModelText}
                    onValueChange={(value) => updateIFLOWModelList(tool.id, value)}
                    minRows={4}
                    description="每行一个模型名，聊天与 Repo Library 会直接使用这些官方 iFlow 模型。"
                    placeholder={'glm-4.7\nminimax-m2.5'}
                  />
                  <Select
                    label="默认模型"
                    selectedKeys={tool.iflow_default_model ? new Set([tool.iflow_default_model]) : new Set()}
                    onSelectionChange={(keys) => updateTool(tool.id, { iflow_default_model: firstSelection(keys) })}
                    disallowEmptySelection={false}
                    isDisabled={(tool.iflow_models ?? []).length === 0}
                  >
                    {(tool.iflow_models ?? []).map((model) => (
                      <SelectItem key={model}>{model}</SelectItem>
                    ))}
                  </Select>
                </div>
              ) : (
                <Select
                  label="默认模型"
                  selectedKeys={effectiveDefaultModelId ? new Set([effectiveDefaultModelId]) : new Set()}
                  onSelectionChange={(keys) => updateTool(tool.id, { default_model_id: firstSelection(keys) })}
                  disallowEmptySelection={false}
                >
                  {models.map((model) => (
                    <SelectItem key={model.id}>{model.label || model.id} · {model.model}</SelectItem>
                  ))}
                </Select>
              )}
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
