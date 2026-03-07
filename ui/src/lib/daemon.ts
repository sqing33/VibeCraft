const DEFAULT_DAEMON_URL_DEV = 'http://127.0.0.1:7777'

function normalizeBaseUrl(raw: string): string {
  const url = raw.trim()
  if (!url) return ''
  return url.endsWith('/') ? url.slice(0, -1) : url
}

/**
 * 功能：从环境变量读取并规范化 daemon base URL。
 * 参数/返回：无入参；返回形如 `http://127.0.0.1:7777` 的字符串。
 * 失败场景：无（缺失时：dev 回退默认值；prod 回退同源 origin）。
 * 副作用：读取 `import.meta.env.VITE_DAEMON_URL`。
 */
export function daemonUrlFromEnv(): string {
  const raw = (import.meta.env.VITE_DAEMON_URL as string | undefined) ?? ''
  const fromEnv = normalizeBaseUrl(raw)
  if (fromEnv) return fromEnv

  // Web 版本（daemon 静态托管 ui/dist）默认使用同源，避免端口变化导致前端连错。
  if (!import.meta.env.DEV) {
    const origin =
      typeof window !== 'undefined' ? normalizeBaseUrl(window.location.origin) : ''
    if (origin) return origin
  }

  // dev server（Vite）下默认连接本地 daemon。
  return DEFAULT_DAEMON_URL_DEV
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

export type DaemonInfo = {
  version: {
    commit: string
    built_at?: string
  }
  paths: {
    config_path: string
    data_dir: string
    logs_dir: string
    state_db_path: string
  }
  now_ms: number
}

/**
 * 功能：读取 daemon info（`GET /api/v1/info`），用于展示版本与数据目录路径。
 * 参数/返回：接收 daemonUrl；返回 DaemonInfo。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchInfo(daemonUrl: string): Promise<DaemonInfo> {
  const res = await fetch(`${daemonUrl}/api/v1/info`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as DaemonInfo
}

export type PublicExpert = {
  id: string
  label: string
  provider: string
  model: string
  runtime_kind?: string
  cli_family?: string
  helper_only?: boolean
  timeout_ms: number
}

/**
 * 功能：读取 experts 列表（`GET /api/v1/experts`），用于 UI 下拉选择。
 * 参数/返回：接收 daemonUrl；返回 PublicExpert[]。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchExperts(daemonUrl: string): Promise<PublicExpert[]> {
  const res = await fetch(`${daemonUrl}/api/v1/experts`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as PublicExpert[]
}

export type LLMSettings = {
  sources: LLMSource[]
  models: LLMModelProfile[]
}

export type BasicSettings = {
  thinking_translation?: ThinkingTranslationSettings
}

export type ThinkingTranslationSettings = {
  source_id: string
  model: string
  target_model_ids: string[]
}

export type PutBasicSettingsRequest = {
  thinking_translation?: {
    source_id: string
    model: string
    target_model_ids: string[]
  }
}

export type LLMSource = {
  id: string
  label: string
  provider: string
  base_url?: string
  has_key: boolean
  masked_key?: string
}

export type LLMModelProfile = {
  id: string
  label: string
  provider: string
  model: string
  source_id: string
}

export type PutLLMSettingsRequest = {
  sources: Array<{
    id: string
    label: string
    provider: string
    base_url?: string
    api_key?: string
  }>
  models: Array<{
    id: string
    label: string
    provider: string
    model: string
    source_id: string
  }>
}

/**
 * 功能：读取 LLM settings（`GET /api/v1/settings/llm`），用于 UI 设置页编辑 Sources/Models。
 * 参数/返回：接收 daemonUrl；返回 LLMSettings。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchLLMSettings(daemonUrl: string): Promise<LLMSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/llm`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as LLMSettings
}

/**
 * 功能：保存 LLM settings（`PUT /api/v1/settings/llm`）。
 * 参数/返回：接收 daemonUrl 与整包 settings payload；返回保存后的 LLMSettings（masked）。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并触发后端写盘与 expert registry 热更新。
 */
export async function putLLMSettings(
  daemonUrl: string,
  req: PutLLMSettingsRequest,
): Promise<LLMSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/llm`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as LLMSettings
}

export async function fetchBasicSettings(daemonUrl: string): Promise<BasicSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/basic`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as BasicSettings
}

