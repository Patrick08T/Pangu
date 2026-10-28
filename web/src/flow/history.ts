import {useEffect,useState} from 'react';
import type {Flow} from './model';

export function useDraft(initial:Flow){
  const [state,setState]=useState({past:[] as Flow[],value:initial,future:[] as Flow[]});
  function change(value:Flow){setState(current=>JSON.stringify(current.value)===JSON.stringify(value)?current:{past:[...current.past,current.value].slice(-50),value,future:[]});}
  function undo(){setState(current=>current.past.length?{past:current.past.slice(0,-1),value:current.past.at(-1)!,future:[current.value,...current.future]}:current);}
  function redo(){setState(current=>current.future.length?{past:[...current.past,current.value],value:current.future[0],future:current.future.slice(1)}:current);}
  function reset(value:Flow){setState({past:[],value,future:[]});}
  return {value:state.value,change,undo,redo,reset,canUndo:!!state.past.length,canRedo:!!state.future.length};
}

export function useLeaveGuard(dirty:boolean,message='还有未保存的修改。确定离开并丢弃吗？'){
  useEffect(()=>{
    if(!dirty)return;
    const unload=(event:BeforeUnloadEvent)=>{event.preventDefault();event.returnValue='';};
    const leave=(event:Event)=>{if(!window.confirm(message))event.preventDefault();};
    window.addEventListener('beforeunload',unload);window.addEventListener('pangu:before-navigate',leave);
    return()=>{window.removeEventListener('beforeunload',unload);window.removeEventListener('pangu:before-navigate',leave);};
  },[dirty,message]);
}
