import {render,screen} from '@testing-library/react'
import {afterEach,describe,it,expect,vi} from 'vitest'
import {MemoryRouter} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import App from './App'
describe('App',()=>{it('renders login',()=>{render(<QueryClientProvider client={new QueryClient()}><MemoryRouter initialEntries={['/login']}><App/></MemoryRouter></QueryClientProvider>);expect(screen.getByRole('heading',{name:'Sign in'})).toBeInTheDocument()})})

describe('public rendering',()=>{
  it('renders sanitized rich text and hides privileged navigation',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',props:{title:'Post'},children:[{component:'ViewBlock',props:{name:'detail',view:'published_post',presentation:{Mode:'detail',TitleField:'title',BodyField:'body',RichTextFields:['body']}}}]}})
      if(path.includes('/api/views/published_post'))return response({data:[{id:'1',title:'Safe post',body:'<p>Safe <strong>body</strong></p>&lt;script&gt;alert(1)&lt;/script&gt;'}],nextCursor:''})
      return response({})
    }))
    renderApp('/posts/safe')
    expect(await screen.findByText('Safe post')).toBeInTheDocument()
    expect(document.querySelector('.rich-text strong')).toHaveTextContent('body')
    expect(document.querySelector('.rich-text script')).toBeNull()
    expect(screen.queryByRole('link',{name:'Admin'})).not.toBeInTheDocument()
    expect(screen.queryByRole('link',{name:'Studio'})).not.toBeInTheDocument()
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

function renderApp(path:string){render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={[path]}><App/></MemoryRouter></QueryClientProvider>)}
function response(body:any){return Promise.resolve(new Response(JSON.stringify(body),{status:200,headers:{'Content-Type':'application/json'}}))}

afterEach(()=>vi.unstubAllGlobals())
