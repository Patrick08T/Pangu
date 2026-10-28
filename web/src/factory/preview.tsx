import {useMemo} from 'react';
import {ReactFlow,Background,Controls,MarkerType} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import type {Schema} from '../console/components';
import {useQuery} from '../api/hooks';
import {QueryState} from '../console/components';
import {AgentDifference} from '../agent/version';

const fields={objective:'任务目标',inputs:'输入与上下文',deliverables:'交付物',acceptance:'验收标准',failure_handling:'异常处理'} as const;
export function Proposal({turn,disabled,apply}:{turn:Schema['FactoryTurn'];disabled:boolean;apply:()=>void}) {
 const proposal=turn.reply?.proposal;
 if(turn.result)return <section className="factory-result"><h3>已创建并发布</h3><p>助手和流程已就绪。只有在运行页面填写任务并点击“开始执行”后，才会执行工作。</p>{turn.result.workflow&&<div><a className="button primary" href={`#/onboarding/${turn.result.workflow.id}`}>运行此流程</a><a href={`#/catalog/${turn.result.workflow.object_id}`}>查看可视化流程</a></div>}<ul>{turn.result.agents.map(v=><li key={v.id}><a href={`#/catalog/${v.object_id}`}>{v.spec.agent?.description||v.spec.agent?.id||'助手'} · v{v.number}</a> <a href={`#/onboarding/${v.id}`}>运行此助手</a></li>)}</ul></section>;
 if(!proposal)return null;
 return <section className="factory-proposal" aria-label="候选方案预览"><div className="factory-proposal-heading"><h3>{proposal.name}</h3><span>待确认 · 尚未创建</span></div><p>{proposal.agents.length} 个助手 · {proposal.steps?.length??0} 个流程步骤</p>{proposal.object_id&&<p className="notice">更新现有流程的修订 {proposal.revision}；历史发布版本保留。</p>}
  {!!proposal.steps?.length&&<Graph proposal={proposal}/>}
  {turn.input.target_id&&<Difference turn={turn} proposal={proposal}/>}
  {proposal.agents.map(agent=><details key={agent.key} className="factory-agent" open={proposal.agents.length===1}><summary>{agent.object_id?'修改':agent.version_id?'复用':'新建'} · {agent.name} <small>{agent.node?.executor?.toUpperCase()}</small></summary>{agent.node?<><p>{agent.node.description}</p><p className="factory-instruction">{agent.node.instruction}</p><div className="factory-permissions"><span>{agent.node.executor==='codex'?(agent.node.read_only?'文件只读':'允许修改项目文件'):'Eino 受限工具'}</span><span>重试 {agent.node.policy?.retries??2} 次</span></div><dl>{Object.entries(fields).map(([key,label])=><div key={key}><dt>{label}</dt><dd>{agent.node?.handoff?.[key as keyof typeof fields]}</dd></div>)}</dl></>:<p>复用固定发布版本：<a href={`#/catalog/${agent.object_id??''}`}>{agent.version_id}</a></p>}</details>)}
  {proposal.steps?.some(s=>s.when||s.approval||s.bindings)&&<details><summary>条件、审批与变量映射</summary>{proposal.steps.map(s=><div key={s.id}><strong>{s.id}</strong>{s.when&&<p>条件：{s.when.field.from}{s.when.field.pointer} 等于 {JSON.stringify(s.when.equals)}</p>}{s.approval&&<p>需人工审批，等待上限 {s.approval.timeout_seconds} 秒</p>}{s.bindings&&Object.entries(s.bindings).map(([name,b])=><p key={name}>{name} ← {b.from}{b.pointer}（{b.type}）</p>)}</div>)}</details>}
  {proposal.budget&&<p>运行预算上限：模型 {proposal.budget.model_calls} 次 / Codex 会话 {proposal.budget.codex_sessions} 次 / 搜索 {proposal.budget.search_calls} 次</p>}
  <div className="factory-confirm"><button className="primary" disabled={disabled||turn.status!=='ready'} onClick={apply}>确认创建并发布</button><small>一次确认整套方案。发布后不会自动运行；继续对话可修改候选。</small></div>
 </section>;
}

function Difference({turn,proposal}:{turn:Schema['FactoryTurn'];proposal:Schema['FactoryProposal']}) {
 const query=useQuery<Schema['CatalogObject']>(`/catalog/${turn.input.target_id}`),before=query.data;
 const after=proposal.agents.find(a=>a.object_id===before?.id);
 return <details className="factory-difference"><summary>与现有配置比较</summary><QueryState {...query} retry={query.reload}/>{before&&(before.revision!==turn.input.target_revision?<p role="alert">原配置已被更新，请重新读取目标并生成候选，当前方案不能直接覆盖。</p>:<><p>原配置：{before.name} · 修订 {before.revision}</p>{before.spec.agent&&after?.node?<AgentDifference before={before.spec} after={{agent:after.node}}/>:before.spec.flow&&proposal.object_id===before.id?<div className="table-scroll"><table><thead><tr><th>步骤</th><th>原依赖</th><th>候选依赖</th><th>候选职责</th></tr></thead><tbody>{[...new Set([...before.spec.flow.nodes.map(n=>n.id),...(proposal.steps??[]).map(n=>n.id)])].map(id=>{const old=before.spec.flow?.nodes.find(n=>n.id===id),next=proposal.steps?.find(n=>n.id===id);return <tr key={id}><td>{id}</td><td>{old?old.depends_on?.join('、')||'开始':'新增'}</td><td>{next?next.depends_on?.join('、')||'开始':'移除'}</td><td>{proposal.agents.find(a=>a.key===next?.agent)?.name??'—'}</td></tr>;})}</tbody></table></div>:<p>本次新建资源，原配置保留。</p>}</>)}</details>;
}

function Graph({proposal}:{proposal:Schema['FactoryProposal']}) {
 const {nodes,edges}=useMemo(()=>{
  const steps=proposal.steps??[],depth:Record<string,number>={};
  for(let pass=0;pass<steps.length;pass++)for(const s of steps)depth[s.id]=Math.max(0,...(s.depends_on??[]).map(id=>(depth[id]??0)+1));
  const rows:Record<number,number>={};
  const nodes=steps.map(s=>{const x=depth[s.id];const y=rows[x]??0;rows[x]=y+1;return {id:s.id,position:{x:x*230,y:y*120},data:{label:`${proposal.agents.find(a=>a.key===s.agent)?.name??s.agent}${s.when?' · 条件':''}${s.approval?' · 审批':''}`}};});
  const edges=steps.flatMap(s=>(s.depends_on??[]).map(id=>({id:`${id}:${s.id}`,source:id,target:s.id,type:'smoothstep',markerEnd:{type:MarkerType.ArrowClosed}})));
  return {nodes,edges};
 },[proposal]);
 return <><div className="factory-graph" aria-label="候选流程画布"><ReactFlow nodes={nodes} edges={edges} fitView nodesDraggable={false} nodesConnectable={false} deleteKeyCode={null}><Background/><Controls showInteractive={false}/></ReactFlow></div><ol className="factory-steps">{proposal.steps?.map(s=><li key={s.id}>{s.id}：{proposal.agents.find(a=>a.key===s.agent)?.name}（{s.depends_on?.length?`等待 ${s.depends_on.join('、')}`:'开始步骤'}）</li>)}</ol></>;
}
