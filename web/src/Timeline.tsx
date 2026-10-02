import {type ReactNode,useId} from 'react'
import type {TimelineEntry} from './api'

// Shared rail + dot markup for both record-backed and literal timelines.
export type TimelineRailItem={key:string|number;label:ReactNode;title:ReactNode;body?:ReactNode;meta?:ReactNode}

export function TimelineEntries({items,testid}:{items:TimelineRailItem[];testid?:string}){
  return <ol className="relative space-y-6 border-l pl-6" role="list" data-testid={testid}>{items.map(item=><li key={item.key}><span className="absolute -ml-[1.85rem] mt-1.5 size-3 rounded-full bg-primary"/>{item.label}{item.title}{item.body??null}{item.meta??null}</li>)}</ol>
}

// Literal semantic timeline: labels are authored text kept verbatim (no date parsing), source order is the chronology.
export function TimelineBlock({title,entries}:{title:string;entries:TimelineEntry[]}){
  const id=useId()
  return <article className="bean-timeline" aria-labelledby={id}>
    <h2 className="bean-timeline-title" id={id}>{title}</h2>
    <TimelineEntries items={entries.map((entry,index)=>({
      key:entry.id||index,
      label:<p className="bean-timeline-label">{entry.Label||''}</p>,
      title:<h3 className="bean-timeline-entry-title">{entry.Title||''}</h3>,
      body:entry.Description?<p className="bean-timeline-description">{entry.Description}</p>:undefined,
    }))}/>
  </article>
}
