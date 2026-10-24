// Чистая логика композера «Спросить»: решает, отправлять ли по нажатию Enter.
// Enter без модификаторов отправляет; Shift+Enter — перевод строки; во время
// ввода через IME (isComposing или keyCode 229) Enter не отправляет.
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
