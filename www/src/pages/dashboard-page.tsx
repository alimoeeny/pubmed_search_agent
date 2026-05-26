import { useNavigate } from 'react-router-dom'
import { toast } from 'sonner'
import { Plus, FlaskConical } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { SessionCard } from '@/components/sessions/session-card'
import { useSessions, useCreateSession } from '@/hooks/use-sessions'

export function DashboardPage() {
  const navigate = useNavigate()
  const { data: sessions, isLoading, isError } = useSessions()
  const { mutate: createSession, isPending: isCreating } = useCreateSession()

  const handleNew = () => {
    createSession(undefined, {
      onSuccess: ({ session_id }) => navigate(`/sessions/${session_id}`),
      onError: () => toast.error('Failed to create session'),
    })
  }

  return (
    <div className="mx-auto max-w-4xl px-4 py-10">
      <div className="mb-8 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Research sessions</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Each session is an independent conversation with the agent.
          </p>
        </div>
        <Button onClick={handleNew} disabled={isCreating}>
          <Plus className="size-4" />
          {isCreating ? 'Creating…' : 'New search'}
        </Button>
      </div>

      {isLoading && (
        <div className="grid gap-3 sm:grid-cols-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-18 rounded-xl" />
          ))}
        </div>
      )}

      {isError && (
        <p className="text-sm text-destructive">Failed to load sessions. Try refreshing.</p>
      )}

      {!isLoading && !isError && sessions && sessions.length === 0 && (
        <div className="flex flex-col items-center justify-center gap-4 rounded-xl border border-dashed py-20 text-center">
          <div className="flex size-14 items-center justify-center rounded-full bg-primary/10">
            <FlaskConical className="size-7 text-primary" />
          </div>
          <div>
            <p className="font-medium">No sessions yet</p>
            <p className="mt-1 text-sm text-muted-foreground">
              Start a new search to explore the literature.
            </p>
          </div>
          <Button onClick={handleNew} disabled={isCreating}>
            <Plus className="size-4" />
            New search
          </Button>
        </div>
      )}

      {!isLoading && sessions && sessions.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-2">
          {sessions.map((s) => (
            <SessionCard key={s.session_id} session={s} />
          ))}
        </div>
      )}
    </div>
  )
}
