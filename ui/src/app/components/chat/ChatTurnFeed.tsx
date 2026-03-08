import { useState } from 'react'

import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

import type { ChatTurnFeed, ChatTurnFeedEntry } from '@/lib/chatTurnFeed'

type ChatTurnFeedProps = {
  feed: ChatTurnFeed
  pending?: boolean
  identity?: string
  compact?: boolean
}

function isVisibleTimelineEntry(entry: ChatTurnFeedEntry): boolean {
  if (entry.kind !== 'tool') return true
  const command = entry.content.trim()
  return command !== '' && command !== 'command execution'
}

function entryTitle(entry: ChatTurnFeedEntry): string {
  switch (entry.kind) {
    case 'thinking':
      return '思考'
    case 'answer':
      return '回答'
    case 'tool':
      return '命令执行'
    case 'plan':
      return '计划'
    case 'question':
      return '等待输入'
    case 'progress':
      return '进度'
    case 'system':
      return '系统'
    case 'error':
      return '错误'
    default:
      return '条目'
  }
}

function cardClass(entry: ChatTurnFeedEntry): string {
  switch (entry.kind) {
    case 'answer':
      return 'border-primary/30 bg-primary/5'
    case 'thinking':
      return 'border-dashed bg-muted/35'
    case 'tool':
      return 'border-amber-200 bg-amber-50/50 dark:border-amber-900 dark:bg-amber-950/20'
    case 'plan':
      return 'border-sky-200 bg-sky-50/50 dark:border-sky-900 dark:bg-sky-950/20'
    case 'question':
      return 'border-violet-200 bg-violet-50/50 dark:border-violet-900 dark:bg-violet-950/20'
    case 'progress':
    case 'system':
      return 'border-default-200/70 bg-background/60'
    case 'error':
      return 'border-danger/40 bg-danger/5'
    default:
      return 'border-default-200/70 bg-background/60'
  }
}

function statusLabel(entry: ChatTurnFeedEntry): string {
  switch (entry.status) {
    case 'streaming':
      return '进行中'
    case 'created':
      return '已创建'
    case 'pending_approval':
      return '等待确认'
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    case 'done':
      return '完成'
    default:
      return entry.status
  }
}

function outputLineCount(text: string): number {
  if (!text.trim()) return 0
  return text.split(/\r?\n/).filter((line) => line.trim()).length
}

function toolOutputSummary(entry: ChatTurnFeedEntry): string[] {
  if (entry.kind !== 'tool') return []
  const meta = entry.meta ?? {}
  const stdout = typeof meta.stdout === 'string' ? meta.stdout.trim() : ''
  const stderr = typeof meta.stderr === 'string' ? meta.stderr.trim() : ''
  const exitCode = typeof meta.exit_code === 'number' ? meta.exit_code : null
  const summary: string[] = []
  if (exitCode !== null) {
    summary.push(`退出码 ${exitCode}`)
  }
  if (stdout) {
    summary.push(`stdout ${outputLineCount(stdout)} 行`)
  }
  if (stderr) {
    summary.push(`stderr ${outputLineCount(stderr)} 行`)
  }
  if (summary.length === 0 && entry.status === 'streaming') {
    summary.push('正在采集输出')
  }
  return summary
}

