import { create } from 'zustand'

import type {
  RepoLibraryCard,
  RepoLibraryCardEvidence,
  RepoLibraryRepositoryDetail,
  RepoLibraryRepositorySummary,
  RepoLibrarySearchResult,
  RepoLibrarySnapshot,
} from '@/lib/daemon'

type RepoLibraryDetailCache = {
  detail: RepoLibraryRepositoryDetail | null
  snapshots: RepoLibrarySnapshot[]
  cards: RepoLibraryCard[]
  selectedCard: RepoLibraryCard | null
  evidence: RepoLibraryCardEvidence[]
  reportMarkdown: string
  selectedSnapshotId: string | null
  selectedAnalysisId: string | null
  selectedCardId: string | null
  loading: boolean
  loaded: boolean
  error: string | null
  cardsLoading: boolean
  cardsError: string | null
  cardLoading: boolean
  cardError: string | null
}

type RepoLibraryUIStore = {
  repositories: RepoLibraryRepositorySummary[]
  repositoriesLoaded: boolean
  repositoriesRefreshing: boolean
  repositoriesError: string | null
  setRepositoriesState: (payload: {
    repositories?: RepoLibraryRepositorySummary[]
    loaded?: boolean
    refreshing?: boolean
    error?: string | null
  }) => void

  searchQuery: string
  searchLimit: string
  searchResults: RepoLibrarySearchResult[]
  searchLoaded: boolean
  searchSubmitting: boolean
  searchError: string | null
  searchLastQueryAt: number | null
  setSearchDraft: (payload: { query?: string; limit?: string }) => void
  setSearchState: (payload: {
    results?: RepoLibrarySearchResult[]
    loaded?: boolean
    submitting?: boolean
    error?: string | null
    lastQueryAt?: number | null
  }) => void

  detailsByRepositoryId: Record<string, RepoLibraryDetailCache>
  lastViewedRepositoryId: string | null
  setRepositoryDetailState: (repositoryId: string, payload: Partial<RepoLibraryDetailCache>) => void
  markLastViewedRepository: (repositoryId: string) => void
}

function createEmptyDetailCache(): RepoLibraryDetailCache {
  return {
    detail: null,
    snapshots: [],
    cards: [],
    selectedCard: null,
    evidence: [],
    reportMarkdown: '',
    selectedSnapshotId: null,
    selectedAnalysisId: null,
    selectedCardId: null,
    loading: false,
    loaded: false,
    error: null,
    cardsLoading: false,
    cardsError: null,
    cardLoading: false,
    cardError: null,
  }
}

export const useRepoLibraryUIStore = create<RepoLibraryUIStore>((set) => ({
  repositories: [],
  repositoriesLoaded: false,
  repositoriesRefreshing: false,
  repositoriesError: null,
  setRepositoriesState: (payload) =>
    set((state) => ({
      repositories: payload.repositories ?? state.repositories,
      repositoriesLoaded: payload.loaded ?? state.repositoriesLoaded,
      repositoriesRefreshing: payload.refreshing ?? state.repositoriesRefreshing,
      repositoriesError: payload.error === undefined ? state.repositoriesError : payload.error,
    })),

  searchQuery: '认证流程是如何落地的？',
  searchLimit: '8',
  searchResults: [],
  searchLoaded: false,
  searchSubmitting: false,
  searchError: null,
  searchLastQueryAt: null,
  setSearchDraft: (payload) =>
    set((state) => ({
      searchQuery: payload.query ?? state.searchQuery,
      searchLimit: payload.limit ?? state.searchLimit,
    })),
  setSearchState: (payload) =>
    set((state) => ({
      searchResults: payload.results ?? state.searchResults,
      searchLoaded: payload.loaded ?? state.searchLoaded,
      searchSubmitting: payload.submitting ?? state.searchSubmitting,
      searchError: payload.error === undefined ? state.searchError : payload.error,
      searchLastQueryAt: payload.lastQueryAt === undefined ? state.searchLastQueryAt : payload.lastQueryAt,
    })),

  detailsByRepositoryId: {},
  lastViewedRepositoryId: null,
  setRepositoryDetailState: (repositoryId, payload) =>
    set((state) => ({
      detailsByRepositoryId: {
        ...state.detailsByRepositoryId,
        [repositoryId]: {
          ...(state.detailsByRepositoryId[repositoryId] ?? createEmptyDetailCache()),
          ...payload,
        },
      },
    })),
  markLastViewedRepository: (repositoryId) => set({ lastViewedRepositoryId: repositoryId }),
}))

export function getEmptyRepoLibraryDetailCache(): RepoLibraryDetailCache {
  return createEmptyDetailCache()
}
