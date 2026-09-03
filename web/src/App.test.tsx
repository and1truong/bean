import {act,fireEvent,render,screen,waitFor} from '@testing-library/react'
import {afterEach,describe,it,expect,vi} from 'vitest'
import {MemoryRouter,useNavigate,type NavigateFunction} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import App,{controlInputValue,controlQueryValue,evaluate} from './App'
describe('App',()=>{it('renders login',()=>{render(<QueryClientProvider client={new QueryClient()}><MemoryRouter initialEntries={['/login']}><App/></MemoryRouter></QueryClientProvider>);expect(screen.getByRole('heading',{name:'Sign in'})).toBeInTheDocument()})})

describe('client expressions',()=>{
  it('implements list membership and fails loudly for unknown operators',()=>{
    expect(evaluate({Op:'in',Left:{Source:'input',Name:'status'},Right:{Source:'literal',Literal:['draft','ready']}},{status:'ready'})).toBe(true)
    expect(evaluate({Op:'not_in',Left:{Source:'input',Name:'status'},Right:{Source:'literal',Literal:['draft']}},{status:'ready'})).toBe(true)
    const adult:import('./api').Expression={Op:'gte',Left:{Source:'input',Name:'age'},Right:{Source:'literal',Literal:18}}
    expect(evaluate(adult,{})).toBe(false)
    expect(evaluate(adult,{age:21})).toBe(true)
    expect(()=>evaluate({...adult,Right:{Source:'literal',Literal:'18'}},{})).toThrow('comparison requires numbers')
    expect(()=>evaluate({Op:'future',Left:{Source:'literal',Literal:true},Right:{Source:'literal',Literal:true}},{})).toThrow('unsupported client expression operator')
  })
})

describe('View datetime controls',()=>{
  it('preserves the original RFC3339 instant when the local control is unchanged',()=>{
    const filter={Field:'published_at',Type:'datetime'}
    const original='2026-11-01T06:30:00Z'
    expect(controlQueryValue(controlInputValue(original,filter),filter,original)).toBe(original)
  })
})

