import { describe, it, expect } from 'vitest'
import { resolveTheme, mainButtonColors } from './theme'

describe('resolveTheme', () => {
  it('берёт colorScheme Telegram, когда он задан', () => {
    expect(resolveTheme({ colorScheme: 'light' }, false)).toBe('light')
    expect(resolveTheme({ colorScheme: 'dark' }, true)).toBe('dark')
  })

  it('без Telegram — по системной настройке', () => {
    expect(resolveTheme(null, true)).toBe('light')
    expect(resolveTheme(null, false)).toBe('dark')
  })

  it('мусорный colorScheme → системная настройка', () => {
    expect(resolveTheme({ colorScheme: 'auto' }, true)).toBe('light')
    expect(resolveTheme({ colorScheme: '' }, false)).toBe('dark')
  })
})

describe('mainButtonColors', () => {
  it('dark: активна/загрузка — белая, неактивна — серая', () => {
    expect(mainButtonColors('dark', { enabled: true, loading: false })).toEqual({
      color: '#FFFFFF',
      text_color: '#161616',
    })
    expect(mainButtonColors('dark', { enabled: false, loading: true })).toEqual({
      color: '#FFFFFF',
      text_color: '#161616',
    })
    expect(mainButtonColors('dark', { enabled: false, loading: false })).toEqual({
      color: '#262626',
      text_color: '#686868',
    })
  })

  it('light: активна — чёрная, неактивна — светло-серая', () => {
    expect(mainButtonColors('light', { enabled: true, loading: false })).toEqual({
      color: '#0A0A0A',
      text_color: '#FFFFFF',
    })
    expect(mainButtonColors('light', { enabled: false, loading: false })).toEqual({
      color: '#E5E5E5',
      text_color: '#8A8A8A',
    })
  })
})
