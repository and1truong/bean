import {useCallback,useEffect,useRef,useState} from 'react'
import {useLocation,useNavigate} from 'react-router-dom'
import {DownloadIcon,FilePlusIcon,FolderOpenIcon,MoonIcon,PlayIcon,RefreshCwIcon,SunIcon,Trash2Icon} from 'lucide-react'
import type {Node} from '../api'
import {humanize,Renderer} from '../registry'
import {Bridge,type BridgeApp,type BridgeDiagnostic,type BridgeFailure,type BridgeUnsupported} from './bridge'
import {readZip,writeZip} from './zip'
import {Field,Page} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'
import {Textarea} from '@/components/ui/textarea'

const STARTER_MANIFEST='app.yaml'
const STARTER_FILES:Record<string,string>={
  'app.yaml':`apiVersion: bean/v1alpha1
name: Playground draft
---
kind: Block
name: welcome
type: content
content:
  - {type: heading, level: 2, text: "Bean playground"}
  - {type: paragraph, text: "Edit the source on the left, compile, and preview. The real Go compiler runs in this browser tab via WebAssembly — no Bean server is involved."}
  - {type: callout, tone: info, text: "Backend-backed blocks such as Views, Webforms, and Actions are marked as unavailable instead of being simulated."}
---
kind: Block
name: practice
type: flashcard
title: Try a flashcard
cards:
  - {id: one, prompt: "What compiles Bean source here?", answer: "The same Go compiler the Bean server uses, compiled to WebAssembly."}
  - {id: two, prompt: "Where does the source live?", answer: "Only in this browser tab — export a zip to keep it."}
---
kind: Panel
name: main
layout: single-column
regions:
  - {name: main, blocks: [welcome, practice]}
---
kind: Page
name: home
panel: main
route: /
title: Playground draft
`,
}

type Example={name:string;title?:string}
type ExampleBundle={manifest:string;files:Record<string,string>}

function assetURL(path:string){return new URL(path,document.baseURI)}

