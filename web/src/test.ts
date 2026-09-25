import '@testing-library/jest-dom'

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

globalThis.ResizeObserver=TestResizeObserver

// Node 25 ships an experimental webstorage global that shadows jsdom's
// localStorage with an object missing getItem — replace it with a stub.
if(typeof globalThis.localStorage!=='object'||typeof globalThis.localStorage?.getItem!=='function'){
  const store=new Map<string,string>()
  Object.defineProperty(globalThis,'localStorage',{configurable:true,writable:true,value:{
    getItem:(key:string)=>store.get(key)??null,
    setItem:(key:string,value:string)=>void store.set(key,String(value)),
    removeItem:(key:string)=>void store.delete(key),
    clear:()=>store.clear(),
    key:(index:number)=>[...store.keys()][index]??null,
    get length(){return store.size},
  }})
}