function renderToolOutput(entry: ChatTurnFeedEntry, expanded: boolean, onToggle: () => void) {
  if (entry.kind !== 'tool') return null
  const meta = entry.meta ?? {}
  const stdout = typeof meta.stdout === 'string' ? meta.stdout.trim() : ''
  const stderr = typeof meta.stderr === 'string' ? meta.stderr.trim() : ''
  const summary = toolOutputSummary(entry)
  const hasOutput = Boolean(stdout || stderr)
  return (
    <div className="mt-3 rounded-xl border border-default-200/70 bg-background/50 p-2 text-xs">
      <div className="flex items-center justify-between gap-3">
        <div className="flex flex-wrap gap-2 text-muted-foreground">
          {summary.length > 0 ? summary.map((item) => <span key={item}>{item}</span>) : <span>暂无输出</span>}
        </div>
        {hasOutput ? (
          <button
            type="button"
            className="rounded-md border px-2 py-1 text-[11px] text-muted-foreground transition hover:bg-muted hover:text-foreground"
            onClick={onToggle}
          >
            {expanded ? '收起输出' : '展开输出'}
          </button>
        ) : null}
      </div>
      {expanded ? (
        <div className="mt-2 space-y-2">
          {stdout ? (
            <div>
              <div className="mb-1 text-[11px] font-medium text-muted-foreground">stdout</div>
              <pre className="overflow-x-auto rounded-md bg-background/80 p-2 whitespace-pre-wrap break-words">{stdout}</pre>
            </div>
          ) : null}
          {stderr ? (
            <div>
              <div className="mb-1 text-[11px] font-medium text-muted-foreground">stderr</div>
              <pre className="overflow-x-auto rounded-md bg-background/80 p-2 whitespace-pre-wrap break-words text-danger">{stderr}</pre>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

function renderQuestionOptions(entry: ChatTurnFeedEntry) {
  if (entry.kind !== 'question') return null
  const meta = entry.meta ?? {}
  const questions = Array.isArray(meta.questions) ? meta.questions : []
  if (questions.length === 0) return null
  return (
    <div className="mt-2 space-y-2 text-xs text-muted-foreground">
      {questions.map((question, idx) => {
        if (!question || typeof question !== 'object') return null
        const q = question as { header?: string; question?: string; options?: { label?: string; description?: string }[] }
        return (
          <div key={idx} className="rounded-md bg-background/60 p-2">
            {q.header ? <div className="font-medium">{q.header}</div> : null}
            {q.question ? <div className="mt-1">{q.question}</div> : null}
            {Array.isArray(q.options) && q.options.length > 0 ? (
              <ul className="mt-2 space-y-1">
                {q.options.map((option, optionIdx) => (
                  <li key={optionIdx} className="list-disc ml-4">
                    <span className="font-medium">{option?.label ?? '选项'}</span>
                    {option?.description ? ` · ${option.description}` : ''}
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}

function renderThinkingVariant(entry: ChatTurnFeedEntry) {
  if (entry.kind !== 'thinking') return null
  const translated = typeof entry.meta?.translated_content === 'string' ? entry.meta.translated_content.trim() : ''
  const content = translated || entry.content
  if (!content.trim()) return <div className="text-sm text-muted-foreground">正在思考…</div>
  return (
    <div className="chat-markdown text-sm text-muted-foreground">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  )
}

function FeedEntry({ entry, compact = false }: { entry: ChatTurnFeedEntry; compact?: boolean }) {
  const [toolOutputExpanded, setToolOutputExpanded] = useState(false)

  if (entry.kind === 'progress' || entry.kind === 'system') {
    return (
      <div className={`rounded-full border px-3 py-1 text-xs text-muted-foreground ${cardClass(entry)}`}>
        {entry.content || entryTitle(entry)}
      </div>
    )
  }

  return (
    <div className={`rounded-2xl border px-3 py-3 ${cardClass(entry)}`}>
      <div className="mb-2 flex items-center justify-between gap-3 text-[11px] font-medium text-muted-foreground">
        <span>{entryTitle(entry)}</span>
        <span>{statusLabel(entry)}</span>
      </div>
      {entry.kind === 'thinking' ? renderThinkingVariant(entry) : null}
      {entry.kind === 'answer' ? (
        <div className="chat-markdown text-sm">
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{entry.content || '...'}</ReactMarkdown>
        </div>
      ) : null}
      {entry.kind !== 'thinking' && entry.kind !== 'answer' ? (
        <div className={entry.kind === 'tool' ? 'text-sm font-mono break-words' : 'chat-markdown text-sm'}>
          {entry.kind === 'tool' ? entry.content || 'command execution' : <ReactMarkdown remarkPlugins={[remarkGfm]}>{entry.content || '...'}</ReactMarkdown>}
        </div>
      ) : null}
      {renderToolOutput(entry, toolOutputExpanded, () => setToolOutputExpanded((current) => !current))}
      {renderQuestionOptions(entry)}
      {!compact && entry.kind === 'thinking' && typeof entry.meta?.translated_content === 'string' && entry.meta.translated_content ? (
        <details className="mt-2 text-xs text-muted-foreground">
          <summary className="cursor-pointer select-none">查看原始 thinking</summary>
          <div className="chat-markdown mt-2">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{entry.content}</ReactMarkdown>
          </div>
        </details>
      ) : null}
    </div>
  )
}

function AnswerContent({ entry }: { entry: ChatTurnFeedEntry }) {
  return (
    <div className="chat-markdown text-sm">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{entry.content || '...'}</ReactMarkdown>
    </div>
  )
}

/**
 * 功能：把一轮 Codex 运行时 feed 按时间线顺序展示，并让重内容默认折叠。
 * 参数/返回：接收 feed、是否 pending、模型标识与紧凑模式；返回可直接渲染的 React 节点。
 * 失败场景：feed 条目为空时显示等待占位，不抛出异常。
 * 副作用：无；仅负责 UI 渲染。
 */
export function ChatTurnFeed({ feed, pending = false, identity, compact = false }: ChatTurnFeedProps) {
  const answerEntry = feed.entries.find((entry) => entry.kind === 'answer' && entry.content.trim())
  const timelineEntries = feed.entries.filter((entry) => entry.kind !== 'answer' && isVisibleTimelineEntry(entry))

  return (
    <div className="space-y-3">
      <div className="text-[11px] font-medium text-muted-foreground">
        AI{identity ? ` · ${identity}` : ''} {pending ? '处理中' : '本轮时间线'}
      </div>
      {timelineEntries.length > 0 ? (
        <div className="space-y-2">
          {timelineEntries.map((entry) => (
            <FeedEntry key={entry.entry_id} entry={entry} compact={compact} />
          ))}
        </div>
      ) : null}
      {!compact && answerEntry ? (
        <div className="border-t border-default-200/70 pt-3">
          <AnswerContent entry={answerEntry} />
        </div>
      ) : null}
      {pending && timelineEntries.length === 0 && !answerEntry ? (
        <div className="rounded-2xl border border-dashed bg-background/80 px-4 py-3 text-sm text-muted-foreground shadow-sm">
          正在等待模型输出…
        </div>
      ) : null}
    </div>
  )
}
