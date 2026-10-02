import {test as base} from './fixtures/bean'

const test=base.extend<{}, {appName:string}>({appName:['presentation',{scope:'worker'}]})
const {expect}=test

test('capture mindmap frame',async({page,bean})=>{
  await page.setViewportSize({width:1280,height:800})
  await page.goto(bean.baseURL+'/presentations/bean')
  await page.getByLabel('Choose frame').selectOption('mindmap')
  await expect(page.getByRole('article',{name:'Bean'}).getByTestId('mindmap-figure').locator('svg')).toBeVisible()
  await page.waitForTimeout(500)
  await page.screenshot({path:'/tmp/mindmap-desktop.png'})
  await page.setViewportSize({width:390,height:844})
  await expect(page.getByTestId('mindmap-figure')).toBeHidden()
  await page.getByRole('article',{name:'Bean'}).scrollIntoViewIfNeeded()
  await page.waitForTimeout(500)
  await page.screenshot({path:'/tmp/mindmap-mobile.png'})
})
