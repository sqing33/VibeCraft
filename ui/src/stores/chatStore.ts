import { create } from 'zustand'

import {
  createChatSession,
  fetchChatMessages,
  fetchChatSessions,
  patchChatSession,
  postChatCompact,
  postChatFork,
  postChatTurn,
  type ChatMessage,
  type ChatSession,
} from '@/lib/daemon'

export type ChatStore = {
  sessions: ChatSession[]
  activeSessionId: string | null
  messagesBySession: Record<string, ChatMessage[]>
  streamingBySession: Record<string, string>
  thinkingBySession: Record<string, string>
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
  setThinking: (sessionId: string, thinking: string) => void
  clearStreaming: (sessionId: string) => void
  clearThinking: (sessionId: string) => void
  refreshSessions: (daemonUrl: string) => Promise<void>
  loadMessages: (daemonUrl: string, sessionId: string) => Promise<void>
  createSession: (
    daemonUrl: string,
    req: { title?: string; expert_id?: string; workspace_path?: string },
  ) => Promise<ChatSession>
  sendTurn: (daemonUrl: string, sessionId: string, input: string) => Promise<void>
  compactSession: (daemonUrl: string, sessionId: string) => Promise<void>
  forkSession: (daemonUrl: string, sessionId: string) => Promise<ChatSession>
  archiveSession: (daemonUrl: string, sessionId: string) => Promise<void>
}

export const useChatStore = create<ChatStore>((set, get) => ({
  sessions: [],
  activeSessionId: null,
  messagesBySession: {},
  streamingBySession: {},
  thinkingBySession: {},
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
    set((state) => ({
      messagesBySession: {
        ...state.messagesBySession,
        [sessionId]: messages,
      },
    })),
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
  setThinking: (sessionId, thinking) =>
    set((state) => ({
      thinkingBySession: {
        ...state.thinkingBySession,
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
  refreshSessions: async (daemonUrl) => {
    set({ loading: true, error: null })
    try {
      const sessions = await fetchChatSessions(daemonUrl)
      set((state) => ({
        sessions,
        loading: false,
        activeSessionId:
          state.activeSessionId && sessions.some((s) => s.session_id === state.activeSessionId)
            ? state.activeSessionId
            : sessions[0]?.session_id ?? null,
      }))
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
  createSession: async (daemonUrl, req) => {
    const session = await createChatSession(daemonUrl, req)
    get().upsertSession(session)
    set({ activeSessionId: session.session_id })
    return session
  },
  sendTurn: async (daemonUrl, sessionId, input) => {
    set({ sending: true, error: null })
    get().clearStreaming(sessionId)
    get().clearThinking(sessionId)
    const now = Date.now()
    const lastTurn = (get().messagesBySession[sessionId] ?? []).at(-1)?.turn ?? 0
    get().appendMessage(sessionId, {
      message_id: `local_user_${sessionId}_${now}`,
      session_id: sessionId,
      turn: lastTurn + 1,
      role: 'user',
      content_text: input,
      created_at: now,
    })
    try {
      await postChatTurn(daemonUrl, sessionId, { input })
      await get().loadMessages(daemonUrl, sessionId)
      await get().refreshSessions(daemonUrl)
      set((state) => ({
        sending: false,
        streamingBySession: {
          ...state.streamingBySession,
          [sessionId]: '',
        },
      }))
    } catch (err: unknown) {
      await get().loadMessages(daemonUrl, sessionId).catch(() => undefined)
      set((state) => ({
        sending: false,
        error: err instanceof Error ? err.message : String(err),
        streamingBySession: {
          ...state.streamingBySession,
          [sessionId]: '',
        },
      }))
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
}))
