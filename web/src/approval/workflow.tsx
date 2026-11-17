import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,Pager,QueryState,Status,canEdit,isManager,timestamp,type Access,type Schema} from '../console/components';
import {ReviewForm} from './review';
import {ChosenOptions} from './choice';
import {ExecutionPlan} from './plan';
import {EvidenceActions} from './repair';

const labels:Record<string,string>={waiting:'等待审核',changes_requested:'已提交纠偏',approved:'已批准，等待执行',rejected:'已驳回',revoked:'批准已撤回',consumed:'已用于执行',expired:'已过期'};

export function Approvals({run,access,stopped}:{run:string;access:Access;stopped:boolean}){
  const query=useQuery<Schema['ApprovalPage']>(`/approvals?project_id=${access.project}&run_id=${run}&limit=100`,5000);
  if(query.data?.items.length===0&&!query.error)return null;
  return <section><h2>节点人工审核</h2><QueryState {...query} retry={query.reload}/>{stopped&&<p>运行已结束，审批记录仅供查阅。</p>}
    {query.data&&<ReviewRows items={query.data.items}/>}<a href="#/approvals">前往人工审核中心</a>
  </section>;
}

export function ApprovalCenter({access}:{access:Access}){
  const [status,setStatus]=useState('waiting'),[offset,setOffset]=useState(0);
  const query=useQuery<Schema['ApprovalPage']>(`/approvals?project_id=${access.project}&offset=${offset}&limit=20${status?`&status=${status}`:''}`,5000);
  return <Page title="人工审核" description="逐项确认计划，提出纠偏，并查看每一轮处理记录。">
    <div className="toolbar"><label className="field">审核状态<select value={status} onChange={e=>{setStatus(e.target.value);setOffset(0);}}><option value="">全部</option>{Object.entries(labels).map(([value,label])=><option key={value} value={value}>{label}</option>)}</select></label></div>
    <QueryState {...query} retry={query.reload} empty={query.data?.items.length===0?'当前没有符合条件的审核记录。':undefined}/>
    {query.data&&<><ReviewRows items={query.data.items}/><Pager offset={offset} next={query.data.next_offset} change={setOffset}/></>}
  </Page>;
}

function ReviewRows({items}:{items:Schema['ApprovalSummary'][]}){
  return <div className="review-list">{items.map(item=><article key={item.id} className="review-row"><div><a className="review-title" href={`#/approvals/${item.id}`}>{item.title}</a><p className="muted">{item.node_id} · 第 {item.round} 轮 · {timestamp(item.created_at)}</p></div><span className="status">{labels[item.status]??item.status}</span><a className="button" href={`#/approvals/${item.id}`}>打开审核</a></article>)}</div>;
}

export function ApprovalDetail({id,access}:{id:string;access:Access}){
  const query=useQuery<Schema['ApprovalDetails']>(`/approvals/${id}`,5000),detail=query.data,gate=detail?.approval;
  const allowed=!!gate&&gate.project_id===access.project&&(isManager(access)||(canEdit(access)&&gate.owner_id===access.session.id));
  return <Page title={gate?.repair?'修正版本与续跑审核':gate?.review?.title??'审核详情'} description="审核只适用于当前轮次的冻结计划。" actions={<a className="button" href="#/approvals">返回人工审核</a>}>
    <QueryState {...query} retry={query.reload}/>{gate&&detail&&<>
      <div className="review-overview"><span className="status">{labels[gate.status]}</span><span>第 {gate.round} 轮</span><Status value={detail.run_status}/><a href={`#/runs/${gate.run_id}`}>查看工作流运行</a></div>
      {gate.inherited_from&&<p className="notice">此运行继承了<a href={`#/approvals/${gate.inherited_from}`}>原计划审批</a>的授权；平台已核对计划、版本与复用成果。</p>}
      <ExecutionPlan plan={gate.execution}/>
      {gate.review?<><p className="intro">{gate.repair?'本次处置可发布修正版本并复用成果续跑。原补证说明和历史结论保留在来源材料中，审批不会改写旧结论。':gate.review.summary}</p><ReviewChanges gate={gate}/>
        {!detail.stopped&&allowed&&gate.status==='waiting'?<PendingReview key={`${gate.id}:${gate.subject_hash}`} gate={gate} access={access} reload={query.reload}/>:<ReadOnlyReview gate={gate}/>}
      </>:<p className="notice">这是旧格式的历史审批，仅供查阅；新工作流需提供结构化审核项。</p>}
      {detail.stopped&&<p className="notice">运行已结束或正在取消，不能再提交审批。</p>}
      {!detail.stopped&&!allowed&&<p className="notice">只有本项目管理者或有操作权限的运行创建者可以审核；请核对当前项目与账号。</p>}
      {!detail.stopped&&allowed&&gate.status==='approved'&&<Revoke gate={gate} access={access} reload={query.reload}/>}
      <ReviewHistory gate={gate}/><details id="review-sources"><summary>来源材料与冻结内容</summary>{gate.repair&&<p>原补证说明：{gate.review?.summary}</p>}<p className="identifier">检查点：{gate.head_commit||'无'} · 内容摘要：{gate.subject_hash}</p><ApprovalReports subject={gate.subject}/><details><summary>完整冻结数据</summary><pre>{JSON.stringify(gate.subject,null,2)}</pre></details></details>
    </>}
  </Page>;
}

