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

type VibeNodeData = {
  title: string
  status: string
  expert_id: string
  node_type: string
}

const NODE_WIDTH = 190
const NODE_HEIGHT = 58

function statusClass(status: string): string {
  switch (status) {
    case 'running':
      return 'vibeNode running'
    case 'succeeded':
      return 'vibeNode succeeded'
    case 'failed':
      return 'vibeNode failed'
    case 'timeout':
      return 'vibeNode timeout'
    case 'canceled':
      return 'vibeNode canceled'
    case 'pending_approval':
      return 'vibeNode pending'
    case 'queued':
      return 'vibeNode queued'
    case 'skipped':
      return 'vibeNode skipped'
    default:
      return 'vibeNode'
  }
}

const VibeNode = memo(function VibeNode(props: NodeProps<VibeNodeData>) {
  const { data, selected } = props
  return (
    <div className={selected ? `${statusClass(data.status)} selected` : statusClass(data.status)}>
      <div className="vibeNodeTop">
        <span className="vibeNodeTitle">{data.title || '(untitled)'}</span>
        <span className="vibeNodeStatus">{data.status}</span>
      </div>
      <div className="vibeNodeMeta">
        <span className="vibeNodeType">{data.node_type}</span>
        <span className="vibeNodeExpert">{data.expert_id}</span>
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
    <div className="dagCanvas">
      <ReactFlow
        nodes={rfNodes}
        edges={rfEdges}
        nodeTypes={{ vibeNode: VibeNode }}
        nodesDraggable={false}
        nodesConnectable={false}
        onNodeClick={(_ev, node) => onSelectNodeId(node.id)}
        fitView
      >
        <Background gap={18} size={0.6} color="rgba(255,255,255,0.08)" />
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
