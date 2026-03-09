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

export type CLITool = {
  id: string
  label: string
  protocol_family: string
  protocol_families?: string[]
  cli_family: string
  default_model_id?: string
  command_path?: string
  enabled: boolean
  iflow_auth_mode?: 'browser' | 'api_key'
  iflow_base_url?: string
  iflow_models?: string[]
  iflow_default_model?: string
  iflow_has_key?: boolean
  iflow_masked_key?: string
  iflow_browser_authenticated?: boolean
  iflow_browser_model?: string
}

export type PutCLITool = CLITool & {
  iflow_api_key?: string
}

/**
 * 功能：返回 CLI tool 支持的协议族列表，兼容旧版单值 `protocol_family`。
 * 参数/返回：接收一个 CLI tool；返回去重后的 provider 列表。
 * 失败场景：无，缺失字段时返回空数组。
 * 副作用：无。
 */
export function cliToolProtocolFamilies(
  tool?: Pick<CLITool, 'protocol_family' | 'protocol_families'> | null,
): string[] {
  const seen = new Set<string>()
  const next: string[] = []
  for (const raw of [...(tool?.protocol_families ?? []), tool?.protocol_family ?? '']) {
    const normalized = raw.trim()
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    next.push(normalized)
  }
  return next
}

/**
 * 功能：返回 CLI tool 的主协议族，用于兼容旧 UI 文案与默认展示。
 * 参数/返回：接收一个 CLI tool；优先返回 `protocol_families[0]`，否则回退 `protocol_family`。
 * 失败场景：无，未配置时返回空字符串。
 * 副作用：无。
 */
export function cliToolPrimaryProtocolFamily(
  tool?: Pick<CLITool, 'protocol_family' | 'protocol_families'> | null,
): string {
  return cliToolProtocolFamilies(tool)[0] ?? ''
}

/**
 * 功能：判断某个 CLI tool 是否支持指定 provider。
 * 参数/返回：接收 tool 与 provider；返回布尔值。
 * 失败场景：无，缺失 provider 时返回 false。
 * 副作用：无。
 */
export function cliToolSupportsProvider(
  tool: Pick<CLITool, 'protocol_family' | 'protocol_families'> | null | undefined,
  provider?: string | null,
): boolean {
  const normalized = provider?.trim() ?? ''
  if (!normalized) return false
  return cliToolProtocolFamilies(tool).includes(normalized)
}

export type CLIToolSettings = {
  tools: CLITool[]
  models: LLMModelProfile[]
}

export type PutCLIToolSettingsRequest = {
  tools: PutCLITool[]
}

export type IFLOWBrowserAuthSession = {
  session_id: string
  status: string
  auth_url?: string
  last_output?: string
  error?: string
  can_submit_code: boolean
  authenticated: boolean
  command_path?: string
  started_at: number
  updated_at: number
}

export async function fetchCLIToolSettings(daemonUrl: string): Promise<CLIToolSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/cli-tools`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as CLIToolSettings
}

export async function putCLIToolSettings(daemonUrl: string, req: PutCLIToolSettingsRequest): Promise<CLIToolSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/cli-tools`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as CLIToolSettings
}

export async function startIFLOWBrowserAuth(
  daemonUrl: string,
  req?: { command_path?: string },
): Promise<IFLOWBrowserAuthSession> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/cli-tools/iflow/browser-auth`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req ?? {}),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as IFLOWBrowserAuthSession
}

export async function fetchIFLOWBrowserAuth(
  daemonUrl: string,
  sessionId: string,
): Promise<IFLOWBrowserAuthSession> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/cli-tools/iflow/browser-auth/${sessionId}`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as IFLOWBrowserAuthSession
}

export async function submitIFLOWBrowserAuthCode(
  daemonUrl: string,
  sessionId: string,
  req: { authorization_code: string },
): Promise<IFLOWBrowserAuthSession> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/cli-tools/iflow/browser-auth/${sessionId}/code`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as IFLOWBrowserAuthSession
}

export async function cancelIFLOWBrowserAuth(
  daemonUrl: string,
  sessionId: string,
): Promise<IFLOWBrowserAuthSession> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/cli-tools/iflow/browser-auth/${sessionId}/cancel`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as IFLOWBrowserAuthSession
}

export type MCPServerSetting = {
  id: string
  raw_json: string
  default_enabled_cli_tool_ids: string[]
  config?: Record<string, unknown>
}

export type MCPSettings = {
  servers: MCPServerSetting[]
  tools: CLITool[]
}

export type PutMCPSettingsRequest = {
  servers: MCPServerSetting[]
}

export async function fetchMCPSettings(daemonUrl: string): Promise<MCPSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/mcp`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as MCPSettings
}

export async function putMCPSettings(
  daemonUrl: string,
  req: PutMCPSettingsRequest,
): Promise<MCPSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/mcp`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as MCPSettings
}

