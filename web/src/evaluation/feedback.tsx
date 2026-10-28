import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,QueryState,canEdit,isManager,type Access,type Schema} from '../console/components';

export const decisions:Record<string,string>={accepted:'采纳',needs_changes:'需要修改',rejected:'拒绝'};
export function Feedback({run,access,owner}:{run:string;access:Access;owner?:string}) {
  const query=useQuery<Schema['FeedbackState']>(`/runs/${run}/feedback`,10000),state=query.data;
  const editable=isManager(access)||(canEdit(access)&&owner===access.session.id);
  return <section><h2>人工采纳评价</h2><p>执行成功不代表采纳。评价绑定本次运行的版本、输入、输出与文件摘要；首次评价另行保留。</p>
    <QueryState {...query} retry={query.reload}/>{state&&<>
      {state.feedback&&<p>当前：{decisions[state.feedback.decision]} · 首次：{decisions[state.feedback.first_decision]} · 修订 {state.feedback.revision}<br/>{state.feedback.payload_expired?'评价正文已清除':state.feedback.reason}</p>}
      {state.feedback?.source==='unknown'&&<p className="notice">历史评价来源未确认，不计入人工采纳或交付放行；请通过飞书会话重新评定。</p>}
      {editable&&state.subject_hash&&!state.feedback?.payload_expired?<FeedbackForm key={`${state.subject_hash}:${state.feedback?.revision}`} run={run} access={access} state={state} reload={query.reload}/>:<p>运行停止且正文可用后，由运行所有者或管理者评定。</p>}
    </>}</section>;
}

function FeedbackForm({run,access,state,reload}:{run:string;access:Access;state:Schema['FeedbackState'];reload:()=>void}) {
  const [decision,setDecision]=useState(state.feedback?.decision??'needs_changes'),[reason,setReason]=useState(state.feedback?.reason??'');
  const action=useAction(),uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500);
  const conflicted=action.error instanceof APIError&&[409,410].includes(action.error.status);
  async function save(){
    await apiRequest(`/runs/${run}/feedback`,{method:'PUT',csrf:access.session.csrf,body:{revision:state.feedback?.revision??0,subject_hash:state.subject_hash,decision,reason}});
    if(action.active())reload();
  }
  return <form onSubmit={e=>{e.preventDefault();void action.run(save);}}>
    <fieldset disabled={action.busy||uncertain||conflicted}><legend>评定结果</legend>
      <label className="field">评价<select value={decision} onChange={e=>setDecision(e.target.value as typeof decision)}>{Object.entries(decisions).map(([key,label])=><option key={key} value={key} disabled={key==='accepted'&&!state.can_accept}>{label}</option>)}</select></label>
      <label className="field">理由与验收依据<textarea value={reason} onChange={e=>setReason(e.target.value)} required maxLength={2048}/></label></fieldset>
    <button disabled={action.busy||conflicted}>{uncertain?'重试同一评价':'保存评价'}</button>
    {uncertain&&<p>结果尚未确认，重试保持相同摘要、修订和评价。</p>}
    {conflicted&&<button type="button" onClick={reload}>重新读取并放弃本地编辑</button>}<ActionNotice {...action}/>
  </form>;
}
