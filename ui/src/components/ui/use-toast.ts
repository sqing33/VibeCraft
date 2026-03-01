import * as React from 'react'

import type { ToastActionElement, ToastProps } from '@/components/ui/toast'

const TOAST_LIMIT = 3
const TOAST_REMOVE_DELAY = 3000

type ToastPayload = ToastProps & {
  id: string
  title?: React.ReactNode
  description?: React.ReactNode
  action?: ToastActionElement
}

type ToastState = {
  toasts: ToastPayload[]
}

const listeners: Array<(state: ToastState) => void> = []
let memoryState: ToastState = { toasts: [] }

function dispatch(next: ToastState) {
  memoryState = next
  for (const l of listeners) l(memoryState)
}

function genId() {
  return Math.random().toString(36).slice(2)
}

function toast(payload: Omit<ToastPayload, 'id'>) {
  const id = genId()

  const next: ToastPayload = {
    id,
    open: true,
    onOpenChange: (open) => {
      if (!open) dismiss(id)
    },
    ...payload,
  }

  dispatch({
    toasts: [next, ...memoryState.toasts].slice(0, TOAST_LIMIT),
  })

  window.setTimeout(() => dismiss(id), TOAST_REMOVE_DELAY)
  return { id, dismiss: () => dismiss(id) }
}

function dismiss(id?: string) {
  dispatch({
    toasts: memoryState.toasts
      .map((t) => (id && t.id !== id ? t : { ...t, open: false }))
      .filter((t) => t.open !== false),
  })
}

function useToast() {
  const [state, setState] = React.useState<ToastState>(memoryState)

  React.useEffect(() => {
    listeners.push(setState)
    return () => {
      const idx = listeners.indexOf(setState)
      if (idx >= 0) listeners.splice(idx, 1)
    }
  }, [])

  return {
    ...state,
    toast,
    dismiss,
  }
}

export { useToast, toast }

