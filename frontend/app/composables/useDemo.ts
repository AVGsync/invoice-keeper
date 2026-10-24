import type { Currency, Period, Person, Template } from '~/utils/demo'

// the demo payments, used by usePayments() while demo mode is on (Настройки).
// the actions below change them in this browser only, until reload
export function useDemo() {
  const { prefs } = usePrefs()
  const enabled = computed(() => prefs.value.demoMode)

  // copies, so the seeds stay untouched for reset. `who` keeps pointing at the same person
  const fresh = () => ({
    periods: seedPeriods.map(period => ({ ...period })),
    templates: seedTemplates.map(template => ({ ...template })),
  })
  const data = useState('demo-data', fresh)

  const periods = computed(() => data.value.periods)
  const templates = computed(() => data.value.templates)

  const dueIso = (period: Period) => fromRu(period.due)

  // paid on time, late, overdue or still waiting: same rules as the case
  function settle(period: Period) {
    const due = dueIso(period)
    if (period.paidAt) {
      const paid = fromRu(period.paidAt)
      period.status = paid > due ? 'late' : 'paid'
      period.overdue = paid > due ? `+${daysBetween(due, paid)} дн.` : undefined
      return
    }
    const late = daysBetween(due, demoToday)
    period.status = late > 0 ? 'overdue' : 'pending'
    period.overdue = late > 0 ? `${late} дн.` : undefined
  }

  function markPaid(period: Period, paidIso: string) {
    period.paidAt = toRu(paidIso)
    settle(period)
  }

  function unmark(period: Period) {
    period.paidAt = undefined
    settle(period)
  }

  function stop(template: Template) {
    template.ended = true
    template.nextDue = undefined
  }

  function add(input: { title: string, amount: number, currency: Currency, dueIso: string, yearly: boolean, who?: Person }) {
    const amount = formatMoney(input.amount, input.currency)
    const frequency = input.yearly ? 'ежегодно' : 'ежемесячно'
    const who = input.who ?? people.olga
    const day = Number(input.dueIso.slice(8, 10))
    const daysLeft = daysBetween(demoToday, input.dueIso)

    const id = `demo-new-${Date.now()}`
    data.value.templates.unshift({
      id,
      title: input.title,
      category: 'Новый',
      amount,
      currency: input.currency,
      frequency,
      day: input.yearly ? `${day} ${monthNamesGenitive[Number(input.dueIso.slice(5, 7)) - 1]}` : `${day}-е число`,
      who,
      nextDue: toRu(input.dueIso),
    })

    const period: Period = {
      id: `${id}-p`,
      title: input.title,
      frequency,
      meta: `${frequency} · новый`,
      who,
      due: toRu(input.dueIso),
      relative: daysLeft === 1 ? 'завтра' : daysLeft > 1 ? `через ${daysLeft} дн.` : daysLeft < 0 ? `${-daysLeft} дн. назад` : 'сегодня',
      amount,
      currency: input.currency,
      status: 'pending',
    }
    settle(period)
    data.value.periods.push(period)
  }

  function reset() {
    data.value = fresh()
  }

  return { enabled, periods, templates, markPaid, unmark, stop, add, reset }
}
