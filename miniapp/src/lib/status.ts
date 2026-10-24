import type { OccStatus, PaymentResponse, AssistantBlockStatus } from '../api/types'

export interface StatusInfo {
  label: string
  colorVar: string
}

export function statusInfo(occ: Pick<PaymentResponse, 'status' | 'paid_late'>): StatusInfo {
  switch (occ.status) {
    case 'pending':
      return { label: 'Ожидает', colorVar: '--c-pending' }
    case 'overdue':
      return { label: 'Просрочен', colorVar: '--c-overdue' }
    case 'paid':
      return { label: occ.paid_late ? 'Оплачен с опозданием' : 'Оплачен', colorVar: '--c-paid' }
    case 'ended':
      return { label: 'Завершён', colorVar: '--c-ended' }
  }
}

// Статусы в блоках ассистента отличаются: upcoming ↔ pending.
export function fromAssistant(s: AssistantBlockStatus): OccStatus {
  if (s === 'upcoming') return 'pending'
  return s
}
