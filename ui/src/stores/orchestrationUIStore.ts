import { create } from 'zustand'

import type { Orchestration, OrchestrationDetail } from '@/lib/daemon'

type OrchestrationDetailCache = {
  detail: OrchestrationDetail | null
  loading: boolean
  loaded: boolean
  error: string | null
}

type OrchestrationUIStore = {
  recentItems: Orchestration[]
  recentLoaded: boolean
  recentRefreshing: boolean
  recentError: string | null
  setRecentState: (payload: {
    items?: Orchestration[]
    loaded?: boolean
    refreshing?: boolean
    error?: string | null
  }) => void

  detailById: Record<string, OrchestrationDetailCache>
  lastViewedOrchestrationId: string | null
  setDetailState: (orchestrationId: string, payload: Partial<OrchestrationDetailCache>) => void
  markLastViewedOrchestration: (orchestrationId: string) => void
}

function createEmptyDetailCache(): OrchestrationDetailCache {
  return {
    detail: null,
    loading: false,
    loaded: false,
    error: null,
  }
}

export const useOrchestrationUIStore = create<OrchestrationUIStore>((set) => ({
  recentItems: [],
  recentLoaded: false,
  recentRefreshing: false,
  recentError: null,
  setRecentState: (payload) =>
    set((state) => ({
      recentItems: payload.items ?? state.recentItems,
      recentLoaded: payload.loaded ?? state.recentLoaded,
      recentRefreshing: payload.refreshing ?? state.recentRefreshing,
      recentError: payload.error === undefined ? state.recentError : payload.error,
    })),

  detailById: {},
  lastViewedOrchestrationId: null,
  setDetailState: (orchestrationId, payload) =>
    set((state) => ({
      detailById: {
        ...state.detailById,
        [orchestrationId]: {
          ...(state.detailById[orchestrationId] ?? createEmptyDetailCache()),
          ...payload,
        },
      },
    })),
  markLastViewedOrchestration: (orchestrationId) => set({ lastViewedOrchestrationId: orchestrationId }),
}))

export function getEmptyOrchestrationDetailCache(): OrchestrationDetailCache {
  return createEmptyDetailCache()
}
