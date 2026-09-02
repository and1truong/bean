import {createContext,FormEvent,useContext,useEffect,useMemo,useRef,useState} from 'react'
import {Link,Route,Routes,useLocation,useNavigate,useSearchParams} from 'react-router-dom'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {api,APIError,FormElement,Manifest,Node,Session,ViewDisplay,ViewFilter,ViewPresentation} from './api'
import {callAction,encodeInput} from './action-client'
import {Admin,ResourceListBlock} from './Admin'
import {Studio} from './Studio'
import {ErrorAlert,Field,LoadingState,Page,PageHeader,SectionCard,StatusAlert} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Card,CardContent,CardDescription,CardHeader,CardTitle} from '@/components/ui/card'
import {Checkbox} from '@/components/ui/checkbox'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'
import {Textarea} from '@/components/ui/textarea'
import {Table,TableBody,TableCell,TableHead,TableHeader,TableRow} from '@/components/ui/table'

type Row=Record<string,any>
const CurrentPath=createContext<React.MutableRefObject<string>|null>(null)

function Login(){
  const nav=useNavigate();const loc=useLocation();const qc=useQueryClient()
  const[email,setEmail]=useState('');const[password,setPassword]=useState('');const[error,setError]=useState('')
  async function submit(event:FormEvent){
    event.preventDefault()
    try{
      const result=await api<{csrfToken:string;user:{Roles:string[]}}>('/api/auth/login',{method:'POST',body:JSON.stringify({email,password})})
      sessionStorage.setItem('bean_csrf',result.csrfToken)
      await qc.cancelQueries()
      qc.clear()
      const fallback=result.user.Roles.some(role=>role==='editor'||role==='administrator')?'/admin':'/'
      const requested=new URLSearchParams(loc.search).get('next')||fallback
      nav(requested.startsWith('/')&&!requested.startsWith('//')?requested:fallback)
    }catch(cause){setError((cause as Error).message)}
  }
  return <Shell><Page narrow><Card><CardHeader><CardTitle><h1 className="text-2xl">Sign in</h1></CardTitle><CardDescription>Access your Bean application.</CardDescription></CardHeader><CardContent><form className="space-y-4" onSubmit={submit}><Field id="login-email" label="Email"><Input id="login-email" data-testid="email" type="email" required value={email} onChange={event=>setEmail(event.target.value)}/></Field><Field id="login-password" label="Password"><Input id="login-password" data-testid="password" type="password" required value={password} onChange={event=>setPassword(event.target.value)}/></Field>{error&&<ErrorAlert error={error}/>}<Button className="w-full" data-testid="login" type="submit">Sign in</Button></form></CardContent></Card></Page></Shell>
}

function Shell({children}:{children:React.ReactNode}){
  const loc=useLocation();const nav=useNavigate();const qc=useQueryClient();const currentPath=useContext(CurrentPath);const logoutStarted=useRef(false)
  const session=useQuery({queryKey:['session'],queryFn:()=>api<Session>('/api/system/session')})
  const manifest=useQuery({queryKey:['manifest'],queryFn:()=>api<Manifest>('/api/system/manifest')})
  const roles=session.data?.user?.Roles||[]
  const editor=roles.includes('editor')||roles.includes('administrator')
  const administrator=roles.includes('administrator')
	const theme=manifest.data?.theme
  const logout=useMutation({mutationFn:async()=>{const path=loc.pathname;const result=await api<{protected?:boolean}>('/api/auth/logout?path='+encodeURIComponent(path),{method:'POST',body:'{}'});return {path,protected:path.startsWith('/admin')||path.startsWith('/studio')||result.protected===true}},onSuccess:async result=>{sessionStorage.removeItem('bean_csrf');await qc.resetQueries();const routeChanged=currentPath?.current!==result.path;logoutStarted.current=false;if(result.protected||routeChanged)nav('/',{replace:true})},onError:()=>{logoutStarted.current=false}})
  const stopNavigation=(event:React.MouseEvent)=>{if(logoutStarted.current||logout.isPending)event.preventDefault()}
  return <div className="min-h-screen bg-background" data-testid="application-shell" data-preset={theme?.Preset||'professional'} data-accent={theme?.Accent||'emerald'}><header className="border-b bg-primary text-primary-foreground"><div className="mx-auto flex w-full max-w-6xl flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6"><Link className="text-lg font-semibold tracking-tight" to="/" aria-disabled={logout.isPending} onClick={stopNavigation}>{theme?.DisplayName||'Bean'}</Link><nav className="flex flex-wrap items-center gap-1" aria-label="Primary navigation">{editor&&<Button variant="ghost" disabled={logout.isPending} asChild><Link to="/admin" aria-disabled={logout.isPending} onClick={stopNavigation}>Admin</Link></Button>}{administrator&&<Button variant="ghost" disabled={logout.isPending} asChild><Link to="/studio" aria-disabled={logout.isPending} onClick={stopNavigation}>Studio</Link></Button>}{session.data?.authenticated?<Button variant="ghost" onClick={()=>{logoutStarted.current=true;logout.mutate()}} disabled={logout.isPending}>Sign out</Button>:manifest.data?.authNavigation!==false?<>{manifest.data?.localRegistration?.Route&&<Button variant="ghost" asChild><Link to={manifest.data.localRegistration.Route}>Sign up</Link></Button>}<Button variant="secondary" asChild><Link to={'/login?next='+encodeURIComponent(loc.pathname)}>Sign in</Link></Button></>:null}</nav></div></header>{children}</div>
}

