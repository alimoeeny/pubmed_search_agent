import { config } from '@/lib/config'

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export type SessionSummary = {
  session_id: string
  last_updated: string
}

export type FunctionResponse = {
  name: string
  id: string
  response: Record<string, unknown>
}

export type PostMessageBody =
  | { text: string; function_responses?: never }
  | { function_responses: FunctionResponse[]; text?: never }

async function request<T>(
  path: string,
  options: RequestInit & { token?: string },
): Promise<T> {
  const { token, ...init } = options
  const headers = new Headers(init.headers)
  headers.set('Content-Type', 'application/json')
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`${config.apiBaseUrl}${path}`, { ...init, headers })

  if (!res.ok) {
    let message = res.statusText
    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // ignore parse error
    }
    throw new ApiError(res.status, message)
  }

  if (res.status === 204) return undefined as T
  return res.json() as Promise<T>
}

export function createApiClient(getToken: () => string | undefined) {
  const token = () => getToken()

  return {
    createSession(): Promise<{ session_id: string }> {
      return request('/v1/sessions', { method: 'POST', token: token() })
    },

    listSessions(): Promise<{ sessions: SessionSummary[] }> {
      return request('/v1/sessions', { method: 'GET', token: token() })
    },

    deleteSession(id: string): Promise<void> {
      return request(`/v1/sessions/${id}`, { method: 'DELETE', token: token() })
    },

    async postMessage(
      id: string,
      body: PostMessageBody,
      signal: AbortSignal,
    ): Promise<Response> {
      const headers: Record<string, string> = { 'Content-Type': 'application/json' }
      const t = token()
      if (t) headers['Authorization'] = `Bearer ${t}`

      const res = await fetch(`${config.apiBaseUrl}/v1/sessions/${id}/messages`, {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
        signal,
      })
      if (!res.ok) throw new ApiError(res.status, res.statusText)
      return res
    },

    async streamSession(id: string, signal: AbortSignal): Promise<Response> {
      const headers: Record<string, string> = {}
      const t = token()
      if (t) headers['Authorization'] = `Bearer ${t}`

      const res = await fetch(`${config.apiBaseUrl}/v1/sessions/${id}/stream`, {
        method: 'GET',
        headers,
        signal,
      })
      if (!res.ok) throw new ApiError(res.status, res.statusText)
      return res
    },
  }
}

export type ApiClient = ReturnType<typeof createApiClient>
