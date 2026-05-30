import ReactMarkdown from 'react-markdown'
import { cn } from '@/lib/utils'
import type { SSEEvent } from '@/lib/types'

type Role = 'user' | 'agent'

type MessageGroup = {
  role: Role
  text: string
  pdfUrl?: string
}

function groupEvents(events: SSEEvent[]): MessageGroup[] {
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
      if (current?.role === 'agent') {
        current.pdfUrl = event.download_url
      } else {
        current = { role: 'agent', text: '', pdfUrl: event.download_url }
        groups.push(current)
      }
    } else {
      current = null
    }
  }

  return groups
}

export function MessageBubble({
  role,
  text,
  pdfUrl,
  streaming = false,
}: MessageGroup & { streaming?: boolean }) {
  const isUser = role === 'user'

  return (
    <div className={cn('flex', isUser ? 'justify-end' : 'justify-start')}>
      <div
        className={cn(
          'max-w-[80%] rounded-2xl px-4 py-3 text-sm',
          isUser
            ? 'rounded-br-sm bg-primary text-primary-foreground'
            : 'rounded-bl-sm bg-muted text-foreground',
        )}
      >
        {isUser ? (
          <p className="whitespace-pre-wrap">{text}</p>
        ) : (
          <div className="prose prose-sm dark:prose-invert max-w-none">
            <ReactMarkdown>{text}</ReactMarkdown>
            {streaming && (
              <span className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-current align-middle" />
            )}
          </div>
        )}
        {pdfUrl && (
          <a
            href={pdfUrl}
            target="_blank"
            rel="noopener noreferrer"
            className={cn(
              'mt-2 block rounded-lg border px-3 py-2 text-xs font-medium transition-colors',
              isUser
                ? 'border-primary-foreground/30 hover:bg-primary-foreground/10'
                : 'border-border hover:bg-accent',
            )}
          >
            Download PDF report
          </a>
        )}
      </div>
    </div>
  )
}

export { groupEvents, type MessageGroup }
