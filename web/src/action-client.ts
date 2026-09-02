import {api,APIError} from './api'

type Input=Record<string,any>

export function encodeInput(values:Input,forceMultipart=false):BodyInit{
  const multipart=forceMultipart||Object.values(values).some(value=>value instanceof File)
  if(!multipart)return JSON.stringify(values)
  const body=new FormData()
  for(const[name,value]of Object.entries(values)){
    if(value instanceof File)body.append(name,value)
    else if(value!==undefined&&value!==null)body.append(name,typeof value==='string'?value:JSON.stringify(value))
  }
  return body
}

export function callAction<T=unknown>(name:string,input:Input):Promise<T>{
  return api<T>('/api/actions/'+name,{method:'POST',body:encodeInput(input)})
}

export type ActionBatchItem={id:string;ok:boolean;data?:unknown;error?:{code:string;message:string}}
export type ActionBatchResult={results:ActionBatchItem[]}
type Failure={id:string;error:Error}

export class ActionBatchError extends APIError{
  failures:Failure[]
  constructor(failures:Failure[]){
    super(failures.map(({id,error})=>id+': '+error.message).join('; '),mergeFields(failures))
    this.failures=failures
  }
}

export async function runActionBatch(name:string,ids:string[],values:Input):Promise<ActionBatchResult>{
	if(ids.length<1||ids.length>200)throw new APIError('Action batch must contain between 1 and 200 record IDs.')
	const response=await api<{data:ActionBatchResult}>('/api/actions/'+name+'/batch',{method:'POST',body:JSON.stringify({ids,values})})
	return response.data
}

export async function callActionBatch(name:string,ids:string[],values:Input):Promise<void>{
	const result=await runActionBatch(name,ids,values)
	const failures:Failure[]=result.results.filter(item=>!item.ok).map(item=>({id:item.id,error:new APIError(item.error?.message||'Action failed.')}))
	if(failures.length)throw new ActionBatchError(failures)
}

function mergeFields(failures:Failure[]):Record<string,string>|undefined{
  const fields:Record<string,string>={}
  for(const{id,error}of failures){
    if(!(error instanceof APIError)||!error.fields)continue
    for(const[name,message]of Object.entries(error.fields))fields[name]=failures.length===1?message:id+': '+message
  }
  return Object.keys(fields).length?fields:undefined
}
