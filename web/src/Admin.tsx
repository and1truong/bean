import {FormEvent,useState} from 'react'
import {Link,Route,Routes,useLocation,useNavigate,useParams} from 'react-router-dom'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {AdminManifest,AdminResource,api,Entity,Field as AdminFieldDefinition} from './api'
import {ErrorAlert,Field,LoadingState,Page,PageHeader,SectionCard,StatusAlert} from '@/components/bean'
import {AlertDialog,AlertDialogAction,AlertDialogCancel,AlertDialogContent,AlertDialogDescription,AlertDialogFooter,AlertDialogHeader,AlertDialogTitle,AlertDialogTrigger} from '@/components/ui/alert-dialog'
import {Badge} from '@/components/ui/badge'
import {Breadcrumb as BreadcrumbRoot,BreadcrumbItem,BreadcrumbLink,BreadcrumbList,BreadcrumbPage,BreadcrumbSeparator} from '@/components/ui/breadcrumb'
import {Button} from '@/components/ui/button'
import {Card,CardDescription,CardHeader,CardTitle} from '@/components/ui/card'
import {Checkbox} from '@/components/ui/checkbox'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'
import {Table,TableBody,TableCell,TableHead,TableHeader,TableRow} from '@/components/ui/table'
import {Textarea} from '@/components/ui/textarea'

type Row=Record<string,any>
type AuditEntry={id:string;at:string;user_id:string;action:string;changed_fields:string;success:number;error:string}

export function Admin(){
  const manifest=useQuery({queryKey:['admin-manifest'],queryFn:()=>api<AdminManifest>('/api/admin/manifest')})
  if(manifest.isPending)return <Page><LoadingState label="Loading administration…"/></Page>
  if(manifest.error)return <Page narrow><PageHeader title="Administration"/><ErrorAlert error={manifest.error}/><Button variant="outline" asChild><Link to="/login">Sign in</Link></Button></Page>
  return <Page><Routes><Route index element={<AdminHome manifest={manifest.data}/>}/><Route path="system" element={<SystemAdmin/>}/><Route path=":resource" element={<ResourceList manifest={manifest.data}/>}/><Route path=":resource/new" element={<ResourceRecord manifest={manifest.data} create/>}/><Route path=":resource/:id" element={<ResourceRecord manifest={manifest.data}/>}/></Routes></Page>
}

function AdminHome({manifest}:{manifest:AdminManifest}){
  const resources=Object.entries(manifest.adminResources).sort((a,b)=>a[1].Label.localeCompare(b[1].Label))
  return <><PageHeader title="Administration" description={<>Application data for release {manifest.version}.</>} action={<span className="max-w-72 truncate text-sm text-muted-foreground" title={manifest.releaseId}>{manifest.releaseId}</span>}/><div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">{manifest.systemAdmin&&<Link className="rounded-xl focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50" to="/admin/system"><Card className="h-full border-primary/30 transition-colors hover:bg-accent"><CardHeader><CardTitle>System</CardTitle><CardDescription>Users, releases, jobs, outbox, migrations, and audit.</CardDescription></CardHeader></Card></Link>}{resources.map(([name,resource])=><Link className="rounded-xl focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50" key={name} to={'/admin/'+name}><Card className="h-full transition-colors hover:bg-accent"><CardHeader><CardTitle>{resource.Label}</CardTitle><CardDescription>{resource.Description||'Manage '+resource.Label.toLowerCase()}</CardDescription></CardHeader></Card></Link>)}</div></>
}

type SystemUser={id:string;email:string;roles:string;tenant_id?:string;created_at:string}
type QueueRow={id:string;name?:string;topic?:string;status:string;attempts:number;max_attempts:number;next_attempt_at?:string;claimed_at?:string;last_error?:string}

