import {test as base,expect} from './fixtures/bean'

const test=base.extend<{}, {appName:string}>({appName:['presentation',{scope:'worker'}]})

test('Bean introduction is a navigable data-backed five-chapter presentation',async({page,bean})=>{
  await page.goto(bean.baseURL+'/presentations/bean')
  await expect(page.getByRole('main')).toHaveAttribute('data-profile','presentation')
  await expect(page.getByTestId('application-shell')).toHaveAttribute('data-accent','indigo')
  await expect(page.getByRole('navigation',{name:'Primary navigation'})).toHaveCount(0)
  await expect(page.getByLabel('1 of 10: Bean')).toBeVisible()
  await expect(page.getByText('1.1 / 5')).toBeVisible()

  await page.getByRole('button',{name:'Speaker notes'}).click()
  await expect(page.getByRole('heading',{name:'Speaker notes'})).toBeVisible()
  await page.getByRole('button',{name:'Down'}).click()
  await expect(page).toHaveURL(/frame=thesis/)
  await expect(page.getByLabel('2 of 10: Why deterministic semantics')).toBeVisible()
  await expect(page.getByText('1.2 / 5')).toBeVisible()

  await page.getByRole('button',{name:'Next'}).click()
  await expect(page.getByLabel('3 of 10: One path to production')).toBeVisible()
  await page.getByRole('button',{name:'Down'}).click()
  await expect(page.getByLabel('4 of 10: A small, complete vocabulary')).toBeVisible()
  await page.getByLabel('4 of 10: A small, complete vocabulary').click()
  await page.keyboard.press('ArrowRight')
  await expect(page.getByLabel('6 of 10: From live data to authorized action')).toBeVisible()

  await page.keyboard.press('End')
  await expect(page).toHaveURL(/frame=start/)
  await expect(page.getByLabel('10 of 10: Start with one useful workflow')).toBeVisible()

  await page.getByLabel('Choose frame').selectOption('capabilities')
  await expect(page).toHaveURL(/frame=capabilities/)
  await expect(page.getByLabel('7 of 10: Live data, same runtime')).toBeVisible()
  await expect(page.getByTestId('bar-chart')).toBeVisible()
  for(const area of ['application','data','operations','safety'])await expect(page.getByLabel(`${area}: 3`)).toBeVisible()

  await page.reload()
  await expect(page.getByLabel('7 of 10: Live data, same runtime')).toBeVisible()
})
