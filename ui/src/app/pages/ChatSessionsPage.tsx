import { type DragEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Chip, Input, Select, SelectItem, Textarea } from '@heroui/react'
import { Eye, Paperclip, Trash2, X } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { onWsEnvelope } from '@/lib/wsBus'
import { chatAttachmentContentUrl, fetchLLMSettings, type ChatAttachment } from '@/lib/daemon'
import { AttachmentPreviewModal, type AttachmentPreviewState } from '@/app/components/AttachmentPreviewModal'
import { canPreviewAttachmentTarget, describeAttachmentPreview } from '@/lib/chatAttachmentPreview'
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

function formatTokenUsage(opts: {
  tokenIn?: number
  tokenOut?: number
  cachedInputTokens?: number
}): string {
  const parts: string[] = []
  if (typeof opts.tokenIn === 'number') parts.push(`输入 ${opts.tokenIn}`)
  if (typeof opts.tokenOut === 'number') parts.push(`输出 ${opts.tokenOut}`)
  if (typeof opts.cachedInputTokens === 'number') parts.push(`缓存 ${opts.cachedInputTokens}`)
  return parts.join(' · ')
}

function formatAttachmentSize(sizeBytes?: number): string {
  if (typeof sizeBytes !== 'number' || sizeBytes <= 0) return ''
  if (sizeBytes < 1024) return `${sizeBytes} B`
  if (sizeBytes < 1024 * 1024) return `${(sizeBytes / 1024).toFixed(1)} KB`
  return `${(sizeBytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatAttachmentKind(kind?: string): string {
  switch ((kind ?? '').trim()) {
    case 'image':
      return '图片'
    case 'pdf':
      return 'PDF'
    case 'text':
      return '文本'
    default:
      return '附件'
  }
}

function guessPendingFileKind(file: File): string {
  const type = file.type.toLowerCase()
  const name = file.name.toLowerCase()
  if (type.startsWith('image/')) return '图片'
  if (type === 'application/pdf' || name.endsWith('.pdf')) return 'PDF'
  return '文本'
}

function fileIdentity(file: File): string {
  return `${file.name}:${file.size}:${file.lastModified}`
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
  const translatedThinkingBySession = useChatStore((s) => s.translatedThinkingBySession)
  const thinkingTranslationStateBySession = useChatStore((s) => s.thinkingTranslationStateBySession)
  const turnMetaBySession = useChatStore((s) => s.turnMetaBySession)
  const turnInputByUserMessageId = useChatStore((s) => s.turnInputByUserMessageId)
  const usageByMessageId = useChatStore((s) => s.usageByMessageId)
  const loading = useChatStore((s) => s.loading)
  const sending = useChatStore((s) => s.sending)
  const error = useChatStore((s) => s.error)

  const setActiveSession = useChatStore((s) => s.setActiveSession)
  const appendStreamingDelta = useChatStore((s) => s.appendStreamingDelta)
  const appendThinkingDelta = useChatStore((s) => s.appendThinkingDelta)
  const appendTranslatedThinkingDelta = useChatStore((s) => s.appendTranslatedThinkingDelta)
  const setThinking = useChatStore((s) => s.setThinking)
  const setTranslatedThinking = useChatStore((s) => s.setTranslatedThinking)
  const clearStreaming = useChatStore((s) => s.clearStreaming)
  const clearThinking = useChatStore((s) => s.clearThinking)
  const resetThinkingTranslation = useChatStore((s) => s.resetThinkingTranslation)
  const setThinkingTranslationState = useChatStore((s) => s.setThinkingTranslationState)
  const setTurnMeta = useChatStore((s) => s.setTurnMeta)
  const setTurnInputMeta = useChatStore((s) => s.setTurnInputMeta)
  const setUsageMeta = useChatStore((s) => s.setUsageMeta)
  const refreshSessions = useChatStore((s) => s.refreshSessions)
  const loadMessages = useChatStore((s) => s.loadMessages)
  const createSession = useChatStore((s) => s.createSession)
  const sendTurn = useChatStore((s) => s.sendTurn)
  const forkSession = useChatStore((s) => s.forkSession)
  const archiveSession = useChatStore((s) => s.archiveSession)

  const [newTitle, setNewTitle] = useState('')
  const [newExpertId, setNewExpertId] = useState('')
  const [input, setInput] = useState('')
  const [turnExpertId, setTurnExpertId] = useState('')
  const [selectedFiles, setSelectedFiles] = useState<File[]>([])
  const [dragActive, setDragActive] = useState(false)
  const [preview, setPreview] = useState<AttachmentPreviewState | null>(null)
  const [allowedModelExpertIds, setAllowedModelExpertIds] = useState<Set<string>>(new Set())
  const messageScrollRef = useRef<HTMLDivElement | null>(null)
  const shouldAutoScrollRef = useRef(true)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const dragDepthRef = useRef(0)

  const activeSession = useMemo(
    () => sessions.find((s) => s.session_id === activeSessionId) ?? null,
    [sessions, activeSessionId],
  )
  const selectableExperts = useMemo(
    () =>
      experts.filter((e) => {
        if (e.provider === 'process') return false
        if (e.helper_only) return false
        if (e.runtime_kind === 'cli') return true
        if (e.provider === 'demo') return true
        return allowedModelExpertIds.has(e.id)
      }),
    [allowedModelExpertIds, experts],
  )
  const expertsById = useMemo(() => {
    const map = new Map<string, (typeof selectableExperts)[number]>()
    for (const e of selectableExperts) map.set(e.id, e)
    return map
  }, [selectableExperts])
  const defaultSelectableExpertId = selectableExperts[0]?.id ?? ''
  const effectiveNewExpertId =
    newExpertId && selectableExperts.some((e) => e.id === newExpertId)
      ? newExpertId
      : defaultSelectableExpertId
  const effectiveTurnExpertId =
    turnExpertId && selectableExperts.some((e) => e.id === turnExpertId)
      ? turnExpertId
      : activeSession?.expert_id && selectableExperts.some((e) => e.id === activeSession.expert_id)
        ? activeSession.expert_id
        : defaultSelectableExpertId

  const formatModelIdentity = useCallback(
    (meta?: { expert_id?: string; provider?: string; model?: string } | null): string => {
      if (!meta) return ''
      const expertId = meta.expert_id?.trim() || ''
      const expert = expertId ? expertsById.get(expertId) : undefined
      const label = expert ? expert.label || expert.id : expertId
      const provider = (meta.provider?.trim() || expert?.provider || '').trim()
      const model = (meta.model?.trim() || expert?.model || '').trim()
      const parts: string[] = []
      if (label) parts.push(label)
      if (provider && model) parts.push(`${provider}/${model}`)
      return parts.join(' · ')
    },
    [expertsById],
  )

  const appendSelectedFiles = useCallback((files: FileList | null) => {
    if (!files || files.length === 0) return
    setSelectedFiles((prev) => {
      const seen = new Set(prev.map((file) => fileIdentity(file)))
      const next = [...prev]
      for (const file of Array.from(files)) {
        const identity = fileIdentity(file)
        if (seen.has(identity)) continue
        seen.add(identity)
        next.push(file)
      }
      return next
    })
  }, [])

  const removeSelectedFile = useCallback((targetIdentity: string) => {
    setSelectedFiles((prev) => prev.filter((file) => fileIdentity(file) !== targetIdentity))
  }, [])

  const openFilePicker = useCallback(() => {
    fileInputRef.current?.click()
  }, [])

  const closePreview = useCallback(() => {
    setPreview((prev) => {
      if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url)
      return null
    })
  }, [])

  const openPreviewForFile = useCallback(async (file: File) => {
    const descriptor = describeAttachmentPreview(file.name, file.type, undefined)
    if (descriptor.kind === 'unsupported') return
    setPreview((prev) => {
      if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url)
      return {
        name: file.name,
        kind: descriptor.kind,
        language: descriptor.language,
        loading: descriptor.kind === 'code' || descriptor.kind === 'markdown' || descriptor.kind === 'text',
        revokeOnClose: false,
      }
    })
    if (descriptor.kind === 'image' || descriptor.kind === 'pdf') {
      const url = URL.createObjectURL(file)
      setPreview((prev) => {
        if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url)
        return {
          name: file.name,
          kind: descriptor.kind,
          url,
          revokeOnClose: true,
        }
      })
      return
    }
    try {
      const content = await file.text()
      setPreview({
        name: file.name,
        kind: descriptor.kind,
        language: descriptor.language,
        content,
        revokeOnClose: false,
      })
    } catch (err) {
      setPreview({
        name: file.name,
        kind: descriptor.kind,
        error: err instanceof Error ? err.message : String(err),
        revokeOnClose: false,
      })
      toast({
        variant: 'destructive',
        title: '附件预览失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }, [])

  const openPreviewForAttachment = useCallback(async (attachment: ChatAttachment) => {
    if (!activeSessionId) return
    const descriptor = describeAttachmentPreview(attachment.file_name, attachment.mime_type, attachment.kind)
    if (descriptor.kind === 'unsupported') return
    const contentUrl = chatAttachmentContentUrl(daemonUrl, activeSessionId, attachment.attachment_id)
    setPreview((prev) => {
      if (prev?.revokeOnClose && prev.url) URL.revokeObjectURL(prev.url)
      return {
        name: attachment.file_name,
        kind: descriptor.kind,
        language: descriptor.language,
        url: descriptor.kind === 'image' || descriptor.kind === 'pdf' ? contentUrl : undefined,
        loading: descriptor.kind === 'code' || descriptor.kind === 'markdown' || descriptor.kind === 'text',
        revokeOnClose: false,
      }
    })
    if (descriptor.kind === 'image' || descriptor.kind === 'pdf') {
      return
    }
    try {
      const res = await fetch(contentUrl)
      if (!res.ok) throw new Error(`HTTP ${res.status} ${res.statusText}`.trim())
      const content = await res.text()
      setPreview({
        name: attachment.file_name,
        kind: descriptor.kind,
        language: descriptor.language,
        content,
        revokeOnClose: false,
      })
    } catch (err) {
      setPreview({
        name: attachment.file_name,
        kind: descriptor.kind,
        error: err instanceof Error ? err.message : String(err),
        revokeOnClose: false,
      })
      toast({
        variant: 'destructive',
        title: '附件预览失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }, [activeSessionId, daemonUrl])

  useEffect(() => () => {
    if (preview?.revokeOnClose && preview.url) URL.revokeObjectURL(preview.url)
  }, [preview])

  const dragHasFiles = useCallback((event: DragEvent<HTMLDivElement>) => {
    return Array.from(event.dataTransfer?.types ?? []).includes('Files')
  }, [])

  const handleComposerDragEnter = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!dragHasFiles(event)) return
    event.preventDefault()
    event.stopPropagation()
    dragDepthRef.current += 1
    setDragActive(true)
  }, [dragHasFiles])

  const handleComposerDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!dragHasFiles(event)) return
    event.preventDefault()
    event.stopPropagation()
    event.dataTransfer.dropEffect = 'copy'
    if (!dragActive) setDragActive(true)
  }, [dragActive, dragHasFiles])

  const handleComposerDragLeave = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!dragHasFiles(event)) return
    event.preventDefault()
    event.stopPropagation()
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1)
    if (dragDepthRef.current === 0) setDragActive(false)
  }, [dragHasFiles])

  const handleComposerDrop = useCallback((event: DragEvent<HTMLDivElement>) => {
    if (!dragHasFiles(event)) return
    event.preventDefault()
    event.stopPropagation()
    dragDepthRef.current = 0
    setDragActive(false)
    appendSelectedFiles(event.dataTransfer.files)
  }, [appendSelectedFiles, dragHasFiles])

  useEffect(() => {
    let cancelled = false
    void fetchLLMSettings(daemonUrl)
      .then((settings) => {
        if (cancelled) return
        const ids = new Set(
          (settings.models ?? [])
            .map((m) => (m.id ?? '').trim())
            .filter((id) => id.length > 0),
        )
        setAllowedModelExpertIds(ids)
      })
      .catch(() => {
        if (cancelled) return
        setAllowedModelExpertIds(new Set())
      })
    return () => {
      cancelled = true
    }
  }, [daemonUrl, experts])

  const messages = useMemo(
    () => (activeSessionId ? messagesBySession[activeSessionId] ?? [] : []),
    [activeSessionId, messagesBySession],
  )
  const streaming = activeSessionId ? streamingBySession[activeSessionId] ?? '' : ''
  const thinking = activeSessionId ? thinkingBySession[activeSessionId] ?? '' : ''
  const translatedThinking = activeSessionId ? translatedThinkingBySession[activeSessionId] ?? '' : ''
  const thinkingTranslationState = activeSessionId
    ? thinkingTranslationStateBySession[activeSessionId] ?? { applied: false, failed: false }
    : { applied: false, failed: false }
  const displayedThinking =
    thinkingTranslationState.applied && !thinkingTranslationState.failed ? translatedThinking : thinking
  const pendingThinkingTranslation =
    thinkingTranslationState.applied &&
    !thinkingTranslationState.failed &&
    !displayedThinking.trim() &&
    Boolean(thinking.trim())
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
    if (!shouldAutoScrollRef.current) return
    el.scrollTop = el.scrollHeight
  }, [messages, streaming, thinking, translatedThinking, thinkingTranslationState.failed])

  useEffect(() => {
    const el = messageScrollRef.current
    if (!el) return
    const onScroll = () => {
      const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight
      shouldAutoScrollRef.current = distanceToBottom < 64
    }
    onScroll()
    el.addEventListener('scroll', onScroll, { passive: true })
    return () => {
      el.removeEventListener('scroll', onScroll)
    }
  }, [activeSessionId])

  useEffect(() => {
    const el = messageScrollRef.current
    if (!el) return
    shouldAutoScrollRef.current = true
    el.scrollTop = el.scrollHeight
  }, [activeSessionId])

  useEffect(() => {
    return onWsEnvelope((env) => {
      if (env.type === 'chat.turn.started') {
        const payload = env.payload as
          | {
              session_id?: string
              expert_id?: string
              provider?: string
              model?: string
            }
          | undefined
        if (!payload?.session_id) return
        clearStreaming(payload.session_id)
        clearThinking(payload.session_id)
        resetThinkingTranslation(payload.session_id)
        setTurnMeta(payload.session_id, {
          expert_id: payload.expert_id,
          provider: payload.provider,
          model: payload.model,
        })
        return
      }
      if (env.type === 'chat.turn.thinking.delta') {
        const payload = env.payload as { session_id?: string; delta?: string } | undefined
        if (!payload?.session_id || typeof payload.delta !== 'string') return
        appendThinkingDelta(payload.session_id, payload.delta)
        return
      }
      if (env.type === 'chat.turn.thinking.translation.delta') {
        const payload = env.payload as { session_id?: string; delta?: string } | undefined
        if (!payload?.session_id || typeof payload.delta !== 'string') return
        setThinkingTranslationState(payload.session_id, { applied: true, failed: false })
        appendTranslatedThinkingDelta(payload.session_id, payload.delta)
        return
      }
      if (env.type === 'chat.turn.thinking.translation.failed') {
        const payload = env.payload as { session_id?: string } | undefined
        if (!payload?.session_id) return
        setThinkingTranslationState(payload.session_id, { applied: true, failed: true })
        return
      }
      if (env.type === 'chat.turn.delta') {
        const payload = env.payload as { session_id?: string; delta?: string } | undefined
        if (!payload?.session_id || typeof payload.delta !== 'string') return
        appendStreamingDelta(payload.session_id, payload.delta)
        return
      }
      if (env.type === 'chat.turn.completed') {
        const payload = env.payload as
          | {
              session_id?: string
              user_message_id?: string
              message?: { message_id?: string; token_in?: number; token_out?: number }
              reasoning_text?: string
              translated_reasoning_text?: string
              thinking_translation_applied?: boolean
              thinking_translation_failed?: boolean
              model_input?: string
              context_mode?: string
              token_in?: number
              token_out?: number
              cached_input_tokens?: number
            }
          | undefined
        if (!payload?.session_id) return
        if (typeof payload.reasoning_text === 'string' && payload.reasoning_text.trim()) {
          setThinking(payload.session_id, payload.reasoning_text)
        }
        if (payload.thinking_translation_applied === true) {
          setThinkingTranslationState(payload.session_id, {
            applied: true,
            failed: payload.thinking_translation_failed === true,
          })
          if (payload.thinking_translation_failed !== true) {
            setTranslatedThinking(payload.session_id, payload.translated_reasoning_text ?? '')
          }
        } else {
          resetThinkingTranslation(payload.session_id)
        }
        if (
          typeof payload.user_message_id === 'string' &&
          payload.user_message_id &&
          typeof payload.model_input === 'string' &&
          payload.model_input.trim()
        ) {
          setTurnInputMeta(payload.user_message_id, {
            model_input: payload.model_input,
            context_mode: payload.context_mode,
          })
        }
        const assistantMessageId =
          typeof payload.message?.message_id === 'string' ? payload.message.message_id : undefined
        if (assistantMessageId) {
          const tokenIn =
            typeof payload.message?.token_in === 'number'
              ? payload.message.token_in
              : typeof payload.token_in === 'number'
                ? payload.token_in
                : undefined
          const tokenOut =
            typeof payload.message?.token_out === 'number'
              ? payload.message.token_out
              : typeof payload.token_out === 'number'
                ? payload.token_out
                : undefined
          const cachedInputTokens =
            typeof payload.cached_input_tokens === 'number' ? payload.cached_input_tokens : undefined
          setUsageMeta(assistantMessageId, {
            token_in: tokenIn,
            token_out: tokenOut,
            cached_input_tokens: cachedInputTokens,
          })
        }
        clearStreaming(payload.session_id)
        setTurnMeta(payload.session_id, null)
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
    appendTranslatedThinkingDelta,
    setThinking,
    setTranslatedThinking,
    clearStreaming,
    clearThinking,
    resetThinkingTranslation,
    setThinkingTranslationState,
    setTurnMeta,
    setTurnInputMeta,
    setUsageMeta,
    daemonUrl,
    loadMessages,
    refreshSessions,
  ])

  const onCreate = async () => {
    try {
      const created = await createSession(daemonUrl, {
        title: newTitle.trim() || undefined,
        expert_id: effectiveNewExpertId.trim() || undefined,
      })
      setNewTitle('')
      setInput('')
      setSelectedFiles([])
      setTurnExpertId(created.expert_id)
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
    if (!text && selectedFiles.length === 0) return
    const draftInput = input
    const draftFiles = selectedFiles
    setInput('')
    setSelectedFiles([])
    try {
      await sendTurn(daemonUrl, activeSessionId, text, effectiveTurnExpertId.trim() || undefined, draftFiles)
    } catch (err: unknown) {
      setInput(draftInput)
      setSelectedFiles(draftFiles)
      toast({
        variant: 'destructive',
        title: '发送失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  const onFork = async () => {
    if (!activeSessionId) return
    try {
      const forked = await forkSession(daemonUrl, activeSessionId)
      setActiveSession(forked.session_id)
      setTurnExpertId(forked.expert_id)
      setSelectedFiles([])
      toast({ title: '已分叉会话', description: forked.session_id })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '分叉失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  const onDeleteSession = async (sessionId: string) => {
    const session = sessions.find((s) => s.session_id === sessionId)
    const label = session?.title?.trim() || sessionId
    if (!window.confirm(`确认删除会话「${label}」吗？\n\n删除按钮当前行为为归档（本地保留，不再显示在活跃列表）。`)) {
      return
    }
    try {
      await archiveSession(daemonUrl, sessionId)
      if (activeSessionId === sessionId) {
        setActiveSession(null)
        setTurnExpertId('')
        setSelectedFiles([])
      }
      toast({ title: '会话已删除（归档）', description: sessionId })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '删除失败',
        description: err instanceof Error ? err.message : String(err),
      })
    }
  }

  const visibleSessions = useMemo(
    () => sessions.filter((s) => s.status === 'active'),
    [sessions],
  )

  const pendingMeta = activeSessionId ? turnMetaBySession[activeSessionId] ?? null : null
  const pendingIdentity = useMemo(() => {
    if (pendingMeta) {
      const id = formatModelIdentity(pendingMeta)
      if (id) return id
    }
    if (effectiveTurnExpertId.trim()) {
      const id = formatModelIdentity({ expert_id: effectiveTurnExpertId.trim() })
      if (id) return id
    }
    if (activeSession) {
      const id = formatModelIdentity({
        expert_id: activeSession.expert_id,
        provider: activeSession.provider,
        model: activeSession.model,
      })
      if (id) return id
    }
    return ''
  }, [activeSession, effectiveTurnExpertId, formatModelIdentity, pendingMeta])

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
          selectedKeys={effectiveNewExpertId ? new Set([effectiveNewExpertId]) : new Set()}
          onSelectionChange={(keys) => {
            if (keys === 'all') return
            const first = keys.values().next().value
            if (typeof first === 'string') setNewExpertId(first)
          }}
          size="sm"
          disallowEmptySelection
          isDisabled={selectableExperts.length === 0}
        >
          {selectableExperts.map((e) => (
            <SelectItem key={e.id}>{e.label || e.id}</SelectItem>
          ))}
        </Select>
        <Button
          color="primary"
          size="sm"
          isDisabled={selectableExperts.length === 0}
          onPress={() => void onCreate()}
        >
          新建会话
        </Button>

        {error ? <Alert color="danger" title="加载失败" description={error} /> : null}

        <div className="min-h-0 flex-1 space-y-2 overflow-auto pr-1">
          {loading ? (
            <div className="text-xs text-muted-foreground">加载中…</div>
          ) : visibleSessions.length === 0 ? (
            <div className="text-xs text-muted-foreground">暂无会话</div>
          ) : (
            visibleSessions.map((s) => (
              <button
                key={s.session_id}
                className={`w-full rounded-lg border p-2 text-left ${
                  s.session_id === activeSessionId ? 'border-primary bg-primary/5' : 'hover:bg-background/50'
                }`}
                onClick={() => {
                  setActiveSession(s.session_id)
                  setTurnExpertId(s.expert_id)
                  setSelectedFiles([])
                }}
              >
                <div className="flex items-center justify-between gap-2">
                  <div className="truncate text-sm font-medium">{s.title}</div>
                  <span
                    className="shrink-0 cursor-pointer text-red-500 transition-colors hover:text-red-600"
                    title="删除会话"
                    onClick={(event) => {
                      event.stopPropagation()
                      void onDeleteSession(s.session_id)
                    }}
                  >
                    <Trash2 className="h-3.5 w-3.5" aria-hidden="true" focusable="false" />
                  </span>
                </div>
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
            <Button size="sm" variant="flat" isDisabled={!activeSessionId} onPress={() => void onFork()}>
              分叉
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
            const identity = formatModelIdentity({
              expert_id: m.expert_id,
              provider: m.provider,
              model: m.model,
            })
            const inputMeta = isUser ? turnInputByUserMessageId[m.message_id] : undefined
            const tokenUsage = isAssistant
              ? formatTokenUsage({
                  tokenIn: m.token_in ?? usageByMessageId[m.message_id]?.token_in,
                  tokenOut: m.token_out ?? usageByMessageId[m.message_id]?.token_out,
                  cachedInputTokens: usageByMessageId[m.message_id]?.cached_input_tokens,
                })
              : ''
            const contextModeLabel =
              inputMeta?.context_mode === 'anchor'
                ? '上下文模式：Anchor 续写'
                : inputMeta?.context_mode === 'reconstructed'
                  ? '上下文模式：重建上下文'
                  : inputMeta?.context_mode === 'demo'
                    ? '上下文模式：Demo'
                    : ''
            const showThinkingDrawer =
              isAssistant &&
              m.message_id === lastAssistantMessageId &&
              Boolean(displayedThinking.trim()) &&
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
                    {identity ? ` · ${identity}` : ''}
                  </div>
                  {showThinkingDrawer ? (
                    <details className="mb-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                      <summary className="cursor-pointer select-none text-muted-foreground">
                        查看完整思考过程
                      </summary>
                      <div className="chat-markdown mt-2 text-xs text-muted-foreground">
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>{displayedThinking}</ReactMarkdown>
                      </div>
                    </details>
                  ) : null}
                  {isAssistant ? (
                    <div className="chat-markdown text-sm">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{m.content_text}</ReactMarkdown>
                    </div>
                  ) : (
                    <div className="whitespace-pre-wrap text-sm">{m.content_text}</div>
                  )}
                  {Array.isArray(m.attachments) && m.attachments.length > 0 ? (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {m.attachments.map((attachment) => {
                        const sizeLabel = formatAttachmentSize(attachment.size_bytes)
                        const kindLabel = formatAttachmentKind(attachment.kind)
                        return (
                          <div
                            key={attachment.attachment_id}
                            className="flex items-center gap-1 rounded-full border bg-background/60 px-2 py-1 text-xs"
                          >
                            <span className="max-w-[200px] truncate">{attachment.file_name}</span>
                            {kindLabel ? <span className="text-muted-foreground">{kindLabel}</span> : null}
                            {sizeLabel ? <span className="text-muted-foreground">{sizeLabel}</span> : null}
                            {canPreviewAttachmentTarget(attachment.file_name, attachment.mime_type, attachment.kind) ? (
                              <button
                                type="button"
                                className="rounded p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
                                onClick={() => void openPreviewForAttachment(attachment)}
                                title="预览附件"
                              >
                                <Eye className="h-3 w-3" />
                              </button>
                            ) : null}
                          </div>
                        )
                      })}
                    </div>
                  ) : null}
                  {isUser && inputMeta?.model_input ? (
                    <details className="mt-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                      <summary className="cursor-pointer select-none text-muted-foreground">
                        查看实际携带内容
                      </summary>
                      {contextModeLabel ? (
                        <div className="mt-2 text-[11px] text-muted-foreground">{contextModeLabel}</div>
                      ) : null}
                      <div className="mt-2 whitespace-pre-wrap break-words text-muted-foreground">
                        {inputMeta.model_input}
                      </div>
                    </details>
                  ) : null}
                  {isAssistant && tokenUsage ? (
                    <div className="mt-2 border-t pt-2 text-[11px] text-muted-foreground">{tokenUsage}</div>
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
                  AI{pendingIdentity ? ` · ${pendingIdentity}` : ''} {streaming ? '回复中' : '思考中'}
                </div>
                {displayedThinking.trim() ? (
                  <details className="mb-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs">
                    <summary className="cursor-pointer select-none text-muted-foreground">
                      查看完整思考过程
                    </summary>
                    <div className="chat-markdown mt-2 text-xs text-muted-foreground">
                      <ReactMarkdown remarkPlugins={[remarkGfm]}>{displayedThinking}</ReactMarkdown>
                    </div>
                  </details>
                ) : pendingThinkingTranslation ? (
                  <div className="mb-2 rounded-md border border-dashed bg-muted/40 px-2 py-1 text-xs text-muted-foreground">
                    正在翻译思考过程…
                  </div>
                ) : null}
                {streaming ? (
                  <div className="chat-markdown text-sm">
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{streaming}</ReactMarkdown>
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">正在思考…</div>
                )}
              </div>
            </div>
          ) : null}
        </div>

        <div
          className={`flex gap-2 rounded-xl border border-dashed p-2 transition ${dragActive ? 'border-primary bg-primary/5 shadow-sm' : 'border-transparent'}` }
          onDragEnter={handleComposerDragEnter}
          onDragOver={handleComposerDragOver}
          onDragLeave={handleComposerDragLeave}
          onDrop={handleComposerDrop}
        >
          <div className="flex flex-1 flex-col gap-2">
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={(event) => {
                appendSelectedFiles(event.target.files)
                event.currentTarget.value = ''
              }}
            />
            {selectedFiles.length > 0 ? (
              <div className="flex flex-wrap gap-2 rounded-lg border bg-background/40 p-2">
                {selectedFiles.map((file) => {
                  const identity = fileIdentity(file)
                  return (
                    <div
                      key={identity}
                      className="flex max-w-full items-center gap-1 rounded-full border px-2 py-1 text-xs text-foreground"
                    >
                      <span className="max-w-[180px] truncate">{file.name}</span>
                      <span className="text-muted-foreground">{guessPendingFileKind(file)}</span>
                      <span className="text-muted-foreground">{formatAttachmentSize(file.size)}</span>
                      {canPreviewAttachmentTarget(file.name, file.type) ? (
                        <button
                          type="button"
                          className="rounded p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
                          onClick={() => void openPreviewForFile(file)}
                          title="预览附件"
                        >
                          <Eye className="h-3 w-3" />
                        </button>
                      ) : null}
                      <button
                        type="button"
                        className="rounded p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
                        onClick={() => removeSelectedFile(identity)}
                        title="移除附件"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </div>
                  )
                })}
              </div>
            ) : null}
            <Textarea
              value={input}
              onValueChange={setInput}
              placeholder="输入消息或上传附件..."
              minRows={2}
              isDisabled={!activeSessionId || sending}
              className="flex-1"
            />
            {dragActive ? <div className="text-xs text-primary">释放鼠标即可添加附件</div> : null}
          </div>
          <div className="flex w-[200px] flex-col gap-2">
            <Button
              variant="flat"
              startContent={<Paperclip className="h-4 w-4" />}
              isDisabled={!activeSessionId || sending}
              onPress={openFilePicker}
            >
              上传附件
            </Button>
            <Select
              label="Expert"
              selectedKeys={effectiveTurnExpertId ? new Set([effectiveTurnExpertId]) : new Set()}
              onSelectionChange={(keys) => {
                if (keys === 'all') return
                const first = keys.values().next().value
                if (typeof first === 'string') setTurnExpertId(first)
              }}
              size="sm"
              disallowEmptySelection
              isDisabled={!activeSessionId || sending || selectableExperts.length === 0}
            >
              {selectableExperts.map((e) => (
                <SelectItem key={e.id}>{e.label || e.id}</SelectItem>
              ))}
            </Select>
            <Button
              color="primary"
              isDisabled={!activeSessionId || sending || (!input.trim() && selectedFiles.length === 0)}
              onPress={() => void onSend()}
            >
              {sending ? '发送中…' : '发送'}
            </Button>
          </div>
        </div>
      </section>
            <AttachmentPreviewModal preview={preview} onClose={closePreview} />

    </div>
  )
}
