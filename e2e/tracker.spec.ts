import {test as base,expect} from './fixtures/bean'
import {action,login} from './helpers/api'

const test=base.extend<{}, {appName:string}>({appName:['tracker',{scope:'worker'}]})

test('issue status chart drills into actionable issue records',async({page,request,bean})=>{
  const csrf=await login(request,bean.baseURL)
  await action(request,bean.baseURL,csrf,'issue_create',{title:'Todo chart issue',status:'todo'})
  const active=await action(request,bean.baseURL,csrf,'issue_create',{title:'Active chart issue',status:'todo'})
  const activeID=(await active.json()).data.id
  expect((await action(request,bean.baseURL,csrf,'move_issue',{id:activeID,status:'in_progress'})).ok()).toBeTruthy()
  const done=await action(request,bean.baseURL,csrf,'issue_create',{title:'Done chart issue',status:'todo'})
  const doneID=(await done.json()).data.id
  expect((await action(request,bean.baseURL,csrf,'move_issue',{id:doneID,status:'in_progress'})).ok()).toBeTruthy()
  expect((await action(request,bean.baseURL,csrf,'move_issue',{id:doneID,status:'done'})).ok()).toBeTruthy()
  expect(await (await request.get(bean.baseURL+'/api/issues/kanban')).text()).toContain('Done chart issue')

  await page.goto(bean.baseURL+'/')
  await expect(page.getByRole('heading',{name:'Issue operations'})).toBeVisible()
  await expect(page.getByTestId('bar-chart')).toBeVisible()
  await expect(page.getByLabel('todo: 1')).toBeVisible()
  await page.getByRole('link',{name:'Open done records'}).click()
  await expect(page).toHaveURL(/\/issues\?status=done/)
  await expect(page.getByRole('cell',{name:'Done chart issue',exact:true})).toBeVisible()
  await expect(page.getByRole('cell',{name:'Todo chart issue',exact:true})).not.toBeVisible()
})
