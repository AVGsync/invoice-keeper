// Данные-фикстуры для e2e. «Сегодня» в тестах = 2026-10-24 (Europe/Moscow).

export interface MockUser {
  id: string
  full_name: string
  login: string
  role: 'admin' | 'user'
  persona: { id: string; name: string } | null
  created_at: string
}

export interface MockOcc {
  id: string
  user_id: string
  recurrent_payment_id?: string
  amount: number
  currency: 'RUB' | 'USD' | 'EUR'
  description: string
  due_date: string
  paid_date?: string
  paid_late?: boolean
  status: 'pending' | 'overdue' | 'paid' | 'ended'
  assignee_name?: string
  ended?: boolean
  created_at: string
  updated_at: string
}

export const PERSONAS = [
  { id: 'aaaaaaaa-0000-4000-8000-000000000001', name: 'Бухгалтер', description: null, users_count: 1, created_at: '2026-01-01T00:00:00Z' },
  { id: 'aaaaaaaa-0000-4000-8000-000000000002', name: 'Менеджер', description: null, users_count: 1, created_at: '2026-01-01T00:00:00Z' },
]

export const USERS: Record<string, MockUser> = {
  admin: {
    id: '11111111-1111-4111-8111-111111111111',
    full_name: 'Админ Админов',
    login: 'admin',
    role: 'admin',
    persona: null,
    created_at: '2026-01-01T00:00:00Z',
  },
  user1: {
    id: '22222222-2222-4222-8222-222222222222',
    full_name: 'Иван Пользователь',
    login: 'user1',
    role: 'user',
    persona: { id: PERSONAS[0]!.id, name: PERSONAS[0]!.name },
    created_at: '2026-01-02T00:00:00Z',
  },
  user2: {
    id: '33333333-3333-4333-8333-333333333333',
    full_name: 'Пётр Второй',
    login: 'user2',
    role: 'user',
    persona: { id: PERSONAS[1]!.id, name: PERSONAS[1]!.name },
    created_at: '2026-01-03T00:00:00Z',
  },
  user3: {
    id: '44444444-4444-4444-8444-444444444444',
    full_name: 'Мария Третья',
    login: 'user3',
    role: 'user',
    persona: null,
    created_at: '2026-01-04T00:00:00Z',
  },
}

export const USER_LIST = Object.values(USERS)

// Серия из 4 периодов одного recurrent_payment_id для «Истории».
const SERIES_RP = '55555555-5555-4555-8555-555555555555'

function base(o: Partial<MockOcc> & Pick<MockOcc, 'id' | 'user_id' | 'amount' | 'currency' | 'description' | 'due_date' | 'status'>): MockOcc {
  return {
    assignee_name: USER_LIST.find((u) => u.id === o.user_id)?.full_name,
    created_at: '2026-09-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
    ...o,
  }
}

