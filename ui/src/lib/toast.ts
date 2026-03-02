import { addToast } from '@heroui/react'

export type ToastInput = {
  title?: string
  description?: string
  variant?: 'default' | 'destructive'
}

/**
 * 功能：统一 toast 入口（兼容历史 shadcn `toast({ title, description, variant })` 调用形态）。
 * 参数/返回：入参为 toast 文案与可选 variant；返回 void。
 * 失败场景：无（底层 toast 队列异常时忽略）。
 * 副作用：向全局 ToastProvider 推送一条 toast。
 */
export function toast(input: ToastInput) {
  const variant = input.variant === 'destructive' ? 'destructive' : 'default'
  const title = (input.title ?? '').trim()
  const description = (input.description ?? '').trim()

  try {
    addToast({
      title: title || undefined,
      description: description || undefined,
      color: variant === 'destructive' ? 'danger' : 'default',
      severity: variant === 'destructive' ? 'danger' : 'default',
      variant: 'flat',
      timeout: 4000,
      shouldShowTimeoutProgress: true,
    })
  } catch {
    // ignore toast errors to avoid breaking primary flows
  }
}

