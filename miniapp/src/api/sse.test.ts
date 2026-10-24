import { describe, it, expect } from 'vitest'
import { createSseParser, type SseEvent } from './sse'

function collect(): { events: SseEvent[]; parser: ReturnType<typeof createSseParser> } {
  const events: SseEvent[] = []
  const parser = createSseParser((e) => events.push(e))
  return { events, parser }
}

describe('createSseParser', () => {
  it('один кадр, разбитый по всем границам чанков', () => {
    const frame = 'event: step\ndata: {"n":1}\n\n'
    for (let i = 1; i < frame.length; i++) {
      const { events, parser } = collect()
      parser.push(frame.slice(0, i))
      parser.push(frame.slice(i))
      expect(events).toEqual([{ event: 'step', data: '{"n":1}' }])
    }
  })

  it('\\r\\n переводы строк', () => {
    const { events, parser } = collect()
    parser.push('event: done\r\ndata: ok\r\n\r\n')
    expect(events).toEqual([{ event: 'done', data: 'ok' }])
  })

  it('CRLF, разорванный между чанками (\\r | \\n)', () => {
    const { events, parser } = collect()
    parser.push('event: done\r')
    parser.push('\ndata: ok\r\n\r\n')
    expect(events).toEqual([{ event: 'done', data: 'ok' }])
  })

  it('CRLF-кадр, разбитый по всем границам чанков', () => {
    const frame = 'event: step\r\ndata: {"n":1}\r\n\r\n'
    for (let i = 1; i < frame.length; i++) {
      const { events, parser } = collect()
      parser.push(frame.slice(0, i))
      parser.push(frame.slice(i))
      expect(events).toEqual([{ event: 'step', data: '{"n":1}' }])
    }
  })

  it('несколько data: склеиваются через \\n', () => {
    const { events, parser } = collect()
    parser.push('data: a\ndata: b\ndata: c\n\n')
    expect(events).toEqual([{ event: 'message', data: 'a\nb\nc' }])
  })

  it('комментарии игнорируются', () => {
    const { events, parser } = collect()
    parser.push(': keepalive\ndata: x\n\n')
    expect(events).toEqual([{ event: 'message', data: 'x' }])
  })

  it('два кадра в одном чанке', () => {
    const { events, parser } = collect()
    parser.push('event: step\ndata: 1\n\nevent: block\ndata: 2\n\n')
    expect(events).toEqual([
      { event: 'step', data: '1' },
      { event: 'block', data: '2' },
    ])
  })

  it('неполный хвост отбрасывается при end()', () => {
    const { events, parser } = collect()
    parser.push('event: step\ndata: 1\n\nevent: block\ndata: incomple')
    parser.end()
    expect(events).toEqual([{ event: 'step', data: '1' }])
  })

  it('многобайтовый UTF-8, разбитый на уровне байтов', () => {
    const { events, parser } = collect()
    const enc = new TextEncoder()
    const dec = new TextDecoder('utf-8')
    const bytes = enc.encode('data: привет\n\n')
    const mid = 8 // режем посреди кириллической буквы
    parser.push(dec.decode(bytes.slice(0, mid), { stream: true }))
    parser.push(dec.decode(bytes.slice(mid), { stream: true }))
    parser.push(dec.decode())
    expect(events).toEqual([{ event: 'message', data: 'привет' }])
  })
})
