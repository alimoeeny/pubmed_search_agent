import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAuth } from '@/hooks/use-auth'
import { Skeleton } from '@/components/ui/skeleton'
import { buttonVariants } from '@/components/ui/button'

const TIMEOUT_MS = 8_000

export function AuthCallbackPage() {
  const { session, loading } = useAuth()
  const navigate = useNavigate()
  const [timedOut, setTimedOut] = useState(false)

  useEffect(() => {
    if (!loading && session) {
      navigate('/', { replace: true })
      return
    }
    const t = setTimeout(() => setTimedOut(true), TIMEOUT_MS)
    return () => clearTimeout(t)
  }, [session, loading, navigate])

  if (timedOut && !session) {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-4 text-center">
        <p className="font-medium">Link expired or invalid</p>
        <p className="text-sm text-muted-foreground">Please request a new magic link.</p>
        <Link to="/login" className={buttonVariants({ variant: 'outline' })}>
          Back to login
        </Link>
      </div>
    )
  }

  return (
    <div className="flex h-screen items-center justify-center">
      <div className="w-64 space-y-3">
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    </div>
  )
}
