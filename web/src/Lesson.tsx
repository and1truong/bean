import {useId} from 'react'
import type {LessonSection} from './api'
import {ContentBlock} from './Content'

export function LessonBlock({title,sections=[]}:{title:string;sections?:LessonSection[]}){
  const id=useId()
  return <article className="bean-lesson" aria-labelledby={id}>
    <h2 className="bean-lesson-title" id={id}>{title}</h2>
    <ol className="bean-lesson-sections" role="list">
      {sections.map((section,index)=><li className="bean-lesson-section" key={section.id||index}>
        {section.Heading&&<h3 className="bean-lesson-section-heading">{section.Heading}</h3>}
        <ContentBlock content={section.Content||[]}/>
      </li>)}
    </ol>
  </article>
}