type RenderProps={
  Page:{title?:string;description?:string;protected?:boolean}
  Panel:{layout?:string}
  Region:{name?:string}
  TextBlock:{text?:string}
  ViewBlock:{name?:string;view?:string;display?:ViewDisplay;filters?:Record<string,ViewFilter>;fieldTypes?:Record<string,string>;presentation?:ViewPresentation;formattedFields?:string[];fileFields?:string[];maxRows?:number}
  EntityBlock:{name?:string;entity?:string;presentation?:ViewPresentation;formattedFields?:string[];fileFields?:string[]}
  ResourceListBlock:{name?:string;resource?:string;view?:string;filters?:string[];defaultFilters?:Record<string,any>}
  WebformBlock:{name?:string;webform?:string;form?:Manifest['webforms'][string]}
  ActionBlock:{action?:string}
  MenuBlock:{items?:Array<{Route:string;Label:string}>}
}
type RenderComponent=keyof RenderProps
type NodeRenderer<K extends RenderComponent>=(props:RenderProps[K],children?:Node[])=>React.ReactNode
const nodeRenderers:{[K in RenderComponent]:NodeRenderer<K>}={
  Page:(props,children)=><StructuralNode component="Page" title={props.title} children={children}/>,
  Panel:(_props,children)=><StructuralNode component="Panel" children={children}/>,
  Region:(_props,children)=><StructuralNode component="Region" children={children}/>,
  TextBlock:props=><p>{props.text}</p>,
  ViewBlock:props=><ViewBlock name={props.view||''} block={props.name||''} display={props.display} filters={props.filters||{}} fieldTypes={props.fieldTypes||{}} presentation={props.presentation||{}} formattedFields={props.formattedFields||[]} fileFields={props.fileFields||[]} maxRows={props.maxRows}/>,
  EntityBlock:props=><ViewBlock name={(props.entity||'')+'_list'} block={props.name||''} presentation={props.presentation||{}} formattedFields={props.formattedFields||[]} fileFields={props.fileFields||[]}/>,
  ResourceListBlock:props=><ResourceListBlock resource={props.resource||''} view={props.view||''} block={props.name||''} filters={props.filters} defaultFilters={props.defaultFilters}/>,
  WebformBlock:props=><WebformBlock name={props.webform||''} block={props.name||''} renderedForm={props.form}/>,
  ActionBlock:props=><ActionBlock name={props.action||''}/>,
  MenuBlock:props=><MenuBlock items={props.items||[]}/>,
}
function isRenderComponent(value:string):value is RenderComponent{return Object.hasOwn(nodeRenderers,value)}
function renderKnownNode(component:RenderComponent,props:Record<string,any>,children?:Node[]){const renderer=nodeRenderers[component] as NodeRenderer<RenderComponent>;return renderer(props,children)}
function StructuralNode({component,title,children}:{component:'Page'|'Panel'|'Region';title?:string;children?:Node[]}){useEffect(()=>{if(component!=='Page'||!title)return;const previous=document.title;document.title=title;return()=>{document.title=previous}},[component,title]);return <section className="space-y-4" data-component={component}>{title&&<h2 className="font-heading text-2xl font-semibold">{title}</h2>}{children?.map((child,index)=><Renderer key={index} node={child}/>)}</section>}
function Renderer({node}:{node:Node}){
  if(!isRenderComponent(node.component))return <section role="alert" data-component={node.component}>Unsupported render component: {node.component}</section>
  return renderKnownNode(node.component,node.props||{},node.children)
}

