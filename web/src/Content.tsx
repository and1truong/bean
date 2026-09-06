import {useEffect,useId,useState} from 'react'
import {Link} from 'react-router-dom'
import {ContentElement} from './api'
import {Choices} from './Choices'
import {AudioContent,YouTubeContent} from './Media'
import {Table,TableBody,TableCaption,TableCell,TableHead,TableHeader,TableRow} from '@/components/ui/table'

export function ContentBlock({content}:{content:ContentElement[]}){
  return <div className="bean-content-block">{content.map((element,index)=><ContentItem key={index} element={element}/>)}</div>
}

function ContentItem({element}:{element:ContentElement}){
  if(element.Type==='heading')return element.Level===3?<h3>{element.Text}</h3>:element.Level===4?<h4>{element.Text}</h4>:<h2>{element.Text}</h2>
  if(element.Type==='paragraph')return <p>{element.Text}</p>
  if(element.Type==='bullets')return <ul>{element.Items.map((item,index)=><li key={index}>{item}</li>)}</ul>
  if(element.Type==='ordered_list')return <ol>{element.Items.map((item,index)=><li key={index}>{item}</li>)}</ol>
  if(element.Type==='quote')return <blockquote><p>{element.Text}</p>{element.Attribution&&<footer>— {element.Attribution}</footer>}</blockquote>
  if(element.Type==='code')return <pre><code data-language={element.Language}>{element.Text}</code></pre>
  if(element.Type==='callout')return <aside className="bean-content-callout" data-tone={element.Tone}>{element.Text}</aside>
  if(element.Type==='image')return <SemanticImage source={element.Source} alt={element.Alt}/>
  if(element.Type==='diagram')return <ol className="bean-content-diagram" data-direction={element.Direction}>{element.Items.map((item,index)=><li key={index}><span>{item}</span>{index<element.Items.length-1&&<span aria-hidden="true">→</span>}</li>)}</ol>
  if(element.Type==='link'){const external=element.OpenIn==='new_tab';const props={target:external?'_blank':undefined,rel:external?'noopener noreferrer':undefined,'aria-label':external?`${element.Label} (opens in a new tab)`:undefined};return element.Target.startsWith('/')?<Link to={element.Target} {...props}>{element.Label}{external&&<span className="sr-only"> (opens in a new tab)</span>}</Link>:<a href={element.Target} {...props}>{element.Label}{external&&<span className="sr-only"> (opens in a new tab)</span>}</a>}
  if(element.Type==='divider')return <hr/>
  if(element.Type==='table')return <StaticTable element={element}/>
  if(element.Type==='audio')return <AudioContent source={element.Source} title={element.Title} transcript={element.Transcript}/>
  if(element.Type==='youtube')return <YouTubeContent kind="video" id={element.VideoID} title={element.Title} transcript={element.Transcript}/>
  if(element.Type==='youtube_playlist')return <YouTubeContent kind="playlist" id={element.PlaylistID} title={element.Title} transcript={element.Transcript}/>
  if(element.Type==='choices')return <Choices question={element.Question} choices={element.Choices} answer={element.Answer} explanation={element.Explanation}/>
  return null
}

function StaticTable({element}:{element:Extract<ContentElement,{Type:'table'}>}){
  const id=useId();return <div className="bean-static-table" role="region" aria-labelledby={id} tabIndex={0} data-keyboard-scrollable="true"><Table><TableCaption id={id}>{element.Caption}</TableCaption><TableHeader><TableRow>{element.Columns.map(column=><TableHead key={column.id} scope="col">{column.Label}</TableHead>)}</TableRow></TableHeader><TableBody>{element.Rows.map((row,rowIndex)=><TableRow key={rowIndex}>{row.map((cell,cellIndex)=>element.RowHeader==='first'&&cellIndex===0?<TableHead className="static" key={cellIndex} scope="row">{cell}</TableHead>:<TableCell key={cellIndex}>{cell}</TableCell>)}</TableRow>)}</TableBody></Table></div>
}

function SemanticImage({source,alt}:{source:string;alt:string}){const[failed,setFailed]=useState(false);useEffect(()=>setFailed(false),[source,alt]);return <figure>{failed?<div className="bean-image-fallback" role="img" aria-label={alt}>{alt}</div>:<img src={source} alt={alt} loading="lazy" decoding="async" onError={()=>setFailed(true)}/>}<figcaption>{alt}</figcaption></figure>}
