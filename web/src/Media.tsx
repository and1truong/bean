import {useEffect,useState} from 'react'
import {useContentActive} from './ContentVisibility'
import {Button} from '@/components/ui/button'

function Transcript({children}:{children:string}){return <details className="bean-media-transcript" open><summary>Transcript</summary><p>{children}</p></details>}

export function AudioContent({source,title,transcript}:{source:string;title:string;transcript:string}){
  const active=useContentActive();const[loaded,setLoaded]=useState(false);const[status,setStatus]=useState('')
  useEffect(()=>{setLoaded(false);setStatus('')},[source,title,transcript])
  useEffect(()=>{if(!active){setLoaded(false);setStatus('')}},[active])
  return <figure className="bean-media"><h3>{title}</h3><Transcript>{transcript}</Transcript>{!loaded?<><p>Audio is loaded from an external or application source only after you choose Load audio.</p><Button type="button" onClick={()=>{setLoaded(true);setStatus('Loading audio…')}}>Load audio</Button></>:<audio aria-label={title} controls preload="none" src={source} onCanPlay={()=>setStatus('Audio ready.')} onError={()=>setStatus('Audio could not be loaded.')}/>}<p role="status">{status}</p><a href={source}>Open audio source</a></figure>
}

export function YouTubeContent({kind,id,title,transcript}:{kind:'video'|'playlist';id:string;title:string;transcript:string}){
  const active=useContentActive();const[loaded,setLoaded]=useState(false);const[status,setStatus]=useState('')
  useEffect(()=>{setLoaded(false);setStatus('')},[kind,id,title,transcript])
  useEffect(()=>{if(!active){setLoaded(false);setStatus('')}},[active])
  const playlist=kind==='playlist'
  const embed=playlist?`https://www.youtube-nocookie.com/embed/videoseries?listType=playlist&list=${id}&autoplay=0&controls=1&playsinline=1&cc_load_policy=1`:`https://www.youtube-nocookie.com/embed/${id}?autoplay=0&controls=1&playsinline=1&cc_load_policy=1`
  const fallback=playlist?`https://www.youtube.com/playlist?list=${id}`:`https://www.youtube.com/watch?v=${id}`
  const label=playlist?'YouTube playlist':'YouTube video'
  return <figure className="bean-media"><h3>{title}</h3><Transcript>{transcript}</Transcript>{!loaded?<><p>Loading this {label.toLowerCase()} contacts YouTube. Privacy-enhanced mode still permits provider processing after loading.</p><Button type="button" onClick={()=>{setLoaded(true);setStatus(`Loading ${label.toLowerCase()}…`)}}>Load {label}</Button></>:<div className="bean-youtube-frame"><iframe src={embed} title={title} allow="encrypted-media; fullscreen; picture-in-picture" allowFullScreen referrerPolicy="strict-origin-when-cross-origin" onLoad={()=>setStatus(`${label} frame loaded. Playback availability is not verified.`)}/></div>}<p role="status">{status}</p><a href={fallback}>Open on YouTube</a></figure>
}
