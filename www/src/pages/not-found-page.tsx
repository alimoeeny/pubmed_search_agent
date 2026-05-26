import { Link } from 'react-router-dom'
import { buttonVariants } from '@/components/ui/button'

export function NotFoundPage() {
  return (
    <div className="flex h-screen flex-col items-center justify-center gap-4">
      <h1 className="text-4xl font-bold">404</h1>
      <p className="text-muted-foreground">Page not found</p>
      <Link to="/" className={buttonVariants({ variant: 'default' })}>
        Go home
      </Link>
    </div>
  )
}
