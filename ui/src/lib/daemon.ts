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

export type Workflow = {
  workflow_id: string
  title: string
  workspace_path: string
  mode: string
  status: string
  created_at: number
  updated_at: number
  error_message?: string
  summary?: string
}

export type Node = {
  node_id: string
  workflow_id: string
  node_type: string
  expert_id: string
  title: string
  prompt: string
  status: string
  created_at: number
  updated_at: number
  last_execution_id?: string
  result_summary?: string
  result_json?: string
  error_message?: string
}

export type Edge = {
  edge_id: string
  workflow_id: string
  from_node_id: string
  to_node_id: string
  source_handle?: string
  target_handle?: string
  type: string
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

/**
 * 功能：拉取 workflow 列表（`GET /api/v1/workflows`）。
 * 参数/返回：接收 daemonUrl；返回 Workflow[]。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchWorkflows(daemonUrl: string): Promise<Workflow[]> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Workflow[]
}

/**
 * 功能：创建 workflow（`POST /api/v1/workflows`）。
 * 参数/返回：接收 daemonUrl 与创建参数；返回创建后的 Workflow。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端写入 SQLite。
 */
export async function createWorkflow(
  daemonUrl: string,
  req: { title?: string; workspace_path: string; mode?: string },
): Promise<Workflow> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Workflow
}

export type StartWorkflowResponse = {
  workflow: Workflow
  master_node: Node
  execution: Execution
}

/**
 * 功能：启动 workflow（`POST /api/v1/workflows/{id}/start`）。
 * 参数/返回：接收 daemonUrl 与 workflowId；返回 StartWorkflowResponse。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求，触发后端创建 master node + execution 并开始执行。
 */
export async function startWorkflow(
  daemonUrl: string,
  workflowId: string,
): Promise<StartWorkflowResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}/start`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as StartWorkflowResponse
}

/**
 * 功能：读取 workflow 下的 nodes（`GET /api/v1/workflows/{id}/nodes`）。
 * 参数/返回：接收 daemonUrl 与 workflowId；返回 Node[]。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchWorkflowNodes(
  daemonUrl: string,
  workflowId: string,
): Promise<Node[]> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}/nodes`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Node[]
}

/**
 * 功能：读取 workflow 下的 edges（`GET /api/v1/workflows/{id}/edges`）。
 * 参数/返回：接收 daemonUrl 与 workflowId；返回 Edge[]。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchWorkflowEdges(
  daemonUrl: string,
  workflowId: string,
): Promise<Edge[]> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}/edges`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Edge[]
}

export type ApproveWorkflowResponse = {
  workflow: Workflow
  nodes: Node[]
}

/**
 * 功能：批准 workflow 下所有 runnable nodes（manual 模式，`POST /api/v1/workflows/{id}/approve`）。
 * 参数/返回：接收 daemonUrl 与 workflowId；返回 ApproveWorkflowResponse（含被推进的 nodes）。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端写入 SQLite。
 */
export async function approveWorkflow(
  daemonUrl: string,
  workflowId: string,
): Promise<ApproveWorkflowResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}/approve`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ApproveWorkflowResponse
}

/**
 * 功能：更新 node 的 prompt/expert_id（`PATCH /api/v1/nodes/{id}`）。
 * 参数/返回：接收 daemonUrl、nodeId 与 patch；返回更新后的 Node。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端写入 SQLite。
 */
export async function patchNode(
  daemonUrl: string,
  nodeId: string,
  patch: { prompt?: string; expert_id?: string },
): Promise<Node> {
  const res = await fetch(`${daemonUrl}/api/v1/nodes/${nodeId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Node
}

export type RetryNodeResponse = {
  workflow: Workflow
  nodes: Node[]
}

/**
 * 功能：重试 node（`POST /api/v1/nodes/{id}/retry`）。
 * 参数/返回：接收 daemonUrl 与 nodeId；返回 RetryNodeResponse（含被更新的 nodes）。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端写入 SQLite；后续由调度器启动新的 execution。
 */
export async function retryNode(
  daemonUrl: string,
  nodeId: string,
): Promise<RetryNodeResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/nodes/${nodeId}/retry`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as RetryNodeResponse
}

export type CancelNodeResponse = {
  ok: boolean
  execution_id?: string
}

/**
 * 功能：取消 node 当前 running execution（`POST /api/v1/nodes/{id}/cancel`）。
 * 参数/返回：接收 daemonUrl 与 nodeId；返回 CancelNodeResponse（best-effort）。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并向后端进程发送取消信号；最终状态由 WS 推送收敛。
 */
export async function cancelNode(
  daemonUrl: string,
  nodeId: string,
): Promise<CancelNodeResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/nodes/${nodeId}/cancel`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as CancelNodeResponse
}

/**
 * 功能：更新 workflow（`PATCH /api/v1/workflows/{id}`），用于切换 mode 或修改 title/workspace。
 * 参数/返回：接收 daemonUrl、workflowId 与 patch；返回更新后的 Workflow。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端写入 SQLite。
 */
export async function patchWorkflow(
  daemonUrl: string,
  workflowId: string,
  patch: { title?: string; workspace_path?: string; mode?: string },
): Promise<Workflow> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Workflow
}

/**
 * 功能：取消 workflow（`POST /api/v1/workflows/{id}/cancel`）。
 * 参数/返回：接收 daemonUrl 与 workflowId；成功 resolve。
 * 失败场景：HTTP 非 2xx 时抛出 Error。
 * 副作用：发起 HTTP 请求并触发后端取消 running execution。
 */
export async function cancelWorkflow(
  daemonUrl: string,
  workflowId: string,
): Promise<void> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}/cancel`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
}