function SystemAdmin(){
  const summary=useQuery({queryKey:['system-summary'],queryFn:()=>api<{releaseId:string;version:number;jobs:Record<string,number>;outbox:Record<string,number>}>('/api/admin/system/summary')})
  const users=useQuery({queryKey:['system-users'],queryFn:()=>api<SystemUser[]>('/api/admin/system/users')})
  const jobs=useQuery({queryKey:['system-jobs'],queryFn:()=>api<QueueRow[]>('/api/admin/system/jobs')})
  const outbox=useQuery({queryKey:['system-outbox'],queryFn:()=>api<QueueRow[]>('/api/admin/system/outbox')})
  const migrations=useQuery({queryKey:['system-migrations'],queryFn:()=>api<Row[]>('/api/admin/system/migrations')})
  const releases=useQuery({queryKey:['releases'],queryFn:()=>api<Row[]>('/api/admin/releases')})
  return <><AdminBreadcrumb current="System"/><PageHeader title="System" description="Operational state and recovery controls." action={<span className="max-w-72 truncate text-sm text-muted-foreground">{summary.data?.releaseId||'No active release'}</span>}/>{summary.error&&<ErrorAlert error={summary.error}/>}<SectionCard title="Queue health"><div className="grid gap-4 sm:grid-cols-2"><StatusGroup label="Jobs" counts={summary.data?.jobs}/><StatusGroup label="Outbox" counts={summary.data?.outbox}/></div></SectionCard><UserAdmin users={users.data||[]} loading={users.isPending} error={users.error}/><QueueAdmin label="Jobs" kind="jobs" rows={jobs.data||[]} loading={jobs.isPending} error={jobs.error}/><QueueAdmin label="Outbox" kind="outbox" rows={outbox.data||[]} loading={outbox.isPending} error={outbox.error}/><SystemTable title="Releases" rows={releases.data||[]} columns={['version','status','createdAt','activatedAt','id']}/><SystemTable title="Migrations" rows={migrations.data||[]} columns={['applied_at','release_id','sequence','description']}/></>
}

function StatusGroup({label,counts={}}:{label:string;counts?:Record<string,number>}){
  return <div className="rounded-lg border p-4"><strong>{label}</strong>{Object.keys(counts).length===0?<p className="mt-2 text-sm text-muted-foreground">No records</p>:<dl className="mt-3 flex flex-wrap gap-4">{Object.entries(counts).sort().map(([status,count])=><div className="grid gap-1" key={status}><dt className="text-xs text-muted-foreground">{humanize(status)}</dt><dd className="m-0 text-2xl font-semibold">{count}</dd></div>)}</dl>}</div>
}

function UserAdmin({users,loading,error}:{users:SystemUser[];loading:boolean;error:Error|null}){
  const qc=useQueryClient();const[email,setEmail]=useState('');const[password,setPassword]=useState('');const[roles,setRoles]=useState('authenticated');const[tenantId,setTenantId]=useState('')
  const create=useMutation({mutationFn:()=>api('/api/admin/system/users',{method:'POST',body:JSON.stringify({email,password,roles:csv(roles),tenantId})}),onSuccess:()=>{setEmail('');setPassword('');void qc.invalidateQueries({queryKey:['system-users']})}})
  return <SectionCard title="Users and roles"><form className="mb-6 grid items-end gap-4 md:grid-cols-2 xl:grid-cols-5" onSubmit={event=>{event.preventDefault();create.mutate()}}><Field id="system-user-email" label="Email"><Input id="system-user-email" type="email" required value={email} onChange={event=>setEmail(event.target.value)}/></Field><Field id="system-user-password" label="Temporary password"><Input id="system-user-password" type="password" required minLength={10} value={password} onChange={event=>setPassword(event.target.value)}/></Field><Field id="system-user-roles" label="Roles"><Input id="system-user-roles" value={roles} onChange={event=>setRoles(event.target.value)}/></Field><Field id="system-user-tenant" label="Tenant ID"><Input id="system-user-tenant" value={tenantId} onChange={event=>setTenantId(event.target.value)}/></Field><Button type="submit" disabled={create.isPending}>{create.isPending?'Creating…':'Create user'}</Button></form>{create.error&&<ErrorAlert error={create.error}/>} {loading?<LoadingState label="Loading users…"/>:error?<ErrorAlert error={error}/>:<Table><TableHeader><TableRow><TableHead>Email</TableHead><TableHead>Roles</TableHead><TableHead>Tenant</TableHead><TableHead>Created</TableHead></TableRow></TableHeader><TableBody>{users.map(user=><TableRow key={user.id}><TableCell>{user.email}</TableCell><TableCell><RoleEditor user={user}/></TableCell><TableCell>{display(user.tenant_id)}</TableCell><TableCell>{display(user.created_at)}</TableCell></TableRow>)}</TableBody></Table>}</SectionCard>
}

