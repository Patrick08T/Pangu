import {Status,type Schema} from '../console/components';

export function currentNodes(nodes:Schema['NodeRun'][]):Schema['NodeRun'][] {
  const rounds=new Map<string,number>(),latest=new Map<string,Schema['NodeRun']>();
  for(const n of nodes)if(n.loop_id)rounds.set(n.loop_id,Math.max(rounds.get(n.loop_id)??0,n.loop_round??0));
  for(const n of nodes){
    if(n.loop_id&&n.loop_round!==rounds.get(n.loop_id))continue;
    const prior=latest.get(n.node_id);
    if(!prior||Date.parse(n.created_at)>Date.parse(prior.created_at))latest.set(n.node_id,n);
  }
  return [...latest.values()];
}

export function gateResult(node?:Schema['NodeRun']):{passed:boolean;summary:string}|undefined {
  if(!node?.output||node.status!=='succeeded')return;
  try{const result=JSON.parse(node.output);if(typeof result.passed==='boolean'&&typeof result.summary==='string')return result;}catch{/* 未获得结构化质量结论。 */}
}

function GateStatus({node}:{node?:Schema['NodeRun']}) {
  const result=gateResult(node);
  if(result)return <>{result.passed?'已通过':'未通过，进入返修'}</>;
  if(node)return <Status value={node.status}/>;
  return <>本轮尚未执行</>;
}

export function LoopProgress({definition,nodes}:{definition:Schema['Workflow'];nodes:Schema['NodeRun'][]}) {
  if(!definition.loops?.length)return null;
  return <section aria-label="循环修复进度"><h2>循环修复进度</h2><p>同一轮、同一代码的全部质量门通过后完成。未通过会保留补丁进入下一轮；模型重试单独计数。</p>
    {definition.loops.map(loop=>{
      const history=nodes.filter(n=>n.loop_id===loop.id),round=Math.max(1,...history.map(n=>n.loop_round??1)),current=currentNodes(history);
      const rounds=[...new Set(history.map(n=>n.loop_round??1))].sort((a,b)=>b-a);
      return <article key={loop.id}><h3>{loop.id} · 第 {round} 轮</h3>
        <ul>{loop.gates.map(id=><li key={id}>{id}：<GateStatus node={current.find(n=>n.node_id===id)}/></li>)}</ul>
        <details><summary>查看各轮记录（{rounds.length} 轮）</summary>{rounds.map(r=><div key={r}><h4>第 {r} 轮</h4>{history.filter(n=>n.loop_round===r).map(n=><div key={n.id}><p>{n.node_id} · <Status value={n.status}/>{n.model_retry?` · 模型重试 ${n.model_retry}`:''}</p>{gateResult(n)&&<p>{gateResult(n)!.summary}</p>}{n.error&&<p>{n.error.message}</p>}</div>)}</div>)}</details>
      </article>;
    })}
  </section>;
}
