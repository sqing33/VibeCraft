export type WsEnvelope = {
  type: string
  ts: number
  workflow_id?: string
  node_id?: string
  orchestration_id?: string
  round_id?: string
  agent_run_id?: string
  execution_id?: string
  payload?: unknown
}

function coerceWsEnvelope(value: unknown): WsEnvelope | null {
  if (!value || typeof value !== 'object') return null
  const obj = value as { type?: unknown }
  if (typeof obj.type !== 'string') return null
  return value as WsEnvelope
}

export function parseWsEnvelopes(raw: string): WsEnvelope[] {
  try {
    const parsed = JSON.parse(raw) as unknown
    if (Array.isArray(parsed)) {
      const out: WsEnvelope[] = []
      for (const item of parsed) {
        const env = coerceWsEnvelope(item)
        if (env) out.push(env)
      }
      return out
    }
    const env = coerceWsEnvelope(parsed)
    return env ? [env] : []
  } catch {
    return []
  }
}
