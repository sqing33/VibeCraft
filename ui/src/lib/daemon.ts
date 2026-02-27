const DEFAULT_DAEMON_URL = 'http://127.0.0.1:7777'

/**
 * 功能：从环境变量读取并规范化 daemon base URL。
 * 参数/返回：无入参；返回形如 `http://127.0.0.1:7777` 的字符串。
 * 失败场景：无（缺失时回退默认值）。
 * 副作用：读取 `import.meta.env.VITE_DAEMON_URL`。
 */
export function daemonUrlFromEnv(): string {
  const raw = (import.meta.env.VITE_DAEMON_URL as string | undefined) ?? ''
  const url = raw.trim()
  if (!url) return DEFAULT_DAEMON_URL
  return url.endsWith('/') ? url.slice(0, -1) : url
}

/**
 * 功能：将 HTTP base URL 转为 WS URL（用于订阅日志事件）。
 * 参数/返回：接收 daemonUrl；返回形如 `ws://host/api/v1/ws` 的字符串。
 * 失败场景：daemonUrl 非法时会抛出 URL 解析异常。
 * 副作用：无。
 */
export function wsUrlFromDaemonUrl(daemonUrl: string): string {
  const url = new URL(daemonUrl)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.pathname = '/api/v1/ws'
  url.search = ''
  return url.toString()
}

/**
 * 功能：探活 daemon（`GET /api/v1/health`）。
 * 参数/返回：接收 daemonUrl 与可选 AbortSignal；成功 resolve，失败抛出 Error。
 * 失败场景：网络错误、非 2xx、或返回体不是 `{ ok: true }`。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchHealth(
  daemonUrl: string,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${daemonUrl}/api/v1/health`, { signal })
  if (!res.ok) {
    throw new Error(`HTTP ${res.status} ${res.statusText}`.trim())
  }
  const body = (await res.json().catch(() => null)) as unknown
  if (!body || typeof body !== 'object') {
    throw new Error('invalid JSON response')
  }
  if (!('ok' in body) || (body as { ok?: unknown }).ok !== true) {
    throw new Error('unexpected response shape')
  }
}

export type Execution = {
  execution_id: string
  status: string
  command: string
  args?: string[]
  cwd?: string
  started_at: string
  ended_at?: string
  exit_code?: number
  signal?: string
}

/**
 * 功能：启动一个 execution（默认 demo 命令）。
 * 参数/返回：daemonUrl 为必填；req 可选覆盖 command/args/cwd/env；返回 Execution。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端创建子进程与日志文件。
 */
export async function startExecution(
  daemonUrl: string,
  req?: {
    command?: string
    args?: string[]
    cwd?: string
    env?: Record<string, string>
  },
): Promise<Execution> {
  const res = await fetch(`${daemonUrl}/api/v1/executions`, {
    method: 'POST',
    headers: req ? { 'Content-Type': 'application/json' } : undefined,
    body: req ? JSON.stringify(req) : undefined,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Execution
}

/**
 * 功能：读取 execution 日志尾部（断线补齐/切换回放）。
 * 参数/返回：tailBytes 为字节数；返回纯文本（包含 ANSI）。
 * 失败场景：日志不存在或 HTTP 非 2xx 时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchExecutionLogTail(
  daemonUrl: string,
  executionId: string,
  tailBytes = 20000,
): Promise<string> {
  const url = new URL(`${daemonUrl}/api/v1/executions/${executionId}/log`)
  url.searchParams.set('tail', String(tailBytes))
  const res = await fetch(url)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return await res.text()
}

/**
 * 功能：取消指定 execution（SIGTERM→grace→SIGKILL）。
 * 参数/返回：接收 daemonUrl 与 executionId；成功 resolve，失败抛出 Error。
 * 失败场景：execution 不存在或取消失败时抛出 Error。
 * 副作用：发起 HTTP 请求并触发后端向子进程发送信号。
 */
export async function cancelExecution(
  daemonUrl: string,
  executionId: string,
): Promise<void> {
  const res = await fetch(`${daemonUrl}/api/v1/executions/${executionId}/cancel`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
}
