import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Pager,QueryState,type Schema} from '../console/components';

export function duration(start?:string|null,end?:string|null) {
 if(!start)return '未知';
 const milliseconds=(end?Date.parse(end):Date.now())-Date.parse(start);
 return Number.isFinite(milliseconds)?`${Math.max(0,milliseconds/1000).toFixed(1)} 秒${end?'':'（尚未结束）'}`:'未知';
}

export function Usage({run}:{run:Schema['RunDetails']}) {
 const [offset,setOffset]=useState(0),query=useQuery<Schema['UsageReport']>(`/runs/${run.id}/usage?offset=${offset}`,10000);
 const labels={model_calls:'Eino 模型请求',search_calls:'搜索请求',codex_sessions:'Codex 会话'};
 return <section><h2>用量、预算与耗时</h2><QueryState {...query} retry={query.reload}/>
  <p>用量按整次运行累计，包含重试；主动填写预算时，在请求前原子预留，异常或结果未知不退回。未设置预算时没有默认调用上限。已发请求仍可能收费。Codex 会话内的模型和工具调用可在行为追踪中查看。</p>
  {query.data&&<><div className="toolbar">{Object.entries(labels).map(([key,label])=><span key={key}>{label}：{query.data!.used[key as keyof typeof labels]} / {query.data!.budget?.[key as keyof typeof labels]??'未设上限'}</span>)}</div>
   <p>账单实值：当前提供方接口未提供。Eino token 来自 HTTP 响应；Codex token 为原生 CLI 报告值，全零默认值视为未知。下表金额仅按连接版本中登记的单价估算，未计缓存折扣、阶梯价、税费；没有 usage 或单价时显示未知。Codex 订阅不折算单次金额。</p>
   <table><thead><tr><th>节点 / 尝试</th><th>调用类型</th><th>输入 / 输出 token</th><th>价格估算</th><th>调用耗时</th></tr></thead><tbody>{query.data.calls.map(call=><tr key={call.id}><td><a href={`#node-${call.node_id}`} onClick={e=>{e.preventDefault();document.getElementById(`node-${call.node_id}`)?.focus();}}>{call.node_id}</a> / {call.attempt}</td><td>{call.kind==='model'?'Eino 模型':call.kind==='search'?'公开搜索':'Codex 会话'} · {call.model}</td><td>{call.input_tokens??'未知'} / {call.output_tokens??'未知'}</td><td>{call.estimate_micros===null?'未知':`${(call.estimate_micros/1000000).toFixed(6)} ${call.price?.currency??''}（估算）`}</td><td>{duration(call.created_at,call.finished_at)}</td></tr>)}</tbody></table>
   <Pager offset={offset} next={query.data.next_offset} change={setOffset}/></>}
  <h3>节点等待与执行</h3><p>排队从子任务创建到开始；执行包含工作区锁、模型、工具和验证。并行节点耗时不能相加当作总耗时；重试细节见尝试记录。</p>
  <table><thead><tr><th>节点</th><th>排队</th><th>执行</th></tr></thead><tbody>{run.nodes.map(node=><tr key={node.id}><td>{node.node_id}</td>{node.reused_from||node.status==='skipped'?<td colSpan={2}>{node.reused_from?'复用，无新增执行':'已跳过，未派发'}</td>:<><td>{duration(node.created_at,node.started_at)}</td><td>{node.started_at?duration(node.started_at,node.finished_at):'尚未开始'}</td></>}</tr>)}</tbody></table>
 </section>;
}