type ViewBlockProps={name:string;block:string;display?:ViewDisplay;filters?:Record<string,ViewFilter>;fieldTypes?:Record<string,string>;presentation:ViewPresentation;formattedFields:string[];fileFields:string[];maxRows?:number}
function ViewBlock(props:ViewBlockProps){
  const path=useLocation().pathname
  return <ViewBlockPage key={`${props.name}:${props.block}:${path}`} {...props} path={path}/>
}
function ViewBlockPage({name,block,display,filters={},fieldTypes={},presentation,formattedFields,fileFields,maxRows,path}:ViewBlockProps&{path:string}){
	const[cursors,setCursors]=useState<string[]>(['']);const cursor=cursors[cursors.length-1]
	const[search,setSearch]=useState('');const[submittedSearch,setSubmittedSearch]=useState('')
	const[urlParams,setURLParams]=useSearchParams();const controls=useMemo(()=>display?.Controls||[],[display?.Controls]);const pageDisplay=display?.Type==='page'
	const parameter=(filter:string)=>pageDisplay?filter:block+'.'+filter
	const initialControls=()=>Object.fromEntries(controls.map(control=>{const value=urlParams.get(parameter(control.Filter))??String(control.Default??'');return[control.Filter,controlInputValue(value,filters[control.Filter])]}))
	const[controlValues,setControlValues]=useState<Record<string,string>>(initialControls)
	const urlState=urlParams.toString()
	useEffect(()=>setControlValues(Object.fromEntries(controls.map(control=>{const value=urlParams.get(pageDisplay?control.Filter:block+'.'+control.Filter)??String(control.Default??'');return[control.Filter,controlInputValue(value,filters[control.Filter])]}))),[block,controls,filters,pageDisplay,urlParams,urlState])
	const mode=display?.Renderer?.Type||presentation.Mode
	const rowLimit=maxRows&&maxRows>0?maxRows:200
	const renderer=display?.Renderer
	const effectivePresentation:ViewPresentation=renderer?{Mode:renderer.Type,TitleField:renderer.TitleField,BodyField:renderer.BodyField,LinkRoute:renderer.LinkRoute,LinkField:renderer.LinkField,EmptyState:renderer.EmptyState||display.EmptyState,MetaFields:renderer.MetaFields,RichTextFields:renderer.RichTextFields,GroupField:renderer.GroupField,OrderField:renderer.OrderField,ParentField:renderer.ParentField,MoveAction:renderer.MoveAction,Columns:renderer.Columns,MetricField:renderer.MetricField,MetricLabel:renderer.MetricLabel,TimeField:renderer.TimeField,SearchFields:renderer.SearchFields}:presentation
	const query=new URLSearchParams({_page:path});if(pageDisplay)query.set('_display',block);else query.set('_block',block);for(const control of controls)query.set(control.Filter,urlParams.get(parameter(control.Filter))??String(control.Default??''));if(mode==='board'||mode==='tree')query.set('limit','200');else if(mode==='detail')query.set('limit',String(rowLimit));else if(display?.Pager?.PageSize)query.set('limit',String(display.Pager.PageSize));if(cursor)query.set('cursor',cursor);if(submittedSearch)query.set('q',submittedSearch)
  const request='/api/views/'+name+'?'+query.toString()
	const structured=mode==='board'||mode==='tree'||mode==='detail';const structuredLimit=mode==='detail'?rowLimit:200;const structuredLimitError=`This View display supports at most ${structuredLimit} rows.`
  const result=useQuery({queryKey:['public-view',request],queryFn:async()=>{const first=await api<{data:Row[];nextCursor:string}>(request);if(!structured)return first;const data=[...first.data];if(data.length>structuredLimit||(mode==='detail'&&first.nextCursor))throw new APIError(structuredLimitError);if(mode==='detail')return {data,nextCursor:''};let nextCursor=first.nextCursor;while(nextCursor&&data.length<200){const nextQuery=new URLSearchParams(query);nextQuery.set('cursor',nextCursor);nextQuery.set('limit',String(200-data.length));const next=await api<{data:Row[];nextCursor:string}>('/api/views/'+name+'?'+nextQuery);data.push(...next.data);if(data.length>200)throw new APIError(structuredLimitError);nextCursor=next.nextCursor}if(nextCursor)throw new APIError(structuredLimitError);return {data,nextCursor:''}}})
	const resolvedTitle=display?.Title?.Field?String(result.data?.data[0]?.[display.Title.Field]??display.Title.Fallback??''):display?.Title?.Text||''
	useEffect(()=>{if(!pageDisplay||!resolvedTitle)return;const previous=document.title;document.title=resolvedTitle;return()=>{document.title=previous}},[pageDisplay,resolvedTitle])
	let content:React.ReactNode
	if(result.isPending)content=<LoadingState/>
	else if(result.error)content=<ErrorAlert error={result.error}/>
	else if(!result.data.data.length)content=<Card><CardContent className="py-8 text-center text-muted-foreground">{display?.EmptyState||effectivePresentation.EmptyState||'Nothing to show.'}</CardContent></Card>
	else if(mode==='table')content=<TableView rows={result.data.data} fields={renderer?.Fields||[]} fieldTypes={fieldTypes}/>
	else if(mode==='board')content=<BoardView rows={result.data.data} presentation={effectivePresentation}/>
	else if(mode==='tree')content=<TreeView rows={result.data.data} presentation={effectivePresentation}/>
	else if(mode==='metric')content=<MetricView row={result.data.data[0]} presentation={effectivePresentation}/>
	else if(mode==='timeline')content=<div className="space-y-4"><TimelineView rows={result.data.data} presentation={effectivePresentation}/>{display?.Pager?.Type!=='none'&&<Pagination previousDisabled={cursors.length===1} nextDisabled={!result.data.nextCursor} previous={()=>setCursors(value=>value.slice(0,-1))} next={()=>setCursors(value=>[...value,result.data.nextCursor])}/>}</div>
	else{const rows=mode==='detail'?[mergeDetail(result.data.data,effectivePresentation.MetaFields||[])]:result.data.data;content=<div className="space-y-4">{rows.map(row=><Card key={String(row.id)+JSON.stringify(row)}><CardHeader>{!(mode==='detail'&&display?.Title?.Field)&&<CardTitle><h3>{effectivePresentation.LinkRoute?<Link className="hover:underline" to={viewLink(effectivePresentation.LinkRoute,row)}>{row[effectivePresentation.TitleField||'title']}</Link>:row[effectivePresentation.TitleField||'title']||row.name}</h3></CardTitle>}{effectivePresentation.MetaFields?.length?<CardDescription className="flex flex-wrap gap-2">{effectivePresentation.MetaFields.map(field=><span key={field}>{String(row[field]??'')}</span>)}</CardDescription>:null}</CardHeader><CardContent><ViewBody row={row} view={name} page={path} block={block} display={pageDisplay} field={effectivePresentation.BodyField||'body'} rich={formattedFields.includes(effectivePresentation.BodyField||'body')} file={fileFields.includes(effectivePresentation.BodyField||'body')}/></CardContent></Card>)}{mode!=='detail'&&display?.Pager?.Type!=='none'&&<Pagination previousDisabled={cursors.length===1} nextDisabled={!result.data.nextCursor} previous={()=>setCursors(value=>value.slice(0,-1))} next={()=>setCursors(value=>[...value,result.data.nextCursor])}/>}</div>}
	const applyControls=(event:FormEvent)=>{event.preventDefault();const next=new URLSearchParams(urlParams);for(const control of controls){const name=parameter(control.Filter);const original=urlParams.get(name)??String(control.Default??'');next.set(name,controlQueryValue(controlValues[control.Filter]??'',filters[control.Filter],original))}setCursors(['']);setURLParams(next,{replace:true})}
	return <div className="space-y-4">{pageDisplay?<PageHeader title={resolvedTitle||display?.Title?.Fallback||humanize(name)} description={display?.Description}/>:display&&resolvedTitle?<h2 className="font-heading text-2xl font-semibold">{resolvedTitle}</h2>:null} {controls.length?<form className="grid items-end gap-4 sm:grid-cols-2 lg:flex lg:flex-wrap" onSubmit={applyControls}>{controls.map(control=><ViewFilterControl key={control.Filter} scope={block} control={control} filter={filters[control.Filter]} value={controlValues[control.Filter]??''} onChange={value=>setControlValues(current=>({...current,[control.Filter]:value}))}/>)}<Button type="submit">Apply</Button></form>:null}{effectivePresentation.SearchFields?.length?<form className="flex gap-2" role="search" onSubmit={event=>{event.preventDefault();setCursors(['']);setSubmittedSearch(search)}}><Input aria-label={'Search '+name.replaceAll('_',' ')} type="search" value={search} onChange={event=>setSearch(event.target.value)}/><Button type="submit">Search</Button></form>:null}{content}{mode==='table'&&display?.Pager?.Type!=='none'&&result.data&&<Pagination previousDisabled={cursors.length===1} nextDisabled={!result.data.nextCursor} previous={()=>setCursors(value=>value.slice(0,-1))} next={()=>setCursors(value=>[...value,result.data.nextCursor])}/>}</div>
}

