export type WsEnvelope = {
  type: string
  ts: number
  workflow_id?: string
  node_id?: string
  execution_id?: string
  payload?: unknown
}

export function parseWsEnvelope(raw: string): WsEnvelope | null {
  try {
    const obj = JSON.parse(raw) as WsEnvelope
    if (!obj || typeof obj !== 'object') return null
    if (typeof obj.type !== 'string') return null
    return obj
  } catch {
    return null
  }
}