function RoleEditor({user}:{user:SystemUser}){
  const qc=useQueryClient();const[value,setValue]=useState(jsonList(user.roles))
  const update=useMutation({mutationFn:()=>api('/api/admin/system/users/'+user.id+'/roles',{method:'PUT',body:JSON.stringify({roles:csv(value),tenantId:user.tenant_id||''})}),onSuccess:()=>void qc.invalidateQueries({queryKey:['system-users']})})
  return <form className="flex min-w-72 items-center gap-2" onSubmit={event=>{event.preventDefault();update.mutate()}}><Input aria-label={'Roles for '+user.email} value={value} onChange={event=>setValue(event.target.value)}/><Button variant="secondary" type="submit" disabled={update.isPending}>Save</Button>{update.error&&<span className="text-sm text-destructive" role="alert">{update.error.message}</span>}</form>
}

function QueueAdmin({label,kind,rows,loading,error}:{label:string;kind:'jobs'|'outbox';rows:QueueRow[];loading:boolean;error:Error|null}){
  const qc=useQueryClient()
  const mutate=useMutation({mutationFn:({id,operation}:{id:string;operation:string})=>api('/api/admin/system/'+kind+'/'+id+'/'+operation,{method:'POST',body:'{}'}),onSuccess:()=>{void qc.invalidateQueries({queryKey:['system-'+kind]});void qc.invalidateQueries({queryKey:['system-summary']})}})
  return <SectionCard title={label}>{loading?<LoadingState label={'Loading '+label.toLowerCase()+'…'}/>:error?<ErrorAlert error={error}/>:rows.length===0?<p className="text-muted-foreground">No {label.toLowerCase()} records.</p>:<Table><TableHeader><TableRow><TableHead>{kind==='jobs'?'Name':'Topic'}</TableHead><TableHead>Status</TableHead><TableHead>Attempts</TableHead><TableHead>Next retry</TableHead><TableHead>Last error</TableHead><TableHead>Controls</TableHead></TableRow></TableHeader><TableBody>{rows.map(row=><TableRow key={row.id}><TableCell>{row.name||row.topic}</TableCell><TableCell><Badge variant="outline">{row.status}</Badge></TableCell><TableCell>{row.attempts}/{row.max_attempts}</TableCell><TableCell>{display(row.next_attempt_at)}</TableCell><TableCell className="max-w-80 truncate" title={row.last_error}>{display(row.last_error)}</TableCell><TableCell><div className="flex gap-2">{row.status==='failed'&&<QueueOperation operation="retry" row={row} run={()=>mutate.mutate({id:row.id,operation:'retry'})}/>} {(row.status==='pending'||row.status==='failed')&&<QueueOperation operation="cancel" row={row} destructive run={()=>mutate.mutate({id:row.id,operation:'cancel'})}/>}</div></TableCell></TableRow>)}</TableBody></Table>}{mutate.error&&<ErrorAlert error={mutate.error}/>}</SectionCard>
}

function QueueOperation({operation,row,run,destructive=false}:{operation:string;row:QueueRow;run:()=>void;destructive?:boolean}){
  const target=row.name||row.topic||row.id
  return <AlertDialog><AlertDialogTrigger asChild><Button size="sm" variant={destructive?'destructive':'outline'}>{humanize(operation)}</Button></AlertDialogTrigger><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>{humanize(operation)} {target}?</AlertDialogTitle><AlertDialogDescription>This operational change is audited and may affect delivery processing.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction variant={destructive?'destructive':'default'} onClick={run}>Confirm {operation}</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>
}

function SystemTable({title,rows,columns}:{title:string;rows:Row[];columns:string[]}){
  return <SectionCard title={title}>{rows.length===0?<p className="text-muted-foreground">No {title.toLowerCase()} recorded.</p>:<Table><TableHeader><TableRow>{columns.map(column=><TableHead key={column}>{humanize(column)}</TableHead>)}</TableRow></TableHeader><TableBody>{rows.map((row,index)=><TableRow key={String(row.id||row.release_id||index)}>{columns.map(column=><TableCell key={column}>{display(row[column])}</TableCell>)}</TableRow>)}</TableBody></Table>}</SectionCard>
}

function csv(value:string){return value.split(',').map(role=>role.trim()).filter(Boolean)}
type ResourceListScope={view:string;block:string;filters:string[];defaultFilters:Record<string,any>}