function TableView({rows,fields,fieldTypes}:{rows:Row[];fields:NonNullable<ViewDisplay['Renderer']['Fields']>;fieldTypes:Record<string,string>}){return <Table><TableHeader><TableRow>{fields.map(column=><TableHead scope="col" key={column.Field}>{column.Label||humanize(column.Field)}</TableHead>)}</TableRow></TableHeader><TableBody>{rows.map((row,index)=><TableRow key={String(row.id??index)}>{fields.map(column=><TableCell key={column.Field}>{column.LinkRoute?<Link className="font-medium text-primary hover:underline" to={viewLink(column.LinkRoute,row)}>{displayValue(row[column.Field],fieldTypes[column.Field])}</Link>:displayValue(row[column.Field],fieldTypes[column.Field])}</TableCell>)}</TableRow>)}</TableBody></Table>}
function displayValue(value:any,type?:string){if(value===null||value===undefined)return '';if(type==='boolean'||typeof value==='boolean')return value?'Yes':'No';if(type==='date'||type==='datetime')return formatDemoDate(value);if(type==='integer'||type==='decimal'||type==='money')return new Intl.NumberFormat().format(Number(value));if(typeof value==='object')return JSON.stringify(value);return String(value)}
function ViewFilterControl({scope,control,filter,value,onChange}:{scope:string;control:NonNullable<ViewDisplay['Controls']>[number];filter?:ViewFilter;value:string;onChange:(value:string)=>void}){const id='view-filter-'+scope+'-'+control.Filter;const label=control.Label||filter?.Label||humanize(control.Filter);const widget=control.Widget==='auto'||!control.Widget?(filter?.Type==='enum'?'select':filter?.Type==='boolean'?'checkbox':filter?.Type==='integer'||filter?.Type==='decimal'||filter?.Type==='money'?'number':filter?.Type==='date'||filter?.Type==='datetime'?'date':'text'):control.Widget;if(widget==='select')return <Field id={id} label={label}><NativeSelect id={id} value={value} onChange={event=>onChange(event.target.value)}><NativeSelectOption value="">All</NativeSelectOption>{filter?.Options?.map(option=><NativeSelectOption key={option} value={option}>{humanize(option)}</NativeSelectOption>)}</NativeSelect></Field>;if(widget==='checkbox')return <div className="flex items-center gap-2"><Checkbox id={id} checked={value==='true'} onCheckedChange={checked=>onChange(checked?'true':'false')}/><Label htmlFor={id}>{label}</Label></div>;const type=widget==='number'?'number':widget==='date'&&filter?.Type==='datetime'?'datetime-local':widget==='date'?'date':'text';return <Field id={id} label={label}><Input id={id} type={type} step={type==='datetime-local'?'1':undefined} value={value} onChange={event=>onChange(event.target.value)}/></Field>}
export function controlInputValue(value:string,filter?:ViewFilter){if(!value||filter?.Type!=='datetime')return value;const date=new Date(value);if(Number.isNaN(date.valueOf()))return value;return new Date(date.valueOf()-date.getTimezoneOffset()*60_000).toISOString().slice(0,19)}
export function controlQueryValue(value:string,filter?:ViewFilter,original=''){if(!value||filter?.Type!=='datetime')return value;if(original&&controlInputValue(original,filter)===value)return original;const date=new Date(value);return Number.isNaN(date.valueOf())?value:date.toISOString()}

