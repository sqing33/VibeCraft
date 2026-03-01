import { create } from 'zustand'

import {
  daemonUrlFromEnv,
  wsUrlFromDaemonUrl,
  type DaemonInfo,
  type PublicExpert,
} from '@/lib/daemon'

export type HealthState =
  | { status: 'checking' }
  | { status: 'ok' }
  | { status: 'error'; message: string }

export type WsState = 'connecting' | 'connected' | 'disconnected'

function normalizeBaseUrl(raw: string): string {
  const url = (raw ?? '').trim()
  if (!url) return ''
  return url.endsWith('/') ? url.slice(0, -1) : url
}

function loadInitialDaemonUrl(): string {
  const saved = normalizeBaseUrl(
    window.localStorage.getItem('vibe-tree.daemon_url') ?? '',
  )
  return saved || daemonUrlFromEnv()
}

export type DaemonStore = {
  daemonUrl: string
  wsUrl: string
  health: HealthState
  wsState: WsState
  info: DaemonInfo | null
  infoError: string | null
  experts: PublicExpert[]
  expertsError: string | null
  setDaemonUrl: (next: string) => void
  resetDaemonUrl: () => void
  setWsState: (state: WsState) => void
  setHealth: (health: HealthState) => void
  setInfo: (info: DaemonInfo | null) => void
  setInfoError: (error: string | null) => void
  setExperts: (experts: PublicExpert[]) => void
  setExpertsError: (error: string | null) => void
}

export const useDaemonStore = create<DaemonStore>((set) => {
  const daemonUrl = loadInitialDaemonUrl()
  const wsUrl = (() => {
    try {
      return wsUrlFromDaemonUrl(daemonUrl)
    } catch {
      return ''
    }
  })()

  return {
    daemonUrl,
    wsUrl,
    health: { status: 'checking' },
    wsState: 'connecting',
    info: null,
    infoError: null,
    experts: [],
    expertsError: null,
    setDaemonUrl: (next) => {
      const normalized = normalizeBaseUrl(next)
      window.localStorage.setItem('vibe-tree.daemon_url', normalized)
      set({
        daemonUrl: normalized,
        wsUrl: (() => {
          try {
            return wsUrlFromDaemonUrl(normalized)
          } catch {
            return ''
          }
        })(),
      })
    },
    resetDaemonUrl: () => {
      window.localStorage.removeItem('vibe-tree.daemon_url')
      const next = daemonUrlFromEnv()
      set({
        daemonUrl: next,
        wsUrl: (() => {
          try {
            return wsUrlFromDaemonUrl(next)
          } catch {
            return ''
          }
        })(),
      })
    },
    setWsState: (state) => set({ wsState: state }),
    setHealth: (health) => set({ health }),
    setInfo: (info) => set({ info }),
    setInfoError: (infoError) => set({ infoError }),
    setExperts: (experts) => set({ experts }),
    setExpertsError: (expertsError) => set({ expertsError }),
  }
})
