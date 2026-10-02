import {fireEvent,render,screen,within} from '@testing-library/react'
import {MemoryRouter} from 'react-router-dom'
import {describe,expect,it,vi} from 'vitest'
import {ContentBlock} from './Content'
import {ContentVisibility} from './ContentVisibility'
import {LessonBlock} from './Lesson'
import {TabsBlock} from './Tabs'
import {TimelineBlock} from './Timeline'
import {MindmapBlock,mindmapSyntax} from './Mindmap'
import {FlashcardBlock} from './Flashcard'
import type {ContentElement,ContentTab,FormulaNode} from './api'

function content(elements:ContentElement[]){return render(<MemoryRouter><ContentBlock content={elements}/></MemoryRouter>)}

describe('semantic content',()=>{
  it('renders semantic markup and escapes literal text',()=>{
    content([
      {Type:'heading',Level:3,Text:'<em>Boundary</em>'},
      {Type:'ordered_list',Items:['Define','Validate']},
      {Type:'link',Label:'Open presentation',Target:'/presentations/bean?frame=architecture',OpenIn:'same_tab'},
      {Type:'link',Label:'External guide',Target:'https://example.test/guide',OpenIn:'new_tab'},
      {Type:'divider'},
      {Type:'table',Caption:'Read and write boundaries',Columns:[{id:'primitive',Label:'Primitive'},{id:'owner',Label:'Owner'}],Rows:[['View','Read'],['Action','Write']],RowHeader:'first'},
    ])
    expect(screen.getByRole('heading',{level:3,name:'<em>Boundary</em>'})).toBeInTheDocument()
    expect(document.querySelector('em')).toBeNull()
    expect(screen.getByRole('list').tagName).toBe('OL')
    expect(screen.getByRole('separator')).toBeInTheDocument()
    expect(screen.getByRole('link',{name:'Open presentation'})).toHaveAttribute('href','/presentations/bean?frame=architecture')
    expect(screen.getByRole('link',{name:/External guide/})).toHaveAttribute('target','_blank')
    expect(screen.getByRole('link',{name:/External guide/})).toHaveAttribute('rel','noopener noreferrer')
    expect(screen.getByRole('region',{name:'Read and write boundaries'})).toHaveAttribute('data-keyboard-scrollable','true')
    expect(screen.getByRole('columnheader',{name:'Primitive'})).toHaveAttribute('scope','col')
    expect(screen.getByRole('rowheader',{name:'View'})).toHaveAttribute('scope','row')
  })

  it('defers media requests until Load and keeps transcript and fallback available',()=>{
    content([
      {Type:'audio',Source:'/media/intro.mp3',Title:'Bean audio',Transcript:'Audio transcript.'},
      {Type:'youtube',VideoID:'M7lc1UVf-VE',Title:'Bean video',Transcript:'Video transcript.'},
      {Type:'youtube_playlist',PlaylistID:'PL1234567890ABCDEFG',Title:'Bean playlist',Transcript:'Playlist transcript.'},
    ])
    expect(document.querySelector('audio')).toBeNull();expect(document.querySelector('iframe')).toBeNull()
    expect(screen.getByText('Audio transcript.')).toBeVisible();expect(screen.getByText('Video transcript.')).toBeVisible()
    fireEvent.click(screen.getByRole('button',{name:'Load audio'}))
    const audio=document.querySelector('audio')!;expect(audio).toHaveAttribute('src','/media/intro.mp3');expect(audio).toHaveAttribute('preload','none')
    fireEvent.error(audio);expect(screen.getByText('Audio could not be loaded.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Load YouTube video'}))
    const frame=document.querySelector('iframe')!;expect(frame).toHaveAttribute('src','https://www.youtube-nocookie.com/embed/M7lc1UVf-VE?autoplay=0&controls=1&playsinline=1&cc_load_policy=1')
    expect(frame).toHaveAttribute('referrerpolicy','strict-origin-when-cross-origin');expect(frame.getAttribute('allow')).not.toContain('autoplay')
    fireEvent.load(frame);expect(screen.getByText(/Playback availability is not verified/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button',{name:'Load YouTube playlist'}))
    expect(document.querySelectorAll('iframe')[1]).toHaveAttribute('src','https://www.youtube-nocookie.com/embed/videoseries?listType=playlist&list=PL1234567890ABCDEFG&autoplay=0&controls=1&playsinline=1&cc_load_policy=1')
    expect(screen.getAllByRole('link',{name:'Open on YouTube'})).toHaveLength(2)
  })

  it('uses readable image fallback and resets replaced quiz and media props',()=>{
    const firstQuiz:ContentElement={Type:'choices',Question:'First?',Choices:[{id:'one',Text:'One'},{id:'two',Text:'Two'}],Answer:'two'}
    const firstAudio:ContentElement={Type:'audio',Source:'/first.mp3',Title:'First audio',Transcript:'First transcript'}
    const view=render(<MemoryRouter><ContentBlock content={[{Type:'image',Source:'/missing.png',Alt:'Release flow'},firstQuiz,firstAudio]}/></MemoryRouter>)
    fireEvent.error(screen.getByRole('img',{name:'Release flow'}));expect(screen.getByRole('img',{name:'Release flow'})).toHaveTextContent('Release flow')
    fireEvent.click(screen.getAllByRole('radio')[1]);fireEvent.click(screen.getByRole('button',{name:'Check answer'}));fireEvent.click(screen.getByRole('button',{name:'Load audio'}))
    const nextQuiz:ContentElement={...firstQuiz,Question:'Second?'};const nextAudio:ContentElement={...firstAudio,Source:'/second.mp3'}
    view.rerender(<MemoryRouter><ContentBlock content={[nextQuiz,nextAudio]}/></MemoryRouter>)
    expect(screen.getAllByRole('radio')[1]).not.toBeChecked();expect(screen.queryByText('Correct')).not.toBeInTheDocument();expect(document.querySelector('audio')).toBeNull()
  })

  it('grades choices only on Check, retries, and isolates instances',()=>{
    const quiz:ContentElement={Type:'choices',Question:'Write boundary?',Choices:[{id:'view',Text:'View'},{id:'action',Text:'Action'}],Answer:'action',Explanation:'Actions write.'}
    content([quiz,quiz])
    const radios=screen.getAllByRole('radio');expect(new Set(radios.map(radio=>radio.getAttribute('name'))).size).toBe(2)
    const checks=screen.getAllByRole('button',{name:'Check answer'});expect(checks[0]).toBeDisabled()
    fireEvent.click(radios[1]);expect(checks[0]).toBeEnabled();expect(screen.queryByText('Correct')).not.toBeInTheDocument()
    fireEvent.click(checks[0]);expect(screen.getByText('Correct')).toBeInTheDocument();expect(radios[0]).toBeDisabled();expect(radios[2]).not.toBeDisabled()
    fireEvent.click(screen.getByRole('button',{name:'Try again'}));expect(radios[1]).not.toBeChecked();expect(screen.queryByText('Correct')).not.toBeInTheDocument()
  })

  it('resets quiz and unloads media only when its visibility leaves',()=>{
    const quiz:ContentElement={Type:'choices',Question:'Boundary?',Choices:[{id:'view',Text:'View'},{id:'action',Text:'Action'}],Answer:'action'}
    const media:ContentElement={Type:'audio',Source:'/intro.mp3',Title:'Audio',Transcript:'Transcript'}
    const view=render(<MemoryRouter><ContentVisibility active><ContentBlock content={[quiz,media]}/></ContentVisibility></MemoryRouter>)
    fireEvent.click(screen.getAllByRole('radio')[1]);fireEvent.click(screen.getByRole('button',{name:'Check answer'}));fireEvent.click(screen.getByRole('button',{name:'Load audio'}))
    view.rerender(<MemoryRouter><ContentVisibility active><ContentBlock content={[quiz,media]}/></ContentVisibility></MemoryRouter>)
    expect(screen.getByText('Correct')).toBeInTheDocument();expect(document.querySelector('audio')).toBeInTheDocument()
    view.rerender(<MemoryRouter><ContentVisibility active={false}><ContentBlock content={[quiz,media]}/></ContentVisibility></MemoryRouter>)
    expect(screen.queryByText('Correct')).not.toBeInTheDocument();expect(document.querySelector('audio')).toBeNull()
  })
})

describe('Tabs Block',()=>{
  const tabs:ContentTab[]=[{id:'reads',Label:'Reads',Content:[{Type:'paragraph',Text:'Views read.'}]},{id:'writes',Label:'Writes',Content:[{Type:'paragraph',Text:'Actions write.'}]}]

  it('uses isolated DOM identities, roving focus, automatic activation, and hidden panels',()=>{
    render(<MemoryRouter><><TabsBlock label="Boundaries" tabs={tabs}/><TabsBlock label="Other boundaries" tabs={tabs}/></></MemoryRouter>)
    const tablists=screen.getAllByRole('tablist');expect(tablists[0]).toHaveAttribute('aria-orientation','horizontal')
    const buttons=screen.getAllByRole('tab');expect(buttons[0]).toHaveAttribute('aria-selected','true');expect(buttons[1]).toHaveAttribute('tabindex','-1')
    expect(buttons[0].id).not.toBe(buttons[2].id);expect(buttons[0].getAttribute('aria-controls')).not.toBe(buttons[2].getAttribute('aria-controls'))
    buttons[0].focus();fireEvent.keyDown(buttons[0],{key:'ArrowLeft'})
    expect(buttons[1]).toHaveFocus();expect(buttons[1]).toHaveAttribute('aria-selected','true');expect(screen.getAllByText('Actions write.')[0]).toBeVisible()
    fireEvent.keyDown(buttons[1],{key:'Home'});expect(buttons[0]).toHaveFocus();expect(buttons[0]).toHaveAttribute('aria-selected','true')
    const panels=screen.getAllByRole('tabpanel',{hidden:true});expect(panels[0]).toHaveAttribute('tabindex','0');expect(panels[1]).toHaveAttribute('aria-hidden','true');expect(panels[1]).toHaveAttribute('data-hidden','true')
  })

  it('uses vertical keys without consuming horizontal keys',()=>{
    render(<MemoryRouter><TabsBlock label="Boundaries" orientation="vertical" variant="pills" tabs={tabs}/></MemoryRouter>)
    const buttons=screen.getAllByRole('tab');const left=new KeyboardEvent('keydown',{key:'ArrowLeft',cancelable:true,bubbles:true});buttons[0].dispatchEvent(left);expect(left.defaultPrevented).toBe(false)
    fireEvent.keyDown(buttons[0],{key:'ArrowDown'});expect(buttons[1]).toHaveAttribute('aria-selected','true')
    fireEvent.keyDown(buttons[1],{key:'ArrowDown'});expect(buttons[0]).toHaveAttribute('aria-selected','true')
  })

  it('preserves selection on rerender and resets it after leaving',()=>{
    const view=render(<MemoryRouter><ContentVisibility active><TabsBlock label="Boundaries" tabs={tabs}/></ContentVisibility></MemoryRouter>)
    fireEvent.click(screen.getAllByRole('tab')[1]);view.rerender(<MemoryRouter><ContentVisibility active><TabsBlock label="Boundaries" tabs={tabs}/></ContentVisibility></MemoryRouter>);expect(screen.getAllByRole('tab')[1]).toHaveAttribute('aria-selected','true')
    view.rerender(<MemoryRouter><ContentVisibility active={false}><TabsBlock label="Boundaries" tabs={tabs}/></ContentVisibility></MemoryRouter>);expect(screen.getAllByRole('tab')[0]).toHaveAttribute('aria-selected','true')
  })

  it('unloads media and resets quiz when their tab is left',()=>{
    const mediaTabs:ContentTab[]=[{id:'media',Label:'Media',Content:[{Type:'audio',Source:'/intro.mp3',Title:'Audio',Transcript:'Transcript'}]},{id:'quiz',Label:'Quiz',Content:[{Type:'choices',Question:'Choose?',Choices:[{id:'one',Text:'One'},{id:'two',Text:'Two'}],Answer:'two'}]}]
    render(<MemoryRouter><TabsBlock label="Interactive" tabs={mediaTabs}/></MemoryRouter>)
    fireEvent.click(screen.getByRole('button',{name:'Load audio'}));expect(document.querySelector('audio')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab',{name:'Quiz'}));fireEvent.click(screen.getAllByRole('radio')[1]);fireEvent.click(screen.getByRole('button',{name:'Check answer'}));expect(screen.getByText('Correct')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab',{name:'Media'}));expect(document.querySelector('audio')).toBeNull()
    fireEvent.click(screen.getByRole('tab',{name:'Quiz'}));expect(screen.getAllByRole('radio')[1]).not.toBeChecked();expect(screen.queryByText('Correct')).not.toBeInTheDocument()
  })
})

