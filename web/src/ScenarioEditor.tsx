import {useState} from 'react'
import {Field} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'
import {Textarea} from '@/components/ui/textarea'

type Node={id:string;type:string;label?:string;next?:string;onFail?:string;[key:string]:unknown}
type Branch={condition:string;ref?:string;text?:string;next?:string}
type Definition={kind:string;metadata:{name:string};spec:Record<string,any>}

const nodeTypes=['navigate','click','fill','select','press','wait','assert','extract','branch','loop','script','api_call','pause']
const waitConditions=['navigation','network_idle','ref_visible','ref_hidden','text_present']
const assertions=['ref_visible','ref_hidden','ref_text','url_equals','url_contains','text_present']
const branchConditions=['last_step_passed','last_step_failed','ref_visible','ref_hidden','url_equals','url_contains','text_present']
const loopConditions=['ref_visible','ref_hidden','url_equals','url_contains','text_present']
const extractAttributes=['text','value','attribute']
const needsRef=(condition:string)=>condition==='ref_visible'||condition==='ref_hidden'
const needsText=(condition:string)=>['text_present','url_equals','url_contains'].includes(condition)
const assertionNeedsRef=(assertion:string)=>['ref_visible','ref_hidden','ref_text'].includes(assertion)
const assertionNeedsText=(assertion:string)=>['ref_text','url_equals','url_contains','text_present'].includes(assertion)

function EdgeSelect({id,label,value,nodes,onChange}:{id:string;label:string;value:string;nodes:string[];onChange:(next:string)=>void}){
  return <Field id={id} label={label}><NativeSelect id={id} value={value} onChange={event=>onChange(event.target.value)}><NativeSelectOption value="">None</NativeSelectOption>{nodes.map(node=><NativeSelectOption key={node}>{node}</NativeSelectOption>)}</NativeSelect></Field>
}

export function ScenarioEditor({spec,definitions,update}:{spec:Record<string,any>;definitions:Definition[];update:(next:Record<string,any>)=>void}){
  const nodes=(spec.nodes||[])as Node[]
  const ids=nodes.map(node=>node.id)
  const actions=definitions.filter(definition=>definition.kind==='Action').map(definition=>definition.metadata.name).sort()
  const [addType,setAddType]=useState('navigate')
  function setNode(index:number,patch:Record<string,unknown>){const next=[...nodes];next[index]={...next[index],...patch};update({...spec,nodes:next})}
  function renameNode(index:number,id:string){const before=nodes[index].id;const next=nodes.map((node,position)=>({...node,id:position===index?id:node.id,next:node.next===before?id:node.next,onFail:node.onFail===before?id:node.onFail,body:node.body===before?id:node.body,branches:Array.isArray(node.branches)?(node.branches as Branch[]).map(branch=>({...branch,next:branch.next===before?id:branch.next})):node.branches}));update({...spec,nodes:next,start:spec.start===before?id:spec.start})}
  function removeNode(index:number){const gone=nodes[index].id;const next=nodes.filter((_,position)=>position!==index).map(node=>({...node,next:node.next===gone?'':node.next,onFail:node.onFail===gone?'':node.onFail,body:node.body===gone?'':node.body,branches:Array.isArray(node.branches)?(node.branches as Branch[]).map(branch=>({...branch,next:branch.next===gone?'':branch.next})):node.branches}));update({...spec,nodes:next,start:spec.start===gone?'':spec.start})}
  function addNode(){let serial=nodes.length+1;let id=`${addType}_${serial}`;while(ids.includes(id))id=`${addType}_${++serial}`;update({...spec,nodes:[...nodes,{id,type:addType}],start:spec.start||id})}
  return <div className="space-y-5" data-testid="scenario-editor">
    <fieldset className="bean-form-section space-y-5"><legend>Scenario</legend>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field id="scenario-title" label="Title"><Input id="scenario-title" value={spec.title||''} onChange={event=>update({...spec,title:event.target.value})}/></Field>
        <Field id="scenario-start" label="Start node"><NativeSelect id="scenario-start" data-testid="scenario-start" value={spec.start||''} onChange={event=>update({...spec,start:event.target.value})}><NativeSelectOption value="">None</NativeSelectOption>{ids.map(id=><NativeSelectOption key={id}>{id}</NativeSelectOption>)}</NativeSelect></Field>
      </div>
      <Field id="scenario-description" label="Description"><Input id="scenario-description" value={spec.description||''} onChange={event=>update({...spec,description:event.target.value})}/></Field>
      <div className="flex items-end gap-2"><Field id="scenario-add-type" label="Add node"><NativeSelect id="scenario-add-type" data-testid="scenario-add-type" value={addType} onChange={event=>setAddType(event.target.value)}>{nodeTypes.map(type=><NativeSelectOption key={type}>{type}</NativeSelectOption>)}</NativeSelect></Field><Button data-testid="scenario-add-node" variant="outline" onClick={addNode}>Add node</Button></div>
    </fieldset>
    <ol className="space-y-4">{nodes.map((node,index)=><li key={index} data-testid={'scenario-node-'+node.id}><NodeCard node={node} index={index} ids={ids} actions={actions} isStart={spec.start===node.id} setNode={setNode} renameNode={renameNode} removeNode={removeNode}/></li>)}</ol>
  </div>
}

