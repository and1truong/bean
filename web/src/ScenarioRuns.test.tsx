import {fireEvent,render,screen,waitFor} from '@testing-library/react'
import {afterEach,expect,it,vi} from 'vitest'
import {MemoryRouter,Route,Routes} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import {ScenarioRuns,ScenarioRunDetail} from './ScenarioRuns'
import {useEditor} from './store'

const scenarios={smoke:{name:'smoke',start:'open',nodes:[{id:'open',type:'navigate',url:'http://app.test/',next:'check'},{id:'check',type:'assert',assertion:'text_present',text:'ok'}]}}
const run={ID:'run-1',AppID:'demo',Scenario:'smoke',Trigger:'api',Status:'failed',Error:'step check failed',CreatedAt:'2026-01-01T00:00:00Z',StartedAt:'2026-01-01T00:00:01Z',FinishedAt:'2026-01-01T00:00:03Z'}
const steps=[
  {ID:'step-1',NodeID:'open',Attempt:1,Status:'passed',Output:'{"url":"http://app.test/"}',Error:'',StartedAt:'2026-01-01T00:00:01Z',FinishedAt:'2026-01-01T00:00:02Z'},
  {ID:'step-2',NodeID:'check',Attempt:1,Status:'failed',Output:'',Error:'text_present not met',StartedAt:'2026-01-01T00:00:02Z',FinishedAt:'2026-01-01T00:00:03Z'},
]
const artifacts=[{ID:'art-1',StepID:'step-2',Kind:'screenshot',ContentType:'image/png',Size:4,Ref:'run-1/fail.png'}]
const events=[{ID:'e1',StepID:'step-1',Sequence:1,Kind:'step_started',Payload:'{"node":"open"}'},{ID:'e2',StepID:'step-2',Sequence:2,Kind:'assertion_result',Payload:'{"node":"check","met":false}'},{ID:'e3',StepID:'step-2',Sequence:3,Kind:'console_event',Payload:'{"text":"boom"}'}]

class EventSourceStub{
  listeners=new Map<string,()=>void>()
  url:string
  constructor(url:string){this.url=url;instances.push(this)}
  addEventListener(kind:string,listener:()=>void){this.listeners.set(kind,listener)}
  close(){closed.push(this.url)}
}
const instances:EventSourceStub[]=[]
const closed:string[]=[]

vi.stubGlobal('EventSource',EventSourceStub)

afterEach(()=>{vi.restoreAllMocks();vi.stubGlobal('EventSource',EventSourceStub);instances.length=0;closed.length=0;useEditor.getState().set({kind:'',name:'',spec:''})})

function respond(body:unknown,status=200){return new Response(JSON.stringify(body),{status,headers:{'content-type':'application/json'}})}

function fetchFor(map:Record<string,unknown>){
  return vi.spyOn(globalThis,'fetch').mockImplementation(async input=>{
    const url=String(input)
    const path=url.replace(/^https?:\/\/[^/]+/,'').split('?')[0]
    if(path in map)return respond(map[path])
    if(path==='/api/scenario-runs'&&url.includes('status='))return respond({runs:[]})
    return respond({},404)
  })
}

function mount(node:React.ReactNode,path='/studio/runs'){
  const client=new QueryClient({defaultOptions:{queries:{retry:false}}})
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><Routes><Route path="/studio/runs" element={node}/><Route path="/studio/runs/:id" element={<ScenarioRunDetail/>}/></Routes></MemoryRouter></QueryClientProvider>)
}

it('lists runs, filters by status, and starts a run',async()=>{
  const fetchMock=fetchFor({'/api/scenarios':scenarios,'/api/scenario-runs':{runs:[run]}})
  mount(<ScenarioRuns/>)
  const row=await screen.findByTestId(`run-${run.ID}`)
  expect(row).toHaveTextContent('smoke')
  expect(row).toHaveTextContent('failed')
  fireEvent.change(screen.getByTestId('run-status-filter'),{target:{value:'failed'}})
  await waitFor(()=>expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('status=failed'),expect.anything()))
  fireEvent.click(screen.getByTestId('start-run'))
  await waitFor(()=>expect(fetchMock).toHaveBeenCalledWith('/api/scenario-runs',expect.objectContaining({method:'POST',body:JSON.stringify({scenario:'smoke'})})))
})

