import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import type { AskUserEvent } from '@/lib/types'

type Props = {
  event: AskUserEvent
  onAnswer: (callId: string, answer: string) => void
  disabled?: boolean
}

export function AskUserPrompt({ event, onAnswer, disabled }: Props) {
  const [selected, setSelected] = useState<string | null>(null)
  const [freeText, setFreeText] = useState('')

  const hasOptions = event.options && event.options.length > 0

  const submit = (value: string) => {
    if (!value.trim()) return
    onAnswer(event.call_id, value)
  }

  return (
    <div className="rounded-2xl rounded-bl-sm bg-muted px-4 py-3 text-sm">
      <p className="mb-3 font-medium">{event.question}</p>

      {hasOptions && (
        <div className="mb-3 flex flex-wrap gap-2">
          {event.options!.map((opt) => (
            <Button
              key={opt}
              size="sm"
              variant={selected === opt ? 'default' : 'outline'}
              disabled={disabled}
              onClick={() => {
                setSelected(opt)
                submit(opt)
              }}
            >
              {opt}
            </Button>
          ))}
        </div>
      )}

      {!hasOptions && (
        <div className="flex gap-2">
          <Textarea
            rows={2}
            placeholder="Type your answer…"
            value={freeText}
            onChange={(e) => setFreeText(e.target.value)}
            disabled={disabled}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault()
                submit(freeText)
              }
            }}
            className="resize-none"
          />
          <Button
            size="sm"
            disabled={disabled || !freeText.trim()}
            onClick={() => submit(freeText)}
          >
            Send
          </Button>
        </div>
      )}
    </div>
  )
}
