import {Link,useNavigate} from 'react-router-dom'
import {ContentBlock} from '../Content'
import {FlashcardBlock} from '../Flashcard'
import {LessonBlock} from '../Lesson'
import {MindmapBlock} from '../Mindmap'
import {SequenceView} from '../Sequence'
import {TabsBlock} from '../Tabs'
import {TimelineBlock} from '../Timeline'
import {MenuItem,PageNode,Renderer,StructuralNode,WorkspaceMenuFrame,type Renderers} from '../registry'
import type {RouteTabsVariant} from '@/components/ui/route-tabs'
import {Button} from '@/components/ui/button'

// The browser-preview capability profile: literal semantic content, supported
// blocks, tabs, static pages/sequences, and interactive learning components.
// Backend-dependent render nodes never arrive here — the bridge rewrites them
// to UnsupportedBlock — but if one ever does, Renderer reports it explicitly.

export const playgroundRenderers:Renderers={
  Page:(props,children)=><PageNode title={props.title} description={props.description} filters={props.filters||{}} children={children}/>,
  Panel:(props,children)=><StructuralNode component="Panel" layout={props.layout} pageWidth={props.pageWidth} children={children}/>,
  Region:(props,children)=><StructuralNode component="Region" name={props.name} expanded={props.expanded} children={children}/>,
  TextBlock:props=><p>{props.text}</p>,
  ContentBlock:props=><ContentBlock content={props.content||[]}/>,
  TabsBlock:props=><TabsBlock label={props.label||''} orientation={props.orientation} variant={props.variant} tabs={props.tabs||[]}/>,
  LessonBlock:props=><LessonBlock title={props.title||''} sections={props.sections||[]}/>,
  TimelineBlock:props=><TimelineBlock title={props.title||''} entries={props.entries||[]}/>,
  MindmapBlock:props=><MindmapBlock root={props.root||{id:'',Label:''}}/>,
  FlashcardBlock:props=><FlashcardBlock title={props.title||''} cards={props.cards||[]}/>,
  Sequence:(props,children)=><SequenceView {...props} children={children} renderNode={node=><Renderer node={node}/>}/>,
  MenuBlock:props=><StaticMenuBlock name={props.name||''} profile={props.profile||''} variant={props.variant||'default'} items={props.items||[]} content={props.content}/>,
  UnsupportedBlock:props=><section className="rounded-lg border border-dashed p-4" data-component="UnsupportedBlock" role="note">
    <p className="text-sm font-semibold">{props.component||'Component'} is not available in the browser preview{props.name?` (${props.name})`:''}.</p>
    <p className="text-sm text-muted-foreground">{props.reason||'This component requires a Bean backend.'}</p>
  </section>,
}

// Static menus ship resolved items in the render tree (beanmenu.StaticTree);
// owner-scoped menus are marked UnsupportedBlock by the bridge instead.
function StaticMenuBlock({name,profile,variant,items,content}:{name:string;profile:string;variant:RouteTabsVariant;items:MenuItem[];content?:React.ReactNode}){
  const navigate=useNavigate()
  if(profile!=='workspace')return <><nav className="flex flex-wrap gap-2" aria-label="Page navigation">{items.map((item,index)=><Button key={item.ID||item.Route||index} variant="outline" asChild><Link aria-current={item.Current?'page':undefined} to={item.Route}>{item.Label}</Link></Button>)}</nav>{content}</>
  const primary=items
  const activePrimary=primary.find(item=>item.Active)||primary[0]
  const secondary=activePrimary?.Children||[]
  const activeSecondary=secondary.find(item=>item.Active)||secondary[0]
  const tertiary=activeSecondary?.Children||[]
  return <WorkspaceMenuFrame menu={name} variant={variant} primary={primary} secondary={secondary} tertiary={tertiary} navigate={navigate} content={content}/>
}
