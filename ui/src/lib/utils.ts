import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * 功能：合并 className（支持条件表达式 + Tailwind 冲突合并）。
 * 参数/返回：接收多个 class 输入；返回合并后的字符串。
 * 失败场景：无。
 * 副作用：无。
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

