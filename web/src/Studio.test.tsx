import {act,fireEvent,render,screen} from '@testing-library/react'
import {afterEach,expect,it,vi} from 'vitest'
import {MemoryRouter} from 'react-router-dom'
import {QueryClient,QueryClientProvider} from '@tanstack/react-query'
import {Studio} from './Studio'
import {useEditor} from './store'

const definitions=[{apiVersion:'bean/v1alpha1',kind:'Entity',metadata:{name:'book'},spec:{label:'Book',fields:[{name:'title',label:'Title',type:'string',required:true}],indexes:[['title']]}}]

afterEach(()=>{vi.restoreAllMocks();act(()=>useEditor.getState().reset())})

it('round-trips visual changes through the canonical specification',async()=>{
  vi.spyOn(globalThis,'fetch').mockImplementation(async input=>new Response(JSON.stringify(String(input).endsWith('/definitions')?definitions:[]),{status:200}))
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter><Studio/></MemoryRouter></QueryClientProvider>)
  fireEvent.click(await screen.findByRole('button',{name:'Entity: book'}))
	await screen.findByText('Active release: none')
  expect(screen.getByLabelText('Definition navigator')).toBeInTheDocument()
  expect(screen.getByLabelText('Definition workspace')).toBeInTheDocument()
  expect(screen.getByLabelText('Release inspector')).toBeInTheDocument()
  fireEvent.change(screen.getByTestId('entity-label'),{target:{value:'Publication'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  const spec=JSON.parse((screen.getByTestId('definition-spec') as HTMLTextAreaElement).value)
  expect(spec.label).toBe('Publication')
  expect(spec.indexes).toEqual([['title']])
  expect(spec.fields[0]).toMatchObject({name:'title',required:true})
})

it('offers reference-aware core definition editors',async()=>{
  vi.spyOn(globalThis,'fetch').mockImplementation(async input=>new Response(JSON.stringify(String(input).endsWith('/definitions')?definitions:[]),{status:200}))
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter><Studio/></MemoryRouter></QueryClientProvider>)
	await screen.findByText('Active release: none')
  fireEvent.change(await screen.findByTestId('definition-kind'),{target:{value:'View'}})
  expect(screen.getByTestId('view-entity')).toHaveTextContent('book')
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'Policy'}})
  expect(screen.getByTestId('policy-read-roles')).toBeInTheDocument()
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'Action'}})
  expect(screen.getByTestId('action-entity')).toHaveTextContent('book')
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'AdminResource'}})
  expect(screen.getByTestId('admin-entity')).toHaveTextContent('book')
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'Menu'}})
  expect(screen.getByLabelText('Profile')).toHaveValue('workspace')
  expect(screen.getByLabelText('Variant')).toHaveValue('default')
  fireEvent.change(screen.getByLabelText('Variant'),{target:{value:'line'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  expect(JSON.parse((screen.getByTestId('definition-spec') as HTMLTextAreaElement).value).variant).toBe('line')
})

it('preserves stable Page section IDs while editing Panel references visually',async()=>{
  const pageDefinitions=[
    ...definitions,
    {apiVersion:'bean/v1alpha1',kind:'Panel',metadata:{name:'hero'},spec:{layout:'single-column',regions:[]}},
    {apiVersion:'bean/v1alpha1',kind:'Panel',metadata:{name:'body'},spec:{layout:'single-column',regions:[]}},
    {apiVersion:'bean/v1alpha1',kind:'Page',metadata:{name:'landing'},spec:{route:'/',sections:[{id:'introduction',panel:'hero'},{id:'content',panel:'body'}]}},
  ]
  vi.spyOn(globalThis,'fetch').mockImplementation(async input=>new Response(JSON.stringify(String(input).endsWith('/definitions')?pageDefinitions:[]),{status:200}))
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter><Studio/></MemoryRouter></QueryClientProvider>)
  fireEvent.click(await screen.findByRole('button',{name:'Page: landing'}))
  fireEvent.change(screen.getByTestId('page-section-0'),{target:{value:'body'}})
  fireEvent.change(screen.getByTestId('page-section-width-0'),{target:{value:'full'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  const spec=JSON.parse((screen.getByTestId('definition-spec') as HTMLTextAreaElement).value)
  expect(spec.sections).toEqual([{id:'introduction',panel:'body',width:'full'},{id:'content',panel:'body'}])
})

it('authors the common View display path without Advanced JSON',async()=>{
  vi.spyOn(globalThis,'fetch').mockImplementation(async input=>new Response(JSON.stringify(String(input).endsWith('/definitions')?definitions:[]),{status:200}))
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter><Studio/></MemoryRouter></QueryClientProvider>)
  await screen.findByText('Active release: none')
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'View'}})
  fireEvent.change(screen.getByTestId('view-entity'),{target:{value:'book'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'title'}))
  fireEvent.click(screen.getByRole('button',{name:'Add exposed filter'}))
  fireEvent.change(screen.getByLabelText('Field',{selector:'#view-filter-field-0'}),{target:{value:'title'}})
  fireEvent.change(screen.getByLabelText('Operator'),{target:{value:'contains'}})
  fireEvent.click(screen.getByTestId('add-view-display'))
  fireEvent.change(screen.getByLabelText('Route'),{target:{value:'/books'}})
  fireEvent.change(screen.getByLabelText('Renderer'),{target:{value:'table'}})
  fireEvent.change(screen.getByLabelText('Title'),{target:{value:'Books'}})
  fireEvent.click(screen.getByRole('button',{name:'Add column'}))
  fireEvent.change(screen.getByLabelText('Field',{selector:'#view-column-field-0-0'}),{target:{value:'title'}})
  fireEvent.change(screen.getByLabelText('Label',{selector:'#view-column-label-0-0'}),{target:{value:'Book title'}})
  fireEvent.click(screen.getByRole('button',{name:'Add control'}))
  fireEvent.change(screen.getByLabelText('Label',{selector:'#view-control-label-0-0'}),{target:{value:'Search titles'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  const spec=JSON.parse((screen.getByTestId('definition-spec') as HTMLTextAreaElement).value)
  expect(spec.exposedFilters.filter_1).toEqual({field:'title',operator:'contains'})
  expect(spec.displays.display_1).toMatchObject({type:'page',route:'/books',title:{text:'Books'},renderer:{type:'table',fields:[{field:'title',label:'Book title'}]},controls:[{filter:'filter_1',label:'Search titles',widget:'auto'}],pager:{type:'cursor',pageSize:25}})
})

it('authors a result-titled detail display with its immutable route binding',async()=>{
  vi.spyOn(globalThis,'fetch').mockImplementation(async input=>new Response(JSON.stringify(String(input).endsWith('/definitions')?definitions:[]),{status:200}))
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter><Studio/></MemoryRouter></QueryClientProvider>)
  await screen.findByText('Active release: none')
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'View'}})
  fireEvent.change(screen.getByTestId('view-entity'),{target:{value:'book'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'title'}))
  fireEvent.click(screen.getByTestId('add-view-display'))
  fireEvent.change(screen.getByLabelText('Route'),{target:{value:'/books'}})
  fireEvent.change(screen.getByLabelText('Title source'),{target:{value:'result'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  const spec=JSON.parse((screen.getByTestId('definition-spec') as HTMLTextAreaElement).value)
  expect(spec.exposedFilters.id).toEqual({field:'id',operator:'eq'})
  expect(spec.displays.display_1).toMatchObject({
    type:'page',route:'/books/:id',bindings:{id:{source:'route',name:'id',required:true}},
    title:{field:'title',fallback:'Record'},renderer:{type:'detail'},
  })
})

it('authors Explore query, display, action, drill, and page-filter semantics visually',async()=>{
  const exploreDefinitions=[
    ...definitions,
    {apiVersion:'bean/v1alpha1',kind:'Action',metadata:{name:'move_book'},spec:{entity:'book',operation:'update'}},
    {apiVersion:'bean/v1alpha1',kind:'View',metadata:{name:'book_records'},spec:{entity:'book',fields:['id','title'],exposedFilters:{title:{field:'title',operator:'eq'}},displays:{table:{type:'page',route:'/books',renderer:{type:'table'}}}}},
    {apiVersion:'bean/v1alpha1',kind:'Block',metadata:{name:'book_chart'},spec:{type:'view',view:'book_records',display:'table'}},
    {apiVersion:'bean/v1alpha1',kind:'Panel',metadata:{name:'book_panel'},spec:{layout:'stack',regions:[]}},
  ]
  vi.spyOn(globalThis,'fetch').mockImplementation(async input=>new Response(JSON.stringify(String(input).endsWith('/definitions')?exploreDefinitions:[]),{status:200}))
  render(<QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter><Studio/></MemoryRouter></QueryClientProvider>)
  await screen.findByText('Active release: none')
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'View'}})
  fireEvent.change(screen.getByTestId('view-entity'),{target:{value:'book'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'title'}))
  fireEvent.change(screen.getByLabelText('Search fields'),{target:{value:'title'}})
  fireEvent.change(screen.getByLabelText('Group field'),{target:{value:'title'}})
  fireEvent.change(screen.getByLabelText('Aggregate'),{target:{value:'count'}})
  fireEvent.click(screen.getByTestId('add-view-display'))
  fireEvent.change(screen.getByLabelText('Explore renderer'),{target:{value:'chart'}})
  fireEvent.change(screen.getByLabelText('Renderer group field'),{target:{value:'title'}})
  fireEvent.change(screen.getByLabelText('Metric field'),{target:{value:'total'}})
  fireEvent.change(screen.getByLabelText('Selection'),{target:{value:'multiple'}})
  fireEvent.change(screen.getByLabelText('Record actions'),{target:{value:'move_book'}})
  fireEvent.change(screen.getByLabelText('Target View'),{target:{value:'book_records'}})
  fireEvent.change(screen.getByLabelText('Target Display'),{target:{value:'table'}})
  fireEvent.change(screen.getByLabelText('Binding source'),{target:{value:'filter'}})
  fireEvent.change(screen.getByLabelText('Binding source'),{target:{value:'group'}})
  fireEvent.change(screen.getByLabelText('Source name'),{target:{value:'title'}})
  fireEvent.change(screen.getByLabelText('Target filter'),{target:{value:'title'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  const view=JSON.parse((screen.getByTestId('definition-spec') as HTMLTextAreaElement).value)
  expect(view).toMatchObject({search:{fields:['title']},groupBy:[{field:'title',as:'title'}],aggregates:[{function:'count',field:'id',alias:'total'}],displays:{display_1:{renderer:{type:'chart',groupField:'title',metricField:'total'},selection:'multiple',actions:['move_book'],drill:{view:'book_records',display:'table',bindings:[{source:'group',name:'title',filter:'title'}]}}}})

  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  fireEvent.change(screen.getByTestId('definition-kind'),{target:{value:'Page'}})
  fireEvent.change(screen.getByTestId('page-composition'),{target:{value:'sections'}})
  fireEvent.change(screen.getByTestId('page-section-0'),{target:{value:'book_panel'}})
  fireEvent.change(screen.getByTestId('page-section-width-0'),{target:{value:'contained'}})
  fireEvent.click(screen.getByTestId('add-page-section'))
  fireEvent.change(screen.getByTestId('page-section-1'),{target:{value:'book_panel'}})
  fireEvent.change(screen.getByTestId('page-section-width-1'),{target:{value:'full'}})
  fireEvent.click(screen.getByRole('button',{name:'Add page filter'}))
  fireEvent.change(screen.getByLabelText('Target Block'),{target:{value:'book_chart'}})
  fireEvent.change(screen.getByLabelText('Target filter'),{target:{value:'title'}})
  fireEvent.click(screen.getByRole('checkbox',{name:'Advanced JSON'}))
  const page=JSON.parse((screen.getByTestId('definition-spec') as HTMLTextAreaElement).value)
  expect(page.panel).toBeUndefined()
  expect(page.sections).toEqual([{panel:'book_panel',width:'contained'},{panel:'book_panel',width:'full'}])
  expect(page.filters.filter_1.targets).toEqual([{block:'book_chart',filter:'title'}])
})
