import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import { App } from './App'
import { Toaster } from '@/components/ui/sonner'
import { initTheme } from '@/stores/theme-store'

initTheme()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
    <Toaster richColors closeButton />
  </StrictMode>,
)
