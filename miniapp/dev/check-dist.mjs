import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'

// postbuild-страж: в dist/** не должно быть следов dev-мока.
const FORBIDDEN = ['__TG_MOCK', 'DEV_TG_', 'telegram-mock']
const distDir = new URL('../dist', import.meta.url).pathname

function walk(dir) {
  const out = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) out.push(...walk(p))
    else out.push(p)
  }
  return out
}

let files
try {
  files = walk(distDir)
} catch {
  console.error('check-dist: нет каталога dist — сначала соберите проект')
  process.exit(1)
}

const hits = []
for (const f of files) {
  if (!/\.(js|css|html|map|svg|json)$/.test(f)) continue
  const text = readFileSync(f, 'utf-8')
  for (const token of FORBIDDEN) {
    if (text.includes(token)) hits.push(`${f}: найдено «${token}»`)
  }
}

if (hits.length) {
  console.error('check-dist: в сборке обнаружены dev-артефакты:\n' + hits.join('\n'))
  process.exit(1)
}
console.log('check-dist: dev-артефактов в dist нет')
