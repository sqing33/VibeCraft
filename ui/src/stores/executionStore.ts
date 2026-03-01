import { create } from 'zustand'

import type { Execution } from '@/lib/daemon'

export type ExecutionStore = {
  executions: Execution[]
  selectedExecutionId: string | null
  executionsError: string | null

  setExecutions: (executions: Execution[]) => void
  setSelectedExecutionId: (executionId: string | null) => void
  setExecutionsError: (error: string | null) => void
}

export const useExecutionStore = create<ExecutionStore>((set) => ({
  executions: [],
  selectedExecutionId: null,
  executionsError: null,

  setExecutions: (executions) => set({ executions }),
  setSelectedExecutionId: (selectedExecutionId) => set({ selectedExecutionId }),
  setExecutionsError: (executionsError) => set({ executionsError }),
}))

