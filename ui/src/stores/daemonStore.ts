import { create } from 'zustand'

import { daemonUrlFromEnv, wsUrlFromDaemonUrl, type PublicExpert } from '@/lib/daemon'

export type HealthState =
  | { status: 'checking' }
  | { status: 'ok' }
  | { status: 'error'; message: string }

export type WsState = 'connecting' | 'connected' | 'disconnected'


export type DaemonStore = {
  daemonUrl: string
  wsUrl: string
  health: HealthState
  wsState: WsState
  experts: PublicExpert[]
  expertsError: string | null
  setWsState: (state: WsState) => void
  setHealth: (health: HealthState) => void
  setExperts: (experts: PublicExpert[]) => void
  setExpertsError: (error: string | null) => void
}

export const useDaemonStore = create<DaemonStore>((set) => {
  const daemonUrl = daemonUrlFromEnv()
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
    experts: [],
    expertsError: null,
    setWsState: (state) => set({ wsState: state }),
    setHealth: (health) => set({ health }),
    setExperts: (experts) => set({ experts }),
    setExpertsError: (expertsError) => set({ expertsError }),
  }
})
