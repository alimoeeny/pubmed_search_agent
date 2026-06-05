import ReactMarkdown from 'react-markdown'
import { cn } from '@/lib/utils'
import { stripPDFDownloadLinks, type MessageGroup } from '@/lib/chat-events'

export function MessageBubble({
  role,
  text,
  streaming = false,
}: MessageGroup & { streaming?: boolean }) {
  const isUser = role === 'user'
  const displayText = stripPDFDownloadLinks(text)

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
            <ReactMarkdown>{displayText}</ReactMarkdown>
            {streaming && (
              <span className="ml-0.5 inline-block h-4 w-0.5 animate-pulse bg-current align-middle" />
            )}
          </div>
        )}
      </div>
    </div>
  )
}
