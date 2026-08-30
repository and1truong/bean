import {act,fireEvent,render,screen,waitFor} from '@testing-library/react'
import {afterEach,describe,it,expect,vi} from 'vitest'
import {MemoryRouter,useNavigate,type NavigateFunction} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import App from './App'
describe('App',()=>{it('renders login',()=>{render(<QueryClientProvider client={new QueryClient()}><MemoryRouter initialEntries={['/login']}><App/></MemoryRouter></QueryClientProvider>);expect(screen.getByRole('heading',{name:'Sign in'})).toBeInTheDocument()})})

describe('public rendering',()=>{
  it('renders sanitized rich text and hides privileged navigation',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',props:{title:'Post'},children:[{component:'ViewBlock',props:{name:'detail',view:'published_post',formattedFields:['body'],presentation:{Mode:'detail',TitleField:'title',BodyField:'body'}}}]}})
      if(path.includes('/api/views/published_post'))return response({data:[{id:'1',title:'Safe post',body:'<p>Safe <strong>body</strong></p>&lt;script&gt;alert(1)&lt;/script&gt;'}],nextCursor:''})
      return response({})
    }))
    renderApp('/posts/safe')
    expect(await screen.findByText('Safe post')).toBeInTheDocument()
    expect(document.querySelector('.rich-text strong')).toHaveTextContent('body')
    expect(document.querySelector('.rich-text script')).toBeNull()
    expect(screen.queryByRole('link',{name:'Admin'})).not.toBeInTheDocument()
    expect(screen.queryByRole('link',{name:'Studio'})).not.toBeInTheDocument()
    expect(screen.queryByRole('link',{name:'Sign up'})).not.toBeInTheDocument()
  })

  it('advertises signup only when local registration is enabled',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/manifest'))return response({localRegistration:{Action:'register_member'}})
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Registration enabled'}}})
      return response({})
    }))
    renderApp('/')
    expect(await screen.findByRole('link',{name:'Sign up'})).toBeInTheDocument()
  })

  it('renders an unfiltered fallback as text when a formatted field is absent',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[{component:'ViewBlock',props:{name:'detail',view:'published_post',formattedFields:['body'],presentation:{Mode:'detail',TitleField:'title',BodyField:'body'}}}]}})
      if(path.includes('/api/views/published_post'))return response({data:[{id:'1',title:'Fallback post',body:null,excerpt:'<img src=x onerror=alert(1)>Safe fallback'}],nextCursor:''})
      return response({})
    }))
    renderApp('/posts/fallback')
    expect(await screen.findByText('<img src=x onerror=alert(1)>Safe fallback')).toBeInTheDocument()
    expect(document.querySelector('.rich-text')).toBeNull()
    expect(document.querySelector('img')).toBeNull()
  })

  it('keeps same-View blocks isolated by their bound block identity',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[
        {component:'ViewBlock',props:{name:'first',view:'shared',formattedFields:[],presentation:{TitleField:'title'}}},
        {component:'ViewBlock',props:{name:'second',view:'shared',formattedFields:[],presentation:{TitleField:'title'}}},
      ]}})
      if(path.includes('_block=first'))return response({data:[{id:'1',title:'First result'}],nextCursor:''})
      if(path.includes('_block=second'))return response({data:[{id:'2',title:'Second result'}],nextCursor:''})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/two-blocks')
    expect(await screen.findByText('First result')).toBeInTheDocument()
    expect(await screen.findByText('Second result')).toBeInTheDocument()
    expect(fetchMock.mock.calls.filter(([input])=>String(input).includes('/api/views/shared'))).toHaveLength(2)
  })

  it('clears protected cached data and leaves protected routes after logout',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/auth/logout'))return response({})
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Signed out home'}}})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    const client=new QueryClient({defaultOptions:{queries:{retry:false,staleTime:Infinity}}})
    client.setQueryData(['session'],{authenticated:true,user:{Roles:['editor']}})
    client.setQueryData(['admin-manifest'],{entities:{},actions:{},adminResources:{},systemAdmin:false,version:1,releaseId:'release-1'})
    client.setQueryData(['admin-record','secret'],{data:{private:'cached value'}})
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/admin']}><App/></MemoryRouter></QueryClientProvider>)
    fireEvent.click(await screen.findByRole('button',{name:'Sign out'}))
    expect(await screen.findByText('Signed out home')).toBeInTheDocument()
    expect(client.getQueryData(['admin-manifest'])).not.toMatchObject({releaseId:'release-1'})
    expect(client.getQueryData(['admin-record','secret'])).toBeUndefined()
  })

  it('uses logout route metadata while the protected page is still pending',async()=>{
    const pendingMembers=new Promise<Response>(()=>{})
    let memberRequests=0
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/auth/logout'))return response({protected:true})
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page')&&path.includes('%2Fmembers')){
        memberRequests++
        if(memberRequests===1)return pendingMembers
        return new Response(JSON.stringify({error:{message:'Page not found'}}),{status:404,headers:{'Content-Type':'application/json'}})
      }
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Public home'}}})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    const client=new QueryClient({defaultOptions:{queries:{retry:false,staleTime:Infinity}}})
    client.setQueryData(['session'],{authenticated:true,user:{Roles:['member']}})
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/members']}><App/></MemoryRouter></QueryClientProvider>)
    fireEvent.click(await screen.findByRole('button',{name:'Sign out'}))
    expect(await screen.findByText('Public home')).toBeInTheDocument()
    expect(fetchMock.mock.calls.some(([input])=>String(input).includes('/api/auth/logout?path=%2Fmembers'))).toBe(true)
    expect(memberRequests).toBeGreaterThanOrEqual(2)
  })

  it('blocks header navigation while logout is in flight',async()=>{
    let resolveLogout:(value:Response)=>void=()=>{}
    const pendingLogout=new Promise<Response>(resolve=>{resolveLogout=resolve})
    let adminRequests=0
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/auth/logout'))return pendingLogout
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/admin/manifest')){adminRequests++;return response({})}
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Public article'}}})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    const client=new QueryClient({defaultOptions:{queries:{retry:false,staleTime:Infinity}}})
    client.setQueryData(['session'],{authenticated:true,user:{Roles:['editor']}})
    client.setQueryData(['page','/posts/public'],{tree:{component:'TextBlock',props:{text:'Public article'}}})
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/posts/public']}><App/></MemoryRouter></QueryClientProvider>)
    fireEvent.click(screen.getByRole('button',{name:'Sign out'}))
    fireEvent.click(screen.getByRole('link',{name:'Admin'}))
    expect(screen.getByText('Public article')).toBeInTheDocument()
    expect(adminRequests).toBe(0)
    resolveLogout(new Response(JSON.stringify({protected:false}),{status:200,headers:{'Content-Type':'application/json'}}))
    expect(await screen.findByRole('link',{name:'Sign in'})).toBeInTheDocument()
    expect(screen.getByText('Public article')).toBeInTheDocument()
  })

  it('tracks route changes across Shell remounts while reset completes',async()=>{
    let resolveReset:(value:Response)=>void=()=>{}
    const pendingReset=new Promise<Response>(resolve=>{resolveReset=resolve})
    let publicRequests=0
    let navigate:NavigateFunction=()=>{}
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/auth/logout'))return response({protected:false})
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page')&&path.includes('%2Fpublic')){publicRequests++;return pendingReset}
      if(path.includes('/api/admin/manifest'))return new Response(JSON.stringify({error:{message:'Forbidden'}}),{status:403,headers:{'Content-Type':'application/json'}})
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Home after logout'}}})
      return response({})
    }))
    const client=new QueryClient({defaultOptions:{queries:{retry:false,staleTime:Infinity}}})
    client.setQueryData(['session'],{authenticated:true,user:{Roles:['member']}})
    client.setQueryData(['page','/public'],{tree:{component:'TextBlock',props:{text:'Public page'}}})
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/public']}><NavigationDriver capture={value=>{navigate=value}}/><App/></MemoryRouter></QueryClientProvider>)
    fireEvent.click(screen.getByRole('button',{name:'Sign out'}))
    await waitFor(()=>expect(publicRequests).toBe(1))
    act(()=>navigate('/admin'))
    resolveReset(new Response(JSON.stringify({tree:{component:'TextBlock',props:{text:'Public page'}}}),{status:200,headers:{'Content-Type':'application/json'}}))
    expect(await screen.findByText('Home after logout')).toBeInTheDocument()
  })

  it('renders Webform elements whose optional visibility is null',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',props:{title:'Sign up'},children:[{component:'WebformBlock',props:{name:'signup_form',webform:'signup'}}]}})
      if(path.includes('/api/system/manifest'))return response({webforms:{signup:{Elements:[{Name:'display_name',Type:'text',Required:true,Visible:null,RequiredWhen:null}],Steps:null}}})
      return response({})
    }))
    renderApp('/signup')
    expect(await screen.findByLabelText('display name')).toBeInTheDocument()
  })

  it('renders a route-scoped resource list with default filters',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:true,user:{Roles:['editor']}})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',props:{title:'Moderation'},children:[{component:'ResourceListBlock',props:{resource:'item',view:'item_admin',name:'queue',filters:['status'],defaultFilters:{status:'pending'}}}]}})
      if(path.includes('/api/admin/manifest'))return response({entities:{item:{Name:'item',Label:'Item',Fields:[{Name:'body',Label:'Body',Type:'text'},{Name:'status',Label:'Status',Type:'enum',Options:['pending','approved']}]}},actions:{},adminResources:{item:{Name:'item',Entity:'item',Label:'Item',Description:'Review queue',LabelField:'body',View:'item_admin',List:{Columns:['body','status'],Search:[],Filters:['status'],Sort:[],PageSize:25},Form:{Fields:[],Readonly:[]},Actions:[]}}})
      if(path.includes('/api/views/item_admin'))return response({data:[{id:'1',body:'Needs review',status:'pending'}],nextCursor:''})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/blog/parent-1/comments')
    expect(await screen.findByText('Needs review')).toBeInTheDocument()
    expect(screen.getByLabelText('Status')).toHaveValue('pending')
    expect(fetchMock.mock.calls.some(([input])=>String(input).includes('_block=queue')&&String(input).includes('status=pending'))).toBe(true)
  })
})

function NavigationDriver({capture}:{capture:(navigate:NavigateFunction)=>void}){capture(useNavigate());return null}
function renderApp(path:string){render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={[path]}><App/></MemoryRouter></QueryClientProvider>)}
function response(body:any){return Promise.resolve(new Response(JSON.stringify(body),{status:200,headers:{'Content-Type':'application/json'}}))}

afterEach(()=>vi.unstubAllGlobals())
