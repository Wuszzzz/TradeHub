import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const stockApiTarget = process.env.VITE_STOCK_API_TARGET || 'http://127.0.0.1:7002'
const fundApiTarget = process.env.VITE_FUND_API_TARGET || 'http://127.0.0.1:7001'
const fundResearchApiTarget = process.env.VITE_FUND_RESEARCH_API_TARGET || 'http://127.0.0.1:17081'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api/stock': {
        target: stockApiTarget,
        changeOrigin: true,
      },
      '/api/fund-research': {
        target: fundResearchApiTarget,
        changeOrigin: true,
      },
      '/api': {
        target: fundApiTarget,
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.js',
  },
})
