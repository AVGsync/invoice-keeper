import type { MiniUser, PaymentResponse } from '../api/types'

// Та же логика, что checkActor на бэкенде: отметить оплату может
// ответственный за период или администратор.
export function canMarkPaid(
  user: Pick<MiniUser, 'id' | 'role'> | null,
  occ: Pick<PaymentResponse, 'user_id'>,
): boolean {
  if (!user) return false
  return user.role === 'admin' || occ.user_id === user.id
}

export function canAdmin(user: Pick<MiniUser, 'role'> | null): boolean {
  return !!user && user.role === 'admin'
}
