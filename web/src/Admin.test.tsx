import {fireEvent,render,screen,waitFor} from '@testing-library/react'
import {afterEach,describe,expect,it,vi} from 'vitest'
import {MemoryRouter} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import {Admin} from './Admin'

const manifest={appId:'default',releaseId:'release-1',version:3,entities:{article:{Name:'article',Label:'Article',Fields:[{Name:'title',Label:'Title',Type:'string',Required:true},{Name:'status',Label:'Status',Type:'enum',Required:true,Options:['draft','published']},{Name:'file',Label:'File',Type:'file',Required:false}]}},actions:{article_create:{Name:'article_create',Entity:'article',Operation:'create',Derive:{file:'generated_file'},Input:{}},article_update:{Name:'article_update',Entity:'article',Operation:'update',Derive:{title:'generated_title'},Input:{}},article_delete:{Name:'article_delete',Entity:'article',Operation:'delete',Input:{}}},lifecycles:{article_flow:{Name:'article_flow',Entity:'article',StateField:'status',Initial:'draft',Transitions:{draft:['published'],published:[]}}},adminResources:{article:{Name:'article',Entity:'article',Label:'Article',Description:'Editorial content',LabelField:'title',View:'article_list',CreateAction:'article_create',UpdateAction:'article_update',DeleteAction:'article_delete',List:{Columns:['id','title','status'],Search:['title'],Filters:['status'],Sort:[],PageSize:25},Form:{Fields:['title','status','file'],Readonly:['created_at','updated_at','version']},Actions:[]}}}

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
    const title=screen.getByTestId('field-title') as HTMLInputElement
    expect(title).toBeRequired()
    const requiredMarker=title.labels?.[0]?.querySelector('[aria-hidden="true"]')
    expect(title.labels?.[0]).toHaveTextContent(/^Title \*$/)
    expect(requiredMarker).toHaveTextContent('*')
    expect(requiredMarker).toHaveClass('text-destructive')
    expect(screen.getByTestId('field-status').tagName).toBe('SELECT')
    expect(screen.getByTestId('field-status')).toHaveValue('draft')
    expect(screen.queryByTestId('field-file')).not.toBeInTheDocument()
  })

  it('submits generated navigation placement fields with a record Action',async()=>{
    const navigationManifest:any=structuredClone(manifest)
    navigationManifest.entities.article.Navigation={LabelField:'title',Destination:{View:'articles',Display:'detail'},Menus:['book_contents']}
    let submitted:any
    vi.spyOn(globalThis,'fetch').mockImplementation(async(input,init)=>{const url=String(input);if(url.includes('/api/admin/navigation/article/_new'))return new Response(JSON.stringify({instances:[{menu:'book_contents',ownerId:'book-1',ownerLabel:'Building Bean',items:[{ID:'chapter-1',Label:'Chapter 1',Level:1}]}],truncated:false}),{status:200});if(url.includes('/api/actions/article_create')){submitted=JSON.parse(String(init?.body));return new Response(JSON.stringify({error:{message:'captured'}}),{status:409,headers:{'Content-Type':'application/json'}})}return new Response(JSON.stringify(navigationManifest),{status:200})})
    show('/article/new')
    fireEvent.click(await screen.findByRole('checkbox',{name:'Book contents — Building Bean'}))
    fireEvent.change(screen.getByLabelText('Parent'),{target:{value:'chapter-1'}})
    fireEvent.change(screen.getByLabelText('Weight'),{target:{value:'30'}})
    fireEvent.change(screen.getByLabelText('Label override'),{target:{value:'Read next'}})
    fireEvent.change(screen.getByTestId('field-title'),{target:{value:'Navigation'}})
    fireEvent.click(screen.getByTestId('create-article'))
    await waitFor(()=>expect(submitted).toBeDefined())
    expect(submitted._navigation).toEqual({placements:[{menu:'book_contents',ownerId:'book-1',parentId:'chapter-1',weight:30,labelOverride:'Read next'}]})
  })

  it('masks sensitive selection Action inputs',async()=>{
    const actionManifest:any=structuredClone(manifest)
    actionManifest.actions.rotate_token={Name:'rotate_token',Entity:'article',Operation:'update',Input:{id:{Name:'id',Type:'uuid'},token:{Name:'token',Label:'Access token',Type:'string',Sensitive:true}}}
    actionManifest.adminResources.article.Actions=['rotate_token']
    vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{const url=String(input);if(url.includes('/api/admin/resources/article'))return new Response(JSON.stringify({data:[{id:'a1',title:'Bean ships',status:'draft'}],nextCursor:''}),{status:200});return new Response(JSON.stringify(actionManifest),{status:200})})
    show('/article')
    fireEvent.click(await screen.findByRole('checkbox',{name:'Select Bean ships'}))
    expect(await screen.findByLabelText('Access token')).toHaveAttribute('type','password')
  })

  it('surfaces server field errors on Admin inputs',async()=>{
    vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{const url=String(input);if(url.includes('/api/actions/article_create'))return new Response(JSON.stringify({error:{message:'invalid input',fields:{title:'must be unique'}}}),{status:400,headers:{'Content-Type':'application/json'}});return new Response(JSON.stringify(manifest),{status:200,headers:{'Content-Type':'application/json'}})})
    show('/article/new')
    fireEvent.change(await screen.findByTestId('field-title'),{target:{value:'Duplicate'}})
    fireEvent.click(screen.getByTestId('create-article'))
    expect(await screen.findByText('must be unique')).toHaveAttribute('role','alert')
  })

  it('uses the AdminResource View for current file downloads',async()=>{
    vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{const url=String(input);if(url.includes('/api/admin/resources/article/a1'))return new Response(JSON.stringify({data:{id:'a1',title:'Bean ships',status:'draft',file:'blob-1'}}),{status:200});return new Response(JSON.stringify(manifest),{status:200})})
    show('/article/a1')
    const download=await screen.findByRole('link',{name:'Download current file'})
    expect(download).toHaveAttribute('href','/api/files/blob-1?view=article_list')
    const status=screen.getByRole('textbox',{name:'Status'}) as HTMLInputElement
    expect(status).toHaveAttribute('readonly')
    expect(status.labels?.[0]).toHaveTextContent(/^Status \*$/)
    expect(screen.queryByTestId('field-title')).not.toBeInTheDocument()
  })

  it('renders protected system operations without secret fields',async()=>{
	vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{const url=String(input);if(url.endsWith('/summary'))return new Response(JSON.stringify({releaseId:'release-1',version:3,jobs:{failed:1},outbox:{pending:2}}),{status:200});if(url.endsWith('/users'))return new Response(JSON.stringify([{id:'u1',email:'operator@example.test',roles:'["administrator"]',tenant_id:null,created_at:'2026-08-30T00:00:00Z'}]),{status:200});if(url.endsWith('/jobs'))return new Response(JSON.stringify([{id:'j1',name:'send',status:'failed',attempts:5,max_attempts:5,last_error:'offline'}]),{status:200});if(url.endsWith('/outbox')||url.endsWith('/migrations')||url.endsWith('/releases'))return new Response('[]',{status:200});return new Response(JSON.stringify(manifest),{status:200})})
	show('/system')
	expect(await screen.findByRole('heading',{name:'System'})).toBeInTheDocument()
	expect(await screen.findByText('operator@example.test')).toBeInTheDocument()
	const retry=screen.getByRole('button',{name:'Retry'})
	expect(retry).toBeInTheDocument()
	fireEvent.click(retry)
	expect(screen.getByRole('alertdialog')).toBeInTheDocument()
	expect(screen.getByRole('button',{name:'Confirm retry'})).toBeInTheDocument()
	expect(screen.queryByText(/password_hash|csrf_token/)).not.toBeInTheDocument()
  })
})