function NodeCard({node,index,ids,actions,isStart,setNode,renameNode,removeNode}:{node:Node;index:number;ids:string[];actions:string[];isStart:boolean;setNode:(index:number,patch:Record<string,unknown>)=>void;renameNode:(index:number,id:string)=>void;removeNode:(index:number)=>void}){
  const others=ids.filter(id=>id!==node.id)
  function patch(update:Record<string,unknown>){setNode(index,update)}
  function branch(position:number,update:Partial<Branch>){const branches=[...((node.branches||[])as Branch[])];branches[position]={...branches[position],...update};patch({branches})}
  function setBranch(position:number,update:Branch){const branches=[...((node.branches||[])as Branch[])];branches[position]=update;patch({branches})}
  return <div className="space-y-3 rounded-lg border p-4" data-testid="scenario-node">
    <div className="flex items-center justify-between gap-2"><span className="text-sm font-medium">{isStart?'Start · ':''}{node.type}</span><Button data-testid={'remove-node-'+node.id} variant="ghost" onClick={()=>removeNode(index)}>Remove</Button></div>
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <Field id={'node-id-'+index} label="ID"><Input id={'node-id-'+index} data-testid={'node-id-'+index} value={node.id} onChange={event=>renameNode(index,event.target.value)}/></Field>
      <Field id={'node-type-'+index} label="Type"><NativeSelect id={'node-type-'+index} data-testid={'node-type-'+index} value={node.type} onChange={event=>patch({type:event.target.value})}>{nodeTypes.map(type=><NativeSelectOption key={type}>{type}</NativeSelectOption>)}</NativeSelect></Field>
      <Field id={'node-label-'+index} label="Label"><Input id={'node-label-'+index} value={node.label||''} onChange={event=>patch({label:event.target.value})}/></Field>
    </div>
    <NodeFields node={node} index={index} others={others} actions={actions} patch={patch} branch={branch} setBranch={setBranch}/>
    <div className="grid gap-3 sm:grid-cols-2">
      <EdgeSelect id={'node-next-'+index} label="Next" value={node.next||''} nodes={others} onChange={value=>patch({next:value||undefined})}/>
      <EdgeSelect id={'node-onfail-'+index} label="On fail" value={node.onFail||''} nodes={others} onChange={value=>patch({onFail:value||undefined})}/>
    </div>
  </div>
}

