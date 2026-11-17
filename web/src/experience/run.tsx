import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Pager,QueryState,timestamp,type Schema} from '../console/components';

const states:Record<string,string>={waiting:'等待审核',approved:'已批准，等待执行',consumed:'已用于执行',changes_requested:'人工纠偏',rejected:'人工驳回',revoked:'批准已撤回',expired:'已过期',invalidated:'系统失效'};
export function RunApprovalGenerations({project,run}:{project:string;run:string}){
  const [offset,setOffset]=useState(0);
  const query=useQuery<Schema['ApprovalPage']>(`/approvals?project_id=${encodeURIComponent(project)}&run_id=${encodeURIComponent(run)}&offset=${offset}&limit=20`);
  return <section><h3>审批代际与失效</h3><p>系统失效不等于人工纠偏，也不代表可以继续使用旧批准。以下仅是审批摘要，不替代完整经验影响链或有效性检查。</p>
    <QueryState {...query} retry={query.reload} empty={query.data?.items?.length===0?'当前页没有审批记录；不能据此推断没有使用经验。':undefined}/>
    {!query.loading&&!query.error&&query.data?.items?.map(item=><article className="panel" key={item.id}>
      <h4>{item.node_id}</h4><p>{Number.isSafeInteger(item.approval_generation)&&item.approval_generation>0?`审批 generation ${item.approval_generation} · 第 ${item.round} 轮`:'历史记录未登记审批代际'}</p>
      <p>{states[item.status]??`未知审批状态（${item.status}）`}{item.status==='invalidated'&&item.invalidation_reason?`：${item.invalidation_reason}`:''}</p>
      {item.invalidated_at&&<p>系统失效时间：{timestamp(item.invalidated_at)}</p>}<a href={`#/approvals/${encodeURIComponent(item.id)}`}>查看此代审批</a>
    </article>)}
    {!query.loading&&!query.error&&query.data&&<Pager offset={offset} next={query.data.next_offset} change={setOffset}/>}<button onClick={query.reload} disabled={query.loading}>刷新审批代际</button>
  </section>;
}
