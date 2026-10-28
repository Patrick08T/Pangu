import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Pager,QueryState,timestamp,type Schema} from '../console/components';
import {differences,engineName,type Draft} from './model';

export function AgentVersions({item,draft,disabled,restore,test}:{item:Schema['CatalogObject'];draft:Draft;disabled:boolean;restore:(version:Schema['CatalogVersion'])=>void;test:(version:Schema['CatalogVersion'])=>void}){
  const [offset,setOffset]=useState(0),[selected,select]=useState<Schema['CatalogVersion']>();
  const query=useQuery<Schema['CatalogVersion'][]>(`/catalog/${item.id}/versions?offset=${offset}`);
  const version=selected??query.data?.[0];
  return <section className="agent-versions"><h2>版本历史</h2><p className="muted">正式版本不可修改。恢复后生成新的草稿；现有流程和定时计划继续使用原版本。</p>
    <QueryState {...query} retry={query.reload} empty={query.data?.length===0?'尚未发布；先保存职责与模型配置，再发布第一个版本。':undefined}/>
    {query.data?.length!==0&&<label className="field">比较发布版本<select value={version?.id??''} onChange={e=>select(query.data?.find(v=>v.id===e.target.value))}>{query.data?.map(v=><option key={v.id} value={v.id}>v{v.number} · 来源 r{v.source_revision} · {timestamp(v.created_at)}</option>)}</select></label>}
    {version&&<><p>{engineName(version.spec.agent!)}</p><div className="toolbar"><button disabled={disabled} onClick={()=>test(version)}>在右侧测试此版本</button><button disabled={disabled} onClick={()=>restore(version)}>恢复此版本为新草稿</button><a href={`#/start/${version.id}`}>使用固定版本</a><a href="#/evaluations">进入固定样本评测</a></div>
      <h3>与当前草稿比较</h3><AgentDifference before={version.spec} after={draft.spec}/>
      <details><summary>查看该版本职责</summary><pre>{version.spec.agent?.instruction}</pre></details>
    </>}
    <Pager offset={offset} next={query.data?.length===100?offset+100:undefined} change={next=>{select(undefined);setOffset(next);}}/>
  </section>;
}
export function AgentDifference({before,after}:{before:Schema['CatalogSpec'];after:Schema['CatalogSpec']}){
  const rows=differences(before,after);
  return rows.length?<div className="agent-difference">{rows.map(row=><article key={row.name}><h4>{row.name}</h4><div><section><small>原内容</small><pre>{row.before}</pre></section><section><small>当前内容</small><pre>{row.after}</pre></section></div></article>)}</div>:<p>配置内容一致。</p>;
}