function NodeFields({node,index,others,actions,patch,branch,setBranch}:{node:Node;index:number;others:string[];actions:string[];patch:(update:Record<string,unknown>)=>void;branch:(position:number,update:Partial<Branch>)=>void;setBranch:(position:number,update:Branch)=>void}){
  const field=(name:string,label:string,value:unknown)=><Field id={`node-${name}-${index}`} label={label}><Input id={`node-${name}-${index}`} data-testid={`node-${name}-${index}`} value={String(value||'')} onChange={event=>patch({[name]:event.target.value})}/></Field>
  switch(node.type){
    case 'navigate':return <div className="grid gap-3 sm:grid-cols-2">{field('url','URL',node.url)}</div>
    case 'click':return <div className="grid gap-3 sm:grid-cols-2">{field('ref','Element ref',node.ref)}</div>
    case 'fill':return <div className="grid gap-3 sm:grid-cols-3">{field('ref','Element ref',node.ref)}{field('text','Text (XOR secret)',node.text)}{field('secret','Secret name',node.secret)}</div>
    case 'select':return <div className="grid gap-3 sm:grid-cols-2">{field('ref','Element ref',node.ref)}{field('value','Value',node.value)}</div>
    case 'press':return <div className="grid gap-3 sm:grid-cols-2">{field('key','Key',node.key)}{field('ref','Element ref (optional)',node.ref)}</div>
    case 'wait':{
      const condition=String(node.condition||'')
      return <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Field id={'node-condition-'+index} label="Condition"><NativeSelect id={'node-condition-'+index} data-testid={'node-condition-'+index} value={condition} onChange={event=>patch({condition:event.target.value})}><NativeSelectOption value="">Choose</NativeSelectOption>{waitConditions.map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
        {needsRef(condition)&&field('ref','Element ref',node.ref)}
        {needsText(condition)&&field('text','Expected text/URL',node.text)}
        {field('timeoutSeconds','Timeout (s)',node.timeoutSeconds)}
      </div>
    }
    case 'assert':{
      const assertion=String(node.assertion||'')
      return <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <Field id={'node-assertion-'+index} label="Assertion"><NativeSelect id={'node-assertion-'+index} data-testid={'node-assertion-'+index} value={assertion} onChange={event=>patch({assertion:event.target.value})}><NativeSelectOption value="">Choose</NativeSelectOption>{assertions.map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
        {assertionNeedsRef(assertion)&&field('ref','Element ref',node.ref)}
        {assertionNeedsText(assertion)&&field('text','Expected text/URL',node.text)}
      </div>
    }
    case 'extract':{
      const attribute=String(node.attribute||'text')
      return <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {field('ref','Element ref',node.ref)}{field('as','Bind as',node.as)}
        <Field id={'node-attribute-'+index} label="Attribute"><NativeSelect id={'node-attribute-'+index} value={attribute} onChange={event=>patch({attribute:event.target.value})}>{extractAttributes.map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
        {attribute==='attribute'&&field('name','Attribute name',node.name)}
      </div>
    }
    case 'branch':{
      const branches=(node.branches||[])as Branch[]
      return <div className="space-y-3">
        {branches.map((item,position)=><div className="grid gap-3 rounded-lg bg-muted/50 p-3 sm:grid-cols-2 lg:grid-cols-4" data-testid={'branch-'+index+'-'+position} key={position}>
          <Field id={`branch-condition-${index}-${position}`} label="Condition"><NativeSelect id={`branch-condition-${index}-${position}`} value={item.condition||''} onChange={event=>branch(position,{condition:event.target.value})}>{branchConditions.map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
          {needsRef(item.condition)&&<Field id={`branch-ref-${index}-${position}`} label="Element ref"><Input id={`branch-ref-${index}-${position}`} value={item.ref||''} onChange={event=>branch(position,{ref:event.target.value})}/></Field>}
          {needsText(item.condition)&&<Field id={`branch-text-${index}-${position}`} label="Expected text/URL"><Input id={`branch-text-${index}-${position}`} value={item.text||''} onChange={event=>branch(position,{text:event.target.value})}/></Field>}
          <EdgeSelect id={`branch-next-${index}-${position}`} label="Next" value={item.next||''} nodes={others} onChange={value=>branch(position,{next:value})}/>
          <Button variant="ghost" data-testid={`branch-remove-${index}-${position}`} onClick={()=>patch({branches:branches.filter((_,at)=>at!==position)})}>Remove</Button>
        </div>)}
        <Button variant="outline" data-testid={'branch-add-'+index} onClick={()=>setBranch(branches.length,{condition:branchConditions[0],next:others[0]||''})}>Add branch</Button>
      </div>
    }
    case 'loop':{
      const until=String(node.until||'')
      return <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Field id={'node-until-'+index} label="Until"><NativeSelect id={'node-until-'+index} value={until} onChange={event=>patch({until:event.target.value})}><NativeSelectOption value="">Until iterations cap</NativeSelectOption>{loopConditions.map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
        {needsRef(until)&&field('ref','Element ref',node.ref)}
        {needsText(until)&&field('text','Expected text/URL',node.text)}
        <EdgeSelect id={'node-body-'+index} label="Body (re-entry)" value={String(node.body||'')} nodes={others} onChange={value=>patch({body:value||undefined})}/>
        {field('maxIterations','Max iterations',node.maxIterations)}
      </div>
    }
    case 'script':return <div className="grid gap-3"><Field id={'node-script-'+index} label="Script"><Textarea id={'node-script-'+index} className="min-h-32 font-mono" value={String(node.script||'')} onChange={event=>patch({script:event.target.value})}/></Field>{field('as','Bind result as (optional)',node.as)}</div>
    case 'api_call':return <div className="grid gap-3 sm:grid-cols-2">
      <Field id={'node-action-'+index} label="Action"><NativeSelect id={'node-action-'+index} data-testid={'node-action-'+index} value={String(node.action||'')} onChange={event=>patch({action:event.target.value})}><NativeSelectOption value="">Choose</NativeSelectOption>{actions.map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
      {field('as','Bind result as (optional)',node.as)}
      <Field id={'node-input-'+index} label="Input JSON"><Textarea id={'node-input-'+index} className="min-h-24 font-mono" value={typeof node.input==='object'?JSON.stringify(node.input):String(node.input||'')} onChange={event=>{try{patch({input:JSON.parse(event.target.value)})}catch{patch({input:event.target.value})}}}/></Field>
    </div>
    default:return null
  }
}
