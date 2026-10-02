import path from 'node:path'
import { readFileSync } from 'node:fs'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// mermaid 12 splits itself and its ~18 dependencies into dozens of lazy chunks;
// merge them into one 'mermaid' chunk so the diagram engine stays a single
// deferred download (and one checked-in asset) that only loads when a diagram
// renders.
const diagramDeps=new Set(['mermaid',...Object.keys(JSON.parse(readFileSync(path.resolve(__dirname,'node_modules/mermaid/package.json'),'utf8')).dependencies||{})])

export default defineConfig({
  plugins:[react(),tailwindcss()],
  resolve:{alias:{'@':path.resolve(__dirname,'./src')}},
  build:{
    outDir:'../internal/uiassets/dist',
    emptyOutDir:true,
    rollupOptions:{output:{manualChunks(id){
      const match=/node_modules\/((?:@[^/]+\/)?[^/]+)/.exec(id)
      if(match&&diagramDeps.has(match[1]))return 'mermaid'
    }}},
  },
  test:{environment:'jsdom',globals:true,setupFiles:['./src/test.ts']},
})
