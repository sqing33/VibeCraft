import type { WsEnvelope } from '@/lib/ws'

type Listener = (env: WsEnvelope) => void

const listeners = new Set<Listener>()

export function emitWsEnvelope(env: WsEnvelope) {
  for (const l of listeners) l(env)
}

export function onWsEnvelope(listener: Listener) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

