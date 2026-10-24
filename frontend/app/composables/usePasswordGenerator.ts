// password for a new user, backend accepts 8-72 chars

const SETS = [
  'abcdefghijklmnopqrstuvwxyz',
  'ABCDEFGHIJKLMNOPQRSTUVWXYZ',
  '0123456789',
  '!@#$%^&*-_=+?',
]
const ALL = SETS.join('')

// random int from 0 to max-1, every number equally likely
// (Math.random isn't safe for passwords, and plain % would make some chars more common)
function randomInt(max: number): number {
  const limit = Math.floor(2 ** 32 / max) * max
  const buf = new Uint32Array(1)
  do crypto.getRandomValues(buf)
  while (buf[0]! >= limit)
  return buf[0]! % max
}

export function usePasswordGenerator() {
  function generate(length = 20): string {
    if (length < 8 || length > 72) throw new RangeError('password length must be 8-72')

    // at least one lowercase, uppercase, digit and symbol
    const chars = SETS.map(set => set[randomInt(set.length)]!)
    while (chars.length < length) chars.push(ALL[randomInt(ALL.length)]!)

    // shuffle, otherwise the first 4 chars always go lower, upper, digit, symbol
    for (let i = chars.length - 1; i > 0; i--) {
      const j = randomInt(i + 1)
      const tmp = chars[i]!
      chars[i] = chars[j]!
      chars[j] = tmp
    }
    return chars.join('')
  }

  return { generate }
}
