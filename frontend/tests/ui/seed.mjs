// fills the live server with the sample personas and users from the design, so screens aren't empty.
// safe to run again: existing ones are skipped. passwords of newly created users go to SAMPLE_USERS.md
// (gitignored, like every *.md here except README).
//   npm run seed
// env: UI_API (default https://77.239.105.84/api/v1), UI_ADMIN_LOGIN, UI_ADMIN_PASSWORD
import { request } from 'playwright'
import { appendFileSync, existsSync, writeFileSync } from 'node:fs'
import { usePasswordGenerator } from '../../app/composables/usePasswordGenerator.ts'

const API = process.env.UI_API ?? 'https://77.239.105.84/api/v1'
const ADMIN = { login: process.env.UI_ADMIN_LOGIN ?? 'admin', password: process.env.UI_ADMIN_PASSWORD ?? 'admin123' }
const OUT = new URL('../../SAMPLE_USERS.md', import.meta.url).pathname

const personas = [
  { name: 'Бухгалтер', description: 'Оплачивает аренду и подписки' },
  { name: 'DevOps', description: 'Платит за серверы и домены' },
  { name: 'Руководитель', description: 'Контроль расходов' },
]

const users = [
  { full_name: 'Ольга Иванова', login: 'olga', persona: 'Бухгалтер' },
  { full_name: 'Анна Смирнова', login: 'anna', persona: 'Руководитель' },
  { full_name: 'Дмитрий Ким', login: 'dima', persona: 'DevOps' },
  { full_name: 'Сергей Орлов', login: 'sergey' },
]

const api = await request.newContext({ baseURL: `${API}/` })
const { generate } = usePasswordGenerator()

async function call(method, path, data) {
  const res = await api.fetch(path, { method, data })
  if (!res.ok()) throw new Error(`${method} ${path}: ${res.status()} ${(await res.text()).trim()}`)
  return res.status() === 204 || !(await res.text()) ? null : res.json()
}

await call('POST', 'auth/login', ADMIN)

const existingPersonas = (await call('GET', 'personas')).items
const personaId = {}
for (const persona of personas) {
  const found = existingPersonas.find(p => p.name.toLowerCase() === persona.name.toLowerCase())
  personaId[persona.name] = found ? found.id : (await call('POST', 'personas', persona)).id
  console.log(`${found ? 'skip ' : 'added'} persona ${persona.name}`)
}

const existingLogins = new Set((await call('GET', 'users')).items.map(u => u.login))
const created = []
for (const user of users) {
  if (existingLogins.has(user.login)) {
    console.log(`skip  user ${user.login}`)
    continue
  }
  const password = generate()
  await call('POST', 'users', {
    full_name: user.full_name,
    login: user.login,
    password,
    ...(user.persona ? { persona_id: personaId[user.persona] } : {}),
  })
  created.push({ ...user, password })
  console.log(`added user ${user.login}`)
}

await call('POST', 'auth/logout')
await api.dispose()

if (created.length) {
  if (!existsSync(OUT)) writeFileSync(OUT, '# Sample users on the live server\n\nCreated by `npm run seed`. Local only, don\'t commit.\n\n| ФИО | Логин | Пароль | Персона |\n| --- | ----- | ------ | ------- |\n')
  for (const u of created) appendFileSync(OUT, `| ${u.full_name} | ${u.login} | \`${u.password}\` | ${u.persona ?? '—'} |\n`)
  console.log(`\npasswords saved to ${OUT}`)
}
