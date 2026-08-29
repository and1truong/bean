import {APIRequestContext} from '@playwright/test'
export async function login(request:APIRequestContext,baseURL:string){const response=await request.post(baseURL+'/api/auth/login',{data:{email:'admin@example.test',password:'test-password'}});if(!response.ok())throw new Error(await response.text());return (await response.json()).csrfToken as string}
export async function action(request:APIRequestContext,baseURL:string,csrf:string,name:string,data:Record<string,unknown>){return request.post(baseURL+'/api/actions/'+name,{headers:{'X-CSRF-Token':csrf},data})}