it('renders the run as a readable test report',async()=>{

  fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${run.ID}`]:{run,sessions:[],steps,artifacts},[`/api/scenario-runs/${run.ID}/events`]:{events}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${run.ID}`)
  await screen.findByTestId('step-open')
  expect(screen.getByTestId('run-outcome')).toHaveTextContent('Check "check" failed')
  expect(screen.getByTestId('run-outcome')).toHaveTextContent('http://app.test/')
  expect(screen.getByTestId('step-open')).toHaveTextContent('Go to http://app.test/')
  expect(screen.getByTestId('step-check')).toHaveTextContent('"ok" appears on the page')
  expect(screen.getByTestId('step-error')).toHaveTextContent('text_present not met')
  expect(screen.getByTestId('step-screenshot')).toHaveAttribute('src',`/api/scenario-runs/${run.ID}/artifacts/art-1`)
  expect(screen.getByTestId('run-event-log-details')).not.toHaveAttribute('open')
  expect(screen.getByTestId('run-event-log')).toHaveTextContent('assertion_result')
  expect(instances.map(i=>i.url)).toContain(`/api/scenario-runs/${run.ID}/events`)
})

it('shows a positive elapsed duration for a step still running',async()=>{

  const running={...run,Status:'running',FinishedAt:'0001-01-01T00:00:00Z',Error:''}
  const runningStep={ID:'step-9',NodeID:'open',Attempt:1,Status:'running',Output:'',Error:'',StartedAt:new Date(Date.now()-1500).toISOString(),FinishedAt:'0001-01-01T00:00:00Z'}
  fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${running.ID}`]:{run:running,sessions:[],steps:[runningStep],artifacts:[]},[`/api/scenario-runs/${running.ID}/events`]:{events:[]}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${running.ID}`)
  const row=await screen.findByTestId('step-open')
  expect(row.textContent).toMatch(/\d+(\.\d+)? (ms|s)$/)
  expect(row.textContent).not.toContain('-')
})

it('classifies a missing browser binary as a setup failure and offers retry',async()=>{

  const setupRun={...run,Status:'failed',Error:"playwright: Executable doesn't exist at /home/u/.cache/ms-playwright/chromium_headless_shell-1187/chrome-linux/headless_shell"}
  const setupStep={ID:'step-9',NodeID:'open',Attempt:1,Status:'failed',Output:'',Error:"playwright: Executable doesn't exist at /home/u/.cache/ms-playwright/chromium_headless_shell-1187/chrome-linux/headless_shell\n╔ Playwright installation ╗",StartedAt:'2026-01-01T00:00:01Z',FinishedAt:'2026-01-01T00:00:02Z'}
  fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${run.ID}`]:{run:setupRun,sessions:[],steps:[setupStep],artifacts:[]},[`/api/scenario-runs/${run.ID}/events`]:{events:[]}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${run.ID}`)
  const banner=await screen.findByTestId('setup-required')
  expect(banner).toHaveTextContent('browser executable is missing')
  expect(banner).toHaveTextContent('cd browser && bunx playwright install chromium')
  expect(screen.getByTestId('step-check')).toHaveTextContent('not run')
  expect(screen.getByTestId('step-error-details')).toBeInTheDocument()
})

it('hides save-as-test with no steps and retries a failed run',async()=>{

  const failedNoSteps={...run,Status:'failed',Error:'browser missing'}
  const fetchMock=fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${run.ID}`]:{run:failedNoSteps,sessions:[],steps:[],artifacts:[]},[`/api/scenario-runs/${run.ID}/events`]:{events:[]},'/api/scenario-runs':{...run,ID:'run-2',Status:'pending',Error:''}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${run.ID}`)
  await screen.findByTestId('retry-run')
  expect(screen.queryByTestId('save-as-test')).not.toBeInTheDocument()
  fireEvent.click(screen.getByTestId('retry-run'))
  await waitFor(()=>expect(fetchMock).toHaveBeenCalledWith('/api/scenario-runs',expect.objectContaining({method:'POST',body:JSON.stringify({scenario:'smoke'})})))
})

