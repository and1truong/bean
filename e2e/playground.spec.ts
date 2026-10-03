import {test,expect} from '@playwright/test'
import {spawn,ChildProcess} from 'node:child_process'
import {existsSync} from 'node:fs'
import {join,resolve} from 'node:path'

// The playground is a fully static bundle under dist/playground (built by
// `make playground`). These journeys serve it with a plain static file
// server and drive a real Chromium — there is no Bean backend on the wire.

const distDir=resolve(import.meta.dirname,'../dist/playground')

async function serveStatic(port:number):Promise<{url:string;stop:()=>void}>{
  const child:ChildProcess=spawn('python3',['-m','http.server',String(port),'--bind','127.0.0.1'],{cwd:distDir,stdio:['ignore','pipe','pipe']})
  const url=`http://127.0.0.1:${port}`
  for(let i=0;i<100;i++){
    try{const response=await fetch(url+'/index.html');if(response.ok)return{url,stop:()=>child.kill('SIGTERM')}}catch{}
    await new Promise(r=>setTimeout(r,50))
  }
  child.kill('SIGTERM')
  throw new Error('static server did not start')
}

test.describe('browser playground',()=>{
  test.skip(!existsSync(join(distDir,'index.html')),'run `make playground` first')
  test.skip(!existsSync(join(distDir,'bean.wasm')),'run `make playground` first')

  let server:{url:string;stop:()=>void}
  test.beforeAll(async()=>{server=await serveStatic(18300)})
  test.afterAll(()=>server.stop())

  test('starter compiles in the browser and makes no /api requests',async({page})=>{
    const apiRequests:string[]=[]
    page.on('request',request=>{if(request.url().includes('/api'))apiRequests.push(request.url())})
    await page.goto(server.url+'/')
    await expect(page.getByRole('button',{name:'Compile & preview'})).toBeEnabled({timeout:60_000})
    await expect(page.getByRole('heading',{name:'Bean playground',exact:true})).toBeVisible({timeout:60_000})
    await expect(page.getByLabel('Preview',{exact:true}).getByText('no Bean server is involved')).toBeVisible()
    await expect(page.getByLabel('Preview route')).toHaveValue('/')
    expect(apiRequests).toEqual([])
  })

  test('example loads, navigates to its route, and marks backend-dependent blocks',async({page})=>{
    const failures:string[]=[]
    page.on('requestfailed',request=>failures.push(request.url()))
    await page.goto(server.url+'/')
    await expect(page.getByRole('button',{name:'Compile & preview'})).toBeEnabled({timeout:60_000})
    await page.getByLabel('Example').selectOption('presentation')
    await expect(page.getByLabel('Preview route')).toHaveValue('/presentations/bean',{timeout:60_000})
    expect(page.url()).toContain('#/presentations/bean')
    await expect(page.getByLabel('Choose frame')).toBeVisible()
    // The presentation's Explore frame is a View block: marked as
    // backend-dependent, never simulated.
    await page.getByLabel('Choose frame').selectOption('explore')
    await expect(page.getByLabel('Backend-dependent components').getByText('ViewBlock')).toBeVisible()
    // Source tabs list the manifest and every resource file.
    await expect(page.getByRole('tab',{name:'app.yaml (manifest)'})).toBeVisible()
    await expect(page.getByRole('tab',{name:'content.yaml',exact:true})).toBeVisible()
    expect(failures.filter(url=>!url.includes('favicon'))).toEqual([])
  })

  test('a broken edit reports diagnostics and keeps the last valid preview',async({page})=>{
    await page.goto(server.url+'/')
    await expect(page.getByRole('button',{name:'Compile & preview'})).toBeEnabled({timeout:60_000})
    await expect(page.getByRole('heading',{name:'Bean playground',exact:true})).toBeVisible({timeout:60_000})
    const editor=page.getByLabel('app.yaml source')
    await editor.fill('apiVersion: bean/v1alpha1\nname: broken\nkind: NoSuchKind\n')
    await page.getByRole('button',{name:'Compile & preview'}).click()
    await expect(page.getByRole('region',{name:'Diagnostics'})).toBeVisible()
    await expect(page.getByRole('heading',{name:'Bean playground',exact:true})).toBeVisible()
    // Repairing the source compiles cleanly again.
    await editor.fill('apiVersion: bean/v1alpha1\nname: Fixed\n---\nkind: Block\nname: only\ntype: content\ncontent:\n  - {type: paragraph, text: "back"}\n---\nkind: Panel\nname: main\nlayout: single-column\nregions:\n  - {name: main, blocks: [only]}\n---\nkind: Page\nname: home\npanel: main\nroute: /\ntitle: Fixed\n')
    await page.getByRole('button',{name:'Compile & preview'}).click()
    await expect(page.getByText('back',{exact:true})).toBeVisible()
    await expect(page.getByRole('region',{name:'Diagnostics'})).toHaveCount(0)
  })

  test('route changes drive hash history for back/forward navigation',async({page})=>{
    await page.goto(server.url+'/')
    await expect(page.getByRole('button',{name:'Compile & preview'})).toBeEnabled({timeout:60_000})
    const editor=page.getByLabel('app.yaml source')
    await editor.fill(TWO_ROUTES)
    await page.getByRole('button',{name:'Compile & preview'}).click()
    await expect(page.getByText('page one',{exact:true})).toBeVisible()
    await page.getByLabel('Preview route').selectOption('/about')
    await expect(page.getByText('page two',{exact:true})).toBeVisible()
    expect(page.url()).toContain('#/about')
    await page.goBack()
    await expect(page.getByText('page one',{exact:true})).toBeVisible()
    await page.goForward()
    await expect(page.getByText('page two',{exact:true})).toBeVisible()
  })

  test('agent surface composes and renders an app via window.bean',async({page})=>{
    await page.goto(server.url+'/')
    await expect(page.getByRole('button',{name:'Compile & preview'})).toBeEnabled({timeout:60_000})
    const meta=await page.evaluate(()=>{
      const bean=(window as unknown as{bean:{v:number;webmcp:boolean;tools:string[]}}).bean
      return{v:bean.v,webmcp:bean.webmcp,tools:bean.tools}
    })
    expect(meta.v).toBe(1)
    expect(meta.tools).toEqual(['bean_state','bean_compile','bean_render','bean_navigate','bean_list_examples','bean_load_example'])
    const compile=await page.evaluate(source=>(window as unknown as{bean:{bean_compile(i?:unknown):Promise<{ok:boolean;app?:{routes:{path:string}[]}}>}}).bean.bean_compile({files:{'app.yaml':source},manifest:'app.yaml'}),TWO_ROUTES)
    expect(compile.ok).toBe(true)
    expect(compile.app?.routes.map(route=>route.path)).toEqual(['/','/about'])
    await page.evaluate(()=>(window as unknown as{bean:{bean_navigate(i:{path:string}):unknown}}).bean.bean_navigate({path:'/about'}))
    await expect(page.getByText('page two',{exact:true})).toBeVisible()
    expect(page.url()).toContain('#/about')
    const render=await page.evaluate(()=>(window as unknown as{bean:{bean_render(i:{path:string}):Promise<{ok:boolean;tree?:unknown}>}}).bean.bean_render({path:'/'}))
    expect(render.ok).toBe(true)
    const state=await page.evaluate(()=>(window as unknown as{bean:{bean_state():{manifest:string;files:Record<string,{bytes:number}>}}}).bean.bean_state())
    expect(state.manifest).toBe('app.yaml')
    expect(Object.keys(state.files)).toEqual(['app.yaml'])
  })

  test('zip export downloads a sources archive',async({page})=>{
    await page.goto(server.url+'/')
    await expect(page.getByRole('button',{name:'Compile & preview'})).toBeEnabled({timeout:60_000})
    const download=page.waitForEvent('download')
    await page.getByRole('button',{name:'Export .zip'}).click()
    expect((await download).suggestedFilename()).toBe('bean-sources.zip')
  })

  test('narrow viewport keeps content inside the window',async({page})=>{
    await page.setViewportSize({width:390,height:800})
    await page.goto(server.url+'/')
    await expect(page.getByRole('button',{name:'Compile & preview'})).toBeEnabled({timeout:60_000})
    await expect(page.getByRole('heading',{name:'Bean playground',exact:true})).toBeVisible({timeout:60_000})
    const scrollWidth=await page.evaluate(()=>document.documentElement.scrollWidth)
    expect(scrollWidth).toBeLessThanOrEqual(391)
  })
})

const TWO_ROUTES=`apiVersion: bean/v1alpha1
name: Two pages
---
kind: Block
name: first
type: content
content: [{type: paragraph, text: "page one"}]
---
kind: Block
name: second
type: content
content: [{type: paragraph, text: "page two"}]
---
kind: Panel
name: one
layout: single-column
regions: [{name: main, blocks: [first]}]
---
kind: Panel
name: two
layout: single-column
regions: [{name: main, blocks: [second]}]
---
kind: Page
name: home
panel: one
route: /
title: One
---
kind: Page
name: about
panel: two
route: /about
title: Two
`
