import {FormEvent,useEffect,useRef,useState} from 'react'
import {Link,useLocation,useNavigate} from 'react-router-dom'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {api,Manifest} from './api'
import {ErrorAlert,Field,LoadingState,Page,PageHeader} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Card,CardContent} from '@/components/ui/card'

export function Recovery({mode}:{mode:'request'|'reset'}){
  const location=useLocation();const navigate=useNavigate();const client=useQueryClient()
  const manifest=useQuery({queryKey:['manifest'],queryFn:()=>api<Manifest>('/api/system/manifest')})
  const[token,setToken]=useState(()=>new URLSearchParams(location.hash.slice(1)).get('token')||'')
  const[email,setEmail]=useState('');const[password,setPassword]=useState('');const[confirmation,setConfirmation]=useState('')
  const[pending,setPending]=useState(false);const[error,setError]=useState('');const[sent,setSent]=useState(false);const inFlight=useRef(false)
  // Keep the bearer token only in component memory. Never persist it to browser
  // storage, copy it to a query parameter or redeem it on GET/mount.
  useEffect(()=>{if(location.hash)navigate(location.pathname+location.search,{replace:true})},[location.hash,location.pathname,location.search,navigate])
  async function submit(event:FormEvent){
    event.preventDefault()
    if(inFlight.current)return
    inFlight.current=true;setPending(true);setError('')
    try{
      await api('/api/auth/recovery/'+mode,{method:'POST',body:JSON.stringify(mode==='request'?{email}:{token,password,confirmation})})
      if(mode==='request'){setSent(true);setEmail('')}
      else{
        setToken('');setPassword('');setConfirmation('');sessionStorage.removeItem('bean_csrf')
        await client.cancelQueries();client.clear()
        navigate('/login?notice=password-changed',{replace:true})
      }
    }catch(cause){setError((cause as Error).message)}
    finally{inFlight.current=false;setPending(false)}
  }
  if(manifest.isPending)return <Page narrow><LoadingState/></Page>
  if(manifest.error)return <Page narrow><ErrorAlert error={manifest.error}/></Page>
  if(!manifest.data?.authentication?.PasswordRecovery)return <Page narrow><PageHeader title="Password recovery is unavailable"/><Link to="/login">Back to sign in</Link></Page>
  return <Page narrow><PageHeader title={mode==='request'?'Forgot password':'Reset password'}/><Card><CardContent className="pt-6">
    {sent?<p role="status">If this address belongs to an account, a reset link will be sent. Check your inbox and spam folder.</p>:mode==='reset'&&!token?<p role="alert">This reset link is missing its token. Request a new link.</p>:<form className="space-y-4" onSubmit={submit} aria-busy={pending}>
      {mode==='request'?<Field id="recovery-email" label="Email"><Input id="recovery-email" name="email" type="email" autoComplete="username" required disabled={pending} value={email} onChange={e=>setEmail(e.target.value)}/></Field>:<>
        <p className="text-sm text-muted-foreground">Choose a password of 10–72 bytes. All sessions for this account will be signed out.</p>
        <Field id="recovery-password" label="New password"><Input id="recovery-password" name="password" type="password" autoComplete="new-password" required disabled={pending} value={password} onChange={e=>setPassword(e.target.value)}/></Field>
        <Field id="recovery-confirmation" label="Confirm new password"><Input id="recovery-confirmation" name="confirmation" type="password" autoComplete="new-password" required disabled={pending} value={confirmation} onChange={e=>setConfirmation(e.target.value)}/></Field>
      </>}
      {error&&<ErrorAlert error={error}/>}
      <Button type="submit" disabled={pending||manifest.isFetching}>{pending?'Working…':mode==='request'?'Send reset link':'Reset password'}</Button>
    </form>}
    <div className="mt-4 flex gap-4 text-sm"><Link to="/login">Back to sign in</Link>{mode==='reset'&&<Link to="/login?recovery=request">Request a new link</Link>}</div>
  </CardContent></Card></Page>
}
