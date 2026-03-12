import { create } from 'zustand'

import type {
  RepoLibraryCard,
  RepoLibraryCardEvidence,
  RepoLibraryDepth,
  RepoLibraryRepositoryDetail,
  RepoLibraryRepositorySummary,
  RepoLibrarySearchResult,
} from '@/lib/daemon'

export type RepoLibraryAnalysisDraft = {
  repo_url: string
  ref: string
  features: string[]
  depth: RepoLibraryDepth
  language: 'zh-CN' | 'en'
  analyzer_mode: 'full' | 'compact'
  cli_tool_id?: string
  model_id?: string
}

type RepoLibraryDetailCache = {
  detail: RepoLibraryRepositoryDetail | null
  cards: RepoLibraryCard[]
  cardsById: Record<string, RepoLibraryCard>
  evidenceByCardId: Record<string, RepoLibraryCardEvidence[]>
  selectedCard: RepoLibraryCard | null
  evidence: RepoLibraryCardEvidence[]
  reportMarkdown: string
  reportExcerptMarkdown: string
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

  analysisDraft: RepoLibraryAnalysisDraft | null
  setAnalysisDraft: (draft: RepoLibraryAnalysisDraft | null) => void
  clearAnalysisDraft: () => void

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
    cards: [],
    cardsById: {},
    evidenceByCardId: {},
    selectedCard: null,
    evidence: [],
    reportMarkdown: '',
    reportExcerptMarkdown: '',
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

  analysisDraft: null,
  setAnalysisDraft: (draft) => set({ analysisDraft: draft }),
  clearAnalysisDraft: () => set({ analysisDraft: null }),

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
