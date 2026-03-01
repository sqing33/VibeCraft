import { FitAddon } from '@xterm/addon-fit'
import { Terminal } from 'xterm'
import 'xterm/css/xterm.css'
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useRef,
} from 'react'
import { cn } from '@/lib/utils'
import { type ThemeMode, useThemeStore } from '@/stores/themeStore'

export type TerminalPaneHandle = {
  write: (data: string) => void
  reset: (data?: string) => void
}

function terminalTheme(mode: ThemeMode) {
  if (mode === 'dark') {
    return {
      background: '#0b0f14',
      foreground: '#d8e1ea',
      cursor: '#d8e1ea',
      selectionBackground: '#243547',
    }
  }
  return {
    background: '#f5f7fb',
    foreground: '#1f2937',
    cursor: '#111827',
    selectionBackground: '#bfdbfe',
  }
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
  const theme = useThemeStore((s) => s.theme)
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
      theme: terminalTheme(theme),
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

  useEffect(() => {
    const term = termRef.current
    if (!term) return
    term.options.theme = terminalTheme(theme)
  }, [theme])

  return (
    <div
      className={cn(
        'h-[420px] overflow-hidden rounded-xl border',
        theme === 'dark' ? 'bg-[#0b0f14]' : 'bg-[#f5f7fb]',
      )}
      ref={containerRef}
    />
  )
})