export function ResourceListBlock({resource,view,block,filters=[],defaultFilters={}}:{resource:string;view:string;block:string;filters?:string[];defaultFilters?:Record<string,any>}){
  const path=useLocation().pathname
  const manifest=useQuery({queryKey:['admin-manifest'],queryFn:()=>api<AdminManifest>('/api/admin/manifest')})
  if(manifest.isPending)return <LoadingState label="Loading resource list…"/>
  if(manifest.error)return <ErrorAlert error={manifest.error}/>
  return <ResourceList key={`${resource}:${block}:${path}`} manifest={manifest.data} resourceName={resource} scope={{view,block,filters,defaultFilters}}/>
}

function ResourceList({manifest,resourceName,scope}:{manifest:AdminManifest;resourceName?:string;scope?:ResourceListScope}){
  const route=useParams();const location=useLocation();const name=resourceName||route.resource||'';const resource=manifest.adminResources[name]
  const initialFilters=Object.fromEntries(Object.entries(scope?.defaultFilters||{}).map(([key,value])=>[key,String(value)]))
  const[query,setQuery]=useState('');const[submitted,setSubmitted]=useState('');const[filters,setFilters]=useState<Record<string,string>>(initialFilters);const[sort,setSort]=useState('');const[direction,setDirection]=useState<'asc'|'desc'>('asc');const[cursors,setCursors]=useState<string[]>(['']);const[selected,setSelected]=useState<string[]>([])
  const cursor=cursors[cursors.length-1];const filterFields=scope?.filters||resource?.List.Filters||[]
  const rows=useQuery({queryKey:['admin-resource',name,scope?.block,location.pathname,submitted,filters,sort,direction,cursor],enabled:Boolean(resource),queryFn:()=>{const queryString=scope?params({_page:location.pathname,_block:scope.block,cursor,limit:String(resource.List.PageSize),...filters}):params({q:submitted,cursor,sort,direction,...Object.fromEntries(Object.entries(filters).map(([key,value])=>['filter.'+key,value]))});return api<{data:Row[];nextCursor:string}>((scope?'/api/views/'+scope.view:'/api/admin/resources/'+name)+'?'+queryString)}})
  if(!resource)return <NotFound/>
  const entity=manifest.entities[resource.Entity]
  function order(field:string){if(sort===field)setDirection(value=>value==='asc'?'desc':'asc');else{setSort(field);setDirection('asc')}setCursors([''])}
  const linkField=resource.List.Columns.includes(resource.LabelField)?resource.LabelField:resource.List.Columns[0]
  const tableRows=rows.data?.data||[]
  return <>{!scope&&<AdminBreadcrumb resource={resource}/>}<PageHeader title={resource.Label} description={resource.Description} action={!scope&&<Button asChild><Link to={'/admin/'+name+'/new'}>Add {resource.Label}</Link></Button>}/><SectionCard><form className="mb-5 grid items-end gap-4 sm:grid-cols-2 lg:flex lg:flex-wrap" onSubmit={event=>{event.preventDefault();setSubmitted(query);setCursors([''])}}>{!scope&&<Field id="admin-search" label="Search" className="min-w-56 flex-1"><Input id="admin-search" aria-label="Search" type="search" value={query} onChange={event=>setQuery(event.target.value)} placeholder={resource.List.Search.length?'Search '+resource.List.Search.join(', '):'Search unavailable'} disabled={!resource.List.Search.length}/></Field>}{filterFields.map(fieldName=><FilterControl key={fieldName} field={findField(entity,fieldName)} value={filters[fieldName]||''} onChange={value=>{setFilters(current=>({...current,[fieldName]:value}));setCursors([''])}}/>)}<Button type="submit">Apply</Button></form>{selected.length>0&&resource.Actions.length>0&&<ActionRunner manifest={manifest} resource={resource} ids={selected} onDone={()=>{setSelected([]);void rows.refetch()}}/>}{rows.isPending?<LoadingState label="Loading records…"/>:rows.error?<ErrorAlert error={rows.error}/>:tableRows.length===0?<div className="py-10 text-center"><p className="text-muted-foreground">No records match this view.</p>{!scope&&<Button className="mt-4" variant="outline" asChild><Link to={'/admin/'+name+'/new'}>Create the first one</Link></Button>}</div>:<Table><TableHeader><TableRow><TableHead className="w-10"><Checkbox aria-label="Select all" checked={tableRows.length>0&&selected.length===tableRows.length} onCheckedChange={checked=>setSelected(checked?tableRows.map(row=>String(row.id)):[])}/></TableHead>{resource.List.Columns.map(field=><TableHead key={field}>{scope?fieldLabel(entity,field):<Button className="-ml-2" size="sm" variant="ghost" onClick={()=>order(field)}>{fieldLabel(entity,field)} {sort===field?(direction==='asc'?'↑':'↓'):''}</Button>}</TableHead>)}</TableRow></TableHeader><TableBody>{tableRows.map(row=><TableRow key={row.id} data-state={selected.includes(String(row.id))?'selected':undefined}><TableCell><Checkbox aria-label={'Select '+String(row[resource.LabelField]||row.id)} checked={selected.includes(String(row.id))} onCheckedChange={checked=>setSelected(current=>checked?[...current,String(row.id)]:current.filter(id=>id!==String(row.id)))}/></TableCell>{resource.List.Columns.map(field=><TableCell key={field}>{field===linkField?<Link className="font-medium text-primary hover:underline" to={'/admin/'+name+'/'+row.id}>{display(row[field])}</Link>:display(row[field])}</TableCell>)}</TableRow>)}</TableBody></Table>}<Pagination previousDisabled={cursors.length===1} nextDisabled={!rows.data?.nextCursor} previous={()=>setCursors(current=>current.slice(0,-1))} next={()=>setCursors(current=>[...current,rows.data?.nextCursor||''])}/></SectionCard></>
}

