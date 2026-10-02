import {useEffect,useId,useState} from 'react'
import type {MindMapNode} from './api'
import {mermaidEntity,renderDiagram} from './mermaid'

// mindmapSyntax serializes the bounded node tree into Mermaid mindmap syntax —
// the only place Mermaid text exists, and it is generated here, never authored.
// Stable machine IDs become mermaid node keys so duplicate labels stay distinct;
// source order is emitted verbatim since indentation defines the hierarchy.
export function mindmapSyntax(root:MindMapNode):string{
  const lines=['mindmap']
  const visit=(node:MindMapNode,depth:number)=>{
    const indent='  '.repeat(depth+1)
    const label=mermaidEntity(node.Label)
    if(depth===0){
      lines.push(indent+'root(("'+label+'"))')
    }else if(depth===1){
      lines.push(indent+node.id+'["'+label+'"]')
    }else{
      lines.push(indent+node.id+'("'+label+'")')
    }
    for(const child of node.Children||[])visit(child,depth+1)
  }
  visit(root,0)
  return lines.join('\n')
}

// MindMapBranch renders one topic and its ordered child topics. Nested <ol>s
// keep the hierarchy machine-readable: the DOM order is the source order and
// relationships are conveyed by nesting, not by connector lines or color.
function MindMapBranch({node,depth}:{node:MindMapNode;depth:number}){
  const children=node.Children||[]
  return <li className="bean-mindmap-branch">
    <div className="bean-mindmap-node" data-depth={depth}>
      {depth===1?<h3 className="bean-mindmap-label">{node.Label}</h3>:<p className="bean-mindmap-label">{node.Label}</p>}
      {node.Description?<p className="bean-mindmap-description">{node.Description}</p>:null}
    </div>
    {children.length>0&&<ol className="bean-mindmap-children">
      {children.map(child=><MindMapBranch key={child.id} node={child} depth={depth+1}/>)}
    </ol>}
  </li>
}

// MindmapBlock renders the literal tree two ways from one source: a Mermaid
// SVG figure on wide screens (aria-hidden), and the semantic <ol> tree that
// stays in the accessibility tree, prints, and is the visible layout below
// 40rem or while the engine is still loading.
export function MindmapBlock({root}:{root:MindMapNode}){
  const labelId=useId()
  const [svg,setSvg]=useState<string|null>(null)
  useEffect(()=>{
    let active=true
    renderDiagram(mindmapSyntax(root)).then(result=>{if(active)setSvg(result)}).catch(()=>{})
    return ()=>{active=false}
  },[root])
  const branches=root.Children||[]
  return <article className={svg?'bean-mindmap bean-mindmap-visual':'bean-mindmap'} aria-labelledby={labelId}>
    {svg&&<div className="bean-mindmap-figure" aria-hidden="true" data-testid="mindmap-figure" dangerouslySetInnerHTML={{__html:svg}}/>}
    <div className="bean-mindmap-dom">
      <div className="bean-mindmap-root">
        <h2 id={labelId} className="bean-mindmap-root-label">{root.Label}</h2>
        {root.Description?<p className="bean-mindmap-root-description">{root.Description}</p>:null}
      </div>
      <ol className="bean-mindmap-branches">
        {branches.map(branch=><MindMapBranch key={branch.id} node={branch} depth={1}/>)}
      </ol>
    </div>
  </article>
}
