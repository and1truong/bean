import {FormEvent,useState} from 'react'
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

function Login(){
  const nav=useNavigate();const loc=useLocation();const qc=useQueryClient()
  const[email,setEmail]=useState('');const[password,setPassword]=useState('');const[error,setError]=useState('')
  async function submit(event:FormEvent){
    event.preventDefault()
    try{
      const result=await api<{csrfToken:string;user:{Roles:string[]}}>('/api/auth/login',{method:'POST',body:JSON.stringify({email,password})})
      sessionStorage.setItem('bean_csrf',result.csrfToken)
      await qc.invalidateQueries({queryKey:['session']})
      const fallback=result.user.Roles.some(role=>role==='editor'||role==='administrator')?'/admin':'/'
      const requested=new URLSearchParams(loc.search).get('next')||fallback
      nav(requested.startsWith('/')&&!requested.startsWith('//')?requested:fallback)
    }catch(cause){setError((cause as Error).message)}
  }
  return <Shell><Page narrow><Card><CardHeader><CardTitle><h1 className="text-2xl">Sign in</h1></CardTitle><CardDescription>Access your Bean application.</CardDescription></CardHeader><CardContent><form className="space-y-4" onSubmit={submit}><Field id="login-email" label="Email"><Input id="login-email" data-testid="email" type="email" required value={email} onChange={event=>setEmail(event.target.value)}/></Field><Field id="login-password" label="Password"><Input id="login-password" data-testid="password" type="password" required value={password} onChange={event=>setPassword(event.target.value)}/></Field>{error&&<ErrorAlert error={error}/>}<Button className="w-full" data-testid="login" type="submit">Sign in</Button></form></CardContent></Card></Page></Shell>
}

function Shell({children}:{children:React.ReactNode}){
  const loc=useLocation();const nav=useNavigate();const qc=useQueryClient()
  const session=useQuery({queryKey:['session'],queryFn:()=>api<Session>('/api/system/session')})
  const roles=session.data?.user?.Roles||[]
  const editor=roles.includes('editor')||roles.includes('administrator')
  const administrator=roles.includes('administrator')
  const logout=useMutation({mutationFn:()=>api('/api/auth/logout',{method:'POST',body:'{}'}),onSuccess:async()=>{const page=qc.getQueryData<{tree:Node}>(['page',loc.pathname]);const protectedRoute=loc.pathname.startsWith('/admin')||loc.pathname.startsWith('/studio')||page?.tree.props?.protected===true;sessionStorage.removeItem('bean_csrf');await qc.resetQueries();if(protectedRoute)nav('/',{replace:true})}})
  return <><header className="border-b bg-primary text-primary-foreground"><div className="mx-auto flex w-full max-w-6xl flex-col gap-3 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6"><Link className="text-lg font-semibold tracking-tight" to="/">Bean</Link><nav className="flex flex-wrap items-center gap-1" aria-label="Primary navigation">{editor&&<Button variant="ghost" asChild><Link to="/admin">Admin</Link></Button>}{administrator&&<Button variant="ghost" asChild><Link to="/studio">Studio</Link></Button>}{session.data?.authenticated?<Button variant="ghost" onClick={()=>logout.mutate()} disabled={logout.isPending}>Sign out</Button>:<><Button variant="ghost" asChild><Link to="/signup">Sign up</Link></Button><Button variant="secondary" asChild><Link to={'/login?next='+encodeURIComponent(loc.pathname)}>Sign in</Link></Button></>}</nav></div></header>{children}</>
}

function Renderer({node}:{node:Node}){
  if(node.component==='TextBlock')return <p>{node.props?.text}</p>
  if(node.component==='ViewBlock'||node.component==='EntityBlock')return <ViewBlock name={node.props?.view||node.props?.entity+'_list'} block={node.props?.name} presentation={node.props?.presentation||{}} formattedFields={node.props?.formattedFields||[]}/>
  if(node.component==='ResourceListBlock')return <ResourceListBlock resource={node.props?.resource} view={node.props?.view} block={node.props?.name} filters={node.props?.filters} defaultFilters={node.props?.defaultFilters}/>
  if(node.component==='WebformBlock')return <WebformBlock name={node.props?.webform} block={node.props?.name}/>
  if(node.component==='ActionBlock')return <ActionBlock name={node.props?.action}/>
  if(node.component==='MenuBlock')return <MenuBlock items={node.props?.items||[]}/>
  return <section className="space-y-4" data-component={node.component}><h2 className="font-heading text-2xl font-semibold">{node.props?.title}</h2>{node.children?.map((child,index)=><Renderer key={index} node={child}/>)}</section>
}

