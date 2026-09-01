import {test as base,expect} from './fixtures/bean'

const test=base.extend<{}, {appName:string}>({appName:['ats',{scope:'worker'}]})

test('applicant tracker opens populated and operational',async({page,bean})=>{
  await page.goto(bean.baseURL+'/')
  await expect(page.getByRole('link',{name:'Acme Recruiting'})).toBeVisible()
  await expect(page.getByTestId('application-shell')).toHaveAttribute('data-accent','indigo')
  await expect(page.getByTestId('metric-value')).toHaveText('18')
  await expect(page.getByTestId('timeline-view')).toBeVisible()

  await page.getByRole('searchbox',{name:'Search candidate pipeline'}).fill('Avery')
  await page.getByRole('button',{name:'Search'}).click()
  await expect(page.getByRole('link',{name:'Avery Nguyen 1',exact:true}).first()).toBeVisible()

  const status=page.getByRole('combobox',{name:'Status for Avery Nguyen 1',exact:true})
  await status.selectOption('screen')
  await expect(status).toHaveValue('screen')

  await page.getByRole('link',{name:'Avery Nguyen 1',exact:true}).first().click()
  await expect(page).toHaveURL(/\/candidates\//)
  await expect(page.getByRole('heading',{name:'Avery Nguyen 1',exact:true})).toBeVisible()
})
