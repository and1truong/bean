// Bean-owned adapter over the mermaid engine: callers hand it a diagram source
// Bean generated from a bounded contract (never author-supplied Mermaid), and it
// returns an inline SVG. The chunk is lazy-loaded so the ~450KB dependency is
// only fetched when a diagram actually renders. securityLevel 'strict' keeps
// DOMPurify on for anything that reaches the serializer.

type MermaidApi=typeof import('mermaid')['default']

let loaded:Promise<MermaidApi>|null=null
let counter=0

function engine():Promise<MermaidApi>{
  if(!loaded){
    loaded=import('mermaid').then(module=>{
      module.default.initialize({startOnLoad:false,securityLevel:'strict',theme:'base',themeVariables:{primaryColor:'#ffffff',primaryBorderColor:'#3f3f46',primaryTextColor:'#18181b',lineColor:'#71717a',tertiaryColor:'#ffffff',noteBkgColor:'#fafafa',fontFamily:'inherit'},logLevel:'fatal'})
      return module.default
    })
  }
  return loaded
}

export async function renderDiagram(source:string):Promise<string>{
  const mermaid=await engine()
  const {svg}=await mermaid.render('bean-diagram-'+(counter++),source)
  return svg
}

// mermaidEntity escapes one label for inclusion inside a quoted Mermaid node.
// Single pass: emitted entities are never reprocessed.
const ENTITIES:Record<string,string>={'#':'#35;',';':'#59;','"':'#quot;','<':'#lt;','>':'#gt;','&':'#amp;','\\':'#92;'}
export function mermaidEntity(text:string):string{
  return text.replace(/[#";\\<>&\n\r\t]/g,char=>ENTITIES[char]||' ')
}
