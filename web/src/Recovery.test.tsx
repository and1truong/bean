import {act,fireEvent,render,screen,waitFor} from '@testing-library/react'
import {afterEach,expect,it,vi} from 'vitest'
import {MemoryRouter,useLocation} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import App from './App'
const response=(body:unknown)=>new Response(JSON.stringify(body),{headers:{'Content-Type':'application/json'}})
afterEach(()=>{vi.unstubAllGlobals();sessionStorage.clear();localStorage.clear()})
function Location(){const location=useLocation();return <span data-testid="location">{location.pathname+location.search+location.hash}</span>}
function renderRecovery(path:string){const client=new QueryClient({defaultOptions:{queries:{retry:false}}});render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><Location/><App/></MemoryRouter></QueryClientProvider>);return client}
it('hides recovery when disabled and rejects direct UI entry',async()=>{
 vi.stubGlobal('fetch',vi.fn(async()=>response({authentication:{PasswordRecovery:false}})))
 renderRecovery('/login?recovery=request')
 expect(await screen.findByRole('heading',{name:'Password recovery is unavailable'})).toBeInTheDocument()
 expect(screen.queryByRole('button',{name:'Send reset link'})).not.toBeInTheDocument()
})
it('keeps tokens out of the URL/storage and consumes them only on explicit POST',async()=>{
 const reset=vi.fn(async()=>response({ok:true}))
 vi.stubGlobal('fetch',vi.fn((url:string,init?:RequestInit)=>{
  if(url==='/api/auth/recovery/reset'){expect(JSON.parse(String(init?.body))).toEqual({token:'secret-token',password:'new-password',confirmation:'new-password'});return reset()}
  return Promise.resolve(response({authentication:{PasswordRecovery:true}}))
 }))
 renderRecovery('/login?recovery=reset#token=secret-token')
 await screen.findByRole('button',{name:'Reset password'})
 await waitFor(()=>expect(screen.getByTestId('location')).toHaveTextContent('/login?recovery=reset'))
 expect(screen.getByTestId('location').textContent).not.toContain('secret-token')
 expect(JSON.stringify({...sessionStorage,...localStorage})).not.toContain('secret-token')
 expect(reset).not.toHaveBeenCalled()
 fireEvent.change(screen.getByLabelText('New password'),{target:{value:'new-password'}})
 fireEvent.change(screen.getByLabelText('Confirm new password'),{target:{value:'new-password'}})
 fireEvent.click(screen.getByRole('button',{name:'Reset password'}))
 expect(await screen.findByRole('heading',{name:'Sign in'})).toBeInTheDocument()
 expect(reset).toHaveBeenCalledTimes(1)
})
it('uses a generic queued response and blocks duplicate requests while pending',async()=>{
 let finish!:(response:Response)=>void
 const request=vi.fn(()=>new Promise<Response>(resolve=>{finish=resolve}))
 vi.stubGlobal('fetch',vi.fn((url:string)=>url==='/api/auth/recovery/request'?request():Promise.resolve(response({authentication:{PasswordRecovery:true}}))))
 renderRecovery('/login?recovery=request')
 await screen.findByRole('button',{name:'Send reset link'})
 fireEvent.change(screen.getByLabelText('Email'),{target:{value:'missing@example.test'}})
 const form=screen.getByRole('button',{name:'Send reset link'}).closest('form')!
 fireEvent.submit(form);fireEvent.submit(form);expect(request).toHaveBeenCalledTimes(1)
 await act(async()=>finish(response({message:'accepted'})))
 expect(await screen.findByRole('status')).toHaveTextContent('If this address belongs to an account')
})