describe('Lesson Block',()=>{
  it('renders ordered sections and keeps literal text literal',()=>{
    render(<MemoryRouter><LessonBlock title="Worked example" sections={[
      {id:'idea',Heading:'The idea',Content:[{Type:'paragraph',Text:'<b>literal</b> stays text'}]},
      {id:'steps',Content:[{Type:'ordered_list',Items:['Substitute','Solve']}]},
    ]}/></MemoryRouter>)
    const lesson=screen.getByRole('article',{name:'Worked example'})
    expect(lesson.querySelectorAll('li')).toHaveLength(4)
    const headings=lesson.querySelectorAll('h2,h3');expect(headings[0]).toHaveTextContent('Worked example');expect(headings[1]).toHaveTextContent('The idea')
    expect(screen.getByText('<b>literal</b> stays text')).toBeInTheDocument();expect(lesson.querySelector('b')).toBeNull()
    expect(within(lesson).getAllByRole('list')[0]).toHaveClass('bean-lesson-sections')
  })

  it('renders image illustration fallback inside a section',()=>{
    render(<MemoryRouter><LessonBlock title="Lesson" sections={[{id:'picture',Content:[{Type:'image',Source:'/missing.png',Alt:'Discriminant sketch'}]}]}/></MemoryRouter>)
    fireEvent.error(screen.getByRole('img',{name:'Discriminant sketch'}))
    expect(screen.getByRole('img',{name:'Discriminant sketch'})).toHaveTextContent('Discriminant sketch')
  })
})

