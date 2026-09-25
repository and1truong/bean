import {useEffect,useState} from 'react'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {Link,useNavigate,useParams} from 'react-router-dom'
import {api} from './api'
import {useEditor} from './store'
import {EmptyState,ErrorAlert,Field,LoadingState,Page,PageHeader,SectionCard,StatusIndicator} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'

type ScenarioNode={id:string;type:string;label?:string;next?:string;onFail?:string;url?:string;ref?:string;text?:string;value?:string;key?:string;secret?:string;condition?:string;assertion?:string;as?:string;attribute?:string;name?:string;until?:string;body?:string;maxIterations?:number;action?:string}
type Scenario={name:string;title?:string;start:string;nodes:ScenarioNode[]}
type Run={ID:string;AppID:string;Scenario:string;Trigger:string;Status:string;Error:string;CreatedAt:string;StartedAt:string;FinishedAt:string}
type Step={ID:string;NodeID:string;Attempt:number;Status:string;Output:string;Error:string;CreatedAt:string;StartedAt:string;FinishedAt:string}
type Artifact={ID:string;StepID:string;Kind:string;ContentType:string;Size:number;Ref:string}
type RunDetail={run:Run;sessions:{ID:string;Status:string}[];steps:Step[];artifacts:Artifact[]}
type RunEvent={ID:string;StepID:string;Sequence:number;Kind:string;Payload:string;CreatedAt:string}

const liveStatuses=['pending','running','paused']
const live=(status:string)=>liveStatuses.includes(status)
const statusKind=(status:string):'success'|'danger'|'info'|'warning'|'neutral'=>status==='completed'?'success':status==='failed'||status==='cancelled'?'danger':status==='running'?'info':status==='paused'?'warning':'neutral'
const parseTime=(value:string)=>{const time=Date.parse(value);return time>0?time:0}
const duration=(from:string,to:string)=>{const start=parseTime(from);if(!start)return '—';const ms=(parseTime(to)||Date.now())-start;return ms<1000?ms+' ms':(ms/1000).toFixed(1)+' s'}
const stamp=(value:string)=>{const time=Date.parse(value);return time?new Date(time).toLocaleTimeString():'—'}
const fmtError=(detail:RunDetail|undefined)=>detail?.run.Error||''
// setupFailure recognizes environment errors (a missing browser binary, not a
// test assertion) so the page can lead with the fix instead of the stack trace.
const setupFailure=(error:string)=>/Executable doesn't exist|headless_shell|playwright install|browserType\.launch/i.test(error)

const refLabel=(node:ScenarioNode)=>node.label||node.ref||''
const conditionText=(condition:string|undefined)=>({navigation:'navigation',network_idle:'network idle',ref_visible:'element visible',ref_hidden:'element hidden',url_equals:'URL equals',url_contains:'URL contains',text_present:'text on page',last_step_passed:'previous step passed',last_step_failed:'previous step failed'})[condition||'']||condition||''
const waitSentence=(node:ScenarioNode):string=>{
  switch(node.condition){
    case 'navigation':return 'Wait for navigation'
    case 'network_idle':return 'Wait for the page to settle'
    case 'ref_visible':return `Wait for ${node.ref} to appear`
    case 'ref_hidden':return `Wait for ${node.ref} to disappear`
    case 'text_present':return `Wait for "${node.text}"`
    case 'url_equals':return `Wait for URL to be "${node.text||node.url}"`
    case 'url_contains':return `Wait for URL to contain "${node.text||node.url}"`
    default:return `Wait (${conditionText(node.condition)||'condition'})`
  }
}
const assertSentence=(node:ScenarioNode):string=>{
  switch(node.assertion){
    case 'url_equals':return `Page URL is "${node.text||node.url}"`
    case 'url_contains':return `Page URL contains "${node.text||node.url}"`
    case 'text_present':return `"${node.text}" appears on the page`
    case 'ref_visible':return `${refLabel(node)} is visible`
    case 'ref_hidden':return `${refLabel(node)} is hidden`
    case 'ref_text':return `${refLabel(node)} shows "${node.text}"`
    default:return `Assert ${node.assertion||'condition'}`
  }
}
// stepSentence renders a node as the user-facing action or check it performs —
// the check list reads like a test report, not machine node IDs.
const stepSentence=(node:ScenarioNode):string=>{
  switch(node.type){
    case 'navigate':return `Go to ${node.url}`
    case 'click':return `Click ${refLabel(node)}`
    case 'fill':return node.secret?`Fill ${refLabel(node)} with a secret`:`Fill ${refLabel(node)}${node.text?` with "${node.text}"`:''}`
    case 'select':return `Choose "${node.value}" in ${refLabel(node)}`
    case 'press':return `Press ${node.key}`
    case 'wait':return waitSentence(node)
    case 'assert':return assertSentence(node)
    case 'extract':return `Remember ${refLabel(node)}'s ${node.attribute||'text'} as ${node.as||node.name||'value'}`
    case 'branch':return `Branch on ${conditionText(node.condition)||'condition'}`
    case 'loop':return `Repeat until ${conditionText(node.until)||`${node.maxIterations||''} iterations`}`
    case 'script':return 'Run script'
    case 'api_call':return `Call action ${node.action}`
    case 'pause':return 'Pause for takeover'
    default:return node.type
  }
}

