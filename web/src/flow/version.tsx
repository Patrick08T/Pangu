import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {QueryState,timestamp,type Schema} from '../console/components';
import {FlowCanvas} from './canvas';
import {useVersions} from './catalog';
import {StepPanel,FlowSettings} from './panel';
import type {Flow} from './model';

export function FlowVersions({id,canRun}:{id:string;canRun:boolean}) {
  const [offset,setOffset]=useState(0),[selected,setSelected]=useState<Schema['CatalogVersion']>();
  const query=useQuery<Schema['CatalogVersion'][]>(`/catalog/${id}/versions?offset=${offset}`);
  return <section className="flow-versions"><h2>发布历史</h2><QueryState {...query} retry={query.reload} empty={query.data?.length===0?'还没有发布版本。':undefined}/>
    <div className="toolbar">{query.data?.map(version=><button type="button" key={version.id} onClick={()=>setSelected(version)}>查看 v{version.number} · {timestamp(version.created_at)}</button>)}{(offset>0||query.data?.length===100)&&<><button type="button" disabled={!offset} onClick={()=>setOffset(0)}>首页</button><button type="button" disabled={query.data?.length!==100} onClick={()=>setOffset(offset+100)}>下一页</button></>}</div>
    {selected&&<div className="flow-published"><div className="section-heading"><h3>v{selected.number} · 只读发布版本</h3><button type="button" onClick={()=>setSelected(undefined)}>收起版本</button></div>{selected.spec.flow?<FlowPreview flow={selected.spec.flow}/>:<p>此历史版本不是工作流。</p>}
      {canRun&&!selected.archived&&<div className="toolbar"><a className="button primary" href={`#/start/${selected.id}`}>运行此版本</a><a className="button" href={`#/schedule/new/${selected.id}`}>设置定时任务</a></div>}
    </div>}
  </section>;
}
function FlowPreview({flow}:{flow:Flow}) {
  const {versions}=useVersions(flow),[selected,select]=useState<string>();const step=flow.nodes.find(n=>n.id===selected);
  return <div className="flow-preview"><FlowCanvas flow={flow} versions={versions} selected={selected} select={select} change={()=>{}} disabled/>{step?<StepPanel step={step} flow={flow} versions={versions} disabled change={()=>{}} copy={()=>{}} remove={()=>{}}/>:<FlowSettings flow={flow} change={()=>{}} disabled/>}</div>;
}
