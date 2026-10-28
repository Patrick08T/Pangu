import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,QueryState,Status,timestamp,canEdit,isManager,type Access,type Schema} from '../console/components';
import {RunDebugger} from './debug';
import {DebugControls,TracePanel} from './trace';
import {Artifacts} from './artifact';
import {Attempts} from './execution';
import {Feedback} from '../evaluation/feedback';
import {Approvals} from '../approval/workflow';
import {Reports} from '../research/evidence';
import {ReportQuality} from '../research/quality';
import {Usage} from '../usage/workflow';
import {Events} from './events';
import {ForkRun} from './fork';
import {RequestReceipt,receipt as receiptID} from './submission';

export function RunDetail({id,access}:{id:string;access:Access}) {
  const query=useQuery<Schema['RunDetails']>(`/runs/${encodeURIComponent(id)}`,5000);
  const action=useAction(),rerunKey=useRef(crypto.randomUUID());
  const [receipt,setReceipt]=useState<string>(),[confirmRerun,setConfirmRerun]=useState(false),[process,setProcess]=useState(false);
  const run=query.data;
  const scoped=!!run&&run.project_id===access.project;
  const cancelAllowed=scoped&&(isManager(access)||(canEdit(access)&&run.created_by===access.session.id));
  async function cancel(){await apiRequest(`/runs/${id}/cancel`,{method:'POST',csrf:access.session.csrf});query.reload();}
  async function rerun(){const item=await apiRequest<Schema['Submission']>(`/runs/${id}/rerun`,{method:'POST',csrf:access.session.csrf,key:rerunKey.current});setReceipt(receiptID(item));}
  return <Page title={run?.title||'任务详情'} description="查看 Agent 状态，点击节点了解当前活动和执行记录。" actions={<a className="button" href="#/runs">返回任务记录</a>}>
    <QueryState {...query} retry={query.reload}/>{run&&<>
      <div className="run-overview"><Status value={run.status}/><span>创建 {timestamp(run.created_at)}</span><span>结束 {timestamp(run.finished_at)}</span><span>{run.stop_confirmed?'已确认执行停止':'尚未确认执行停止'}</span></div>
      {run.status==='succeeded'&&<p className="muted">执行已结束；结果是否满足任务目标，请结合下方资料检查和人工验收判断。</p>}
      {run.status==='succeeded'&&!run.logs_expired&&run.definition?.nodes.some(node=>node.role==='product_manager')&&<ReportQuality run={id}/>}
      {run.source_run_id&&<p>重新运行来源：<a href={`#/runs/${run.source_run_id}`}>{run.source_run_id}</a></p>}
      {run.fork&&<p>返工检查点：{run.fork.checkpoint_node||'整次重做'}；复用节点：{Object.keys(run.fork.reused??{}).join('、')||'无'}。新运行保留独立历史。</p>}
      {!scoped&&<p className="notice">此运行属于项目 {run.project_id??'未登记'}。切换到该项目后可执行有权限的操作。</p>}
      <div className="toolbar">{cancelAllowed&&!run.stop_confirmed&&<button disabled={action.busy} onClick={()=>void action.run(cancel,'取消请求已记录，正在等待所有节点停止')}>请求取消</button>}
        {scoped&&canEdit(access)&&run.stop_confirmed&&!run.logs_expired&&<button disabled={action.busy} onClick={()=>setConfirmRerun(value=>!value)}>整次重新运行</button>}</div>
      {confirmRerun&&<div className="notice"><p>创建新运行，固定原定义与原 Git 基准，从干净工作区执行全部节点；旧产物保留。原基准已过期时会拒绝。</p><button disabled={action.busy} onClick={()=>void action.run(rerun)}>确认创建新运行</button></div>}
      <ActionNotice {...action}/>{receipt&&<RequestReceipt id={receipt}/>}
      <DebugControls key={run.id} run={run} allowed={cancelAllowed} access={access}/>
      {run.error&&<div className="notice error"><strong>{run.error.category}</strong><pre>{run.error.message}</pre>{run.error.truncated&&<p>错误内容已截断。</p>}</div>}
      {run.definition&&<RunDebugger key={`agents-${run.id}`} run={run}/>}
      {scoped&&<Approvals run={id} access={access} stopped={run.stop_confirmed||run.cancel_requested}/>}
      {!run.stop_confirmed&&<p className="notice">任务正在推进，已完成的结果会自动显示。你可以返回任务记录，稍后继续查看。</p>}
      {run.logs_expired?<p className="notice">日志、产物及重跑基准已超过 30 天保留期，无法下载或重新运行。</p>:<><Artifacts run={id}/><section><h2>执行摘要</h2>{run.nodes.filter(node=>node.output||node.error).map(node=><article key={node.id}><h3>{node.node_id} <Status value={node.status}/></h3>{node.error&&<p className="error-text">{node.error.message}</p>}{node.output&&<pre className="report-body">{outputText(node.output)}</pre>}</article>)}</section></>}
      {scoped&&canEdit(access)&&run.stop_confirmed&&!run.logs_expired&&run.definition&&<details><summary>根据结果继续修改</summary><ForkRun key={run.id} run={run} access={access}/></details>}
      <details onToggle={event=>setProcess(event.currentTarget.open)}><summary>执行过程与日志</summary>{process&&<ExecutionDetails run={run}/>}</details>
      {scoped&&!run.logs_expired&&<details><summary>核验报告依据与继续交付</summary><Reports run={id} access={access} owner={run.created_by}/></details>}
      {scoped&&<details><summary>评价此次结果</summary><Feedback run={id} access={access} owner={run.created_by}/></details>}
      {scoped&&<details><summary>用量与费用</summary><Usage run={run}/></details>}
      <details><summary>任务编号与运行定义</summary><p className="identifier">{id}</p><pre>{JSON.stringify(run.definition,null,2)}</pre></details>
    </>}
  </Page>;
}

function outputText(value:string){try{const parsed=JSON.parse(value);return typeof parsed?.text==='string'?parsed.text:value;}catch{return value;}}

function ExecutionDetails({run}:{run:Schema['RunDetails']}) {
  return <>
    <TracePanel key={run.id} run={run}/>
    <section><h2>执行步骤</h2>{run.nodes.map(node=><article className="node-result" key={node.id}><h3 id={`node-${node.node_id}`} tabIndex={-1}>{node.node_id} <Status value={node.status}/></h3><p>{node.executor} · 第 {node.attempt} 次尝试 · {timestamp(node.started_at)} 至 {timestamp(node.finished_at)}</p>{node.reused_from&&<p>复用了此前成功的检查点。</p>}</article>)}</section>
    {!run.logs_expired&&<><Attempts run={run.id}/><Events run={run.id} nodes={run.nodes}/></>}
  </>;
}
