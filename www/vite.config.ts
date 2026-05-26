import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 600,
    // advancedChunks is the current rolldown API for vendor splitting; rename to codeSplitting when rolldown types catch up
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
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
    } as any,
  },
})
