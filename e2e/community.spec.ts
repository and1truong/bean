import {expect,request as playwrightRequest} from '@playwright/test'
import {test as beanBase,login} from './fixtures/bean'
const test=beanBase.extend<{}, {appName:string}>({appName:['community',{scope:'worker'}]})
async function user(baseURL:string,email:string){const api=await playwrightRequest.newContext({baseURL});const login=await api.post('/api/auth/login',{data:{email,password:'test-password'}});return {api,csrf:(await login.json()).csrfToken}}
test('home renders the public feed without exposing private posts',async({page,bean})=>{
  const rendered=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/system/page')
  await page.goto(bean.baseURL+'/')
  expect((await rendered).status()).toBe(200)
  await expect(page.getByRole('heading',{name:'Community',exact:true})).toBeVisible()
  await expect(page.getByText('No public posts yet.',{exact:true})).toBeVisible()

  const author=await user(bean.baseURL,'user-a@example.test')
  try{
    for(const visibility of ['private','public']){
      const made=await author.api.post('/api/actions/post_create',{headers:{'X-CSRF-Token':author.csrf},data:{body:visibility+' homepage post',visibility}})
      expect(made.ok()).toBeTruthy()
    }
  }finally{await author.api.dispose()}
  await page.reload()
  await expect(page.getByText('public homepage post',{exact:true})).toBeVisible()
  await expect(page.getByText('private homepage post',{exact:true})).toHaveCount(0)
  await expect(page.getByRole('link',{name:'Sign in',exact:true})).toBeVisible()

  await login(page,bean.baseURL,'user-a@example.test','test-password',/\/$/)
  await expect(page.getByText('public homepage post',{exact:true})).toBeVisible()
  await expect(page.getByText('private homepage post',{exact:true})).toHaveCount(0)
})
test('members publish their private posts and react through Admin',async({page,bean})=>{
  await login(page,bean.baseURL,'editor-a@example.test')
  await page.getByRole('link',{name:'Post Manage post',exact:true}).click()
  await page.getByRole('link',{name:'Add Post',exact:true}).click()
  await page.getByTestId('field-body').fill('Community browser publication')
  await page.getByTestId('field-visibility').selectOption('private')
  await page.getByTestId('create-post').click()
  await expect(page.getByRole('textbox',{name:'Visibility',exact:true})).toHaveValue('private')
  const postURL=page.url()
  const id=postURL.split('/').at(-1)!

  await login(page,bean.baseURL,'editor-b@example.test')
  await page.getByRole('link',{name:'Post Manage post',exact:true}).click()
  await expect(page.getByText('No matching records')).toBeVisible()
  expect((await page.request.get(bean.baseURL+'/api/admin/resources/post/'+id)).status()).toBe(404)
  expect(await (await page.request.get(bean.baseURL+'/api/feed')).text()).not.toContain('Community browser publication')

  await login(page,bean.baseURL,'editor-a@example.test')
  await page.goto(postURL)
  await expect(page.getByRole('combobox',{name:'Action',exact:true})).toHaveValue('publish_post')
  await page.getByTestId('field-visibility').selectOption('public')
  await page.getByRole('button',{name:'Run for 1'}).click()
  await expect(page.getByRole('status')).toContainText('Action completed')
  await page.reload()
  await expect(page.getByRole('textbox',{name:'Visibility',exact:true})).toHaveValue('public')

  await login(page,bean.baseURL,'editor-b@example.test')
  expect(await (await page.request.get(bean.baseURL+'/api/feed')).text()).toContain('Community browser publication')
  await page.getByRole('link',{name:'Reaction Manage reaction',exact:true}).click()
  await page.getByRole('link',{name:'Add Reaction',exact:true}).click()
  await page.getByTestId('field-post_id').selectOption(id)
  await page.getByTestId('field-kind').selectOption('like')
  await page.getByTestId('create-reaction').click()
  await expect(page).toHaveURL(/\/admin\/reaction\/[0-9a-f-]+$/)
  await page.reload()
  await expect(page.getByTestId('field-post_id')).toHaveValue(id)
  await expect(page.getByTestId('field-kind')).toHaveValue('like')
})
test('private post is isolated, then public and reactable through Actions',async({bean})=>{const a=await user(bean.baseURL,'user-a@example.test');const b=await user(bean.baseURL,'user-b@example.test');const made=await a.api.post('/api/actions/post_create',{headers:{'X-CSRF-Token':a.csrf},data:{body:'Private thought',visibility:'private'}});const madeBody=await made.text();expect(made.ok(),madeBody).toBeTruthy();const id=JSON.parse(madeBody).data.id;expect(await (await a.api.get('/api/views/post_list')).text()).toContain('Private thought');for(const path of ['/api/views/post_list','/api/feed'])expect(await (await b.api.get(path)).text()).not.toContain('Private thought');expect((await b.api.post('/api/actions/post_update',{headers:{'X-CSRF-Token':b.csrf},data:{id,body:'stolen'}})).status()).toBe(404);expect((await a.api.post('/api/actions/publish_post',{headers:{'X-CSRF-Token':a.csrf},data:{id,visibility:'public'}})).ok()).toBeTruthy();expect(await (await b.api.get('/api/feed')).text()).toContain('Private thought');expect((await b.api.post('/api/actions/reaction_create',{headers:{'X-CSRF-Token':b.csrf},data:{post_id:id,kind:'like'}})).ok()).toBeTruthy();await Promise.all([a.api.dispose(),b.api.dispose()])})
