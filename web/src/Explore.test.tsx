import {fireEvent,render,screen,waitFor} from '@testing-library/react'
import {afterEach,expect,it,vi} from 'vitest'
import {MemoryRouter} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import {Explore} from './Explore'

afterEach(()=>vi.restoreAllMocks())

it('previews and saves an ordinary typed View definition',async()=>{
  const calls:Array<{path:string;init?:RequestInit}>=[]
  vi.spyOn(globalThis,'fetch').mockImplementation(async(input,init)=>{
    const path=String(input);calls.push({path,init})
    if(path.endsWith('/api/admin/manifest'))return response({entities:{candidate:{Name:'candidate',Label:'Candidate',Fields:[{Name:'name',Label:'Name',Type:'string'},{Name:'stage',Label:'Stage',Type:'enum',Options:['applied','interview']}] }},actions:{},lifecycles:{},adminResources:{},systemAdmin:true})
    if(path.endsWith('/api/admin/definitions')){
      if(init?.method==='POST')return response({saved:true})
      return response([])
    }
    if(path.endsWith('/api/admin/explore/preview'))return response({valid:true,data:[{id:'candidate-1',name:'Avery Nguyen',stage:'interview'}],nextCursor:''})
    if(path.endsWith('/api/admin/releases/validate'))return response({valid:true,diagnostics:[],changes:[{operation:'add',path:'views.candidate_explore'}]})
    return response({})
  })
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}})}><MemoryRouter><Explore/></MemoryRouter></QueryClientProvider>)
  expect(await screen.findByTestId('explore-entity')).toHaveValue('candidate')
  expect(screen.getByTestId('explore-name')).toHaveValue('candidate_explore')
  fireEvent.change(screen.getByLabelText('Search preview'),{target:{value:'Avery'}})
  fireEvent.change(screen.getByLabelText('Filter field'),{target:{value:'stage'}})
  fireEvent.change(screen.getByLabelText('Filter value'),{target:{value:'interview'}})
  fireEvent.click(screen.getByTestId('explore-preview'))
  expect(await screen.findByText('Avery Nguyen')).toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Search preview'),{target:{value:'Morgan'}})
  expect(screen.getByTestId('explore-save')).toBeDisabled()
  expect(screen.queryByText('Avery Nguyen')).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Search preview'),{target:{value:'Avery'}})
  fireEvent.click(screen.getByTestId('explore-preview'))
  expect(await screen.findByText('Avery Nguyen')).toBeInTheDocument()
  fireEvent.click(screen.getByTestId('explore-save'))
  expect(await screen.findByText('Saved View candidate_explore to the deterministic Studio draft.')).toBeInTheDocument()
  expect(await screen.findByText('Draft validates. Semantic changes: 1.')).toBeInTheDocument()

  const preview=calls.find(call=>call.path.endsWith('/api/admin/explore/preview'))
  const previewBody=JSON.parse(String(preview?.init?.body))
  expect(previewBody).toMatchObject({name:'candidate_explore',search:'Avery',filter:{stage:'interview'},spec:{entity:'candidate',search:{fields:['name']},exposedFilters:{stage:{field:'stage',operator:'eq'}}}})
  const save=calls.find(call=>call.path.endsWith('/api/admin/definitions')&&call.init?.method==='POST')
  const saved=JSON.parse(String(save?.init?.body))
  expect(saved).toMatchObject({kind:'View',metadata:{name:'candidate_explore'},spec:{entity:'candidate',displays:{table:{type:'block',renderer:{type:'table'}}}}})
  expect(new Headers(save?.init?.headers).get('If-Match')).toBe('"draft-1"')
  await waitFor(()=>expect(calls.filter(call=>call.path.endsWith('/api/admin/definitions')).length).toBeGreaterThan(1))
})

it('fails closed when draft conflicts cannot be loaded',async()=>{
  vi.spyOn(globalThis,'fetch').mockImplementation(async(input,init)=>{
    const path=String(input)
    if(path.endsWith('/api/admin/manifest'))return response({entities:{candidate:{Name:'candidate',Label:'Candidate',Fields:[]}},actions:{},lifecycles:{},adminResources:{},systemAdmin:true})
    if(path.endsWith('/api/admin/definitions')&&!init?.method)return response({error:{message:'Definitions unavailable'}},503)
    if(path.endsWith('/api/admin/explore/preview'))return response({valid:true,data:[{id:'candidate-1'}]})
    return response({})
  })
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}})}><MemoryRouter><Explore/></MemoryRouter></QueryClientProvider>)
  await screen.findByTestId('explore-entity')
  fireEvent.click(screen.getByTestId('explore-preview'))
  await screen.findByText('candidate-1')
  expect(await screen.findByText('Definitions unavailable')).toBeInTheDocument()
  expect(screen.getByTestId('explore-save')).toBeDisabled()
})

