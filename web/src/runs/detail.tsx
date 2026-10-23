import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,QueryState,Status,timestamp,canEdit,isManager,type Access,type Schema} from '../console/components';
import {WorkflowGraph} from './workflow';
import {Artifacts} from './artifact';
import {Attempts} from './execution';
import {Events} from './events';
import {RequestReceipt,receipt as receiptID} from './submission';

export function RunDetail({id,access}:{id:string;access:Access}) {
  const query=useQuery<Schema['RunDetails']>(`/runs/${encodeURIComponent(id)}`,5000);
  const action=useAction(),rerunKey=useRef(crypto.randomUUID());
  const [receipt,setReceipt]=useState<string>(),[confirmRerun,setConfirmRerun]=useState(false);
  const run=query.data;
  const scoped=!!run&&run.project_id===access.project;
  const cancelAllowed=scoped&&(isManager(access)||(canEdit(access)&&run.created_by===access.session.id));
  async function cancel(){await apiRequest(`/runs/${id}/cancel`,{method:'POST',csrf:access.session.csrf});query.reload();}
  async function rerun(){const item=await apiRequest<Schema['Submission']>(`/runs/${id}/rerun`,{method:'POST',csrf:access.session.csrf,key:rerunKey.current});setReceipt(receiptID(item));}
  return <Page title="运行详情" description={id} actions={<a className="button" href="#/runs">返回列表</a>}>
    <QueryState {...query} retry={query.reload}/>{run&&<>
      <div className="run-overview"><Status value={run.status}/><span>创建 {timestamp(run.created_at)}</span><span>结束 {timestamp(run.finished_at)}</span><span>{run.stop_confirmed?'已确认执行停止':'尚未确认执行停止'}</span></div>
      {run.source_run_id&&<p>重新运行来源：<a href={`#/runs/${run.source_run_id}`}>{run.source_run_id}</a></p>}
      {!scoped&&<p className="notice">此运行属于项目 {run.project_id??'未登记'}。切换到该项目后可执行有权限的操作。</p>}
      <div className="toolbar">{cancelAllowed&&!run.stop_confirmed&&<button disabled={action.busy} onClick={()=>void action.run(cancel,'取消请求已记录，正在等待所有节点停止')}>请求取消</button>}
        {scoped&&canEdit(access)&&run.stop_confirmed&&!run.logs_expired&&<button disabled={action.busy} onClick={()=>setConfirmRerun(value=>!value)}>整次重新运行</button>}</div>
      {confirmRerun&&<div className="notice"><p>创建新运行，固定原定义与原 Git 基准，从干净工作区执行全部节点；旧产物保留。原基准已过期时会拒绝。</p><button disabled={action.busy} onClick={()=>void action.run(rerun)}>确认创建新运行</button></div>}
      <ActionNotice {...action}/>{receipt&&<RequestReceipt id={receipt}/>}
      {run.error&&<div className="notice error"><strong>{run.error.category}</strong><pre>{run.error.message}</pre>{run.error.truncated&&<p>错误内容已截断。</p>}</div>}
      {run.definition?<WorkflowGraph definition={run.definition} nodes={run.nodes}/>:<p>此运行没有可展示的产品定义快照。</p>}
      <section><h2>节点</h2>{run.nodes.map(node=><article className="node-result" key={node.id}><h3 id={`node-${node.node_id}`} tabIndex={-1}>{node.node_id} <Status value={node.status}/></h3>
        <p>{node.executor} · 第 {node.attempt} 次尝试 · {timestamp(node.started_at)} 至 {timestamp(node.finished_at)}</p>
        {node.error&&<pre className="error-text">{node.error.category}：{node.error.message}</pre>}<pre>{node.output||'尚无输出。'}</pre></article>)}</section>
      {run.logs_expired?<p className="notice">日志、产物及重跑基准已超过 30 天保留期，无法下载或整次重新运行。</p>:<><Attempts run={id}/><Artifacts run={id}/></>}
      <details><summary>运行定义快照</summary><pre>{JSON.stringify(run.definition,null,2)}</pre></details>
      {!run.logs_expired&&<Events run={id} nodes={run.nodes}/>}
    </>}
  </Page>;
}
