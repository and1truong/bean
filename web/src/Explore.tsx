import {useEffect,useMemo,useState} from 'react'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {api,apiResponse,AdminManifest,Entity} from './api'
import {ErrorAlert,Field,LoadingState,Page,PageHeader,SectionCard,StatusAlert} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Checkbox} from '@/components/ui/checkbox'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'
import {Table,TableBody,TableCell,TableHead,TableHeader,TableRow} from '@/components/ui/table'

type Definition={apiVersion:string;kind:string;metadata:{name:string;namespace?:string};spec:Record<string,any>}
type Row=Record<string,any>
type Preview={valid:boolean;data:Row[];shape?:string;nextCursor?:string;diagnostics?:Array<{Code?:string;Path?:string;Message:string}>}
type Validation={valid:boolean;diagnostics?:Array<{Code?:string;Path?:string;Message:string}>;changes?:Array<{operation:string;path:string}>}
type ExploreMode='records'|'groups'|'metric'

export function Explore(){
  const qc=useQueryClient()
  const manifest=useQuery({queryKey:['admin-manifest'],queryFn:()=>api<AdminManifest>('/api/admin/manifest')})
  const definitions=useQuery({queryKey:['definitions','etag'],queryFn:async()=>{const result=await apiResponse<Definition[]>('/api/admin/definitions');return {items:result.data,etag:result.response.headers.get('ETag')||''}}})
  const entityNames=useMemo(()=>Object.keys(manifest.data?.entities||{}).sort(),[manifest.data])
  const[entityName,setEntityName]=useState('')
  const[viewName,setViewName]=useState('')
  const[pageRoute,setPageRoute]=useState('')
  const[fields,setFields]=useState<string[]>([])
  const[searchFields,setSearchFields]=useState<string[]>([])
  const[search,setSearch]=useState('')
  const[filterField,setFilterField]=useState('')
  const[filterOperator,setFilterOperator]=useState('eq')
  const[filterValue,setFilterValue]=useState('')
  const[sortField,setSortField]=useState('id')
  const[descending,setDescending]=useState(false)
  const[mode,setMode]=useState<ExploreMode>('records')
  const[groupField,setGroupField]=useState('')
  const[groupBucket,setGroupBucket]=useState('')
  const[aggregateFunction,setAggregateFunction]=useState('count')
  const[aggregateField,setAggregateField]=useState('id')
  const[aggregateAlias,setAggregateAlias]=useState('total')
  const[replace,setReplace]=useState(false)
  const[saved,setSaved]=useState(false)
  const[previewedFingerprint,setPreviewedFingerprint]=useState('')
  const entity=manifest.data?.entities[entityName]
  const available=useMemo(()=>entityFields(entity),[entity])
  useEffect(()=>{
    if(entityName||!entityNames.length)return
    setEntityName(entityNames[0])
  },[entityName,entityNames])
  useEffect(()=>{
    if(!entity)return
    const next=entityFields(entity).map(field=>field.Name)
    setViewName(entityName+'_explore')
    setPageRoute('')
    setFields(next)
    setSearchFields(next.filter(name=>searchable(fieldByName(entity,name)?.Type)))
    setFilterField('')
    setFilterValue('')
    setSortField('id')
    setMode('records')
    setGroupField('')
    setGroupBucket('')
    setAggregateFunction('count')
    setAggregateField('id')
    setAggregateAlias('total')
    setReplace(false)
    setSaved(false)
  },[entityName,entity])
  const spec=useMemo(()=>viewSpec(entityName,fields,searchFields,filterField,filterOperator,sortField,descending,mode,groupField,groupBucket,aggregateFunction,aggregateField,aggregateAlias,pageRoute),[entityName,fields,searchFields,filterField,filterOperator,sortField,descending,mode,groupField,groupBucket,aggregateFunction,aggregateField,aggregateAlias,pageRoute])
  const filterName=filterField?filterField.replaceAll('.','_'):''
  const previewCandidate=useMemo(()=>({name:viewName,spec,search,filter:filterName&&filterValue?{[filterName]:filterValue}:{},limit:25}),[viewName,spec,search,filterName,filterValue])
  const previewFingerprint=useMemo(()=>JSON.stringify(previewCandidate),[previewCandidate])
  const preview=useMutation({mutationFn:({candidate}:{fingerprint:string;candidate:typeof previewCandidate})=>api<Preview>('/api/admin/explore/preview',{method:'POST',body:JSON.stringify(candidate)}),onMutate:()=>setPreviewedFingerprint(''),onSuccess:(_data,request)=>{setPreviewedFingerprint(request.fingerprint);setSaved(false)}})
  const validate=useMutation({mutationFn:()=>api<Validation>('/api/admin/releases/validate',{method:'POST'})})
  const save=useMutation({mutationFn:()=>api('/api/admin/definitions',{method:'POST',headers:{'If-Match':definitions.data!.etag},body:JSON.stringify({apiVersion:'bean/v1alpha1',kind:'View',metadata:{namespace:'default',name:viewName},spec})}),onSuccess:()=>{setSaved(true);setReplace(false);validate.mutate();void qc.invalidateQueries({queryKey:['definitions']})},onError:()=>{setReplace(false);void qc.invalidateQueries({queryKey:['definitions']})}})
  const conflict=Boolean(definitions.data?.items.some(definition=>definition.kind==='View'&&definition.metadata.name===viewName))
  const previewIsCurrent=Boolean(preview.data&&previewedFingerprint===previewFingerprint)
  if(manifest.isPending)return <Page><LoadingState label="Loading Explore…"/></Page>
  if(manifest.error)return <Page narrow><PageHeader title="Explore"/><ErrorAlert error={manifest.error}/></Page>
  return <Page><PageHeader title="Explore" description="Build a typed View from an existing Entity, preview it with current Policy, then save an ordinary draft definition."/>
    <SectionCard title="View query"><div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <Field id="explore-entity" label="Entity"><NativeSelect id="explore-entity" data-testid="explore-entity" value={entityName} onChange={event=>setEntityName(event.target.value)}>{entityNames.map(name=><NativeSelectOption key={name}>{name}</NativeSelectOption>)}</NativeSelect></Field>
      <Field id="explore-name" label="View name"><Input id="explore-name" data-testid="explore-name" required pattern="[a-z][a-z0-9_]*" value={viewName} onChange={event=>{setViewName(event.target.value);setReplace(false);setSaved(false)}}/></Field>
      <Field id="explore-mode" label="Result"><NativeSelect id="explore-mode" value={mode} onChange={event=>setMode(event.target.value as ExploreMode)}>{['records','groups','metric'].map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
      <Field id="explore-route" label="Page route (optional)"><Input id="explore-route" placeholder="/my-view" value={pageRoute} onChange={event=>setPageRoute(event.target.value)}/></Field>
    </div>
    {mode==='records'?<><div className="mt-4 grid gap-4 md:grid-cols-2"><Field id="explore-sort" label="Sort"><NativeSelect id="explore-sort" value={sortField} onChange={event=>setSortField(event.target.value)}>{fields.map(name=><NativeSelectOption key={name}>{name}</NativeSelectOption>)}</NativeSelect></Field><div className="flex items-end pb-2"><Check id="explore-desc" label="Descending sort" checked={descending} onChange={setDescending}/></div></div><fieldset className="mt-6"><legend className="font-semibold">Projection</legend><div className="mt-3 flex flex-wrap gap-4">{available.map(field=><Check key={field.Name} id={'explore-field-'+field.Name} label={field.Label||field.Name} checked={fields.includes(field.Name)} disabled={field.Name==='id'} onChange={checked=>setFields(toggle(fields,field.Name,checked))}/>)}</div></fieldset><fieldset className="mt-6"><legend className="font-semibold">Search fields</legend><div className="mt-3 flex flex-wrap gap-4">{available.filter(field=>searchable(field.Type)&&fields.includes(field.Name)).map(field=><Check key={field.Name} id={'explore-search-'+field.Name} label={field.Label||field.Name} checked={searchFields.includes(field.Name)} onChange={checked=>setSearchFields(toggle(searchFields,field.Name,checked))}/>)}</div></fieldset></>:<div className="mt-6 grid gap-4 md:grid-cols-2 lg:grid-cols-4">{mode==='groups'&&<><Field id="explore-group" label="Group field"><NativeSelect id="explore-group" value={groupField} onChange={event=>{setGroupField(event.target.value);setGroupBucket('')}}><NativeSelectOption value="">Select…</NativeSelectOption>{available.map(field=><NativeSelectOption key={field.Name}>{field.Name}</NativeSelectOption>)}</NativeSelect></Field><Field id="explore-bucket" label="Date bucket"><NativeSelect id="explore-bucket" disabled={!dateLike(fieldByName(entity,groupField)?.Type)} value={groupBucket} onChange={event=>setGroupBucket(event.target.value)}><NativeSelectOption value="">None</NativeSelectOption>{['day','week','month'].map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field></>}<Field id="explore-aggregate-function" label="Aggregate"><NativeSelect id="explore-aggregate-function" value={aggregateFunction} onChange={event=>setAggregateFunction(event.target.value)}>{['count','sum','min','max'].map(value=><NativeSelectOption key={value}>{value}</NativeSelectOption>)}</NativeSelect></Field><Field id="explore-aggregate-field" label="Aggregate field"><NativeSelect id="explore-aggregate-field" value={aggregateField} onChange={event=>setAggregateField(event.target.value)}>{available.filter(field=>aggregateFunction==='count'||numeric(field.Type)).map(field=><NativeSelectOption key={field.Name}>{field.Name}</NativeSelectOption>)}</NativeSelect></Field><Field id="explore-aggregate-alias" label="Output name"><Input id="explore-aggregate-alias" pattern="[a-z][a-z0-9_]*" value={aggregateAlias} onChange={event=>setAggregateAlias(event.target.value)}/></Field></div>}
    <div className="mt-6 grid gap-4 md:grid-cols-2 lg:grid-cols-4"><Field id="explore-search-value" label="Search preview"><Input id="explore-search-value" type="search" value={search} onChange={event=>setSearch(event.target.value)}/></Field><Field id="explore-filter-field" label="Filter field"><NativeSelect id="explore-filter-field" value={filterField} onChange={event=>{setFilterField(event.target.value);setFilterOperator('eq');setFilterValue('')}}><NativeSelectOption value="">None</NativeSelectOption>{available.filter(field=>fields.includes(field.Name)).map(field=><NativeSelectOption key={field.Name}>{field.Name}</NativeSelectOption>)}</NativeSelect></Field><Field id="explore-filter-operator" label="Operator"><NativeSelect id="explore-filter-operator" disabled={!filterField} value={filterOperator} onChange={event=>setFilterOperator(event.target.value)}>{operators(fieldByName(entity,filterField)?.Type).map(operator=><NativeSelectOption key={operator}>{operator}</NativeSelectOption>)}</NativeSelect></Field><Field id="explore-filter-value" label="Filter value"><Input id="explore-filter-value" disabled={!filterField} value={filterValue} onChange={event=>setFilterValue(event.target.value)}/></Field></div>
    <div className="mt-6 flex flex-wrap items-center gap-3"><Button data-testid="explore-preview" disabled={!viewName||(mode==='records'&&!fields.includes('id'))||(mode==='groups'&&!groupField)||!aggregateAlias||preview.isPending} onClick={()=>preview.mutate({fingerprint:previewFingerprint,candidate:previewCandidate})}>{preview.isPending?'Previewing…':'Preview'}</Button>{conflict&&<Check id="explore-replace" label="Replace existing draft" checked={replace} onChange={setReplace}/>}<Button variant="outline" data-testid="explore-save" disabled={!previewIsCurrent||preview.isPending||save.isPending||definitions.isFetching||Boolean(definitions.error)||!definitions.data?.etag||(conflict&&!replace)} onClick={()=>save.mutate()}>{save.isPending?'Saving…':conflict?'Replace draft':'Save draft'}</Button></div>
    {definitions.error&&<ErrorAlert error={definitions.error}/>} {preview.error&&<ErrorAlert error={preview.error}/>} {save.error&&<ErrorAlert error={save.error}/>} {validate.error&&<ErrorAlert error={validate.error}/>} {saved&&<StatusAlert>Saved View {viewName} to the deterministic Studio draft.</StatusAlert>}{validate.data&&<div className="mt-3 rounded-lg bg-muted p-3 text-sm" data-testid="explore-validation"><p>{validate.data.valid?'Draft validates.':'Draft has diagnostics.'} Semantic changes: {validate.data.changes?.length||0}.</p>{Boolean(validate.data.diagnostics?.length)&&<pre className="mt-2 overflow-auto text-xs">{JSON.stringify(validate.data.diagnostics,null,2)}</pre>}</div>}
    </SectionCard>
    <SectionCard title="Preview" description="Preview is ephemeral and executes through the same compiled View service as a published definition.">{preview.isPending?<LoadingState label="Running View preview…"/>:previewIsCurrent?<PreviewTable rows={preview.data!.data} fields={previewFields(preview.data!.data,mode,fields)}/>:<p className="text-muted-foreground">Configure the View and run a preview.</p>}</SectionCard>
  </Page>
}

function viewSpec(entity:string,fields:string[],searchFields:string[],filterField:string,filterOperator:string,sortField:string,descending:boolean,mode:ExploreMode,groupField:string,groupBucket:string,aggregateFunction:string,aggregateField:string,aggregateAlias:string,pageRoute:string){
  const filterName=filterField.replaceAll('.','_')
  const common={entity,exposedFilters:filterField?{[filterName]:{field:filterField,operator:filterOperator}}:{},defaultLimit:25,maxLimit:200}
  if(mode==='groups'){
    const groupOutput=groupBucket?groupField.replaceAll('.','_')+'_'+groupBucket:groupField.replaceAll('.','_')
    return {...common,fields:[groupField],groupBy:[{field:groupField,as:groupOutput,bucket:groupBucket||undefined}],aggregates:[{function:aggregateFunction,field:aggregateField,alias:aggregateAlias}],sort:[{field:aggregateAlias,desc:true}],displays:{chart:{type:pageRoute?'page':'block',route:pageRoute||undefined,title:{text:humanize(entity)},renderer:{type:'chart',groupField:groupOutput,metricField:aggregateAlias},pager:{type:'none'}},table:{type:'block',title:{text:humanize(entity)},renderer:{type:'table',fields:[{field:groupOutput,label:humanize(groupOutput)},{field:aggregateAlias,label:humanize(aggregateAlias)}]},pager:{type:'none'}}}}
  }
  if(mode==='metric')return {...common,fields:[],aggregates:[{function:aggregateFunction,field:aggregateField,alias:aggregateAlias}],displays:{metric:{type:pageRoute?'page':'block',route:pageRoute||undefined,title:{text:humanize(entity)},renderer:{type:'metric',metricField:aggregateAlias,metricLabel:humanize(aggregateAlias)},pager:{type:'none'}}}}
  return {...common,fields,search:searchFields.length?{fields:searchFields}:undefined,sort:sortField?[{field:sortField,desc:descending}]:[],displays:{table:{type:pageRoute?'page':'block',route:pageRoute||undefined,title:{text:humanize(entity)},renderer:{type:'table',fields:fields.map(field=>({field,label:humanize(field)}))},pager:{type:'cursor',pageSize:25}}}}
}
function entityFields(entity?:Entity){return entity?[{Name:'id',Label:'ID',Type:'uuid'},...entity.Fields,{Name:'created_at',Label:'Created at',Type:'datetime'},{Name:'updated_at',Label:'Updated at',Type:'datetime'},{Name:'version',Label:'Version',Type:'integer'}]:[]}
function fieldByName(entity:Entity|undefined,name:string){return entityFields(entity).find(field=>field.Name===name)}
function searchable(type?:string){return ['email','richtext','slug','string','text','url'].includes(type||'')}
function numeric(type?:string){return ['decimal','integer','money'].includes(type||'')}
function dateLike(type?:string){return ['date','datetime'].includes(type||'')}
function operators(type?:string){if(searchable(type))return ['eq','contains'];if(['date','datetime','decimal','integer','money'].includes(type||''))return ['eq','gte','lte'];return ['eq']}
function toggle(values:string[],name:string,checked:boolean){return checked?[...new Set([...values,name])]:values.filter(value=>value!==name)}
function Check({id,label,checked,onChange,disabled=false}:{id:string;label:string;checked:boolean;onChange:(checked:boolean)=>void;disabled?:boolean}){return <div className="flex items-center gap-2"><Checkbox id={id} checked={checked} disabled={disabled} onCheckedChange={value=>onChange(Boolean(value))}/><Label htmlFor={id}>{label}</Label></div>}
function PreviewTable({rows,fields}:{rows:Row[];fields:string[]}){if(!rows.length)return <p className="text-muted-foreground">No records match this preview.</p>;return <Table><TableHeader><TableRow>{fields.map(field=><TableHead key={field}>{humanize(field)}</TableHead>)}</TableRow></TableHeader><TableBody>{rows.map((row,index)=><TableRow key={String(row.id??index)}>{fields.map(field=><TableCell key={field}>{display(row[field])}</TableCell>)}</TableRow>)}</TableBody></Table>}
function previewFields(rows:Row[],mode:ExploreMode,fields:string[]){return mode==='records'?fields:Object.keys(rows[0]||{})}
function display(value:any){if(value==null)return '';if(typeof value==='object')return JSON.stringify(value);return String(value)}
function humanize(value:string){return value.replaceAll('_',' ').replace(/^./,letter=>letter.toUpperCase())}
