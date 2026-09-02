import {afterEach,describe,expect,it,vi} from 'vitest'
import {callActionBatch,encodeInput,runActionBatch} from './action-client'

afterEach(()=>vi.unstubAllGlobals())

describe('Action client',()=>{
  it('owns JSON and multipart encoding',()=>{
    expect(encodeInput({name:'Bean'})).toBe('{"name":"Bean"}')
    const multipart=encodeInput({name:'Bean',file:new File(['hello'],'hello.txt')})
    expect(multipart).toBeInstanceOf(FormData)
    expect((multipart as FormData).get('name')).toBe('Bean')
    expect((multipart as FormData).get('file')).toBeInstanceOf(File)
  })

	it('uses the bounded batch contract and preserves ordered partial results',async()=>{
		const fetchMock=vi.fn(async(input:string|URL|Request,init?:RequestInit)=>{void input;void init;return new Response(JSON.stringify({data:{results:[{id:'1',ok:true},{id:'2',ok:false,error:{code:'conflict',message:'invalid'}},{id:'3',ok:true}]}}),{status:200,headers:{'Content-Type':'application/json'}})})
		vi.stubGlobal('fetch',fetchMock)
		const result=await runActionBatch('move',['1','2','3'],{status:'done'})
		expect(result.results.map(item=>item.id)).toEqual(['1','2','3'])
		await expect(callActionBatch('move',['1','2','3'],{status:'done'})).rejects.toThrow('2: invalid')
		expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({ids:['1','2','3'],values:{status:'done'}})
	})
})