function ViewBlock({name,block,presentation,formattedFields}:{name:string;block:string;presentation:ViewPresentation;formattedFields:string[]}){
  const loc=useLocation();const[cursors,setCursors]=useState<string[]>(['']);const cursor=cursors[cursors.length-1]
  const query=new URLSearchParams({_page:loc.pathname,_block:block});if(cursor)query.set('cursor',cursor)
  const request='/api/views/'+name+'?'+query.toString()
  const result=useQuery({queryKey:['public-view',request],queryFn:()=>api<{data:Row[];nextCursor:string}>(request)})
  if(result.isPending)return <LoadingState/>
  if(result.error)return <ErrorAlert error={result.error}/>
  if(!result.data.data.length)return <Card><CardContent className="py-8 text-center text-muted-foreground">{presentation.EmptyState||'Nothing to show.'}</CardContent></Card>
  const rows=presentation.Mode==='detail'?[mergeDetail(result.data.data,presentation.MetaFields||[])]:result.data.data
  return <div className="space-y-4">{rows.map(row=><Card key={String(row.id)+JSON.stringify(row)}><CardHeader><CardTitle><h3>{presentation.LinkRoute?<Link className="hover:underline" to={viewLink(presentation.LinkRoute,row)}>{row[presentation.TitleField||'title']}</Link>:row[presentation.TitleField||'title']||row.name}</h3></CardTitle>{presentation.MetaFields?.length?<CardDescription className="flex flex-wrap gap-2">{presentation.MetaFields.map(field=><span key={field}>{String(row[field]??'')}</span>)}</CardDescription>:null}</CardHeader><CardContent><ViewBody row={row} field={presentation.BodyField||'body'} rich={formattedFields.includes(presentation.BodyField||'body')}/></CardContent></Card>)}{presentation.Mode!=='detail'&&<Pagination previousDisabled={cursors.length===1} nextDisabled={!result.data.nextCursor} previous={()=>setCursors(value=>value.slice(0,-1))} next={()=>setCursors(value=>[...value,result.data.nextCursor])}/>}</div>
}

function mergeDetail(rows:Row[],meta:string[]){const result={...rows[0]};for(const field of meta){const values=[...new Set(rows.map(row=>row[field]).filter(value=>value!==null&&value!==undefined&&value!==''))];result[field]=values.join(', ')}return result}
function ViewBody({row,field,rich}:{row:Row;field:string;rich:boolean}){const selected=row[field];const value=String(selected??row.excerpt??row.description??'');return rich&&selected!==null&&selected!==undefined?<div className="rich-text" dangerouslySetInnerHTML={{__html:String(selected)}}/>:<p className="leading-7">{value}</p>}
function viewLink(template:string,row:Row){return template.replace(/:([a-zA-Z0-9_.]+)/g,(_,field)=>encodeURIComponent(String(row[field]??'')))}

function WebformBlock({name,block}:{name:string;block:string}){
  const loc=useLocation();const manifest=useQuery({queryKey:['manifest'],queryFn:()=>api<Manifest>('/api/system/manifest')});const form=manifest.data?.webforms?.[name]
  const[values,setValues]=useState<Row>({});const[step,setStep]=useState(0);const[done,setDone]=useState('')
  const query=new URLSearchParams({_page:loc.pathname,_block:block})
  const submit=useMutation({mutationFn:()=>api<{confirmation:string}>('/api/webforms/'+name+'/submit?'+query,{method:'POST',body:JSON.stringify(values)}),onSuccess:result=>setDone(result.confirmation)})
  if(!form)return null
  if(done)return <StatusAlert>{done}</StatusAlert>
  const names=form.Steps?.[step]
  const elements=(names?form.Elements.filter(element=>names.includes(element.Name)):form.Elements).filter(element=>evaluate(element.Visible,values))
  return <SectionCard><form className="space-y-4" onSubmit={event=>{event.preventDefault();if(form.Steps&&step<form.Steps.length-1)setStep(step+1);else submit.mutate()}}>{elements.map(element=><FormField key={element.Name} element={{...element,Required:element.Required||(element.RequiredWhen?evaluate(element.RequiredWhen,values):false)}} value={values[element.Name]} error={(submit.error as APIError|undefined)?.fields?.[element.Name]} onChange={value=>setValues(current=>({...current,[element.Name]:value}))}/>)}{submit.error&&<ErrorAlert error={submit.error}/>}<Button type="submit" disabled={submit.isPending}>{form.Steps&&step<form.Steps.length-1?'Next':submit.isPending?'Submitting…':'Submit'}</Button></form></SectionCard>
}

