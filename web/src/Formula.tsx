import {Fragment} from 'react'
import type {FormulaNode} from './api'

const parenStyles:Record<string,[string,string]>={round:['(',')'],square:['[',']'],brace:['{','}'],abs:['|','|']}
const isNumber=/^-?\d+([.,]\d+)?$/

// Literal leaves classify into MathML token elements: numbers, operators, and
// identifiers. Text is always literal element content — never markup.
function literal(text:string):React.ReactNode{
  if(isNumber.test(text))return <mn>{text}</mn>
  if(text.length===1&&!/[A-Za-z0-9]/.test(text))return <mo>{text}</mo>
  return <mi>{text}</mi>
}

function node(item:FormulaNode|undefined,key?:number):React.ReactNode{
  if(!item)return null
  switch(item.Kind){
    case 'literal':return literal(item.Text||'')
    case 'group':return <mrow key={key}>{(item.Parts||[]).map((part,index)=><Fragment key={index}>{node(part)}</Fragment>)}</mrow>
    case 'paren':{const[open,close]=parenStyles[item.Style||'round']||parenStyles.round;return <mrow key={key}><mo stretchy="true">{open}</mo>{node(item.Inner)}<mo stretchy="true">{close}</mo></mrow>}
    case 'frac':return <mfrac key={key}>{node(item.Numerator)}{node(item.Denominator)}</mfrac>
    case 'sqrt':return <msqrt key={key}>{node(item.Inner)}</msqrt>
    case 'root':return <mroot key={key}>{node(item.Inner)}{node(item.Index)}</mroot>
    case 'sup':return <msup key={key}>{node(item.Base)}{node(item.Exponent)}</msup>
    case 'sub':return <msub key={key}>{node(item.Base)}{node(item.Subscript)}</msub>
    case 'func':return <mrow key={key}><mi mathvariant="normal">{item.Name}</mi><mo>&#8289;</mo>{node(item.Argument)}</mrow>
    case 'sum':return <mrow key={key}><munderover><mo>&#8721;</mo>{node(item.Lower)}{node(item.Upper)}</munderover>{node(item.Body)}</mrow>
    default:return <mtext key={key}>{item.Text||''}</mtext>
  }
}

// The MathML rendering is hidden from assistive technology; the bounded plain
// text fallback is always rendered visibly, so it carries meaning for screen
// readers, non-MathML renderers, and print.
export function Formula({expr,text}:{expr:FormulaNode;text:string}){
  return <figure className="bean-formula" tabIndex={0} data-keyboard-scrollable="true"><math xmlns="http://www.w3.org/1998/Math/MathML" display="block" aria-hidden="true">{node(expr)}</math><figcaption className="bean-formula-alt">{text}</figcaption></figure>
}