export function ScenarioRuns(){
  const nav=useNavigate();const[status,setStatus]=useState('');const[scenario,setScenario]=useState('')
  const runs=useQuery({queryKey:['scenario-runs',status],queryFn:()=>api<{runs:Run[]}>('/api/scenario-runs'+(status?'?status='+encodeURIComponent(status):'')),refetchInterval:5000})
  const scenarios=useQuery({queryKey:['scenarios'],queryFn:()=>api<Record<string,Scenario>>('/api/scenarios')})
  const start=useMutation({mutationFn:(name:string)=>api<Run>('/api/scenario-runs',{method:'POST',body:JSON.stringify({scenario:name})}),onSuccess:run=>nav(`/studio/runs/${run.ID}`)})
  const names=Object.keys(scenarios.data||{}).sort()
  return <Page><PageHeader title="Scenario runs" description="Run compiled scenarios and watch execution live." action={<div className="flex items-end gap-2"><Field id="run-scenario" label="Scenario"><NativeSelect id="run-scenario" data-testid="run-scenario" value={scenario||names[0]||''} onChange={event=>setScenario(event.target.value)}>{names.map(name=><NativeSelectOption key={name}>{name}</NativeSelectOption>)}</NativeSelect></Field><Button data-testid="start-run" disabled={!names.length||start.isPending} onClick={()=>start.mutate(scenario||names[0])}>Run</Button></div>}/>
    {start.isError&&<ErrorAlert error={start.error}/>}
    <Field id="run-status-filter" label="Status"><NativeSelect id="run-status-filter" data-testid="run-status-filter" value={status} onChange={event=>setStatus(event.target.value)}><NativeSelectOption value="">All</NativeSelectOption>{['pending','running','paused','completed','failed','cancelled'].map(value=><NativeSelectOption key={value} value={value}>{value}</NativeSelectOption>)}</NativeSelect></Field>
    {runs.isPending?<LoadingState label="Loading runs…"/>:runs.error?<ErrorAlert error={runs.error}/>:runs.data?.runs.length?<ul className="space-y-2" data-testid="run-list">{runs.data.runs.map(run=><li key={run.ID}><Link className="flex items-center gap-4 rounded-lg border border-border px-4 py-3 hover:bg-muted/50" to={`/studio/runs/${run.ID}`} data-testid={`run-${run.ID}`}><span className="font-mono text-sm font-medium">{run.Scenario}</span><StatusIndicator status={statusKind(run.Status)} label={run.Status}/><span className="text-sm text-muted-foreground">{duration(run.StartedAt,run.FinishedAt)}</span><span className="ml-auto text-sm text-muted-foreground">{stamp(run.CreatedAt)}</span>{run.Error&&<span className="max-w-64 truncate text-sm text-destructive" title={run.Error}>{run.Error}</span>}</Link></li>)}</ul>:<EmptyState title="No runs" description="Start a scenario run to see it here."/>}
  </Page>
}

