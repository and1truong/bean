import {createContext,FormEvent,useContext,useRef,useState} from 'react'
import {Link,Route,Routes,useLocation,useNavigate} from 'react-router-dom'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {api,APIError,FormElement,Manifest,Node,Session,ViewPresentation} from './api'
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
  const logout=useMutation({mutationFn:async()=>{const path=loc.pathname;const result=await api<{protected?:boolean}>('/api/auth/logout?path='+encodeURIComponent(path),{method:'POST',body:'{}'});return {path,protected:path.startsWith('/admin')||path.startsWith('/studio')||result.protected===true}},onSuccess:async result=>{sessionStorage.removeItem('bean_csrf');await qc.resetQueries();const routeChanged=currentPath?.current!==result.path;logoutStarted.current=false;if(result.protected||routeChanged)nav('/',{replace:true})},onError:()=>{logoutStarted.current=false}})
  const stopNavigation=(event:React.MouseEvent)=>{if(logoutStarted.current||logout.isPending)event.preventDefault()}
  return <><header className="border-b bg-primary text-primary-foreground"><div className="mx-auto flex w-full max-w-6xl flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6"><Link className="text-lg font-semibold tracking-tight" to="/" aria-disabled={logout.isPending} onClick={stopNavigation}>Bean</Link><nav className="flex flex-wrap items-center gap-1" aria-label="Primary navigation">{editor&&<Button variant="ghost" disabled={logout.isPending} asChild><Link to="/admin" aria-disabled={logout.isPending} onClick={stopNavigation}>Admin</Link></Button>}{administrator&&<Button variant="ghost" disabled={logout.isPending} asChild><Link to="/studio" aria-disabled={logout.isPending} onClick={stopNavigation}>Studio</Link></Button>}{session.data?.authenticated?<Button variant="ghost" onClick={()=>{logoutStarted.current=true;logout.mutate()}} disabled={logout.isPending}>Sign out</Button>:manifest.data?.authNavigation!==false?<>{manifest.data?.localRegistration?.Route&&<Button variant="ghost" asChild><Link to={manifest.data.localRegistration.Route}>Sign up</Link></Button>}<Button variant="secondary" asChild><Link to={'/login?next='+encodeURIComponent(loc.pathname)}>Sign in</Link></Button></>:null}</nav></div></header>{children}</>
}

function Renderer({node}:{node:Node}){
  if(node.component==='TextBlock')return <p>{node.props?.text}</p>
  if(node.component==='ViewBlock'||node.component==='EntityBlock')return <ViewBlock name={node.props?.view||node.props?.entity+'_list'} block={node.props?.name} presentation={node.props?.presentation||{}} formattedFields={node.props?.formattedFields||[]} fileFields={node.props?.fileFields||[]}/>
  if(node.component==='ResourceListBlock')return <ResourceListBlock resource={node.props?.resource} view={node.props?.view} block={node.props?.name} filters={node.props?.filters} defaultFilters={node.props?.defaultFilters}/>
  if(node.component==='WebformBlock')return <WebformBlock name={node.props?.webform} block={node.props?.name} renderedForm={node.props?.form}/>
  if(node.component==='ActionBlock')return <ActionBlock name={node.props?.action}/>
  if(node.component==='MenuBlock')return <MenuBlock items={node.props?.items||[]}/>
  return <section className="space-y-4" data-component={node.component}><h2 className="font-heading text-2xl font-semibold">{node.props?.title}</h2>{node.children?.map((child,index)=><Renderer key={index} node={child}/>)}</section>
}

