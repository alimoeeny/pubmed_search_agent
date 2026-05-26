import { createClient } from '@supabase/supabase-js'
import { config } from '@/lib/config'

export const supabase = createClient(config.supabaseUrl, config.supabaseAnonKey, {
  auth: {
    detectSessionInUrl: true,
    persistSession: true,
    autoRefreshToken: true,
  },
})
