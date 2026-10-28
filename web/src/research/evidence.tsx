import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {APIError,apiRequest} from '../api/request';
import {ActionNotice,QueryState,canEdit,isManager,timestamp,type Access,type Schema} from '../console/components';
import {CreateDelivery} from '../delivery/workflow';

const verdicts:Record<string,string>={supported:'依据支持结论',partial:'部分支持，仍有限制',unsupported:'依据不支持结论',outdated:'资料已不适用'};
const kinds:Record<string,string>={fact:'事实主张（尚需核验）',inference:'推断',unknown:'未知'};
export function Reports({run,owner,access}:{run:string;owner?:string;access:Access}) {
  const query=useQuery<Schema['ReportDetail'][]>(`/runs/${run}/reports`),editable=isManager(access)||(canEdit(access)&&owner===access.session.id);
  return <section><div className="section-heading"><h2>报告证据核验</h2><button onClick={query.reload}>刷新报告与核验</button></div>
    <p>原文匹配只能证明读取过这段资料。结论是否成立、资料日期和适用范围是否正确，仍需人工判断。</p>
    <QueryState {...query} retry={query.reload}/>{query.data?.length===0&&<p>当前没有产品经理报告。</p>}
    {query.data?.map(report=><article key={report.id}><h3>{report.title}</h3><p className="identifier">报告摘要：{report.subject_hash}</p>
      {!report.data&&<p>该报告没有结构化证据，可在文件与报告中读取原文。</p>}
      {canEdit(access)&&report.data?.mode==='design'&&<CreateDelivery key={report.subject_hash} run={run} report={report} access={access}/>}
      {report.data?.claims?.map(claim=>{
        const review=report.reviews.find(item=>item.claim_id===claim.id);
        return <section className="node-result" key={claim.id}><h4>{claim.id}：{claim.statement}</h4><p>{kinds[claim.kind]} · 适用范围：{claim.scope}</p>
          <p>局限与待核验：{claim.uncertainty}</p>{claim.conflicts?.map((item,index)=><p className="notice" key={index}>冲突：{item}</p>)}
          {claim.evidence?.map((item,index)=>{const source=report.data?.sources?.find(value=>value.id===item.source_id);return <blockquote key={index}><p>{item.quote}</p><p>来源 {item.source_id}：{source?.title??'来源信息缺失'}</p><p>资料日期（待核对）：{item.source_date==='unknown'?'未知':item.source_date} · 获取时间：{timestamp(source?.retrieved_at)}</p>{source?.url&&<p>{source.url}</p>}<small>来源摘要：{source?.sha256??'未提供'}</small></blockquote>;})}
          <p>{review?.source==='feishu_session'?`${verdicts[review.verdict]} · ${review.reason}`:review?'历史核验来源未知，需通过飞书会话重新核验':'尚未人工核验'}</p>
          {editable&&<ClaimForm key={`${report.subject_hash}:${review?.revision}`} run={run} report={report} claim={claim.id} review={review} access={access} reload={query.reload}/>}
        </section>;
      })}
    </article>)}
  </section>;
}

function ClaimForm({run,report,claim,review,access,reload}:{run:string;report:Schema['ReportDetail'];claim:string;review?:Schema['ClaimReview'];access:Access;reload:()=>void}) {
  const [verdict,setVerdict]=useState(review?.verdict??''),[reason,setReason]=useState(review?.reason??''),action=useAction();
  const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500),conflict=action.error instanceof APIError&&[404,409].includes(action.error.status);
  async function save(){
    if(!Object.hasOwn(verdicts,verdict))throw Error('请选择人工判定。');
    await apiRequest(`/runs/${run}/reports/${report.id}/claims/${encodeURIComponent(claim)}`,{method:'PUT',csrf:access.session.csrf,body:{revision:review?.revision??0,subject_hash:report.subject_hash,verdict,reason}});
    if(action.active())reload();
  }
  return <form onSubmit={e=>{e.preventDefault();void action.run(save);}}><fieldset disabled={action.busy||uncertain||conflict}><legend>人工核验</legend>
    <label className="field">证据判定<select required value={verdict} onChange={e=>setVerdict(e.target.value)}><option value="">核对后选择</option>{Object.entries(verdicts).map(([value,text])=><option key={value} value={value}>{text}</option>)}</select></label>
    <label className="field">判定理由<textarea required maxLength={2048} value={reason} onChange={e=>setReason(e.target.value)}/></label></fieldset>
    <button disabled={action.busy||conflict}>{uncertain?'重试同一核验':'保存证据核验'}</button>{conflict&&<button type="button" onClick={reload}>重新读取并放弃本地编辑</button>}<ActionNotice {...action}/>
  </form>;
}
