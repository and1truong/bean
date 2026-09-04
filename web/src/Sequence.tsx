import {Fragment,useEffect,useMemo,useState} from 'react'
import {useSearchParams} from 'react-router-dom'
import {ContentElement,Node} from './api'
import {Button} from '@/components/ui/button'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'

type SequenceProps={title?:string;description?:string;profile?:string;aspectRatio?:string;children?:Node[];renderNode:(node:Node)=>React.ReactNode}

export function SequenceView({title,description,profile,aspectRatio,children=[],renderNode}:SequenceProps){
  const[urlParams,setURLParams]=useSearchParams();const frames=children.filter(child=>child.component==='SequenceFrame')
  const requested=urlParams.get('frame');const activeIndex=Math.max(0,frames.findIndex(frame=>frame.props?.name===requested));const active=frames[activeIndex]
  const[showNotes,setShowNotes]=useState(false)
  useEffect(()=>{if(!title)return;const previous=document.title;document.title=title;return()=>{document.title=previous}},[title])
  const frameNames=useMemo(()=>frames.map(frame=>String(frame.props?.name||'')),[frames])
  const positions=useMemo(()=>{let horizontal=-1,vertical=0;return frames.map((frame,index)=>{if(index===0||frame.props?.direction!=='down'){horizontal++;vertical=0}else vertical++;return{index,horizontal,vertical}})},[frames])
  const activePosition=positions[activeIndex]||{index:0,horizontal:0,vertical:0};const horizontalCount=(positions[positions.length-1]?.horizontal??-1)+1;const verticalCount=positions.filter(position=>position.horizontal===activePosition.horizontal).length;const hasDepth=positions.some(position=>position.vertical>0)
  const go=(index:number)=>{const bounded=Math.max(0,Math.min(frames.length-1,index));const next=new URLSearchParams(urlParams);next.set('frame',frameNames[bounded]);setURLParams(next,{replace:true});setShowNotes(false)}
  const goHorizontal=(offset:number)=>{const target=positions.find(position=>position.horizontal===activePosition.horizontal+offset&&position.vertical===0);if(target)go(target.index)}
  const goVertical=(offset:number)=>{const target=positions.find(position=>position.horizontal===activePosition.horizontal&&position.vertical===activePosition.vertical+offset);if(target)go(target.index)}
  useEffect(()=>{const handler=(event:KeyboardEvent)=>{const target=event.target;if(target instanceof Element&&target.matches('input, select, textarea, button, [contenteditable="true"]'))return;let handled=true;if(event.key==='ArrowRight')goHorizontal(1);else if(event.key==='ArrowLeft')goHorizontal(-1);else if(event.key==='ArrowDown')goVertical(1);else if(event.key==='ArrowUp')goVertical(-1);else if(event.key==='PageDown')go(activeIndex+1);else if(event.key==='PageUp')go(activeIndex-1);else if(event.key==='Home')go(0);else if(event.key==='End')go(frames.length-1);else handled=false;if(handled)event.preventDefault()};window.addEventListener('keydown',handler);return()=>window.removeEventListener('keydown',handler)})
  if(!active)return <section role="alert">This sequence has no visible frames.</section>
  const activeNotes=String(active.props?.notes||'')
  return <main className="bean-sequence" data-profile={profile||'presentation'} data-aspect-ratio={aspectRatio||'wide'}>
    <header className="bean-sequence-toolbar" data-sequence-control="true"><div><p className="font-semibold">{title}</p>{description&&<p className="text-sm text-muted-foreground">{description}</p>}</div><div className="flex flex-wrap items-center gap-2"><label className="text-sm" htmlFor="sequence-frame">Frame</label><NativeSelect id="sequence-frame" aria-label="Choose frame" value={String(active.props?.name||'')} onChange={event=>go(frameNames.indexOf(event.target.value))}>{frames.map((frame,index)=>{const position=positions[index];const number=hasDepth?`${position.horizontal+1}.${position.vertical+1}`:String(index+1);return <NativeSelectOption key={String(frame.props?.name)} value={String(frame.props?.name)}>{number}. {String(frame.props?.title)}</NativeSelectOption>})}</NativeSelect>{activeNotes&&<Button variant="outline" aria-pressed={showNotes} onClick={()=>setShowNotes(value=>!value)}>Speaker notes</Button>}<Button variant="outline" onClick={()=>window.print()}>Print</Button></div></header>
    <div className="bean-sequence-stage">{frames.map((frame,index)=>{const frameNotes=String(frame.props?.notes||'');const position=positions[index];return <article className="bean-sequence-frame" data-active={index===activeIndex} data-direction={position.vertical>0?'down':'next'} data-horizontal={position.horizontal+1} data-vertical={position.vertical+1} data-layout={String(frame.props?.layout||'')} aria-hidden={index!==activeIndex} aria-roledescription="slide" aria-label={`${index+1} of ${frames.length}: ${String(frame.props?.title||'')}`} key={String(frame.props?.name)} style={{aspectRatio:aspectRatio==='standard'?'4 / 3':'16 / 9'}}><h1>{String(frame.props?.title||'')}</h1><div className="bean-sequence-frame-body">{frame.children?.map((child,childIndex)=><Fragment key={childIndex}>{renderNode(child)}</Fragment>)}</div>{frameNotes&&<aside className="bean-speaker-notes" hidden={!showNotes||index!==activeIndex}><h2>Speaker notes</h2><p>{frameNotes}</p></aside>}</article>})}</div>
    <footer className="bean-sequence-navigation" data-sequence-control="true"><Button variant="outline" disabled={activePosition.horizontal===0} onClick={()=>goHorizontal(-1)}>Previous</Button>{hasDepth&&<div className="flex gap-2"><Button variant="outline" disabled={activePosition.vertical===0} onClick={()=>goVertical(-1)}>Up</Button><Button variant="outline" disabled={activePosition.vertical===verticalCount-1} onClick={()=>goVertical(1)}>Down</Button></div>}<div className="min-w-32 text-center" aria-live="polite"><span>{hasDepth?`${activePosition.horizontal+1}.${activePosition.vertical+1} / ${horizontalCount}`:`${activeIndex+1} / ${frames.length}`}</span><progress className="ml-3" aria-label="Presentation progress" max={frames.length} value={activeIndex+1}/></div><Button disabled={activePosition.horizontal===horizontalCount-1} onClick={()=>goHorizontal(1)}>Next</Button></footer>
  </main>
}

