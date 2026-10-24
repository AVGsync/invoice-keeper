import type { PaymentSearchParams, RecurrentPayment, RecurringSearchParams, User } from '~/types/api'
import type { Currency, Period, Person, Template } from '~/utils/periods'

export interface NewPayment {
  title: string
  // in rubles / dollars / euros, not kopecks
  amount: number
  currency: Currency
  dueIso: string
  frequency: 'weekly' | 'monthly' | 'quarterly' | 'yearly'
  assigneeId?: string
}

// someone a payment can be assigned to, for the pickers and the filter
export interface Assignee {
  id?: string
  person: Person
}

// payments every screen reads, from the server
export function usePayments() {
  const api = useApi()
  const { user } = useAuth()
  const { isAdmin } = useRole()

  // recurring: the server's templates as they came, search results need them for the periods' frequency
  const live = useState('payments-live', () => ({ periods: [] as Period[], templates: [] as Template[], users: [] as User[], recurring: [] as RecurrentPayment[] }))
  // false until the first answer, the screens show "loading" meanwhile
  const loaded = useState('payments-loaded', () => false)
  // when the list last came from the server, for the footer
  const loadedAt = useState<Date | undefined>('payments-loaded-at', () => undefined)
  const loading = useState('payments-loading', () => false)
  const loadError = useState('payments-error', () => '')
  // a failed action (mark paid, stop...), shown as a note in the corner
  const actionError = useState('payments-action-error', () => '')

  const today = computed(() => todayIso())
  const periods = computed(() => live.value.periods)
  const templates = computed(() => live.value.templates)
  // a failed first load is not «loading» any more: the screens show the error instead
  const pending = computed(() => !loaded.value && !loadError.value)
  // numbers (totals, counts) are only real once a load went through, until then they stay grey
  const ready = computed(() => loaded.value)

  // the person logged in
  const meName = computed(() => user.value?.full_name ?? '')

  const assignees = computed<Assignee[]>(() => {
    // only the admin can list users. a user gets everyone who has a payment
    if (live.value.users.length) {
      return live.value.users
        .filter(item => item.role !== 'admin')
        .map(item => ({ id: item.id, person: personOf(item.full_name, item.persona?.name) }))
    }
    const names = [...new Set(live.value.periods.map(period => period.who.name))].filter(Boolean)
    return names.map((name) => {
      const period = live.value.periods.find(item => item.who.name === name)!
      return { id: period.userId, person: period.who }
    })
  })

  // the assignee or the admin can mark a payment
  function canMark(period: Period) {
    if (isAdmin.value) return true
    return !!user.value?.id && period.userId === user.value.id
  }

  async function load() {
    if (loading.value) return
    loading.value = true
    const session = sessionGeneration()
    try {
      const [occurrences, recurring, users] = await Promise.all([
        api.occurrences.list(),
        api.recurring.list(),
        // users (for personas and the assignee picker) are admin only
        isAdmin.value ? api.users.list().catch(() => ({ items: [] })) : Promise.resolve({ items: [] as User[] }),
      ])
      // logged out while this was on its way: it's the previous person's data, drop it
      if (session !== sessionGeneration()) return
      const userList = users.items ?? []
      const personaOf = (id?: string) => personaFrom(userList, id)
      const byId = new Map<string, RecurrentPayment>((recurring ?? []).map(item => [item.id ?? '', item]))

      live.value = {
        periods: (occurrences.items ?? [])
          .map(item => toPeriod(item, byId.get(item.recurrent_payment_id ?? ''), personaOf(item.user_id), today.value))
          .sort((a, b) => fromRu(a.due).localeCompare(fromRu(b.due))),
        templates: (recurring ?? []).map(item => toTemplate(item, personaOf(item.assignee_id ?? undefined), today.value)),
        users: userList,
        recurring: recurring ?? [],
      }
      loadError.value = ''
      loaded.value = true
      loadedAt.value = new Date()
    }
    catch (e) {
      if (session === sessionGeneration()) loadError.value = apiError(e)
    }
    finally {
      loading.value = false
    }
  }

  // the persona (job title) of a user: yours from your profile, others' from the users list (admin only)
  function personaFrom(userList: User[], id?: string) {
    if (id && id === user.value?.id) return user.value?.persona?.name ?? ''
    return userList.find(item => item.id === id)?.persona?.name ?? ''
  }

  // the server filters, sorts and pages (GET /payment-occurrences/search). the rows come out
  // the same as in the full list, so the tables don't care where they came from
  async function searchPeriods(params: PaymentSearchParams) {
    const res = await api.occurrences.search(params)
    const byId = new Map<string, RecurrentPayment>(live.value.recurring.map(item => [item.id ?? '', item]))
    return {
      periods: (res.items ?? []).map(item => toPeriod(item, byId.get(item.recurrent_payment_id ?? ''), personaFrom(live.value.users, item.user_id), today.value)),
      total: res.total ?? 0,
    }
  }

  // the same for recurring payments (GET /recurring-payments/search). the server knows the next unpaid date
  async function searchTemplates(params: RecurringSearchParams) {
    const res = await api.recurring.search(params)
    return {
      templates: (res.items ?? []).map((item) => {
        const template = toTemplate(item, personaFrom(live.value.users, item.assignee_id ?? undefined), today.value)
        if (item.next_due_date !== undefined) template.nextDue = item.next_due_date ? toRu(item.next_due_date) : undefined
        return template
      }),
      total: res.total ?? 0,
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
    await run('Не удалось отметить оплату', () => api.occurrences.patch(period.id, { action: 'confirm', paid_date: paidIso }))
  }

  async function unmark(period: Period) {
    await run('Не удалось снять отметку', () => api.occurrences.patch(period.id, { action: 'cancel' }))
  }

  async function stop(template: Template) {
    await run('Не удалось прекратить платёж', () => api.recurring.stop(template.id))
  }

  async function add(input: NewPayment) {
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
    today, periods, templates, assignees, meName, pending, ready, loading, loadError, actionError, loadedAt,
    canMark, load, searchPeriods, searchTemplates, markPaid, unmark, stop, add,
  }
}
