// 格式化工具
export function fmtBytes(n: number | undefined | null): string {
  if (n === undefined || n === null || n < 0) return '-'
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let v = n
  let i = -1
  do {
    v /= 1024
    i++
  } while (v >= 1024 && i < units.length - 1)
  return `${v.toFixed(v >= 100 ? 0 : 1)} ${units[i]}`
}

export function fmtTTL(ms: number): string {
  if (ms === -1) return '永久'
  if (ms === -2) return '不存在'
  if (ms <= 0) return '0s'
  const s = Math.floor(ms / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m${s % 60 ? ` ${s % 60}s` : ''}`
  const h = Math.floor(m / 60)
  if (h < 48) return `${h}h${m % 60 ? ` ${m % 60}m` : ''}`
  const d = Math.floor(h / 24)
  return `${d}d`
}

export function fmtNum(n: number | undefined): string {
  if (n === undefined || n === null) return '-'
  if (Math.abs(n) >= 1e9) return `${(n / 1e9).toFixed(1)}G`
  if (Math.abs(n) >= 1e6) return `${(n / 1e6).toFixed(1)}M`
  if (Math.abs(n) >= 1e4) return `${(n / 1e3).toFixed(1)}k`
  return n.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

export function fmtTime(unixSec: number): string {
  return new Date(unixSec * 1000).toLocaleString('zh-CN', { hour12: false })
}

export function fmtTimeMs(unixMs: number): string {
  return new Date(unixMs).toLocaleTimeString('zh-CN', { hour12: false })
}

export function fmtUsec(us: number): string {
  if (us < 1000) return `${us}µs`
  return `${(us / 1000).toFixed(1)}ms`
}

// 类型显示名与颜色
export const TYPE_META: Record<string, { label: string; color: string }> = {
  string: { label: 'STR', color: 'var(--t-string)' },
  hash: { label: 'HASH', color: 'var(--t-hash)' },
  list: { label: 'LIST', color: 'var(--t-list)' },
  set: { label: 'SET', color: 'var(--t-set)' },
  zset: { label: 'ZSET', color: 'var(--t-zset)' },
  stream: { label: 'STREAM', color: 'var(--t-stream)' },
  json: { label: 'JSON', color: 'var(--t-json)' },
  module: { label: 'MODULE', color: 'var(--t-none)' },
  none: { label: 'NONE', color: 'var(--t-none)' },
}

export function typeMeta(t: string) {
  return TYPE_META[t] || { label: t.toUpperCase().slice(0, 6), color: 'var(--t-none)' }
}

// JSON 语法高亮（返回 HTML）
export function highlightJSON(json: string): string {
  const escaped = json
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  return escaped.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false)\b|\bnull\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    (match) => {
      let cls = 'j-num'
      if (/^"/.test(match)) {
        cls = /:$/.test(match) ? 'j-key' : 'j-str'
      } else if (/true|false/.test(match)) {
        cls = 'j-bool'
      } else if (/null/.test(match)) {
        cls = 'j-null'
      }
      return `<span class="${cls}">${match}</span>`
    },
  )
}

// 尝试美化 JSON；失败返回原文
export function prettyJSON(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}
