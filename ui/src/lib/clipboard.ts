/**
 * 功能：复制文本到剪贴板（优先 Clipboard API，失败回退 execCommand）。
 * 参数/返回：接收 text；成功 resolve。
 * 失败场景：极端浏览器限制下可能无法复制（此时 reject）。
 * 副作用：写入系统剪贴板；可能创建临时 DOM 节点。
 */
export async function copyToClipboard(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
    return
  } catch {
    // fallback
  }

  const el = document.createElement('textarea')
  el.value = text
  el.style.position = 'fixed'
  el.style.left = '-9999px'
  document.body.appendChild(el)
  el.select()
  const ok = document.execCommand('copy')
  document.body.removeChild(el)
  if (!ok) throw new Error('copy failed')
}

