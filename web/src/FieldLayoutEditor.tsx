import {useId} from 'react'
import {Field} from './components/bean'
import {Button} from './components/ui/button'
import {Input} from './components/ui/input'
import {NativeSelect,NativeSelectOption} from './components/ui/native-select'

type Item={field:string;span?:'single'|'full'}
type Group={name:string;label:string;columns?:1|2;fields:Item[]}
export type SourceFieldLayout={groups:Group[]}
function move<T>(items:T[],index:number,delta:number){const next=[...items];[next[index],next[index+delta]]=[next[index+delta],next[index]];return next}

export function FieldLayoutEditor({value,fields,onChange}:{value?:SourceFieldLayout;fields:string[];onChange:(value:SourceFieldLayout|undefined)=>void}){
  const id=useId()
  const groups=value?.groups||[]
  const used=new Set(groups.flatMap(group=>group.fields.map(item=>item.field)))
  const available=fields.filter(field=>!used.has(field))
  const setGroup=(index:number,group:Group)=>onChange({groups:groups.map((current,i)=>i===index?group:current)})
  return <fieldset className="min-w-0 space-y-4 rounded-lg border p-4"><legend className="px-1 font-semibold">Field layout</legend>{!value?<Button variant="outline" disabled={!fields.length||fields.length>128} onClick={()=>onChange({groups:Array.from({length:Math.ceil(fields.length/64)},(_,i)=>({name:'group_'+(i+1),label:'Fields '+(i+1),columns:1,fields:fields.slice(i*64,(i+1)*64).map(field=>({field,span:'single'}))}))})}>Use grouped layout</Button>:<>
    <p className="text-sm text-muted-foreground">Groups and fields follow this reading order at every screen size. Compilation checks field membership; layout does not change access or writes.</p>
    {groups.map((group,index)=><div className="min-w-0 space-y-3 rounded-lg border p-3" key={index}>
      <div className="grid gap-3 md:grid-cols-3"><Field id={`${id}-${index}-name`} label="Group name"><Input id={`${id}-${index}-name`} value={group.name} maxLength={64} onChange={event=>setGroup(index,{...group,name:event.target.value})}/></Field><Field id={`${id}-${index}-label`} label="Group label"><Input id={`${id}-${index}-label`} value={group.label} maxLength={120} onChange={event=>setGroup(index,{...group,label:event.target.value})}/></Field><Field id={`${id}-${index}-columns`} label="Group columns"><NativeSelect id={`${id}-${index}-columns`} value={group.columns||1} onChange={event=>setGroup(index,{...group,columns:Number(event.target.value) as 1|2})}><NativeSelectOption value={1}>One</NativeSelectOption><NativeSelectOption value={2}>Two</NativeSelectOption></NativeSelect></Field></div>
      {group.fields.map((item,itemIndex)=><div className="flex flex-wrap items-end gap-2" key={itemIndex}><Field id={`${id}-${index}-${itemIndex}-field`} label="Layout field"><NativeSelect id={`${id}-${index}-${itemIndex}-field`} value={item.field} onChange={event=>setGroup(index,{...group,fields:group.fields.map((current,i)=>i===itemIndex?{...item,field:event.target.value}:current)})}>{[item.field,...available].map(field=><NativeSelectOption key={field}>{field}</NativeSelectOption>)}</NativeSelect></Field><Field id={`${id}-${index}-${itemIndex}-span`} label="Field span"><NativeSelect id={`${id}-${index}-${itemIndex}-span`} value={item.span||'single'} onChange={event=>setGroup(index,{...group,fields:group.fields.map((current,i)=>i===itemIndex?{...item,span:event.target.value as Item['span']}:current)})}><NativeSelectOption value="single">Single</NativeSelectOption><NativeSelectOption value="full">Full row</NativeSelectOption></NativeSelect></Field>
        <Button variant="outline" aria-label={'Move '+item.field+' up'} disabled={itemIndex===0} onClick={()=>setGroup(index,{...group,fields:move(group.fields,itemIndex,-1)})}>Up</Button><Button variant="outline" aria-label={'Move '+item.field+' down'} disabled={itemIndex===group.fields.length-1} onClick={()=>setGroup(index,{...group,fields:move(group.fields,itemIndex,1)})}>Down</Button><Button variant="destructive" aria-label={'Remove layout field '+item.field} onClick={()=>setGroup(index,{...group,fields:group.fields.filter((_,i)=>i!==itemIndex)})}>Remove</Button>
      </div>)}
      <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={!available.length||group.fields.length>=64||used.size>=128} onClick={()=>setGroup(index,{...group,fields:[...group.fields,{field:available[0],span:'single'}]})}>Add layout field</Button><Button variant="outline" aria-label={'Move group '+group.name+' up'} disabled={index===0} onClick={()=>onChange({groups:move(groups,index,-1)})}>Group up</Button><Button variant="outline" aria-label={'Move group '+group.name+' down'} disabled={index===groups.length-1} onClick={()=>onChange({groups:move(groups,index,1)})}>Group down</Button><Button variant="destructive" onClick={()=>onChange({groups:groups.filter((_,i)=>i!==index)})}>Remove group</Button></div>
    </div>)}
    <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={groups.length>=16} onClick={()=>{let n=groups.length+1;while(groups.some(group=>group.name==='group_'+n))n++;onChange({groups:[...groups,{name:'group_'+n,label:'Fields '+n,columns:1,fields:[]}]})}}>Add field group</Button><Button variant="outline" onClick={()=>onChange(undefined)}>Use legacy layout</Button></div>
  </>}</fieldset>
}
