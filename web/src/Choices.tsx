import {useEffect,useId,useRef,useState} from 'react'
import {ContentChoice} from './api'
import {useContentActive} from './ContentVisibility'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'

export function Choices({question,choices,answer,explanation=''}:{question:string;choices:ContentChoice[];answer:string;explanation?:string}){
  const active=useContentActive();const namespace=useId().replaceAll(':','');const first=useRef<HTMLInputElement>(null)
  const[selected,setSelected]=useState('');const[checked,setChecked]=useState(false)
  const identity=JSON.stringify([question,choices,answer,explanation])
  useEffect(()=>{setSelected('');setChecked(false)},[identity])
  useEffect(()=>{if(!active){setSelected('');setChecked(false)}},[active])
  const retry=()=>{setSelected('');setChecked(false);requestAnimationFrame(()=>first.current?.focus())}
  return <fieldset className="bean-choices">
    <legend>{question}</legend>
    {choices.map((choice,index)=>{const id=`bean-choice-${namespace}-${choice.id}`;return <label key={choice.id} htmlFor={id}><Input className="size-4 w-4 shrink-0 p-0" ref={index===0?first:undefined} id={id} name={`bean-choices-${namespace}`} type="radio" value={choice.id} checked={selected===choice.id} disabled={checked} onChange={()=>setSelected(choice.id)}/><span>{choice.Text}</span></label>})}
    <div className="bean-choice-actions"><Button type="button" disabled={!selected||checked} onClick={()=>setChecked(true)}>Check answer</Button>{checked&&<Button type="button" variant="outline" onClick={retry}>Try again</Button>}</div>
    <div className="bean-choice-feedback" aria-live="polite" aria-atomic="true">{checked?<><strong>{selected===answer?'Correct':'Incorrect'}</strong>{explanation&&<p>{explanation}</p>}</>:''}</div>
  </fieldset>
}