function MetricView({row,presentation}:{row:Row;presentation:ViewPresentation}){return <Card><CardHeader><CardDescription>{presentation.MetricLabel||humanize(presentation.MetricField||'metric')}</CardDescription><CardTitle><span className="text-4xl" data-testid="metric-value">{String(row[presentation.MetricField||'']??0)}</span></CardTitle></CardHeader></Card>}
function TimelineView({rows,presentation}:{rows:Row[];presentation:ViewPresentation}){return <ol className="relative space-y-6 border-l pl-6" data-testid="timeline-view">{rows.map(row=><li key={String(row.id)+JSON.stringify(row)}><span className="absolute -ml-[1.85rem] mt-1.5 size-3 rounded-full bg-primary"/><time className="text-sm text-muted-foreground">{formatDemoDate(row[presentation.TimeField||''])}</time><h3 className="font-semibold">{presentation.LinkRoute?<Link className="hover:underline" to={viewLink(presentation.LinkRoute,row)}>{row[presentation.TitleField||'title']}</Link>:row[presentation.TitleField||'title']}</h3>{presentation.BodyField?<p>{String(row[presentation.BodyField]??'')}</p>:null}{presentation.MetaFields?.length?<p className="text-sm text-muted-foreground">{presentation.MetaFields.map(field=>String(row[field]??'')).filter(Boolean).join(' · ')}</p>:null}</li>)}</ol>}
function formatDemoDate(value:any){const date=new Date(String(value));return Number.isNaN(date.valueOf())?String(value??''):new Intl.DateTimeFormat('en-US',{year:'numeric',month:'short',day:'numeric',timeZone:'UTC'}).format(date)}

function mergeDetail(rows:Row[],meta:string[]){const result={...rows[0]};for(const field of meta){const values=[...new Set(rows.map(row=>row[field]).filter(value=>value!==null&&value!==undefined&&value!==''))];result[field]=values.join(', ')}return result}
function ViewBody({row,view,page,block,display,field,rich,file}:{row:Row;view:string;page:string;block:string;display:boolean;field:string;rich:boolean;file:boolean}){const selected=row[field];const value=String(selected??row.excerpt??row.description??'');if(file&&selected){const query=new URLSearchParams({view,_page:page});query.set(display?'_display':'_block',block);return <Button variant="outline" asChild><a href={'/api/files/'+encodeURIComponent(String(selected))+'?'+query}>Download attachment</a></Button>}return rich&&selected!==null&&selected!==undefined?<div className="rich-text" dangerouslySetInnerHTML={{__html:String(selected)}}/>:<p className="leading-7">{value}</p>}
function viewLink(template:string,row:Row){return template.replace(/:([a-zA-Z0-9_.]+)/g,(_,field)=>encodeURIComponent(String(row[field]??'')))}

