import {request} from '@playwright/test'
import {test as base,expect,login} from './fixtures/bean'
const test=base.extend<{}, {appName:string}>({appName:['community',{scope:'worker'}]})

test('ordinary member changes password and revokes every session without email',async({page,bean})=>{
  const other=await request.newContext({baseURL:bean.baseURL})
  try{
    expect((await other.post('/api/auth/login',{data:{email:'user-a@example.test',password:'test-password'}})).ok()).toBeTruthy()
    await login(page,bean.baseURL,'user-a@example.test','test-password',/\/$/)
    await page.getByRole('link',{name:'Account',exact:true}).click()
    await expect(page.getByRole('heading',{name:'Account security'})).toBeVisible()
    await page.reload()
    await page.getByLabel('Current password',{exact:true}).fill('wrong-password')
    await page.getByLabel('New password',{exact:true}).fill('changed-password')
    await page.getByLabel('Confirm new password',{exact:true}).fill('changed-password')
    await page.getByRole('button',{name:'Change password',exact:true}).click()
    await expect(page.getByRole('alert')).toContainText('current password is incorrect')
    await page.getByLabel('Current password',{exact:true}).fill('test-password')
    await page.getByRole('button',{name:'Change password',exact:true}).click()
    await expect(page).toHaveURL(/\/login\?notice=password-changed$/)
    expect((await (await other.get('/api/system/session')).json()).authenticated).toBe(false)
    expect((await other.post('/api/auth/login',{data:{email:'user-a@example.test',password:'test-password'}})).status()).toBe(401)
    await login(page,bean.baseURL,'user-a@example.test','changed-password',/\/$/)
    expect((await other.post('/api/auth/login',{data:{email:'user-a@example.test',password:'changed-password'}})).ok()).toBeTruthy()
    await page.getByRole('link',{name:'Account',exact:true}).click()
    await page.getByRole('button',{name:'Sign out all devices',exact:true}).click()
    await page.getByRole('button',{name:'Sign out everywhere',exact:true}).click()
    await expect(page).toHaveURL(/\/login\?notice=sessions-revoked$/)
    expect((await (await other.get('/api/system/session')).json()).authenticated).toBe(false)
  }finally{await other.dispose()}
})
