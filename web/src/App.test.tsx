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
      if(path.includes('/api/system/manifest'))return response({localRegistration:{Action:'register_member',Route:'/register'}})
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Registration enabled'}}})
      return response({})
    }))
    renderApp('/')
    expect(await screen.findByRole('link',{name:'Sign up'})).toHaveAttribute('href','/register')
  })

  it('hides authentication navigation for an anonymous-only application',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/manifest'))return response({authNavigation:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Local tasks'}}})
      return response({})
    }))
    renderApp('/')
    expect(await screen.findByText('Local tasks')).toBeInTheDocument()
    expect(screen.queryByRole('link',{name:'Sign in'})).not.toBeInTheDocument()
  })

  it('renders allowed board movement and an arbitrary-depth task tree',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request,init?:RequestInit)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/manifest'))return response({authNavigation:false,actions:{move_task:{Transitions:{todo:['in_progress'],in_progress:['done'],done:[]}}}})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[
        {component:'ViewBlock',props:{name:'board',view:'roots',formattedFields:[],fileFields:[],presentation:{Mode:'board',TitleField:'title',BodyField:'description',GroupField:'status',OrderField:'position',MoveAction:'move_task',Columns:['todo','in_progress','done']}}},
        {component:'ViewBlock',props:{name:'tree',view:'tree',formattedFields:[],fileFields:[],presentation:{Mode:'tree',TitleField:'title',ParentField:'parent_id',OrderField:'position',LinkRoute:'/tasks/:id'}}},
      ]}})
      if(path.includes('_block=board')&&path.includes('cursor=board-next'))return response({data:[{id:'z',title:'Earlier task',description:'First',status:'todo',position:1}],nextCursor:''})
      if(path.includes('_block=board'))return response({data:[{id:'a',title:'Root A',description:'Plan',status:'todo',position:2}],nextCursor:'board-next'})
      if(path.includes('_block=tree'))return response({data:[{id:'a',title:'Root A',parent_id:null,position:1},{id:'b',title:'Child B',parent_id:'a',position:1},{id:'c',title:'Grandchild C',parent_id:'b',position:1}],nextCursor:''})
      if(path.includes('/api/actions/move_task'))return response({data:{id:'a',status:JSON.parse(String(init?.body)).status}})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/projects/p')
    expect(await screen.findByText('Grandchild C')).toBeInTheDocument()
    const todoColumn=screen.getByRole('heading',{name:'Todo'}).parentElement!
    expect(todoColumn.textContent!.indexOf('Earlier task')).toBeLessThan(todoColumn.textContent!.indexOf('Root A'))
    const presentationRequests=fetchMock.mock.calls.map(([input])=>String(input)).filter(path=>path.includes('/api/views/'))
    expect(presentationRequests).toHaveLength(3)
    expect(presentationRequests.every(path=>path.includes('limit='))).toBe(true)
    expect(presentationRequests.some(path=>path.includes('cursor=board-next'))).toBe(true)
    const status=screen.getByRole('combobox',{name:'Status for Root A'})
    expect(status).toHaveTextContent('Todo')
    expect(status).toHaveTextContent('In progress')
    expect(status).not.toHaveTextContent('Done')
    fireEvent.change(status,{target:{value:'in_progress'}})
    await waitFor(()=>expect(fetchMock.mock.calls.some(([input])=>String(input).includes('/api/actions/move_task'))).toBe(true))
    await waitFor(()=>expect(fetchMock.mock.calls.filter(([input])=>String(input).includes('_block=tree'))).toHaveLength(2))
    expect(screen.getByTestId('tree-view')).toContainElement(screen.getByRole('link',{name:'Grandchild C'}))
  })

  it('submits file Webforms as multipart data',async()=>{
    let submitted:BodyInit|null|undefined
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request,init?:RequestInit)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/manifest'))return response({authNavigation:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'WebformBlock',props:{name:'upload',webform:'upload',form:{Name:'upload',Elements:[{Name:'label',Type:'text',Required:true},{Name:'file',Type:'file',Required:true}],Confirmation:'Uploaded'}}}})
      if(path.includes('/api/webforms/upload/submit')){submitted=init?.body;return response({confirmation:'Uploaded'})}
      return response({})
    }))
    renderApp('/upload')
    fireEvent.change(await screen.findByLabelText('label'),{target:{value:'Plan'}})
    fireEvent.change(screen.getByLabelText('file'),{target:{files:[new File(['hello'],'plan.txt',{type:'text/plain'})]}})
    fireEvent.submit(screen.getByRole('button',{name:'Submit'}).closest('form')!)
    expect(await screen.findByText('Uploaded')).toBeInTheDocument()
    expect(submitted).toBeInstanceOf(FormData)
    expect((submitted as FormData).get('file')).toBeInstanceOf(File)
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

  it('resets View pagination when a reused block moves to another bound page',async()=>{
    let navigate:NavigateFunction=()=>{}
    const viewRequests:string[]=[]
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'ViewBlock',props:{name:'category_posts',view:'posts',formattedFields:[],presentation:{TitleField:'title'}}}})
      if(path.includes('/api/views/posts')){
        viewRequests.push(path)
        if(path.includes('%2Fcategories%2Fa')&&path.includes('cursor='))return response({data:[{id:'a2',title:'A page two'}],nextCursor:''})
        if(path.includes('%2Fcategories%2Fa'))return response({data:[{id:'a1',title:'A page one'}],nextCursor:'category-a-page-2'})
        return response({data:[{id:'b1',title:'B page one'}],nextCursor:''})
      }
      return response({})
    }))
    render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={['/categories/a']}><NavigationDriver capture={value=>{navigate=value}}/><App/></MemoryRouter></QueryClientProvider>)
    expect(await screen.findByText('A page one')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Next'}))
    expect(await screen.findByText('A page two')).toBeInTheDocument()
    act(()=>navigate('/categories/b'))
    expect(await screen.findByText('B page one')).toBeInTheDocument()
    const categoryBRequest=viewRequests.find(request=>request.includes('%2Fcategories%2Fb'))
    expect(categoryBRequest).not.toContain('cursor=')
  })

  it('clears cached data from the previous identity before navigating after login',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/auth/login'))return response({csrfToken:'new-csrf',user:{Roles:['member']}})
      if(path.includes('/api/system/session'))return response({authenticated:true,user:{Roles:['member']}})
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Member home'}}})
      return response({})
    }))
    const client=new QueryClient({defaultOptions:{queries:{retry:false,staleTime:Infinity}}})
    client.setQueryData(['admin-record','previous-user-secret'],{data:{private:'cached value'}})
    render(<QueryClientProvider client={client}><MemoryRouter initialEntries={['/login']}><App/></MemoryRouter></QueryClientProvider>)
    fireEvent.change(screen.getByTestId('email'),{target:{value:'member@example.test'}})
    fireEvent.change(screen.getByTestId('password'),{target:{value:'password'}})
    fireEvent.click(screen.getByTestId('login'))
    expect(await screen.findByText('Member home')).toBeInTheDocument()
    expect(client.getQueryData(['admin-record','previous-user-secret'])).toBeUndefined()
  })

  it('clears protected cached data and leaves protected routes after logout',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/auth/logout'))return response({})
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/admin/manifest'))return response({entities:{},actions:{},adminResources:{},systemAdmin:false,version:2,releaseId:'release-2'})
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

  it('resets Webform values and completion when its bound route changes',async()=>{
    let navigate:NavigateFunction=()=>{}
    const submissions:Array<{path:string;body:any}>=[]
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request,init?:RequestInit)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:true,user:{Roles:['member']}})
      if(path.includes('/api/system/page'))return response({tree:{component:'WebformBlock',props:{name:'comment_form',webform:'comment'}}})
      if(path.includes('/api/system/manifest'))return response({webforms:{comment:{Elements:[{Name:'body',Type:'textarea',Required:true}],Confirmation:'Sent'}}})
      if(path.includes('/api/webforms/comment/submit')){submissions.push({path,body:JSON.parse(String(init?.body))});return response({confirmation:'Sent'})}
      return response({})
    }))
    render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={['/posts/a']}><NavigationDriver capture={value=>{navigate=value}}/><App/></MemoryRouter></QueryClientProvider>)
    const input=await screen.findByLabelText('body')
    fireEvent.change(input,{target:{value:'Draft for A'}})
    act(()=>navigate('/posts/b'))
    expect(await screen.findByLabelText('body')).toHaveValue('')
    fireEvent.change(screen.getByLabelText('body'),{target:{value:'Comment for B'}})
    fireEvent.click(screen.getByRole('button',{name:'Submit'}))
    expect(await screen.findByText('Sent')).toBeInTheDocument()
    expect(submissions[0].path).toContain('_page=%2Fposts%2Fb')
    expect(submissions[0].body).toEqual({body:'Comment for B'})
    act(()=>navigate('/posts/a'))
    expect(await screen.findByLabelText('body')).toHaveValue('')
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

  it('resets scoped resource pagination and selection when its route changes',async()=>{
    let navigate:NavigateFunction=()=>{}
    const viewRequests:string[]=[]
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:true,user:{Roles:['editor']}})
      if(path.includes('/api/system/page'))return response({tree:{component:'ResourceListBlock',props:{resource:'item',view:'item_admin',name:'queue',filters:[],defaultFilters:{}}}})
      if(path.includes('/api/admin/manifest'))return response({entities:{item:{Name:'item',Label:'Item',Fields:[{Name:'body',Label:'Body',Type:'text'}]}},actions:{},adminResources:{item:{Name:'item',Entity:'item',Label:'Item',Description:'Queue',LabelField:'body',View:'item_admin',List:{Columns:['body'],Search:[],Filters:[],Sort:[],PageSize:1},Form:{Fields:[],Readonly:[]},Actions:[]}}})
      if(path.includes('/api/views/item_admin')){
        viewRequests.push(path)
        if(path.includes('%2Fblog%2FA%2Fcomments')&&path.includes('cursor='))return response({data:[{id:'a2',body:'A page two'}],nextCursor:''})
        if(path.includes('%2Fblog%2FA%2Fcomments'))return response({data:[{id:'a1',body:'A page one'}],nextCursor:'A-next'})
        return response({data:[{id:'b1',body:'B page one'}],nextCursor:''})
      }
      return response({})
    }))
    render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={['/blog/A/comments']}><NavigationDriver capture={value=>{navigate=value}}/><App/></MemoryRouter></QueryClientProvider>)
    expect(await screen.findByText('A page one')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Next'}))
    expect(await screen.findByText('A page two')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('checkbox',{name:'Select A page two'}))
    act(()=>navigate('/blog/B/comments'))
    expect(await screen.findByText('B page one')).toBeInTheDocument()
    expect(screen.getByRole('checkbox',{name:'Select B page one'})).not.toBeChecked()
    const requestB=viewRequests.find(request=>request.includes('%2Fblog%2FB%2Fcomments'))
    expect(requestB).not.toContain('cursor=')
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
