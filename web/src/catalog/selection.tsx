import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';

export function VersionPicker({kind,value,change,disabled=false,agentsOnly=false}:{kind:string;value:string;change:(version:Schema['CatalogVersion']|undefined)=>void;disabled?:boolean;agentsOnly?:boolean}) {
  const [object,setObject]=useState(''),[offset,setOffset]=useState(0),[page,setPage]=useState(0);
  const params=new URLSearchParams({kind,offset:String(offset)});
  const objects=useQuery<Schema['CatalogPage']>(`/catalog?${params}`);
  const versions=useQuery<Schema['CatalogVersion'][]>(object?`/catalog/${object}/versions?offset=${page}`:null);
  function selectObject(id:string){setObject(id);setPage(0);change(undefined);}
  function nextObjects(next:number){setOffset(next);selectObject('');}
  function nextVersions(next:number){setPage(next);change(undefined);}
  return <fieldset disabled={disabled}><legend>选择已发布版本</legend>
    <QueryState {...objects} retry={objects.reload}/>
    <label className="field">选择配置<select value={object} onChange={e=>selectObject(e.target.value)}><option value="">请选择配置</option>{objects.data?.items.filter(item=>!item.archived&&(!agentsOnly||!!item.spec.agent)).map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
    <div className="toolbar"><button type="button" disabled={!offset} onClick={()=>nextObjects(0)}>配置首页</button><button type="button" disabled={objects.data?.next_offset===undefined} onClick={()=>nextObjects(objects.data!.next_offset!)}>更多配置</button></div>
    {object&&<><QueryState {...versions} retry={versions.reload} empty={versions.data?.length===0?'此配置尚未发布，请先保存并发布。':undefined}/>
      <label className="field">选择发布版本<select value={value} disabled={versions.loading||!!versions.error} onChange={e=>change(versions.data?.find(item=>item.id===e.target.value&&item.object_id===object))}><option value="">请选择固定版本</option>{versions.data?.filter(item=>item.object_id===object&&!item.archived&&(!agentsOnly||!!item.spec.agent)).map(item=><option key={item.id} value={item.id}>v{item.number} · {new Date(item.created_at).toLocaleString('zh-CN')}</option>)}</select></label>
      <div className="toolbar"><button type="button" disabled={!page} onClick={()=>nextVersions(0)}>版本首页</button><button type="button" disabled={versions.data?.length!==100} onClick={()=>nextVersions(page+100)}>更多版本</button><a href={`#/catalog/${object}`}>管理此配置</a></div>
    </>}
  </fieldset>;
}
