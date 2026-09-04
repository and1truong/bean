import {test,expect} from '@playwright/test'
import {execFileSync,spawn} from 'node:child_process'
import {mkdtempSync,mkdirSync,readFileSync,writeFileSync,rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {join,resolve} from 'node:path'

test('field layout survives source-independent packaging',async({page},testInfo)=>{
  const root=resolve(import.meta.dirname,'..')
  const directory=mkdtempSync(join(tmpdir(),'bean-field-layout-package-'))
  const source=join(directory,'source')
  const output=join(directory,'package')
  mkdirSync(source)
  writeFileSync(join(source,'app.yaml'),readFileSync(join(root,'examples/ats/app.yaml'),'utf8')+`
---
kind: View
name: candidate_layout
entity: candidate
fields: [id, name, email, summary]
exposedFilters:
  id: {field: id, operator: eq}
displays:
  record:
    type: page
    route: /candidate-layout/:id
    title: {field: name, fallback: Candidate}
    bindings:
      id: {source: route, name: id, required: true}
    renderer:
      type: detail
      layout:
        groups:
          - name: profile
            label: Candidate profile
            columns: 2
            fields:
              - {field: name}
              - {field: email}
              - {field: summary, span: full}
`)
  execFileSync(join(root,'bin/bean'),['package','--file',join(source,'app.yaml'),'--output',output,'--seed','42'])
  rmSync(source,{recursive:true})
  const url=`http://127.0.0.1:${18400+testInfo.workerIndex}`
  const child=spawn(join(output,'bean'),['serve','--db','./bean.db','--addr',url.slice(7)],{cwd:output,stdio:['ignore','pipe','pipe']})
  try{
    for(let attempt=0;attempt<100;attempt++){
      try{if((await fetch(url+'/healthz')).ok)break}catch{}
      await new Promise(resolve=>setTimeout(resolve,50))
    }
    const response=await page.request.get(url+'/api/views/candidate_layout')
    expect(response.ok()).toBe(true)
    const {data}=await response.json()
    expect(data.length).toBeGreaterThan(0)
    await page.goto(url+'/candidate-layout/'+data[0].id)
    await expect(page.getByRole('heading',{name:data[0].name,exact:true})).toBeVisible()
    await expect(page.getByRole('region',{name:'Candidate profile'})).toContainText(data[0].email)
    await expect(page.locator('[data-layout-field="summary"]')).toHaveClass(/col-span-full/)
    await page.reload()
    await expect(page.getByRole('region',{name:'Candidate profile'})).toBeVisible()
  }finally{
    if(child.exitCode===null&&child.signalCode===null){
      const exited=new Promise<void>(resolve=>child.once('exit',()=>resolve()))
      child.kill('SIGTERM')
      await exited
    }
    if(testInfo.status===testInfo.expectedStatus)rmSync(directory,{recursive:true,force:true})
  }
})
