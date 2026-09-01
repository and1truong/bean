import {afterEach,describe,expect,it,vi} from 'vitest'
import {callActionBatch,encodeInput} from './action-client'

afterEach(()=>vi.unstubAllGlobals())

describe('Action client',()=>{
  it('owns JSON and multipart encoding',()=>{
    expect(encodeInput({name:'Bean'})).toBe('{"name":"Bean"}')
    const multipart=encodeInput({name:'Bean',file:new File(['hello'],'hello.txt')})
    expect(multipart).toBeInstanceOf(FormData)
    expect((multipart as FormData).get('name')).toBe('Bean')
    expect((multipart as FormData).get('file')).toBeInstanceOf(File)
  })

  it('runs batches sequentially and aggregates field failures',async()=>{
    const active:number[]=[];let concurrent=0;let maximum=0
    vi.stubGlobal('fetch',vi.fn(async(_input:string|URL|Request,init?:RequestInit)=>{
      const id=JSON.parse(String(init?.body)).id as string
      concurrent++;maximum=Math.max(maximum,concurrent);active.push(Number(id))
      await Promise.resolve();concurrent--
      if(id==='2')return new Response(JSON.stringify({error:{message:'invalid',fields:{status:'is invalid'}}}),{status:400,headers:{'Content-Type':'application/json'}})
      return new Response(JSON.stringify({data:{id}}),{status:200,headers:{'Content-Type':'application/json'}})
    }))
    await expect(callActionBatch('move',['1','2','3'],{})).rejects.toMatchObject({fields:{status:'is invalid'}})
    expect(active).toEqual([1,2,3])
    expect(maximum).toBe(1)
  })
})
