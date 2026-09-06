import {useEffect,useId,useRef,useState} from 'react'
import {ContentTab} from './api'
import {ContentBlock} from './Content'
import {ContentVisibility,useContentActive} from './ContentVisibility'
import {Button} from '@/components/ui/button'

export function TabsBlock({label,orientation='horizontal',variant='underline',tabs}:{label:string;orientation?:'horizontal'|'vertical';variant?:'underline'|'pills';tabs:ContentTab[]}){
  const parentActive=useContentActive();const namespace=useId().replaceAll(':','');const[active,setActive]=useState(0);const refs=useRef<Array<HTMLButtonElement|null>>([])
  const identity=JSON.stringify([label,orientation,variant,tabs])
  useEffect(()=>setActive(0),[identity])
  useEffect(()=>{if(!parentActive)setActive(0)},[parentActive])
  const select=(index:number)=>{const next=(index+tabs.length)%tabs.length;setActive(next);refs.current[next]?.focus()}
  const key=(event:React.KeyboardEvent,index:number)=>{let next:number|undefined;if(event.key==='Home')next=0;else if(event.key==='End')next=tabs.length-1;else if(orientation==='horizontal'&&event.key==='ArrowRight'||orientation==='vertical'&&event.key==='ArrowDown')next=index+1;else if(orientation==='horizontal'&&event.key==='ArrowLeft'||orientation==='vertical'&&event.key==='ArrowUp')next=index-1;if(next===undefined)return;event.preventDefault();select(next)}
  return <section className="bean-tabs" data-orientation={orientation} data-variant={variant}>
    <div role="tablist" aria-label={label} aria-orientation={orientation}>{tabs.map((tab,index)=>{const tabID=`bean-tab-${namespace}-${tab.id}`;const panelID=`bean-tabpanel-${namespace}-${tab.id}`;return <Button variant="ghost" type="button" role="tab" id={tabID} aria-selected={index===active} aria-controls={panelID} tabIndex={index===active?0:-1} ref={node=>{refs.current[index]=node}} key={tab.id} onClick={()=>setActive(index)} onKeyDown={event=>key(event,index)}>{tab.Label}</Button>})}</div>
    {tabs.map((tab,index)=>{const tabID=`bean-tab-${namespace}-${tab.id}`;const panelID=`bean-tabpanel-${namespace}-${tab.id}`;return <div className="bean-tabs-panel" role="tabpanel" id={panelID} aria-labelledby={tabID} aria-hidden={index!==active} data-hidden={index!==active} tabIndex={index===active?0:-1} key={tab.id}><ContentVisibility active={parentActive&&index===active}><ContentBlock content={tab.Content}/></ContentVisibility></div>})}
  </section>
}