describe('public rendering',()=>{
  it('renders and navigates an accessible semantic Sequence',async()=>{
    const print=vi.fn();vi.stubGlobal('print',print)
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Sequence',props:{title:'Introducing Bean',description:'Deterministic applications',profile:'presentation',aspectRatio:'wide'},children:[
        {component:'SequenceFrame',props:{name:'opening',title:'Bean',layout:'title',notes:''},children:[{component:'Panel',props:{layout:'single-column'},children:[{component:'Region',props:{name:'main'},children:[{component:'ContentBlock',props:{content:[{Type:'heading',Text:'Build with intent'},{Type:'paragraph',Text:'Compile to deterministic software.'}]}}]}]}]},
        {component:'SequenceFrame',props:{name:'architecture',title:'Architecture',layout:'architecture',notes:'Explain every deterministic boundary.'},children:[{component:'Panel',props:{layout:'single-column'},children:[{component:'Region',props:{name:'main'},children:[{component:'ContentBlock',props:{content:[{Type:'diagram',Items:['Intent','Compiler','Runtime'],Direction:'horizontal'}]}}]}]}]},
        {component:'SequenceFrame',props:{name:'closing',title:'Start building',layout:'closing',notes:''},children:[{component:'Panel',props:{layout:'single-column'},children:[{component:'Region',props:{name:'main'},children:[{component:'ContentBlock',props:{content:[{Type:'callout',Text:'Five minutes to a useful app.',Tone:'success'}]}}]}]}]},
      ]}})
      return response({})
    }))
    renderApp('/presentations/bean')
    expect(await screen.findByRole('heading',{name:'Bean',level:1})).toBeVisible()
    expect(screen.getByText('1 / 3')).toBeVisible()
    expect(screen.queryByRole('navigation',{name:'Primary navigation'})).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Next'}))
    expect(screen.getByRole('heading',{name:'Architecture',level:1})).toBeVisible()
    expect(screen.getByText('2 / 3')).toBeVisible()
    const notes=document.querySelector('.bean-speaker-notes')
    expect(notes).not.toBeVisible()
    fireEvent.click(screen.getByRole('button',{name:'Speaker notes'}))
    expect(notes).toBeVisible()
    expect(screen.getByText('Explain every deterministic boundary.')).toBeVisible()
    fireEvent.keyDown(window,{key:'End'})
    expect(screen.getByRole('heading',{name:'Start building',level:1})).toBeVisible()
    expect(screen.getByText('3 / 3')).toBeVisible()
    fireEvent.click(screen.getByRole('button',{name:'Print'}))
    expect(print).toHaveBeenCalledOnce()
    expect(document.querySelectorAll('.bean-sequence-frame')).toHaveLength(3)
    expect(document.title).toBe('Introducing Bean')
  })

  it('opens a Sequence frame by stable URL identity',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Sequence',props:{title:'Introducing Bean'},children:[
        {component:'SequenceFrame',props:{name:'opening',title:'Bean',layout:'title'},children:[{component:'Panel'}]},
        {component:'SequenceFrame',props:{name:'architecture',title:'Architecture',layout:'architecture'},children:[{component:'Panel'}]},
      ]}})
      return response({})
    }))
    renderApp('/presentations/bean?frame=architecture')
    expect(await screen.findByRole('heading',{name:'Architecture',level:1})).toBeVisible()
    expect(screen.getByLabelText('Choose frame')).toHaveValue('architecture')
  })

  it('renders an explicit error for an unknown server component',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'FutureBlock',props:{}}})
      return response({})
    }))
    renderApp('/future')
    expect(await screen.findByRole('alert')).toHaveTextContent('Unsupported render component: FutureBlock')
  })

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

  it('applies typed theme and renders metric timeline and declared search',async()=>{
	const fetchMock=vi.fn(async(input:string|URL|Request)=>{
	  const path=String(input)
	  if(path.includes('/api/system/session'))return response({authenticated:false})
	  if(path.includes('/api/system/manifest'))return response({authNavigation:false,theme:{DisplayName:'Acme Recruiting',Preset:'professional',Accent:'indigo'}})
	  if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[
		{component:'ViewBlock',props:{name:'metric',view:'metrics',formattedFields:[],fileFields:[],presentation:{Mode:'metric',MetricField:'candidate_count',MetricLabel:'Candidates'}}},
		{component:'ViewBlock',props:{name:'timeline',view:'recent',formattedFields:[],fileFields:[],presentation:{Mode:'timeline',TitleField:'name',BodyField:'note',TimeField:'applied_at',MetaFields:['stage']}}},
		{component:'ViewBlock',props:{name:'search',view:'candidates',formattedFields:[],fileFields:[],presentation:{Mode:'list',TitleField:'name',SearchFields:['name','email']}}},
	  ]}})
	  if(path.includes('_block=metric'))return response({data:[{candidate_count:12}],nextCursor:''})
	  if(path.includes('_block=timeline')&&path.includes('cursor=timeline-next'))return response({data:[{id:'c',name:'Alex Kim',note:'Offer accepted',stage:'hired',applied_at:'2026-09-01T10:00:00Z'}],nextCursor:''})
	  if(path.includes('_block=timeline'))return response({data:[{id:'a',name:'Jamie Chen',note:'Interview scheduled',stage:'interview',applied_at:'2026-08-31T10:00:00Z'}],nextCursor:'timeline-next'})
	  if(path.includes('_block=search')&&path.includes('q=Jane'))return response({data:[{id:'b',name:'Jane Doe',email:'jane@example.test'}],nextCursor:''})
	  if(path.includes('_block=search'))return response({data:[{id:'a',name:'Jamie Chen'}],nextCursor:''})
	  return response({})
	})
	vi.stubGlobal('fetch',fetchMock)
	renderApp('/')
	expect(await screen.findByRole('link',{name:'Acme Recruiting'})).toBeInTheDocument()
	expect(screen.getByTestId('application-shell')).toHaveAttribute('data-preset','professional')
	expect(screen.getByTestId('application-shell')).toHaveAttribute('data-accent','indigo')
	expect(await screen.findByText('12')).toBeInTheDocument()
	expect(screen.getByText('Interview scheduled')).toBeInTheDocument()
	expect(screen.getByText('Aug 31, 2026')).toBeInTheDocument()
	fireEvent.click(screen.getAllByRole('button',{name:'Next'})[0])
	expect(await screen.findByText('Offer accepted')).toBeInTheDocument()
	expect(fetchMock.mock.calls.some(([input])=>String(input).includes('cursor=timeline-next'))).toBe(true)
	fireEvent.change(screen.getByRole('searchbox',{name:'Search candidates'}),{target:{value:'Jane'}})
	fireEvent.submit(screen.getByRole('searchbox',{name:'Search candidates'}).closest('form')!)
	expect(await screen.findByText('Jane Doe')).toBeInTheDocument()
	expect(fetchMock.mock.calls.some(([input])=>String(input).includes('q=Jane'))).toBe(true)
  })

  it('renders a null aggregate metric as the declared empty state',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[{component:'ViewBlock',props:{name:'pipeline',view:'pipeline_value',display:{Type:'block',EmptyState:'No pipeline value.',Renderer:{Type:'metric',MetricField:'pipeline_amount',MetricLabel:'Pipeline value'},Pager:{Type:'none'}}}}]}})
      if(path.includes('/api/views/pipeline_value'))return response({data:[{pipeline_amount:null}],nextCursor:''})
      return response({})
    }))
    renderApp('/')
    expect(await screen.findByText('No pipeline value.')).toBeInTheDocument()
    expect(screen.queryByTestId('metric-value')).not.toBeInTheDocument()
  })

  it('switches compatible grouped displays and renders a calendar',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[
        {component:'ViewBlock',props:{name:'stage_chart',view:'candidates_by_stage',displayName:'chart',display:{Type:'block',Renderer:{Type:'chart',GroupField:'stage',MetricField:'candidate_count',MetricLabel:'Candidates by stage'},Pager:{Type:'none'},Drill:{View:'candidate_records',Display:'page',Route:'/candidates',Bindings:[{Source:'group',Name:'stage',Filter:'stage'}]}},displays:{chart:{Type:'block',Renderer:{Type:'chart',GroupField:'stage',MetricField:'candidate_count',MetricLabel:'Candidates by stage'},Pager:{Type:'none'},Drill:{View:'candidate_records',Display:'page',Route:'/candidates',Bindings:[{Source:'group',Name:'stage',Filter:'stage'}]}},table:{Type:'block',Renderer:{Type:'table',Fields:[{Field:'stage',Label:'Stage'},{Field:'candidate_count',Label:'Candidates'}]},Pager:{Type:'none'}}},fieldTypes:{candidate_count:'integer'}}},
        {component:'ViewBlock',props:{name:'booking_calendar',view:'resource_calendar',display:{Type:'block',Renderer:{Type:'calendar',TitleField:'resource.name',TimeField:'start_at',EndField:'end_at'},Pager:{Type:'none'}}}},
        {component:'ViewBlock',props:{name:'milestone_calendar',view:'milestone_calendar',display:{Type:'block',Renderer:{Type:'calendar',TitleField:'title',TimeField:'starts_at'},Pager:{Type:'none'}}}},
      ]}})
      if(path.includes('/api/views/candidates_by_stage'))return response({data:[{stage:'interview',candidate_count:4},{stage:'offer',candidate_count:2}],nextCursor:'',shape:'groups'})
      if(path.includes('/api/views/resource_calendar'))return response({data:[{id:'b1','resource.name':'Room A',start_at:'2026-09-03T09:00:00Z',end_at:'2026-09-03T10:00:00Z'}],nextCursor:'',shape:'records'})
      if(path.includes('/api/views/milestone_calendar'))return response({data:[{id:'m1',title:'Launch',starts_at:'2026-09-04T09:00:00Z'}],nextCursor:'',shape:'records'})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/')
    expect(await screen.findByTestId('bar-chart')).toHaveAccessibleName('Candidates by stage')
    expect(screen.getByLabelText('interview: 4')).toBeInTheDocument()
    expect(screen.getByRole('link',{name:'Open interview records'})).toHaveAttribute('href','/candidates?stage=interview')
    expect((await screen.findAllByTestId('calendar-view'))[0]).toHaveTextContent('Room A')
    expect(await screen.findByText('Sep 4, 2026')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Display'),{target:{value:'table'}})
    expect(await screen.findByRole('columnheader',{name:'Candidates'})).toBeInTheDocument()
    await waitFor(()=>expect(fetchMock.mock.calls.some(([input])=>String(input).includes('_viewDisplay=table'))).toBe(true))
  })

  it('fans a typed page filter out only to declared blocks',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',props:{title:'Recruiting overview',filters:{pipeline_stage:{Label:'Stage',Type:'enum',Widget:'select',Options:['applied','interview']}}},children:[
        {component:'ViewBlock',props:{name:'candidate_metric',view:'candidate_total',pageFilters:{stage:'pipeline_stage'},presentation:{Mode:'metric',MetricField:'candidate_count'}}},
        {component:'ViewBlock',props:{name:'activity',view:'recent_activity',presentation:{Mode:'list',TitleField:'subject'}}},
      ]}})
      if(path.includes('/api/views/candidate_total'))return response({data:[{candidate_count:path.includes('stage=interview')?4:12}],nextCursor:''})
      if(path.includes('/api/views/recent_activity'))return response({data:[{id:'a',subject:'Interview scheduled'}],nextCursor:''})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/')
    expect(await screen.findByText('12')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Stage'),{target:{value:'interview'}})
    fireEvent.click(screen.getByRole('button',{name:'Apply filters'}))
    expect(await screen.findByText('4')).toBeInTheDocument()
    const viewRequests=fetchMock.mock.calls.map(([input])=>String(input)).filter(path=>path.includes('/api/views/'))
    expect(viewRequests.some(path=>path.includes('candidate_total')&&path.includes('stage=interview'))).toBe(true)
    expect(viewRequests.filter(path=>path.includes('recent_activity')).every(path=>!path.includes('stage='))).toBe(true)
  })

  it('applies page defaults, serializes datetimes, and resets targeted cursors',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',props:{filters:{pipeline_stage:{Label:'Stage',Type:'enum',Widget:'select',Options:['applied','interview'],Default:'interview'},applied_after:{Label:'Applied after',Type:'datetime',Widget:'date'}}},children:[
        {component:'ViewBlock',props:{name:'candidates',view:'candidate_records',pageFilters:{stage:'pipeline_stage',applied_after:'applied_after'},display:{Type:'block',Renderer:{Type:'table',Fields:[{Field:'name',Label:'Candidate'}]},Pager:{Type:'cursor',PageSize:1}}}},
      ]}})
      if(path.includes('/api/views/candidate_records')&&path.includes('cursor=next'))return response({data:[{id:'c2',name:'Grace'}],nextCursor:''})
      if(path.includes('/api/views/candidate_records'))return response({data:[{id:'c1',name:'Ada'}],nextCursor:'next'})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/')
    expect(await screen.findByText('Ada')).toBeInTheDocument()
    const initialRequest=fetchMock.mock.calls.map(([input])=>String(input)).find(path=>path.includes('/api/views/candidate_records'))!
    expect(initialRequest).toContain('stage=interview')
    fireEvent.click(screen.getByRole('button',{name:'Next'}))
    expect(await screen.findByText('Grace')).toBeInTheDocument()
    const localDateTime='2026-09-01T09:30:00'
    fireEvent.change(screen.getByLabelText('Applied after'),{target:{value:localDateTime}})
    fireEvent.click(screen.getByRole('button',{name:'Apply filters'}))
    await waitFor(()=>expect(fetchMock.mock.calls.some(([input])=>String(input).includes('applied_after='))).toBe(true))
    const filteredRequests=fetchMock.mock.calls.map(([input])=>String(input)).filter(path=>path.includes('/api/views/candidate_records')&&path.includes('applied_after='))
    expect(filteredRequests.every(path=>!path.includes('cursor='))).toBe(true)
    expect(new URL(filteredRequests.at(-1)!,'http://bean').searchParams.get('applied_after')).toBe(new Date(localDateTime).toISOString())
  })

  it('runs a selected record Action through the bounded batch contract',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request,init?:RequestInit)=>{
      void init
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:true,user:{Roles:['administrator']}})
      if(path.includes('/api/system/manifest'))return response({actions:{move_candidate:{Name:'move_candidate',Entity:'candidate',Operation:'transition',Confirm:'Move selected candidates?',Input:{id:{Name:'id',Type:'uuid'},stage:{Name:'stage',Label:'Stage',Type:'enum',Options:['applied','interview']},notify:{Name:'notify',Label:'Notify recruiter',Type:'boolean',Required:true},priority:{Name:'priority',Label:'Priority',Type:'integer'},score:{Name:'score',Label:'Score',Type:'decimal'},scheduled_at:{Name:'scheduled_at',Label:'Scheduled at',Type:'datetime'}}}},lifecycles:{}})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[{component:'ViewBlock',props:{name:'records',view:'candidate_records',display:{Type:'block',Selection:'multiple',Actions:['move_candidate'],Renderer:{Type:'table',Fields:[{Field:'name',Label:'Candidate'},{Field:'stage',Label:'Stage'}]},Pager:{Type:'none'}}}}]}})
      if(path.includes('/api/actions/move_candidate/batch'))return response({data:{results:[{id:'c1',ok:true}]}})
      if(path.includes('/api/views/candidate_records'))return response({data:[{id:'c1',name:'Ada',stage:'applied'}],nextCursor:''})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/')
    fireEvent.click(await screen.findByRole('checkbox',{name:'Select Ada'}))
    fireEvent.change(screen.getByLabelText('Stage'),{target:{value:'interview'}})
    fireEvent.change(screen.getByLabelText('Priority'),{target:{value:'3'}})
    fireEvent.change(screen.getByLabelText('Score'),{target:{value:'1.25'}})
    const scheduledAt='2026-09-04T10:30:00'
    fireEvent.change(screen.getByLabelText('Scheduled at'),{target:{value:scheduledAt}})
    fireEvent.click(screen.getByRole('button',{name:'Run for 1'}))
    expect(await screen.findByText('Move selected candidates?')).toBeInTheDocument()
    expect(fetchMock.mock.calls.some(([input])=>String(input).includes('/api/actions/move_candidate/batch'))).toBe(false)
    fireEvent.click(screen.getByRole('button',{name:'Confirm'}))
    await waitFor(()=>expect(fetchMock.mock.calls.some(([input])=>String(input).includes('/api/actions/move_candidate/batch'))).toBe(true))
    const call=fetchMock.mock.calls.find(([input])=>String(input).includes('/api/actions/move_candidate/batch'))!
    expect(JSON.parse(String(call[1]?.body))).toEqual({ids:['c1'],values:{stage:'interview',notify:false,priority:3,score:'1.25',scheduled_at:new Date(scheduledAt).toISOString()}})
  })

  it('clears record selection when cursor paging changes the visible rows',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:true,user:{Roles:['administrator']}})
      if(path.includes('/api/system/manifest'))return response({actions:{move_candidate:{Name:'move_candidate',Entity:'candidate',Operation:'transition',Input:{id:{Name:'id',Type:'uuid'}}}},lifecycles:{}})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[{component:'ViewBlock',props:{name:'records',view:'candidate_records',display:{Type:'block',Selection:'multiple',Actions:['move_candidate'],Renderer:{Type:'table',Fields:[{Field:'name',Label:'Candidate'}]},Pager:{Type:'cursor',PageSize:1}}}}]}})
      if(path.includes('/api/views/candidate_records')&&path.includes('cursor=next'))return response({data:[{id:'c2',name:'Grace'}],nextCursor:''})
      if(path.includes('/api/views/candidate_records'))return response({data:[{id:'c1',name:'Ada'}],nextCursor:'next'})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/')
    fireEvent.click(await screen.findByRole('checkbox',{name:'Select Ada'}))
    expect(screen.getByRole('button',{name:'Run for 1'})).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Next'}))
    expect(await screen.findByText('Grace')).toBeInTheDocument()
    await waitFor(()=>expect(screen.queryByRole('button',{name:'Run for 1'})).not.toBeInTheDocument())
  })

  it('renders allowed board movement and an arbitrary-depth task tree',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request,init?:RequestInit)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/manifest'))return response({authNavigation:false,actions:{move_task:{Lifecycle:'task_flow'}},lifecycles:{task_flow:{Name:'task_flow',Entity:'task',StateField:'status',Initial:'todo',Transitions:{todo:['in_progress'],in_progress:['done'],done:[]}}}})
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

  it('rejects a structured View response that exceeds 200 rows',async()=>{
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/manifest'))return response({authNavigation:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'ViewBlock',props:{name:'board',view:'tasks',formattedFields:[],fileFields:[],presentation:{Mode:'board',TitleField:'title',GroupField:'status',Columns:['todo']}}}})
      if(path.includes('cursor=next'))return response({data:Array.from({length:100},(_,index)=>({id:'next-'+index,title:'Task',status:'todo'})),nextCursor:''})
      if(path.includes('/api/views/tasks'))return response({data:Array.from({length:150},(_,index)=>({id:'first-'+index,title:'Task',status:'todo'})),nextCursor:'next'})
      return response({})
    }))
    renderApp('/tasks')
    expect(await screen.findByRole('alert')).toHaveTextContent('This View display supports at most 200 rows.')
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

  it('keeps sibling View state mounted while filter URL state refetches the Page',async()=>{
    let resolveFilteredPage:(value:Response)=>void=()=>{}
    const filteredPage=new Promise<Response>(resolve=>{resolveFilteredPage=resolve})
    const tree={component:'Page',children:[
      {component:'ViewBlock',props:{name:'first',block:'first',view:'shared',filters:{status:{Field:'status',Type:'string'}},display:{Type:'block',Renderer:{Type:'list',TitleField:'title'},Controls:[{Filter:'status',Label:'Status',Widget:'text'}],Pager:{Type:'none'}}}},
      {component:'ViewBlock',props:{name:'second',block:'second',view:'shared',presentation:{Mode:'list',TitleField:'title'}}},
    ]}
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page')&&path.includes('first.status=open'))return filteredPage
      if(path.includes('/api/system/page'))return response({tree})
      if(path.includes('_block=first'))return response({data:[{id:'1',title:'First result'}],nextCursor:''})
      if(path.includes('_block=second')&&path.includes('cursor=second-next'))return response({data:[{id:'3',title:'Second page'}],nextCursor:''})
      if(path.includes('_block=second'))return response({data:[{id:'2',title:'Second result'}],nextCursor:'second-next'})
      return response({})
    }))
    renderApp('/two-blocks')
    expect(await screen.findByText('Second result')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Next'}))
    expect(await screen.findByText('Second page')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Status'),{target:{value:'open'}})
    fireEvent.click(screen.getByRole('button',{name:'Apply'}))
    expect(screen.getByText('Second page')).toBeInTheDocument()
    await act(async()=>resolveFilteredPage(await response({tree})))
    expect(screen.getByText('Second page')).toBeInTheDocument()
  })

  it('does not retain the previous Page tree while another path loads',async()=>{
    let navigate:NavigateFunction=()=>{}
    let resolveNextPage:(value:Response)=>void=()=>{}
    const nextPage=new Promise<Response>(resolve=>{resolveNextPage=resolve})
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page')&&path.includes('%2Fnext'))return nextPage
      if(path.includes('/api/system/page'))return response({tree:{component:'TextBlock',props:{text:'Previous page'}}})
      return response({})
    }))
    render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={['/previous']}><NavigationDriver capture={value=>{navigate=value}}/><App/></MemoryRouter></QueryClientProvider>)
    expect(await screen.findByText('Previous page')).toBeInTheDocument()
    act(()=>navigate('/next'))
    expect(screen.queryByText('Previous page')).not.toBeInTheDocument()
    expect(screen.getByText('Loading…')).toBeInTheDocument()
    await act(async()=>resolveNextPage(await response({tree:{component:'TextBlock',props:{text:'Next page'}}})))
    expect(await screen.findByText('Next page')).toBeInTheDocument()
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

  it('renders a page display table with labelled URL filters, title, and cursor pager',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[{component:'ViewBlock',props:{name:'index',view:'articles',formattedFields:[],fileFields:[],fieldTypes:{published_at:'datetime'},filters:{status:{Field:'status',Type:'enum',Options:['draft','published']},featured:{Field:'featured',Type:'boolean'},published_after:{Field:'published_at',Type:'datetime'}},display:{Type:'page',Description:'Browse articles.',Title:{Text:'Articles'},Renderer:{Type:'table',Fields:[{Field:'title',Label:'Article',LinkRoute:'/articles/:id'},{Field:'status',Label:'State'},{Field:'published_at',Label:'Published'}]},Controls:[{Filter:'status',Label:'Publication status',Widget:'select'},{Filter:'featured',Label:'Featured',Widget:'checkbox'},{Filter:'published_after',Label:'Published after',Widget:'auto'}],Pager:{Type:'cursor',PageSize:1}}}}]}})
      if(path.includes('/api/views/articles')&&path.includes('cursor=next-page'))return response({data:[{id:'2',title:'Second article',status:'published'}],nextCursor:''})
      if(path.includes('/api/views/articles')&&path.includes('status=published'))return response({data:[{id:'2',title:'Published article',status:'published'}],nextCursor:''})
      if(path.includes('/api/views/articles'))return response({data:[{id:'1',title:'Draft article',status:'draft',published_at:'2026-01-02T10:00:00Z'}],nextCursor:'next-page'})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/articles?status=draft&featured=true')
    expect(await screen.findByRole('heading',{name:'Articles'})).toBeInTheDocument()
    expect(document.title).toBe('Articles')
    expect(screen.getByText('Browse articles.')).toBeInTheDocument()
    expect(await screen.findByRole('columnheader',{name:'Article'})).toBeInTheDocument()
    expect(screen.getByRole('link',{name:'Draft article'})).toHaveAttribute('href','/articles/1')
    expect(screen.getByText('Jan 2, 2026')).toBeInTheDocument()
    expect(screen.getByLabelText('Publication status')).toHaveValue('draft')
    expect(screen.getByRole('checkbox',{name:'Featured'})).toBeChecked()
    fireEvent.click(screen.getByRole('button',{name:'Next'}))
    expect(await screen.findByText('Second article')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Previous'}))
    expect(await screen.findByText('Draft article')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Publication status'),{target:{value:'published'}})
    fireEvent.click(screen.getByRole('checkbox',{name:'Featured'}))
    const localDateTime='2026-01-02T10:30:00'
    expect(screen.getByLabelText('Published after')).toHaveAttribute('type','datetime-local')
    fireEvent.change(screen.getByLabelText('Published after'),{target:{value:localDateTime}})
    fireEvent.click(screen.getByRole('button',{name:'Apply'}))
    expect(await screen.findByText('Published article')).toBeInTheDocument()
    expect(fetchMock.mock.calls.some(([input])=>String(input).includes('_display=index')&&String(input).includes('status=published')&&String(input).includes('featured=false')&&String(input).includes('limit=1'))).toBe(true)
    const filteredRequest=fetchMock.mock.calls.map(([input])=>String(input)).find(path=>path.includes('status=published')&&path.includes('published_after='))
    expect(new URL(filteredRequest!,'http://bean').searchParams.get('published_after')).toBe(new Date(localDateTime).toISOString())
  })

  it('resolves a detail display heading and browser title from its result',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[{component:'ViewBlock',props:{name:'detail',view:'articles',maxRows:50,display:{Type:'page',Title:{Field:'title',Fallback:'Article'},Renderer:{Type:'detail',TitleField:'title',MetaFields:['tag']},Pager:{Type:'none',PageSize:50}}}}]}})
      if(path.includes('/api/views/articles'))return response({data:Array.from({length:50},(_,index)=>({id:'1',title:'Resolved article',tag:'Tag '+(index+1)})),nextCursor:''})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/articles/1')
    expect(await screen.findByRole('heading',{name:'Resolved article',level:1})).toBeInTheDocument()
    expect(document.title).toBe('Resolved article')
    expect(await screen.findByText(/Tag 50/)).toBeInTheDocument()
    const requests=fetchMock.mock.calls.map(([input])=>String(input)).filter(path=>path.includes('/api/views/articles'))
    expect(requests[0]).toContain('limit=50')
    expect(requests).toHaveLength(1)
  })

  it('uses the title fallback for an empty detail result value',async()=>{
    document.title='Bean'
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',children:[{component:'ViewBlock',props:{name:'detail',view:'articles',display:{Type:'page',Title:{Field:'title',Fallback:'Article'},Renderer:{Type:'detail',TitleField:'title'},Pager:{Type:'none',PageSize:1}}}}]}})
      if(path.includes('/api/views/articles'))return response({data:[{id:'1',title:''}],nextCursor:''})
      return response({})
    }))
    renderApp('/articles/empty')
    expect(await screen.findByRole('heading',{name:'Article',level:1})).toBeInTheDocument()
    expect(document.title).toBe('Article')
  })

  it('keeps a block result title out of the browser title',async()=>{
    document.title='Bean'
    vi.stubGlobal('fetch',vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'Page',props:{title:'Outer page'},children:[{component:'ViewBlock',props:{name:'detail',block:'sidebar',view:'articles',display:{Type:'block',Title:{Field:'title',Fallback:'Article'},Renderer:{Type:'detail',TitleField:'title'}}}}]}})
      if(path.includes('/api/views/articles'))return response({data:[{id:'1',title:'Block article'}],nextCursor:''})
      return response({})
    }))
    renderApp('/composed')
    expect(await screen.findByRole('heading',{name:'Block article',level:2})).toBeInTheDocument()
    expect(document.title).toBe('Outer page')
  })

  it('rejects detail data that cannot fit in one bounded snapshot',async()=>{
    const fetchMock=vi.fn(async(input:string|URL|Request)=>{
      const path=String(input)
      if(path.includes('/api/system/session'))return response({authenticated:false})
      if(path.includes('/api/system/page'))return response({tree:{component:'ViewBlock',props:{name:'detail',view:'articles',maxRows:50,presentation:{Mode:'detail',TitleField:'title'}}}})
      if(path.includes('/api/views/articles'))return response({data:[{id:'1',title:'Article'}],nextCursor:'more-detail'})
      return response({})
    })
    vi.stubGlobal('fetch',fetchMock)
    renderApp('/articles/1')
    expect(await screen.findByRole('alert')).toHaveTextContent('This View display supports at most 50 rows.')
    expect(fetchMock.mock.calls.filter(([input])=>String(input).includes('/api/views/articles'))).toHaveLength(1)
  })
})

function NavigationDriver({capture}:{capture:(navigate:NavigateFunction)=>void}){capture(useNavigate());return null}
function renderApp(path:string){render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter initialEntries={[path]}><App/></MemoryRouter></QueryClientProvider>)}
function response(body:any){return Promise.resolve(new Response(JSON.stringify(body),{status:200,headers:{'Content-Type':'application/json'}}))}

afterEach(()=>vi.unstubAllGlobals())