describe('Timeline Block',()=>{
  it('keeps source order and verbatim labels, and keeps literal text literal',()=>{
    render(<MemoryRouter><TimelineBlock title="Milestones" entries={[
      {id:'b',Label:'Day 1',Title:'Second'},
      {id:'a',Label:'5th century BCE',Title:'First',Description:'<i>literal</i> stays text'},
    ]}/></MemoryRouter>)
    const block=screen.getByRole('article',{name:'Milestones'})
    const items=within(block).getAllByRole('listitem')
    expect(items).toHaveLength(2)
    expect(items[0].querySelector('h3')).toHaveTextContent('Second');expect(items[0]).toHaveTextContent('Day 1')
    expect(items[1].querySelector('h3')).toHaveTextContent('First');expect(items[1]).toHaveTextContent('5th century BCE')
    expect(items[1]).toHaveTextContent('<i>literal</i> stays text');expect(block.querySelector('i')).toBeNull()
    expect(block.querySelector('time')).toBeNull()
  })

  it('reuses the ordered rail structure shared with the record-backed timeline view',()=>{
    render(<MemoryRouter><TimelineBlock title="Milestones" entries={[{id:'one',Label:'1440',Title:'Movable type'}]}/></MemoryRouter>)
    const rail=screen.getByRole('list')
    expect(rail.tagName).toBe('OL');expect(rail).toHaveClass('relative','space-y-6','border-l')
  })
})

