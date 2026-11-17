import {useState} from 'react';
import {useQuery,useAction} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,QueryState,timestamp,type Schema} from '../console/components';
import {Lifecycle} from './status';
export type EntriesProps={project:string;csrf:string;permissions:{revoke?:boolean;delete?:boolean}};
const prefix=(project:string)=>`/projects/${encodeURIComponent(project)}/experiences`;

export function Entries(props:EntriesProps){
  const [cursor,setCursor]=useState(''),[selected,setSelected]=useState('');
  const query=useQuery<Schema['ExperiencePage']>(`${prefix(props.project)}?limit=20${cursor?`&cursor=${encodeURIComponent(cursor)}`:''}`);
  return <section><h2>经验目录</h2><p>真实性状态与投放状态分开。组合收益不等于每条经验的独立贡献；一次成功不能归因于经验。</p>
    <QueryState {...query} retry={query.reload} empty={!query.data?.items?.length?'暂无经验记录。':undefined}/>
    {!query.error&&!query.loading&&query.data?.items?.map(item=><article className="panel" key={item.id}><h3>{item.entry_id}</h3><p>修订 {item.revision} · 投放修订 {item.delivery_revision}</p><Lifecycle validity={item.validity} rollout={item.delivery}/><p>保留至 {timestamp(item.expires_at)}</p><button aria-label={`查看经验 ${item.entry_id} 修订 ${item.revision}`} onClick={()=>setSelected(item.id)}>查看来源与使用记录</button></article>)}
    <CursorPager cursor={cursor} next={!query.error?query.data?.next_cursor:undefined} busy={query.loading} change={value=>{setCursor(value);setSelected('');}}/>
    {selected&&<ExperienceDetail key={`${props.project}:${selected}`} {...props} id={selected} close={()=>setSelected('')} changed={query.reload}/>}
  </section>;
}

function ExperienceDetail({id,close,changed,...props}:EntriesProps&{id:string;close:()=>void;changed:()=>void}){
  const query=useQuery<Schema['ExperienceVersion']>(`${prefix(props.project)}/${encodeURIComponent(id)}`),[usages,setUsages]=useState(false);
  const version=!query.loading&&!query.error?query.data:undefined;
  return <section className="panel"><h2>经验详情</h2><button onClick={close}>关闭详情</button><QueryState {...query} retry={query.reload}/>
    {version&&<><Lifecycle validity={version.validity} rollout={version.delivery}/><p>保留至 {timestamp(version.expires_at)}</p>
      {version.invalid_reason&&<p className="notice error">失效原因：{version.invalid_reason}</p>}
      {version.tombstoned?<p>正文已清理，不可读取或重新采用。</p>:<><dl><dt>观察事实</dt><dd>{version.content?.observation}</dd><dt>适用条件</dt><dd>{version.content?.conditions}</dd><dt>建议做法</dt><dd>{version.content?.practice}</dd><dt>反例与限制</dt><dd>{version.content?.counterexamples}</dd></dl>
        <h3>经服务授权的来源</h3><ul>{version.sources?.map(source=><li key={`${source.kind}:${source.id}`}>{source.kind} · {source.id} · 节点 {source.node_id}{source.run_id&&<> · <a href={`#/runs/${encodeURIComponent(source.run_id)}`}>查看来源运行</a></>}</li>)}</ul><p>来源引用不代表已复核原文；来源撤销或过期会阻止后续读取与复用。</p></>}
      <VersionActions {...props} id={id} invalid={version.tombstoned||version.validity==='revoked'} done={()=>{query.reload();changed();}}/>
      <details onToggle={event=>setUsages(event.currentTarget.open)}><summary>查看使用记录</summary>{usages&&<VersionUsages project={props.project} id={id}/>}</details>
    </>}
  </section>;
}

function VersionActions({id,invalid,done,...props}:EntriesProps&{id:string;invalid?:boolean;done:()=>void}){
  const [reason,setReason]=useState(''),[confirmed,setConfirmed]=useState(false),action=useAction();
  async function mutate(kind:'revoke'|'delete',signal:AbortSignal){
    await apiRequest(`${prefix(props.project)}/${encodeURIComponent(id)}${kind==='revoke'?'/revoke':''}`,{method:kind==='revoke'?'POST':'DELETE',csrf:props.csrf,body:{reason:reason.trim()},signal});
    if(action.active()){setConfirmed(false);done();}
  }
  return <div><h3>失效与正文清理</h3><p>撤销阻止新的读取并使衍生依赖失效。删除清理正文，保留必要审计；两者都不撤销已经发生的业务副作用。</p>
    <label className="field">变更理由<input value={reason} disabled={action.busy} onChange={event=>setReason(event.target.value)}/></label>
    <label className="check"><input type="checkbox" checked={confirmed} disabled={action.busy} onChange={event=>setConfirmed(event.target.checked)}/>确认影响衍生成果与审批依赖</label>
    <div className="toolbar"><button disabled={props.permissions.revoke!==true||!props.csrf||invalid||!confirmed||!reason.trim()||action.busy} onClick={()=>void action.run(signal=>mutate('revoke',signal),'经验已撤销；依赖失效以服务记录为准')}>撤销此经验</button>
      <button disabled={props.permissions.delete!==true||!props.csrf||!confirmed||!reason.trim()||action.busy} onClick={()=>void action.run(signal=>mutate('delete',signal),'经验正文清理已由服务确认')}>删除经验正文</button></div>
    <ActionNotice {...action}/>
  </div>;
}

function VersionUsages({project,id}:{project:string;id:string}){
  const [cursor,setCursor]=useState('');
  const query=useQuery<Schema['ExperienceUsagePage']>(`${prefix(project)}/${encodeURIComponent(id)}/usages?limit=20${cursor?`&cursor=${encodeURIComponent(cursor)}`:''}`);
  return <section><QueryState {...query} retry={query.reload} empty={!query.data?.items?.length?'暂无可查看的使用记录。':undefined}/>
    {!query.error&&!query.loading&&query.data?.items?.map((usage,index)=><article key={`${usage.run_id}:${usage.node_id}:${index}`}><span>运行 / 提交标识：{usage.run_id}</span><p>节点 {usage.node_id} · 轮次 {usage.iteration} · 审批 generation {usage.approval_generation} · 审批修订 {usage.approval_revision}</p><p>{timestamp(usage.created_at)}</p></article>)}
    <p>此列表记录曾经使用，不表示依赖现在仍有效，也不是单条经验收益证明。</p><CursorPager cursor={cursor} next={!query.error?query.data?.next_cursor:undefined} busy={query.loading} change={setCursor}/>
  </section>;
}

export function CursorPager({cursor,next,busy,change}:{cursor:string;next?:string;busy:boolean;change:(value:string)=>void}){
  return <div className="pager"><button disabled={!cursor||busy} onClick={()=>change('')}>第一页</button><button disabled={!next||busy} onClick={()=>next&&change(next)}>下一页</button></div>;
}
