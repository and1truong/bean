import {render,screen} from '@testing-library/react'
import {afterEach,describe,expect,it,vi} from 'vitest'
import {MemoryRouter} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import {Admin} from './Admin'

const manifest={appId:'default',releaseId:'release-1',version:3,entities:{article:{Name:'article',Label:'Article',Fields:[{Name:'title',Label:'Title',Type:'string',Required:true},{Name:'status',Label:'Status',Type:'enum',Required:true,Options:['draft','published']}]}},actions:{article_create:{Name:'article_create',Entity:'article',Operation:'create',Input:{}},article_update:{Name:'article_update',Entity:'article',Operation:'update',Input:{}},article_delete:{Name:'article_delete',Entity:'article',Operation:'delete',Input:{}}},adminResources:{article:{Name:'article',Entity:'article',Label:'Article',Description:'Editorial content',LabelField:'title',View:'article_list',CreateAction:'article_create',UpdateAction:'article_update',DeleteAction:'article_delete',List:{Columns:['id','title','status'],Search:['title'],Filters:['status'],Sort:[],PageSize:25},Form:{Fields:['title','status'],Readonly:['created_at','updated_at','version']},Actions:[]}}}

afterEach(()=>vi.restoreAllMocks())
function show(path:string){return render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={[path]}><Admin/></MemoryRouter></QueryClientProvider>)}

describe('Admin',()=>{
  it('renders a searchable labelled change list',async()=>{
    vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{const url=String(input);if(url.includes('/api/admin/resources/article'))return new Response(JSON.stringify({data:[{id:'a1',title:'Bean ships',status:'draft'}],nextCursor:''}),{status:200});return new Response(JSON.stringify(manifest),{status:200})})
    show('/article')
    expect(await screen.findByRole('heading',{name:'Article'})).toBeInTheDocument()
    expect(await screen.findByText('Bean ships')).toBeInTheDocument()
    expect(screen.getByRole('columnheader',{name:'Title'})).toBeInTheDocument()
    expect(screen.getByLabelText('Status')).toBeInTheDocument()
  })

  it('renders a separate typed create form',async()=>{
    vi.spyOn(globalThis,'fetch').mockResolvedValue(new Response(JSON.stringify(manifest),{status:200}))
    show('/article/new')
    expect(await screen.findByRole('heading',{name:'Add Article'})).toBeInTheDocument()
    expect(screen.getByTestId('field-title')).toBeRequired()
    expect(screen.getByTestId('field-status').tagName).toBe('SELECT')
  })

  it('renders protected system operations without secret fields',async()=>{
	vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{const url=String(input);if(url.endsWith('/summary'))return new Response(JSON.stringify({releaseId:'release-1',version:3,jobs:{failed:1},outbox:{pending:2}}),{status:200});if(url.endsWith('/users'))return new Response(JSON.stringify([{id:'u1',email:'operator@example.test',roles:'["administrator"]',tenant_id:null,created_at:'2026-08-30T00:00:00Z'}]),{status:200});if(url.endsWith('/jobs'))return new Response(JSON.stringify([{id:'j1',name:'send',status:'failed',attempts:5,max_attempts:5,last_error:'offline'}]),{status:200});if(url.endsWith('/outbox')||url.endsWith('/migrations')||url.endsWith('/releases'))return new Response('[]',{status:200});return new Response(JSON.stringify(manifest),{status:200})})
	show('/system')
	expect(await screen.findByRole('heading',{name:'System'})).toBeInTheDocument()
	expect(await screen.findByText('operator@example.test')).toBeInTheDocument()
	expect(screen.getByRole('button',{name:'Retry'})).toBeInTheDocument()
	expect(screen.queryByText(/password_hash|csrf_token/)).not.toBeInTheDocument()
  })
})
