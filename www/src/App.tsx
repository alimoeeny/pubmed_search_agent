import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AppShell } from '@/components/layout/app-shell'
import { RequireAuth } from '@/components/auth/require-auth'
import { ApiProvider } from '@/lib/api-context'
import { LoginPage } from '@/pages/login-page'
import { AuthCallbackPage } from '@/pages/auth-callback-page'
import { DashboardPage } from '@/pages/dashboard-page'
import { SessionPage } from '@/pages/session-page'
import { NotFoundPage } from '@/pages/not-found-page'

const queryClient = new QueryClient()

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ApiProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<LoginPage />} />
            <Route path="/auth/callback" element={<AuthCallbackPage />} />
            <Route
              element={
                <RequireAuth>
                  <AppShell />
                </RequireAuth>
              }
            >
              <Route index element={<DashboardPage />} />
              <Route path="/sessions/:id" element={<SessionPage />} />
            </Route>
            <Route path="/404" element={<NotFoundPage />} />
            <Route path="*" element={<Navigate to="/404" replace />} />
          </Routes>
        </BrowserRouter>
      </ApiProvider>
    </QueryClientProvider>
  )
}
