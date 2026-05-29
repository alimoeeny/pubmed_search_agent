import { useCallback, useEffect, useReducer, useRef } from 'react'
import { useApi } from '@/lib/api-context'
import { parseSSE } from '@/lib/sse'
import type { AskUserEvent, SSEEvent } from '@/lib/types'

export type StreamStatus = 'idle' | 'streaming' | 'awaiting_user' | 'error'

type StreamState = {
  history: SSEEvent[]
  events: SSEEvent[]
  status: StreamStatus
  pendingAskUser: AskUserEvent | null
  errorMessage: string | null
}

type StreamAction =
  | { type: 'HYDRATE'; events: SSEEvent[] }
  | { type: 'STREAM_START' }
  | { type: 'EVENT'; event: SSEEvent }
  | { type: 'TURN_DONE' }
  | { type: 'ERROR'; message: string }
  | { type: 'RESET_TURN' }

const initial: StreamState = {
  history: [],
  events: [],
  status: 'idle',
  pendingAskUser: null,
  errorMessage: null,
}

function reducer(state: StreamState, action: StreamAction): StreamState {
  switch (action.type) {
    case 'HYDRATE':
      return { ...state, history: action.events }

    case 'STREAM_START':
      return { ...state, status: 'streaming', events: [], pendingAskUser: null, errorMessage: null }

    case 'EVENT': {
      const event = action.event
      if (event.type === 'ask_user') {
        return {
          ...state,
          status: 'awaiting_user',
          pendingAskUser: event,
          events: [...state.events, event],
        }
      }
      if (event.type === 'error') {
        return { ...state, status: 'error', errorMessage: event.message, events: [...state.events, event] }
      }
      if (event.type === 'done') {
        return { ...state, status: 'idle', history: [...state.history, ...state.events], events: [] }
      }
      return { ...state, events: [...state.events, event] }
    }

    case 'TURN_DONE':
      return { ...state, status: 'idle', history: [...state.history, ...state.events], events: [] }

    case 'ERROR':
      return { ...state, status: 'error', errorMessage: action.message }

    case 'RESET_TURN':
      return { ...state, events: [], status: 'idle', pendingAskUser: null, errorMessage: null }

    default:
      return state
  }
}

export type UseStreamResult = {
  history: SSEEvent[]
  events: SSEEvent[]
  status: StreamStatus
  pendingAskUser: AskUserEvent | null
  errorMessage: string | null
  sendMessage: (text: string) => void
  sendFunctionResponse: (callId: string, result: string) => void
  abort: () => void
}

export function useStream(sessionId: string): UseStreamResult {
  const api = useApi()
  const [state, dispatch] = useReducer(reducer, initial)
  const abortRef = useRef<AbortController | null>(null)
  const hydrateAbortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    hydrateAbortRef.current = controller

    async function hydrate() {
      try {
        const res = await api.streamSession(sessionId, controller.signal)
        if (!res.body) return
        const events: SSEEvent[] = []
        for await (const event of parseSSE(res.body, controller.signal)) {
          if (event.type !== 'done') events.push(event)
        }
        dispatch({ type: 'HYDRATE', events })
      } catch {
        // aborted or session has no history yet — silently ignore
      }
    }

    void hydrate()

    return () => controller.abort()
  }, [sessionId, api])

  const runStream = useCallback(
    async (body: Parameters<typeof api.postMessage>[1]) => {
      if (abortRef.current) abortRef.current.abort()
      const controller = new AbortController()
      abortRef.current = controller

      dispatch({ type: 'STREAM_START' })
      try {
        const res = await api.postMessage(sessionId, body, controller.signal)
        if (!res.body) {
          dispatch({ type: 'TURN_DONE' })
          return
        }
        let cleanEnd = false
        for await (const event of parseSSE(res.body, controller.signal)) {
          dispatch({ type: 'EVENT', event })
          if (event.type === 'done' || event.type === 'error') {
            cleanEnd = true
            break
          }
        }
        if (!cleanEnd) {
          dispatch({ type: 'TURN_DONE' })
        }
      } catch (err) {
        if ((err as { name?: string }).name !== 'AbortError') {
          dispatch({ type: 'ERROR', message: err instanceof Error ? err.message : 'Stream failed' })
        }
      }
    },
    [sessionId, api],
  )

  const sendMessage = useCallback(
    (text: string) => {
      void runStream({ text })
    },
    [runStream],
  )

  const sendFunctionResponse = useCallback(
    (callId: string, result: string) => {
      void runStream({
        function_responses: [{ name: 'ask_user', id: callId, response: { result } }],
      })
    },
    [runStream],
  )

  const abort = useCallback(() => {
    abortRef.current?.abort()
    dispatch({ type: 'RESET_TURN' })
  }, [])

  return {
    history: state.history,
    events: state.events,
    status: state.status,
    pendingAskUser: state.pendingAskUser,
    errorMessage: state.errorMessage,
    sendMessage,
    sendFunctionResponse,
    abort,
  }
}
