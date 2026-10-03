import {describe,expect,it,vi} from 'vitest'
import {exposeAgentTools,type AgentState,type AgentTools} from './mcp'

const response={v:1,ok:true,diagnostics:[],app:{name:'demo',routes:[]}}

function handlers():AgentTools{
  return {
    state:vi.fn(()=>({manifest:'app.yaml',files:{'app.yaml':{bytes:10}},app:null,diagnostics:[],unsupported:[],route:'/'})as AgentState),
    compile:vi.fn(async()=>response),
    render:vi.fn(async()=>response),
    navigate:vi.fn((input:{path:string})=>({ok:true,path:input.path})),
    listExamples:vi.fn(()=>[{name:'presentation'}]),
    loadExample:vi.fn(async()=>({load:response})),
  }
}

type BeanWindow=Window&{bean?:Record<string,unknown>}

describe('exposeAgentTools',()=>{
  it('publishes window.bean with snake-name tools and camelCase methods',()=>{
    const bag=handlers()
    const teardown=exposeAgentTools(bag)
    const bean=(window as BeanWindow).bean!
    expect(bean.v).toBe(1)
    expect(bean.webmcp).toBe(false)
    expect(bean.tools).toEqual(['bean_state','bean_compile','bean_render','bean_navigate','bean_list_examples','bean_load_example'])
    expect(bean.bean_state).toBeTypeOf('function')
    expect(bean.compile).toBe(bean.bean_compile)
    expect((bean.state as AgentTools['state'])()).toMatchObject({manifest:'app.yaml'})
    teardown()
    expect((window as BeanWindow).bean).toBeUndefined()
  })

  it('passes tool input through to the handlers',async()=>{
    const bag=handlers()
    const teardown=exposeAgentTools(bag)
    const bean=(window as BeanWindow).bean as Record<string,(i?:Record<string,unknown>)=>unknown>
    await bean.bean_compile({files:{'app.yaml':'x'},manifest:'app.yaml'})
    expect(bag.compile).toHaveBeenCalledWith({files:{'app.yaml':'x'},manifest:'app.yaml'})
    await bean.bean_render({path:'/one'})
    expect(bag.render).toHaveBeenCalledWith({path:'/one'})
    bean.bean_navigate({path:'/two'})
    expect(bag.navigate).toHaveBeenCalledWith({path:'/two'})
    teardown()
  })

  it('registers WebMCP tools when document.modelContext exists',async()=>{
    const registered:{name:string;inputSchema:Record<string,unknown>;execute:(i?:Record<string,unknown>)=>unknown}[]=[]
    const fake={registerTool:vi.fn((tool:never)=>{registered.push(tool);return Promise.resolve()})}
    Object.defineProperty(document,'modelContext',{value:fake,configurable:true})
    const bag=handlers()
    const teardown=exposeAgentTools(bag)
    expect((window as BeanWindow).bean).toMatchObject({webmcp:true})
    expect(fake.registerTool).toHaveBeenCalledTimes(6)
    expect(registered.map(tool=>tool.name)).toEqual(['bean_state','bean_compile','bean_render','bean_navigate','bean_list_examples','bean_load_example'])
    const compile=registered.find(tool=>tool.name==='bean_compile')!
    expect(compile.inputSchema).toMatchObject({type:'object'})
    await compile.execute({files:{},manifest:'app.yaml'})
    expect(bag.compile).toHaveBeenCalled()
    teardown()
    delete (document as Document&{modelContext?:unknown}).modelContext
  })
})