it('does not accept an in-flight preview after the candidate changes',async()=>{
  let finishPreview:(value:Response)=>void=()=>{}
  const pendingPreview=new Promise<Response>(resolve=>{finishPreview=resolve})
  vi.spyOn(globalThis,'fetch').mockImplementation(async(input,init)=>{
    const path=String(input)
    if(path.endsWith('/api/admin/manifest'))return response({entities:{candidate:{Name:'candidate',Label:'Candidate',Fields:[{Name:'name',Label:'Name',Type:'string'}]}},actions:{},lifecycles:{},adminResources:{},systemAdmin:true})
    if(path.endsWith('/api/admin/definitions')&&!init?.method)return response([])
    if(path.endsWith('/api/admin/explore/preview'))return pendingPreview
    return response({})
  })
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}})}><MemoryRouter><Explore/></MemoryRouter></QueryClientProvider>)
  await screen.findByTestId('explore-entity')
  fireEvent.click(screen.getByTestId('explore-preview'))
  fireEvent.change(screen.getByLabelText('Search preview'),{target:{value:'changed'}})
  finishPreview(new Response(JSON.stringify({valid:true,data:[{id:'candidate-1',name:'Old preview'}]}),{status:200,headers:{'Content-Type':'application/json'}}))
  await waitFor(()=>expect(screen.getByTestId('explore-preview')).toBeEnabled())
  expect(screen.queryByText('Old preview')).not.toBeInTheDocument()
  expect(screen.getByTestId('explore-save')).toBeDisabled()
})

it('authors a grouped chart as an ordinary View',async()=>{
  const calls:Array<{path:string;init?:RequestInit}>=[]
  vi.spyOn(globalThis,'fetch').mockImplementation(async(input,init)=>{
    const path=String(input);calls.push({path,init})
    if(path.endsWith('/api/admin/manifest'))return response({entities:{candidate:{Name:'candidate',Label:'Candidate',Fields:[{Name:'stage',Label:'Stage',Type:'enum',Options:['applied','interview']}] }},actions:{},lifecycles:{},adminResources:{},systemAdmin:true})
    if(path.endsWith('/api/admin/definitions'))return init?.method==='POST'?response({saved:true}):response([])
    if(path.endsWith('/api/admin/explore/preview'))return response({valid:true,shape:'groups',data:[{stage:'interview',total:2}]})
    if(path.endsWith('/api/admin/releases/validate'))return response({valid:true,diagnostics:[],changes:[]})
    return response({})
  })
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}})}><MemoryRouter><Explore/></MemoryRouter></QueryClientProvider>)
  await screen.findByTestId('explore-entity')
  fireEvent.change(screen.getByLabelText('Result'),{target:{value:'groups'}})
  fireEvent.change(screen.getByLabelText('Group field'),{target:{value:'stage'}})
  fireEvent.change(screen.getByLabelText('Page route (optional)'),{target:{value:'/candidate-stages'}})
  fireEvent.click(screen.getByTestId('explore-preview'))
  expect(await screen.findByText('interview')).toBeInTheDocument()
  fireEvent.click(screen.getByTestId('explore-save'))
  await screen.findByText('Saved View candidate_explore to the deterministic Studio draft.')
  const saved=JSON.parse(String(calls.find(call=>call.path.endsWith('/api/admin/definitions')&&call.init?.method==='POST')?.init?.body))
  expect(saved.spec).toMatchObject({fields:['stage'],groupBy:[{field:'stage',as:'stage'}],aggregates:[{function:'count',field:'id',alias:'total'}],displays:{chart:{type:'page',route:'/candidate-stages',renderer:{type:'chart',groupField:'stage',metricField:'total'}}}})
})

function response(body:any,status=200){return Promise.resolve(new Response(JSON.stringify(body),{status,headers:{'Content-Type':'application/json','ETag':'"draft-1"'}}))}
