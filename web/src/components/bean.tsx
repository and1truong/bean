import {Children,cloneElement,isValidElement,type FormEventHandler,type ReactElement,type ReactNode} from 'react'
import {XIcon} from 'lucide-react'
import {Alert,AlertDescription} from '@/components/ui/alert'
import {Button} from '@/components/ui/button'
import {Label} from '@/components/ui/label'
import {Skeleton} from '@/components/ui/skeleton'
import {cn} from '@/lib/utils'

export function Page({children,narrow=false,className}:{children:ReactNode;narrow?:boolean;className?:string}){
  return <main className={cn('bean-page-root',narrow&&'max-w-md',className)}>{children}</main>
}

export function PageHeader({title,description,action,context}:{title:ReactNode;description?:ReactNode;action?:ReactNode;context?:ReactNode}){
  return <header className="bean-page-header"><div className="min-w-0">{context&&<div className="mb-2">{context}</div>}<h1 className="bean-page-title">{title}</h1>{description&&<p className="bean-page-description">{description}</p>}</div>{action&&<div className="flex shrink-0 flex-wrap items-center justify-end gap-2">{action}</div>}</header>
}

export function SectionCard({title,description,action,children,className}:{title?:ReactNode;description?:ReactNode;action?:ReactNode;children:ReactNode;className?:string}){
  return <section data-slot="card" className={cn('bean-section',className)}>{(title||description||action)&&<header className="bean-section-header"><div className="flex min-w-0 items-start justify-between gap-3"><div className="min-w-0">{title&&<h2 className="bean-section-heading">{title}</h2>}{description&&<p className="mt-0.5 text-sm text-text-secondary">{description}</p>}</div>{action}</div></header>}<div className="bean-section-content">{children}</div></section>
}

type DescribedControlProps={'aria-describedby'?:string;'aria-invalid'?:boolean}
export function Field({id,label,children,hint,error,className,required=false}:{id?:string;label:ReactNode;children:ReactNode;hint?:ReactNode;error?:string;className?:string;required?:boolean}){
  const hintID=id&&hint?`${id}-description`:undefined
  const errorID=id&&error?`${id}-error`:undefined
  const describedBy=[hintID,errorID].filter(Boolean).join(' ')||undefined
  const child=Children.count(children)===1&&isValidElement(children)
    ?cloneElement(children as ReactElement<DescribedControlProps>,{'aria-describedby':describedBy,'aria-invalid':error?true:undefined})
    :children
  return <div className={cn('bean-field',className)}><Label htmlFor={id}>{label}{required&&<span className="text-destructive" aria-hidden="true"> *</span>}</Label>{child}{hint&&<p className="bean-field-description" id={hintID}>{hint}</p>}{error&&<p className="bean-validation" id={errorID} role="alert">{error}</p>}</div>
}

export function Toolbar({children,label='Page tools',className}:{children:ReactNode;label?:string;className?:string}){
  return <div className={cn('bean-toolbar',className)} role="toolbar" aria-label={label}>{children}</div>
}

export function FilterBar({children,onSubmit,label='Filters',className}:{children:ReactNode;onSubmit:FormEventHandler<HTMLFormElement>;label?:string;className?:string}){
  return <form className={cn('bean-toolbar',className)} aria-label={label} onSubmit={onSubmit}>{children}</form>
}

export type ActiveFilter={key:string;label:string;operator?:string;value:string}
export function ActiveFilters({filters,onRemove,onClear}:{filters:ActiveFilter[];onRemove:(key:string)=>void;onClear:()=>void}){
  if(!filters.length)return null
  return <div className="bean-filter-summary" aria-label="Active filters"><span className="bean-metadata">{filters.length} active</span>{filters.map(filter=><Button className="bean-filter-chip" key={filter.key} type="button" size="xs" variant="outline" onClick={()=>onRemove(filter.key)} aria-label={`Remove ${filter.value} filter`}><strong>{filter.label}</strong><span>{filter.operator||'is'} {filter.value}</span><XIcon className="size-3" aria-hidden="true"/></Button>)}<Button type="button" variant="ghost" size="xs" onClick={onClear}>Clear all</Button></div>
}

export function DataTable({children,count,label,selected=0,className}:{children:ReactNode;count?:number;label?:string;selected?:number;className?:string}){
  return <section className={cn('bean-data-table-shell',className)} aria-label={label}>{(count!==undefined||selected>0)&&<div className="bean-data-table-meta"><span>{count===undefined?'Results':`${count} ${count===1?'result':'results'}`}</span>{selected>0&&<strong aria-live="polite">{selected} selected</strong>}</div>}{children}</section>
}

export function EmptyState({title,description,action}:{title:string;description?:ReactNode;action?:ReactNode}){
  return <div className="bean-empty-state"><div><p className="bean-empty-title">{title}</p>{description&&<p className="bean-empty-description">{description}</p>}{action&&<div className="mt-3 flex justify-center">{action}</div>}</div></div>
}

export function Divider(){return <hr className="my-4 border-0 border-t border-border-subtle"/>}

export function Spinner({label='Loading'}:{label?:string}){
  return <span className="inline-flex items-center gap-2" role="status"><span className="size-3.5 animate-spin rounded-full border-2 border-border-strong border-t-primary" aria-hidden="true"/><span className="sr-only">{label}</span></span>
}

export function StatusIndicator({status,label}:{status:'success'|'warning'|'danger'|'info'|'neutral';label:string}){
  const colors={success:'bg-success',warning:'bg-warning',danger:'bg-destructive',info:'bg-info',neutral:'bg-text-muted'}
  return <span className="inline-flex items-center gap-1.5 text-xs text-text-secondary"><span className={cn('size-1.5 rounded-full',colors[status])} aria-hidden="true"/>{label}</span>
}

export function ErrorAlert({error}:{error:Error|string}){
  return <Alert variant="destructive" className="my-4"><AlertDescription>{typeof error==='string'?error:error.message}</AlertDescription></Alert>
}

export function StatusAlert({children}:{children:ReactNode}){
  return <Alert className="my-4" role="status"><AlertDescription>{children}</AlertDescription></Alert>
}

export function LoadingState({label='Loading…'}:{label?:string}){
  return <div className="space-y-2 py-4" role="status" aria-label={label}><span className="text-xs text-muted-foreground">{label}</span><Skeleton className="h-8 w-full"/><Skeleton className="h-8 w-4/5"/></div>
}
