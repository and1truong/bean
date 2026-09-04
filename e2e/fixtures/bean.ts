import {test as base,expect} from '@playwright/test'
import {execFileSync,spawn,ChildProcess} from 'node:child_process'
import {mkdtempSync,rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {join,resolve} from 'node:path'

type Bean={baseURL:string;csrf:string;stop:()=>void}

export const test=base.extend<{bean:Bean},{appName:string}>({
  appName:['cms',{option:true,scope:'worker'}],
  bean:async({appName},use,testInfo)=>{
    const root=resolve(import.meta.dirname,'../..')
    const bin=join(root,'bin/bean')
    const dir=mkdtempSync(join(tmpdir(),'bean-e2e-'))
    const databaseURL=process.env.BEAN_E2E_DATABASE_URL
    const target=databaseURL?['--database-url',databaseURL]:['--db',join(dir,'bean.db')]
    const port=18100+testInfo.workerIndex
    execFileSync(bin,['init',...target,'--admin-email','admin@example.test','--admin-password','test-password'])
    if(appName!=='empty'){
      execFileSync(bin,['app','import',...target,'--file',join(root,'examples',appName,'app.yaml')])
      execFileSync(bin,['publish',...target])
	  if(appName==='ats'||appName==='presentation')execFileSync(bin,['demo','seed',...target,'--file',join(root,'examples',appName,'app.yaml'),'--seed','42'])
    }
    const createUser=(email:string,roles:string,tenant='')=>execFileSync(bin,['user','create',...target,'--email',email,'--password','test-password','--roles',roles,'--tenant',tenant])
    if(appName==='crm'){
      createUser('sales-a@example.test','salesperson')
      createUser('sales-b@example.test','salesperson')
      createUser('manager@example.test','manager')
    }
    if(appName==='saas'){
      createUser('tenant-a@example.test','member','00000000-0000-4000-8000-00000000000a')
      createUser('tenant-b@example.test','member','00000000-0000-4000-8000-00000000000b')
    }
    if(appName==='community'){
      createUser('user-a@example.test','member')
      createUser('user-b@example.test','member')
      createUser('editor-a@example.test','member,editor')
      createUser('editor-b@example.test','member,editor')
    }
    if(appName==='blog')createUser('editor@example.test','editor')
    const child:ChildProcess=spawn(bin,['serve',...target,'--addr',`127.0.0.1:${port}`],{stdio:['ignore','pipe','pipe']})
    const url=`http://127.0.0.1:${port}`
    for(let i=0;i<100;i++){
      try{const response=await fetch(url+'/healthz');if(response.ok)break}catch{}
      await new Promise(resolve=>setTimeout(resolve,50))
    }
    const request=await fetch(url+'/api/auth/login',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({email:'admin@example.test',password:'test-password'})})
    if(!request.ok)throw new Error('login setup failed')
    const csrf=(await request.json()).csrfToken
    const stop=()=>{child.kill('SIGTERM');if(testInfo.status===testInfo.expectedStatus)rmSync(dir,{recursive:true,force:true})}
    await use({baseURL:url,csrf,stop})
    stop()
  },
})

export {expect}
export async function login(page:any,baseURL:string,email='admin@example.test',password='test-password',destination=/\/admin$/){
  await page.goto(baseURL+'/login')
  await page.getByTestId('email').fill(email)
  await page.getByTestId('password').fill(password)
  await page.getByTestId('login').click()
  await expect(page).toHaveURL(destination)
}
