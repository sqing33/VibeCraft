import { useCallback, useEffect, useRef, useState } from 'react'
import { Bug, Play, XCircle } from 'lucide-react'
import {
  Button,
  Chip,
  Modal,
  ModalBody,
  ModalContent,
  ModalHeader,
  Skeleton,
} from '@heroui/react'

import { toast } from '@/lib/toast'
import {
  cancelExecution,
  fetchExecutionLogTail,
  startExecution,
  type Execution,
} from '@/lib/daemon'
import { onWsEnvelope } from '@/lib/wsBus'
import { TerminalPane, type TerminalPaneHandle } from '@/components/TerminalPane'
import { useDaemonStore } from '@/stores/daemonStore'

function wsText(state: string): string {
  if (state === 'connected') return '已连接'
  if (state === 'connecting') return '连接中'
  return '未连接'
}

function executionStatusText(status: string): string {
  if (status === 'running') return '运行中'
  if (status === 'done') return '已完成'
  if (status === 'failed') return '失败'
  if (status === 'canceled') return '已取消'
  if (status === 'timeout') return '超时'
  return status
}

export function DevToolsDialog() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const wsState = useDaemonStore((s) => s.wsState)

  const [open, setOpen] = useState(false)
  const [executions, setExecutions] = useState<Execution[]>([])
  const [selectedExecutionId, setSelectedExecutionId] = useState<string | null>(null)
  const [starting, setStarting] = useState(false)
  const [canceling, setCanceling] = useState(false)

  const terminalRef = useRef<TerminalPaneHandle | null>(null)
  const terminalPendingRef = useRef<string>('')
  const terminalFlushRafRef = useRef<number | null>(null)
  const selectedExecutionIdRef = useRef<string | null>(null)

  const flushTerminalPending = useCallback(() => {
    if (terminalFlushRafRef.current != null) {
      window.cancelAnimationFrame(terminalFlushRafRef.current)
      terminalFlushRafRef.current = null
    }
    const chunk = terminalPendingRef.current
    if (!chunk) return
    terminalPendingRef.current = ''
    terminalRef.current?.write(chunk)
  }, [])

  const enqueueTerminalWrite = useCallback(
    (chunk: string) => {
      if (!chunk) return
      terminalPendingRef.current += chunk

      if (terminalPendingRef.current.length >= 512 * 1024) {
        flushTerminalPending()
        return
      }

      if (terminalFlushRafRef.current != null) return
      terminalFlushRafRef.current = window.requestAnimationFrame(() => {
        terminalFlushRafRef.current = null
        const data = terminalPendingRef.current
        if (!data) return
        terminalPendingRef.current = ''
        terminalRef.current?.write(data)
      })
    },
    [flushTerminalPending],
  )

  const loadTailIntoTerminal = useCallback(
    async (executionId: string) => {
      terminalPendingRef.current = ''
      if (terminalFlushRafRef.current != null) {
        window.cancelAnimationFrame(terminalFlushRafRef.current)
        terminalFlushRafRef.current = null
      }
      terminalRef.current?.reset('正在加载日志…\r\n')
      try {
        const text = await fetchExecutionLogTail(daemonUrl, executionId, 200000)
        terminalRef.current?.reset(text)
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : String(err)
        terminalRef.current?.reset(`日志加载失败：${message}\r\n`)
      }
    },
    [daemonUrl],
  )

  useEffect(() => {
    selectedExecutionIdRef.current = selectedExecutionId
  }, [selectedExecutionId])

  useEffect(() => {
    if (!selectedExecutionId) {
      terminalRef.current?.reset('未选择执行记录。\r\n')
      return
    }
    void loadTailIntoTerminal(selectedExecutionId)
  }, [selectedExecutionId, loadTailIntoTerminal])

  useEffect(() => {
    if (wsState !== 'connected') return
    const exId = selectedExecutionIdRef.current
    if (!exId) return
    void loadTailIntoTerminal(exId)
  }, [wsState, loadTailIntoTerminal])

  useEffect(() => {
    return onWsEnvelope((env) => {
      const exId = env.execution_id
      if (!exId) return

      if (env.type === 'node.log') {
        if (exId !== selectedExecutionIdRef.current) return
        const payload = env.payload as { chunk?: unknown } | undefined
        const chunk = typeof payload?.chunk === 'string' ? payload.chunk : ''
        enqueueTerminalWrite(chunk)
        return
      }

      if (env.type === 'execution.exited') {
        const payload = env.payload as { status?: unknown } | undefined
        const status = typeof payload?.status === 'string' ? payload.status : 'failed'
        setExecutions((prev) => prev.map((e) => (e.execution_id === exId ? { ...e, status } : e)))
      }
    })
  }, [enqueueTerminalWrite])

  const onRunDemo = async () => {
    setStarting(true)
    try {
      const exec = await startExecution(daemonUrl)
      setExecutions((prev) => [exec, ...prev])
      setSelectedExecutionId(exec.execution_id)
      toast({ title: '执行已启动', description: exec.execution_id })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '启动失败', description: message })
    } finally {
      setStarting(false)
    }
  }

  const onCancelSelected = async () => {
    if (!selectedExecutionId) return
    setCanceling(true)
    try {
      await cancelExecution(daemonUrl, selectedExecutionId)
      toast({ title: '已发起取消', description: selectedExecutionId })
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err)
      toast({ variant: 'destructive', title: '取消失败', description: message })
    } finally {
      setCanceling(false)
    }
  }

  if (!import.meta.env.DEV) return null

  return (
    <>
      <Button
        variant="light"
        size="sm"
        isIconOnly
        aria-label="开发工具"
        onPress={() => setOpen(true)}
      >
        <Bug className="h-4 w-4" aria-hidden="true" focusable="false" />
      </Button>

      <Modal isOpen={open} onOpenChange={setOpen} size="5xl" scrollBehavior="inside">
        <ModalContent>
          {() => (
            <>
              <ModalHeader className="flex flex-col gap-1">
                <div>开发工具</div>
                <div className="text-sm font-normal text-muted-foreground">
                  仅开发环境可见（生产构建中隐藏）
                </div>
              </ModalHeader>

              <ModalBody>
                <div className="space-y-4">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <Chip variant="flat" size="sm">
                        连接：{wsText(wsState)}
                      </Chip>
                      <Chip variant="bordered" size="sm">
                        {daemonUrl}
                      </Chip>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        color="primary"
                        onPress={() => void onRunDemo()}
                        isDisabled={starting}
                        startContent={
                          <Play className="h-4 w-4" aria-hidden="true" focusable="false" />
                        }
                      >
                        {starting ? '启动中…' : '运行示例'}
                      </Button>
                      <Button
                        color="danger"
                        onPress={() => void onCancelSelected()}
                        isDisabled={!selectedExecutionId || canceling}
                        startContent={
                          <XCircle className="h-4 w-4" aria-hidden="true" focusable="false" />
                        }
                      >
                        {canceling ? '取消中…' : '取消执行'}
                      </Button>
                    </div>
                  </div>

                  <div className="grid gap-4 lg:grid-cols-[360px_1fr]">
                    <div className="rounded-xl border bg-card p-3">
                      <div className="text-sm font-semibold">执行列表</div>
                      <div className="mt-3 space-y-2">
                        {executions.length === 0 ? (
                          <div className="text-sm text-muted-foreground">
                            点击“运行示例”开始一次执行。
                          </div>
                        ) : (
                          executions.map((e) => (
                            <button
                              key={e.execution_id}
                              className={
                                selectedExecutionId === e.execution_id
                                  ? 'w-full rounded-lg border bg-muted/30 p-3 text-left'
                                  : 'w-full rounded-lg border bg-background/40 p-3 text-left hover:bg-background/60'
                              }
                              onClick={() => setSelectedExecutionId(e.execution_id)}
                            >
                              <div className="flex items-center justify-between gap-2">
                                <Chip variant="flat" size="sm">
                                  {executionStatusText(e.status)}
                                </Chip>
                                <code className="truncate text-xs">{e.execution_id}</code>
                              </div>
                              <div className="mt-2 truncate text-xs text-muted-foreground">
                                {e.command}
                              </div>
                            </button>
                          ))
                        )}
                      </div>
                    </div>

                    <div className="rounded-xl border bg-card p-3">
                      <div className="flex items-center justify-between gap-2">
                        <div className="text-sm font-semibold">终端</div>
                        {selectedExecutionId ? (
                          <Chip variant="bordered" size="sm">
                            {selectedExecutionId}
                          </Chip>
                        ) : (
                          <Skeleton className="h-5 w-40 rounded-md" />
                        )}
                      </div>
                      <div className="mt-3">
                        <TerminalPane ref={terminalRef} />
                      </div>
                    </div>
                  </div>
                </div>
              </ModalBody>
            </>
          )}
        </ModalContent>
      </Modal>
    </>
  )
}
