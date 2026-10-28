import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {APIError,apiRequest} from '../api/request';
import {ActionNotice,QueryState,canEdit,isManager,timestamp,type Access,type Schema} from '../console/components';
import {duration} from '../usage/workflow';

const labels:Record<string,string>={waiting:'等待审核',approved:'已批准，等待执行',rejected:'已驳回',revoked:'批准已撤回',consumed:'已用于执行',expired:'已过期'};
export function Approvals({run,access,stopped}:{run:string;access:Access;stopped:boolean}) {
  const query=useQuery<Schema['ApprovalGate'][]>(`/runs/${run}/approvals`,5000);
  if(query.data?.length===0&&!query.error)return null;
  return <section><h2>节点人工审核</h2><QueryState {...query} retry={query.reload}/>{query.data?.length===0&&<p>当前没有需要人工审核的节点。</p>}
    {query.data?.map(gate=><article className="node-result" key={gate.id}><h3>{gate.node_id} · {labels[gate.status]}</h3>
      <p>创建 {timestamp(gate.created_at)} · 截止 {timestamp(gate.expires_at)} · 执行 {timestamp(gate.consumed_at??undefined)}</p>
      <p>人工等待：{duration(gate.created_at,gate.decided_at??(gate.status==='expired'?gate.expires_at:undefined))}</p>
      <p>Git 检查点：{gate.head_commit||'无文件检查点'}</p><p className="identifier">审核摘要：{gate.subject_hash}</p>
      {gate.reason&&<p>审核理由：{gate.reason}</p>}
      <details open={gate.status==='waiting'}><summary>待执行的固定内容与上游产物</summary><pre>{gate.subject?JSON.stringify(gate.subject,null,2):'正文已过保留期'}</pre></details>
      {gate.project_id===access.project&&!stopped&&gate.subject&&['waiting','approved'].includes(gate.status)&&(isManager(access)||(canEdit(access)&&gate.owner_id===access.session.id))&&<Decision key={`${gate.id}:${gate.status}`} gate={gate} access={access} reload={query.reload}/>}
    </article>)}
  </section>;
}

function Decision({gate,access,reload}:{gate:Schema['ApprovalGate'];access:Access;reload:()=>void}) {
  const [decision,setDecision]=useState(''),[reason,setReason]=useState(''),action=useAction();
  const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500),conflict=action.error instanceof APIError&&[409,410].includes(action.error.status);
  async function submit(){
    if(!['approved','rejected','revoked'].includes(decision))throw Error('请选择审核决定。');
    await apiRequest(`/approvals/${gate.id}`,{method:'PUT',csrf:access.session.csrf,body:{subject_hash:gate.subject_hash,decision,reason}});
    if(action.active())reload();
  }
  return <form onSubmit={e=>{e.preventDefault();void action.run(submit);}}>
    <p>批准仅适用于以上内容。开始执行前可撤回；开始后请使用取消运行。人工审批使用上方独立的等待期限。</p>
    <fieldset disabled={action.busy||uncertain||conflict}><legend>人工决定</legend><label className="field">审核决定<select required value={decision} onChange={e=>setDecision(e.target.value)}><option value="">请核对内容后选择</option>{gate.status==='waiting'?<><option value="approved">批准执行</option><option value="rejected">驳回并结束</option></>:<option value="revoked">撤回批准</option>}</select></label>
      <label className="field">审核理由<textarea required maxLength={2048} value={reason} onChange={e=>setReason(e.target.value)}/></label></fieldset>
    <button disabled={action.busy||conflict}>{uncertain?'重试同一审核决定':'提交审核决定'}</button>
    {uncertain&&<p>响应未确认，重试保持原始审核摘要和决定。</p>}{conflict&&<button type="button" onClick={reload}>重新读取审核状态</button>}<ActionNotice {...action}/>
  </form>;
}