export function ContentBlock({content}:{content:ContentElement[]}){
  return <div className="bean-content-block">{content.map((element,index)=><ContentItem key={index} element={element}/>)}</div>
}

function ContentItem({element}:{element:ContentElement}){
  if(element.Type==='heading')return <h2>{element.Text}</h2>
  if(element.Type==='paragraph')return <p>{element.Text}</p>
  if(element.Type==='bullets')return <ul>{element.Items?.map((item,index)=><li key={index}>{item}</li>)}</ul>
  if(element.Type==='quote')return <blockquote><p>{element.Text}</p>{element.Attribution&&<footer>— {element.Attribution}</footer>}</blockquote>
  if(element.Type==='code')return <pre><code data-language={element.Language}>{element.Text}</code></pre>
  if(element.Type==='callout')return <aside className="bean-content-callout" data-tone={element.Tone}>{element.Text}</aside>
  if(element.Type==='image')return <SemanticImage source={element.Source||''} alt={element.Alt||''}/>
  if(element.Type==='diagram')return <ol className="bean-content-diagram" data-direction={element.Direction}>{element.Items?.map((item,index)=><li key={index}><span>{item}</span>{index<(element.Items?.length||0)-1&&<span aria-hidden="true">→</span>}</li>)}</ol>
  return <section role="alert">Unsupported content element: {element.Type}</section>
}

function SemanticImage({source,alt}:{source:string;alt:string}){const[failed,setFailed]=useState(false);return <figure>{failed?<div className="bean-image-fallback" role="img" aria-label={alt}>{alt}</div>:<img src={source} alt={alt} onError={()=>setFailed(true)}/>}<figcaption>{alt}</figcaption></figure>}
