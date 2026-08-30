import {test,expect,login} from './fixtures/bean'

test('shadcn surfaces remain usable on a mobile viewport',async({page,bean})=>{
  await page.setViewportSize({width:375,height:812})
  await login(page,bean.baseURL)
  await expect(page.getByRole('heading',{name:'Administration'})).toBeVisible()
  await expect(page.locator('[data-slot="card"]').first()).toBeVisible()

  await page.goto(bean.baseURL+'/admin/system')
  await expect(page.getByRole('heading',{name:'System'})).toBeVisible()
  await expect(page.locator('[data-slot="table"]').first()).toBeVisible()

  await page.goto(bean.baseURL+'/')
  await expect(page.getByRole('heading',{name:'News'})).toBeVisible()

  await page.goto(bean.baseURL+'/admin/article')
  await expect(page.locator('[data-slot="native-select"]').first()).toBeVisible()
  await page.getByRole('link',{name:'Add Article'}).click()
  await page.getByTestId('field-title').fill('Responsive record')
  await page.getByTestId('field-status').selectOption('draft')
  await page.getByTestId('create-article').click()
  await expect(page.getByRole('heading',{name:'Responsive record'})).toBeVisible()

  await page.getByRole('button',{name:'Delete'}).click()
  await expect(page.getByRole('alertdialog')).toBeVisible()
  await page.getByRole('button',{name:'Cancel'}).click()
  await expect(page.getByRole('alertdialog')).toHaveCount(0)
  await expect(page.getByRole('heading',{name:'Responsive record'})).toBeVisible()

  const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth)
  expect(overflow).toBe(false)

  await page.goto(bean.baseURL+'/studio')
  await expect(page.getByRole('heading',{name:'Studio'})).toBeVisible()
  await expect(page.locator('[data-slot="card"]').first()).toBeVisible()
})
