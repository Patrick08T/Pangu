import {useEffect} from 'react';

export function useDraftGuard(dirty:boolean){
  useEffect(()=>{
    if(!dirty)return;
    const leave=(event:Event)=>{if(!window.confirm('经验策略有未保存修改。离开将丢弃本地编辑，是否继续？'))event.preventDefault();};
    const unload=(event:BeforeUnloadEvent)=>{event.preventDefault();event.returnValue='';};
    window.addEventListener('pangu:before-navigate',leave);
    window.addEventListener('beforeunload',unload);
    return()=>{window.removeEventListener('pangu:before-navigate',leave);window.removeEventListener('beforeunload',unload);};
  },[dirty]);
}
