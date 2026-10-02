import {useEffect,useId,useState} from 'react'
import type {Flashcard} from './api'
import {useContentActive} from './ContentVisibility'
import {Button} from '@/components/ui/button'

// Literal semantic flashcard deck: prompts and answers are authored text kept
// verbatim; source order is the deck order. Reveal state is instance-local and
// resets when the enclosing frame/tab goes inactive or the cards change.
export function FlashcardBlock({title,cards}:{title:string;cards:Flashcard[]}){
  const id=useId();const active=useContentActive()
  const[revealed,setRevealed]=useState<Set<string>>(new Set())
  const identity=JSON.stringify(cards)
  useEffect(()=>{setRevealed(new Set())},[identity])
  useEffect(()=>{if(!active){setRevealed(new Set())}},[active])
  const toggle=(cardID:string)=>setRevealed(previous=>{const next=new Set(previous);if(next.has(cardID)){next.delete(cardID)}else{next.add(cardID)};return next})
  return <article className="bean-flashcards" aria-labelledby={id}>
    <div className="bean-flashcards-header"><h2 className="bean-flashcards-title" id={id}>{title}</h2><p className="bean-flashcards-count">{cards.length} cards</p></div>
    <ol className="bean-flashcards-deck">{cards.map((card,index)=>{
      const shown=revealed.has(card.id)
      return <li key={card.id} className="bean-flashcard" data-revealed={shown}>
        <Button type="button" variant="ghost" className="h-full w-full flex-col items-stretch justify-start gap-0 rounded-lg border-border-strong px-3 py-2 text-left text-base font-normal whitespace-normal" aria-expanded={shown} onClick={()=>toggle(card.id)}>
          <span className="bean-flashcard-face bean-flashcard-prompt"><span className="bean-flashcard-face-label">Card {index+1} · Prompt</span><span className="bean-flashcard-text">{card.Prompt}</span></span>
          {shown&&<span className="bean-flashcard-face bean-flashcard-answer"><span className="bean-flashcard-face-label">Answer</span><span className="bean-flashcard-text">{card.Answer}</span></span>}
        </Button>
      </li>})}
    </ol>
    {revealed.size>0&&<Button type="button" variant="outline" className="bean-flashcards-reset" onClick={()=>setRevealed(new Set())}>Hide all answers</Button>}
  </article>
}