function PendingReview(props:{gate:Schema['ApprovalGate'];access:Access;reload:()=>void}){
  return props.gate.purpose==='evidence'?<EvidenceActions {...props}/>:<ReviewForm {...props}/>;
}

function ReadOnlyReview({gate}:{gate:Schema['ApprovalGate']}){
  if(gate.repair&&gate.repair.status!=='prepared')return <section><h2>已选择修正版本续跑</h2><p>{gate.repair.summary}</p><p>复用节点：{gate.repair.reused_nodes.join('、')}</p><p>补跑节点：{gate.repair.execute_nodes.join('、')}</p><p>原补证项未被追溯改判通过。</p><ExecutionPlan plan={gate.repair.execution}/></section>;
  return <section><h2>本轮审核项与决定</h2>{gate.review?.items.map(item=>{const answer=gate.items.find(a=>a.item_id===item.id);return <article key={item.id} className="review-item"><h3>{item.title}</h3><p>{item.proposal}</p><p>{answer?{accept:'同意',revise:'需调整',reject:'不接受'}[answer.decision]:'尚未决定'}</p><ChosenOptions item={item} answer={answer}/>{answer?.correction&&<p>纠偏意见：{answer.correction}</p>}</article>;})}{gate.reason&&<p>整体意见：{gate.reason}</p>}</section>;
}

function ReviewChanges({gate}:{gate:Schema['ApprovalGate']}){
  const query=useQuery<Schema['ApprovalDetails']>(gate.previous_id?`/approvals/${gate.previous_id}`:null);
  if(!gate.previous_id)return null;
  const previous=query.data?.approval;
  return <section className="review-changes"><h2>上一轮意见与修订回应</h2><QueryState {...query} retry={query.reload}/>{previous?.reason&&<p>整体意见：{previous.reason}</p>}
    {gate.review?.resolutions.map(r=>{const old=previous?.review?.items.find(i=>i.id===r.item_id),next=gate.review?.items.find(i=>i.id===r.item_id),answer=previous?.items.find(i=>i.item_id===r.item_id);return <article key={r.item_id}><h3>{next?.title??old?.title??'整体意见'} · {r.status==='addressed'?'已回应，待你确认':'尚未解决'}</h3>{old&&<><p>原建议：{old.proposal}</p><ChosenOptions item={old} answer={answer}/></>}{answer?.correction&&<p>你的意见：{answer.correction}</p>}<p>当前建议：{next?.proposal??'此项已移除，见处理说明'}</p><p>处理说明：{r.explanation}</p></article>;})}
  </section>;
}

function ReviewHistory({gate}:{gate:Schema['ApprovalGate']}){
  const [offset,setOffset]=useState(0),query=useQuery<Schema['ApprovalPage']>(`/approvals?project_id=${gate.project_id}&run_id=${gate.run_id}&node_id=${gate.node_id}&offset=${offset}&limit=20`,5000);
  return <details open={gate.status==='changes_requested'}><summary>审核轮次与历史</summary><QueryState {...query} retry={query.reload}/>{query.data&&<><ReviewRows items={query.data.items}/><Pager offset={offset} next={query.data.next_offset} change={setOffset}/></>}</details>;
}

function Revoke({gate,access,reload}:{gate:Schema['ApprovalGate'];access:Access;reload:()=>void}){
  const [reason,setReason]=useState(''),action=useAction();
  return <section><p>开始执行前可以撤回批准；开始后请取消运行。</p><label className="field">撤回原因<textarea value={reason} maxLength={4000} onChange={e=>setReason(e.target.value)}/></label><button disabled={!reason.trim()||action.busy} onClick={()=>void action.run(async()=>{await apiRequest(`/approvals/${gate.id}`,{method:'PUT',csrf:access.session.csrf,body:{subject_hash:gate.subject_hash,decision:'revoked',reason,items:[]}});reload();})}>撤回批准</button><ActionNotice {...action}/></section>;
}

function ApprovalReports({subject}:{subject:Schema['ApprovalGate']['subject']}){
  const input=subject?.node_input;
  if(!input||typeof input!=='object'||!('parents' in input)||!input.parents||typeof input.parents!=='object')return null;
  return Object.entries(input.parents).map(([name,value])=>{
    if(!value||typeof value!=='object'||!('text' in value)||typeof value.text!=='string')return null;
    let text=value.text;try{text=JSON.stringify(JSON.parse(text),null,2);}catch{/* 研究正文保留原文。 */}
    return <section key={name}><h3>{name} · 完整报告</h3><pre>{text}</pre></section>;
  });
}
