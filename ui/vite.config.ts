import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backendPort = process.env.BACKEND_PORT || env.BACKEND_PORT || '8080'
  const target = `http://localhost:${backendPort}`
  console.log(`Proxying API requests to: ${target}`)

  return {
    plugins: [react()],
    build: {
      outDir: '../static',
      emptyOutDir: true,
    },
    server: {
      proxy: {
        '/api': target,
        '/audio': target
      }
    }
  }
})