export async function putBasicSettings(
  daemonUrl: string,
  req: PutBasicSettingsRequest,
): Promise<BasicSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/basic`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as BasicSettings
}

export type LLMTestRequest = {
  provider: string
  model: string
  base_url?: string
  source_id?: string
  api_key?: string
  prompt?: string
}

export type LLMTestResponse = {
  ok: boolean
  output: string
  latency_ms: number
}

/**
 * 功能：对指定模型配置做一次短 SDK 测试（`POST /api/v1/settings/llm/test`）。
 * 参数/返回：接收 daemonUrl 与测试参数；返回 LLMTestResponse。
 * 失败场景：HTTP 非 2xx 或后端执行失败时抛出 Error。
 * 副作用：发起 HTTP 请求，并触发一次真实 SDK 调用（可能计费）。
 */
export async function postLLMTest(
  daemonUrl: string,
  req: LLMTestRequest,
): Promise<LLMTestResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/llm/test`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as LLMTestResponse
}


export type ExpertSettingsItem = {
  id: string
  label: string
  description?: string
  category?: string
  avatar?: string
  managed_source?: string
  primary_model_id?: string
  secondary_model_id?: string
  fallback_on?: string[]
  enabled_skills?: string[]
  provider?: string
  model?: string
  runtime_kind?: string
  cli_family?: string
  helper_only?: boolean
  system_prompt?: string
  prompt_template?: string
  output_format?: string
  max_output_tokens?: number
  temperature?: number
  timeout_ms?: number
  builder_expert_id?: string
  builder_session_id?: string
  builder_snapshot_id?: string
  generated_by?: string
  generated_at?: number
  updated_at?: number
  enabled: boolean
  editable: boolean
}

export type SkillCatalogItem = {
  id: string
  description?: string
  path?: string
}

export type ExpertSettings = {
  experts: ExpertSettingsItem[]
  skills: SkillCatalogItem[]
  builder_experts: Array<{
    id: string
    label: string
    provider: string
    model: string
    description?: string
  }>
}

export type PutExpertSettingsRequest = {
  experts: Array<{
    id: string
    label: string
    description?: string
    category?: string
    avatar?: string
    primary_model_id?: string
    secondary_model_id?: string
    fallback_on?: string[]
    enabled_skills?: string[]
    system_prompt?: string
    prompt_template?: string
    output_format?: string
    max_output_tokens?: number
    temperature?: number
    timeout_ms?: number
    builder_expert_id?: string
    builder_session_id?: string
    builder_snapshot_id?: string
    generated_by?: string
    generated_at?: number
    updated_at?: number
    enabled: boolean
  }>
}

export async function fetchExpertSettings(
  daemonUrl: string,
): Promise<ExpertSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/experts`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ExpertSettings
}

export async function putExpertSettings(
  daemonUrl: string,
  req: PutExpertSettingsRequest,
): Promise<ExpertSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/experts`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ExpertSettings
}

export type ExpertBuilderMessage = {
  role: string
  content: string
}

export type ExpertGenerateRequest = {
  builder_expert_id?: string
  builder_model_id?: string
  messages: ExpertBuilderMessage[]
}

export type ExpertGenerateResponse = {
  assistant_message: string
  draft: ExpertSettingsItem
  warnings?: string[]
  raw_json?: string
}

export type ExpertBuilderSession = {
  id: string
  title: string
  target_expert_id?: string
  builder_model_id: string
  status: string
  latest_snapshot_id?: string
  created_at: number
  updated_at: number
}

export type ExpertBuilderMessageItem = {
  id: string
  session_id: string
  role: string
  content_text: string
  created_at: number
}

export type ExpertBuilderSnapshot = {
  id: string
  session_id: string
  version: number
  assistant_message: string
  draft: ExpertSettingsItem
  raw_json?: string
  warnings?: string[]
  created_at: number
}

export type ExpertBuilderSessionDetail = {
  session: ExpertBuilderSession
  messages: ExpertBuilderMessageItem[]
  snapshots: ExpertBuilderSnapshot[]
}

