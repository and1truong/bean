import React from 'react'
import ReactDOM from 'react-dom/client'
import {HashRouter} from 'react-router-dom'
import PlaygroundApp from './App'
import {RenderersProvider} from '../registry'
import {playgroundRenderers} from './renderers'
import '../style.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <HashRouter>
      <RenderersProvider renderers={playgroundRenderers}>
        <PlaygroundApp/>
      </RenderersProvider>
    </HashRouter>
  </React.StrictMode>,
)
