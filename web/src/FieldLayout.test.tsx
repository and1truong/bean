import {render,screen} from '@testing-library/react'
import {describe,expect,it} from 'vitest'
import {FieldLayout} from './FieldLayout'
import {Input} from './components/ui/input'
import {Textarea} from './components/ui/textarea'
import type {FieldLayout as Layout} from './api'

const layout:Layout={Groups:[{Name:'content',Label:'Content',Columns:2,Fields:[{Field:'title',Span:'single'},{Field:'slug',Span:'single'},{Field:'body',Span:'full'}]},{Name:'derived',Label:'Derived',Columns:1,Fields:[{Field:'secret',Span:'single'}]}]}
describe('FieldLayout',()=>{
 it('keeps source order, bounded responsive tracks, and eligible controls only',()=>{
  const {container}=render(<FieldLayout layout={layout} fields={{title:<Input aria-label="Title"/>,slug:<Input aria-label="Slug"/>,body:<Textarea aria-label="Body"/>}}/>)
  expect(screen.getByRole('group',{name:'Content'}).tagName).toBe('FIELDSET')
  expect(screen.queryByRole('group',{name:'Derived'})).not.toBeInTheDocument()
  expect(screen.getAllByRole('textbox').map(element=>element.getAttribute('aria-label'))).toEqual(['Title','Slug','Body'])
  expect(container.querySelector('[data-layout-field="body"]')).toHaveClass('col-span-full','min-w-0')
  expect(container.querySelector('[data-layout-field="title"]')?.parentElement).toHaveClass('grid-cols-1','md:grid-cols-2')
 })
 it('renders readonly groups as labelled sections and definition lists, not form controls',()=>{
  const {container}=render(<><FieldLayout layout={layout} mode="detail" fields={{title:<><dt>Title</dt><dd>Bean</dd></>}}/><FieldLayout layout={layout} mode="detail" fields={{title:<><dt>Title</dt><dd>Second</dd></>}}/></>)
  expect(screen.getAllByRole('region',{name:'Content'})).toHaveLength(2)
  expect(container.querySelectorAll('dl > div > dt')).toHaveLength(2)
  expect(container.querySelector('fieldset')).toBeNull()
  const ids=Array.from(container.querySelectorAll('[id]'),element=>element.id)
  expect(new Set(ids).size).toBe(ids.length)
 })
})
