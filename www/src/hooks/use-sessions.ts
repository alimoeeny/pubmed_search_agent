import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useApi } from '@/lib/api-context'

export const SESSION_KEYS = {
  all: ['sessions'] as const,
}

export function useSessions() {
  const api = useApi()
  return useQuery({
    queryKey: SESSION_KEYS.all,
    queryFn: () => api.listSessions().then((r) => r.sessions),
  })
}

export function useCreateSession() {
  const api = useApi()
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => api.createSession(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: SESSION_KEYS.all }),
  })
}

export function useDeleteSession() {
  const api = useApi()
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.deleteSession(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: SESSION_KEYS.all }),
  })
}
