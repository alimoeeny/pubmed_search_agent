import type { SSEEvent } from '@/lib/types'

function parseLine(line: string): SSEEvent | null {
  if (!line.startsWith('data: ')) return null
  const raw = line.slice(6).trim()
  if (!raw) return null

  let parsed: unknown
  try {
    parsed = JSON.parse(raw)
  } catch {
    return null
  }

  if (typeof parsed !== 'object' || parsed === null || !('type' in parsed)) return null

  const obj = parsed as Record<string, unknown>

  switch (obj['type']) {
    case 'text_delta':
      return {
        type: 'text_delta',
        content: typeof obj['content'] === 'string' ? obj['content'] : '',
        partial: Boolean(obj['partial']),
      }
    case 'ask_user':
      return {
        type: 'ask_user',
        call_id: typeof obj['call_id'] === 'string' ? obj['call_id'] : '',
        question: typeof obj['question'] === 'string' ? obj['question'] : '',
        options: Array.isArray(obj['options'])
          ? (obj['options'] as unknown[]).filter((o): o is string => typeof o === 'string')
          : undefined,
      }
    case 'pdf_ready':
      return {
        type: 'pdf_ready',
        download_url: typeof obj['download_url'] === 'string' ? obj['download_url'] : '',
      }
    case 'user_message':
      return {
        type: 'user_message',
        content: typeof obj['content'] === 'string' ? obj['content'] : '',
      }
    case 'done':
      return { type: 'done' }
    case 'error':
      return {
        type: 'error',
        message: typeof obj['message'] === 'string' ? obj['message'] : 'Unknown error',
      }
    default:
      return null
  }
}

export async function* parseSSE(
  body: ReadableStream<Uint8Array>,
  signal?: AbortSignal,
): AsyncGenerator<SSEEvent> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buf = ''

  try {
    while (true) {
      if (signal?.aborted) break

      const { done, value } = await reader.read()
      if (done) break

      buf += decoder.decode(value, { stream: true })

      const frames = buf.split('\n\n')
      buf = frames.pop() ?? ''

      for (const frame of frames) {
        for (const line of frame.split('\n')) {
          const event = parseLine(line)
          if (event) {
            yield event
            if (event.type === 'done' || event.type === 'error') return
          }
        }
      }
    }
  } finally {
    reader.releaseLock()
  }
}