function BoardView({rows,presentation}:{rows:Row[];presentation:ViewPresentation}){
  const queryClient=useQueryClient();const manifest=useQuery({queryKey:['manifest'],queryFn:()=>api<Manifest>('/api/system/manifest')})
  const move=useMutation({mutationFn:({id,status}:{id:string;status:string})=>callAction(presentation.MoveAction||'',{id,[presentation.GroupField||'status']:status}),onSuccess:()=>void queryClient.invalidateQueries({queryKey:['public-view']})})
  const columns=presentation.Columns||[];const group=presentation.GroupField||'status';const title=presentation.TitleField||'title';const order=presentation.OrderField;const action=manifest.data?.actions?.[presentation.MoveAction||''];const transitions=action?.Transitions??(action?.Lifecycle?manifest.data?.lifecycles?.[action.Lifecycle]?.Transitions:undefined)??{}
  return <div className="overflow-x-auto"><div className="grid min-w-[48rem] gap-4" style={{gridTemplateColumns:`repeat(${columns.length}, minmax(15rem, 1fr))`}}>{columns.map(column=><section className="rounded-xl bg-muted/60 p-3" key={column}><h3 className="mb-3 font-semibold">{humanize(column)}</h3><div className="space-y-3">{rows.filter(row=>row[group]===column).sort((a,b)=>order?Number(a[order]??0)-Number(b[order]??0):0).map(row=>{const current=String(row[group]);const allowed=new Set([current,...(transitions[current]||[])]);const targets=columns.filter(value=>allowed.has(value));return <Card key={row.id}><CardHeader><CardTitle>{presentation.LinkRoute?<Link className="hover:underline" to={viewLink(presentation.LinkRoute,row)}>{row[title]}</Link>:row[title]}</CardTitle>{presentation.MetaFields?.length?<CardDescription>{presentation.MetaFields.map(field=>String(row[field]??'')).filter(Boolean).join(' · ')}</CardDescription>:null}</CardHeader><CardContent className="space-y-3"><p>{String(row[presentation.BodyField||'description']??'')}</p><Field id={'move-'+row.id} label="Status"><NativeSelect id={'move-'+row.id} aria-label={'Status for '+row[title]} value={current} disabled={move.isPending||targets.length===1} onChange={event=>move.mutate({id:String(row.id),status:event.target.value})}>{targets.map(value=><NativeSelectOption key={value} value={value}>{humanize(value)}</NativeSelectOption>)}</NativeSelect></Field></CardContent></Card>})}{!rows.some(row=>row[group]===column)&&<p className="text-sm text-muted-foreground">No tasks</p>}</div></section>)}</div>{move.error&&<ErrorAlert error={move.error}/>}</div>
}

function TreeView({rows,presentation}:{rows:Row[];presentation:ViewPresentation}){
  const parent= presentation.ParentField||'parent_id';const order=presentation.OrderField;const ids=new Set(rows.map(row=>String(row.id)))
  const[collapsed,setCollapsed]=useState<Set<string>>(new Set())
  const children=new Map<string,Row[]>();const roots:Row[]=[]
  for(const row of rows){const parentID=String(row[parent]??'');if(!parentID||!ids.has(parentID))roots.push(row);else children.set(parentID,[...(children.get(parentID)||[]),row])}
  const reachable=new Set<string>();const mark=(seed:Row)=>{const queue=[seed];while(queue.length){const current=queue.shift()!;const id=String(current.id);if(reachable.has(id))continue;reachable.add(id);queue.push(...(children.get(id)||[]))}}
  roots.forEach(mark);for(const row of rows)if(!reachable.has(String(row.id))){roots.push(row);mark(row)}
  const sort=(items:Row[])=>items.sort((a,b)=>order?Number(a[order]||0)-Number(b[order]||0):String(a[presentation.TitleField||'title']).localeCompare(String(b[presentation.TitleField||'title'])))
  const branch=(row:Row,depth:number,path:Set<string>):React.ReactNode=>{const id=String(row.id);if(path.has(id))return <li className="text-destructive" key={id}>Invalid hierarchy cycle at {id}</li>;const descendants=sort([...(children.get(id)||[])]);const next=new Set(path).add(id);return <li key={id}><div className="flex items-center gap-2 border-b py-2" style={{paddingLeft:`${Math.min(depth,12)*1.25}rem`}}>{descendants.length?<Button size="sm" variant="ghost" aria-label={(collapsed.has(id)?'Expand ':'Collapse ')+String(row[presentation.TitleField||'title'])} onClick={()=>setCollapsed(current=>{const value=new Set(current);if(value.has(id))value.delete(id);else value.add(id);return value})}>{collapsed.has(id)?'›':'⌄'}</Button>:<span className="inline-block w-8"/>}{presentation.LinkRoute?<Link className="font-medium text-primary hover:underline" to={viewLink(presentation.LinkRoute,row)}>{row[presentation.TitleField||'title']}</Link>:<span>{row[presentation.TitleField||'title']}</span>}{presentation.MetaFields?.map(field=><span className="text-sm text-muted-foreground" key={field}>{String(row[field]??'')}</span>)}</div>{!collapsed.has(id)&&descendants.length>0&&<ul>{descendants.map(child=>branch(child,depth+1,next))}</ul>}</li>}
  return <ul className="rounded-xl border" data-testid="tree-view">{sort(roots).map(row=>branch(row,0,new Set()))}</ul>
}

