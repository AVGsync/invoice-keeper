import type { RecurrentPayment, User } from '~/types/api'
import type { Currency, Period, Person, Template } from '~/utils/demo'

export interface NewPayment {
  title: string
  // in rubles / dollars / euros, not kopecks
  amount: number
  currency: Currency
  dueIso: string
  frequency: 'weekly' | 'monthly' | 'quarterly' | 'yearly'
  assigneeId?: string
  // demo mode keeps the person itself
  who?: Person
}

// someone a payment can be assigned to, for the pickers and the filter
export interface Assignee {
  id?: string
  person: Person
}

// payments every screen reads: the server's, or the demo ones when demo mode is on (Настройки)
export function usePayments() {
  const api = useApi()
  const demo = useDemo()
  const { user } = useAuth()
  const { isAdmin } = useRole()

  const live = useState('payments-live', () => ({ periods: [] as Period[], templates: [] as Template[], users: [] as User[] }))
  // false until the first answer, the screens show "loading" meanwhile
  const loaded = useState('payments-loaded', () => false)
  // when the list last came from the server, for the footer
  const loadedAt = useState<Date | undefined>('payments-loaded-at', () => undefined)
  const loading = useState('payments-loading', () => false)
  const loadError = useState('payments-error', () => '')
  // a failed action (mark paid, stop...), shown as a note in the corner
  const actionError = useState('payments-action-error', () => '')

  const isDemo = computed(() => demo.enabled.value)
  const today = computed(() => (isDemo.value ? demoToday : todayIso()))
  const periods = computed(() => (isDemo.value ? demo.periods.value : live.value.periods))
  const templates = computed(() => (isDemo.value ? demo.templates.value : live.value.templates))
  const pending = computed(() => !isDemo.value && !loaded.value)

  // the person logged in. the demo user screens pretend to be Ольга
  const meName = computed(() => (isDemo.value ? currentUser.name : user.value?.full_name ?? ''))

  const assignees = computed<Assignee[]>(() => {
    if (isDemo.value) return Object.values(people).map(person => ({ person }))
    // only the admin can list users. a user gets everyone who has a payment
    if (live.value.users.length) {
      return live.value.users
        .filter(item => item.role !== 'admin')
        .map(item => ({ id: item.id, person: personOf(item.full_name, item.persona?.name) }))
    }
    const names = [...new Set(live.value.periods.map(period => period.who.name))].filter(Boolean)
    return names.map(name => ({ person: live.value.periods.find(period => period.who.name === name)!.who }))
  })

  // the assignee or the admin can mark a payment
  function canMark(period: Period) {
    if (isDemo.value || isAdmin.value) return true
    return !!user.value?.id && period.userId === user.value.id
  }

  async function load() {
    if (isDemo.value || loading.value) return
    loading.value = true
    try {
      const [occurrences, recurring, users] = await Promise.all([
        api.occurrences.list(),
        api.recurring.list(),
        // users (for personas and the assignee picker) are admin only
        isAdmin.value ? api.users.list().catch(() => ({ items: [] })) : Promise.resolve({ items: [] as User[] }),
      ])
      const userList = users.items ?? []
      const personaOf = (id?: string) => {
        if (id && id === user.value?.id) return user.value?.persona?.name ?? ''
        return userList.find(item => item.id === id)?.persona?.name ?? ''
      }
      const byId = new Map<string, RecurrentPayment>((recurring ?? []).map(item => [item.id ?? '', item]))

      live.value = {
        periods: (occurrences.items ?? [])
          .map(item => toPeriod(item, byId.get(item.recurrent_payment_id ?? ''), personaOf(item.user_id), today.value))
          .sort((a, b) => fromRu(a.due).localeCompare(fromRu(b.due))),
        templates: (recurring ?? []).map(item => toTemplate(item, personaOf(item.assignee_id ?? undefined), today.value)),
        users: userList,
      }
      loadError.value = ''
      loaded.value = true
      loadedAt.value = new Date()
    }
    catch (e) {
      loadError.value = apiError(e)
    }
    finally {
      loading.value = false
    }
  }

  // every action reloads the list after it, so status, overdue days and totals come from the server.
  // the reload isn't awaited: a modal or popover can close as soon as the server said yes
  async function run(what: string, action: () => Promise<unknown>) {
    actionError.value = ''
    try {
      await action()
    }
    catch (e) {
      actionError.value = `${what}: ${apiError(e)}`
      throw e
    }
    finally {
      load()
    }
  }

  async function markPaid(period: Period, paidIso: string) {
    if (isDemo.value) return demo.markPaid(period, paidIso)
    await run('Не удалось отметить оплату', () => api.occurrences.patch(period.id, { action: 'confirm', paid_date: paidIso }))
  }

  async function unmark(period: Period) {
    if (isDemo.value) return demo.unmark(period)
    await run('Не удалось снять отметку', () => api.occurrences.patch(period.id, { action: 'cancel' }))
  }

  async function stop(template: Template) {
    if (isDemo.value) return demo.stop(template)
    await run('Не удалось прекратить платёж', () => api.recurring.stop(template.id))
  }

  async function add(input: NewPayment) {
    if (isDemo.value) {
      return demo.add({ ...input, yearly: input.frequency === 'yearly' })
    }
    const steps = { weekly: ['weekly', 1], monthly: ['monthly', 1], quarterly: ['monthly', 3], yearly: ['monthly', 12] } as const
    const [frequency, interval] = steps[input.frequency]
    await run('Не удалось добавить платёж', () => api.recurring.create({
      description: input.title,
      // the server keeps kopecks / cents
      amount: Math.round(input.amount * 100),
      currency: input.currency,
      frequency,
      interval,
      start_date: input.dueIso,
      assignee_id: input.assigneeId,
    }))
  }

  return {
    isDemo, today, periods, templates, assignees, meName, pending, loading, loadError, actionError, loadedAt,
    canMark, load, markPaid, unmark, stop, add, reset: demo.reset,
  }
}
