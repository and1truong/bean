import type {Node} from '../api'

// Bridge protocol v1 mirrors internal/playground.

export type BridgeDiagnostic={
  code?:string
  kind?:string
  name?:string
  path?:string
  message:string
  source?:{path?:string;line?:number;column?:number}
  related?:{path?:string;line?:number;column?:number}
  candidates?:string[]
}
export type BridgeFailure={code:string;message:string}
export type BridgeRoute={path:string;kind:string;title?:string;protected?:boolean;unsupported?:string}
export type BridgeUnsupported={component:string;reason:string;count:number}
export type BridgeApp={name:string;title?:string;theme?:{name?:string;displayName?:string;preset?:string;accent?:string};routes:BridgeRoute[];unsupported?:BridgeUnsupported[]}
export type BridgeResponse={
  v:number
  ok:boolean
  diagnostics?:BridgeDiagnostic[]
  app?:BridgeApp
  tree?:Node
  unsupported?:BridgeUnsupported[]
  error?:BridgeFailure
}
export type BridgeRequest={v:1;op:'compile'|'render';files?:Record<string,string>;manifest?:string;path?:string;query?:Record<string,string>}

const REQUEST_TIMEOUT_MS=60_000

// Bridge owns one compile/render worker. Requests are answered in order; the
// caller stamps generations so stale results can be dropped.
export class Bridge{
  private worker:Worker|null=null
  private nextID=0
  private pending=new Map<number,{resolve:(response:BridgeResponse)=>void;timer:number}>()
  private readyWaiter:{resolve:()=>void;reject:(error:Error)=>void}|null=null
  private ready=false
  private dead=''
  onFatal:((error:string)=>void)|null=null

  start(){
    this.dispose()
    this.dead=''
    this.ready=false
    this.worker=new Worker(new URL('worker.js',document.baseURI))
    this.worker.onmessage=event=>this.onMessage(event.data)
    this.worker.onerror=()=>this.fail('the compiler worker failed to start')
    return new Promise<void>((resolve,reject)=>{this.readyWaiter={resolve,reject}})
  }

  dispose(){
    this.worker?.terminate()
    this.worker=null
    for(const[,pending]of this.pending){clearTimeout(pending.timer);pending.resolve({v:1,ok:false,error:{code:'BEAN-P4205',message:'the compiler worker was restarted'}})}
    this.pending.clear()
  }

  isReady(){return this.ready&&!this.dead}
  fatalError(){return this.dead}

  call(request:BridgeRequest):Promise<BridgeResponse>{
    if(this.dead)return Promise.resolve({v:1,ok:false,error:{code:'BEAN-P4205',message:this.dead}})
    if(!this.worker)return Promise.resolve({v:1,ok:false,error:{code:'BEAN-P4205',message:'the compiler worker is not running'}})
    const id=this.nextID++
    return new Promise<BridgeResponse>(resolve=>{
      const timer=window.setTimeout(()=>{
        this.pending.delete(id)
        resolve({v:1,ok:false,error:{code:'BEAN-P4205',message:'the compiler did not answer within 60 seconds'}})
      },REQUEST_TIMEOUT_MS)
      this.pending.set(id,{resolve,timer})
      this.worker!.postMessage({id,request})
    })
  }

  private onMessage(message:{type:string;id?:number;response?:BridgeResponse;error?:string}){
    if(message.type==='ready'){
      this.ready=true
      this.readyWaiter?.resolve()
      this.readyWaiter=null
      return
    }
    if(message.type==='fatal'){
      this.fail(message.error||'the compiler failed')
      return
    }
    if(message.type==='result'&&typeof message.id==='number'){
      const pending=this.pending.get(message.id)
      if(!pending)return
      this.pending.delete(message.id)
      clearTimeout(pending.timer)
      pending.resolve(message.response||{v:1,ok:false,error:{code:'BEAN-P4100',message:'the compiler sent an empty response'}})
    }
  }

  private fail(error:string){
    this.dead=error
    this.onFatal?.(error)
    this.readyWaiter?.reject(new Error(error))
    this.readyWaiter=null
    for(const[,pending]of this.pending){clearTimeout(pending.timer);pending.resolve({v:1,ok:false,error:{code:'BEAN-P4205',message:error}})}
    this.pending.clear()
  }
}
