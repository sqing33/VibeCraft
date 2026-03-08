import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

import {
  createChatSession,
  fetchChatMessages,
  fetchChatSessions,
  fetchChatTurns,
  patchChatSession,
  postChatCompact,
  postChatFork,
  postChatTurn,
  type ChatAttachment,
  type ChatMessage,
  type ChatSession,
  type ChatTurnTimeline,
} from '@/lib/daemon'
import {
  applyThinkingTranslationDelta,
  applyTurnFeedEvent,
  buildTurnFeedFromTimeline,
  ensureTurnFeed,
  finalizeTurnFeed,
  type ChatTurnEventPayload,
  type ChatTurnFeed,
} from '@/lib/chatTurnFeed'

type TurnMeta = {
  expert_id?: string
  provider?: string
  model?: string
}

type TurnInputMeta = {
  model_input: string
  context_mode?: string
}

type UsageMeta = {
  token_in?: number
  token_out?: number
  cached_input_tokens?: number
}

type ThinkingTranslationState = {
  applied: boolean
  failed: boolean
}

const chatRuntimeStorageKey = 'vibe-tree-chat-runtime'

function clearLiveTurnStateShape<T extends {
  streamingBySession: Record<string, string>
  thinkingBySession: Record<string, string>
  translatedThinkingBySession: Record<string, string>
  thinkingTranslationStateBySession: Record<string, ThinkingTranslationState | undefined>
  turnMetaBySession: Record<string, TurnMeta | null>
  activeTurnFeedBySession: Record<string, ChatTurnFeed | undefined>
}>(state: T, sessionId: string): T {
  const nextStreaming = { ...state.streamingBySession }
  const nextThinking = { ...state.thinkingBySession }
  const nextTranslated = { ...state.translatedThinkingBySession }
  const nextTranslationState = { ...state.thinkingTranslationStateBySession }
  const nextTurnMeta = { ...state.turnMetaBySession }
  const nextActiveFeed = { ...state.activeTurnFeedBySession }
  delete nextStreaming[sessionId]
  delete nextThinking[sessionId]
  delete nextTranslated[sessionId]
  delete nextTranslationState[sessionId]
  delete nextTurnMeta[sessionId]
  delete nextActiveFeed[sessionId]
  return {
    ...state,
    streamingBySession: nextStreaming,
    thinkingBySession: nextThinking,
    translatedThinkingBySession: nextTranslated,
    thinkingTranslationStateBySession: nextTranslationState,
    turnMetaBySession: nextTurnMeta,
    activeTurnFeedBySession: nextActiveFeed,
  }
}

function shouldClearActiveTurnFeed(feed: ChatTurnFeed | undefined, messages: ChatMessage[]): boolean {
  if (!feed || messages.length === 0) return false
  const userMessage = messages.find((message) => message.message_id === feed.user_message_id && message.role === 'user')
  if (!userMessage) return false
  return messages.some((message) => message.role === 'assistant' && message.turn === userMessage.turn)
}

function guessAttachmentKind(file: File): string {
  const name = file.name.toLowerCase()
  const type = file.type.toLowerCase()
  if (type.startsWith('image/')) return 'image'
  if (type === 'application/pdf' || name.endsWith('.pdf')) return 'pdf'
  return 'text'
}

function buildOptimisticAttachments(
  sessionId: string,
  messageId: string,
  files: File[],
  createdAt: number,
): ChatAttachment[] {
  return files.map((file, index) => ({
    attachment_id: `${messageId}_att_${index + 1}`,
    session_id: sessionId,
    message_id: messageId,
    kind: guessAttachmentKind(file),
    file_name: file.name,
    mime_type: file.type || 'application/octet-stream',
    size_bytes: file.size,
    created_at: createdAt,
  }))
}

