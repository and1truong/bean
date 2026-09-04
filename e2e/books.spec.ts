import {test as base,expect} from './fixtures/bean'
import {action,login} from './helpers/api'

const test=base.extend<{}, {appName:string}>({appName:['books',{scope:'worker'}]})

async function create(request:any,baseURL:string,csrf:string,name:string,data:Record<string,unknown>){
  const response=await action(request,baseURL,csrf,name,data)
  expect(response.ok(),await response.text()).toBeTruthy()
  return (await response.json()).data as Record<string,any>
}

async function menu(request:any,baseURL:string,book:string){
  const response=await request.get(baseURL+'/api/menus/book_contents?_page='+encodeURIComponent('/books/'+book)+'&_block=contents')
  expect(response.ok(),await response.text()).toBeTruthy()
  return (await response.json()).items as any[]
}

test('scoped Book contents resolve, edit, filter, and clean up atomically',async({page,request,bean})=>{
  const csrf=await login(request,bean.baseURL)
  const first=await create(request,bean.baseURL,csrf,'create_book',{title:'Building Bean'})
  const second=await create(request,bean.baseURL,csrf,'create_book',{title:'Operating Bean'})
  const root=await create(request,bean.baseURL,csrf,'create_page',{title:'Architecture',body:'Start with immutable definitions.',_navigation:{placements:[{menu:'book_contents',ownerId:first.id,weight:10}]}})
  const rootPlacement=(await menu(request,bean.baseURL,first.id))[0].ID
  const child=await create(request,bean.baseURL,csrf,'create_page',{title:'Compiler',body:'Compile one immutable AppIR.',_navigation:{placements:[{menu:'book_contents',ownerId:first.id,parentId:rootPlacement,weight:20}]}})
  const childPlacement=(await menu(request,bean.baseURL,first.id))[0].Children[0].ID
  const leaf=await create(request,bean.baseURL,csrf,'create_page',{title:'Validation',body:'Reject invalid navigation before activation.',_navigation:{placements:[{menu:'book_contents',ownerId:first.id,parentId:childPlacement,weight:30,labelOverride:'Validate'}]}})
  const shared=await create(request,bean.baseURL,csrf,'create_page',{title:'Shared appendix',body:'One record can appear in several Books.',_navigation:{placements:[{menu:'book_contents',ownerId:first.id,weight:40},{menu:'book_contents',ownerId:second.id,weight:5}]}})

  await page.goto(bean.baseURL+'/books/'+first.id)
  await expect(page.getByText('Architecture')).toHaveCount(0)
  await loginPage(page,bean.baseURL)
  await page.goto(bean.baseURL+'/books/'+first.id)
  await expect(page.getByRole('navigation',{name:'Primary navigation'}).getByRole('link',{name:'Architecture'})).toBeVisible()
  await expect(page.getByRole('navigation',{name:'Section navigation'}).getByRole('link',{name:'Validate'})).toBeVisible()
  await page.getByRole('navigation',{name:'Section navigation'}).getByRole('link',{name:'Validate'}).click()
  await expect(page).toHaveURL(new RegExp('/pages/'+leaf.id+'\\?_menu=book_contents&_owner='+first.id))
  await expect(page.getByRole('heading',{name:'Validation'})).toBeVisible()
  await expect(page.getByRole('link',{name:'Validate'})).toHaveAttribute('aria-current','page')

  await page.setViewportSize({width:500,height:800})
  await expect(page.getByLabel('Section',{exact:true})).toBeVisible()
  await expect(page.getByRole('navigation',{name:'Section navigation'})).toBeHidden()
  await page.setViewportSize({width:1100,height:800})

  await page.goto(bean.baseURL+'/admin/pages/'+leaf.id)
  await expect(page.getByRole('group',{name:'Navigation'})).toBeVisible()
  await page.getByLabel('Parent',{exact:true}).selectOption(rootPlacement)
  await page.getByLabel('Weight',{exact:true}).fill('15')
  await page.getByLabel('Label override',{exact:true}).fill('Validation moved')
  await page.getByTestId('save-page').click()
  await expect(page).toHaveURL(/\/admin\/pages$/)
  const movedTree=await menu(request,bean.baseURL,first.id)
  expect(movedTree[0].Children.map((item:any)=>item.Label)).toEqual(['Validation moved','Compiler'])

  const removed=await action(request,bean.baseURL,csrf,'delete_page',{id:shared.id,version:shared.version})
  expect(removed.ok(),await removed.text()).toBeTruthy()
  expect(JSON.stringify(await menu(request,bean.baseURL,first.id))).not.toContain('Shared appendix')
  expect(JSON.stringify(await menu(request,bean.baseURL,second.id))).not.toContain('Shared appendix')

  const ownerRemoved=await action(request,bean.baseURL,csrf,'delete_book',{id:first.id,version:first.version})
  expect(ownerRemoved.ok(),await ownerRemoved.text()).toBeTruthy()
  const missing=await request.get(bean.baseURL+'/api/menus/book_contents?_page='+encodeURIComponent('/books/'+first.id)+'&_block=contents')
  expect(missing.status()).toBe(404)
})

async function loginPage(page:any,baseURL:string){
  await page.goto(baseURL+'/login')
  await page.getByTestId('email').fill('admin@example.test')
  await page.getByTestId('password').fill('test-password')
  await page.getByTestId('login').click()
  await expect(page).toHaveURL(/\/admin$/)
}
