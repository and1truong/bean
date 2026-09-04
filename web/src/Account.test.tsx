import {act,fireEvent,render,screen,waitFor} from '@testing-library/react'
import {afterEach,expect,it,vi} from 'vitest'
import {MemoryRouter} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import App from './App'

const response=(body:unknown)=>new Response(JSON.stringify(body),{headers:{'Content-Type':'application/json'}})
afterEach(()=>{vi.unstubAllGlobals();sessionStorage.clear()})
function setup(authenticated=true){
  const client=new QueryClient({defaultOptions:{queries:{retry:false}}})
  client.setQueryData(['session'],{authenticated,csrfToken:'session-token',user:{Email:'member@example.test',Roles:['authenticated']}})
  client.setQueryData(['private-data'],{secret:'old identity'})
  render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/admin/system/account']}><App/></MemoryRouter></QueryClientProvider>)
  return client
}
it('does not offer account mutations to an anonymous visitor',async()=>{
  vi.stubGlobal('fetch',vi.fn(async()=>response({authenticated:false})))
  setup(false)
  expect(await screen.findByRole('link',{name:'Sign in to manage your account'})).toBeInTheDocument()
  expect(screen.queryByLabelText('Current password')).not.toBeInTheDocument()
})
it('guards duplicate password changes and clears credentials and protected cache after success',async()=>{
  let finish!:(response:Response)=>void
  const change=vi.fn(()=>new Promise<Response>(resolve=>{finish=resolve}))
  let changed=false
  vi.stubGlobal('fetch',vi.fn((url:string,init?:RequestInit)=>{
    if(url==='/api/auth/password'){
      expect(new Headers(init?.headers).get('X-CSRF-Token')).toBe('session-token')
      expect(JSON.parse(String(init?.body))).toEqual({currentPassword:'old-password',password:'new-password',confirmation:'new-password'})
      return change()
    }
    return Promise.resolve(response(url==='/api/system/session'?{authenticated:!changed,csrfToken:'session-token',user:{Roles:['authenticated']}}:{}))
  }))
  const client=setup()
  fireEvent.change(screen.getByLabelText('Current password'),{target:{value:'old-password'}})
  fireEvent.change(screen.getByLabelText('New password'),{target:{value:'new-password'}})
  fireEvent.change(screen.getByLabelText('Confirm new password'),{target:{value:'new-password'}})
  const form=screen.getByRole('button',{name:'Change password'}).closest('form')!
  fireEvent.submit(form);fireEvent.submit(form)
  expect(change).toHaveBeenCalledTimes(1)
  expect(screen.getByLabelText('Current password')).toBeDisabled()
  await act(async()=>{changed=true;finish(response({ok:true}))})
  expect(await screen.findByRole('heading',{name:'Sign in'})).toBeInTheDocument()
  expect(client.getQueryData(['private-data'])).toBeUndefined()
  expect(screen.getByLabelText('Password')).toHaveValue('')
  expect(screen.getByRole('status')).toHaveTextContent('signed out on all devices')
})
it('requires confirmation to revoke sessions and keeps the account usable on failure',async()=>{
  const revoke=vi.fn(async()=>new Response(JSON.stringify({error:{message:'Please try again'}}),{status:503}))
  vi.stubGlobal('fetch',vi.fn((url:string)=>url==='/api/auth/sessions/revoke'?revoke():Promise.resolve(response(url==='/api/system/session'?{authenticated:true,csrfToken:'session-token',user:{Roles:['authenticated']}}:{}))))
  setup()
  fireEvent.click(screen.getByRole('button',{name:'Sign out all devices'}))
  expect(revoke).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button',{name:'Cancel'}))
  expect(revoke).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button',{name:'Sign out all devices'}))
  fireEvent.click(screen.getByRole('button',{name:'Sign out everywhere'}))
  expect(await screen.findByText('Please try again')).toBeInTheDocument()
  await waitFor(()=>expect(screen.getByRole('button',{name:'Sign out all devices'})).toBeEnabled())
})