type ViewBlockProps={name:string;block:string;presentation:ViewPresentation;formattedFields:string[];fileFields:string[]}
function ViewBlock(props:ViewBlockProps){
  const path=useLocation().pathname
  return <ViewBlockPage key={`${props.name}:${props.block}:${path}`} {...props} path={path}/>
}
function ViewBlockPage({name,block,presentation,formattedFields,fileFields,path}:ViewBlockProps&{path:string}){
  const[cursors,setCursors]=useState<string[]>(['']);const cursor=cursors[cursors.length-1]
  const query=new URLSearchParams({_page:path,_block:block});if(presentation.Mode==='board'||presentation.Mode==='tree')query.set('limit','200');else if(cursor)query.set('cursor',cursor)
  const request='/api/views/'+name+'?'+query.toString()
  const structured=presentation.Mode==='board'||presentation.Mode==='tree'
  const result=useQuery({queryKey:['public-view',request],queryFn:async()=>{const first=await api<{data:Row[];nextCursor:string}>(request);if(!structured)return first;const data=[...first.data];let nextCursor=first.nextCursor;while(nextCursor&&data.length<200){const nextQuery=new URLSearchParams(query);nextQuery.set('cursor',nextCursor);nextQuery.set('limit',String(200-data.length));const next=await api<{data:Row[];nextCursor:string}>('/api/views/'+name+'?'+nextQuery);data.push(...next.data);nextCursor=next.nextCursor}if(nextCursor)throw new APIError('Board and tree Views support at most 200 rows.');return {data,nextCursor:''}}})
  if(result.isPending)return <LoadingState/>
  if(result.error)return <ErrorAlert error={result.error}/>
  if(!result.data.data.length)return <Card><CardContent className="py-8 text-center text-muted-foreground">{presentation.EmptyState||'Nothing to show.'}</CardContent></Card>
  if(presentation.Mode==='board')return <BoardView rows={result.data.data} presentation={presentation}/>
  if(presentation.Mode==='tree')return <TreeView rows={result.data.data} presentation={presentation}/>
  const rows=presentation.Mode==='detail'?[mergeDetail(result.data.data,presentation.MetaFields||[])]:result.data.data
  return <div className="space-y-4">{rows.map(row=><Card key={String(row.id)+JSON.stringify(row)}><CardHeader><CardTitle><h3>{presentation.LinkRoute?<Link className="hover:underline" to={viewLink(presentation.LinkRoute,row)}>{row[presentation.TitleField||'title']}</Link>:row[presentation.TitleField||'title']||row.name}</h3></CardTitle>{presentation.MetaFields?.length?<CardDescription className="flex flex-wrap gap-2">{presentation.MetaFields.map(field=><span key={field}>{String(row[field]??'')}</span>)}</CardDescription>:null}</CardHeader><CardContent><ViewBody row={row} view={name} page={path} block={block} field={presentation.BodyField||'body'} rich={formattedFields.includes(presentation.BodyField||'body')} file={fileFields.includes(presentation.BodyField||'body')}/></CardContent></Card>)}{presentation.Mode!=='detail'&&<Pagination previousDisabled={cursors.length===1} nextDisabled={!result.data.nextCursor} previous={()=>setCursors(value=>value.slice(0,-1))} next={()=>setCursors(value=>[...value,result.data.nextCursor])}/>}</div>
}

function mergeDetail(rows:Row[],meta:string[]){const result={...rows[0]};for(const field of meta){const values=[...new Set(rows.map(row=>row[field]).filter(value=>value!==null&&value!==undefined&&value!==''))];result[field]=values.join(', ')}return result}
function ViewBody({row,view,page,block,field,rich,file}:{row:Row;view:string;page:string;block:string;field:string;rich:boolean;file:boolean}){const selected=row[field];const value=String(selected??row.excerpt??row.description??'');if(file&&selected){const query=new URLSearchParams({view,_page:page,_block:block});return <Button variant="outline" asChild><a href={'/api/files/'+encodeURIComponent(String(selected))+'?'+query}>Download attachment</a></Button>}return rich&&selected!==null&&selected!==undefined?<div className="rich-text" dangerouslySetInnerHTML={{__html:String(selected)}}/>:<p className="leading-7">{value}</p>}
function viewLink(template:string,row:Row){return template.replace(/:([a-zA-Z0-9_.]+)/g,(_,field)=>encodeURIComponent(String(row[field]??'')))}

