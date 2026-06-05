import type { SSEEvent } from '@/lib/types'

type Role = 'user' | 'agent'

export type MessageGroup = {
  role: Role
  text: string
}

export function groupEvents(events: SSEEvent[]): MessageGroup[] {
  const groups: MessageGroup[] = []
  let current: MessageGroup | null = null

  for (const event of events) {
    if (event.type === 'text_delta') {
      if (current?.role === 'agent') {
        current.text += event.content
      } else {
        current = { role: 'agent', text: event.content }
        groups.push(current)
      }
    } else if (event.type === 'user_message') {
      current = { role: 'user', text: event.content }
      groups.push(current)
    } else if (event.type === 'pdf_ready') {
      continue
    } else {
      current = null
    }
  }

  return groups
}

export function stripPDFDownloadLinks(text: string): string {
  if (!text.includes('storage.googleapis.com') || !text.toLowerCase().includes('.pdf')) {
    return text
  }

  return text
    .split('\n')
    .filter((line) => {
      const lower = line.toLowerCase()
      return !(lower.includes('https://storage.googleapis.com/') && lower.includes('.pdf'))
    })
    .join('\n')
}
