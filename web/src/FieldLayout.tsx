import {useId,type ReactNode} from 'react'
import type {FieldLayout as Layout} from './api'

// The caller supplies only eligible controls or projected values. This component
// owns geometry and semantics, never field access, value fetching, or writes.
export function FieldLayout({layout,fields,mode='form'}:{layout:Layout;fields:Record<string,ReactNode>;mode?:'form'|'detail'}){
  const id=useId()
  return <div className="space-y-6" data-testid="field-layout">{layout.Groups.map(group=>{
    const items=group.Fields.filter(item=>Object.hasOwn(fields,item.Field)&&fields[item.Field]!==null&&fields[item.Field]!==undefined)
    if(!items.length)return null
    const gridClass=group.Columns===2?'grid min-w-0 grid-cols-1 gap-4 md:grid-cols-2':'grid min-w-0 grid-cols-1 gap-4'
    const content=items.map(item=><div key={item.Field} data-layout-field={item.Field} className={item.Span==='full'?'col-span-full min-w-0 [overflow-wrap:anywhere]':'min-w-0 [overflow-wrap:anywhere]'}>{fields[item.Field]}</div>)
    return mode==='form'?<fieldset key={group.Name} className="min-w-0 space-y-3" data-field-group={group.Name}><legend className="mb-3 font-semibold">{group.Label}</legend><div className={gridClass}>{content}</div></fieldset>:<section key={group.Name} aria-labelledby={id+'-'+group.Name} data-field-group={group.Name} className="min-w-0 space-y-3"><h3 id={id+'-'+group.Name} className="font-semibold">{group.Label}</h3><dl className={gridClass}>{content}</dl></section>
  })}</div>
}
