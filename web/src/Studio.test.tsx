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
