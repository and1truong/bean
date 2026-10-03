// Agent surface for the playground. The page registers WebMCP tools when the
// runtime provides document.modelContext (navigator.modelContext is the older,
// deprecated spell) and always exposes the same handlers as window.bean so any
// agent that can evaluate JS — bookmarklets, DevTools, MCP-driven browser
// automation — can compose Bean apps on a purely static page.

import type {BridgeApp,BridgeDiagnostic,BridgeResponse,BridgeUnsupported} from './bridge'

export interface AgentState{
  manifest:string
  files:Record<string,{bytes:number}>
  app:BridgeApp|null
  diagnostics:BridgeDiagnostic[]
  unsupported:BridgeUnsupported[]
  route:string
}
export interface AgentTools{
  state:()=>AgentState
  compile:(input?:{files?:Record<string,string>;manifest?:string})=>Promise<BridgeResponse>
  render:(input:{path:string;query?:Record<string,string>})=>Promise<BridgeResponse>
  navigate:(input:{path:string})=>{ok:boolean;path:string}
  listExamples:()=>{name:string;title?:string}[]
  loadExample:(input:{name:string})=>Promise<unknown>
}

interface WebMCPTool{
  name:string
  description:string
  inputSchema:Record<string,unknown>
  execute:(input?:Record<string,unknown>)=>unknown
}
interface ModelContextLike{
  registerTool:(tool:WebMCPTool,options?:{signal?:AbortSignal})=>unknown
}
interface AgentWindow{bean?:unknown}

const TOOL_DESCRIPTIONS:Record<keyof AgentTools,{name:string;description:string;inputSchema:Record<string,unknown>}>={
  state:{name:'bean_state',description:'Read the current playground state: manifest path, source file list, compiled app summary (routes), diagnostics, and backend-dependent components.',inputSchema:{type:'object',properties:{}}},
  compile:{name:'bean_compile',description:'Compile Bean YAML/JSON sources with the real Go compiler (WASM). Pass files+manifest to replace the editor sources and compose a new app; omit them to recompile the current sources. Returns diagnostics and the app summary; the preview updates only on a zero-diagnostic compile.',inputSchema:{type:'object',properties:{files:{type:'object',description:'Map of relative .yaml/.yml/.json paths (≤64 files, ≤256 KiB each, ≤1 MiB total) to source text.'},manifest:{type:'string',description:'Entry manifest path, e.g. "app.yaml".'}},additionalProperties:false}},
  render:{name:'bean_render',description:'Render one app route to the Bean semantic tree. Backend-dependent components come back as UnsupportedBlock entries, never simulated.',inputSchema:{type:'object',properties:{path:{type:'string',description:'Route path from the compiled app summary, e.g. "/".'},query:{type:'object',description:'Optional query map, e.g. {frame:"3"} for sequences.'}},required:['path'],additionalProperties:false}},
  navigate:{name:'bean_navigate',description:'Switch the on-screen preview to a route from the compiled app summary.',inputSchema:{type:'object',properties:{path:{type:'string'}},required:['path'],additionalProperties:false}},
  listExamples:{name:'bean_list_examples',description:'List the checked-in example Bean apps bundled with this page.',inputSchema:{type:'object',properties:{}}},
  loadExample:{name:'bean_load_example',description:'Load a bundled example into the editor and compile it. Returns the compile result.',inputSchema:{type:'object',properties:{name:{type:'string'}},required:['name'],additionalProperties:false}},
}

// Registers every handler as a WebMCP tool when the runtime supports it, and
// always assigns them to window.bean (snake-name tools plus camelCase methods).
// Returns a teardown that removes the window binding and unregisters tools.
export function exposeAgentTools(handlers:AgentTools):()=>void{
  const controller=new AbortController()
  const entries=Object.entries(TOOL_DESCRIPTIONS)as[keyof AgentTools,(typeof TOOL_DESCRIPTIONS)[keyof AgentTools]][]
  const bag=handlers as unknown as Record<string,(input?:Record<string,unknown>)=>unknown>
  const modelContext=((document as unknown as{modelContext?:ModelContextLike}).modelContext)||((navigator as unknown as{modelContext?:ModelContextLike}).modelContext)
  const webmcp=!!modelContext&&typeof modelContext.registerTool==='function'
  if(webmcp){
    for(const[key,spec]of entries){
      void Promise.resolve(modelContext.registerTool({...spec,execute:input=>bag[key](input)},{signal:controller.signal})).catch(()=>{})
    }
  }
  const bean:Record<string,unknown>={v:1,webmcp,tools:entries.map(([,spec])=>spec.name)}
  for(const[key,spec]of entries){
    const fn=(input?:Record<string,unknown>)=>bag[key](input)
    bean[spec.name]=fn
    bean[key]=fn
  }
  const w=window as unknown as AgentWindow
  w.bean=bean
  return ()=>{controller.abort();if(w.bean===bean)delete w.bean}
}
