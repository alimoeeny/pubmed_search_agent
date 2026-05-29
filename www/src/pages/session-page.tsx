import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Send, Square } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import { MessageBubble, groupEvents, type MessageGroup } from '@/components/chat/message-bubble'
import { AskUserPrompt } from '@/components/chat/ask-user-prompt'
import { useStream } from '@/hooks/use-stream'

export function SessionPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const sessionId = id!

  const { history, events, status, pendingAskUser, errorMessage, sendMessage, sendFunctionResponse, abort } =
    useStream(sessionId)

  const [draft, setDraft] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const isStreaming = status === 'streaming'
  const isAwaitingUser = status === 'awaiting_user'
  const canSend = draft.trim().length > 0 && !isStreaming && !isAwaitingUser

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [history, events])

  const handleSend = () => {
    if (!canSend) return
    const text = draft.trim()
    setDraft('')
    sendMessage(text)
    textareaRef.current?.focus()
  }

  const historyGroups: MessageGroup[] = groupEvents(history)
  const liveGroups: MessageGroup[] = groupEvents(events)

  return (
    <div className="flex h-[calc(100vh-3.5rem)] flex-col">
      <div className="flex items-center gap-2 border-b px-4 py-2">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => navigate('/')}
          aria-label="Back to dashboard"
        >
          <ArrowLeft className="size-4" />
        </Button>
        <span className="font-mono text-sm text-muted-foreground">…{sessionId.slice(-8)}</span>
      </div>

      <ScrollArea className="flex-1 px-4 py-6">
        {history.length === 0 && events.length === 0 && status === 'idle' && (
          <div className="flex h-full flex-col items-center justify-center gap-2 text-center text-muted-foreground">
            <p className="text-base font-medium">What would you like to research?</p>
            <p className="text-sm">Ask a question and the agent will search PubMed for you.</p>
          </div>
        )}

        {history.length === 0 && status === 'streaming' && events.length === 0 && (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        )}

        <div className="space-y-4 max-w-3xl mx-auto">
          {historyGroups.map((g, i) => (
            <MessageBubble key={i} {...g} />
          ))}

          {isStreaming && liveGroups.map((g, i) => (
            <MessageBubble key={`live-${i}`} {...g} streaming={i === liveGroups.length - 1} />
          ))}

          {isAwaitingUser && pendingAskUser && (
            <AskUserPrompt
              event={pendingAskUser}
              onAnswer={sendFunctionResponse}
            />
          )}

          {status === 'error' && errorMessage && (
            <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {errorMessage}
            </div>
          )}
        </div>
        <div ref={bottomRef} />
      </ScrollArea>

      <div className="border-t bg-background px-4 py-3">
        <div className="mx-auto flex max-w-3xl gap-2">
          <Textarea
            ref={textareaRef}
            rows={1}
            placeholder="Ask a research question…"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            disabled={isStreaming || isAwaitingUser}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                handleSend()
              }
            }}
            className="resize-none"
          />
          {isStreaming ? (
            <Button variant="outline" size="icon" onClick={abort} aria-label="Stop">
              <Square className="size-4" />
            </Button>
          ) : (
            <Button size="icon" onClick={handleSend} disabled={!canSend} aria-label="Send">
              <Send className="size-4" />
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