type WebformBlockProps={name:string;block:string;renderedForm?:Manifest['webforms'][string]}
function WebformBlock(props:WebformBlockProps){
  const path=useLocation().pathname
  return <WebformBlockPage key={`${props.name}:${props.block}:${path}`} {...props} path={path}/>
}
function WebformBlockPage({name,block,renderedForm,path}:WebformBlockProps&{path:string}){
  const manifest=useQuery({queryKey:['manifest'],queryFn:()=>api<Manifest>('/api/system/manifest'),enabled:!renderedForm});const form=renderedForm||manifest.data?.webforms?.[name]
  const[values,setValues]=useState<Row>({});const[step,setStep]=useState(0);const[done,setDone]=useState('')
  const query=new URLSearchParams({_page:path,_block:block})
  const submit=useMutation({mutationFn:()=>api<{confirmation:string}>('/api/webforms/'+name+'/submit?'+query,{method:'POST',body:formBody(form,values)}),onSuccess:result=>setDone(result.confirmation)})
  if(!form)return null
  if(done)return <StatusAlert>{done}</StatusAlert>
  const names=form.Steps?.[step]
  const elements=(names?form.Elements.filter(element=>names.includes(element.Name)):form.Elements).filter(element=>evaluate(element.Visible,values))
  return <SectionCard><form className="space-y-4" onSubmit={event=>{event.preventDefault();if(form.Steps&&step<form.Steps.length-1)setStep(step+1);else submit.mutate()}}>{elements.map(element=><FormField key={element.Name} element={{...element,Required:element.Required||(element.RequiredWhen?evaluate(element.RequiredWhen,values):false)}} value={values[element.Name]} error={(submit.error as APIError|undefined)?.fields?.[element.Name]} onChange={value=>setValues(current=>({...current,[element.Name]:value}))}/>)}{submit.error&&<ErrorAlert error={submit.error}/>}<Button type="submit" disabled={submit.isPending}>{form.Steps&&step<form.Steps.length-1?'Next':submit.isPending?'Submitting…':'Submit'}</Button></form></SectionCard>
}

function formBody(form:Manifest['webforms'][string]|undefined,values:Row):BodyInit{
  return encodeInput(values,(form?.Elements||[]).some(element=>element.Type==='file'))
}

export function evaluate(expression:import('./api').Expression|undefined|null,values:Row):boolean{
  if(!expression)return true
  const args=expression.Args||[]
  if(expression.Op==='and')return args.every(item=>evaluate(item,values))
  if(expression.Op==='or')return args.some(item=>evaluate(item,values))
  if(expression.Op==='not'){if(args.length!==1)throw new Error('not requires one argument');return !evaluate(args[0],values)}
  const resolve=(value:typeof expression.Left)=>{if(!value)throw new Error('expression value is missing');if(value.Source==='input')return values[value.Name||''];if(value.Source==='literal')return value.Literal;throw new Error('unsupported client value source '+value.Source)}
  const left=resolve(expression.Left)
  if(expression.Op==='is_null')return left==null
  if(expression.Op==='is_not_null')return left!=null
  const right=resolve(expression.Right)
  if(expression.Op==='eq')return left===right
  if(expression.Op==='ne')return left!==right
  if(expression.Op==='gt'||expression.Op==='gte'||expression.Op==='lt'||expression.Op==='lte'){
    const inputPending=(value:typeof expression.Left)=>value?.Source==='input'&&!Object.prototype.hasOwnProperty.call(values,value.Name||'')
    const leftPending=inputPending(expression.Left),rightPending=inputPending(expression.Right)
    if((!leftPending&&typeof left!=='number')||(!rightPending&&typeof right!=='number'))throw new Error('comparison requires numbers')
    if(leftPending||rightPending)return false
    if(expression.Op==='gt')return left>right;if(expression.Op==='gte')return left>=right;if(expression.Op==='lt')return left<right;return left<=right
  }
  if(expression.Op==='in'||expression.Op==='not_in'){if(!Array.isArray(right))throw new Error(expression.Op+' requires a list');const included=right.some(value=>Object.is(value,left));return expression.Op==='in'?included:!included}
  if(expression.Op==='contains')return String(left).includes(String(right))
  if(expression.Op==='starts_with')return String(left).startsWith(String(right))
  if(expression.Op==='ends_with')return String(left).endsWith(String(right))
  throw new Error('unsupported client expression operator '+expression.Op)
}

