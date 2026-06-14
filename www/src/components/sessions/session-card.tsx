import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Trash2, MessageSquare } from 'lucide-react'
import { toast } from 'sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { useDeleteSession } from '@/hooks/use-sessions'
import type { SessionSummary } from '@/lib/api'

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  return `${Math.floor(hrs / 24)}d ago`
}

export function SessionCard({ session }: { session: SessionSummary }) {
  const navigate = useNavigate()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const { mutate: deleteSession, isPending } = useDeleteSession()

  const shortId = session.session_id.slice(-8)
  const title = session.title.trim() || 'New research session'

  const handleDelete = () => {
    deleteSession(session.session_id, {
      onSuccess: () => {
        toast.success('Session deleted')
        setConfirmOpen(false)
      },
      onError: () => toast.error('Failed to delete session'),
    })
  }

  return (
    <>
      <Card
        className="group/session-card cursor-pointer transition-shadow hover:ring-2 hover:ring-ring"
        onClick={() => navigate(`/sessions/${session.session_id}`)}
      >
        <CardContent className="flex min-h-24 items-start justify-between gap-3 py-4">
          <div className="flex min-w-0 items-start gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <MessageSquare className="size-4 text-primary" />
            </div>
            <div className="min-w-0">
              <p className="line-clamp-2 text-sm font-medium leading-5">{title}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {formatRelative(session.last_updated)}
                <span aria-hidden="true"> · </span>
                <span className="font-mono">…{shortId}</span>
              </p>
            </div>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Delete session"
            className="shrink-0 opacity-0 group-hover/session-card:opacity-100 text-muted-foreground hover:text-destructive"
            onClick={(e) => {
              e.stopPropagation()
              setConfirmOpen(true)
            }}
          >
            <Trash2 className="size-4" />
          </Button>
        </CardContent>
      </Card>

      <Dialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete session?</DialogTitle>
            <DialogDescription>
              Session <span className="font-mono">…{shortId}</span> will be permanently deleted.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={isPending}>
              {isPending ? 'Deleting…' : 'Delete'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