export default function PlaygroundApp(){
  const location=useLocation()
  const navigate=useNavigate()
  const[colorMode,setColorMode]=useState<'light'|'dark'>(()=>localStorage.getItem('bean_color_mode')==='dark'?'dark':'light')
  const[files,setFiles]=useState<Record<string,string>>({...STARTER_FILES})
  const[manifest,setManifest]=useState(STARTER_MANIFEST)
  const[activeFile,setActiveFile]=useState(STARTER_MANIFEST)
  const[newFileName,setNewFileName]=useState('')
  const[examples,setExamples]=useState<Example[]>([])
  const[app,setApp]=useState<BridgeApp|null>(null)
  const[diagnostics,setDiagnostics]=useState<BridgeDiagnostic[]>([])
  const[tree,setTree]=useState<Node|null>(null)
  const[unsupported,setUnsupported]=useState<BridgeUnsupported[]>([])
  const[editorError,setEditorError]=useState<BridgeFailure|null>(null)
  const[renderError,setRenderError]=useState<BridgeFailure|null>(null)
  const[fatal,setFatal]=useState('')
  const[busy,setBusy]=useState<'starting'|'compiling'|'rendering'|''>('starting')
  const[compileMs,setCompileMs]=useState<number|null>(null)
  const fileInputRef=useRef<HTMLInputElement>(null)
  const bridgeRef=useRef<Bridge|null>(null)
  const generation=useRef(0)
  const filesRef=useRef(files);filesRef.current=files
  const manifestRef=useRef(manifest);manifestRef.current=manifest

  const compile=useCallback(async(sourceFiles?:Record<string,string>,sourceManifest?:string)=>{
    const bridge=bridgeRef.current
    if(!bridge||!bridge.isReady())return
    const id=++generation.current
    setBusy('compiling')
    setEditorError(null)
    const started=performance.now()
    const response=await bridge.call({v:1,op:'compile',files:sourceFiles||filesRef.current,manifest:sourceManifest||manifestRef.current})
    if(id!==generation.current)return
    setCompileMs(Math.round(performance.now()-started))
    setBusy('')
    if(response.error){setEditorError(response.error);return}
    setDiagnostics(response.diagnostics||[])
    if(response.ok&&response.app)setApp(response.app)
  },[])

  useEffect(()=>{
    const bridge=new Bridge()
    bridgeRef.current=bridge
    let cancelled=false
    bridge.onFatal=error=>{if(!cancelled){setFatal(error);setBusy('')}}
    bridge.start()
      .then(()=>{if(!cancelled){setBusy('');void compile()}})
      .catch((error:Error)=>{if(!cancelled){setFatal(error.message);setBusy('')}})
    return ()=>{cancelled=true;bridge.dispose();bridgeRef.current=null}
  },[compile])

  useEffect(()=>{
    document.documentElement.classList.toggle('dark',colorMode==='dark')
    document.documentElement.dataset.theme=colorMode
    document.documentElement.dataset.preset=app?.theme?.preset||'professional'
    document.documentElement.dataset.accent=app?.theme?.accent||'emerald'
  },[colorMode,app?.theme?.preset,app?.theme?.accent])

  useEffect(()=>{
    let cancelled=false
    fetch(assetURL('examples/index.json'))
      .then(response=>response.ok?response.json():[])
      .then(list=>{if(!cancelled)setExamples(list)})
      .catch(()=>{})
    return()=>{cancelled=true}
  },[])

  useEffect(()=>{
    const bridge=bridgeRef.current
    if(!app||!bridge)return
    const route=app.routes.find(item=>item.path===location.pathname)
    if(!route){
      const fallback=app.routes.find(item=>!item.unsupported)||app.routes[0]
      if(fallback)navigate(fallback.path,{replace:true})
      return
    }
    const id=++generation.current
    setBusy('rendering')
    const query=Object.fromEntries(new URLSearchParams(location.search))
    void bridge.call({v:1,op:'render',path:route.path,query}).then(response=>{
      if(id!==generation.current)return
      setBusy('')
      if(response.ok&&response.tree){setTree(response.tree);setUnsupported(response.unsupported||[]);setRenderError(null)}
      else setRenderError(response.error||{code:'BEAN-P4205',message:'the compiler sent an empty render response'})
    })
  },[app,location.pathname,location.search,navigate])

  const restart=()=>{
    const bridge=bridgeRef.current
    if(!bridge)return
    setFatal('');setBusy('starting');setApp(null);setTree(null);setDiagnostics([])
    bridge.start()
      .then(()=>{setBusy('');void compile()})
      .catch((error:Error)=>{setFatal(error.message);setBusy('')})
  }

  const loadExample=async(name:string)=>{
    try{
      const bundle=await fetch(assetURL(`examples/${name}.json`)).then(response=>{
        if(!response.ok)throw new Error('HTTP '+response.status)
        return response.json() as Promise<ExampleBundle>
      })
      setFiles(bundle.files);setManifest(bundle.manifest);setActiveFile(bundle.manifest)
      setApp(null);setTree(null);setDiagnostics([]);setRenderError(null);setEditorError(null)
      void compile(bundle.files,bundle.manifest)
    }catch{
      setEditorError({code:'BEAN-P4100',message:`Could not load the "${name}" example from this host.`})
    }
  }

  const importFiles=async(list:FileList|null)=>{
    setEditorError(null)
    try{
      const next:Record<string,string>={}
      let nextManifest=''
      for(const file of Array.from(list||[])){
        if(file.name.endsWith('.zip')){
          const entries=await readZip(file)
          for(const[name,contents]of Object.entries(entries))if(/\.(yaml|yml|json)$/i.test(name)&&!name.includes('..'))next[name]=contents
        }else if(file.name.endsWith('.json')){
          const text=await file.text()
          try{
            const parsed=JSON.parse(text)as ExampleBundle
            if(parsed&&typeof parsed==='object'&&parsed.files&&typeof parsed.files==='object'){
              Object.assign(next,parsed.files)
              if(typeof parsed.manifest==='string')nextManifest=parsed.manifest
              continue
            }
          }catch{/* a plain .json source file, not an export bundle */}
          next[file.name]=text
        }else next[file.name]=await file.text()
      }
      const names=Object.keys(next)
      if(!names.length){setEditorError({code:'BEAN-P4101',message:'no .yaml/.yml/.json sources found in the import'});return}
      if(names.length>64){setEditorError({code:'BEAN-P4101',message:'too many source files (maximum 64)'});return}
      if(names.some(name=>next[name].length>256*1024)){setEditorError({code:'BEAN-P4101',message:'a source file exceeds the 256 KiB limit'});return}
      if(names.reduce((sum,name)=>sum+next[name].length,0)>1024*1024){setEditorError({code:'BEAN-P4101',message:'imported sources exceed the 1 MiB limit'});return}
      const candidate=nextManifest||(names.includes('app.yaml')?'app.yaml':names.filter(name=>/\.(yaml|yml)$/i.test(name)).sort()[0])
      if(!candidate||!(candidate in next)){setEditorError({code:'BEAN-P4103',message:'could not find an app.yaml manifest in the import'});return}
      setFiles(next);setManifest(candidate);setActiveFile(candidate)
      setApp(null);setTree(null);setDiagnostics([]);setRenderError(null)
      void compile(next,candidate)
    }catch(cause){
      setEditorError({code:'BEAN-P4100',message:'import failed: '+String((cause as Error).message||cause)})
    }
  }

  const download=(blob:Blob,name:string)=>{
    const url=URL.createObjectURL(blob)
    const anchor=document.createElement('a')
    anchor.href=url;anchor.download=name;anchor.click()
    setTimeout(()=>URL.revokeObjectURL(url),5000)
  }

  const addFile=()=>{
    const name=newFileName.trim()
    if(!/^[a-zA-Z0-9_][a-zA-Z0-9_./-]*\.(yaml|yml|json)$/i.test(name)||name.includes('..')||name.startsWith('/')||name.includes('//')){
      setEditorError({code:'BEAN-P4102',message:'file names must be relative .yaml/.yml/.json paths without ".."'})
      return
    }
    if(files[name]!==undefined){setEditorError({code:'BEAN-P4102',message:name+' already exists'});return}
    setEditorError(null)
    setFiles(current=>({...current,[name]:''}))
    setActiveFile(name)
    setNewFileName('')
  }

  const deleteFile=(name:string)=>{
    if(name===manifest)return
    setFiles(current=>{const next={...current};delete next[name];return next})
    if(activeFile===name)setActiveFile(manifest)
  }

  const theme=()=>setColorMode(current=>{const next=current==='light'?'dark':'light';localStorage.setItem('bean_color_mode',next);return next})
  const fileNames=Object.keys(files)
  const route=app?.routes.find(item=>item.path===location.pathname)

  return <div className={`bean-app-shell ${colorMode}`} data-theme={colorMode} data-preset={app?.theme?.preset||'professional'} data-accent={app?.theme?.accent||'emerald'}>
    <header className="bean-topbar"><div className="bean-topbar-inner">
      <span className="bean-brand"><span className="bean-brand-mark" aria-hidden="true">B</span><span className="truncate">Bean playground</span></span>
      <span className="bean-workspace-label">{app?.title||'browser-only preview'}</span>
      <nav className="ml-auto flex min-w-0 flex-wrap items-center justify-end gap-1" aria-label="Playground actions">
        <Button size="icon" variant="ghost" onClick={theme} aria-label={colorMode==='light'?'Use dark theme':'Use light theme'} title={colorMode==='light'?'Use dark theme':'Use light theme'}>{colorMode==='light'?<MoonIcon/>:<SunIcon/>}</Button>
        <Button variant="ghost" onClick={()=>fileInputRef.current?.click()}><FolderOpenIcon data-icon="inline-start"/>Import</Button>
        <Input ref={fileInputRef} type="file" className="hidden" multiple accept=".yaml,.yml,.json,.zip,application/zip" aria-label="Import Bean sources" onChange={event=>{void importFiles(event.target.files);event.target.value=''}}/>
        <Button variant="ghost" onClick={()=>void writeZip(files).then(blob=>download(blob,'bean-sources.zip'))}><DownloadIcon data-icon="inline-start"/>Export .zip</Button>
        <Button variant="ghost" onClick={()=>download(new Blob([JSON.stringify({v:1,manifest,files},null,2)],{type:'application/json'}),'bean-sources.json')}><DownloadIcon data-icon="inline-start"/>Export .json</Button>
      </nav>
    </div></header>
    <main className="bean-playground mx-auto grid w-full max-w-[110rem] gap-4 p-4 lg:grid-cols-[minmax(24rem,2fr)_minmax(0,3fr)]">
      <section aria-label="Source" className="min-w-0 space-y-3">
        <div className="flex flex-wrap items-end gap-2">
          <Field label="Example" id="playground-example">
            <NativeSelect id="playground-example" value="" onChange={event=>{if(event.target.value)void loadExample(event.target.value)}} disabled={!examples.length}>
              <NativeSelectOption value="">{examples.length?'Load an example…':'No examples on this host'}</NativeSelectOption>
              {examples.map(example=><NativeSelectOption key={example.name} value={example.name}>{example.title||humanize(example.name)}</NativeSelectOption>)}
            </NativeSelect>
          </Field>
          <div className="flex-1"/>
          <Field label="New file" id="playground-new-file">
            <div className="flex gap-2">
              <Input id="playground-new-file" value={newFileName} onChange={event=>setNewFileName(event.target.value)} onKeyDown={event=>{if(event.key==='Enter')addFile()}} placeholder="blocks.yaml" aria-label="New file name"/>
              <Button type="button" variant="outline" onClick={addFile} aria-label="Add file"><FilePlusIcon/></Button>
            </div>
          </Field>
        </div>
        <div className="flex flex-wrap items-center gap-1" role="tablist" aria-label="Source files">
          {fileNames.map(name=><span key={name} className="inline-flex items-center gap-0.5">
            <Button role="tab" aria-selected={name===activeFile} variant={name===activeFile?'secondary':'ghost'} size="sm" onClick={()=>setActiveFile(name)} className="font-mono text-xs">{name}{name===manifest?' (manifest)':''}</Button>
            {name!==manifest&&<Button size="icon" variant="ghost" className="size-6" aria-label={`Delete ${name}`} onClick={()=>deleteFile(name)}><Trash2Icon/></Button>}
          </span>)}
        </div>
        <Label htmlFor="playground-editor" className="sr-only">{activeFile} source</Label>
        <Textarea id="playground-editor" className="min-h-[30rem] w-full font-mono text-sm" spellCheck={false} value={files[activeFile]??''} onChange={event=>setFiles(current=>({...current,[activeFile]:event.target.value}))}/>
        {editorError&&<p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm"><code>{editorError.code}</code> {editorError.message}</p>}
        {diagnostics.length>0&&<section aria-label="Diagnostics" className="rounded-md border border-destructive/40 bg-destructive/10 p-3">
          <h2 className="text-sm font-semibold">{diagnostics.length} diagnostic{diagnostics.length>1?'s':''} — the preview keeps the last working version</h2>
          <ul className="mt-2 space-y-1 text-sm">
            {diagnostics.map((diagnostic,index)=><li key={index}>
              <Button type="button" variant="ghost" className="h-auto w-full justify-start whitespace-normal p-1 text-left font-mono text-xs" onClick={()=>{if(diagnostic.source?.path&&files[diagnostic.source.path]!==undefined)setActiveFile(diagnostic.source.path)}}>
                <span><code className="font-semibold">{diagnostic.code}</code> {diagnostic.source?.path}{diagnostic.source?.line?`:${diagnostic.source.line}`:''} — {diagnostic.message}{diagnostic.candidates?.length?` (did you mean ${diagnostic.candidates.join(', ')}?)`:''}</span>
              </Button>
            </li>)}
          </ul>
        </section>}
        <p className="text-xs text-muted-foreground">Sources live in memory only — export a zip or JSON to keep them; reloading resets to the starter.</p>
      </section>
      <section aria-label="Preview" className="min-w-0 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <Button onClick={()=>void compile()} disabled={busy==='starting'||busy==='compiling'}><PlayIcon data-icon="inline-start"/>{busy==='compiling'?'Compiling…':'Compile & preview'}</Button>
          {compileMs!==null&&<span className="text-xs text-muted-foreground">compiled in {compileMs} ms</span>}
          <div className="flex-1"/>
          {app&&<Field label="Route" id="playground-route">
            <NativeSelect id="playground-route" value={route?.path||''} onChange={event=>navigate(event.target.value)} aria-label="Preview route">
              {app.routes.map(item=><NativeSelectOption key={item.path+item.kind} value={item.path}>{item.path}{item.kind!=='page'?` [${item.kind}]`:''}{item.title?` · ${item.title}`:''}{item.unsupported?' — needs backend':item.protected?' — needs sign-in':''}</NativeSelectOption>)}
            </NativeSelect>
          </Field>}
        </div>
        {fatal&&<div role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm">
          <p><strong>The compiler could not start:</strong> {fatal}</p>
          <p className="mt-1">Serve this directory over HTTP with any static file server (a <code>file://</code> URL cannot load WebAssembly). If it stays broken, try restarting the compiler.</p>
          <Button className="mt-2" variant="outline" size="sm" onClick={restart}><RefreshCwIcon data-icon="inline-start"/>Restart compiler</Button>
        </div>}
        {busy==='starting'&&!fatal&&<p className="text-sm text-muted-foreground" role="status">Loading the WebAssembly compiler…</p>}
        {(()=>{const chips=unsupported.length?unsupported:(app?.unsupported||[])
          return chips.length?<div className="flex flex-wrap gap-2" aria-label="Backend-dependent components">
          {chips.map((item,index)=><span key={index} className="rounded-full border px-2 py-0.5 text-xs text-muted-foreground" title={item.reason}>{item.count}× {item.component} — {item.reason}</span>)}
        </div>:null})()}
        {renderError&&!fatal&&<p role="alert" className="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm"><code>{renderError.code}</code> {renderError.message}</p>}
        <div className="rounded-lg border bg-background">
          {tree?(tree.component==='Sequence'?<Renderer node={tree}/>:<Page className="max-w-none space-y-6 px-0"><Renderer node={tree}/></Page>):busy===''&&!fatal&&!renderError?<p className="p-6 text-sm text-muted-foreground">Compile the source to preview it.</p>:null}
        </div>
      </section>
    </main>
  </div>
}
