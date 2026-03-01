import dagre from 'dagre'
import { memo, useEffect, useMemo } from 'react'
import ReactFlow, {
  Background,
  Controls,
  Position,
  ReactFlowProvider,
  type Edge as RFEdge,
  type Node as RFNode,
  type NodeProps,
  useReactFlow,
} from 'reactflow'
import 'reactflow/dist/style.css'
import type { Edge, Node } from '../lib/daemon'
import { cn } from '@/lib/utils'

type VibeNodeData = {
  title: string
  status: string
  expert_id: string
  node_type: string
}

const NODE_WIDTH = 190
const NODE_HEIGHT = 58

function formatNodeStatus(status: string): string {
  if (status === 'running') return '进行中'
  if (status === 'succeeded') return '成功'
  if (status === 'failed') return '失败'
  if (status === 'timeout') return '超时'
  if (status === 'canceled') return '已取消'
  if (status === 'pending_approval') return '待审批'
  if (status === 'queued') return '排队中'
  if (status === 'skipped') return '已跳过'
  if (status === 'draft') return '草稿'
  return status
}

function formatNodeType(nodeType: string): string {
  if (nodeType === 'master') return '主节点'
  if (nodeType === 'worker') return '工作节点'
  return nodeType
}

function statusClass(status: string): string {
  const base =
    'flex h-[58px] w-[190px] flex-col justify-between gap-1 rounded-xl border px-3 py-2 text-xs shadow-sm'
  switch (status) {
    case 'running':
      return cn(base, 'border-amber-400/50 bg-amber-500/10')
    case 'succeeded':
      return cn(base, 'border-emerald-400/50 bg-emerald-500/10')
    case 'failed':
      return cn(base, 'border-red-400/50 bg-red-500/10 animate-pulse')
    case 'timeout':
      return cn(base, 'border-fuchsia-400/50 bg-fuchsia-500/10')
    case 'canceled':
      return cn(base, 'border-border/60 bg-muted/20 opacity-80')
    case 'pending_approval':
      return cn(base, 'border-border/60 bg-muted/10')
    case 'queued':
      return cn(base, 'border-sky-400/50 bg-sky-500/10')
    case 'skipped':
      return cn(base, 'border-border/40 bg-muted/10 opacity-70')
    default:
      return cn(base, 'border-border/60 bg-card')
  }
}

const VibeNode = memo(function VibeNode(props: NodeProps<VibeNodeData>) {
  const { data, selected } = props
  return (
    <div
      className={cn(
        statusClass(data.status),
        selected ? 'ring-2 ring-ring ring-offset-2 ring-offset-background' : null,
      )}
    >
      <div className="flex items-baseline justify-between gap-2">
        <span className="truncate font-semibold">{data.title || '（未命名）'}</span>
        <span className="shrink-0 text-[10px] text-muted-foreground">
          {formatNodeStatus(data.status)}
        </span>
      </div>
      <div className="flex items-center justify-between gap-2 text-[10px] text-muted-foreground">
        <span className="truncate">{formatNodeType(data.node_type)}</span>
        <span className="truncate">{data.expert_id}</span>
      </div>
    </div>
  )
})

function buildLayout(rawNodes: RFNode<VibeNodeData>[], rawEdges: RFEdge[]): RFNode<VibeNodeData>[] {
  const g = new dagre.graphlib.Graph()
  g.setDefaultEdgeLabel(() => ({}))
  g.setGraph({ rankdir: 'LR', ranksep: 90, nodesep: 40 })

  for (const n of rawNodes) {
    g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT })
  }
  for (const e of rawEdges) {
    g.setEdge(e.source, e.target)
  }

  dagre.layout(g)

  return rawNodes.map((n) => {
    const p = g.node(n.id) as { x: number; y: number } | undefined
    if (!p) return n
    return {
      ...n,
      position: { x: p.x - NODE_WIDTH / 2, y: p.y - NODE_HEIGHT / 2 },
      targetPosition: Position.Left,
      sourcePosition: Position.Right,
    }
  })
}

type DAGViewInnerProps = {
  nodes: Node[]
  edges: Edge[]
  selectedNodeId: string | null
  onSelectNodeId: (nodeId: string) => void
}

function DAGViewInner(props: DAGViewInnerProps) {
  const { fitView } = useReactFlow()
  const { nodes, edges, selectedNodeId, onSelectNodeId } = props

  const rfNodes = useMemo(() => {
    const base: RFNode<VibeNodeData>[] = nodes.map((n) => ({
      id: n.node_id,
      type: 'vibeNode',
      data: {
        title: n.title,
        status: n.status,
        expert_id: n.expert_id,
        node_type: n.node_type,
      },
      position: { x: 0, y: 0 },
      selected: n.node_id === selectedNodeId,
    }))
    const rfEdges: RFEdge[] = edges.map((e) => ({
      id: e.edge_id,
      source: e.from_node_id,
      target: e.to_node_id,
      type: 'smoothstep',
    }))
    return buildLayout(base, rfEdges)
  }, [nodes, edges, selectedNodeId])

  const rfEdges = useMemo(
    () =>
      edges.map(
        (e): RFEdge => ({
          id: e.edge_id,
          source: e.from_node_id,
          target: e.to_node_id,
          type: 'smoothstep',
        }),
      ),
    [edges],
  )

  useEffect(() => {
    // 功能：当 DAG 变化时自动 fit view，避免用户手动缩放定位。
    // 参数/返回：依赖 fitView；无返回值。
    // 失败场景：React Flow 尚未就绪时可能抛异常（捕获忽略）。
    // 副作用：改变画布 viewport。
    try {
      fitView({ padding: 0.2, duration: 200 })
    } catch {
      // ignore
    }
  }, [fitView, nodes.length, edges.length])

  return (
    <div className="h-[320px] overflow-hidden rounded-xl border bg-muted/20">
      <ReactFlow
        nodes={rfNodes}
        edges={rfEdges}
        nodeTypes={{ vibeNode: VibeNode }}
        nodesDraggable={false}
        nodesConnectable={false}
        onNodeClick={(_ev, node) => onSelectNodeId(node.id)}
        fitView
      >
        <Background gap={18} size={0.6} color="rgba(255,255,255,0.06)" />
        <Controls />
      </ReactFlow>
    </div>
  )
}

type DAGViewProps = {
  nodes: Node[]
  edges: Edge[]
  selectedNodeId: string | null
  onSelectNodeId: (nodeId: string) => void
}

/**
 * 功能：使用 React Flow + dagre 展示 workflow DAG，并支持点击节点联动外部终端。
 * 参数/返回：接收 nodes/edges 与选中节点信息；返回 DAG 画布组件。
 * 失败场景：nodes/edges 为空时仍可渲染（显示空画布）。
 * 副作用：根据 DAG 变化自动 fit view，并触发 onSelectNodeId 回调。
 */
export function DAGView(props: DAGViewProps) {
  return (
    <ReactFlowProvider>
      <DAGViewInner {...props} />
    </ReactFlowProvider>
  )
}
