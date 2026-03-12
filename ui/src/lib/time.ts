/**
 * 功能：将 Unix ms 时间戳格式化为“人类可读”的相对时间（例如 2 分钟前）。
 * 参数/返回：接收 ms；返回字符串。
 * 失败场景：ms 非法时返回 `-`。
 * 副作用：无。
 */
export function formatRelativeTime(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '-'
  const diff = Date.now() - ms
  const abs = Math.abs(diff)

  const sec = Math.round(abs / 1000)
  if (sec < 10) return '刚刚'
  if (sec < 60) return `${sec} 秒前`

  const min = Math.round(sec / 60)
  if (min < 60) return `${min} 分钟前`

  const hr = Math.round(min / 60)
  if (hr < 24) return `${hr} 小时前`

  const day = Math.round(hr / 24)
  if (day < 30) return `${day} 天前`

  const month = Math.round(day / 30)
  if (month < 12) return `${month} 个月前`

  const year = Math.round(month / 12)
  return `${year} 年前`
}

/**
 * 功能：将 Unix ms 时间戳格式化为本地绝对时间（YYYY-MM-DD HH:mm）。
 * 参数/返回：接收 ms；返回字符串。
 * 失败场景：ms 非法时返回 `-`。
 * 副作用：无。
 */
export function formatAbsoluteTime(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '-'
  const d = new Date(ms)
  const pad2 = (n: number) => String(n).padStart(2, '0')
  const yyyy = d.getFullYear()
  const mm = pad2(d.getMonth() + 1)
  const dd = pad2(d.getDate())
  const hh = pad2(d.getHours())
  const min = pad2(d.getMinutes())
  return `${yyyy}-${mm}-${dd} ${hh}:${min}`
}