vi.mock('./mermaid',async(importOriginal)=>{
  const actual=await importOriginal<typeof import('./mermaid')>()
  return {...actual,renderDiagram:(source:string)=>Promise.resolve('<svg data-engine="mermaid"><text>'+source.length+'</text></svg>')}
})

describe('Mindmap Block',()=>{
  const tree={id:'topic',Label:'Bean',Description:'Declarative apps',Children:[
    {id:'definitions',Label:'Definitions',Children:[{id:'entities',Label:'<i>Entities</i>'},{id:'views',Label:'Views'}]},
    {id:'runtime',Label:'Runtime',Description:'Atomic activation'},
  ]}

  it('serializes the tree with stable IDs, verbatim order, and escaped labels',()=>{
    const syntax=mindmapSyntax({id:'a',Label:'Root "quoted" #1',Children:[{id:'b',Label:'Same',Children:[{id:'c',Label:'Leaf; end'}]},{id:'d',Label:'Same'}]})
    const lines=syntax.split('\n')
    expect(lines[0]).toBe('mindmap')
    expect(lines[1]).toBe('  root(("Root #quot;quoted#quot; #35;1"))')
    expect(lines[2]).toBe('    b["Same"]')
    expect(lines[3]).toBe('      c("Leaf#59; end")')
    expect(lines[4]).toBe('    d["Same"]')
  })

  it('renders nested hierarchy in source order with a named root and literal labels',async()=>{
    render(<MemoryRouter><MindmapBlock root={tree}/></MemoryRouter>)
    await screen.findByTestId('mindmap-figure')
    const block=screen.getByRole('article',{name:'Bean'})
    const branches=within(block).getAllByRole('listitem')
    expect(branches).toHaveLength(4)
    expect(branches[0]).toHaveTextContent('Definitions');expect(branches[0].querySelector('h3')).not.toBeNull()
    expect(within(branches[0]).getByText('<i>Entities</i>')).toBeVisible();expect(block.querySelector('i')).toBeNull()
    expect(block.querySelector('.bean-mindmap-branches ol')).not.toBeNull()
    expect(block).toHaveTextContent('Declarative apps');expect(block).toHaveTextContent('Atomic activation')
  })

  it('keeps hierarchy machine-readable without connectors',async()=>{
    render(<MemoryRouter><MindmapBlock root={tree}/></MemoryRouter>)
    await screen.findByTestId('mindmap-figure')
    const nested=document.querySelectorAll('.bean-mindmap-children')
    expect(nested.length).toBe(1)
    const defs=screen.getByText('<i>Entities</i>',{selector:'p'})
    expect(defs.closest('li')!.closest('ol.bean-mindmap-children')).not.toBeNull()
  })

  it('adds the mermaid figure as a decorative layer while the DOM tree stays accessible',async()=>{
    render(<MemoryRouter><MindmapBlock root={tree}/></MemoryRouter>)
    const figure=await screen.findByTestId('mindmap-figure')
    expect(figure).toHaveAttribute('aria-hidden','true')
    expect(figure.querySelector('svg[data-engine="mermaid"]')).not.toBeNull()
    const block=screen.getByRole('article',{name:'Bean'})
    expect(block).toHaveClass('bean-mindmap-visual')
    expect(within(block).getAllByRole('listitem')).toHaveLength(4)
  })
})

