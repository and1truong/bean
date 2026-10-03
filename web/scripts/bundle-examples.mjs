#!/usr/bin/env node
// Bundles examples/ into dist/playground/examples/*.json for the playground's
// example picker. Each bundle is {manifest, files} with paths relative to the
// example directory so resources: references resolve in the virtual FS.
import {readdirSync,readFileSync,mkdirSync,writeFileSync,statSync} from 'node:fs'
import path from 'node:path'

const repo=path.resolve(import.meta.dirname,'../..')
const examplesDir=path.join(repo,'examples')
const outDir=path.join(repo,'dist','playground','examples')
mkdirSync(outDir,{recursive:true})

const index=[]
for(const entry of readdirSync(examplesDir).sort()){
  const dir=path.join(examplesDir,entry)
  if(!statSync(dir).isDirectory())continue
  const files={}
  for(const file of readdirSync(dir).sort()){
    if(!/\.(yaml|yml|json)$/i.test(file))continue
    files[file]=readFileSync(path.join(dir,file),'utf8')
  }
  if(!files['app.yaml'])continue
  const manifest=readFileSync(path.join(dir,'app.yaml'),'utf8')
  const title=/^name:\s*(.+)$/m.exec(manifest)?.[1]?.trim()
  index.push({name:entry,title})
  writeFileSync(path.join(outDir,entry+'.json'),JSON.stringify({manifest:'app.yaml',files}))
}
writeFileSync(path.join(outDir,'index.json'),JSON.stringify(index))
console.log(`bundled ${index.length} examples into ${path.relative(repo,outDir)}`)
