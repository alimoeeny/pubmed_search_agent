export type AppConfig = {
  apiBaseUrl: string
  supabaseUrl: string
  supabaseAnonKey: string
  mode: 'dev' | 'prod'
}

const dev: AppConfig = {
  apiBaseUrl: 'http://localhost:8080',
  supabaseUrl: 'https://gexyiaiwmhlurqkbnguv.supabase.co',
  supabaseAnonKey: 'sb_publishable_317SE226_ybU4MLrs1AgKA_Guqa_Xop',
  mode: 'dev',
}

const prod: AppConfig = {
  apiBaseUrl: 'https://api.pubmedagent.ai-goblins.com',
  supabaseUrl: 'https://gexyiaiwmhlurqkbnguv.supabase.co',
  supabaseAnonKey: 'sb_publishable_317SE226_ybU4MLrs1AgKA_Guqa_Xop',
  mode: 'prod',
}

export const config: AppConfig = import.meta.env.MODE === 'prod' ? prod : dev