export type SkillBindingSetting = {
  id: string
  description?: string
  path?: string
  source?: string
  enabled: boolean
}

export type SkillSettings = {
  skills: SkillBindingSetting[]
}

export type PutSkillSettingsRequest = {
  skills: SkillBindingSetting[]
}

export async function fetchSkillSettings(daemonUrl: string): Promise<SkillSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/skills`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as SkillSettings
}

export async function putSkillSettings(
  daemonUrl: string,
  req: PutSkillSettingsRequest,
): Promise<SkillSettings> {
  const res = await fetch(`${daemonUrl}/api/v1/settings/skills`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as SkillSettings
}

export async function postTranslateText(daemonUrl: string, text: string): Promise<{ translated: string }> {
  const res = await fetch(`${daemonUrl}/api/v1/translate/text`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text }),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => null) as { error?: string } | null
    throw new Error(data?.error ?? `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as { translated: string }
}

export async function postSkillInstallArchive(
  daemonUrl: string,
  archive: File,
): Promise<SkillSettings> {
  const form = new FormData()
  form.set('archive', archive)
  const res = await fetch(`${daemonUrl}/api/v1/settings/skills/install`, {
    method: 'POST',
    body: form,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as SkillSettings
}

export async function postSkillInstallDirectory(
  daemonUrl: string,
  files: Array<{ file: File; relativePath: string }>,
): Promise<SkillSettings> {
  const form = new FormData()
  for (const item of files) {
    form.append('files', item.file, item.relativePath)
    form.append('paths', item.relativePath)
  }
  const res = await fetch(`${daemonUrl}/api/v1/settings/skills/install`, {
    method: 'POST',
    body: form,
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as SkillSettings
}

export type LLMSettings = {
  sources: LLMSource[]
  models: LLMModelProfile[]
}

export type BasicSettings = {
  thinking_translation?: ThinkingTranslationSettings
}

export type ThinkingTranslationSettings = {
  model_id: string
  target_model_ids: string[]
}

export type PutBasicSettingsRequest = {
  thinking_translation?: {
    model_id: string
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
  cli_tool_id?: string
  model_id?: string
  cli_session_id?: string
  reasoning_effort?: string
  mcp_server_ids?: string[]
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
export type ChatTurnTimelineItem = {
  entry_id: string
  seq: number
  kind: string
  status: string
  content_text: string
  meta?: Record<string, unknown>
  created_at: number
  updated_at: number
}

export type ChatTurnTimeline = {
  turn_id: string
  session_id: string
  user_message_id: string
  assistant_message_id?: string
  turn: number
  status: string
  expert_id?: string
  provider?: string
  model?: string
  model_input?: string
  context_mode?: string
  thinking_translation_applied?: boolean
  thinking_translation_failed?: boolean
  token_in?: number
  token_out?: number
  cached_input_tokens?: number
  created_at: number
  updated_at: number
  completed_at?: number
  items: ChatTurnTimelineItem[]
}


export async function createChatSession(
  daemonUrl: string,
  req: { title?: string; expert_id?: string; cli_tool_id?: string; model_id?: string; reasoning_effort?: string; workspace_path?: string; mcp_server_ids?: string[] },
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
export async function fetchChatTurns(
  daemonUrl: string,
  sessionId: string,
  limit = 200,
): Promise<ChatTurnTimeline[]> {
  const url = new URL(`${daemonUrl}/api/v1/chat/sessions/${sessionId}/turns`)
  url.searchParams.set('limit', String(limit))
  const res = await fetch(url)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as ChatTurnTimeline[]
}


export async function postChatTurn(
  daemonUrl: string,
  sessionId: string,
  req: { input?: string; expert_id?: string; cli_tool_id?: string; model_id?: string; reasoning_effort?: string; files?: File[]; mcp_server_ids?: string[] },
): Promise<ChatTurnResult> {
  const hasFiles = Array.isArray(req.files) && req.files.length > 0
  const init: RequestInit = { method: 'POST' }
  if (hasFiles) {
    const form = new FormData()
    if (typeof req.input === 'string') form.set('input', req.input)
    if (typeof req.expert_id === 'string' && req.expert_id.trim()) {
      form.set('expert_id', req.expert_id)
    }
    if (typeof req.cli_tool_id === 'string' && req.cli_tool_id.trim()) {
      form.set('cli_tool_id', req.cli_tool_id)
    }
    if (typeof req.model_id === 'string' && req.model_id.trim()) {
      form.set('model_id', req.model_id)
    }
    if (typeof req.reasoning_effort === 'string' && req.reasoning_effort.trim()) {
      form.set('reasoning_effort', req.reasoning_effort)
    }
    if (Array.isArray(req.mcp_server_ids)) {
      form.set('mcp_server_ids', JSON.stringify(req.mcp_server_ids))
    }
    for (const file of req.files ?? []) {
      form.append('files', file)
    }
    init.body = form
  } else {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify({
      input: req.input ?? '',
      expert_id: req.expert_id,
      cli_tool_id: req.cli_tool_id,
      model_id: req.model_id,
      reasoning_effort: req.reasoning_effort,
      mcp_server_ids: req.mcp_server_ids,
    })
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
  req: { title?: string; status?: string; mcp_server_ids?: string[] },
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

export type RepoLibraryDepth = 'standard' | 'deep'

export type RepoLibraryAnalysisRequest = {
  repo_url: string
  ref: string
  features: string[]
  depth?: RepoLibraryDepth
  language?: string
  analyzer_mode?: string
  cli_tool_id?: string
  model_id?: string
}

export type RepoLibraryAnalysisRun = {
  analysis_id: string
  repository_id: string
  snapshot_id?: string
  execution_id?: string
  repo_url: string
  ref?: string
  resolved_ref?: string
  commit_sha?: string
  features: string[]
  depth?: RepoLibraryDepth
  language?: string
  analyzer_mode?: string
  cli_tool_id?: string
  model_id?: string
  runtime_kind?: string
  chat_session_id?: string
  chat_user_message_id?: string
  chat_assistant_message_id?: string
  status: string
  failure_message?: string
  storage_path?: string
  report_path?: string
  report_url?: string
  created_at: number
  updated_at: number
  finished_at?: number
}

export type RepoLibrarySnapshot = {
  snapshot_id: string
  repository_id: string
  ref?: string
  resolved_ref?: string
  commit_sha?: string
  storage_path?: string
  report_path?: string
  report_url?: string
  report_excerpt?: string
  report_markdown?: string
  created_at: number
  updated_at?: number
}

export type RepoLibraryRepository = {
  repository_id: string
  host?: string
  owner?: string
  name?: string
  full_name?: string
  repo_url: string
  description?: string
  default_branch?: string
  latest_ref?: string
  latest_commit_sha?: string
  snapshot_count?: number
  card_count?: number
  created_at?: number
  updated_at: number
}

export type RepoLibraryRepositorySummary = RepoLibraryRepository & {
  latest_snapshot?: RepoLibrarySnapshot | null
  latest_analysis?: RepoLibraryAnalysisRun | null
}

export type RepoLibraryRepositoryDetail = {
  repository: RepoLibraryRepository
  latest_snapshot?: RepoLibrarySnapshot | null
  latest_analysis?: RepoLibraryAnalysisRun | null
  analysis_runs?: RepoLibraryAnalysisRun[]
  report_excerpt?: string
  report_markdown?: string
}

export type RepoLibraryCard = {
  card_id: string
  repository_id: string
  snapshot_id?: string
  analysis_id?: string
  card_type: string
  title: string
  summary?: string
  detail?: string
  confidence?: number
  tags?: string[]
  created_at?: number
  updated_at?: number
}

export type RepoLibraryCardEvidence = {
  evidence_id: string
  card_id?: string
  repository_id?: string
  snapshot_id?: string
  source_path: string
  start_line?: number
  end_line?: number
  label?: string
  excerpt?: string
}

export type RepoLibraryCreateAnalysisResponse = {
  analysis: RepoLibraryAnalysisRun
  repository?: RepoLibraryRepository
  snapshot?: RepoLibrarySnapshot
}

export type RepoLibrarySyncChatResponse = {
  ok?: boolean
  analysis?: RepoLibraryAnalysisRun
  detail?: RepoLibraryRepositoryDetail
}

export type RepoLibrarySearchRequest = {
  query: string
  repository_ids?: string[]
  limit?: number
}

export type RepoLibrarySearchResult = {
  result_id?: string
  repository_id: string
  snapshot_id?: string
  card_id?: string
  score?: number
  title?: string
  summary?: string
  rationale?: string
  repository?: RepoLibraryRepositorySummary
  snapshot?: RepoLibrarySnapshot
  card?: RepoLibraryCard
  evidence_preview?: RepoLibraryCardEvidence[]
}

export type RepoLibrarySearchResponse = {
  query_id?: string
  results: RepoLibrarySearchResult[]
}

function normalizeRepoLibraryRepository(body: unknown): RepoLibraryRepository {
  const record = (body ?? {}) as Record<string, unknown>
  const repositoryId = String(record.repository_id ?? record.id ?? '')
  const owner = typeof record.owner === 'string' ? record.owner : undefined
  const name = typeof record.name === 'string' ? record.name : typeof record.repo === 'string' ? record.repo : undefined
  const fullName =
    typeof record.full_name === 'string'
      ? record.full_name
      : owner && name
        ? `${owner}/${name}`
        : undefined
  return {
    repository_id: repositoryId,
    owner,
    name,
    full_name: fullName,
    repo_url: String(record.repo_url ?? ''),
    description: typeof record.description === 'string' ? record.description : undefined,
    default_branch:
      typeof record.default_branch === 'string' ? record.default_branch : undefined,
    latest_ref:
      typeof record.latest_resolved_ref === 'string'
        ? record.latest_resolved_ref
        : typeof record.latest_ref === 'string'
          ? record.latest_ref
          : undefined,
    latest_commit_sha:
      typeof record.latest_commit_sha === 'string' ? record.latest_commit_sha : undefined,
    snapshot_count:
      typeof record.snapshot_count === 'number' ? record.snapshot_count : undefined,
    card_count:
      typeof record.cards_count === 'number'
        ? record.cards_count
        : typeof record.card_count === 'number'
          ? record.card_count
          : undefined,
    created_at: typeof record.created_at === 'number' ? record.created_at : undefined,
    updated_at: typeof record.updated_at === 'number' ? record.updated_at : 0,
  }
}

function normalizeRepoLibrarySnapshot(body: unknown): RepoLibrarySnapshot {
  const record = (body ?? {}) as Record<string, unknown>
  return {
    snapshot_id: String(record.snapshot_id ?? record.id ?? ''),
    repository_id: String(record.repository_id ?? record.repo_source_id ?? ''),
    ref:
      typeof record.ref === 'string'
        ? record.ref
        : typeof record.requested_ref === 'string'
          ? record.requested_ref
          : undefined,
    resolved_ref: typeof record.resolved_ref === 'string' ? record.resolved_ref : undefined,
    commit_sha: typeof record.commit_sha === 'string' ? record.commit_sha : undefined,
    storage_path: typeof record.storage_path === 'string' ? record.storage_path : undefined,
    report_path: typeof record.report_path === 'string' ? record.report_path : undefined,
    report_url: typeof record.report_url === 'string' ? record.report_url : undefined,
    report_excerpt:
      typeof record.report_excerpt === 'string' ? record.report_excerpt : undefined,
    report_markdown:
      typeof record.report_markdown === 'string' ? record.report_markdown : undefined,
    created_at: typeof record.created_at === 'number' ? record.created_at : 0,
    updated_at: typeof record.updated_at === 'number' ? record.updated_at : undefined,
  }
}

function normalizeRepoLibraryAnalysisRun(body: unknown): RepoLibraryAnalysisRun {
  const record = (body ?? {}) as Record<string, unknown>
  return {
    analysis_id: String(record.analysis_id ?? record.analysis_run_id ?? record.id ?? ''),
    repository_id: String(record.repository_id ?? record.repo_source_id ?? ''),
    snapshot_id: typeof record.snapshot_id === 'string' ? record.snapshot_id : typeof record.repo_snapshot_id === 'string' ? record.repo_snapshot_id : undefined,
    execution_id: typeof record.execution_id === 'string' ? record.execution_id : undefined,
    repo_url: typeof record.repo_url === 'string' ? record.repo_url : '',
    ref: typeof record.ref === 'string' ? record.ref : typeof record.requested_ref === 'string' ? record.requested_ref : undefined,
    resolved_ref: typeof record.resolved_ref === 'string' ? record.resolved_ref : undefined,
    commit_sha: typeof record.commit_sha === 'string' ? record.commit_sha : undefined,
    features: Array.isArray(record.features) ? record.features.filter((item): item is string => typeof item === 'string') : [],
    depth:
      record.depth === 'deep' || record.depth === 'standard'
        ? record.depth
        : undefined,
    language: typeof record.language === 'string' ? record.language : undefined,
    analyzer_mode: typeof record.analyzer_mode === 'string' ? record.analyzer_mode : typeof record.agent_mode === 'string' ? record.agent_mode : undefined,
    cli_tool_id: typeof record.cli_tool_id === 'string' ? record.cli_tool_id : typeof record.tool_id === 'string' ? record.tool_id : undefined,
    model_id: typeof record.model_id === 'string' ? record.model_id : undefined,
    runtime_kind: typeof record.runtime_kind === 'string' ? record.runtime_kind : undefined,
    chat_session_id: typeof record.chat_session_id === 'string' ? record.chat_session_id : typeof record.chatSessionId === 'string' ? record.chatSessionId : undefined,
    chat_user_message_id: typeof record.chat_user_message_id === 'string' ? record.chat_user_message_id : typeof record.chatUserMessageId === 'string' ? record.chatUserMessageId : undefined,
    chat_assistant_message_id: typeof record.chat_assistant_message_id === 'string' ? record.chat_assistant_message_id : typeof record.chatAssistantMessageId === 'string' ? record.chatAssistantMessageId : undefined,
    status: typeof record.status === 'string' ? record.status : 'unknown',
    failure_message: typeof record.failure_message === 'string' ? record.failure_message : typeof record.error_message === 'string' ? record.error_message : undefined,
    storage_path: typeof record.storage_path === 'string' ? record.storage_path : undefined,
    report_path: typeof record.report_path === 'string' ? record.report_path : undefined,
    report_url: typeof record.report_url === 'string' ? record.report_url : undefined,
    created_at: typeof record.created_at === 'number' ? record.created_at : 0,
    updated_at: typeof record.updated_at === 'number' ? record.updated_at : 0,
    finished_at: typeof record.finished_at === 'number' ? record.finished_at : typeof record.ended_at === 'number' ? record.ended_at : undefined,
  }
}

function normalizeRepoLibraryCard(body: unknown): RepoLibraryCard {
  const record = (body ?? {}) as Record<string, unknown>
  let confidence: number | undefined
  if (typeof record.confidence === 'number') {
    confidence = record.confidence
  } else if (typeof record.confidence === 'string') {
    if (record.confidence === 'high') confidence = 0.9
    if (record.confidence === 'medium') confidence = 0.6
    if (record.confidence === 'low') confidence = 0.3
  }
  return {
    card_id: String(record.card_id ?? record.id ?? ''),
    repository_id: String(record.repository_id ?? record.repo_source_id ?? ''),
    snapshot_id: typeof record.snapshot_id === 'string' ? record.snapshot_id : typeof record.repo_snapshot_id === 'string' ? record.repo_snapshot_id : undefined,
    analysis_id: typeof record.analysis_id === 'string' ? record.analysis_id : typeof record.analysis_run_id === 'string' ? record.analysis_run_id : undefined,
    card_type: String(record.card_type ?? ''),
    title: String(record.title ?? ''),
    summary: typeof record.summary === 'string' ? record.summary : undefined,
    detail: typeof record.detail === 'string' ? record.detail : typeof record.mechanism === 'string' ? record.mechanism : undefined,
    confidence,
    tags: Array.isArray(record.tags) ? record.tags.filter((item): item is string => typeof item === 'string') : undefined,
    created_at: typeof record.created_at === 'number' ? record.created_at : undefined,
    updated_at: typeof record.updated_at === 'number' ? record.updated_at : undefined,
  }
}

function normalizeRepoLibraryCardEvidence(body: unknown): RepoLibraryCardEvidence {
  const record = (body ?? {}) as Record<string, unknown>
  return {
    evidence_id: String(record.evidence_id ?? record.id ?? ''),
    card_id: typeof record.card_id === 'string' ? record.card_id : undefined,
    repository_id: typeof record.repository_id === 'string' ? record.repository_id : typeof record.repo_source_id === 'string' ? record.repo_source_id : undefined,
    snapshot_id: typeof record.snapshot_id === 'string' ? record.snapshot_id : typeof record.repo_snapshot_id === 'string' ? record.repo_snapshot_id : undefined,
    source_path: String(record.source_path ?? record.path ?? ''),
    start_line: typeof record.start_line === 'number' ? record.start_line : typeof record.line === 'number' ? record.line : undefined,
    end_line: typeof record.end_line === 'number' ? record.end_line : typeof record.line === 'number' ? record.line : undefined,
    label: typeof record.label === 'string' ? record.label : typeof record.dimension === 'string' ? record.dimension : undefined,
    excerpt: typeof record.excerpt === 'string' ? record.excerpt : typeof record.snippet === 'string' ? record.snippet : undefined,
  }
}

function normalizeRepoLibraryRepositorySummary(body: unknown): RepoLibraryRepositorySummary {
  const record = (body ?? {}) as Record<string, unknown>
  const repository = normalizeRepoLibraryRepository(body)
  return {
    ...repository,
    latest_snapshot:
      record.latest_snapshot && typeof record.latest_snapshot === 'object'
        ? normalizeRepoLibrarySnapshot(record.latest_snapshot)
        : record.latest_snapshot_id || record.latest_commit_sha || record.latest_resolved_ref
          ? normalizeRepoLibrarySnapshot({
              snapshot_id: record.latest_snapshot_id,
              repository_id: repository.repository_id,
              commit_sha: record.latest_commit_sha,
              resolved_ref: record.latest_resolved_ref,
            })
          : null,
    latest_analysis:
      record.latest_analysis && typeof record.latest_analysis === 'object'
        ? normalizeRepoLibraryAnalysisRun(record.latest_analysis)
        : record.latest_analysis_run_id || record.latest_analysis_status
          ? normalizeRepoLibraryAnalysisRun({
              analysis_run_id: record.latest_analysis_run_id,
              repository_id: repository.repository_id,
              status: record.latest_analysis_status,
              updated_at: record.latest_analysis_updated_at,
            })
          : null,
  }
}

function normalizeRepoLibraryRepositoryDetail(body: unknown): RepoLibraryRepositoryDetail {
  const record = (body ?? {}) as Record<string, unknown>
  const repository = normalizeRepoLibraryRepository(record.repository ?? body)
  const snapshots = Array.isArray(record.snapshots)
    ? record.snapshots.map((item) => normalizeRepoLibrarySnapshot(item))
    : []
  const analysisRuns = Array.isArray(record.analysis_runs)
    ? record.analysis_runs.map((item) => normalizeRepoLibraryAnalysisRun(item))
    : Array.isArray(record.runs)
      ? record.runs.map((item) => normalizeRepoLibraryAnalysisRun(item))
      : []
  return {
    repository,
    latest_snapshot: snapshots[0] ?? null,
    latest_analysis: analysisRuns[0] ?? null,
    analysis_runs: analysisRuns,
    report_excerpt: typeof record.report_excerpt === 'string' ? record.report_excerpt : undefined,
    report_markdown: typeof record.report_markdown === 'string' ? record.report_markdown : undefined,
  }
}

function normalizeRepoLibrarySearchResult(body: unknown): RepoLibrarySearchResult {
  const record = (body ?? {}) as Record<string, unknown>
  const repositoryPayload = record.repository && typeof record.repository === 'object' ? record.repository : null
  const snapshotPayload = record.snapshot && typeof record.snapshot === 'object' ? record.snapshot : null
  return {
    result_id: typeof record.result_id === 'string' ? record.result_id : typeof record.chunk_id === 'string' ? record.chunk_id : undefined,
    repository_id: String((repositoryPayload as Record<string, unknown> | null)?.repo_key ?? (repositoryPayload as Record<string, unknown> | null)?.repository_id ?? record.repository_id ?? ''),
    snapshot_id: typeof record.snapshot_id === 'string' ? record.snapshot_id : typeof (snapshotPayload as Record<string, unknown> | null)?.snapshot_id === 'string' ? String((snapshotPayload as Record<string, unknown>).snapshot_id) : undefined,
    card_id: typeof record.card_id === 'string' ? record.card_id : undefined,
    score: typeof record.score === 'number' ? record.score : undefined,
    title: typeof record.title === 'string' ? record.title : typeof (record.chunk as Record<string, unknown> | undefined)?.section_title === 'string' ? String((record.chunk as Record<string, unknown>).section_title) : undefined,
    summary: typeof record.summary === 'string' ? record.summary : typeof record.text_excerpt === 'string' ? record.text_excerpt : undefined,
    rationale: typeof record.rationale === 'string' ? record.rationale : undefined,
    repository: repositoryPayload ? normalizeRepoLibraryRepositorySummary(repositoryPayload) : undefined,
    snapshot: snapshotPayload ? normalizeRepoLibrarySnapshot(snapshotPayload) : undefined,
    evidence_preview: Array.isArray(record.evidence_preview)
      ? record.evidence_preview.map((item) => normalizeRepoLibraryCardEvidence(item))
      : Array.isArray(record.evidence_refs)
        ? (record.evidence_refs as unknown[]).map((item, index) => normalizeRepoLibraryCardEvidence({ evidence_id: `ref-${index}`, source_path: String(item) }))
        : undefined,
  }
}

function readRepoLibraryList<T>(body: unknown, keys: string[]): T[] {
  if (Array.isArray(body)) return body as T[]
  if (!body || typeof body !== 'object') {
    throw new Error('unexpected response shape')
  }
  for (const key of keys) {
    const value = (body as Record<string, unknown>)[key]
    if (Array.isArray(value)) return value as T[]
  }
  throw new Error('unexpected response shape')
}

function normalizeRepoLibraryCreateAnalysisResponse(
  body: unknown,
): RepoLibraryCreateAnalysisResponse {
  if (!body || typeof body !== 'object') {
    throw new Error('unexpected response shape')
  }
  const record = body as Record<string, unknown>
  if (record.analysis && typeof record.analysis === 'object') {
    return {
      analysis: normalizeRepoLibraryAnalysisRun(record.analysis),
      repository:
        record.repository && typeof record.repository === 'object'
          ? normalizeRepoLibraryRepository(record.repository)
          : undefined,
      snapshot:
        record.snapshot && typeof record.snapshot === 'object'
          ? normalizeRepoLibrarySnapshot(record.snapshot)
          : undefined,
    }
  }
  if (record.run && typeof record.run === 'object') {
    return {
      analysis: normalizeRepoLibraryAnalysisRun(record.run),
      repository:
        record.repository && typeof record.repository === 'object'
          ? normalizeRepoLibraryRepository(record.repository)
          : undefined,
      snapshot:
        record.snapshot && typeof record.snapshot === 'object'
          ? normalizeRepoLibrarySnapshot(record.snapshot)
          : undefined,
    }
  }
  return { analysis: normalizeRepoLibraryAnalysisRun(body) }
}

function normalizeRepoLibrarySyncChatResponse(
  body: unknown,
): RepoLibrarySyncChatResponse {
  if (!body || typeof body !== 'object') {
    return {}
  }
  const record = body as Record<string, unknown>
  const detail =
    record.detail && typeof record.detail === 'object'
      ? normalizeRepoLibraryRepositoryDetail(record.detail)
      : record.repository && typeof record.repository === 'object'
        ? normalizeRepoLibraryRepositoryDetail(body)
        : undefined
  const analysis =
    record.analysis && typeof record.analysis === 'object'
      ? normalizeRepoLibraryAnalysisRun(record.analysis)
      : record.run && typeof record.run === 'object'
        ? normalizeRepoLibraryAnalysisRun(record.run)
        : undefined
  const ok =
    typeof record.ok === 'boolean'
      ? record.ok
      : typeof record.success === 'boolean'
        ? record.success
        : undefined
  return { ok, analysis, detail }
}

/**
 * 功能：提交 Repo Library 仓库分析任务（`POST /api/v1/repo-library/analyses`）。
 * 参数/返回：接收 daemonUrl 与分析请求体；返回创建后的 analysis 元数据，以及可能附带的 repository/snapshot。
 * 失败场景：HTTP 非 2xx、校验失败或返回体缺少基本结构时抛出 Error。
 * 副作用：发起 HTTP 请求并触发后端创建异步分析运行。
 */
export async function createRepoLibraryAnalysis(
  daemonUrl: string,
  req: RepoLibraryAnalysisRequest,
): Promise<RepoLibraryCreateAnalysisResponse> {
  const payload = {
    repo_url: req.repo_url,
    ref: req.ref,
    features: req.features,
    depth: req.depth ?? 'standard',
    language: req.language === 'en' ? 'en' : 'zh',
    agent_mode: req.analyzer_mode === 'compact' ? 'single' : 'single',
    cli_tool_id: req.cli_tool_id?.trim() || undefined,
    model_id: req.model_id?.trim() || undefined,
  }
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/analyses`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return normalizeRepoLibraryCreateAnalysisResponse(await res.json())
}

/**
 * 功能：同步指定分析关联 Chat 的最新回复（`POST /api/v1/repo-library/analyses/{id}/sync-chat`）。
 * 参数/返回：接收 daemonUrl 与 analysisId；返回可选的 analysis/detail 刷新载荷或 success 标记。
 * 失败场景：HTTP 非 2xx 时抛出 Error。
 * 副作用：发起 HTTP 请求，并触发后端将最新 Chat 结果回写到 Repo Library。
 */
export async function syncRepoLibraryAnalysisChat(
  daemonUrl: string,
  analysisId: string,
): Promise<RepoLibrarySyncChatResponse> {
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/analyses/${encodeURIComponent(analysisId)}/sync-chat`, {
    method: 'POST',
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return normalizeRepoLibrarySyncChatResponse(await res.json())
}

/**
 * 功能：读取 Repo Library 仓库摘要列表（`GET /api/v1/repo-library/repositories`）。
 * 参数/返回：接收 daemonUrl；返回按最近活动排序的仓库摘要数组。
 * 失败场景：HTTP 非 2xx 或返回体不是数组/已知列表包装时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchRepoLibraryRepositories(
  daemonUrl: string,
): Promise<RepoLibraryRepositorySummary[]> {
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/repositories`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return readRepoLibraryList<unknown>(await res.json(), [
    'items',
    'repositories',
  ]).map((item) => normalizeRepoLibraryRepositorySummary(item))
}

/**
 * 功能：读取单个 Repo Library 仓库详情（`GET /api/v1/repo-library/repositories/{id}`）。
 * 参数/返回：接收 daemonUrl 与 repositoryId；返回仓库详情、最近快照与分析运行概览。
 * 失败场景：HTTP 非 2xx 或返回体缺少仓库详情结构时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchRepoLibraryRepository(
  daemonUrl: string,
  repositoryId: string,
): Promise<RepoLibraryRepositoryDetail> {
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/repositories/${repositoryId}`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  const body = (await res.json()) as unknown
  if (!body || typeof body !== 'object') {
    throw new Error('unexpected response shape')
  }
  const record = body as Record<string, unknown>
  if (record.repository && typeof record.repository === 'object') {
    return normalizeRepoLibraryRepositoryDetail(body)
  }
  return { repository: normalizeRepoLibraryRepository(body) }
}

/**
 * 功能：读取指定仓库下的快照列表（`GET /api/v1/repo-library/repositories/{id}/snapshots`）。
 * 参数/返回：接收 daemonUrl 与 repositoryId；返回快照数组。
 * 失败场景：HTTP 非 2xx 或返回体不是数组/已知列表包装时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchRepoLibrarySnapshots(
  daemonUrl: string,
  repositoryId: string,
): Promise<RepoLibrarySnapshot[]> {
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/repositories/${repositoryId}/snapshots`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return readRepoLibraryList<unknown>(await res.json(), ['items', 'snapshots']).map((item) => normalizeRepoLibrarySnapshot(item))
}

/**
 * 功能：按仓库或快照读取 Repo Library 卡片列表（`GET /api/v1/repo-library/cards`）。
 * 参数/返回：接收 daemonUrl 与可选过滤参数；返回知识卡片数组。
 * 失败场景：HTTP 非 2xx 或返回体不是数组/已知列表包装时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchRepoLibraryCards(
  daemonUrl: string,
  opts?: { repository_id?: string; snapshot_id?: string; limit?: number },
): Promise<RepoLibraryCard[]> {
  const url = new URL(`${daemonUrl}/api/v1/repo-library/cards`)
  if (opts?.repository_id) url.searchParams.set('repository_id', opts.repository_id)
  if (opts?.snapshot_id) url.searchParams.set('snapshot_id', opts.snapshot_id)
  if (typeof opts?.limit === 'number') url.searchParams.set('limit', String(opts.limit))
  const res = await fetch(url)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return readRepoLibraryList<unknown>(await res.json(), ['items', 'cards']).map((item) => normalizeRepoLibraryCard(item))
}

/**
 * 功能：读取单张 Repo Library 卡片详情（`GET /api/v1/repo-library/cards/{id}`）。
 * 参数/返回：接收 daemonUrl 与 cardId；返回卡片完整结构。
 * 失败场景：HTTP 非 2xx 或返回体缺少卡片结构时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchRepoLibraryCard(
  daemonUrl: string,
  cardId: string,
): Promise<RepoLibraryCard> {
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/cards/${cardId}`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return normalizeRepoLibraryCard(await res.json())
}

/**
 * 功能：读取单张卡片的 evidence 列表（`GET /api/v1/repo-library/cards/{id}/evidence`）。
 * 参数/返回：接收 daemonUrl 与 cardId；返回 evidence 数组。
 * 失败场景：HTTP 非 2xx 或返回体不是数组/已知列表包装时抛出 Error。
 * 副作用：发起 HTTP 请求。
 */
export async function fetchRepoLibraryCardEvidence(
  daemonUrl: string,
  cardId: string,
): Promise<RepoLibraryCardEvidence[]> {
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/cards/${cardId}/evidence`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return readRepoLibraryList<unknown>(await res.json(), [
    'items',
    'evidence',
  ]).map((item) => normalizeRepoLibraryCardEvidence(item))
}

export async function fetchRepoLibrarySnapshotReport(
  daemonUrl: string,
  snapshotId: string,
): Promise<{ snapshot_id: string; report_markdown: string }> {
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/snapshots/${snapshotId}/report`)
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  return (await res.json()) as { snapshot_id: string; report_markdown: string }
}

/**
 * 功能：执行 Repo Library 语义模式搜索（`POST /api/v1/repo-library/search`）。
 * 参数/返回：接收 daemonUrl 与搜索请求体；返回结果数组及可选 query_id。
 * 失败场景：HTTP 非 2xx 或返回体缺少结果结构时抛出 Error。
 * 副作用：发起 HTTP 请求，并触发后端向量/结构化检索。
 */
export async function searchRepoLibrary(
  daemonUrl: string,
  req: RepoLibrarySearchRequest,
): Promise<RepoLibrarySearchResponse> {
  const payload = {
    query: req.query,
    repo_filters: req.repository_ids,
    mode: 'semi',
    top_k: typeof req.limit === 'number' ? req.limit : 20,
  }
  const res = await fetch(`${daemonUrl}/api/v1/repo-library/search`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(text || `HTTP ${res.status} ${res.statusText}`.trim())
  }
  const body = (await res.json()) as unknown
  if (Array.isArray(body)) {
    return { results: body as RepoLibrarySearchResult[] }
  }
  if (!body || typeof body !== 'object') {
    throw new Error('unexpected response shape')
  }
  const record = body as Record<string, unknown>
  if (Array.isArray(record.results)) {
    return {
      query_id: typeof record.query_id === 'string' ? record.query_id : undefined,
      results: (record.results as unknown[]).map((item) => normalizeRepoLibrarySearchResult(item)),
    }
  }
  return {
    query_id: typeof record.query_id === 'string' ? record.query_id : undefined,
    results: readRepoLibraryList<unknown>(body, ['items']).map((item) => normalizeRepoLibrarySearchResult(item)),
  }
}
