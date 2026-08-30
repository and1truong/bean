import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins:[react(),tailwindcss()],
  resolve:{alias:{'@':path.resolve(__dirname,'./src')}},
  build:{outDir:'../internal/uiassets/dist',emptyOutDir:true},
  test:{environment:'jsdom',globals:true,setupFiles:['./src/test.ts']},
})