function ResourceRecord({manifest,create=false}:{manifest:AdminManifest;create?:boolean}){
  const{resource:name='',id=''}=useParams();const resource=manifest.adminResources[name]
  const record=useQuery({queryKey:['admin-record',name,id],enabled:Boolean(resource&&!create),queryFn:()=>api<{data:Row}>('/api/admin/resources/'+name+'/'+id)})
  if(!resource)return <NotFound/>
  if(!create&&record.isPending)return <LoadingState label="Loading record…"/>
  if(!create&&record.error)return <ErrorAlert error={record.error}/>
  return <RecordEditor key={create?'new':id} manifest={manifest} resource={resource} initial={create?{}:record.data?.data||{}} create={create}/>
}

function RecordEditor({manifest,resource,initial,create}:{manifest:AdminManifest;resource:AdminResource;initial:Row;create:boolean}){
  const entity=manifest.entities[resource.Entity];const nav=useNavigate();const qc=useQueryClient();const[values,setValues]=useState<Row>(initial);const[confirmDelete,setConfirmDelete]=useState(false)
  const save=useMutation({mutationFn:()=>{const action=create?resource.CreateAction:resource.UpdateAction;const body=create?pick(values,resource.Form.Fields):{id:initial.id,...changed(initial,values,resource.Form.Fields)};return api<{data:Row}>('/api/actions/'+action,{method:'POST',body:actionBody(body)})},onSuccess:result=>{qc.setQueryData(['admin-record',resource.Name,result.data.id],result);void qc.invalidateQueries({queryKey:['admin-resource',resource.Name]});nav(create?'/admin/'+resource.Name+'/'+result.data.id:'/admin/'+resource.Name)}})
  const remove=useMutation({mutationFn:()=>api('/api/actions/'+resource.DeleteAction,{method:'POST',body:JSON.stringify({id:initial.id})}),onSuccess:()=>{void qc.invalidateQueries({queryKey:['admin-resource',resource.Name]});nav('/admin/'+resource.Name)}})
  const protectedFields=new Set([...Object.values(manifest.lifecycles||{}).filter(lifecycle=>lifecycle.Entity===resource.Entity).map(lifecycle=>lifecycle.StateField),...Object.values(manifest.actions).filter(action=>action.Entity===resource.Entity&&action.Operation==='transition'&&!action.Lifecycle).map(action=>action.StateField||'status')])
  const title=create?'Add '+resource.Label:String(initial[resource.LabelField]||resource.Label)
  return <><AdminBreadcrumb resource={resource} current={create?'Add':String(initial[resource.LabelField]||initial.id)}/><PageHeader title={title} action={!create&&<AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}><AlertDialogTrigger asChild><Button variant="destructive">Delete</Button></AlertDialogTrigger><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Delete this {resource.Label.toLowerCase()}?</AlertDialogTitle><AlertDialogDescription>This operation cannot be undone.</AlertDialogDescription></AlertDialogHeader>{remove.error&&<ErrorAlert error={remove.error}/>}<AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction variant="destructive" onClick={event=>{event.preventDefault();remove.mutate()}}>Confirm delete</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog>}/><SectionCard><form className="space-y-4" onSubmit={event=>{event.preventDefault();save.mutate()}}>{resource.Form.Fields.map(name=>{const field=findField(entity,name);const readonly=!create&&protectedFields.has(name);return field?<AdminField key={name} field={field} value={readonly?initial[name]:values[name]} readonly={readonly} manifest={manifest} view={resource.View} onChange={value=>setValues(current=>({...current,[name]:value}))}/>:null})}{save.error&&<ErrorAlert error={save.error}/>}<div className="flex flex-wrap gap-2"><Button data-testid={create?'create-'+resource.Entity:'save-'+resource.Entity} type="submit" disabled={save.isPending}>{save.isPending?'Saving…':'Save'}</Button><Button variant="outline" asChild><Link to={'/admin/'+resource.Name}>Cancel</Link></Button></div></form>{!create&&<dl className="mt-6 grid gap-4 border-t pt-6 sm:grid-cols-2 lg:grid-cols-4">{resource.Form.Readonly.map(name=><div key={name}><dt className="font-medium">{fieldLabel(entity,name)}</dt><dd className="mt-1 text-sm text-muted-foreground">{display(initial[name])}</dd></div>)}</dl>}</SectionCard>{!create&&resource.Actions.length>0&&<SectionCard title="Actions"><ActionRunner manifest={manifest} resource={resource} ids={[String(initial.id)]} onDone={()=>{void qc.invalidateQueries({queryKey:['admin-record',resource.Name,initial.id]});void qc.invalidateQueries({queryKey:['admin-history',resource.Entity,initial.id]})}}/></SectionCard>}{!create&&<History entity={resource.Entity} id={String(initial.id)}/>}</>
}

