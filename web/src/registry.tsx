import {createContext,FormEvent,useContext,useEffect,useState} from 'react'
import {Link,useNavigate,useSearchParams} from 'react-router-dom'
import type {Node,PageFilter,ViewControl,ViewDisplay,ViewFilter,ViewPresentation} from './api'
import {ActiveFilters,Field,FilterBar} from '@/components/bean'
import {Button} from '@/components/ui/button'
import {Checkbox} from '@/components/ui/checkbox'
import {Input} from '@/components/ui/input'
import {Label} from '@/components/ui/label'
import {NativeSelect,NativeSelectOption} from '@/components/ui/native-select'
import {RouteTabsLink,RouteTabsList,type RouteTabsVariant} from '@/components/ui/route-tabs'

// The semantic render surface shared by the server-rendered application and
// the browser playground. Renderers are supplied through RenderersContext so
// the playground can substitute a backend-free subset.

export type RenderProps={
  Page:{title?:string;description?:string;protected?:boolean;filters?:Record<string,PageFilter>}
  Panel:{layout?:string;pageWidth?:'contained'|'wide'|'full'}
  Region:{name?:string;expanded?:boolean}
  TextBlock:{text?:string}
  ContentBlock:{content?:import('./api').ContentElement[]}
  TabsBlock:{label?:string;orientation?:'horizontal'|'vertical';variant?:'underline'|'pills';tabs?:import('./api').ContentTab[]}
  LessonBlock:{title?:string;sections?:import('./api').LessonSection[]}
  TimelineBlock:{title?:string;entries?:import('./api').TimelineEntry[]}
  MindmapBlock:{root?:import('./api').MindMapNode}
  FlashcardBlock:{title?:string;cards?:import('./api').Flashcard[]}
  Sequence:{title?:string;description?:string;profile?:string;aspectRatio?:string;protected?:boolean}
  ViewBlock:{name?:string;view?:string;display?:ViewDisplay;displayName?:string;displays?:Record<string,ViewDisplay>;filters?:Record<string,ViewFilter>;pageFilters?:Record<string,string>;fieldTypes?:Record<string,string>;presentation?:ViewPresentation;searchFields?:string[];formattedFields?:string[];fileFields?:string[];maxRows?:number}
  EntityBlock:{name?:string;entity?:string;presentation?:ViewPresentation;formattedFields?:string[];fileFields?:string[]}
  ResourceListBlock:{name?:string;resource?:string;view?:string;filters?:string[];defaultFilters?:Record<string,any>}
  WebformBlock:{name?:string;webform?:string;form?:import('./api').Manifest['webforms'][string]}
  ActionBlock:{action?:string}
  MenuBlock:{name?:string;menu?:string;profile?:string;variant?:RouteTabsVariant;ownerEntity?:string;ownerID?:string;inputs?:Record<string,any>;items?:MenuItem[];content?:React.ReactNode}
  UnsupportedBlock:{name?:string;component?:string;reason?:string}
}
export type RenderComponent=keyof RenderProps
export type NodeRenderer<K extends RenderComponent>=(props:RenderProps[K],children?:Node[])=>React.ReactNode
export type Renderers={[K in RenderComponent]?:NodeRenderer<K>}

const RenderersContext=createContext<Renderers|null>(null)
export function RenderersProvider({renderers,children}:{renderers:Renderers;children:React.ReactNode}){return <RenderersContext.Provider value={renderers}>{children}</RenderersContext.Provider>}
export function useRenderers():Renderers{const renderers=useContext(RenderersContext);if(!renderers)throw new Error('Renderer requires a RenderersProvider');return renderers}

export type Row=Record<string,any>
export const noPageFilters:Record<string,string>={}
export const PageFilterValues=createContext<Record<string,string>>(noPageFilters)

