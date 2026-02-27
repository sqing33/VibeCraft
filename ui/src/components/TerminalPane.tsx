import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from 'xterm'
import 'xterm/css/xterm.css'
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from 'react'

export type TerminalPaneHandle = {
  write: (data: string) => void
  reset: (data?: string) => void
}

/**
 * 功能：渲染 xterm.js 终端，并通过 ref 提供 `write/reset` 命令式接口。
 * 参数/返回：不接收 props；ref 为 TerminalPaneHandle；返回终端容器节点。
 * 失败场景：容器未就绪时跳过初始化；fit 失败时忽略异常。
 * 副作用：创建 Terminal 实例、绑定 ResizeObserver、占用 DOM 节点。
 */
export const TerminalPane = forwardRef<TerminalPaneHandle>(function TerminalPane(
  _props,
  ref,
) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const termRef = useRef<Terminal | null>(null)
  const fitRef = useRef<FitAddon | null>(null)

  useImperativeHandle(
    ref,
    () => ({
      write: (data: string) => {
        termRef.current?.write(data)
      },
      reset: (data?: string) => {
        const term = termRef.current
        if (!term) return
        term.clear()
        if (data) term.write(data)
      },
    }),
    [],
  )

  useEffect(() => {
    const el = containerRef.current
    if (!el) return

    const term = new Terminal({
      convertEol: true,
      cursorBlink: false,
      scrollback: 5000,
      fontSize: 12,
      fontFamily:
        "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
      theme: { background: '#111316' },
    })
    const fit = new FitAddon()
    term.loadAddon(fit)
    term.open(el)
    fit.fit()

    termRef.current = term
    fitRef.current = fit

    const ro = new ResizeObserver(() => {
      try {
        fit.fit()
      } catch {
        // ignore
      }
    })
    ro.observe(el)

    return () => {
      ro.disconnect()
      term.dispose()
      termRef.current = null
      fitRef.current = null
    }
  }, [])

  return <div className="terminal" ref={containerRef} />
})
