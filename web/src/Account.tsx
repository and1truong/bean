import {FormEvent,useRef,useState} from 'react'
import {Link,useNavigate} from 'react-router-dom'
import {useQuery,useQueryClient} from '@tanstack/react-query'
import {api,Session} from './api'
import {ErrorAlert,Field,LoadingState,Page,PageHeader} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Card,CardContent,CardHeader,CardTitle} from '@/components/ui/card'
import {AlertDialog,AlertDialogAction,AlertDialogCancel,AlertDialogContent,AlertDialogDescription,AlertDialogFooter,AlertDialogHeader,AlertDialogTitle,AlertDialogTrigger} from '@/components/ui/alert-dialog'

export function Account(){
  const session=useQuery({queryKey:['session'],queryFn:()=>api<Session>('/api/system/session')})
  const client=useQueryClient();const navigate=useNavigate();const inFlight=useRef(false)
  const[currentPassword,setCurrentPassword]=useState('');const[password,setPassword]=useState('');const[confirmation,setConfirmation]=useState('')
  const[pending,setPending]=useState(false);const[error,setError]=useState('')
  async function run(operation:'password'|'sessions/revoke'){
    if(inFlight.current)return
    inFlight.current=true;setPending(true);setError('')
    try{
      // Restore the session's token even on direct refresh, before mutation.
      if(session.data?.csrfToken)sessionStorage.setItem('bean_csrf',session.data.csrfToken)
      await api('/api/auth/'+operation,{method:'POST',body:JSON.stringify(operation==='password'?{currentPassword,password,confirmation}:{})})
      setCurrentPassword('');setPassword('');setConfirmation('')
      sessionStorage.removeItem('bean_csrf')
      await client.cancelQueries();client.clear()
      navigate('/login?notice='+ (operation==='password'?'password-changed':'sessions-revoked'),{replace:true})
    }catch(cause){setError((cause as Error).message)}
    finally{inFlight.current=false;setPending(false)}
  }
  function submit(event:FormEvent){event.preventDefault();void run('password')}
  if(session.isPending)return <Page narrow><LoadingState/></Page>
  if(session.error)return <Page narrow><ErrorAlert error={session.error}/></Page>
  if(!session.data?.authenticated)return <Page narrow><PageHeader title="Account security"/><Button asChild><Link to="/login?next=%2Fadmin%2Fsystem%2Faccount">Sign in to manage your account</Link></Button></Page>
  return <Page narrow><PageHeader title="Account security" description={session.data.user?.Email}/>
    {error&&<ErrorAlert error={error}/>}
    <Card><CardHeader><CardTitle>Change password</CardTitle></CardHeader><CardContent>
      <p className="mb-4 text-sm text-muted-foreground">Changing your password signs you out on every device, including this one.</p>
      <form className="space-y-4" onSubmit={submit} aria-busy={pending}>
        <Field id="account-current" label="Current password"><Input id="account-current" name="currentPassword" type="password" autoComplete="current-password" required disabled={pending} value={currentPassword} onChange={e=>setCurrentPassword(e.target.value)}/></Field>
        <Field id="account-new" label="New password"><Input id="account-new" name="password" type="password" autoComplete="new-password" required disabled={pending} value={password} onChange={e=>setPassword(e.target.value)}/></Field>
        <Field id="account-confirm" label="Confirm new password"><Input id="account-confirm" name="confirmation" type="password" autoComplete="new-password" required disabled={pending} value={confirmation} onChange={e=>setConfirmation(e.target.value)}/></Field>
        <p className="text-sm text-muted-foreground">Use 10–72 bytes. Non-ASCII characters can use more than one byte.</p>
        <Button type="submit" disabled={pending}>{pending?'Working…':'Change password'}</Button>
      </form>
    </CardContent></Card>
    <Card><CardHeader><CardTitle>Sessions</CardTitle></CardHeader><CardContent>
      <AlertDialog><AlertDialogTrigger asChild><Button variant="outline" disabled={pending}>Sign out all devices</Button></AlertDialogTrigger>
        <AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Sign out all devices?</AlertDialogTitle><AlertDialogDescription>You will need to sign in again on this device too. Your password will not change.</AlertDialogDescription></AlertDialogHeader>
          <AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction onClick={()=>void run('sessions/revoke')}>Sign out everywhere</AlertDialogAction></AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </CardContent></Card>
  </Page>
}