export const OCCURRENCES: MockOcc[] = [
  // overdue, RUB, mine (user1)
  base({ id: '00000000-0000-4000-8000-00000000a001', user_id: USERS.user1!.id, amount: 1500000, currency: 'RUB', description: 'Аренда офиса', due_date: '2026-10-19', status: 'overdue' }),
  // overdue, USD, mine
  base({ id: '00000000-0000-4000-8000-00000000a002', user_id: USERS.user1!.id, amount: 9900, currency: 'USD', description: 'Хостинг', due_date: '2026-10-20', status: 'overdue' }),
  // pending, RUB, mine, в окне 7 дней
  base({ id: '00000000-0000-4000-8000-00000000a003', user_id: USERS.user1!.id, amount: 300000, currency: 'RUB', description: 'Интернет', due_date: '2026-10-26', status: 'pending' }),
  // pending, EUR, mine
  base({ id: '00000000-0000-4000-8000-00000000a004', user_id: USERS.user1!.id, amount: 4500, currency: 'EUR', description: 'Подписка Figma', due_date: '2026-10-28', status: 'pending' }),
  // pending, RUB, чужой (user2)
  base({ id: '00000000-0000-4000-8000-00000000a005', user_id: USERS.user2!.id, amount: 1200000, currency: 'RUB', description: 'Бухгалтерия', due_date: '2026-10-27', status: 'pending' }),
  // paid, RUB, mine
  base({ id: '00000000-0000-4000-8000-00000000a006', user_id: USERS.user1!.id, amount: 500000, currency: 'RUB', description: 'Канцтовары', due_date: '2026-10-10', status: 'paid', paid_date: '2026-10-09' }),
  // paid late, USD, mine
  base({ id: '00000000-0000-4000-8000-00000000a007', user_id: USERS.user1!.id, amount: 15000, currency: 'USD', description: 'Домен', due_date: '2026-10-05', status: 'paid', paid_date: '2026-10-08', paid_late: true }),
  // ended, RUB, mine
  base({ id: '00000000-0000-4000-8000-00000000a008', user_id: USERS.user1!.id, amount: 700000, currency: 'RUB', description: 'Старый сервис', due_date: '2026-10-15', status: 'ended', ended: true }),
  // pending, EUR, чужой
  base({ id: '00000000-0000-4000-8000-00000000a009', user_id: USERS.user2!.id, amount: 20000, currency: 'EUR', description: 'Реклама', due_date: '2026-10-30', status: 'pending' }),
  // paid, RUB, чужой
  base({ id: '00000000-0000-4000-8000-00000000a010', user_id: USERS.user3!.id, amount: 250000, currency: 'RUB', description: 'Клининг', due_date: '2026-10-12', status: 'paid', paid_date: '2026-10-12' }),
  // Серия (recurrent_payment_id = SERIES_RP), 4 периода, mine
  base({ id: '00000000-0000-4000-8000-00000000b001', user_id: USERS.user1!.id, recurrent_payment_id: SERIES_RP, amount: 1000000, currency: 'RUB', description: 'Подписка CRM', due_date: '2026-07-15', status: 'paid', paid_date: '2026-07-15' }),
  base({ id: '00000000-0000-4000-8000-00000000b002', user_id: USERS.user1!.id, recurrent_payment_id: SERIES_RP, amount: 1000000, currency: 'RUB', description: 'Подписка CRM', due_date: '2026-08-15', status: 'paid', paid_date: '2026-08-16', paid_late: true }),
  base({ id: '00000000-0000-4000-8000-00000000b003', user_id: USERS.user1!.id, recurrent_payment_id: SERIES_RP, amount: 1000000, currency: 'RUB', description: 'Подписка CRM', due_date: '2026-09-15', status: 'paid', paid_date: '2026-09-15' }),
  base({ id: '00000000-0000-4000-8000-00000000b004', user_id: USERS.user1!.id, recurrent_payment_id: SERIES_RP, amount: 1000000, currency: 'RUB', description: 'Подписка CRM', due_date: '2026-10-25', status: 'pending' }),
]

export const RECURRING = [
  {
    id: SERIES_RP,
    amount: 1000000,
    currency: 'RUB',
    description: 'Подписка CRM',
    frequency: 'monthly',
    interval: 1,
    day_offset: 15,
    start_date: '2026-10-25',
    assignee_id: USERS.user1!.id,
    assignee_name: USERS.user1!.full_name,
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-01T00:00:00Z',
  },
  {
    id: '66666666-6666-4666-8666-666666666666',
    amount: 50000,
    currency: 'USD',
    description: 'VPN команды',
    frequency: 'weekly',
    interval: 2,
    day_offset: 2,
    start_date: '2026-10-27',
    assignee_id: USERS.user2!.id,
    assignee_name: USERS.user2!.full_name,
    created_at: '2026-07-01T00:00:00Z',
    updated_at: '2026-07-01T00:00:00Z',
  },
  {
    id: '77777777-7777-4777-8777-777777777777',
    amount: 300000,
    currency: 'RUB',
    description: 'Остановленная аренда',
    frequency: 'monthly',
    interval: 1,
    day_offset: 1,
    start_date: '2026-06-01',
    assignee_id: USERS.user3!.id,
    assignee_name: USERS.user3!.full_name,
    stopped_at: '2026-09-01T00:00:00Z',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-09-01T00:00:00Z',
  },
]

export const ASSISTANT_STATUS = {
  enabled: true,
  healthy: true,
  mode: 'rules',
  examples: ['Что просрочено?', 'Сколько платить в этом месяце?'],
}

// occ для startapp-теста (hex32 без дефисов):
export const START_OCC_HEX = '0000000000004000800000000000a003'
