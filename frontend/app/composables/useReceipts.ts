import type { Receipt } from '~/types/api'

// receipts of payment periods, kept per period once loaded (cleared on logout with the rest).
// the table asks for the rows on screen, a few at a time, so it can show how many each one has
export function useReceipts() {
  const api = useApi()
  const byPeriod = useState<Record<string, Receipt[]>>('receipts', () => ({}))

  async function load(periodId: string) {
    const list = await api.receipts.list(periodId)
    byPeriod.value = { ...byPeriod.value, [periodId]: list.items ?? [] }
  }

  // loads once per period; the table calls it for every row it shows
  const queued = new Set<string>()
  function ensure(periodId: string) {
    if (byPeriod.value[periodId] || queued.has(periodId)) return
    queued.add(periodId)
    // a failed list just shows no count, the popover tries again when opened
    queue.push(() => load(periodId).catch(() => {}).finally(() => queued.delete(periodId)))
    pump()
  }

  async function upload(periodId: string, file: File) {
    await api.receipts.upload(periodId, file)
    await load(periodId)
  }

  async function addLink(periodId: string, url: string, title = '') {
    await api.receipts.addLink(periodId, { url, title: title || undefined })
    await load(periodId)
  }

  async function remove(periodId: string, receipt: Receipt) {
    await api.receipts.remove(receipt.id!)
    await load(periodId)
  }

  // a file opens in a new tab (pdf and pictures show there, the rest downloads), a link goes to its site.
  // the tab is opened right on the click, otherwise the browser blocks it as a popup
  async function open(receipt: Receipt) {
    const tab = window.open('', '_blank')
    try {
      const access = await api.receipts.access(receipt.id!)
      if (!access.url) throw new Error('no url')
      // a file comes as /api/v1/receipt-files/…: the blank tab needs the full address of this site
      const url = new URL(access.url, window.location.origin).href
      if (tab) {
        tab.opener = null
        tab.location.href = url
      }
      else {
        window.location.href = url
      }
    }
    catch (e) {
      tab?.close()
      throw e
    }
  }

  return { byPeriod, load, ensure, upload, addLink, remove, open }
}

// at most this many receipt lists are asked for at once, the rest wait their turn
const PARALLEL = 4
const queue: (() => Promise<void>)[] = []
let running = 0

function pump() {
  while (running < PARALLEL && queue.length) {
    const job = queue.shift()!
    running++
    job().finally(() => {
      running--
      pump()
    })
  }
}
