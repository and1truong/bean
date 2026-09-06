import {createContext,useContext} from 'react'

const Visibility=createContext(true)

export function ContentVisibility({active,children}:{active:boolean;children:React.ReactNode}){
  return <Visibility.Provider value={active}>{children}</Visibility.Provider>
}

export function useContentActive(){return useContext(Visibility)}
