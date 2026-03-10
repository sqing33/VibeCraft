import { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Button, Input, Skeleton, Switch } from '@heroui/react'
import { RefreshCw, Save } from 'lucide-react'

import {
  cancelIFLOWBrowserAuth,
  fetchCLIToolSettings,
  fetchIFLOWBrowserAuth,
  putCLIToolSettings,
  startIFLOWBrowserAuth,
  submitIFLOWBrowserAuthCode,
  type CLITool,
  type IFLOWBrowserAuthSession,
  type PutCLIToolSettingsRequest,
} from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

import { SETTINGS_INPUT_CLASSNAMES, SETTINGS_PANEL_BUTTON_CLASS, SettingsTabLayout } from './settingsUi'

type ToolDraft = CLITool & {
  local_id: string
}

function newLocalID(): string {
  return `${Date.now()}_${Math.random().toString(16).slice(2)}`
}

function toDraft(tool: CLITool): ToolDraft {
  return { ...tool, local_id: newLocalID() }
}

export function CLIToolSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [drafts, setDrafts] = useState<ToolDraft[]>([])
  const [iflowAuth, setIflowAuth] = useState<IFLOWBrowserAuthSession | null>(null)
  const [iflowAuthBusy, setIflowAuthBusy] = useState(false)
  const [iflowAuthCode, setIflowAuthCode] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const settings = await fetchCLIToolSettings(daemonUrl)
      setDrafts((settings.tools ?? []).map(toDraft))
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      setDrafts([])
    } finally {
      setLoading(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load()
  }, [load])

  const iflowTool = useMemo(
    () => drafts.find((tool) => (tool.id ?? '').trim() === 'iflow' || (tool.cli_family ?? '').trim() === 'iflow') ?? null,
    [drafts],
  )

  const onSave = async () => {
    setSaving(true)
    setError(null)
    try {
      const payload: PutCLIToolSettingsRequest = {
        tools: drafts.map((tool) => ({
          ...tool,
          command_path: tool.command_path?.trim() || undefined,
        })),
      }
      const saved = await putCLIToolSettings(daemonUrl, payload)
      setDrafts((saved.tools ?? []).map(toDraft))
      toast({ title: 'CLI 工具设置已保存' })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      setError(message)
      toast({ variant: 'destructive', title: '保存失败', description: message })
    } finally {
      setSaving(false)
    }
  }

  const refreshIFLOWAuth = useCallback(async (sessionId: string) => {
    try {
      const session = await fetchIFLOWBrowserAuth(daemonUrl, sessionId)
      setIflowAuth(session)
      if (session.authenticated || session.status === 'completed') {
        await load()
      }
    } catch {
      setIflowAuth(null)
    }
  }, [daemonUrl, load])

  const startIFLOWAuth = async () => {
    if (!iflowTool) return
    setIflowAuthBusy(true)
    try {
      const session = await startIFLOWBrowserAuth(daemonUrl, { command_path: iflowTool.command_path?.trim() || undefined })
      setIflowAuth(session)
      setIflowAuthCode('')
      toast({ title: '已启动 iFlow 官方网页登录流程' })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '启动登录失败', description: message })
    } finally {
      setIflowAuthBusy(false)
    }
  }

  const submitIFLOWCode = async () => {
    if (!iflowAuth?.session_id) return
    if (!iflowAuthCode.trim()) {
      toast({ variant: 'destructive', title: '请输入授权码' })
      return
    }
    setIflowAuthBusy(true)
    try {
      const session = await submitIFLOWBrowserAuthCode(daemonUrl, iflowAuth.session_id, {
        authorization_code: iflowAuthCode.trim(),
      })
      setIflowAuth(session)
      if (session.authenticated || session.status === 'completed') {
        await load()
      }
      toast({ title: '授权码已提交' })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '提交授权码失败', description: message })
    } finally {
      setIflowAuthBusy(false)
    }
  }

  const cancelIFLOWAuthSession = async () => {
    if (!iflowAuth?.session_id) return
    setIflowAuthBusy(true)
    try {
      const session = await cancelIFLOWBrowserAuth(daemonUrl, iflowAuth.session_id)
      setIflowAuth(session)
      toast({ title: '已取消 iFlow 登录流程' })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '取消登录失败', description: message })
    } finally {
      setIflowAuthBusy(false)
    }
  }

  useEffect(() => {
    if (!iflowAuth?.session_id) return
    if (iflowAuth.status === 'completed' || iflowAuth.status === 'failed' || iflowAuth.status === 'canceled') return
    const timer = window.setInterval(() => {
      void refreshIFLOWAuth(iflowAuth.session_id)
    }, 2000)
    return () => window.clearInterval(timer)
  }, [iflowAuth?.session_id, iflowAuth?.status, refreshIFLOWAuth])

  return (
    <SettingsTabLayout
      footer={
        <>
          <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} variant="flat" startContent={<RefreshCw className="h-4 w-4" />} onPress={() => void load()}>
            重新加载
          </Button>
          <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} color="primary" startContent={<Save className="h-4 w-4" />} isLoading={saving} onPress={() => void onSave()}>
            保存
          </Button>
        </>
      }
    >
      <section className="space-y-2">
        <div className="text-sm font-semibold">CLI 工具</div>
        <div className="text-xs text-muted-foreground">
          这里只管理工具级开关、命令路径与健康/登录动作。模型池与 API 来源请分别到“模型设置”和“API 来源”页维护。
        </div>
      </section>

      {error ? <Alert color="danger" title="加载或保存失败" description={error} /> : null}

      {loading ? (
        <div className="space-y-3">
          <Skeleton className="h-28 rounded-xl" />
          <Skeleton className="h-28 rounded-xl" />
        </div>
      ) : (
        <div className="space-y-4">
          {drafts.map((tool) => {
            const isIFLOW = (tool.id ?? '').trim() === 'iflow' || (tool.cli_family ?? '').trim() === 'iflow'
            return (
              <section key={tool.local_id} className="space-y-3 rounded-xl border bg-background/40 p-4">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <div className="text-sm font-medium">{tool.label}</div>
                    <div className="text-xs text-muted-foreground">
                      {tool.cli_family || tool.id} · {(tool.protocol_families ?? []).join(' / ') || tool.protocol_family}
                    </div>
                  </div>
                  <Switch
                    isSelected={tool.enabled}
                    onValueChange={(value) => setDrafts((prev) => prev.map((item) => item.local_id === tool.local_id ? { ...item, enabled: value } : item))}
                  >
                    启用
                  </Switch>
                </div>
                <Input
                  radius="full"
                  size="sm"
                  classNames={SETTINGS_INPUT_CLASSNAMES}
                  label="命令路径"
                  value={tool.command_path ?? ''}
                  onValueChange={(value) => setDrafts((prev) => prev.map((item) => item.local_id === tool.local_id ? { ...item, command_path: value } : item))}
                  placeholder={`默认使用系统 PATH 中的 ${tool.cli_family || tool.id}`}
                />
                {isIFLOW ? (
                  <div className="space-y-3 rounded-lg border bg-background/60 p-3">
                    <div className="text-sm font-medium">iFlow 官方网页登录</div>
                    <div className="text-xs text-muted-foreground">
                      当前状态：{tool.iflow_browser_authenticated ? '已登录' : '未登录'}
                      {tool.iflow_browser_model ? ` · 浏览器模型：${tool.iflow_browser_model}` : ''}
                    </div>
                    {iflowAuth ? (
                      <div className="space-y-2 text-xs text-muted-foreground">
                        <div>
                          最近会话：{iflowAuth.status}
                          {iflowAuth.authenticated ? ' · 已完成认证' : ''}
                        </div>
                        {iflowAuth.auth_url ? <div className="break-all">OAuth URL：{iflowAuth.auth_url}</div> : null}
                        {iflowAuth.last_output ? (
                          <div className="max-h-24 overflow-auto whitespace-pre-wrap rounded-md border bg-background/70 p-2">
                            {iflowAuth.last_output}
                          </div>
                        ) : null}
                        {iflowAuth.error ? <div className="text-danger">错误：{iflowAuth.error}</div> : null}
                      </div>
                    ) : null}
                    {iflowAuth?.can_submit_code ? (
                      <Input
                        radius="full"
                        size="sm"
                        classNames={SETTINGS_INPUT_CLASSNAMES}
                        label="授权码"
                        value={iflowAuthCode}
                        onValueChange={setIflowAuthCode}
                        placeholder="粘贴网页登录返回的 authorization code"
                      />
                    ) : null}
                    <div className="flex flex-wrap gap-2">
                      <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} color="primary" isLoading={iflowAuthBusy} onPress={() => void startIFLOWAuth()}>
                        启动网页登录
                      </Button>
                      {iflowAuth?.session_id ? (
                        <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} variant="flat" isLoading={iflowAuthBusy} onPress={() => void refreshIFLOWAuth(iflowAuth.session_id)}>
                          刷新状态
                        </Button>
                      ) : null}
                      {iflowAuth?.can_submit_code ? (
                        <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} color="secondary" isLoading={iflowAuthBusy} onPress={() => void submitIFLOWCode()}>
                          提交授权码
                        </Button>
                      ) : null}
                      {iflowAuth?.session_id && iflowAuth.status !== 'completed' && iflowAuth.status !== 'canceled' ? (
                        <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} variant="light" color="danger" isLoading={iflowAuthBusy} onPress={() => void cancelIFLOWAuthSession()}>
                          取消登录
                        </Button>
                      ) : null}
                    </div>
                  </div>
                ) : null}
              </section>
            )
          })}
        </div>
      )}

    </SettingsTabLayout>
  )
}
