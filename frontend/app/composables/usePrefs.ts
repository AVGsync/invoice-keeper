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

  return { prefs, reduceMotion }
}