export function Renderer({node}:{node:Node}){
  const renderers=useRenderers()
  if(!isRenderComponent(renderers,node.component))return <section role="alert" data-component={node.component}>Unsupported render component: {node.component}</section>
  const renderer=renderers[node.component] as NodeRenderer<RenderComponent>
  return renderer(node.props||{},node.children)
}
function isRenderComponent(renderers:Renderers,value:string):value is RenderComponent{return Object.hasOwn(renderers,value)}
export function renderNodes(nodes:Node[]){return nodes.map((node,index)=><Renderer key={String(node.props?.pageSection??index)} node={node}/>)}
export function RenderChildren({nodes}:{nodes?:Node[]}){if(!nodes?.length)return null;const menuIndex=nodes.findIndex((node,index)=>index<nodes.length-1&&workspaceMenuNode(node));if(menuIndex<0)return <>{renderNodes(nodes)}</>;return <>{renderNodes(nodes.slice(0,menuIndex))}<WorkspaceMenuNode node={nodes[menuIndex]} contentNodes={nodes.slice(menuIndex+1)}/></>}
export function RenderPanelChildren({nodes}:{nodes?:Node[]}){if(!nodes?.length)return null;for(let regionIndex=0;regionIndex<nodes.length;regionIndex++){const region=nodes[regionIndex];if(region.component!=='Region'||!region.children?.length)continue;const menuIndex=region.children.findIndex(workspaceMenuNode);if(menuIndex<0)continue;const remainingRegionChildren=region.children.filter((_,index)=>index!==menuIndex);const contentNodes=nodes.flatMap((node,index)=>index!==regionIndex?[node]:remainingRegionChildren.length?[{...region,children:remainingRegionChildren}]:[]);if(contentNodes.length)return <WorkspaceMenuNode node={region.children[menuIndex]} contentNodes={contentNodes}/>};return <RenderChildren nodes={nodes}/>}
function WorkspaceMenuNode({node,contentNodes}:{node:Node;contentNodes:Node[]}){
  const renderers=useRenderers()
  const content=<div className="bean-workspace-content">{renderNodes(contentNodes)}</div>
  if(!renderers.MenuBlock)return content
  return <>{renderers.MenuBlock({...(node.props||{}),content} as RenderProps['MenuBlock'])}</>
}
export function workspaceMenuNode(node:Node){return node.component==='MenuBlock'&&node.props?.profile==='workspace'}

export function StructuralNode({component,title,layout,name,expanded,pageWidth,children}:{component:'Page'|'Panel'|'Region';title?:string;layout?:string;name?:string;expanded?:boolean;pageWidth?:'contained'|'wide'|'full';children?:Node[]}){useEffect(()=>{if(component!=='Page'||!title)return;const previous=document.title;document.title=title;return()=>{document.title=previous}},[component,title]);const className=component==='Panel'?'bean-panel':component==='Region'?'bean-region':'space-y-4';return <section className={className} data-component={component} data-layout={layout} data-region={name} data-expanded={expanded||undefined} data-page-width={pageWidth}>{title&&<h2 className="font-heading text-2xl font-semibold">{title}</h2>}{component==='Panel'?<RenderPanelChildren nodes={children}/>:<RenderChildren nodes={children}/>}</section>}

export function PageNode({title,description,filters,children}:{title?:string;description?:string;filters:Record<string,PageFilter>;children?:Node[]}){const[urlParams,setURLParams]=useSearchParams();const names=Object.keys(filters);const[state,setState]=useState<Record<string,string>>(()=>Object.fromEntries(names.map(name=>{const value=urlParams.get(name)??String(filters[name].Default??'');return[name,controlInputValue(value,{Type:filters[name].Type})]})));useEffect(()=>{if(!title)return;const previous=document.title;document.title=title;return()=>{document.title=previous}},[title]);const urlState=urlParams.toString();const filterState=JSON.stringify(filters);useEffect(()=>{const definitions=JSON.parse(filterState)as Record<string,PageFilter>;const current=new URLSearchParams(urlState);setState(Object.fromEntries(Object.keys(definitions).map(name=>{const value=current.get(name)??String(definitions[name].Default??'');return[name,controlInputValue(value,{Type:definitions[name].Type})]})))},[filterState,urlState]);const values=Object.fromEntries(names.map(name=>[name,urlParams.get(name)??String(filters[name].Default??'')]));const apply=(event:FormEvent)=>{event.preventDefault();const next=new URLSearchParams(urlParams);for(const name of names){const definition={Type:filters[name].Type};const original=urlParams.get(name)??String(filters[name].Default??'');const value=controlQueryValue(state[name]??'',definition,original);if(value||filters[name].Default!==undefined&&filters[name].Default!==null)next.set(name,value);else next.delete(name)}setURLParams(next,{replace:true})};const active=names.filter(name=>values[name]).map(name=>({key:name,label:filters[name].Label||humanize(name),value:values[name]}));const remove=(name:string)=>{const next=new URLSearchParams(urlParams);if(filters[name].Default!==undefined&&filters[name].Default!==null)next.set(name,'');else next.delete(name);setState(current=>({...current,[name]:''}));setURLParams(next,{replace:true})};const clear=()=>{const next=new URLSearchParams(urlParams);for(const name of names){if(filters[name].Default!==undefined&&filters[name].Default!==null)next.set(name,'');else next.delete(name)}setState(Object.fromEntries(names.map(name=>[name,''])));setURLParams(next,{replace:true})};return <PageFilterValues.Provider value={values}><section className="bean-page" data-component="Page">{(title||description||names.length>0)&&<div className="bean-page-chrome space-y-3">{title&&<h1 className="bean-page-title">{title}</h1>}{description&&<p className="bean-page-description">{description}</p>}{names.length?<FilterBar label="Page filters" onSubmit={apply}>{names.map(name=><ViewFilterControl key={name} scope="page" control={{Filter:name,Label:filters[name].Label,Widget:filters[name].Widget}} filter={{Type:filters[name].Type,Options:filters[name].Options}} value={state[name]??''} onChange={value=>setState(current=>({...current,[name]:value}))}/>)}<Button type="submit">Apply filters</Button><ActiveFilters filters={active} onRemove={remove} onClear={clear}/></FilterBar>:null}</div>}<RenderChildren nodes={children}/></section></PageFilterValues.Provider>}

