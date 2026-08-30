export type Expression={Op:string;Left?:{Source:string;Name?:string;Literal?:any};Right?:{Source:string;Name?:string;Literal?:any};Args?:Expression[]}
export type FormElement={Name:string;Type:string;Required:boolean;Options?:string[];Children?:FormElement[];Visible?:Expression;RequiredWhen?:Expression}
export type Field={Name:string;Label:string;Type:string;Required:boolean;Sensitive?:boolean;Options?:string[];Relation?:{Entity:string;Kind:string;TargetField:string}}
export type Entity={Name:string;Label:string;Fields:Field[]}
export type Action={Name:string;Entity:string;Operation:string;StateField?:string;Confirm?:string;Input:Record<string,Field>}
export type ContentFilter={Name:string;Steps:Array<{Type:string}>}
export type AdminResource={Name:string;Entity:string;Label:string;Description:string;LabelField:string;View:string;CreateAction:string;UpdateAction:string;DeleteAction:string;List:{Columns:string[];Search:string[];Filters:string[];Sort:Array<{Field:string;Desc:boolean}>;PageSize:number};Form:{Fields:string[];Readonly:string[]};Actions:string[]}
export type AdminManifest={appId:string;releaseId:string;version:number;systemAdmin:boolean;entities:Record<string,Entity>;actions:Record<string,Action>;adminResources:Record<string,AdminResource>}
export type Manifest={appId:string;releaseId:string;version:number;entities:Record<string,Entity>;views:Record<string,unknown>;actions:Record<string,Action>;filters:Record<string,ContentFilter>;webforms:Record<string,{Name:string;Elements:FormElement[];Steps?:string[][];Confirmation?:string}>;pages:Record<string,unknown>;localRegistration?:{Action:string};renderTree?:Node}
export type Session={authenticated:boolean;csrfToken?:string;user?:{ID:string;Email:string;DisplayName:string;Roles:string[]}}
export type ViewPresentation={Mode?:string;TitleField?:string;BodyField?:string;LinkRoute?:string;LinkField?:string;EmptyState?:string;MetaFields?:string[];RichTextFields?:string[]}
export type Node={component:string;props?:Record<string,any>;children?:Node[]}
export class APIError extends Error{fields?:Record<string,string>;constructor(message:string,fields?:Record<string,string>){super(message);this.fields=fields}}
export async function api<T>(path:string,init?:RequestInit):Promise<T>{const headers=new Headers(init?.headers);if(init?.body)headers.set('Content-Type','application/json');const csrf=sessionStorage.getItem('bean_csrf');if(csrf)headers.set('X-CSRF-Token',csrf);const response=await fetch(path,{...init,headers});const body=await response.json().catch(()=>({}));if(!response.ok)throw new APIError(body?.error?.message||response.statusText,body?.error?.fields);return body as T}