function AdminField({field,value,onChange,manifest,view,readonly=false,idPrefix='admin'}:{field:AdminFieldDefinition;value:any;onChange:(value:any)=>void;manifest:AdminManifest;view?:string;readonly?:boolean;idPrefix?:string}){
  const id=idPrefix+'-'+field.Name;const label=field.Label||field.Name
  if(readonly)return <Field id={id} label={label}><Input id={id} value={display(value)} readOnly/></Field>
  if(field.Type==='relation')return <RelationControl field={field} value={value} onChange={onChange} manifest={manifest}/>
  if(field.Type==='enum')return <Field id={id} label={label}><NativeSelect id={id} data-testid={'field-'+field.Name} required={field.Required} value={value??''} onChange={event=>onChange(event.target.value)}><NativeSelectOption value="">Select…</NativeSelectOption>{field.Options?.map(option=><NativeSelectOption key={option}>{option}</NativeSelectOption>)}</NativeSelect></Field>
  if(field.Type==='boolean')return <div className="flex items-center gap-2"><Checkbox id={id} data-testid={'field-'+field.Name} checked={Boolean(value)} onCheckedChange={checked=>onChange(Boolean(checked))}/><Label htmlFor={id}>{label}</Label></div>
  if(field.Type==='text'||field.Type==='richtext')return <Field id={id} label={label}><Textarea id={id} data-testid={'field-'+field.Name} required={field.Required} value={value??''} onChange={event=>onChange(event.target.value)}/></Field>
  if(field.Type==='file')return <Field id={id} label={label}>{value&&view&&<a className="text-primary underline" href={'/api/files/'+encodeURIComponent(String(value))+'?view='+encodeURIComponent(view)}>Download current file</a>}<Input id={id} data-testid={'field-'+field.Name} type="file" required={field.Required&&!value} onChange={event=>onChange(event.target.files?.[0])}/></Field>
  const numeric=['integer','money','decimal'].includes(field.Type);const type=numeric?'number':field.Type==='email'?'email':field.Type==='date'?'date':field.Type==='datetime'?'datetime-local':'text'
  return <Field id={id} label={label}><Input id={id} data-testid={'field-'+field.Name} type={type} required={field.Required} value={value??''} onChange={event=>onChange(numeric?(event.target.value===''?'':Number(event.target.value)):event.target.value)}/></Field>
}

