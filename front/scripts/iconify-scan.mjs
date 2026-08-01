/**
 * Shared icon-name extraction for the iconify subset pipeline.
 *
 * Consumed by:
 *  - scripts/generate-icon-subset.mjs (generator)
 *  - app/assets/iconify-subset.test.ts (consistency check)
 *
 * Both sides call the same functions so the extraction rule can never drift
 * between generation and verification.
 */
import { existsSync, readdirSync, readFileSync } from 'node:fs'
import { extname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

// "prefix:name" — lowercase letters / digits / hyphens only.
const ICON_NAME_RE = /^([a-z0-9-]+):([a-z0-9-]+)$/
// Quoted string literal (double / single / backtick), escape-aware.
const QUOTED_RE = /(["'`])(?:\\.|(?!\1)[^\\])*\1/g

const SOURCE_EXTS = new Set(['.vue', '.ts'])
const SKIP_DIRS = new Set(['node_modules', '_deprecated'])
// Guard against pathological nesting when recursing into attribute values
// like :icon="'mdi:rss'". Depth shrinks by at least one quote pair per level.
const MAX_NESTING_DEPTH = 4

function quotedContents(source) {
  const out = []
  for (const match of source.matchAll(QUOTED_RE)) {
    out.push(match[0].slice(1, -1))
  }
  return out
}

function extractInto(source, mdi, other, depth = 0) {
  if (depth > MAX_NESTING_DEPTH) return
  for (const content of quotedContents(source)) {
    const match = content.match(ICON_NAME_RE)
    if (match) {
      ;(match[1] === 'mdi' ? mdi : other).add(content)
      continue
    }
    // Nested literal inside a larger string (e.g. :icon="'mdi:rss'").
    extractInto(content, mdi, other, depth + 1)
  }
}

/** Recursively collect .vue / .ts source files under dir. */
export function collectSourceFiles(dir, out = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (!SKIP_DIRS.has(entry.name)) collectSourceFiles(p, out)
    } else if (SOURCE_EXTS.has(extname(entry.name))) {
      out.push(p)
    }
  }
  return out
}

/**
 * Extract iconify icon names from one source string.
 * @returns {{ mdi: string[], other: string[] }} mdi names (the ones we can
 * localize today) plus any other "prefix:name" lookalikes (Tailwind classes,
 * Vue event names, times, future non-mdi icons) for visibility.
 */
export function extractIconNames(source) {
  const mdi = new Set()
  const other = new Set()
  extractInto(source, mdi, other)
  return { mdi: [...mdi].sort(), other: [...other].sort() }
}

// Resolve the app dir relative to this script. Under plain node (generator)
// import.meta.url is a file:// URL; under vitest it is an http(s) URL served
// by Vite, so fileURLToPath throws — fall back to the cwd (vitest and the
// pnpm scripts both run from the front/ root).
function defaultAppDir() {
  try {
    const fromMeta = fileURLToPath(new URL('../app', import.meta.url))
    if (existsSync(fromMeta)) return fromMeta
  } catch {
    // non-file scheme (vitest) — fall through to cwd
  }
  return resolve(process.cwd(), 'app')
}

function defaultSubsetPath() {
  try {
    const fromMeta = fileURLToPath(new URL('../app/assets/iconify-subset.json', import.meta.url))
    if (existsSync(fromMeta)) return fromMeta
  } catch {
    // non-file scheme (vitest) — fall through to cwd
  }
  return resolve(process.cwd(), 'app/assets/iconify-subset.json')
}

/**
 * Scan every source file under the app dir (defaults to ../app relative to
 * this script) and return the union of icon names.
 */
export function scanAppIconNames(appDir = defaultAppDir()) {
  const files = collectSourceFiles(appDir)
  const mdi = new Set()
  const other = new Set()
  for (const file of files) {
    const result = extractIconNames(readFileSync(file, 'utf8'))
    for (const name of result.mdi) mdi.add(name)
    for (const name of result.other) other.add(name)
  }
  return {
    mdi: [...mdi].sort(),
    other: [...other].sort(),
    fileCount: files.length,
  }
}

/** Read the generated subset artifact (defaults to app/assets/iconify-subset.json). */
export function readIconSubset(jsonPath = defaultSubsetPath()) {
  return JSON.parse(readFileSync(jsonPath, 'utf8'))
}
