// personal settings, kept in this browser (localStorage) and applied as attributes on <html>:
//   data-theme="light" | "dark", data-motion="reduced", data-font="readable"
// main.css reacts to those attributes

export interface Prefs {
  theme: 'dark' | 'light'
  lessMotion: boolean
  // Andika: clearer letter shapes, easier with dyslexia, has Cyrillic
  readableFont: boolean
  // demo mode: actions change the demo data in the browser until reload
  demoMode: boolean
}

const KEY = 'invoice-keeper:prefs'
const defaults: Prefs = { theme: 'dark', lessMotion: false, readableFont: false, demoMode: false }

function load(): Prefs {
  try {
    return { ...defaults, ...JSON.parse(localStorage.getItem(KEY) ?? '{}') }
  }
  catch {
    return { ...defaults }
  }
}

const READABLE_FONT_URL = 'https://fonts.googleapis.com/css2?family=Andika:ital,wght@0,400;0,700;1,400&display=swap'

export function applyPrefs(prefs: Prefs) {
  const root = document.documentElement
  root.dataset.theme = prefs.theme
  if (prefs.lessMotion) root.dataset.motion = 'reduced'
  else delete root.dataset.motion
  if (prefs.readableFont) root.dataset.font = 'readable'
  else delete root.dataset.font

  // the readable font is only downloaded once somebody turns it on
  if (prefs.readableFont && !document.getElementById('readable-font')) {
    const link = Object.assign(document.createElement('link'), { id: 'readable-font', rel: 'stylesheet', href: READABLE_FONT_URL })
    document.head.appendChild(link)
  }
}

export function savePrefs(prefs: Prefs) {
  try {
    localStorage.setItem(KEY, JSON.stringify(prefs))
  }
  catch {
    // private mode etc: the setting just won't survive a reload
  }
}

export function usePrefs() {
  const prefs = useState<Prefs>('prefs', () => (import.meta.client ? load() : { ...defaults }))

  // also true when the system asks for less motion
  const reduceMotion = computed(() => prefs.value.lessMotion
    || (import.meta.client && matchMedia('(prefers-reduced-motion: reduce)').matches))

  // the new theme spreads out in a circle from the point that was clicked
  async function switchTheme(theme: Prefs['theme'], from?: { x: number, y: number }) {
    if (theme === prefs.value.theme) return
    const set = () => {
      prefs.value.theme = theme
      applyPrefs(prefs.value)
    }
    if (reduceMotion.value || !document.startViewTransition) return set()

    const root = document.documentElement
    // named blocks (table rows etc) would fade on their own instead of being uncovered with the rest
    root.classList.add('theme-switch')
    const x = from?.x ?? innerWidth / 2
    const y = from?.y ?? innerHeight / 2
    const radius = Math.hypot(Math.max(x, innerWidth - x), Math.max(y, innerHeight - y))
    const transition = document.startViewTransition(set)
    await transition.ready
    root.animate(
      { clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${radius}px at ${x}px ${y}px)`] },
      { duration: 650, easing: 'cubic-bezier(0.65, 0, 0.35, 1)', pseudoElement: '::view-transition-new(root)' },
    )
    await transition.finished
    root.classList.remove('theme-switch')
  }

  return { prefs, reduceMotion, switchTheme }
}