export function ViewFilterControl({scope,control,filter,value,onChange}:{scope:string;control:ViewControl;filter?:ViewFilter;value:string;onChange:(value:string)=>void}){const id='view-filter-'+scope+'-'+control.Filter;const label=control.Label||filter?.Label||humanize(control.Filter);const widget=control.Widget==='auto'||!control.Widget?(filter?.Type==='enum'?'select':filter?.Type==='boolean'?'checkbox':filter?.Type==='integer'||filter?.Type==='decimal'||filter?.Type==='money'?'number':filter?.Type==='date'||filter?.Type==='datetime'?'date':'text'):control.Widget;if(widget==='select')return <Field id={id} label={label}><NativeSelect id={id} value={value} onChange={event=>onChange(event.target.value)}><NativeSelectOption value="">All</NativeSelectOption>{filter?.Options?.map(option=><NativeSelectOption key={option} value={option}>{humanize(option)}</NativeSelectOption>)}</NativeSelect></Field>;if(widget==='checkbox')return <div className="flex items-center gap-2"><Checkbox id={id} checked={value==='true'} onCheckedChange={checked=>onChange(checked?'true':'false')}/><Label htmlFor={id}>{label}</Label></div>;const type=widget==='number'?'number':widget==='date'&&filter?.Type==='datetime'?'datetime-local':widget==='date'?'date':'text';return <Field id={id} label={label}><Input id={id} type={type} step={filter?.Type==='decimal'?'any':type==='datetime-local'?'1':undefined} value={value} onChange={event=>onChange(event.target.value)}/></Field>}

export type MenuItem={ID?:string;Label:string;Route:string;Level?:number;Current?:boolean;Active?:boolean;Children?:MenuItem[]}
export function WorkspaceMenuFrame({menu,variant,primary=[],secondary=[],tertiary=[],navigate,content,status}:{menu:string;variant:RouteTabsVariant;primary?:MenuItem[];secondary?:MenuItem[];tertiary?:MenuItem[];navigate?:ReturnType<typeof useNavigate>;content?:React.ReactNode;status?:React.ReactNode}){const hasTertiary=tertiary.length>0;return <div className="bean-workspace-menu" data-menu={menu} data-has-tertiary={hasTertiary||undefined}><div className="bean-workspace-menu-header">{status||<><MenuLevel label="Primary navigation" items={primary} variant={variant}/>{secondary.length?<MenuLevel label="Secondary navigation" items={secondary} variant={variant}/>:null}</>}</div>{hasTertiary?<div className="bean-menu-tertiary-mobile"><Field id={'menu-'+menu+'-section'} label="Section"><NativeSelect id={'menu-'+menu+'-section'} value={tertiary.find(item=>item.Current)?.Route||''} onChange={event=>navigate?.(event.target.value)}><NativeSelectOption value="" disabled>Select section…</NativeSelectOption>{tertiary.map(item=><NativeSelectOption key={item.ID||item.Route} value={item.Route}>{item.Label}</NativeSelectOption>)}</NativeSelect></Field></div>:null}{content?<div className="bean-workspace-menu-body" data-has-tertiary={hasTertiary||undefined}>{hasTertiary?<div className="bean-menu-tertiary-desktop"><MenuLevel label="Section navigation" items={tertiary} variant={variant} vertical/></div>:null}{content}</div>:hasTertiary?<div className="bean-menu-tertiary-desktop"><MenuLevel label="Section navigation" items={tertiary} variant={variant} vertical/></div>:null}</div>}
export function MenuLevel({label,items,variant,vertical=false}:{label:string;items:MenuItem[];variant:RouteTabsVariant;vertical?:boolean}){const orientation=vertical?'vertical':'horizontal';return <RouteTabsList aria-label={label} variant={variant} orientation={orientation}>{items.map(item=><RouteTabsLink asChild active={Boolean(item.Active)} variant={variant} orientation={orientation} key={item.ID||item.Route}><Link aria-current={item.Current?'page':undefined} to={item.Route}>{item.Label}</Link></RouteTabsLink>)}</RouteTabsList>}

export function controlInputValue(value:string,filter?:ViewFilter){if(!value||filter?.Type!=='datetime')return value;const date=new Date(value);if(Number.isNaN(date.valueOf()))return value;return new Date(date.valueOf()-date.getTimezoneOffset()*60_000).toISOString().slice(0,19)}
export function controlQueryValue(value:string,filter?:ViewFilter,original=''){if(!value||filter?.Type!=='datetime')return value;if(original&&controlInputValue(original,filter)===value)return original;const date=new Date(value);return Number.isNaN(date.valueOf())?value:date.toISOString()}
export function humanize(value:string){return value.replaceAll('_',' ').replace(/^./,letter=>letter.toUpperCase())}