function RelationControl({field,value,onChange,manifest}:{field:AdminFieldDefinition;value:any;onChange:(value:any)=>void;manifest:AdminManifest}){
  const target=Object.values(manifest.adminResources).find(resource=>resource.Entity===field.Relation?.Entity)
  const options=useQuery({queryKey:['admin-relation',target?.Name],enabled:Boolean(target),queryFn:()=>api<{data:Row[]}>('/api/admin/resources/'+target?.Name+'?limit=200')})
  const multiple=field.Relation?.Kind==='one-to-many'||field.Relation?.Kind==='many-to-many';const selected=multiple?(Array.isArray(value)?value:[]):value??'';const id='admin-'+field.Name
  return <Field id={id} label={field.Label||field.Name} hint={!target?'No admin resource exists for the related entity.':undefined}><NativeSelect id={id} data-testid={'field-'+field.Name} multiple={multiple} required={field.Required} value={selected} onChange={event=>onChange(multiple?Array.from(event.target.selectedOptions,option=>option.value):event.target.value)}><NativeSelectOption value="">Select…</NativeSelectOption>{options.data?.data.map(row=><NativeSelectOption key={row.id} value={row.id}>{display(row[target?.LabelField||'id'])}</NativeSelectOption>)}</NativeSelect></Field>
}

function FilterControl({field,value,onChange}:{field?:AdminFieldDefinition;value:string;onChange:(value:string)=>void}){
  if(!field)return null;const id='filter-'+field.Name;const options=field.Type==='enum'?field.Options||[]:field.Type==='boolean'?['true','false']:null
  return <Field id={id} label={field.Label||field.Name} className="min-w-40">{options?<NativeSelect id={id} value={value} onChange={event=>onChange(event.target.value)}><NativeSelectOption value="">All</NativeSelectOption>{options.map(option=><NativeSelectOption key={option} value={option}>{field.Type==='boolean'?(option==='true'?'Yes':'No'):option}</NativeSelectOption>)}</NativeSelect>:<Input id={id} value={value} onChange={event=>onChange(event.target.value)}/>}</Field>
}

function ActionRunner({manifest,resource,ids,onDone}:{manifest:AdminManifest;resource:AdminResource;ids:string[];onDone:()=>void}){
  const[name,setName]=useState(resource.Actions[0]||'');const[values,setValues]=useState<Row>({});const[result,setResult]=useState('');const[confirmOpen,setConfirmOpen]=useState(false);const action=manifest.actions[name]
  const run=useMutation({mutationFn:async()=>{const failures:string[]=[];for(const id of ids){try{await api('/api/actions/'+name,{method:'POST',body:actionBody({...values,id})})}catch(error){failures.push(id+': '+(error as Error).message)}}if(failures.length)throw new Error(failures.join('; '))},onSuccess:()=>{setResult('Action completed for '+ids.length+' record'+(ids.length===1?'':'s')+'.');setConfirmOpen(false);onDone()}})
  if(!resource.Actions.length)return null
  function submit(event:FormEvent){event.preventDefault();if(action?.Confirm)setConfirmOpen(true);else run.mutate()}
  return <form className="space-y-4 rounded-lg bg-muted/60 p-4" onSubmit={submit}><div className="grid items-end gap-4 md:grid-cols-2"><Field id="admin-action" label="Action"><NativeSelect id="admin-action" aria-label="Action" value={name} onChange={event=>{setName(event.target.value);setValues({});setResult('')}}>{resource.Actions.map(actionName=><NativeSelectOption key={actionName} value={actionName}>{humanize(actionName)}</NativeSelectOption>)}</NativeSelect></Field>{action&&Object.entries(action.Input).filter(([input])=>input!=='id').map(([input,field])=><AdminField key={input} field={field} value={values[input]} manifest={manifest} idPrefix={'action-'+name} onChange={value=>setValues(current=>({...current,[input]:value}))}/>)}</div><Button type="submit" disabled={run.isPending}>{run.isPending?'Running…':'Run for '+ids.length}</Button>{run.error&&<ErrorAlert error={run.error}/>} {result&&<StatusAlert>{result}</StatusAlert>}<AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Confirm action</AlertDialogTitle><AlertDialogDescription>{action?.Confirm}</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction onClick={event=>{event.preventDefault();run.mutate()}}>Confirm action</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog></form>
}

