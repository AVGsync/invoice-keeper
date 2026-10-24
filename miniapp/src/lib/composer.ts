// Чистая логика композера «Спросить»: решает, отправлять ли по нажатию Enter.
export interface KeyInfo {
  key: string
  shiftKey: boolean
  isComposing: boolean
  keyCode: number
}

export function shouldSubmit(e: KeyInfo): boolean {
  if (e.key !== 'Enter') return false
  if (e.shiftKey) return false
  if (e.isComposing || e.keyCode === 229) return false
  return true
}