function hydrateTurnsIntoState<T extends {
  messagesBySession: Record<string, ChatMessage[]>
  streamingBySession: Record<string, string>
  thinkingBySession: Record<string, string>
  translatedThinkingBySession: Record<string, string>
  thinkingTranslationStateBySession: Record<string, ThinkingTranslationState | undefined>
  turnMetaBySession: Record<string, TurnMeta | null>
  turnInputByUserMessageId: Record<string, TurnInputMeta | undefined>
  usageByMessageId: Record<string, UsageMeta | undefined>
  activeTurnFeedBySession: Record<string, ChatTurnFeed | undefined>
  completedTurnFeedByAssistantMessageId: Record<string, ChatTurnFeed | undefined>
}>(state: T, sessionId: string, turns: ChatTurnTimeline[]): T {
  const sessionMessages = state.messagesBySession[sessionId] ?? []
  const sessionUserIDs = new Set(sessionMessages.filter((message) => message.role === 'user').map((message) => message.message_id))
  const sessionAssistantIDs = new Set(sessionMessages.filter((message) => message.role === 'assistant').map((message) => message.message_id))
  for (const turn of turns) {
    sessionUserIDs.add(turn.user_message_id)
    if (typeof turn.assistant_message_id === 'string' && turn.assistant_message_id) {
      sessionAssistantIDs.add(turn.assistant_message_id)
    }
  }

  const nextTurnInputByUserMessageId = { ...state.turnInputByUserMessageId }
  for (const userMessageId of sessionUserIDs) {
    delete nextTurnInputByUserMessageId[userMessageId]
  }
  const nextUsageByMessageId = { ...state.usageByMessageId }
  for (const assistantMessageId of sessionAssistantIDs) {
    delete nextUsageByMessageId[assistantMessageId]
  }
  const nextCompleted = Object.fromEntries(
    Object.entries(state.completedTurnFeedByAssistantMessageId).filter(([, feed]) => feed?.session_id !== sessionId),
  ) as Record<string, ChatTurnFeed | undefined>
  const nextActive = { ...state.activeTurnFeedBySession }
  delete nextActive[sessionId]
  const nextStreaming = { ...state.streamingBySession, [sessionId]: '' }
  const nextThinking = { ...state.thinkingBySession, [sessionId]: '' }
  const nextTranslatedThinking = { ...state.translatedThinkingBySession, [sessionId]: '' }
  const nextTranslationState = { ...state.thinkingTranslationStateBySession }
  delete nextTranslationState[sessionId]
  const nextTurnMeta = { ...state.turnMetaBySession, [sessionId]: null }

  for (const turn of turns) {
    if (typeof turn.model_input === 'string' && turn.model_input.trim()) {
      nextTurnInputByUserMessageId[turn.user_message_id] = {
        model_input: turn.model_input,
        context_mode: turn.context_mode,
      }
    }
    if (typeof turn.assistant_message_id === 'string' && turn.assistant_message_id) {
      nextUsageByMessageId[turn.assistant_message_id] = {
        token_in: turn.token_in,
        token_out: turn.token_out,
        cached_input_tokens: turn.cached_input_tokens,
      }
    }
    const feed = buildTurnFeedFromTimeline(turn)
    if (typeof turn.assistant_message_id === 'string' && turn.assistant_message_id && turn.status === 'completed') {
      nextCompleted[turn.assistant_message_id] = feed
      continue
    }
    if ((turn.status === 'running' || turn.status === 'failed') && feed.entries.length > 0) {
      nextActive[sessionId] = feed
      nextTurnMeta[sessionId] = feed.turnMeta ?? null
      nextTranslationState[sessionId] = {
        applied:
          turn.thinking_translation_applied === true ||
          feed.entries.some((entry) => typeof entry.meta?.translated_content === 'string' && entry.meta.translated_content.trim()),
        failed: turn.thinking_translation_failed === true,
      }
    }
  }

  return {
    ...state,
    streamingBySession: nextStreaming,
    thinkingBySession: nextThinking,
    translatedThinkingBySession: nextTranslatedThinking,
    thinkingTranslationStateBySession: nextTranslationState,
    turnMetaBySession: nextTurnMeta,
    turnInputByUserMessageId: nextTurnInputByUserMessageId,
    usageByMessageId: nextUsageByMessageId,
    activeTurnFeedBySession: nextActive,
    completedTurnFeedByAssistantMessageId: nextCompleted,
  }
}

