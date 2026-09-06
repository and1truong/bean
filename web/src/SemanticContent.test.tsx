import {fireEvent,render,screen} from '@testing-library/react'
import {MemoryRouter} from 'react-router-dom'
import {describe,expect,it} from 'vitest'
import {ContentBlock} from './Content'
import {ContentVisibility} from './ContentVisibility'
import {TabsBlock} from './Tabs'
import type {ContentElement,ContentTab} from './api'

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
