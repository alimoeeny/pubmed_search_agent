export type TextDeltaEvent = {
  type: 'text_delta'
  content: string
  partial: boolean
}

export type AskUserEvent = {
  type: 'ask_user'
  call_id: string
  question: string
  options?: string[]
}

export type PDFReadyEvent = {
  type: 'pdf_ready'
  download_url: string
}

export type DoneEvent = {
  type: 'done'
}

export type ErrorEvent = {
  type: 'error'
  message: string
}

export type UserMessageEvent = {
  type: 'user_message'
  content: string
}

export type SSEEvent = TextDeltaEvent | UserMessageEvent | AskUserEvent | PDFReadyEvent | DoneEvent | ErrorEvent
