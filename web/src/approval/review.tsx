import {useEffect,useRef,useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,type Access,type Schema} from '../console/components';
import {changeAnswer,PlanOptions,validAnswer} from './choice';

type Answer=Schema['ReviewAnswer'];
type Decision=Schema['ApprovalDecision'];
export function ReviewForm({gate,access,reload}:{gate:Schema['ApprovalGate'];access:Access;reload:()=>void}){
  const [answers,setAnswers]=useState<Record<string,Answer>>({}),[reason,setReason]=useState(''),[dirty,setDirty]=useState(false);
  const action=useAction(),submitted=useRef<Decision|undefined>(undefined);
  const items=gate.review!.items,values=items.flatMap(i=>answers[i.id]?[answers[i.id]]:[]);
  const evidence=gate.purpose==='evidence';
  const unresolved=gate.review!.resolutions.some(r=>r.status==='unresolved');
  const corrections=values.filter(v=>v.decision!=='accept').length;
  const hasMaterial=!!reason.trim()||values.some(v=>!!v.correction.trim());
  const invalid=values.some(v=>!validAnswer(items.find(item=>item.id===v.item_id)!,v));
  const canApprove=!evidence&&values.length===items.length&&!invalid&&!corrections&&!reason.trim()&&!unresolved;
  const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500);
  const conflict=action.error instanceof APIError&&[409,410].includes(action.error.status);
  useUnsavedReview(dirty);
  function change(id:string,patch:Partial<Answer>){setDirty(true);setAnswers(old=>{const next={...old},answer=changeAnswer(items.find(item=>item.id===id)!,old[id],patch);if(answer)next[id]=answer;else delete next[id];return next;});}
  async function submit(decision:Decision['decision'],retry=false){
    const data=retry?submitted.current!:{subject_hash:gate.subject_hash,decision,reason,items:values};
    submitted.current=data;
    await apiRequest(`/approvals/${gate.id}`,{method:'PUT',csrf:access.session.csrf,body:data});
    if(action.active()){setDirty(false);reload();}
  }
  return <div className="review-form"><p className="notice">{evidence?'工作流在此暂停，不会自动过期。补充材料说明后仅复验此只读节点；仍有缺口会再次等待，不会直接放行。':'工作流在此暂停，不会自动过期。纠偏后将修订并再次请你审核，全部同意后才开始实施。'}</p>
    <fieldset className="review-fields" disabled={action.busy||uncertain||conflict}><legend className="sr-only">逐项审核</legend>
      {items.map((item,index)=><ReviewChoice key={item.id} item={item} index={index} answer={answers[item.id]} change={patch=>change(item.id,patch)}/>)}
      <label className="field">{evidence?'新增证据位置、处置记录或判断纠正依据':'整体纠偏意见／结束原因'}<textarea rows={3} maxLength={4000} value={reason} onChange={e=>{setDirty(true);setReason(e.target.value);}} placeholder={evidence?'说明本次新增了什么材料；没有新材料请先准备修正方案。':'涉及多个项目的调整写在这里；填写纠偏后需修订复审。'}/></label>
    </fieldset>
    <div className="review-actions"><p role="status">共 {items.length} 项 · 未决定 {items.length-values.length} 项 · 需纠偏 {corrections} 项</p>
      {unresolved&&<p className="error-text">上一轮仍有未解决意见，暂不能批准。请补充纠偏，或驳回并结束。</p>}
      <div className="toolbar">{!evidence&&<button className="primary" disabled={!canApprove||action.busy||uncertain||conflict} onClick={()=>void action.run(()=>submit('approved'),'已批准，工作流将继续执行')}>全部同意并批准执行</button>}
        <button disabled={invalid||(!corrections&&!reason.trim())||(evidence&&!hasMaterial)||action.busy||uncertain||conflict} onClick={()=>void action.run(()=>submit('changes_requested'),evidence?'补充说明已提交，将重新核验':'纠偏已提交，修订完成后请重新审核')}>{evidence?'提交说明并重新核验':'提交纠偏，修订后复审'}</button>
        <button className="danger" disabled={invalid||!reason.trim()||action.busy||uncertain||conflict} onClick={()=>void action.run(()=>submit('rejected'),'已驳回，工作流将结束')}>驳回并结束</button></div>
      {invalid&&<p>请选择有效方案，或为需调整、不接受的项目填写具体纠偏意见。</p>}
      {uncertain&&<button disabled={action.busy} onClick={()=>void action.run(()=>submit(submitted.current!.decision,true))}>重试同一审核决定</button>}
      {conflict&&<button onClick={reload}>重新读取审核状态</button>}<ActionNotice {...action}/>
    </div>
  </div>;
}

function ReviewChoice({item,index,answer,change}:{item:Schema['ReviewItem'];index:number;answer?:Answer;change:(patch:Partial<Answer>)=>void}){
  return <fieldset className="review-item"><legend>{item.title}</legend><div className="review-meta"><span>{String(index+1).padStart(2,'0')}</span><span>{item.category}</span></div>
    <p className="review-proposal">{item.proposal}</p><p className="muted">依据：{item.rationale}</p>
    {!!item.task_ids.length&&<p className="muted">关联任务：{item.task_ids.join('、')}</p>}
    {!!item.evidence_refs.length&&<button type="button" className="review-source" onClick={()=>{const el=document.getElementById('review-sources');el?.setAttribute('open','');el?.scrollIntoView({behavior:'smooth'});}}>查看依据 · {item.evidence_refs.join('、')}</button>}
    {item.options?.length?<PlanOptions item={item} answer={answer} change={change}/>:<div className="review-options">{([['accept','同意'],['revise','需调整'],['reject','不接受']] as const).map(([value,label])=><label key={value} className={answer?.decision===value?'selected':''}><input type="radio" name={`review-${item.id}`} value={value} checked={answer?.decision===value} onChange={()=>change({decision:value,...(value==='accept'?{correction:''}:{})})}/>{label}</label>)}</div>}
    <label className="field">纠偏意见<textarea rows={2} maxLength={4000} value={answer?.correction??''} onChange={e=>change({correction:e.target.value})} placeholder="说明希望如何调整；输入后将进入修订复审。"/></label>
  </fieldset>;
}

function useUnsavedReview(dirty:boolean){
  useEffect(()=>{
    if(!dirty)return;
    const leave=(event:Event)=>{if(!window.confirm('审核意见尚未提交，确定离开？'))event.preventDefault();};
    const unload=(event:BeforeUnloadEvent)=>{event.preventDefault();event.returnValue='';};
    window.addEventListener('pangu:before-navigate',leave);window.addEventListener('beforeunload',unload);
    return()=>{window.removeEventListener('pangu:before-navigate',leave);window.removeEventListener('beforeunload',unload);};
  },[dirty]);
}