export async function fetchExpertBuilderSessions(
  daemonUrl: string,
  params?: { targetExpertId?: string; limit?: number },
): Promise<{ sessions: ExpertBuilderSession[] }> {
  const url = new URL(`${daemonUrl}/api/v1/settings/experts/sessions`)
  if (params?.targetExpertId) url.searchParams.set('target_expert_id', params.targetExpertId)
  if (params?.limit) url.searchParams.set('limit', String(params.limit))
  const res = await fetch(url)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as { sessions: ExpertBuilderSession[] }
}

export async function postExpertBuilderSession(
  daemonUrl: string,
  req: { title?: string; target_expert_id?: string; builder_model_id: string },
): Promise<{ session: ExpertBuilderSession }> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/experts/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as { session: ExpertBuilderSession }
}

export async function fetchExpertBuilderSession(
  daemonUrl: string,
  sessionId: string,
): Promise<ExpertBuilderSessionDetail> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/experts/sessions/${sessionId}`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ExpertBuilderSessionDetail
}

export async function postExpertBuilderMessage(
  daemonUrl: string,
  sessionId: string,
  req: { content: string },
): Promise<ExpertBuilderSessionDetail> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/experts/sessions/${sessionId}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ExpertBuilderSessionDetail
}

export async function postExpertBuilderPublish(
  daemonUrl: string,
  sessionId: string,
  req?: { snapshot_id?: string; expert_id?: string },
): Promise<{ session: ExpertBuilderSession; published_expert: ExpertSettingsItem; messages: ExpertBuilderMessageItem[]; snapshots: ExpertBuilderSnapshot[] }> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/experts/sessions/${sessionId}/publish`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: req ? JSON.stringify(req) : undefined,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as { session: ExpertBuilderSession; published_expert: ExpertSettingsItem; messages: ExpertBuilderMessageItem[]; snapshots: ExpertBuilderSnapshot[] }
}

export async function postExpertGeneration(
  daemonUrl: string,
  req: ExpertGenerateRequest,
): Promise<ExpertGenerateResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/experts/generate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ExpertGenerateResponse
}

export type ChatSession = {
  session_id: string
  title: string
  expert_id: string
  provider: string
  model: string
  workspace_path: string
  status: string
  summary?: string
  created_at: number
  updated_at: number
  last_turn: number
}

export type ChatAttachment = {
  attachment_id: string
  session_id: string
  message_id: string
  kind: string
  file_name: string
  mime_type: string
  size_bytes: number
  created_at: number
}

export type ChatMessage = {
  message_id: string
  session_id: string
  turn: number
  role: string
  content_text: string
  attachments?: ChatAttachment[]
  expert_id?: string
  provider?: string
  model?: string
  token_in?: number
  token_out?: number
  provider_message_id?: string
  created_at: number
}

export type ChatTurnResult = {
  user_message: ChatMessage
  assistant_message: ChatMessage
  reasoning_text?: string
  translated_reasoning_text?: string
  model_input?: string
  context_mode?: string
  cached_input_tokens?: number
  thinking_translation_applied?: boolean
  thinking_translation_failed?: boolean
}

