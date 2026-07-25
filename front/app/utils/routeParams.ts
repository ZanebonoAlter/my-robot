/**
 * RSSHub 路由参数解析（纯函数，可单测）。
 *
 * 与后端 D3 规则对齐：
 * - path 中 `:param` 为参数段；`:param?`（含 `:param{.+}?`）为可选，其余必填；
 * - 目录自带的 parameters 是原始 JSON（对象或数组），值为中文说明。
 */

export interface RouteParamSpec {
  name: string
  required: boolean
  /** 目录自带的中文参数说明；无说明为空串 */
  description: string
}

/** 解析 path 中的参数段：`:uid` → 必填；`:category{.+}?` → 可选。 */
export function parsePathParams(path: string): Array<{ name: string, required: boolean }> {
  const out: Array<{ name: string, required: boolean }> = []
  for (const seg of path.split('/')) {
    if (!seg.startsWith(':')) continue
    let raw = seg.slice(1)
    let required = true
    if (raw.endsWith('?')) {
      required = false
      raw = raw.slice(0, -1)
    }
    // 去正则约束 {..}
    const brace = raw.indexOf('{')
    if (brace >= 0) raw = raw.slice(0, brace)
    if (raw) out.push({ name: raw, required })
  }
  return out
}

/**
 * 解析目录自带的 parameters 原始 JSON → { 参数名: 中文说明 }。
 * 容错两种形态：对象（{ name: "说明" | { description: "说明" } }）与字符串数组。
 * 非法 JSON / 非预期结构一律返回 {}（缺说明不报错）。
 */
export function parseParameterDescriptions(raw: string): Record<string, string> {
  if (!raw) return {}
  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return {}
  }
  const out: Record<string, string> = {}
  if (Array.isArray(parsed)) {
    for (const item of parsed) {
      if (typeof item === 'string') {
        out[item] = ''
      } else if (item && typeof item === 'object') {
        const rec = item as Record<string, unknown>
        if (typeof rec.name === 'string' && rec.name) {
          out[rec.name] = typeof rec.description === 'string' ? rec.description : ''
        }
      }
    }
    return out
  }
  if (parsed && typeof parsed === 'object') {
    for (const [key, val] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof val === 'string') {
        out[key] = val
      } else if (val && typeof val === 'object') {
        const rec = val as Record<string, unknown>
        out[key] = typeof rec.description === 'string' ? rec.description : ''
      } else {
        out[key] = ''
      }
    }
  }
  return out
}

/** 合并 path 参数段与目录说明，产出填参表单的字段列表（必填在前）。 */
export function buildRouteParamSpecs(path: string, rawParameters: string): RouteParamSpec[] {
  const descriptions = parseParameterDescriptions(rawParameters)
  return parsePathParams(path)
    .map(p => ({ ...p, description: descriptions[p.name] ?? '' }))
    .sort((a, b) => Number(b.required) - Number(a.required))
}