export type ChatStore = {
  sessions: ChatSession[]
  activeSessionId: string | null
  messagesBySession: Record<string, ChatMessage[]>
  streamingBySession: Record<string, string>
  thinkingBySession: Record<string, string>
  translatedThinkingBySession: Record<string, string>
  thinkingTranslationStateBySession: Record<string, ThinkingTranslationState | undefined>
  turnMetaBySession: Record<string, TurnMeta | null>
  turnInputByUserMessageId: Record<string, TurnInputMeta | undefined>
  usageByMessageId: Record<string, UsageMeta | undefined>
  activeTurnFeedBySession: Record<string, ChatTurnFeed | undefined>
  completedTurnFeedByAssistantMessageId: Record<string, ChatTurnFeed | undefined>
  loading: boolean
  sending: boolean
  error: string | null
  setActiveSession: (sessionId: string | null) => void
  setSessions: (sessions: ChatSession[]) => void
  upsertSession: (session: ChatSession) => void
  setMessages: (sessionId: string, messages: ChatMessage[]) => void
  appendMessage: (sessionId: string, msg: ChatMessage) => void
  appendStreamingDelta: (sessionId: string, delta: string) => void
  appendThinkingDelta: (sessionId: string, delta: string) => void
  appendTranslatedThinkingDelta: (sessionId: string, delta: string, entryId?: string) => void
  setThinking: (sessionId: string, thinking: string) => void
  setTranslatedThinking: (sessionId: string, thinking: string) => void
  clearStreaming: (sessionId: string) => void
  clearThinking: (sessionId: string) => void
  resetThinkingTranslation: (sessionId: string) => void
  setThinkingTranslationState: (sessionId: string, state: ThinkingTranslationState) => void
  setTurnMeta: (sessionId: string, meta: TurnMeta | null) => void
  setTurnInputMeta: (userMessageId: string, meta: TurnInputMeta) => void
  setUsageMeta: (messageId: string, meta: UsageMeta) => void
  startTurnFeed: (sessionId: string, userMessageId: string, meta?: TurnMeta | null) => void
  applyTurnEvent: (event: ChatTurnEventPayload) => void
  completeTurnFeed: (
    sessionId: string,
    assistantMessageId: string,
    opts?: { thinking?: string; translatedThinking?: string; translationFailed?: boolean },
  ) => void
  clearTurnFeed: (sessionId: string) => void
  refreshSessions: (daemonUrl: string) => Promise<void>
  loadMessages: (daemonUrl: string, sessionId: string) => Promise<void>
  loadTurns: (daemonUrl: string, sessionId: string) => Promise<void>
  createSession: (
    daemonUrl: string,
    req: { title?: string; expert_id?: string; cli_tool_id?: string; model_id?: string; workspace_path?: string; mcp_server_ids?: string[] },
  ) => Promise<ChatSession>
  sendTurn: (
    daemonUrl: string,
    sessionId: string,
    input: string,
    expertId?: string,
    cliToolId?: string,
    modelId?: string,
    files?: File[],
    mcpServerIDs?: string[],
  ) => Promise<void>
  compactSession: (daemonUrl: string, sessionId: string) => Promise<void>
  forkSession: (daemonUrl: string, sessionId: string) => Promise<ChatSession>
  archiveSession: (daemonUrl: string, sessionId: string) => Promise<void>
}