export async function createChatSession(
  daemonUrl: string,
  req: { title?: string; expert_id?: string; workspace_path?: string },
): Promise<ChatSession> {
  const res = await fetch(`${daemonUrl}/api/v1/chat/sessions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ChatSession
}

export async function fetchChatSessions(
  daemonUrl: string,
  limit = 100,
): Promise<ChatSession[]> {
  const url = new URL(`${daemonUrl}/api/v1/chat/sessions`)
  url.searchParams.set('limit', String(limit))
  const res = await fetch(url)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ChatSession[]
}

export async function fetchChatMessages(
  daemonUrl: string,
  sessionId: string,
  limit = 200,
): Promise<ChatMessage[]> {
  const url = new URL(`${daemonUrl}/api/v1/chat/sessions/${sessionId}/messages`)
  url.searchParams.set('limit', String(limit))
  const res = await fetch(url)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ChatMessage[]
}

export async function postChatTurn(
  daemonUrl: string,
  sessionId: string,
  req: { input?: string; expert_id?: string; files?: File[] },
): Promise<ChatTurnResult> {
  const hasFiles = Array.isArray(req.files) && req.files.length > 0
  const init: RequestInit = { method: 'POST' }
  if (hasFiles) {
    const form = new FormData()
    if (typeof req.input === 'string') form.set('input', req.input)
    if (typeof req.expert_id === 'string' && req.expert_id.trim()) {
      form.set('expert_id', req.expert_id)
    }
    for (const file of req.files ?? []) {
      form.append('files', file)
    }
    init.body = form
  } else {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify({ input: req.input ?? '', expert_id: req.expert_id })
  }
  const res = await fetch(`${daemonUrl}/api/v1/chat/sessions/${sessionId}/turns`, init)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ChatTurnResult
}


export function chatAttachmentContentUrl(
  daemonUrl: string,
  sessionId: string,
  attachmentId: string,
): string {
  return `${normalizeBaseUrl(daemonUrl)}/api/v1/chat/sessions/${encodeURIComponent(sessionId)}/attachments/${encodeURIComponent(attachmentId)}/content`
}

export async function postChatCompact(
  daemonUrl: string,
  sessionId: string,
): Promise<{ session: ChatSession; compaction?: unknown }> {
  const res = await fetch(`${daemonUrl}/api/v1/chat/sessions/${sessionId}/compact`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as { session: ChatSession; compaction?: unknown }
}

export async function postChatFork(
  daemonUrl: string,
  sessionId: string,
  req?: { title?: string },
): Promise<ChatSession> {
  const res = await fetch(`${daemonUrl}/api/v1/chat/sessions/${sessionId}/fork`, {
    method: 'POST',
    headers: req ? { 'Content-Type': 'application/json' } : undefined,
    body: req ? JSON.stringify(req) : undefined,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ChatSession
}

export async function patchChatSession(
  daemonUrl: string,
  sessionId: string,
  req: { title?: string; status?: string },
): Promise<ChatSession> {
  const res = await fetch(`${daemonUrl}/api/v1/chat/sessions/${sessionId}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ChatSession
}

export type Execution = {
  execution_id: string
  workflow_id?: string
  node_id?: string
  orchestration_id?: string
  round_id?: string
  agent_run_id?: string
  status: string
  command: string
  args?: string[]
  cwd?: string
  started_at: string
  ended_at?: string
  exit_code?: number
  signal?: string
}

export type Orchestration = {
  orchestration_id: string
  title: string
  goal: string
  workspace_path: string
  status: string
  current_round: number
  created_at: number
  updated_at: number
  running_agent_runs_count?: number
  error_message?: string
  summary?: string
}

export type OrchestrationRound = {
  round_id: string
  orchestration_id: string
  round_index: number
  goal: string
  status: string
  created_at: number
  updated_at: number
  summary?: string
  synthesis_step_id?: string
}

export type AgentRun = {
  agent_run_id: string
  orchestration_id: string
  round_id: string
  role: string
  title: string
  goal: string
  expert_id: string
  intent: string
  workspace_mode: string
  workspace_path: string
  branch_name?: string
  base_ref?: string
  worktree_path?: string
  status: string
  created_at: number
  updated_at: number
  last_execution_id?: string
  result_summary?: string
  error_message?: string
  modified_code: boolean
}

export type SynthesisStep = {
  synthesis_step_id: string
  orchestration_id: string
  round_id: string
  decision: string
  summary: string
  created_at: number
  updated_at: number
}

export type OrchestrationArtifact = {
  artifact_id: string
  orchestration_id: string
  round_id?: string
  agent_run_id?: string
  synthesis_step_id?: string
  kind: string
  title: string
  summary?: string
  payload_json?: string
  created_at: number
}

export type OrchestrationDetail = {
  orchestration: Orchestration
  rounds: OrchestrationRound[]
  agent_runs: AgentRun[]
  synthesis_steps: SynthesisStep[]
  artifacts: OrchestrationArtifact[]
}

export type Workflow = {
  workflow_id: string
  title: string
  workspace_path: string
  mode: string
  status: string
  created_at: number
  updated_at: number
  running_nodes_count?: number
  error_message?: string
  summary?: string
}

/**
 * 功能：创建一条 orchestration（`POST /api/v1/orchestrations`）。
 * 参数/返回：接收 daemonUrl 与自然语言 goal/workspace；返回 OrchestrationDetail。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端创建 orchestration 与首轮 agent runs。
 */
export async function createOrchestration(
  daemonUrl: string,
  req: { title?: string; goal: string; workspace_path: string },
): Promise<OrchestrationDetail> {
  const res = await fetch(`${daemonUrl}/api/v1/orchestrations`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as OrchestrationDetail
}

/**
 * 功能：读取 orchestration 列表（`GET /api/v1/orchestrations`）。
 * 参数/返回：接收 daemonUrl；返回 Orchestration[]。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchOrchestrations(
  daemonUrl: string,
  limit = 50,
): Promise<Orchestration[]> {
  const url = new URL(`${daemonUrl}/api/v1/orchestrations`)
  url.searchParams.set('limit', String(limit))
  const res = await fetch(url)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Orchestration[]
}

/**
 * 功能：读取 orchestration 详情（`GET /api/v1/orchestrations/{id}`）。
 * 参数/返回：接收 daemonUrl 与 orchestrationId；返回 OrchestrationDetail。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchOrchestrationDetail(
  daemonUrl: string,
  orchestrationId: string,
): Promise<OrchestrationDetail> {
  const res = await fetch(`${daemonUrl}/api/v1/orchestrations/${orchestrationId}`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as OrchestrationDetail
}

/**
 * 功能：取消 orchestration（`POST /api/v1/orchestrations/{id}/cancel`）。
 * 参数/返回：接收 daemonUrl 与 orchestrationId；返回最新 Orchestration。
 * 失败场景：HTTP 非 2xx 时抛出 Error。
 * 副作用：发起 HTTP 请求并触发后端取消运行中的 agent runs。
 */
export async function cancelOrchestration(
  daemonUrl: string,
  orchestrationId: string,
): Promise<Orchestration> {
  const res = await fetch(`${daemonUrl}/api/v1/orchestrations/${orchestrationId}/cancel`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Orchestration
}

/**
 * 功能：继续 orchestration（`POST /api/v1/orchestrations/{id}/continue`）。
 * 参数/返回：接收 daemonUrl 与 orchestrationId；返回最新 OrchestrationDetail。
 * 失败场景：HTTP 非 2xx 时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端创建下一轮 round/agent runs。
 */
export async function continueOrchestration(
  daemonUrl: string,
  orchestrationId: string,
): Promise<OrchestrationDetail> {
  const res = await fetch(`${daemonUrl}/api/v1/orchestrations/${orchestrationId}/continue`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as OrchestrationDetail
}

/**
 * 功能：重试一个失败的 agent run（`POST /api/v1/agent-runs/{id}/retry`）。
 * 参数/返回：接收 daemonUrl 与 agentRunId；返回更新后的 orchestration/round/agent_run。
 * 失败场景：HTTP 非 2xx 时抛出 Error。
 * 副作用：发起 HTTP 请求并在后端将 agent run 重新排队。
 */
export async function retryAgentRun(
  daemonUrl: string,
  agentRunId: string,
): Promise<{ orchestration: Orchestration; round: OrchestrationRound; agent_run: AgentRun }> {
  const res = await fetch(`${daemonUrl}/api/v1/agent-runs/${agentRunId}/retry`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as { orchestration: Orchestration; round: OrchestrationRound; agent_run: AgentRun }
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
 * 功能：读取 workflow 详情（`GET /api/v1/workflows/{id}`）。
 * 参数/返回：接收 daemonUrl 与 workflowId；返回 Workflow。
 * 失败场景：HTTP 非 2xx 或返回体非预期时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchWorkflow(
  daemonUrl: string,
  workflowId: string,
): Promise<Workflow> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as Workflow
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
  req?: { prompt?: string; expert_id?: string },
): Promise<StartWorkflowResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/workflows/${workflowId}/start`, {
    method: 'POST',
    headers: req ? { 'Content-Type': 'application/json' } : undefined,
    body: req ? JSON.stringify(req) : undefined,
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