function History({entity,id}:{entity:string;id:string}){
  const history=useQuery({queryKey:['admin-history',entity,id],queryFn:()=>api<AuditEntry[]>('/api/admin/audit?'+params({entity,id}))})
  return <SectionCard title="History">{history.isPending?<LoadingState label="Loading history…"/>:history.error?<ErrorAlert error={history.error}/>:history.data.length===0?<p className="text-muted-foreground">No changes recorded.</p>:<ol className="history space-y-4">{history.data.map(entry=><li className="grid gap-1 border-l-2 pl-4" key={entry.id}><strong>{humanize(entry.action)}</strong><span className="text-sm text-muted-foreground">{new Date(entry.at).toLocaleString()} · {entry.success?'Succeeded':'Failed'}</span>{entry.changed_fields&&<small className="text-muted-foreground">Fields: {jsonList(entry.changed_fields)}</small>}{entry.error&&<small className="text-destructive">{entry.error}</small>}</li>)}</ol>}</SectionCard>
}

function AdminBreadcrumb({resource,current}:{resource?:AdminResource;current?:string}){
  return <BreadcrumbRoot className="mb-5"><BreadcrumbList><BreadcrumbItem>{resource||current?<BreadcrumbLink asChild><Link to="/admin">Administration</Link></BreadcrumbLink>:<BreadcrumbPage>Administration</BreadcrumbPage>}</BreadcrumbItem>{resource&&<><BreadcrumbSeparator/><BreadcrumbItem>{current?<BreadcrumbLink asChild><Link to={'/admin/'+resource.Name}>{resource.Label}</Link></BreadcrumbLink>:<BreadcrumbPage>{resource.Label}</BreadcrumbPage>}</BreadcrumbItem></>}{current&&<><BreadcrumbSeparator/><BreadcrumbItem><BreadcrumbPage>{current}</BreadcrumbPage></BreadcrumbItem></>}</BreadcrumbList></BreadcrumbRoot>
}
function NotFound(){return <><PageHeader title="Admin resource not found"/><Button variant="outline" asChild><Link to="/admin">Back to administration</Link></Button></>}
function Pagination({previousDisabled,nextDisabled,previous,next}:{previousDisabled:boolean;nextDisabled:boolean;previous:()=>void;next:()=>void}){return <nav className="mt-5 flex justify-end gap-2" aria-label="Pagination"><Button variant="outline" disabled={previousDisabled} onClick={previous}>Previous</Button><Button variant="outline" disabled={nextDisabled} onClick={next}>Next</Button></nav>}
function findField(entity:Entity,name:string){return entity.Fields.find(field=>field.Name===name)}
function fieldLabel(entity:Entity,name:string){if(name==='id')return 'ID';return findField(entity,name)?.Label||humanize(name)}
function humanize(value:string){return value.replaceAll('_',' ').replace(/^./,letter=>letter.toUpperCase())}
function display(value:any){if(value===null||value===undefined||value==='')return '—';if(typeof value==='boolean')return value?'Yes':'No';if(Array.isArray(value))return value.join(', ');return String(value)}
function params(values:Record<string,string>){const query=new URLSearchParams();for(const[key,value]of Object.entries(values))if(value)query.set(key,value);return query.toString()}
function actionBody(values:Row):BodyInit{if(!Object.values(values).some(value=>value instanceof File))return JSON.stringify(values);const body=new FormData();for(const[name,value]of Object.entries(values)){if(value instanceof File)body.append(name,value);else if(value!==undefined&&value!==null)body.append(name,typeof value==='string'?value:JSON.stringify(value))}return body}
function pick(values:Row,fields:string[]){return Object.fromEntries(fields.filter(field=>values[field]!==undefined&&values[field]!=='').map(field=>[field,values[field]]))}
function changed(before:Row,after:Row,fields:string[]){return Object.fromEntries(fields.filter(field=>after[field]!==before[field]).map(field=>[field,after[field]]))}
function jsonList(value:string){try{return(JSON.parse(value)as string[]).join(', ')}catch{return value}}
