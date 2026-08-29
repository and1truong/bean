import {test,expect,login} from './fixtures/bean'

test('draft is hidden then published to HTML, JSON, RSS, and CSV',async({page,bean})=>{
  await login(page,bean.baseURL)
  await page.getByTestId('field-title').fill('Bean ships')
  await page.getByTestId('field-body').fill('Metadata works')
  await page.getByTestId('field-status').fill('draft')
  await page.getByTestId('create-article').click()
  await expect(page.getByText('Bean ships')).toBeVisible()
  expect(await (await page.request.get(bean.baseURL+'/api/news')).text()).not.toContain('Bean ships')
  const row=(await (await page.request.get(bean.baseURL+'/api/views/article_list')).json()).data[0]
  const session=await (await page.request.get(bean.baseURL+'/api/system/session')).json()
  const published=await page.request.post(bean.baseURL+'/api/actions/publish_article',{headers:{'X-CSRF-Token':session.csrfToken},data:{id:row.id,status:'published'}})
  expect(published.ok(),await published.text()).toBeTruthy()
  expect(await (await page.request.get(bean.baseURL+'/api/news')).text()).toContain('Bean ships')
  await page.goto(bean.baseURL+'/')
  await expect(page.getByText('Bean ships')).toBeVisible()
  for(const path of ['/api/news','/news.rss','/news.csv'])expect(await (await page.request.get(bean.baseURL+path)).text()).toContain('Bean ships')
})
