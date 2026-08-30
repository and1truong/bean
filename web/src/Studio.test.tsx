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
