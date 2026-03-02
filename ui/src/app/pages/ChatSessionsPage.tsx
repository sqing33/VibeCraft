import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Chip, Input, Select, SelectItem, Textarea } from '@heroui/react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { onWsEnvelope } from '@/lib/wsBus'
import { toast } from '@/lib/toast'
import { formatRelativeTime } from '@/lib/time'
import { useDaemonStore } from '@/stores/daemonStore'
import { useChatStore } from '@/stores/chatStore'

function shouldUseFullWidth(text: string): boolean {
  const value = text.trim()
  return (
    value.length > 160 ||
    value.includes('\n') ||
    value.includes('```') ||
    value.includes('|') ||
    value.includes('\t')
  )
}

export function ChatSessionsPage() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const health = useDaemonStore((s) => s.health)
  const experts = useDaemonStore((s) => s.experts)

  const sessions = useChatStore((s) => s.sessions)
  const activeSessionId = useChatStore((s) => s.activeSessionId)
  const messagesBySession = useChatStore((s) => s.messagesBySession)
  const streamingBySession = useChatStore((s) => s.streamingBySession)
  const thinkingBySession = useChatStore((s) => s.thinkingBySession)
  const loading = useChatStore((s) => s.loading)
  const sending = useChatStore((s) => s.sending)
  const error = useChatStore((s) => s.error)

  const setActiveSession = useChatStore((s) => s.setActiveSession)
  const appendStreamingDelta = useChatStore((s) => s.appendStreamingDelta)
  const appendThinkingDelta = useChatStore((s) => s.appendThinkingDelta)
  const setThinking = useChatStore((s) => s.setThinking)
  const clearStreaming = useChatStore((s) => s.clearStreaming)
  const clearThinking = useChatStore((s) => s.clearThinking)
  const refreshSessions = useChatStore((s) => s.refreshSessions)
  const loadMessages = useChatStore((s) => s.loadMessages)
  const createSession = useChatStore((s) => s.createSession)
  const sendTurn = useChatStore((s) => s.sendTurn)
  const compactSession = useChatStore((s) => s.compactSession)
  const forkSession = useChatStore((s) => s.forkSession)
  const archiveSession = useChatStore((s) => s.archiveSession)

  const [newTitle, setNewTitle] = useState('')
  const [newExpertId, setNewExpertId] = useState('codex')
  const [input, setInput] = useState('')
  const messageScrollRef = useRef<HTMLDivElement | null>(null)

  const activeSession = useMemo(
    () => sessions.find((s) => s.session_id === activeSessionId) ?? null,
    [sessions, activeSessionId],
  )
  const messages = activeSessionId ? messagesBySession[activeSessionId] ?? [] : []
  const streaming = activeSessionId ? streamingBySession[activeSessionId] ?? '' : ''
  const thinking = activeSessionId ? thinkingBySession[activeSessionId] ?? '' : ''
  const lastAssistantMessageId = useMemo(() => {
    for (let i = messages.length - 1; i >= 0; i -= 1) {
      if (messages[i]?.role === 'assistant') return messages[i]?.message_id ?? null
    }
    return null
  }, [messages])
  const pendingAssistant = sending || streaming.length > 0

  const refresh = useCallback(async () => {
    await refreshSessions(daemonUrl)
  }, [daemonUrl, refreshSessions])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    if (!activeSessionId) return
    void loadMessages(daemonUrl, activeSessionId)
  }, [activeSessionId, daemonUrl, loadMessages])

  useEffect(() => {
    const el = messageScrollRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [messages, streaming, thinking])

  useEffect(() => {
    return onWsEnvelope((env) => {
      if (env.type === 'chat.turn.started') {
        const payload = env.payload as { session_id?: string } | undefined
        if (!payload?.session_id) return
        clearStreaming(payload.session_id)
        clearThinking(payload.session_id)
        return
      }
      if (env.type === 'chat.turn.thinking.delta') {
        const payload = env.payload as { session_id?: string; delta?: string } | undefined
        if (!payload?.session_id || typeof payload.delta !== 'string') return
        appendThinkingDelta(payload.session_id, payload.delta)
        return
      }
      if (env.type === 'chat.turn.delta') {
        const payload = env.payload as { session_id?: string; delta?: string } | undefined
        if (!payload?.session_id || typeof payload.delta !== 'string') return
        appendStreamingDelta(payload.session_id, payload.delta)
        return
      }
      if (env.type === 'chat.turn.completed') {
        const payload = env.payload as { session_id?: string; reasoning_text?: string } | undefined
        if (!payload?.session_id) return
        if (typeof payload.reasoning_text === 'string' && payload.reasoning_text.trim()) {
          setThinking(payload.session_id, payload.reasoning_text)
        }
        clearStreaming(payload.session_id)
        void refreshSessions(daemonUrl)
        void loadMessages(daemonUrl, payload.session_id)
        return
      }
      if (env.type === 'chat.session.compacted') {
        void refreshSessions(daemonUrl)
      }
    })
  }, [
    appendStreamingDelta,
    appendThinkingDelta,
    setThinking,
    clearStreaming,
    clearThinking,
    daemonUrl,
    loadMessages,
    refreshSessions,
  ])

  const onCreate = async () => {
    try {
      const created = await createSession(daemonUrl, {
        title: newTitle.trim() || undefined,
        expert_id: newExpertId.trim() || undefined,
      })
      setNewTitle('')
      setInput('')
      toast({ title: '会话已创建', description: created.session_id })
      await loadMessages(daemonUrl, created.session_id)
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '创建会话失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  const onSend = async () => {
    if (!activeSessionId) return
    const text = input.trim()
    if (!text) return
    setInput('')
    try {
      await sendTurn(daemonUrl, activeSessionId, text)
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '发送失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  const onCompact = async () => {
    if (!activeSessionId) return
    try {
      await compactSession(daemonUrl, activeSessionId)
      toast({ title: '压缩完成', description: activeSessionId })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '压缩失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  const onFork = async () => {
    if (!activeSessionId) return
    try {
      const forked = await forkSession(daemonUrl, activeSessionId)
      setActiveSession(forked.session_id)
      toast({ title: '已分叉会话', description: forked.session_id })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '分叉失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  const onArchive = async () => {
    if (!activeSessionId) return
    try {
      await archiveSession(daemonUrl, activeSessionId)
      toast({ title: '会话已归档', description: activeSessionId })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '归档失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  if (health.status === 'error') {
    return <Alert color="danger" title="无法连接守护进程" description={health.message} />
  }

  return (
    <div className="mx-auto grid h-full min-h-0 w-full max-w-[1536px] gap-4 overflow-hidden md:grid-cols-[320px_minmax(0,1200px)]">
      <section className="flex min-h-0 flex-col gap-3 rounded-xl border bg-card p-3">
        <div className="text-sm font-semibold">会话</div>
        <Input
          label="标题"
          value={newTitle}
          onValueChange={setNewTitle}
          placeholder="新会话"
          size="sm"
        />
        <Select
          label="Expert"
          selectedKeys={new Set([newExpertId])}
          onSelectionChange={(keys) => {
            if (keys === 'all') return
            const first = keys.values().next().value
            if (typeof first === 'string') setNewExpertId(first)
          }}
          size="sm"
          disallowEmptySelection
        >
          {experts
            .filter((e) => e.provider !== 'process')
            .map((e) => (
              <SelectItem key={e.id}>{e.label || e.id}</SelectItem>
            ))}
        </Select>
        <Button color="primary" size="sm" onPress={() => void onCreate()}>
          新建会话
        </Button>

        {error ? <Alert color="danger" title="加载失败" description={error} /> : null}

        <div className="min-h-0 flex-1 space-y-2 overflow-auto pr-1">
          {loading ? (
            <div className="text-xs text-muted-foreground">加载中…</div>
          ) : sessions.length === 0 ? (
            <div className="text-xs text-muted-foreground">暂无会话</div>
          ) : (
            sessions.map((s) => (
              <button
                key={s.session_id}
                className={`w-full rounded-lg border p-2 text-left ${
                  s.session_id === activeSessionId ? 'border-primary bg-primary/5' : 'hover:bg-background/50'
                }`}
                onClick={() => setActiveSession(s.session_id)}
              >
                <div className="truncate text-sm font-medium">{s.title}</div>
                <div className="mt-1 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                  <span className="truncate">{s.provider}/{s.model}</span>
                  <span>{formatRelativeTime(s.updated_at)}</span>
                </div>
                <div className="mt-1">
                  <Chip size="sm" variant="flat">
                    {s.status}
                  </Chip>
                </div>
              </button>
            ))
          )}
        </div>
      </section>

      <section className="flex min-h-0 flex-col rounded-xl border bg-card p-3">
        <div className="mb-3 flex items-center justify-between gap-2">
          <div>
            <div className="text-sm font-semibold">对话</div>
            <div className="text-xs text-muted-foreground">
              {activeSession ? `${activeSession.title} · ${activeSession.session_id}` : '请选择或创建会话'}
            </div>
          </div>
          <div className="flex gap-2">
            <Button size="sm" variant="flat" isDisabled={!activeSessionId} onPress={() => void onCompact()}>
              压缩
            </Button>
            <Button size="sm" variant="flat" isDisabled={!activeSessionId} onPress={() => void onFork()}>
              分叉
            </Button>
            <Button size="sm" variant="flat" isDisabled={!activeSessionId} onPress={() => void onArchive()}>
              归档
            </Button>
          </div>
        </div>

        <div
          ref={messageScrollRef}
          className="mb-3 min-h-0 flex-1 space-y-2 overflow-auto rounded-lg border bg-background/30 p-3"
        >
          {messages.length === 0 && !pendingAssistant ? (
            <div className="text-xs text-muted-foreground">暂无消息</div>
          ) : null}
          {messages.map((m) => {
            const isUser = m.role === 'user'
            const isAssistant = m.role === 'assistant'
            const fullWidth = shouldUseFullWidth(m.content_text)
            const showThinkingDrawer =
              isAssistant &&
              m.message_id === lastAssistantMessageId &&
              Boolean(thinking.trim()) &&
              !pendingAssistant
            return (
              <div key={m.message_id} className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
                <div
                  className={`rounded-2xl border px-3 py-2 shadow-sm ${
                    isUser ? 'border-primary/40 bg-primary/10' : 'bg-background'
                  } ${fullWidth ? 'w-full' : 'max-w-[82%]'}`}
                >
                  <div className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                    {isUser ? '你' : 'AI'}
                  </div>
                  {isAssistant ? (
                    <div className="chat-markdown text-sm">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{m.content_text}</ReactMarkdown>
                    </div>
                  ) : (
                    <div className="whitespace-pre-wrap text-sm">{m.content_text}</div>
                  )}
                  {showThinkingDrawer ? (
                    <details className="mt-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                      <summary className="cursor-pointer select-none text-muted-foreground">
                        查看完整思考过程
                      </summary>
                      <div className="mt-2 whitespace-pre-wrap break-words text-muted-foreground">
                        {thinking}
                      </div>
                    </details>
                  ) : null}
                </div>
              </div>
            )
          })}
          {pendingAssistant ? (
            <div className="flex justify-start">
              <div
                className={`rounded-2xl border border-dashed bg-background px-3 py-2 shadow-sm ${
                  shouldUseFullWidth(streaming) ? 'w-full' : 'max-w-[82%]'
                }`}
              >
                <div className="mb-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
                  AI {streaming ? '回复中' : '思考中'}
                </div>
                {streaming ? (
                  <div className="chat-markdown text-sm">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{streaming}</ReactMarkdown>
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">正在思考…</div>
                )}
                {thinking.trim() ? (
                  <details className="mt-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                    <summary className="cursor-pointer select-none text-muted-foreground">
                      查看完整思考过程
                    </summary>
                    <div className="mt-2 whitespace-pre-wrap break-words text-muted-foreground">
                      {thinking}
                    </div>
                  </details>
                ) : null}
              </div>
            </div>
          ) : null}
        </div>

        <div className="flex gap-2">
          <Textarea
            value={input}
            onValueChange={setInput}
            placeholder="输入消息..."
            minRows={2}
            isDisabled={!activeSessionId || sending}
            className="flex-1"
          />
          <Button color="primary" isDisabled={!activeSessionId || sending || !input.trim()} onPress={() => void onSend()}>
            {sending ? '发送中…' : '发送'}
          </Button>
        </div>
      </section>
    </div>
  )
}
