import { useState } from 'react';
import { useQuery } from '../api/hooks';
import { Pager,QueryState,Status,timestamp,type Schema } from '../console/components';

export function Attempts({run}:{run:string}) {
  const [offset,setOffset]=useState(0),[selected,select]=useState<Schema['Attempt']>();
  const query=useQuery<Schema['AttemptPage']>(`/runs/${run}/attempts?offset=${offset}&limit=20`,5000);
  return <section><h2>节点尝试与工具证据</h2><QueryState {...query} retry={query.reload} empty={query.data&&!query.data.items.length?'尚无执行尝试。':undefined}/>
    <div className="table-scroll"><table><thead><tr><th>节点 / 尝试</th><th>状态</th><th>开始</th><th>结束</th><th>证据</th></tr></thead><tbody>{query.data?.items.map(item=><tr key={`${item.node_run_id}/${item.attempt}`}>
      <td>{item.node_id} / {item.attempt}</td><td><Status value={item.status}/></td><td>{timestamp(item.started_at)}</td><td>{timestamp(item.finished_at)}</td><td><button onClick={()=>select(item)}>查看证据</button></td>
    </tr>)}</tbody></table></div>{query.data&&<Pager offset={offset} next={query.data.next_offset} change={setOffset}/>}
    {selected&&<AttemptDetail key={`${selected.node_run_id}/${selected.attempt}`} run={run} item={selected}/>}
  </section>;
}

export function AttemptDetail({run,item,stopped=false}:{run:string;item:Pick<Schema['Attempt'],'node_run_id'|'node_id'|'attempt'>;stopped?:boolean}) {
  const query=useQuery<Schema['Attempt']>(`/runs/${run}/nodes/${item.node_run_id}/attempts/${item.attempt}`,stopped?0:5000);
  return <div className="evidence"><h3>{item.node_id} · 第 {item.attempt} 次尝试</h3><QueryState {...query} retry={query.reload}/>{query.data&&<>
    <p>错误分类：{query.data.category||'无'}{query.data.truncated?'；诊断已截断':''}</p><pre>{query.data.message||'没有错误诊断。'}</pre>
    {query.data.evidence.map((value,index)=><details key={`${value.id}/${index}`}><summary>{value.command}（退出码 {value.exit_code}）</summary><pre>{value.output}</pre>{value.truncated&&<p>工具输出已截断。</p>}</details>)}
    {!query.data.evidence.length&&<p>没有可验证的工具输出记录。</p>}{query.data.evidence_truncated&&<p>工具证据达到上限，部分记录未保存。</p>}
  </>}</div>;
}
