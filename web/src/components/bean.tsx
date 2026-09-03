import type {ReactNode} from 'react'
import {Alert,AlertDescription} from '@/components/ui/alert'
import {Card,CardContent,CardDescription,CardHeader,CardTitle} from '@/components/ui/card'
import {Label} from '@/components/ui/label'
import {Skeleton} from '@/components/ui/skeleton'
import {cn} from '@/lib/utils'

export function Page({children,narrow=false,className}:{children:ReactNode;narrow?:boolean;className?:string}){
  return <main className={cn('mx-auto w-full max-w-6xl px-4 py-8 sm:px-6',narrow&&'max-w-md',className)}>{children}</main>
}

export function PageHeader({title,description,action}:{title:ReactNode;description?:ReactNode;action?:ReactNode}){
  return <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between"><div className="space-y-1"><h1 className="font-heading text-3xl font-semibold tracking-tight">{title}</h1>{description&&<p className="text-muted-foreground">{description}</p>}</div>{action}</div>
}

export function SectionCard({title,description,action,children,className}:{title?:ReactNode;description?:ReactNode;action?:ReactNode;children:ReactNode;className?:string}){
  return <Card className={cn('mb-6',className)}>{(title||description||action)&&<CardHeader><CardTitle>{title}</CardTitle>{description&&<CardDescription>{description}</CardDescription>}{action}</CardHeader>}<CardContent>{children}</CardContent></Card>
}

export function Field({id,label,children,hint,error,className,required=false}:{id?:string;label:ReactNode;children:ReactNode;hint?:ReactNode;error?:string;className?:string;required?:boolean}){
  return <div className={cn('grid gap-2',className)}><Label htmlFor={id}>{label}{required&&<span className="text-destructive" aria-hidden="true"> *</span>}</Label>{children}{hint&&<p className="text-xs text-muted-foreground">{hint}</p>}{error&&<p className="text-sm text-destructive" role="alert">{error}</p>}</div>
}

export function ErrorAlert({error}:{error:Error|string}){
  return <Alert variant="destructive" className="my-4"><AlertDescription>{typeof error==='string'?error:error.message}</AlertDescription></Alert>
}

export function StatusAlert({children}:{children:ReactNode}){
  return <Alert className="my-4" role="status"><AlertDescription>{children}</AlertDescription></Alert>
}

export function LoadingState({label='Loading…'}:{label?:string}){
  return <div className="space-y-3 py-4" aria-label={label}><span className="text-sm text-muted-foreground">{label}</span><Skeleton className="h-8 w-full"/><Skeleton className="h-8 w-4/5"/></div>
}