it('drives run controls',async()=>{

  const liveRun={...run,Status:'running',FinishedAt:'',Error:''}
  const fetchMock=fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${run.ID}`]:{run:liveRun,sessions:[],steps:[steps[0]],artifacts:[]},[`/api/scenario-runs/${run.ID}/events`]:{events:[]},[`/api/scenario-runs/${run.ID}/pause`]:{}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${run.ID}`)
  fireEvent.click(await screen.findByTestId('pause-run'))
  await waitFor(()=>expect(fetchMock).toHaveBeenCalledWith(`/api/scenario-runs/${run.ID}/pause`,expect.objectContaining({method:'POST'})))
})

it('saves the run trace as an editable scenario draft',async()=>{
  const fetchMock=fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${run.ID}`]:{run,sessions:[],steps,artifacts:[]},[`/api/scenario-runs/${run.ID}/events`]:{events:[]},[`/api/scenario-runs/${run.ID}/save-as-test`]:{name:'smoke_saved_run',spec:{title:'Smoke (saved run)',start:'step_1',nodes:[{id:'step_1',type:'navigate',url:'http://app.test/'}]}}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${run.ID}`)
  fireEvent.click(await screen.findByTestId('save-as-test'))
  await waitFor(()=>expect(fetchMock).toHaveBeenCalledWith(`/api/scenario-runs/${run.ID}/save-as-test`,expect.objectContaining({method:'POST'})))
  await waitFor(()=>expect(useEditor.getState().name).toBe('smoke_saved_run'))
  expect(useEditor.getState().kind).toBe('Scenario')
  expect(useEditor.getState().spec).toContain('saved run')
})

it('proposes a repair draft with a graph diff for a failed run',async()=>{
  const fetchMock=fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${run.ID}`]:{run,sessions:[],steps,artifacts:[]},[`/api/scenario-runs/${run.ID}/events`]:{events:[]},[`/api/scenario-runs/${run.ID}/repair`]:{valid:true,name:'smoke',spec:{title:'Smoke',start:'open',nodes:[{id:'open',type:'navigate',url:'http://app.test/',next:'check'},{id:'check',type:'assert',assertion:'text_present',text:'ok'},{id:'verify',type:'wait',condition:'navigation'}]},diff:['node verify added','node open changed url']}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${run.ID}`)
  fireEvent.click(await screen.findByTestId('propose-repair'))
  await waitFor(()=>expect(fetchMock).toHaveBeenCalledWith(`/api/scenario-runs/${run.ID}/repair`,expect.objectContaining({method:'POST'})))
  const draft=await screen.findByTestId('repair-draft')
  expect(draft).toHaveTextContent('node verify added')
  expect(draft).toHaveTextContent('node open changed url')
  fireEvent.click(screen.getByTestId('load-repair-draft'))
  expect(useEditor.getState().name).toBe('smoke')
  expect(useEditor.getState().kind).toBe('Scenario')
  expect(useEditor.getState().spec).toContain('verify')
})

it('drives manual ops on a paused run',async()=>{
  const pausedRun={...run,Status:'paused',FinishedAt:'',Error:''}
  const fetchMock=fetchFor({'/api/scenarios':scenarios,[`/api/scenario-runs/${run.ID}`]:{run:pausedRun,sessions:[],steps:[steps[0]],artifacts:[]},[`/api/scenario-runs/${run.ID}/events`]:{events:[]},[`/api/scenario-runs/${run.ID}/manual`]:{snapshot:'<encoded tree>'}})
  mount(<ScenarioRunDetail/>,`/studio/runs/${run.ID}`)
  const takeover=await screen.findByTestId('takeover')
  expect(takeover).toBeInTheDocument()
  expect(screen.getByTestId('manual-op')).toHaveValue('snapshot')
  fireEvent.click(screen.getByTestId('manual-go'))
  await waitFor(()=>expect(fetchMock).toHaveBeenCalledWith(`/api/scenario-runs/${run.ID}/manual`,expect.objectContaining({method:'POST',body:JSON.stringify({op:'snapshot'})})))
  expect(await screen.findByTestId('manual-result')).toHaveTextContent('encoded tree')
})
