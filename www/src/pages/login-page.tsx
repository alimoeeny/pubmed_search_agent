import { useState } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useForm } from 'react-hook-form'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { toast } from 'sonner'
import { FlaskConical, Mail } from 'lucide-react'
import { useAuth } from '@/hooks/use-auth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { ThemeToggle } from '@/components/layout/theme-toggle'

const schema = z.object({
  email: z.string().email('Please enter a valid email address'),
})
type FormValues = z.infer<typeof schema>

export function LoginPage() {
  const { session, loading, signInWithMagicLink } = useAuth()
  const location = useLocation()
  const from = (location.state as { from?: Location } | null)?.from?.pathname ?? '/'
  const [sent, setSent] = useState(false)
  const [lastEmail, setLastEmail] = useState('')

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  if (!loading && session) return <Navigate to={from} replace />

  const onSubmit = async ({ email }: FormValues) => {
    try {
      await signInWithMagicLink(email)
      setLastEmail(email)
      setSent(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to send magic link')
    }
  }

  const resend = async () => {
    try {
      await signInWithMagicLink(lastEmail)
      toast.success('Magic link resent!')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to resend')
    }
  }

  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center p-4">
      <div className="absolute right-4 top-4">
        <ThemeToggle />
      </div>

      <div className="mb-8 flex items-center gap-2 text-2xl font-semibold">
        <FlaskConical className="size-7 text-primary" />
        PubMed Agent
      </div>

      <Card className="w-full max-w-sm">
        {!sent ? (
          <>
            <CardHeader className="text-center">
              <CardTitle>Sign in</CardTitle>
              <CardDescription>We'll send a magic link to your inbox</CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
                <div className="space-y-1.5">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    placeholder="you@example.com"
                    autoComplete="email"
                    autoFocus
                    {...register('email')}
                    aria-invalid={!!errors.email}
                  />
                  {errors.email && (
                    <p className="text-sm text-destructive">{errors.email.message}</p>
                  )}
                </div>
                <Button type="submit" className="w-full" disabled={isSubmitting}>
                  {isSubmitting ? 'Sending…' : 'Send magic link'}
                </Button>
              </form>
            </CardContent>
          </>
        ) : (
          <>
            <CardHeader className="text-center">
              <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-primary/10">
                <Mail className="size-6 text-primary" />
              </div>
              <CardTitle>Check your inbox</CardTitle>
              <CardDescription>
                We sent a magic link to <strong>{lastEmail}</strong>
              </CardDescription>
            </CardHeader>
            <CardContent className="text-center">
              <p className="mb-4 text-sm text-muted-foreground">
                Didn't receive it? Check your spam folder or resend.
              </p>
              <Button variant="outline" onClick={resend} className="w-full">
                Resend magic link
              </Button>
            </CardContent>
          </>
        )}
      </Card>
    </div>
  )
}
