import {create} from 'zustand'
type Editor={kind:string;name:string;spec:string;set:(patch:Partial<Pick<Editor,'kind'|'name'|'spec'>>)=>void;reset:()=>void}
export const useEditor=create<Editor>(set=>({kind:'Entity',name:'',spec:'{}',set:patch=>set(patch),reset:()=>set({kind:'Entity',name:'',spec:'{}'})}))
