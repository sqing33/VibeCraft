import { create } from 'zustand'

import type { Edge, Node, Workflow } from '@/lib/daemon'

export type WorkflowStore = {
  workflows: Workflow[]
  workflowsError: string | null
  workflowsLoading: boolean

  selectedWorkflowId: string | null
  nodes: Node[]
  edges: Edge[]
  graphError: string | null
  graphLoading: boolean
  selectedNodeId: string | null

  setWorkflows: (workflows: Workflow[]) => void
  setWorkflowsError: (error: string | null) => void
  setWorkflowsLoading: (loading: boolean) => void
  setSelectedWorkflowId: (workflowId: string | null) => void
  setGraph: (nodes: Node[], edges: Edge[]) => void
  setGraphError: (error: string | null) => void
  setGraphLoading: (loading: boolean) => void
  setSelectedNodeId: (nodeId: string | null) => void
}

export const useWorkflowStore = create<WorkflowStore>((set) => ({
  workflows: [],
  workflowsError: null,
  workflowsLoading: false,

  selectedWorkflowId: null,
  nodes: [],
  edges: [],
  graphError: null,
  graphLoading: false,
  selectedNodeId: null,

  setWorkflows: (workflows) => set({ workflows }),
  setWorkflowsError: (workflowsError) => set({ workflowsError }),
  setWorkflowsLoading: (workflowsLoading) => set({ workflowsLoading }),
  setSelectedWorkflowId: (selectedWorkflowId) => set({ selectedWorkflowId }),
  setGraph: (nodes, edges) => set({ nodes, edges }),
  setGraphError: (graphError) => set({ graphError }),
  setGraphLoading: (graphLoading) => set({ graphLoading }),
  setSelectedNodeId: (selectedNodeId) => set({ selectedNodeId }),
}))