function evaluate(expression:import('./api').Expression|undefined|null,values:Row):boolean{
  if(!expression)return true;const args=expression.Args||[]
  if(expression.Op==='and')return args.every(item=>evaluate(item,values));if(expression.Op==='or')return args.some(item=>evaluate(item,values));if(expression.Op==='not')return !evaluate(args[0],values)
  const resolve=(value:typeof expression.Left)=>value?.Source==='input'?values[value.Name||'']:value?.Literal
  const left=resolve(expression.Left),right=resolve(expression.Right)
  if(expression.Op==='eq')return left===right;if(expression.Op==='ne')return left!==right;if(expression.Op==='is_null')return left==null;if(expression.Op==='is_not_null')return left!=null;return false
}

function FormField({element,value,error,onChange}:{element:FormElement;value:any;error?:string;onChange:(value:any)=>void}){
  if(element.Type==='group')return <SectionCard title={humanize(element.Name)}><div className="space-y-4"><Button type="button" variant="outline" onClick={()=>onChange([...(value||[]),{}])}>Add</Button>{(value||[]).map((row:Row,index:number)=><div className="grid gap-4 rounded-lg border p-4 sm:grid-cols-2" key={index}>{element.Children?.map(child=><FormField key={child.Name} element={child} value={row[child.Name]} onChange={next=>{const rows=[...value];rows[index]={...row,[child.Name]:next};onChange(rows)}}/>)}</div>)}</div></SectionCard>
  const id='form-'+element.Name;const label=element.Name.replaceAll('_',' ')
  if(element.Type==='textarea')return <Field id={id} label={label} error={error}><Textarea id={id} required={element.Required} value={value??''} onChange={event=>onChange(event.target.value)}/></Field>
  if(element.Type==='select'||element.Type==='entity reference')return <Field id={id} label={label} error={error}><NativeSelect id={id} required={element.Required} value={value??''} onChange={event=>onChange(event.target.value)}><NativeSelectOption value=""/>{element.Options?.map(option=><NativeSelectOption key={option}>{option}</NativeSelectOption>)}</NativeSelect></Field>
  if(element.Type==='checkbox')return <div className="grid gap-2"><div className="flex items-center gap-2"><Checkbox id={id} checked={Boolean(value)} onCheckedChange={checked=>onChange(Boolean(checked))}/><Label htmlFor={id}>{label}</Label></div>{error&&<p className="text-sm text-destructive" role="alert">{error}</p>}</div>
  const type=element.Type==='email'?'email':element.Type==='password'?'password':element.Type==='number'||element.Type==='integer'?'number':element.Type==='date'?'date':element.Type==='datetime'?'datetime-local':'text'
  return <Field id={id} label={label} error={error}><Input id={id} type={type} required={element.Required} value={value??''} onChange={event=>onChange(element.Type==='number'||element.Type==='integer'?Number(event.target.value):event.target.value)}/></Field>
}

function ActionBlock({name}:{name:string}){const mutation=useMutation({mutationFn:()=>api('/api/actions/'+name,{method:'POST',body:'{}'})});return <div className="space-y-3"><Button onClick={()=>mutation.mutate()} disabled={mutation.isPending}>{humanize(name)}</Button>{mutation.error&&<ErrorAlert error={mutation.error}/>}</div>}
function MenuBlock({items}:{items:Array<{Route:string;Label:string}>}){return <nav className="flex flex-wrap gap-2" aria-label="Page navigation">{items.map(item=><Button key={item.Route} variant="outline" asChild><Link to={item.Route}>{item.Label}</Link></Button>)}</nav>}
function Pagination({previousDisabled,nextDisabled,previous,next}:{previousDisabled:boolean;nextDisabled:boolean;previous:()=>void;next:()=>void}){return <nav className="flex justify-end gap-2" aria-label="Pagination"><Button variant="outline" disabled={previousDisabled} onClick={previous}>Previous</Button><Button variant="outline" disabled={nextDisabled} onClick={next}>Next</Button></nav>}
function humanize(value:string){return value.replaceAll('_',' ').replace(/^./,letter=>letter.toUpperCase())}

function Public(){
  const loc=useLocation();const result=useQuery({queryKey:['page',loc.pathname],queryFn:()=>api<{tree:Node}>('/api/system/page?path='+encodeURIComponent(loc.pathname))})
  if(result.isPending)return <Shell><Page><LoadingState/></Page></Shell>
  if(result.error)return <Shell><Page><PageHeader title="Bean" description="Metadata-driven applications, compiled."/></Page></Shell>
  return <Shell><Page className="space-y-6"><Renderer node={result.data.tree}/></Page></Shell>
}

export default function App(){return <Routes><Route path="/login" element={<Login/>}/><Route path="/studio" element={<Shell><Studio/></Shell>}/><Route path="/admin/*" element={<Shell><Admin/></Shell>}/><Route path="*" element={<Public/>}/></Routes>}