function FormField({element,value,error,onChange}:{element:FormElement;value:any;error?:string;onChange:(value:any)=>void}){
  if(element.Type==='group')return <SectionCard title={humanize(element.Name)}><div className="space-y-4"><Button type="button" variant="outline" onClick={()=>onChange([...(value||[]),{}])}>Add</Button>{(value||[]).map((row:Row,index:number)=><div className="grid gap-4 rounded-lg border p-4 sm:grid-cols-2" key={index}>{element.Children?.map(child=><FormField key={child.Name} element={child} value={row[child.Name]} onChange={next=>{const rows=[...value];rows[index]={...row,[child.Name]:next};onChange(rows)}}/>)}</div>)}</div></SectionCard>
  const id='form-'+element.Name;const label=element.Name.replaceAll('_',' ')
  if(element.Type==='textarea')return <Field id={id} label={label} error={error}><Textarea id={id} required={element.Required} value={value??''} onChange={event=>onChange(event.target.value)}/></Field>
  if(element.Type==='file')return <Field id={id} label={label} error={error}><Input id={id} type="file" required={element.Required} onChange={event=>onChange(event.target.files?.[0])}/></Field>
  if(element.Type==='select'||element.Type==='entity reference')return <Field id={id} label={label} error={error}><NativeSelect id={id} required={element.Required} value={value??''} onChange={event=>onChange(event.target.value)}><NativeSelectOption value=""/>{element.Options?.map(option=><NativeSelectOption key={option}>{option}</NativeSelectOption>)}</NativeSelect></Field>
  if(element.Type==='checkbox')return <div className="grid gap-2"><div className="flex items-center gap-2"><Checkbox id={id} checked={Boolean(value)} onCheckedChange={checked=>onChange(Boolean(checked))}/><Label htmlFor={id}>{label}</Label></div>{error&&<p className="text-sm text-destructive" role="alert">{error}</p>}</div>
  const type=element.Type==='email'?'email':element.Type==='password'?'password':element.Type==='number'||element.Type==='integer'?'number':element.Type==='date'?'date':element.Type==='datetime'?'datetime-local':'text'
  return <Field id={id} label={label} error={error}><Input id={id} type={type} required={element.Required} value={value??''} onChange={event=>onChange(element.Type==='number'||element.Type==='integer'?Number(event.target.value):event.target.value)}/></Field>
}

function ActionBlock({name}:{name:string}){const mutation=useMutation({mutationFn:()=>callAction(name,{})});return <div className="space-y-3"><Button onClick={()=>mutation.mutate()} disabled={mutation.isPending}>{humanize(name)}</Button>{mutation.error&&<ErrorAlert error={mutation.error}/>}</div>}
function MenuBlock({items}:{items:Array<{Route:string;Label:string}>}){return <nav className="flex flex-wrap gap-2" aria-label="Page navigation">{items.map(item=><Button key={item.Route} variant="outline" asChild><Link to={item.Route}>{item.Label}</Link></Button>)}</nav>}
function Pagination({previousDisabled,nextDisabled,previous,next}:{previousDisabled:boolean;nextDisabled:boolean;previous:()=>void;next:()=>void}){return <nav className="flex justify-end gap-2" aria-label="Pagination"><Button variant="outline" disabled={previousDisabled} onClick={previous}>Previous</Button><Button variant="outline" disabled={nextDisabled} onClick={next}>Next</Button></nav>}
function humanize(value:string){return value.replaceAll('_',' ').replace(/^./,letter=>letter.toUpperCase())}

type PageResult={tree:Node}
function loadPage(path:string,search:string){const query=new URLSearchParams(search);query.set('path',path);return api<PageResult>('/api/system/page?'+query)}
function Public(){
  const loc=useLocation();const pageKey=loc.search?['page',loc.pathname,loc.search]:['page',loc.pathname];const result=useQuery({queryKey:pageKey,queryFn:()=>loadPage(loc.pathname,loc.search)})
  if(result.isPending)return <Shell><Page><LoadingState/></Page></Shell>
  if(result.error)return <Shell><Page><PageHeader title="Bean" description="Metadata-driven applications, compiled."/></Page></Shell>
  return <Shell><Page className="space-y-6"><Renderer node={result.data.tree}/></Page></Shell>
}

export default function App(){const loc=useLocation();const currentPath=useRef(loc.pathname);currentPath.current=loc.pathname;return <CurrentPath.Provider value={currentPath}><Routes><Route path="/login" element={<Login/>}/><Route path="/studio" element={<Shell><Studio/></Shell>}/><Route path="/admin/*" element={<Shell><Admin/></Shell>}/><Route path="*" element={<Public/>}/></Routes></CurrentPath.Provider>}
