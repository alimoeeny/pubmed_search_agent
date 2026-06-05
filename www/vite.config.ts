import path from 'path'
import { defineConfig, type UserConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

type BuildRolldownOptions = NonNullable<NonNullable<UserConfig['build']>['rolldownOptions']>

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 600,
    rolldownOptions: {
      output: {
        advancedChunks: {
          groups: [
            { name: 'vendor-react', test: /node_modules\/(react|react-dom|react-router)/ },
            { name: 'vendor-supabase', test: /node_modules\/@supabase/ },
            { name: 'vendor-query', test: /node_modules\/@tanstack/ },
            { name: 'vendor-ui', test: /node_modules\/(@base-ui|lucide-react)/ },
            { name: 'vendor-md', test: /node_modules\/react-markdown/ },
          ],
        },
      },
    } as unknown as BuildRolldownOptions,
  },
})
