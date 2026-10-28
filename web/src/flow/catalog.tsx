import {useEffect,useState} from 'react';
import {apiRequest} from '../api/request';
import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';
import type {Flow,Versions} from './model';

export function useVersions(flow:Flow){
  const ids=[...new Set(flow.nodes.map(n=>n.agent_version))].sort().join(',');
  const [state,setState]=useState<{versions:Versions;loading:boolean;error?:Error}>({versions:{},loading:!!ids});
  useEffect(()=>{
    const controller=new AbortController();setState({versions:{},loading:!!ids});
    if(!ids)return()=>controller.abort();
    void Promise.all(ids.split(',').map(async id=>{try{
      const item=await apiRequest<Schema['CatalogVersion']>(`/versions/${id}`,{signal:controller.signal});
      if(item.kind!=='agent'||!item.spec.agent)throw Error('版本不是共享 Agent 角色');
      return [id,item] as const;
    }catch(error){if(controller.signal.aborted)throw error;return [id,undefined] as const;}})).then(items=>{
      if(!controller.signal.aborted)setState({versions:Object.fromEntries(items.flatMap(([id,v])=>v?[[id,v]]:[])),loading:false});
    }).catch(()=>{if(!controller.signal.aborted)setState({versions:{},loading:false,error:Error('读取助手版本失败')});});
    return()=>controller.abort();
  },[ids]);return state;
}

export function AssistantLibrary({disabled,add}:{disabled:boolean;add:(choice:Schema['RunnableChoicePage']['items'][number])=>void}) {
  const [offset,setOffset]=useState(0),[search,setSearch]=useState('');
  const query=useQuery<Schema['RunnableChoicePage']>(`/runnables?offset=${offset}`);
  const choices=query.data?.items.filter(item=>item.version.kind==='agent'&&!!item.version.spec.agent&&!item.version.archived)??[];
  return <div className="flow-library"><h2>添加 Agent 角色</h2><label className="field">搜索本页助手<input value={search} onChange={e=>setSearch(e.target.value)} placeholder="按名称查找"/></label>
    <QueryState {...query} retry={query.reload}/>
    {choices.filter(item=>item.name.includes(search)).map(item=><button type="button" className="flow-library-item" key={item.version.id} disabled={disabled} onClick={()=>add(item)}><span className={`flow-kind ${item.version.spec.agent?.executor}`}>{item.version.spec.agent?.executor==='codex'?'C':'E'}</span><span><strong>{item.name}</strong><small>{item.version.spec.agent?.executor==='codex'?'Codex':'Eino'} · v{item.version.number}</small></span><span aria-hidden="true">＋</span></button>)}
    {!query.loading&&!choices.length&&<p className="muted">本页没有已发布助手。<a href="#/agent">准备助手</a></p>}
    {(offset>0||query.data?.next_offset!==undefined)&&<div className="toolbar"><button type="button" disabled={!offset} onClick={()=>setOffset(0)}>首页</button><button type="button" disabled={query.data?.next_offset===undefined} onClick={()=>setOffset(query.data!.next_offset!)}>下一页</button></div>}
    <p className="muted">点击添加到画布，再拖动圆点连接步骤。支持最多 32 个步骤。</p>
  </div>;
}
