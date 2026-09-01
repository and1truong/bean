import {test,expect} from '@playwright/test'
import {execFileSync,spawn} from 'node:child_process'
import {copyFileSync,mkdtempSync,mkdirSync,rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {dirname,join,resolve} from 'node:path'

test('package starts without its source and retains populated Views',async({page},testInfo)=>{
  const root=resolve(import.meta.dirname,'..')
  const directory=mkdtempSync(join(tmpdir(),'bean-package-e2e-'))
  const source=join(directory,'source','app.yaml')
  const output=join(directory,'dist','ats')
  mkdirSync(dirname(source),{recursive:true})
  copyFileSync(join(root,'examples/ats/app.yaml'),source)
  execFileSync(join(root,'bin/bean'),['package','--file',source,'--output',output,'--seed','42'])
  rmSync(dirname(source),{recursive:true})

  const port=18200+testInfo.workerIndex
  const child=spawn(join(output,'bean'),['serve','--db','./bean.db','--addr',`127.0.0.1:${port}`],{cwd:output,stdio:['ignore','pipe','pipe']})
  const url=`http://127.0.0.1:${port}`
  try{
    for(let index=0;index<100;index++){
      try{if((await fetch(url+'/healthz')).ok)break}catch{}
      await new Promise(resolve=>setTimeout(resolve,50))
    }
    await page.goto(url+'/')
    await expect(page.getByRole('link',{name:'Acme Recruiting'})).toBeVisible()
    await expect(page.getByTestId('metric-value')).toHaveText('18')
    await expect(page.getByTestId('timeline-view')).toBeVisible()
  }finally{
    child.kill('SIGTERM')
    if(testInfo.status===testInfo.expectedStatus)rmSync(directory,{recursive:true,force:true})
  }
})
