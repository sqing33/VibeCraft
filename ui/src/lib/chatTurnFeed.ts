import type { ChatTurnTimeline } from '@/lib/daemon'

export type ChatTurnEntryKind = 'progress' | 'thinking' | 'answer' | 'tool' | 'plan' | 'question' | 'system' | 'error'

export type ChatTurnEntryStatus = 'streaming' | 'created' | 'pending_approval' | 'success' | 'failed' | 'done'

export type ChatTurnEventPayload = {
  session_id: string
  user_message_id: string
  entry_id: string
  seq?: number
  kind: ChatTurnEntryKind
  op: 'append' | 'replace' | 'upsert' | 'complete'
  status?: ChatTurnEntryStatus | string
  delta?: string
  content?: string
  meta?: Record<string, unknown>
}

export type ChatTurnFeedEntry = {
  entry_id: string
  seq: number
  kind: ChatTurnEntryKind
  status: string
  content: string
  meta?: Record<string, unknown>
}

export type ChatTurnFeed = {
  session_id: string
  user_message_id: string
  entries: ChatTurnFeedEntry[]
  turnMeta?: {
    expert_id?: string
    provider?: string
    model?: string
  } | null
  completed_assistant_message_id?: string
}

function mergeMeta(
  prev?: Record<string, unknown>,
  next?: Record<string, unknown>,
): Record<string, unknown> | undefined {
  if (!prev && !next) return undefined
  return { ...(prev ?? {}), ...(next ?? {}) }
}

function normalizeStatus(status: string | undefined, fallback: string): string {
  const value = (status ?? '').trim()
  return value || fallback
}

function normalizeSeq(seq: number | undefined, fallback: number): number {
  return typeof seq === 'number' && Number.isFinite(seq) && seq > 0 ? seq : fallback
}

function nextFeedSeq(entries: ChatTurnFeedEntry[]): number {
  return entries.reduce((max, entry) => Math.max(max, entry.seq), 0) + 1
}

function sortEntries(entries: ChatTurnFeedEntry[]): ChatTurnFeedEntry[] {
  return [...entries].sort((left, right) => left.seq - right.seq)
}

function findThinkingEntryIndex(entries: ChatTurnFeedEntry[], entryId?: string): number {
  if (entryId) {
    const exactIdx = entries.findIndex((entry) => entry.entry_id === entryId && entry.kind === 'thinking')
    if (exactIdx >= 0) return exactIdx
  }
  for (let idx = entries.length - 1; idx >= 0; idx -= 1) {
    if (entries[idx]?.kind === 'thinking') {
      return idx
    }
  }
  return -1
}

/**
 * 功能：确保当前会话存在一份与用户消息绑定的运行时 feed。
 * 参数/返回：接收当前 feed、sessionId、userMessageId 与可选元信息；返回可继续增量写入的 feed。
 * 失败场景：入参为空时不会抛错，而是按默认结构返回空 feed。
 * 副作用：无；仅创建或复用内存对象。
 */
export function ensureTurnFeed(
  current: ChatTurnFeed | undefined,
  sessionId: string,
  userMessageId: string,
  turnMeta?: ChatTurnFeed['turnMeta'],
): ChatTurnFeed {
  if (current && current.user_message_id === userMessageId) {
    return turnMeta !== undefined ? { ...current, turnMeta } : current
  }
  return {
    session_id: sessionId,
    user_message_id: userMessageId,
    entries: [],
    turnMeta,
  }
}

/**
 * 功能：把后端发来的结构化 turn 事件合并到前端 feed 中。
 * 参数/返回：接收当前 feed 与单条事件，返回合并后的 feed。
 * 失败场景：未知 op 会保留已有内容；不会抛出运行时异常。
 * 副作用：无；调用方负责把返回值写回状态容器。
 */