function BoardView({rows,presentation}:{rows:Row[];presentation:ViewPresentation}){
  const queryClient=useQueryClient();const manifest=useQuery({queryKey:['manifest'],queryFn:()=>api<Manifest>('/api/system/manifest')})
  const move=useMutation({mutationFn:({id,status}:{id:string;status:string})=>api('/api/actions/'+presentation.MoveAction,{method:'POST',body:JSON.stringify({id,[presentation.GroupField||'status']:status})}),onSuccess:()=>void queryClient.invalidateQueries({queryKey:['public-view']})})
  const columns=presentation.Columns||[];const group=presentation.GroupField||'status';const title=presentation.TitleField||'title';const order=presentation.OrderField;const transitions=manifest.data?.actions?.[presentation.MoveAction||'']?.Transitions||{}
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
  const hasFile=(form?.Elements||[]).some(element=>element.Type==='file')
  if(!hasFile)return JSON.stringify(values)
  const body=new FormData();for(const[name,value]of Object.entries(values)){if(value instanceof File)body.append(name,value);else if(value!==undefined&&value!==null)body.append(name,typeof value==='string'?value:JSON.stringify(value))}return body
}

function evaluate(expression:import('./api').Expression|undefined|null,values:Row):boolean{
  if(!expression)return true;const args=expression.Args||[]
  if(expression.Op==='and')return args.every(item=>evaluate(item,values));if(expression.Op==='or')return args.some(item=>evaluate(item,values));if(expression.Op==='not')return !evaluate(args[0],values)
  const resolve=(value:typeof expression.Left)=>value?.Source==='input'?values[value.Name||'']:value?.Source==='literal'?value.Literal:undefined
  const left=resolve(expression.Left),right=resolve(expression.Right)
  if(expression.Op==='eq')return left===right;if(expression.Op==='ne')return left!==right;if(expression.Op==='is_null')return left==null;if(expression.Op==='is_not_null')return left!=null;return false
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

function ActionBlock({name}:{name:string}){const mutation=useMutation({mutationFn:()=>api('/api/actions/'+name,{method:'POST',body:'{}'})});return <div className="space-y-3"><Button onClick={()=>mutation.mutate()} disabled={mutation.isPending}>{humanize(name)}</Button>{mutation.error&&<ErrorAlert error={mutation.error}/>}</div>}
function MenuBlock({items}:{items:Array<{Route:string;Label:string}>}){return <nav className="flex flex-wrap gap-2" aria-label="Page navigation">{items.map(item=><Button key={item.Route} variant="outline" asChild><Link to={item.Route}>{item.Label}</Link></Button>)}</nav>}
function Pagination({previousDisabled,nextDisabled,previous,next}:{previousDisabled:boolean;nextDisabled:boolean;previous:()=>void;next:()=>void}){return <nav className="flex justify-end gap-2" aria-label="Pagination"><Button variant="outline" disabled={previousDisabled} onClick={previous}>Previous</Button><Button variant="outline" disabled={nextDisabled} onClick={next}>Next</Button></nav>}
function humanize(value:string){return value.replaceAll('_',' ').replace(/^./,letter=>letter.toUpperCase())}

type PageResult={tree:Node}
function loadPage(path:string){return api<PageResult>('/api/system/page?path='+encodeURIComponent(path))}
function Public(){
  const loc=useLocation();const result=useQuery({queryKey:['page',loc.pathname],queryFn:()=>loadPage(loc.pathname)})
  if(result.isPending)return <Shell><Page><LoadingState/></Page></Shell>
  if(result.error)return <Shell><Page><PageHeader title="Bean" description="Metadata-driven applications, compiled."/></Page></Shell>
  return <Shell><Page className="space-y-6"><Renderer node={result.data.tree}/></Page></Shell>
}

export default function App(){const loc=useLocation();const currentPath=useRef(loc.pathname);currentPath.current=loc.pathname;return <CurrentPath.Provider value={currentPath}><Routes><Route path="/login" element={<Login/>}/><Route path="/studio" element={<Shell><Studio/></Shell>}/><Route path="/admin/*" element={<Shell><Admin/></Shell>}/><Route path="*" element={<Public/>}/></Routes></CurrentPath.Provider>}
