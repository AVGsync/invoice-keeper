import { describe, it, expect } from 'vitest'
import { shouldSubmit } from './composer'

describe('shouldSubmit', () => {
  it('Enter без модификаторов → true', () => {
    expect(shouldSubmit({ key: 'Enter', shiftKey: false, isComposing: false, keyCode: 13 })).toBe(true)
  })
  it('Shift+Enter → false (перевод строки)', () => {
    expect(shouldSubmit({ key: 'Enter', shiftKey: true, isComposing: false, keyCode: 13 })).toBe(false)
  })
  it('во время IME-композиции → false', () => {
    expect(shouldSubmit({ key: 'Enter', shiftKey: false, isComposing: true, keyCode: 13 })).toBe(false)
  })
  it('keyCode 229 (IME) → false', () => {
    expect(shouldSubmit({ key: 'Enter', shiftKey: false, isComposing: false, keyCode: 229 })).toBe(false)
  })
  it('другая клавиша → false', () => {
    expect(shouldSubmit({ key: 'a', shiftKey: false, isComposing: false, keyCode: 65 })).toBe(false)
  })
})