export function applyTurnFeedEvent(
  current: ChatTurnFeed | undefined,
  event: ChatTurnEventPayload,
): ChatTurnFeed {
  const feed = ensureTurnFeed(current, event.session_id, event.user_message_id, current?.turnMeta)
  const entries = [...feed.entries]
  const idx = entries.findIndex((entry) => entry.entry_id === event.entry_id)
  const prev = idx >= 0 ? entries[idx] : undefined
  const entry: ChatTurnFeedEntry = prev
    ? { ...prev }
    : {
        entry_id: event.entry_id,
        seq: normalizeSeq(event.seq, nextFeedSeq(entries)),
        kind: event.kind,
        status: normalizeStatus(event.status, 'created'),
        content: '',
      }

  entry.seq = prev?.seq ?? normalizeSeq(event.seq, nextFeedSeq(entries))
  entry.kind = event.kind
  entry.status = normalizeStatus(event.status, entry.status)
  if (event.op === 'append') {
    entry.content += event.delta ?? event.content ?? ''
  } else if (event.op === 'replace') {
    entry.content = event.content ?? event.delta ?? ''
  } else if (event.op === 'upsert') {
    if (typeof event.content === 'string') {
      entry.content = event.content
    } else if (typeof event.delta === 'string' && event.delta) {
      entry.content += event.delta
    }
  } else if (event.op === 'complete') {
    if (typeof event.content === 'string') {
      entry.content = event.content
    }
  }
  entry.meta = mergeMeta(entry.meta, event.meta)
  if (idx >= 0) {
    entries[idx] = entry
  } else {
    entries.push(entry)
  }
  return {
    ...feed,
    entries: sortEntries(entries),
  }
}

/**
 * 功能：把思考过程翻译增量附着到 thinking 条目的元数据上。
 * 参数/返回：接收当前 feed 与翻译 delta，返回追加翻译后的 feed。
 * 失败场景：feed 不存在或缺少 thinking 条目时直接原样返回。
 * 副作用：无；仅返回新的浅拷贝对象。
 */
export function applyThinkingTranslationDelta(
  current: ChatTurnFeed | undefined,
  delta: string,
  entryId?: string,
): ChatTurnFeed | undefined {
  if (!current || !delta) return current
  const entries = [...current.entries]
  const idx = findThinkingEntryIndex(entries, entryId)
  if (idx < 0) return current
  const entry = { ...entries[idx] }
  const meta = { ...(entry.meta ?? {}) }
  const prev = typeof meta.translated_content === 'string' ? meta.translated_content : ''
  meta.translated_content = `${prev}${delta}`
  entry.meta = meta
  entries[idx] = entry
  return { ...current, entries }
}

/**
 * 功能：在 assistant 消息落库后收敛本轮 feed，并补齐 thinking 最终态。
 * 参数/返回：接收当前 feed、assistantMessageId 与思考补全选项，返回完成态 feed。
 * 失败场景：feed 不存在时返回原值；翻译失败时仅写入失败标记。
 * 副作用：无；调用方负责把完成态 feed 挂到消息上。
 */
