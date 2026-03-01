import { create } from 'zustand'

export type ThemeMode = 'light' | 'dark'

const THEME_STORAGE_KEY = 'vibe-tree.theme'

function normalizeTheme(raw: string | null | undefined): ThemeMode | null {
  if (raw === 'dark') return 'dark'
  if (raw === 'light') return 'light'
  return null
}

function readSavedTheme(): ThemeMode | null {
  if (typeof window === 'undefined') return null
  try {
    return normalizeTheme(window.localStorage.getItem(THEME_STORAGE_KEY))
  } catch {
    return null
  }
}

function applyThemeClass(theme: ThemeMode) {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

function resolveInitialTheme(): ThemeMode {
  return readSavedTheme() ?? 'light'
}

const initialTheme = resolveInitialTheme()
applyThemeClass(initialTheme)

export type ThemeStore = {
  theme: ThemeMode
  setTheme: (theme: ThemeMode) => void
  toggleTheme: () => void
}

function persistTheme(theme: ThemeMode) {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme)
  } catch {
    // ignore localStorage errors and keep in-memory theme
  }
}

export const useThemeStore = create<ThemeStore>((set, get) => ({
  theme: initialTheme,
  setTheme: (nextTheme) => {
    const theme: ThemeMode = nextTheme === 'dark' ? 'dark' : 'light'
    persistTheme(theme)
    applyThemeClass(theme)
    set({ theme })
  },
  toggleTheme: () => {
    const current = get().theme
    const next: ThemeMode = current === 'dark' ? 'light' : 'dark'
    persistTheme(next)
    applyThemeClass(next)
    set({ theme: next })
  },
}))
