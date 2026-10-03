import path from 'node:path'
import {readFileSync} from 'node:fs'
import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// The playground is a fully static bundle — index.html + JS/CSS + worker.js +
// bean.wasm + wasm_exec.js + examples/*.json — served from any static host
// (or any subdirectory) with no Bean backend. base './' + hash routing keep
// direct links and refreshes working without server rewrites.

// Same mermaid merge as the application build: one deferred chunk instead of
// ~70 lazy fragments, loaded only when a diagram or mindmap renders.
const diagramDeps=new Set(['mermaid',...Object.keys(JSON.parse(readFileSync(path.resolve(__dirname,'node_modules/mermaid/package.json'),'utf8')).dependencies||{})])

export default defineConfig({
  plugins:[react(),tailwindcss()],
  resolve:{alias:{'@':path.resolve(__dirname,'./src')}},
  base:'./',
  publicDir:'public-playground',
  build:{
    outDir:'../dist/playground',
    emptyOutDir:true,
    rollupOptions:{
      input:path.resolve(__dirname,'playground.html'),
      output:{manualChunks(id){
        const match=/node_modules\/((?:@[^/]+\/)?[^/]+)/.exec(id)
        if(match&&diagramDeps.has(match[1]))return 'mermaid'
      }},
    },
  },
})