describe('formula content',()=>{
  const quadratic:FormulaNode={Kind:'group',Parts:[{Kind:'literal',Text:'x'},{Kind:'literal',Text:'='},{Kind:'frac',Numerator:{Kind:'group',Parts:[{Kind:'literal',Text:'−b'},{Kind:'literal',Text:'±'},{Kind:'sqrt',Inner:{Kind:'group',Parts:[{Kind:'sup',Base:{Kind:'literal',Text:'b'},Exponent:{Kind:'literal',Text:'2'}},{Kind:'literal',Text:'−4ac'}]}}]},Denominator:{Kind:'literal',Text:'2a'}}]}
  const kinds:FormulaNode={Kind:'group',Parts:[
    {Kind:'paren',Style:'abs',Inner:{Kind:'literal',Text:'x'}},
    {Kind:'root',Inner:{Kind:'literal',Text:'x'},Index:{Kind:'literal',Text:'3'}},
    {Kind:'sub',Base:{Kind:'literal',Text:'x'},Subscript:{Kind:'literal',Text:'1'}},
    {Kind:'func',Name:'sin',Argument:{Kind:'literal',Text:'θ'}},
    {Kind:'literal',Text:'2'},
    {Kind:'sum',Lower:{Kind:'literal',Text:'i=1'},Upper:{Kind:'literal',Text:'n'},Body:{Kind:'literal',Text:'i'}},
  ]}

  it('renders MathML structure with a visible readable fallback',()=>{
    content([{Type:'formula',Expr:quadratic,Text:'x equals (−b ± √(b² − 4ac)) / 2a'}])
    const formula=document.querySelector('math')!
    expect(formula.querySelector('mfrac')).not.toBeNull();expect(formula.querySelector('msqrt')).not.toBeNull();expect(formula.querySelector('msup')).not.toBeNull()
    expect(formula.textContent.replace(/\s/g,'')).toMatch(/^x=.+2a$/)
    expect(formula.getAttribute('aria-hidden')).toBe('true')
    expect(screen.getByText('x equals (−b ± √(b² − 4ac)) / 2a')).toBeVisible()
    expect(document.querySelector('.bean-formula')).toHaveAttribute('data-keyboard-scrollable','true')
  })

  it('renders every supported node kind and escapes literal content',()=>{
    kinds.Parts!.push({Kind:'literal',Text:'<mi>'})
    content([{Type:'formula',Expr:kinds,Text:'every kind'}])
    const formula=document.querySelector('math')!
    for(const tag of ['mrow','mroot','msub','munderover','mi','mo','mn'])expect(formula.querySelector(tag),tag).not.toBeNull()
    expect(formula.textContent).toContain('<mi>')
    expect(formula.querySelector('munderover')).toHaveTextContent('∑')
  })
})