export const useChatStore = create<ChatStore>()(
  persist(
    (set, get) => ({
  sessions: [],
  activeSessionId: null,
  messagesBySession: {},
  streamingBySession: {},
  thinkingBySession: {},
  translatedThinkingBySession: {},
  thinkingTranslationStateBySession: {},
  turnMetaBySession: {},
  turnInputByUserMessageId: {},
  usageByMessageId: {},
  activeTurnFeedBySession: {},
  completedTurnFeedByAssistantMessageId: {},
  loading: false,
  sending: false,
  error: null,
  setActiveSession: (activeSessionId) => set({ activeSessionId }),
  setSessions: (sessions) => set({ sessions }),
  upsertSession: (session) =>
    set((state) => {
      const exists = state.sessions.some((s) => s.session_id === session.session_id)
      const next = exists
        ? state.sessions.map((s) => (s.session_id === session.session_id ? session : s))
        : [session, ...state.sessions]
      next.sort((a, b) => b.updated_at - a.updated_at)
      return { sessions: next }
    }),
  setMessages: (sessionId, messages) =>
    set((state) => {
      const nextState = {
        ...state,
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: messages,
        },
      }
      if (!shouldClearActiveTurnFeed(state.activeTurnFeedBySession[sessionId], messages)) {
        return nextState
      }
      return clearLiveTurnStateShape(nextState, sessionId)
    }),
  appendMessage: (sessionId, msg) =>
    set((state) => {
      const prev = state.messagesBySession[sessionId] ?? []
      if (prev.some((m) => m.message_id === msg.message_id)) {
        return state
      }
      return {
        messagesBySession: {
          ...state.messagesBySession,
          [sessionId]: [...prev, msg],
        },
      }
    }),
  appendStreamingDelta: (sessionId, delta) =>
    set((state) => ({
      streamingBySession: {
        ...state.streamingBySession,
        [sessionId]: (state.streamingBySession[sessionId] ?? '') + delta,
      },
    })),
  appendThinkingDelta: (sessionId, delta) =>
    set((state) => ({
      thinkingBySession: {
        ...state.thinkingBySession,
        [sessionId]: (state.thinkingBySession[sessionId] ?? '') + delta,
      },
    })),
  appendTranslatedThinkingDelta: (sessionId, delta, entryId) =>
    set((state) => ({
      translatedThinkingBySession: {
        ...state.translatedThinkingBySession,
        [sessionId]: (state.translatedThinkingBySession[sessionId] ?? '') + delta,
      },
      activeTurnFeedBySession: {
        ...state.activeTurnFeedBySession,
        [sessionId]: applyThinkingTranslationDelta(state.activeTurnFeedBySession[sessionId], delta, entryId),
      },
    })),
  setThinking: (sessionId, thinking) =>
    set((state) => ({
      thinkingBySession: {
        ...state.thinkingBySession,
        [sessionId]: thinking,
      },
    })),
  setTranslatedThinking: (sessionId, thinking) =>
    set((state) => ({
      translatedThinkingBySession: {
        ...state.translatedThinkingBySession,
        [sessionId]: thinking,
      },
    })),
  clearStreaming: (sessionId) =>
    set((state) => ({
      streamingBySession: {
        ...state.streamingBySession,
        [sessionId]: '',
      },
    })),
  clearThinking: (sessionId) =>
    set((state) => ({
      thinkingBySession: {
        ...state.thinkingBySession,
        [sessionId]: '',
      },
    })),
  resetThinkingTranslation: (sessionId) =>
    set((state) => ({
      translatedThinkingBySession: {
        ...state.translatedThinkingBySession,
        [sessionId]: '',
      },
      thinkingTranslationStateBySession: {
        ...state.thinkingTranslationStateBySession,
        [sessionId]: { applied: false, failed: false },
      },
    })),
  setThinkingTranslationState: (sessionId, translationState) =>
    set((state) => ({
      thinkingTranslationStateBySession: {
        ...state.thinkingTranslationStateBySession,
        [sessionId]: translationState,
      },
    })),
  setTurnMeta: (sessionId, meta) =>
    set((state) => ({
      turnMetaBySession: {
        ...state.turnMetaBySession,
        [sessionId]: meta,
      },
    })),
  setTurnInputMeta: (userMessageId, meta) =>
    set((state) => ({
      turnInputByUserMessageId: {
        ...state.turnInputByUserMessageId,
        [userMessageId]: meta,
      },
    })),
  setUsageMeta: (messageId, meta) =>
    set((state) => ({
      usageByMessageId: {
        ...state.usageByMessageId,
        [messageId]: meta,
      },
    })),
  startTurnFeed: (sessionId, userMessageId, meta) =>
    set((state) => ({
      activeTurnFeedBySession: {
        ...state.activeTurnFeedBySession,
        [sessionId]: ensureTurnFeed(state.activeTurnFeedBySession[sessionId], sessionId, userMessageId, meta),
      },
    })),
  applyTurnEvent: (event) =>
    set((state) => ({
      activeTurnFeedBySession: {
        ...state.activeTurnFeedBySession,
        [event.session_id]: applyTurnFeedEvent(state.activeTurnFeedBySession[event.session_id], event),
      },
    })),
  completeTurnFeed: (sessionId, assistantMessageId, opts) =>
    set((state) => {
      const activeFeed = finalizeTurnFeed(state.activeTurnFeedBySession[sessionId], assistantMessageId, opts)
      const clearedState = clearLiveTurnStateShape(state, sessionId)
      if (!activeFeed) {
        return clearedState
      }
      return {
        ...clearedState,
        completedTurnFeedByAssistantMessageId: {
          ...clearedState.completedTurnFeedByAssistantMessageId,
          [assistantMessageId]: activeFeed,
        },
      }
    }),
  clearTurnFeed: (sessionId) =>
    set((state) => clearLiveTurnStateShape(state, sessionId)),
  refreshSessions: async (daemonUrl) => {
    set({ loading: true, error: null })
    try {
      const sessions = await fetchChatSessions(daemonUrl)
      const activeSessions = sessions.filter((s) => s.status === 'active')
      const fallbackSessionId = activeSessions[0]?.session_id ?? sessions[0]?.session_id ?? null
      set((state) => {
        const validSessionIDs = new Set(sessions.map((session) => session.session_id))
        let nextState = {
          ...state,
          sessions,
          loading: false,
          activeSessionId:
            state.activeSessionId && sessions.some((s) => s.session_id === state.activeSessionId)
              ? state.activeSessionId
              : fallbackSessionId,
        }
        for (const sessionId of Object.keys(state.activeTurnFeedBySession)) {
          if (!validSessionIDs.has(sessionId)) {
            nextState = clearLiveTurnStateShape(nextState, sessionId)
          }
        }
        return nextState
      })
    } catch (err: unknown) {
      set({
        loading: false,
        error: err instanceof Error ? err.message : String(err),
      })
      throw err
    }
  },
  loadMessages: async (daemonUrl, sessionId) => {
    const messages = await fetchChatMessages(daemonUrl, sessionId)
    get().setMessages(sessionId, messages)
  },
  loadTurns: async (daemonUrl, sessionId) => {
    const turns = await fetchChatTurns(daemonUrl, sessionId)
    set((state) => hydrateTurnsIntoState(state, sessionId, turns))
  },
  createSession: async (daemonUrl, req) => {
    const session = await createChatSession(daemonUrl, req)
    get().upsertSession(session)
    set({ activeSessionId: session.session_id })
    return session
  },
  sendTurn: async (daemonUrl, sessionId, input, expertId, cliToolId, modelId, files = [], mcpServerIDs) => {
    set({ sending: true, error: null })
    get().clearStreaming(sessionId)
    get().clearThinking(sessionId)
    get().resetThinkingTranslation(sessionId)
    get().clearTurnFeed(sessionId)
    get().setTurnMeta(sessionId, expertId ? { expert_id: expertId } : null)
    const now = Date.now()
    const lastTurn = (get().messagesBySession[sessionId] ?? []).at(-1)?.turn ?? 0
    const messageId = `local_user_${sessionId}_${now}`
    get().appendMessage(sessionId, {
      message_id: messageId,
      session_id: sessionId,
      turn: lastTurn + 1,
      role: 'user',
      content_text: input.trim() || '（仅附件）',
      attachments: buildOptimisticAttachments(sessionId, messageId, files, now),
      expert_id: expertId,
      created_at: now,
    })
    try {
      await postChatTurn(daemonUrl, sessionId, {
        input,
        expert_id: expertId,
        cli_tool_id: cliToolId,
        model_id: modelId,
        files,
        mcp_server_ids: mcpServerIDs,
      })
      await Promise.all([
        get().loadMessages(daemonUrl, sessionId),
        get().loadTurns(daemonUrl, sessionId),
        get().refreshSessions(daemonUrl),
      ])
      set((state) => ({
        sending: false,
        streamingBySession: {
          ...state.streamingBySession,
          [sessionId]: '',
        },
      }))
      get().setTurnMeta(sessionId, null)
    } catch (err: unknown) {
      await Promise.all([
        get().loadMessages(daemonUrl, sessionId).catch(() => undefined),
        get().loadTurns(daemonUrl, sessionId).catch(() => undefined),
      ])
      set((state) => ({
        sending: false,
        error: err instanceof Error ? err.message : String(err),
        streamingBySession: {
          ...state.streamingBySession,
          [sessionId]: '',
        },
      }))
      get().clearTurnFeed(sessionId)
      get().setTurnMeta(sessionId, null)
      throw err
    }
  },
  compactSession: async (daemonUrl, sessionId) => {
    await postChatCompact(daemonUrl, sessionId)
    await get().refreshSessions(daemonUrl)
  },
  forkSession: async (daemonUrl, sessionId) => {
    const forked = await postChatFork(daemonUrl, sessionId)
    get().upsertSession(forked)
    return forked
  },
  archiveSession: async (daemonUrl, sessionId) => {
    await patchChatSession(daemonUrl, sessionId, { status: 'archived' })
    await get().refreshSessions(daemonUrl)
  },
}),
    {
      name: chatRuntimeStorageKey,
      storage: createJSONStorage(() => sessionStorage),
      partialize: (state) => ({
        activeSessionId: state.activeSessionId,
      }),
    },
  ),
)