export function finalizeTurnFeed(
  current: ChatTurnFeed | undefined,
  assistantMessageId: string,
  opts?: {
    thinking?: string
    translatedThinking?: string
    translationFailed?: boolean
  },
): ChatTurnFeed | undefined {
  if (!current) return current
  let entries = [...current.entries]
  const thinkingIndexes = entries
    .map((entry, idx) => (entry.kind === 'thinking' ? idx : -1))
    .filter((idx) => idx >= 0)
  if (thinkingIndexes.length === 0 && opts?.thinking?.trim()) {
    entries.push({
      entry_id: 'thinking:1',
      seq: nextFeedSeq(entries),
      kind: 'thinking',
      status: 'done',
      content: opts.thinking.trim(),
      meta: opts?.translatedThinking?.trim()
        ? {
            translated_content: opts.translatedThinking.trim(),
            translation_failed: opts.translationFailed === true,
          }
        : opts?.translationFailed
          ? { translation_failed: true }
          : undefined,
    })
  } else {
    thinkingIndexes.forEach((thinkingIndex) => {
      const entry = { ...entries[thinkingIndex] }
      if (!entry.content.trim() && thinkingIndexes.length === 1 && opts?.thinking?.trim()) {
        entry.content = opts.thinking.trim()
      }
      if (
        thinkingIndexes.length === 1 &&
        opts?.translatedThinking?.trim() &&
        typeof entry.meta?.translated_content !== 'string'
      ) {
        entry.meta = mergeMeta(entry.meta, {
          translated_content: opts.translatedThinking.trim(),
          translation_failed: opts.translationFailed === true,
        })
      } else if (opts?.translationFailed) {
        entry.meta = mergeMeta(entry.meta, { translation_failed: true })
      }
      entry.status = entry.status === 'failed' ? 'failed' : 'done'
      entries[thinkingIndex] = entry
    })
  }
  entries = entries.map((entry) => ({
    ...entry,
    status: entry.status === 'failed' ? 'failed' : entry.status === 'success' ? 'success' : 'done',
  }))
  return {
    ...current,
    entries: sortEntries(entries),
    completed_assistant_message_id: assistantMessageId,
  }
}

/**
 * 功能：判断 feed 是否已有任意可展示条目。
 * 参数/返回：接收 feed，返回布尔值。
 * 失败场景：feed 为空时返回 false。
 * 副作用：无。
 */
export function hasFeedEntries(feed: ChatTurnFeed | undefined): boolean {
  return Boolean(feed && feed.entries.length > 0)
}

/**
 * 功能：判断 feed 中是否已经出现可见的 answer 内容。
 * 参数/返回：接收 feed，返回布尔值。
 * 失败场景：feed 为空或 answer 为空字符串时返回 false。
 * 副作用：无。
 */
export function hasActiveAnswer(feed: ChatTurnFeed | undefined): boolean {
  return Boolean(feed?.entries.some((entry) => entry.kind === 'answer' && entry.content.trim()))
}

/**
 * 功能：提取当前 feed 中 answer 条目的文本内容。
 * 参数/返回：接收 feed，返回 answer 文本；不存在时返回空字符串。
 * 失败场景：feed 为空时返回空字符串。
 * 副作用：无。
 */
export function feedAnswerText(feed: ChatTurnFeed | undefined): string {
  return feed?.entries.find((entry) => entry.kind === 'answer')?.content ?? ''
}

function normalizePersistedEntryStatus(turnStatus: string, entryStatus: string): string {
  const turn = turnStatus.trim().toLowerCase()
  const entry = entryStatus.trim() || 'created'
  if (turn === 'completed') {
    if (entry === 'failed' || entry === 'success' || entry === 'pending_approval') return entry
    return 'done'
  }
  if (turn === 'failed' && entry === 'created') {
    return 'failed'
  }
  return entry
}

/**
 * 功能：把后端持久化的 turn 快照投影为前端可直接渲染的 ChatTurnFeed。
 * 参数/返回：接收单个 ChatTurnTimeline；返回排序完成的 ChatTurnFeed。
 * 失败场景：条目为空时仍返回空 feed，不抛异常。
 * 副作用：无；仅做数据映射。
 */
export function buildTurnFeedFromTimeline(turn: ChatTurnTimeline): ChatTurnFeed {
  return {
    session_id: turn.session_id,
    user_message_id: turn.user_message_id,
    completed_assistant_message_id: turn.assistant_message_id,
    turnMeta: {
      expert_id: turn.expert_id?.trim() || undefined,
      provider: turn.provider?.trim() || undefined,
      model: turn.model?.trim() || undefined,
    },
    entries: [...(turn.items ?? [])]
      .map((item) => ({
        entry_id: item.entry_id,
        seq: item.seq,
        kind: item.kind as ChatTurnEntryKind,
        status: normalizePersistedEntryStatus(turn.status, item.status),
        content: item.content_text,
        meta: item.meta,
      }))
      .sort((left, right) => left.seq - right.seq),
  }
}