export function ScenarioRunDetail(){
  const {id=''}=useParams();const qc=useQueryClient();const nav=useNavigate()
  const detail=useQuery({queryKey:['scenario-run',id],queryFn:()=>api<RunDetail>(`/api/scenario-runs/${id}`)})
  const saveAsTest=useMutation({mutationFn:()=>api<{name:string;spec:Record<string,any>}>(`/api/scenario-runs/${id}/save-as-test`,{method:'POST',body:'{}'}),onSuccess:result=>{useEditor.getState().set({kind:'Scenario',name:result.name,spec:JSON.stringify(result.spec,null,2)});nav('/studio')}})
  const retry=useMutation({mutationFn:()=>api<Run>('/api/scenario-runs',{method:'POST',body:JSON.stringify({scenario:run?.Scenario})}),onSuccess:fresh=>nav(`/studio/runs/${fresh.ID}`)})
  const repair=useMutation({mutationFn:()=>api<{name:string;spec:Record<string,any>;diff:string[]}>(`/api/scenario-runs/${id}/repair`,{method:'POST',body:'{}'})})
  const[repairDraft,setRepairDraft]=useState<{name:string;spec:Record<string,any>;diff:string[]}|null>(null)
  const events=useQuery({queryKey:['scenario-run-events',id],queryFn:()=>api<{events:RunEvent[]}>(`/api/scenario-runs/${id}/events`)})
  const scenarios=useQuery({queryKey:['scenarios'],queryFn:()=>api<Record<string,Scenario>>('/api/scenarios')})
  const[selected,setSelected]=useState('')
  const control=useMutation({mutationFn:(name:string)=>api(`/api/scenario-runs/${id}/${name}`,{method:'POST',body:'{}'}),onSuccess:()=>void qc.invalidateQueries({queryKey:['scenario-run',id]})})
  useEffect(()=>{
    if(!id)return
    const refresh=()=>{void qc.invalidateQueries({queryKey:['scenario-run',id]});void qc.invalidateQueries({queryKey:['scenario-run-events',id]})}
    const source=new EventSource(`/api/scenario-runs/${id}/events`)
    for(const kind of['step_started','step_finished','run_paused','run_resumed','run_finished','artifact_recorded','policy_blocked','policy_pause','secret_used','manual_action','resume_point'])source.addEventListener(kind,refresh)
    source.addEventListener('end',()=>{source.close();refresh()})
    return ()=>source.close()
  },[id,qc])
  const run=detail.data?.run
  const steps=detail.data?.steps||[]
  const artifacts=detail.data?.artifacts||[]
  const chosen=steps.find(step=>step.ID===selected)||steps.filter(step=>step.Status==='failed').at(-1)||steps.at(-1)
  const stepEvents=(events.data?.events||[]).filter(event=>chosen&&event.StepID===chosen.ID)
  const stepArtifacts=artifacts.filter(artifact=>chosen&&artifact.StepID===chosen.ID)
  const scenario=detail.data?scenarios.data?.[detail.data.run.Scenario]:undefined
  const statusByNode=new Map<string,Step>()
  for(const step of steps)statusByNode.set(step.NodeID,step)
  if(detail.isPending)return <Page><LoadingState label="Loading run…"/></Page>
  if(detail.error)return <Page><ErrorAlert error={detail.error}/></Page>
  if(!run)return null
  const nodes=scenario?.nodes||[]
  const route=nodes.find(node=>node.type==='navigate'&&node.url)?.url
  const passed=steps.filter(step=>step.Status==='passed').length
  const failedStep=steps.filter(step=>step.Status==='failed').at(-1)
  const isSetup=setupFailure(fmtError(detail.data)||failedStep?.Error||'')
  // Checks are assertions, not step executions — count them separately so a
  // navigate-plus-two-asserts scenario reports 2/2 assertions, not 3/3 steps.
  const assertNodes=nodes.filter(node=>node.type==='assert')
  const assertPassed=assertNodes.filter(node=>statusByNode.get(node.id)?.Status==='passed').length
  const outcome=isSetup?'Browser setup failed — the run could not start'
    :run.Status==='completed'&&assertNodes.length?`${assertPassed}/${assertNodes.length} assertions passed`
    :run.Status==='completed'?`${passed}/${nodes.length||steps.length} steps completed`
    :run.Status==='failed'&&failedStep?`Check "${failedStep.NodeID}" failed`
    :run.Status==='failed'?'Run failed'
    :live(run.Status)?`${run.Status} — ${passed} step${passed===1?'':'s'} done`
    :run.Status
  return <Page><PageHeader title={scenario?.title||`Run ${run.Scenario}`} description={<span data-testid="run-outcome">{outcome} · {duration(run.StartedAt,run.FinishedAt)}{route&&<> · {route}</>}</span>} context={<Link className="text-sm text-muted-foreground underline" to="/studio/runs">All runs</Link>} action={<div className="flex items-center gap-2"><StatusIndicator status={statusKind(run.Status)} label={run.Status}/>{run.Status==='running'&&<Button variant="outline" data-testid="pause-run" disabled={control.isPending} onClick={()=>control.mutate('pause')}>Pause</Button>}{run.Status==='paused'&&<Button data-testid="resume-run" disabled={control.isPending} onClick={()=>control.mutate('resume')}>Resume</Button>}{live(run.Status)&&<Button variant="destructive" data-testid="stop-run" disabled={control.isPending} onClick={()=>control.mutate('stop')}>Stop</Button>}{(run.Status==='failed'||run.Status==='cancelled'||run.Status==='completed')&&<Button variant="outline" data-testid="retry-run" disabled={retry.isPending} onClick={()=>retry.mutate()}>Retry run</Button>}{!isSetup&&steps.length>0&&<Button variant="outline" data-testid="save-as-test" disabled={saveAsTest.isPending} onClick={()=>saveAsTest.mutate()}>Save as test</Button>}{!isSetup&&run.Status==='failed'&&<Button variant="outline" data-testid="propose-repair" disabled={repair.isPending} onClick={()=>{setRepairDraft(null);repair.mutate(undefined,{onSuccess:setRepairDraft})}}>{repair.isPending?'Repairing…':'Propose repair'}</Button>}</div>}/>
    {control.isError&&<ErrorAlert error={control.error}/>}
    {saveAsTest.isError&&<ErrorAlert error={saveAsTest.error}/>}
    {repair.isError&&<ErrorAlert error={repair.error}/>}
    {repairDraft&&<SectionCard title="Proposed repair" className="mb-6"><div className="space-y-3" data-testid="repair-draft"><p className="text-sm text-muted-foreground">Agent-proposed correction — review the graph diff, then load the draft into the Scenario editor to save it.</p>{repairDraft.diff.length?<ul className="list-disc space-y-1 pl-5 font-mono text-xs">{repairDraft.diff.map(line=><li key={line}>{line}</li>)}</ul>:<p className="text-sm text-muted-foreground">No node-level changes.</p>}<Button variant="outline" data-testid="load-repair-draft" onClick={()=>{useEditor.getState().set({kind:'Scenario',name:repairDraft.name,spec:JSON.stringify(repairDraft.spec,null,2)});nav('/studio')}}>Review in editor</Button></div></SectionCard>}
    {isSetup?<SectionCard title="Environment setup required" className="mb-6"><div className="space-y-2" data-testid="setup-required"><p className="text-sm">The browser executable is missing — the scenario runner needs Playwright's Chromium before any node can execute.</p><p className="text-sm">Install it with <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">cd browser && bunx playwright install chromium</code>, then use <strong>Retry run</strong> above. The original launcher output stays under Technical details.</p></div></SectionCard>:fmtError(detail.data)&&<ErrorAlert error={fmtError(detail.data)}/>}
    {run.Status==='paused'&&<TakeoverCard id={id}/>}
    <div className="grid gap-6 lg:grid-cols-2">
      <SectionCard title="Check outcome" className="min-w-0"><ol className="space-y-1" data-testid="step-timeline">{nodes.length?nodes.map(node=>{const step=statusByNode.get(node.id);return <li key={node.id}><Button variant="ghost" className={`flex w-full items-center justify-start gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-muted/70 ${chosen?.ID===step?.ID?'bg-muted':''}`} data-testid={`step-${node.id}`} disabled={!step} onClick={()=>step&&setSelected(step.ID)}><StatusIndicator status={step?statusKind(step.Status):'neutral'} label={step?.Status||'not run'}/><span>{stepSentence(node)}</span>{step&&step.Attempt>1&&<span className="text-xs text-muted-foreground">#{step.Attempt}</span>}<span className="ml-auto text-xs text-muted-foreground">{step?duration(step.StartedAt,step.FinishedAt):''}</span></Button></li>}):steps.map(step=><li key={step.ID}><Button variant="ghost" className={`flex w-full items-center justify-start gap-2 rounded px-2 py-1.5 text-left text-sm hover:bg-muted/70 ${chosen?.ID===step.ID?'bg-muted':''}`} data-testid={`step-${step.NodeID}`} onClick={()=>setSelected(step.ID)}><StatusIndicator status={statusKind(step.Status)} label={step.Status}/><span className="font-mono">{step.NodeID}</span>{step.Attempt>1&&<span className="text-xs text-muted-foreground">#{step.Attempt}</span>}<span className="ml-auto text-xs text-muted-foreground">{duration(step.StartedAt,step.FinishedAt)}</span></Button></li>)}{!steps.length&&!nodes.length&&<EmptyState title="Waiting" description="The run has not claimed its first step yet."/>}</ol><p className="mt-3 font-mono text-xs text-muted-foreground">Run {run.ID} · started {stamp(run.StartedAt)}</p></SectionCard>
      <SectionCard title={chosen?`Diagnostics: ${chosen.NodeID}`:'Diagnostics'} className="min-w-0"><div data-testid="step-diagnostics">
        {chosen?.Error&&(setupFailure(chosen.Error)?<details className="mb-3" data-testid="step-error-details"><summary className="cursor-pointer text-xs font-semibold uppercase text-muted-foreground">Launcher output</summary><pre className="mt-1 max-h-48 overflow-auto rounded-md bg-destructive/10 p-3 font-mono text-xs text-destructive" data-testid="step-error">{chosen.Error}</pre></details>:<p className="mb-3 rounded-md bg-destructive/10 p-3 font-mono text-xs text-destructive" data-testid="step-error">{chosen.Error}</p>)}
        {chosen?.Output&&chosen.Output!=='{}'&&<p className="mb-3 rounded-md bg-muted p-3 font-mono text-xs" data-testid="step-output">{chosen.Output}</p>}
        {stepArtifacts.map(artifact=>artifact.ContentType.startsWith('image/')?<a key={artifact.ID} href={`/api/scenario-runs/${id}/artifacts/${artifact.ID}`} target="_blank" rel="noreferrer"><img className="mb-3 w-full rounded-md border" src={`/api/scenario-runs/${id}/artifacts/${artifact.ID}`} alt={`${artifact.Kind} for ${chosen?.NodeID}`} data-testid="step-screenshot"/></a>:<p key={artifact.ID} className="mb-1 text-sm"><a className="font-mono text-xs underline" href={`/api/scenario-runs/${id}/artifacts/${artifact.ID}`} target="_blank" rel="noreferrer">{artifact.Kind}: {artifact.Ref}</a></p>)}
        {stepEvents.length>0&&<details className="mt-3" data-testid="step-events-details"><summary className="cursor-pointer text-xs font-semibold uppercase text-muted-foreground">Step events ({stepEvents.length})</summary><pre className="mt-1 max-h-48 overflow-auto rounded-md bg-muted p-2 text-xs" data-testid="step-events">{stepEvents.map(event=>`#${event.Sequence} ${event.Kind} ${event.Payload}`).join('\n')}</pre></details>}
        {!chosen&&<EmptyState title="No step" description="Step diagnostics appear once the run claims steps."/>}
      </div></SectionCard>
    </div>
    <SectionCard><details data-testid="run-event-log-details"><summary className="cursor-pointer text-sm font-semibold">Technical details · {(events.data?.events||[]).length} events</summary><pre className="mt-2 max-h-72 overflow-auto rounded-md bg-muted p-3 text-xs" data-testid="run-event-log">{(events.data?.events||[]).map(event=>`#${event.Sequence} ${event.Kind} ${event.Payload}`).join('\n')||'Waiting for events…'}</pre></details></SectionCard>
  </Page>
}

// TakeoverCard is the interactive surface for a paused run: the browser
// session stays open under human control, ops post to /manual, and every
// action lands in the run log as a manual_action event. Resume (header)
// hands the session back to the engine, which continues at the recorded
// boundary node — steps never re-run, so assertions see the state the
// human left.
const manualOps:{op:string;fields:string[]}[]=[
  {op:'snapshot',fields:[]},
  {op:'screenshot',fields:[]},
  {op:'navigate',fields:['url']},
  {op:'click',fields:['ref']},
  {op:'fill',fields:['ref','text','secret']},
  {op:'select',fields:['ref','value']},
  {op:'press',fields:['key']},
  {op:'wait',fields:['condition','ref','text']},
  {op:'extract',fields:['ref','as','attribute']},
]
const waitConditions=['navigation','ref_visible','ref_hidden','text_present','url_equals','url_contains']
const extractKinds=['text','value','attribute']

type ManualResult={text?:string;png?:string}

function TakeoverCard({id}:{id:string}){
  const qc=useQueryClient()
  const[op,setOp]=useState('snapshot')
  const[args,setArgs]=useState<Record<string,string>>({})
  const[result,setResult]=useState<ManualResult|null>(null)
  const manual=useMutation({
    mutationFn:(body:Record<string,string>)=>api<Record<string,string>>(`/api/scenario-runs/${id}/manual`,{method:'POST',body:JSON.stringify(body)}),
    onSuccess:(data)=>{
      setResult(data.png?{png:data.png}:{text:JSON.stringify(data,null,2)})
      void qc.invalidateQueries({queryKey:['scenario-run-events',id]})
    },
  })
  const fields=manualOps.find(entry=>entry.op===op)?.fields||[]
  const set=(name:string)=>(event:{target:{value:string}})=>setArgs(prev=>({...prev,[name]:event.target.value}))
  const go=()=>{
    const body:Record<string,string>={op}
    for(const name of fields)if(args[name])body[name]=args[name]
    manual.mutate(body)
  }
  return <SectionCard title="Takeover" className="mb-6"><div className="space-y-3" data-testid="takeover">
    <p className="text-sm text-muted-foreground">Run paused — the browser session is open under your control. Drive it below; every action is logged as <span className="font-mono">manual_action</span>. Resume hands control back to the engine.</p>
    <div className="flex flex-wrap items-end gap-2">
      <Field id="manual-op" label="Op"><NativeSelect id="manual-op" data-testid="manual-op" value={op} onChange={event=>setOp(event.target.value)}>{manualOps.map(entry=><NativeSelectOption key={entry.op} value={entry.op}>{entry.op}</NativeSelectOption>)}</NativeSelect></Field>
      {fields.includes('url')&&<Field id="manual-url" label="URL"><Input id="manual-url" className="w-72" value={args.url||''} onChange={set('url')}/></Field>}
      {fields.includes('ref')&&<Field id="manual-ref" label="Ref"><Input id="manual-ref" className="w-40" value={args.ref||''} onChange={set('ref')}/></Field>}
      {fields.includes('text')&&<Field id="manual-text" label="Text"><Input id="manual-text" className="w-48" value={args.text||''} onChange={set('text')}/></Field>}
      {fields.includes('secret')&&<Field id="manual-secret" label="Secret"><Input id="manual-secret" className="w-36" value={args.secret||''} onChange={set('secret')}/></Field>}
      {fields.includes('value')&&<Field id="manual-value" label="Value"><Input id="manual-value" className="w-32" value={args.value||''} onChange={set('value')}/></Field>}
      {fields.includes('key')&&<Field id="manual-key" label="Key"><Input id="manual-key" className="w-28" value={args.key||''} onChange={set('key')}/></Field>}
      {fields.includes('condition')&&<Field id="manual-condition" label="Condition"><NativeSelect id="manual-condition" data-testid="manual-condition" value={args.condition||'navigation'} onChange={set('condition')}>{waitConditions.map(kind=><NativeSelectOption key={kind} value={kind}>{kind}</NativeSelectOption>)}</NativeSelect></Field>}
      {fields.includes('as')&&<Field id="manual-as" label="As"><NativeSelect id="manual-as" value={args.as||'text'} onChange={set('as')}>{extractKinds.map(kind=><NativeSelectOption key={kind} value={kind}>{kind}</NativeSelectOption>)}</NativeSelect></Field>}
      {fields.includes('attribute')&&<Field id="manual-attribute" label="Attribute"><Input id="manual-attribute" className="w-32" value={args.attribute||''} onChange={set('attribute')}/></Field>}
      <Button data-testid="manual-go" disabled={manual.isPending} onClick={go}>Run op</Button>
    </div>
    {manual.isError&&<ErrorAlert error={manual.error}/>}
    {result?.png&&<img className="max-w-full rounded-md border" src={`data:image/png;base64,${result.png}`} alt="Manual screenshot" data-testid="manual-screenshot"/>}
    {result?.text&&<pre className="max-h-64 overflow-auto rounded-md bg-muted p-3 text-xs" data-testid="manual-result">{result.text}</pre>}
  </div></SectionCard>
}
