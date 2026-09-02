import {test as base,expect} from './fixtures/bean'

const test=base.extend<{}, {appName:string}>({appName:['presentation',{scope:'worker'}]})

test('Bean introduction is a navigable data-backed ten-frame presentation',async({page,bean})=>{
  await page.goto(bean.baseURL+'/presentations/bean')
  await expect(page.getByRole('main')).toHaveAttribute('data-profile','presentation')
  await expect(page.getByTestId('application-shell')).toHaveAttribute('data-accent','indigo')
  await expect(page.getByRole('navigation',{name:'Primary navigation'})).toHaveCount(0)
  await expect(page.getByLabel('1 of 10: Introducing Bean')).toBeVisible()
  await expect(page.getByText('1 / 10')).toBeVisible()

  await page.getByRole('button',{name:'Speaker notes'}).click()
  await expect(page.getByRole('heading',{name:'Speaker notes'})).toBeVisible()
  await page.getByLabel('1 of 10: Introducing Bean').click()
  await page.keyboard.press('End')
  await expect(page).toHaveURL(/frame=start/)
  await expect(page.getByLabel('10 of 10: Build the next application')).toBeVisible()

  await page.getByLabel('Choose frame').selectOption('capabilities')
  await expect(page).toHaveURL(/frame=capabilities/)
  await expect(page.getByLabel('7 of 10: The presentation can contain live data')).toBeVisible()
  await expect(page.getByTestId('bar-chart')).toBeVisible()
  for(const area of ['application','data','operations','safety'])await expect(page.getByLabel(`${area}: 3`)).toBeVisible()

  await page.reload()
  await expect(page.getByLabel('7 of 10: The presentation can contain live data')).toBeVisible()
})
