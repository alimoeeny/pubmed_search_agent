import { createContext, useContext, useMemo } from 'react'
import { createApiClient, type ApiClient } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'

const ApiContext = createContext<ApiClient | null>(null)

export function ApiProvider({ children }: { children: React.ReactNode }) {
  const { session } = useAuth()

  const client = useMemo(
    () => createApiClient(() => session?.access_token),
    [session?.access_token],
  )

  return <ApiContext.Provider value={client}>{children}</ApiContext.Provider>
}

export function useApi(): ApiClient {
  const ctx = useContext(ApiContext)
  if (!ctx) throw new Error('useApi must be used within ApiProvider')
  return ctx
}