describe('Flashcard Block',()=>{
  const deck=[
    {id:'b',Prompt:'What is a Block?',Answer:'A named region of metadata-rendered content.'},
    {id:'a',Prompt:'What is a <i>Panel</i>?',Answer:'A <b>layout</b> that hosts Blocks.'},
  ]

  it('renders an ordered deck with prompts visible and answers hidden',()=>{
    render(<MemoryRouter><FlashcardBlock title="Bean vocabulary" cards={deck}/></MemoryRouter>)
    const block=screen.getByRole('article',{name:'Bean vocabulary'})
    expect(block).toHaveTextContent('2 cards')
    const items=within(block).getAllByRole('listitem')
    expect(items).toHaveLength(2)
    expect(items[0]).toHaveTextContent('What is a Block?');expect(items[0]).not.toHaveTextContent('A named region')
    const toggles=within(block).getAllByRole('button',{expanded:false})
    expect(toggles).toHaveLength(2)
    expect(within(block).queryByRole('button',{name:/Hide all/})).toBeNull()
  })

  it('keeps literal prompt and answer text literal',()=>{
    render(<MemoryRouter><FlashcardBlock title="Deck" cards={deck}/></MemoryRouter>)
    const block=screen.getByRole('article',{name:'Deck'})
    expect(block).toHaveTextContent('What is a <i>Panel</i>?');expect(block.querySelector('i')).toBeNull()
    fireEvent.click(within(block).getAllByRole('button')[1])
    expect(block).toHaveTextContent('A <b>layout</b> that hosts Blocks.');expect(block.querySelector('b')).toBeNull()
  })

  it('reveals one card at a time and hides all answers from the deck',()=>{
    render(<MemoryRouter><FlashcardBlock title="Deck" cards={deck}/></MemoryRouter>)
    const block=screen.getByRole('article',{name:'Deck'})
    const buttons=within(block).getAllByRole('button')
    fireEvent.click(buttons[0])
    expect(buttons[0]).toHaveAttribute('aria-expanded','true');expect(buttons[1]).toHaveAttribute('aria-expanded','false')
    expect(block).toHaveTextContent('A named region of metadata-rendered content.');expect(block).not.toHaveTextContent('hosts Blocks')
    fireEvent.click(buttons[1])
    expect(within(block).getByRole('button',{name:/Hide all answers/})).toBeVisible()
    fireEvent.click(within(block).getByRole('button',{name:/Hide all answers/}))
    expect(within(block).getAllByRole('button',{expanded:false})).toHaveLength(2)
    expect(block).not.toHaveTextContent('A named region')
  })

  it('keeps reveal state instance-local across two decks',()=>{
    render(<MemoryRouter><><FlashcardBlock title="One" cards={deck}/><FlashcardBlock title="Two" cards={deck}/></></MemoryRouter>)
    const one=screen.getByRole('article',{name:'One'});const two=screen.getByRole('article',{name:'Two'})
    fireEvent.click(within(one).getAllByRole('button')[0])
    expect(within(one).getAllByRole('button')[0]).toHaveAttribute('aria-expanded','true')
    expect(within(two).getAllByRole('button')[0]).toHaveAttribute('aria-expanded','false')
    expect(two).not.toHaveTextContent('A named region')
  })
})
