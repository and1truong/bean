import {useId} from 'react'
import type {MindMapNode} from './api'

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

export function MindmapBlock({root}:{root:MindMapNode}){
  const labelId=useId()
  const branches=root.Children||[]
  return <article className="bean-mindmap" aria-labelledby={labelId}>
    <div className="bean-mindmap-root">
      <h2 id={labelId} className="bean-mindmap-root-label">{root.Label}</h2>
      {root.Description?<p className="bean-mindmap-root-description">{root.Description}</p>:null}
    </div>
    <ol className="bean-mindmap-branches">
      {branches.map(branch=><MindMapBranch key={branch.id} node={branch} depth={1}/>)}
    </ol>
  </article>
}
