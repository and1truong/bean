import {test,expect} from '@playwright/test'
import {execFileSync,spawn,type ChildProcess} from 'node:child_process'
import {appendFileSync,cpSync,mkdtempSync,readFileSync,rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {join,resolve} from 'node:path'
import net from 'node:net'
import tls from 'node:tls'
import {randomBytes} from 'node:crypto'

// A local STARTTLS SMTP sink exercises the actual host transport and durable
// outbox. Its temporary CA is trusted only by this test's host mail configuration.
test('forgot password delivers a fragment link and resets through the browser',async({page},testInfo)=>{
  const root=resolve(import.meta.dirname,'..');const binary=join(root,'bin/bean')
  const dir=mkdtempSync(join(tmpdir(),'bean-recovery-e2e-'));const messages:string[]=[]
  let child:ChildProcess|undefined
  const sockets=new Set<net.Socket>()
  execFileSync('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-keyout',join(dir,'key.pem'),'-out',join(dir,'cert.pem'),'-days','1','-subj','/CN=127.0.0.1','-addext','subjectAltName=IP:127.0.0.1'],{stdio:'ignore'})
  const secureContext=tls.createSecureContext({key:readFileSync(join(dir,'key.pem')),cert:readFileSync(join(dir,'cert.pem'))})
  const smtp=net.createServer(socket=>{
    sockets.add(socket);socket.on('close',()=>sockets.delete(socket));socket.on('error',()=>{})
    socket.write('220 localhost ESMTP\r\n')
    function handle(connection:net.Socket){
      let buffer='';let data=false;let body=''
      const receive=(chunk:Buffer)=>{
        buffer+=chunk.toString()
        while(buffer.includes('\r\n')){
          const end=buffer.indexOf('\r\n');const line=buffer.slice(0,end);buffer=buffer.slice(end+2)
          if(data){if(line==='.') {messages.push(body);body='';data=false;connection.write('250 accepted\r\n')}else body+=line+'\n';continue}
          if(line.startsWith('EHLO'))connection.write('250-localhost\r\n250 STARTTLS\r\n')
          else if(line==='STARTTLS'){
            connection.removeListener('data',receive);connection.write('220 begin TLS\r\n')
            const secure=new tls.TLSSocket(connection,{isServer:true,secureContext});secure.on('error',()=>{});handle(secure);return
          }else if(line==='DATA'){data=true;connection.write('354 send message\r\n')}
          else if(line==='QUIT')connection.end('221 bye\r\n')
          else connection.write('250 ok\r\n')
        }
      }
      connection.on('data',receive)
    }
    handle(socket)
  })
  try{
    await new Promise<void>(resolve=>smtp.listen(0,'127.0.0.1',resolve))
    const smtpPort=(smtp.address() as net.AddressInfo).port
    const port=19200+testInfo.workerIndex;const origin=`http://127.0.0.1:${port}`
    const env={...process.env,BEAN_AUTH_EMAIL:JSON.stringify({address:`127.0.0.1:${smtpPort}`,from:'bean@example.test',origin,key:randomBytes(32).toString('base64'),rootCAFile:join(dir,'cert.pem')})}
    const db=join(dir,'bean.db');const source=join(dir,'blog');cpSync(join(root,'examples/blog'),source,{recursive:true})
    appendFileSync(join(source,'access.yaml'),'\n---\nkind: Authentication\nname: auth\npreset: public\npasswordRecovery: true\nregistration: true\n')
    for(const args of [['init','--db',db],['app','import','--db',db,'--file',join(source,'app.yaml')],['publish','--db',db]])execFileSync(binary,args,{env,stdio:'pipe'})
    child=spawn(binary,['serve','--db',db,'--addr',`127.0.0.1:${port}`],{env,stdio:['ignore','pipe','pipe']})
    let serverOutput='';child.stderr!.on('data',chunk=>{serverOutput+=String(chunk)})
    await expect.poll(async()=>{try{if(child!.exitCode!==null)throw new Error(serverOutput);return (await fetch(origin+'/healthz')).ok}catch(error){if(child!.exitCode!==null)throw error;return false}}).toBe(true)
    await page.goto(origin+'/login');await page.getByRole('link',{name:'Forgot password?'}).click()
    await expect(page.getByRole('button',{name:'Send reset link',exact:true})).toBeEnabled()
    await page.getByLabel('Email',{exact:true}).fill('admin@example.test')
    await expect(page.getByLabel('Email',{exact:true})).toHaveValue('admin@example.test')
    await page.getByRole('button',{name:'Send reset link',exact:true}).click()
    await expect(page.getByRole('status')).toContainText('If this address belongs to an account')
    await expect.poll(()=>messages.length,{timeout:15000}).toBe(1)
    const link=messages[0].split('\n').find(line=>line.startsWith(origin+'/login?recovery=reset#token='))!
    expect(link).toBeTruthy();const token=new URL(link).hash.slice('#token='.length)
    await page.goto(link)
    await expect(page).toHaveURL(origin+'/login?recovery=reset')
    expect(await page.evaluate(()=>JSON.stringify({...sessionStorage,...localStorage}))).not.toContain(token)
    await page.getByLabel('New password',{exact:true}).fill('recovered-password')
    await page.getByLabel('Confirm new password',{exact:true}).fill('recovered-password')
    await page.getByRole('button',{name:'Reset password',exact:true}).click()
    await expect(page).toHaveURL(origin+'/login?notice=password-changed')
    await page.getByLabel('Email',{exact:true}).fill('admin@example.test')
    await page.getByLabel('Password',{exact:true}).fill('recovered-password')
    await page.getByTestId('login').click();await expect(page).toHaveURL(origin+'/admin')
    const replay=await page.request.post(origin+'/api/auth/recovery/reset',{data:{token,password:'other-password',confirmation:'other-password'}})
    expect(replay.status()).toBe(400)
  }finally{
    if(child){child.kill('SIGTERM');await new Promise<void>(resolve=>{if(child!.exitCode!==null)resolve();else child!.once('exit',()=>resolve())})}
    for(const socket of sockets)socket.destroy()
    await new Promise<void>(resolve=>smtp.close(()=>resolve()))
    rmSync(dir,{recursive:true,force:true})
  }
})
