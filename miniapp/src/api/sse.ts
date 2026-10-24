// Инкрементальный разбор SSE.

export interface SseEvent {
  event: string
  data: string
}

export interface SseParser {
  push(text: string): void
  end(): void
}

export function createSseParser(onEvent: (e: SseEvent) => void): SseParser {
  let buffer = ''
  let eventName = ''
  let dataLines: string[] = []
  let hasData = false

  function dispatch(): void {
    if (hasData) {
      onEvent({ event: eventName || 'message', data: dataLines.join('\n') })
    }
    eventName = ''
    dataLines = []
    hasData = false
  }

  function handleLine(line: string): void {
    if (line === '') {
      dispatch()
      return
    }
    if (line.startsWith(':')) {
      return
    }
    const colon = line.indexOf(':')
    let field: string
    let value: string
    if (colon === -1) {
      field = line
      value = ''
    } else {
      field = line.slice(0, colon)
      value = line.slice(colon + 1)
      if (value.startsWith(' ')) value = value.slice(1)
    }
    if (field === 'event') {
      eventName = value
    } else if (field === 'data') {
      dataLines.push(value)
      hasData = true
    }
  }

  return {
    push(text: string): void {
      buffer += text
      // Если буфер заканчивается на '\r', это может быть начало CRLF, разорванного между чанками.
      let work = buffer
      let held = ''
      if (work.endsWith('\r')) {
        held = '\r'
        work = work.slice(0, -1)
      }
      // Разбиваем по любым переводам строки, последняя (неполная) остаётся в буфере.
      const parts = work.split(/\r\n|\n|\r/)
      buffer = (parts.pop() ?? '') + held
      for (const line of parts) handleLine(line)
    },
    end(): void {
      buffer = ''
    },
  }
}
