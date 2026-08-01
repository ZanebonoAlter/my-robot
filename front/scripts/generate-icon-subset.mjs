/**
 * Generates app/assets/iconify-subset.json — the local mdi icon subset
 * registered at startup by app/plugins/iconify-local.ts so <Icon> renders
 * without fetching from api.iconify.design at runtime.
 *
 * Usage: pnpm generate:icons  (or: node scripts/generate-icon-subset.mjs)
 *
 * Steps:
 *  1. Scan app/ source for iconify icon names (same rule as the consistency
 *     test, via scripts/iconify-scan.mjs).
 *  2. Import each needed icon from @iconify-icons/mdi (per-icon ESM modules —
 *     this package version ships no single icons.json).
 *  3. Write the subset as an IconifyJSON collection.
 *
 * Run it again and commit the result whenever a new icon is added to the app.
 */
import { writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { scanAppIconNames } from './iconify-scan.mjs'

const outPath = fileURLToPath(new URL('../app/assets/iconify-subset.json', import.meta.url))

const { mdi, other, fileCount } = scanAppIconNames()

if (other.length > 0) {
  const examples = other.slice(0, 8).join(', ')
  console.warn(
    `[generate-icon-subset] skipped ${other.length} non-mdi iconify-looking name(s): ${examples}${other.length > 8 ? ', …' : ''}`,
  )
}

if (mdi.length === 0) {
  console.error('[generate-icon-subset] no icons found, aborting')
  process.exit(1)
}

const icons = {}
for (const name of mdi) {
  const iconName = name.slice('mdi:'.length)
  const mod = await import(`@iconify-icons/mdi/${iconName}.js`)
  icons[iconName] = mod.default
}

const subset = {
  prefix: 'mdi',
  icons,
  width: 24,
  height: 24,
}

writeFileSync(outPath, `${JSON.stringify(subset, null, 2)}\n`)
console.log(`[generate-icon-subset] scanned ${fileCount} files, wrote ${mdi.length} icons → ${outPath}`)
