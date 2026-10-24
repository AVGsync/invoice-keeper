// "2026-10-23T17:32:30Z" -> "23.10.2026", in the app's timezone
export function formatDate(iso?: string): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('ru-RU', { timeZone: 'Europe/Moscow' })
}

// "Ольга Иванова" -> "ОИ", "Администратор" -> "АД"
export function initials(name?: string): string {
  const words = (name ?? '').trim().split(/\s+/).filter(Boolean)
  const letters = words.length > 1 ? words[0]![0]! + words[1]![0]! : (words[0] ?? '').slice(0, 2)
  return letters.toUpperCase()
}

// role from the server -> how the screens show it
export function roleName(role?: string): string {
  if (role === 'admin') return 'Администратор'
  if (role === 'user') return 'Пользователь'
  return role ?? ''
}
