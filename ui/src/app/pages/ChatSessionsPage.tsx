import { type DragEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Chip, Input, Select, SelectItem, Textarea } from '@heroui/react'
import { Eye, Paperclip, Trash2, X } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import { onWsEnvelope } from '@/lib/wsBus'
import { chatAttachmentContentUrl, fetchCLIToolSettings, type ChatAttachment, type CLITool, type LLMModelProfile } from '@/lib/daemon'
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
  const [newModelId, setNewModelId] = useState('')
  const [turnModelId, setTurnModelId] = useState('')
  const [cliTools, setCliTools] = useState<CLITool[]>([])
  const [toolModels, setToolModels] = useState<LLMModelProfile[]>([])
  const messageScrollRef = useRef<HTMLDivElement | null>(null)
  const shouldAutoScrollRef = useRef(true)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const dragDepthRef = useRef(0)

  const activeSession = useMemo(
    () => sessions.find((s) => s.session_id === activeSessionId) ?? null,
    [sessions, activeSessionId],
  )
  const expertsById = useMemo(() => {
    const map = new Map<string, (typeof experts)[number]>()
    for (const e of experts) map.set(e.id, e)
    return map
  }, [experts])
  const selectableTools = useMemo(() => cliTools.filter((tool) => tool.enabled), [cliTools])
  const toolsById = useMemo(() => {
    const map = new Map<string, CLITool>()
    for (const tool of selectableTools) map.set(tool.id, tool)
    return map
  }, [selectableTools])
  const defaultSelectableExpertId = selectableTools[0]?.id ?? ''
  const inferToolId = useCallback(
    (meta?: { expert_id?: string; provider?: string } | null) => {
      const explicit = meta?.expert_id?.trim() || ''
      if (explicit && toolsById.has(explicit)) return explicit
      const expert = explicit ? expertsById.get(explicit) : undefined
      if (expert?.cli_family === 'claude') return 'claude'
      if (expert?.cli_family === 'codex') return 'codex'
      const provider = (meta?.provider?.trim() || '').toLowerCase()
      if (provider === 'anthropic') return 'claude'
      if (provider === 'openai') return 'codex'
      return defaultSelectableExpertId
    },
    [defaultSelectableExpertId, expertsById, toolsById],
  )
  const effectiveNewExpertId =
    newExpertId && selectableTools.some((tool) => tool.id === newExpertId)
      ? newExpertId
      : defaultSelectableExpertId
  const effectiveTurnExpertId =
    turnExpertId && selectableTools.some((tool) => tool.id === turnExpertId)
      ? turnExpertId
      : inferToolId(activeSession)
  const modelsForTool = useCallback(
    (toolId: string) => {
      const tool = toolsById.get(toolId)
      if (!tool) return [] as LLMModelProfile[]
      return toolModels.filter((model) => (model.provider || '').trim() === tool.protocol_family)
    },
    [toolModels, toolsById],
  )
  const effectiveNewModelId = useMemo(() => {
    const models = modelsForTool(effectiveNewExpertId)
    if (newModelId && models.some((model) => model.id === newModelId)) return newModelId
    const tool = toolsById.get(effectiveNewExpertId)
    if (tool?.default_model_id && models.some((model) => model.id === tool.default_model_id)) return tool.default_model_id
    return models[0]?.id ?? ''
  }, [effectiveNewExpertId, modelsForTool, newModelId, toolsById])
  const effectiveTurnModelId = useMemo(() => {
    const models = modelsForTool(effectiveTurnExpertId)
    if (turnModelId && models.some((model) => model.id === turnModelId)) return turnModelId
    if (activeSession?.model && models.some((model) => model.model === activeSession.model || model.id === activeSession.model)) {
      return models.find((model) => model.model === activeSession.model || model.id === activeSession.model)?.id ?? ''
    }
    const tool = toolsById.get(effectiveTurnExpertId)
    if (tool?.default_model_id && models.some((model) => model.id === tool.default_model_id)) return tool.default_model_id
    return models[0]?.id ?? ''
  }, [activeSession?.model, effectiveTurnExpertId, modelsForTool, toolsById, turnModelId])

  const formatModelIdentity = useCallback(
    (meta?: { expert_id?: string; provider?: string; model?: string } | null): string => {
      if (!meta) return ''
      const expertId = meta.expert_id?.trim() || ''
      const tool = expertId ? toolsById.get(expertId) : undefined
      const expert = expertId ? expertsById.get(expertId) : undefined
      const label = tool?.label || expert?.label || expertId
      const provider = (meta.provider?.trim() || tool?.protocol_family || expert?.provider || '').trim()
      const model = (meta.model?.trim() || expert?.model || '').trim()
      const parts: string[] = []
      if (label) parts.push(label)
      if (provider && model) parts.push(`${provider}/${model}`)
      return parts.join(' · ')
    },
    [expertsById, toolsById],
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
    void fetchCLIToolSettings(daemonUrl)
      .then((settings) => {
        if (cancelled) return
        setCliTools(settings.tools ?? [])
        setToolModels(settings.models ?? [])
      })
      .catch(() => {
        if (cancelled) return
        setCliTools([])
        setToolModels([])
      })
    return () => {
      cancelled = true
    }
  }, [daemonUrl])

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
        cli_tool_id: effectiveNewExpertId.trim() || undefined,
        model_id: effectiveNewModelId.trim() || undefined,
      })
      setNewTitle('')
      setInput('')
      setSelectedFiles([])
      setTurnExpertId(inferToolId(created))
      setTurnModelId(created.model || '')
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
      await sendTurn(daemonUrl, activeSessionId, text, undefined, effectiveTurnExpertId.trim() || undefined, effectiveTurnModelId.trim() || undefined, draftFiles)
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
      setTurnExpertId(inferToolId(forked))
      setTurnModelId(forked.model || '')
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
        setTurnModelId('')
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
      const model = modelsForTool(effectiveTurnExpertId).find((item) => item.id === effectiveTurnModelId)?.model || ''
      const id = formatModelIdentity({ expert_id: effectiveTurnExpertId.trim(), provider: toolsById.get(effectiveTurnExpertId)?.protocol_family, model })
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

  const activeSessionIdentity = useMemo(() => {
    if (!activeSession) return ''
    return formatModelIdentity({
      expert_id: activeSession.expert_id,
      provider: activeSession.provider,
      model: activeSession.model,
    })
  }, [activeSession, formatModelIdentity])

  if (health.status === 'error') {
    return <Alert color="danger" title="无法连接守护进程" description={health.message} />
  }

  return (
    <>
      <div className="grid h-full min-h-0 w-full grid-cols-1 gap-3 lg:grid-cols-[272px_minmax(0,1fr)]">
        <section className="flex min-h-0 flex-col overflow-hidden rounded-[28px] border bg-card/70 p-3 shadow-sm">
          <div className="mb-3 flex items-start justify-between gap-3 px-1">
            <div className="min-w-0">
              <div className="text-sm font-semibold">会话</div>
              <div className="mt-1 text-xs text-muted-foreground">选择或创建一个对话工作区</div>
            </div>
            <Chip size="sm" variant="flat">
              {visibleSessions.length}
            </Chip>
          </div>

          <div className="space-y-2 rounded-[22px] border bg-background/60 p-2">
            <Input
              aria-label="新会话标题"
              value={newTitle}
              onValueChange={setNewTitle}
              placeholder="新会话标题"
              size="sm"
            />
            <Select
              label="CLI 工具"
              selectedKeys={effectiveNewExpertId ? new Set([effectiveNewExpertId]) : new Set()}
              onSelectionChange={(keys) => {
                if (keys === 'all') return
                const first = keys.values().next().value
                if (typeof first === 'string') setNewExpertId(first)
              }}
              size="sm"
              disallowEmptySelection
              isDisabled={selectableTools.length === 0}
            >
              {selectableTools.map((tool) => (
                <SelectItem key={tool.id}>{tool.label}</SelectItem>
              ))}
            </Select>
            <Select
              label="模型"
              selectedKeys={effectiveNewModelId ? new Set([effectiveNewModelId]) : new Set()}
              onSelectionChange={(keys) => {
                if (keys === 'all') return
                const first = keys.values().next().value
                if (typeof first === 'string') setNewModelId(first)
              }}
              size="sm"
              disallowEmptySelection
              isDisabled={modelsForTool(effectiveNewExpertId).length === 0}
            >
              {modelsForTool(effectiveNewExpertId).map((model) => (
                <SelectItem key={model.id}>{model.label || model.id} · {model.model}</SelectItem>
              ))}
            </Select>
            <Button
              color="primary"
              size="sm"
              className="w-full"
              isDisabled={selectableTools.length === 0}
              onPress={() => void onCreate()}
            >
              新建会话
            </Button>
          </div>

          {error ? <Alert color="danger" title="加载失败" description={error} className="mt-3" /> : null}

          <div className="mt-3 min-h-0 flex-1 space-y-2 overflow-auto pr-1">
            {loading ? (
              <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">加载中…</div>
            ) : visibleSessions.length === 0 ? (
              <div className="rounded-2xl border border-dashed px-3 py-4 text-xs text-muted-foreground">暂无会话</div>
            ) : (
              visibleSessions.map((s) => (
                <button
                  key={s.session_id}
                  className={`w-full rounded-[22px] border px-3 py-3 text-left transition ${
                    s.session_id === activeSessionId
                      ? 'border-primary/50 bg-primary/5 shadow-sm'
                      : 'border-transparent bg-background/40 hover:border-default-200 hover:bg-background/80'
                  }`}
                  onClick={() => {
                    setActiveSession(s.session_id)
                    setTurnExpertId(inferToolId(s))
                    setTurnModelId(s.model || '')
                    setSelectedFiles([])
                  }}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium">{s.title}</div>
                      <div className="mt-1 truncate text-[11px] text-muted-foreground">{s.session_id}</div>
                    </div>
                    <span
                      className="shrink-0 rounded-full p-1 text-muted-foreground transition-colors hover:bg-danger/10 hover:text-danger"
                      title="删除会话"
                      onClick={(event) => {
                        event.stopPropagation()
                        void onDeleteSession(s.session_id)
                      }}
                    >
                      <Trash2 className="h-3.5 w-3.5" aria-hidden="true" focusable="false" />
                    </span>
                  </div>
                  <div className="mt-2 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                    <span className="truncate">{s.provider}/{s.model}</span>
                    <span className="shrink-0">{formatRelativeTime(s.updated_at)}</span>
                  </div>
                </button>
              ))
            )}
          </div>
        </section>

        <section className="flex min-h-0 flex-col overflow-hidden rounded-[30px] border bg-card/70 shadow-sm">
          <div className="flex shrink-0 items-start justify-between gap-3 border-b bg-background/60 px-5 py-4 md:px-6">
            <div className="min-w-0">
              <div className="truncate text-base font-semibold">
                {activeSession ? activeSession.title : '请选择或创建会话'}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
                {activeSession ? (
                  <>
                    <span className="truncate">{activeSession.session_id}</span>
                    {activeSessionIdentity ? <span>· {activeSessionIdentity}</span> : null}
                  </>
                ) : (
                  <span>左侧创建会话后即可开始对话</span>
                )}
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              {activeSession ? (
                <Chip size="sm" variant="flat">
                  {activeSession.status}
                </Chip>
              ) : null}
              <Button size="sm" variant="light" isDisabled={!activeSessionId} onPress={() => void onFork()}>
                分叉
              </Button>
            </div>
          </div>

          <div className="flex min-h-0 flex-1 flex-col overflow-hidden bg-background/30">
            <div ref={messageScrollRef} className="min-h-0 flex-1 overflow-y-auto px-4 py-6 md:px-8">
              <div className="mx-auto flex w-full max-w-[880px] flex-col gap-5">
                {messages.length === 0 && !pendingAssistant ? (
                  <div className="flex min-h-full flex-1 items-center justify-center py-16">
                    <div className="max-w-md rounded-[24px] border border-dashed bg-background/70 px-6 py-8 text-center">
                      <div className="text-base font-medium">开始新的对话</div>
                      <div className="mt-2 text-sm text-muted-foreground">
                        从左侧选择会话，或先创建一个新会话。
                      </div>
                    </div>
                  </div>
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
                        className={`rounded-[24px] px-4 py-3 ${
                          isUser
                            ? 'border border-default-200 bg-default-100/90 shadow-sm'
                            : 'border border-default-200/70 bg-background/80 shadow-sm'
                        } ${fullWidth ? 'w-full' : isUser ? 'max-w-[78%]' : 'max-w-[90%]'}`}
                      >
                        <div className="mb-1 text-[11px] font-medium text-muted-foreground">
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
                      className={`rounded-[24px] border border-dashed bg-background/80 px-4 py-3 shadow-sm ${
                        shouldUseFullWidth(streaming) ? 'w-full' : 'max-w-[90%]'
                      }`}
                    >
                      <div className="mb-1 text-[11px] font-medium text-muted-foreground">
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
            </div>

            <div className="shrink-0 border-t bg-background/80 px-4 py-4 backdrop-blur md:px-8">
              <div className="mx-auto w-full max-w-[880px]">
                <div
                  className={`rounded-[28px] border bg-background p-3 shadow-sm transition ${dragActive ? 'border-primary bg-primary/5' : 'border-default-200/80'}`}
                  onDragEnter={handleComposerDragEnter}
                  onDragOver={handleComposerDragOver}
                  onDragLeave={handleComposerDragLeave}
                  onDrop={handleComposerDrop}
                >
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
                    <div className="mb-3 flex flex-wrap gap-2 rounded-2xl border bg-background/40 p-2">
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
                    minRows={3}
                    isDisabled={!activeSessionId || sending}
                    className="w-full"
                  />

                  <div className="mt-3 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
                    <div className="flex flex-col gap-2 md:flex-row md:items-end">
                      <Button
                        variant="flat"
                        startContent={<Paperclip className="h-4 w-4" />}
                        isDisabled={!activeSessionId || sending}
                        onPress={openFilePicker}
                      >
                        上传附件
                      </Button>
                      <Select
                        aria-label="本条 Expert"
                        placeholder="选择本条 Expert"
                        selectedKeys={effectiveTurnExpertId ? new Set([effectiveTurnExpertId]) : new Set()}
                        onSelectionChange={(keys) => {
                          if (keys === 'all') return
                          const first = keys.values().next().value
                          if (typeof first === 'string') setTurnExpertId(first)
                        }}
                        size="sm"
                        disallowEmptySelection
                        isDisabled={!activeSessionId || sending || selectableTools.length === 0}
                        className="md:min-w-[260px]"
                      >
                        {selectableTools.map((tool) => (
                          <SelectItem key={tool.id}>{tool.label}</SelectItem>
                        ))}
                      </Select>
                      <Select
                        label="模型"
                        selectedKeys={effectiveTurnModelId ? new Set([effectiveTurnModelId]) : new Set()}
                        onSelectionChange={(keys) => {
                          if (keys === 'all') return
                          const first = keys.values().next().value
                          if (typeof first === 'string') setTurnModelId(first)
                        }}
                        size="sm"
                        disallowEmptySelection
                        isDisabled={!activeSessionId || sending || modelsForTool(effectiveTurnExpertId).length === 0}
                        className="md:min-w-[260px]"
                      >
                        {modelsForTool(effectiveTurnExpertId).map((model) => (
                          <SelectItem key={model.id}>{model.label || model.id} · {model.model}</SelectItem>
                        ))}
                      </Select>
                    </div>
                    <div className="flex items-center justify-end gap-2">
                      {dragActive ? <div className="text-xs text-primary">释放鼠标即可添加附件</div> : null}
                      <Button
                        color="primary"
                        className="min-w-[112px]"
                        isDisabled={!activeSessionId || sending || (!input.trim() && selectedFiles.length === 0)}
                        onPress={() => void onSend()}
                      >
                        {sending ? '发送中…' : '发送'}
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
      <AttachmentPreviewModal preview={preview} onClose={closePreview} />
    </>
  )

}
