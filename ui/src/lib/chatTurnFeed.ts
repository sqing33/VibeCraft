export type ChatTurnEntryKind = 'progress' | 'thinking' | 'answer' | 'tool' | 'plan' | 'question' | 'system' | 'error'

export type ChatTurnEntryStatus = 'streaming' | 'created' | 'pending_approval' | 'success' | 'failed' | 'done'

export type ChatTurnEventPayload = {
  session_id: string
  user_message_id: string
  entry_id: string
  kind: ChatTurnEntryKind
  op: 'append' | 'replace' | 'upsert' | 'complete'
  status?: ChatTurnEntryStatus | string
  delta?: string
  content?: string
  meta?: Record<string, unknown>
}

export type ChatTurnFeedEntry = {
  entry_id: string
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
        kind: event.kind,
        status: normalizeStatus(event.status, 'created'),
        content: '',
      }

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
    entries,
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
): ChatTurnFeed | undefined {
  if (!current || !delta) return current
  const entries = [...current.entries]
  const idx = entries.findIndex((entry) => entry.kind === 'thinking')
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
  const thinkingIndex = entries.findIndex((entry) => entry.kind === 'thinking')
  if (thinkingIndex >= 0) {
    const entry = { ...entries[thinkingIndex] }
    if (!entry.content.trim() && opts?.thinking?.trim()) {
      entry.content = opts.thinking.trim()
    }
    if (opts?.translatedThinking?.trim()) {
      entry.meta = mergeMeta(entry.meta, {
        translated_content: opts.translatedThinking.trim(),
        translation_failed: opts.translationFailed === true,
      })
    } else if (opts?.translationFailed) {
      entry.meta = mergeMeta(entry.meta, { translation_failed: true })
    }
    entry.status = entry.status === 'failed' ? 'failed' : 'done'
    entries[thinkingIndex] = entry
  }
  entries = entries.map((entry) => ({
    ...entry,
    status: entry.status === 'failed' ? 'failed' : entry.status === 'success' ? 'success' : 'done',
  }))
  return {
    ...current,
    entries,
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
