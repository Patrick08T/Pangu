import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {APIError,apiRequest} from '../api/request';
import {ActionNotice,Page,Pager,QueryState,canEdit,canManageCatalog,timestamp,type Access,type Schema} from '../console/components';
import {Composer} from './composer';
import {receipt} from '../runs/submission';
import {confirmed} from './receipt';
import {Proposal} from './preview';
import './style.css';

export function Factory({access,id}:{access:Access;id?:string}) {
 const [offset,setOffset]=useState(0),action=useAction();
 const history=useQuery<Schema['FactorySessionPage']>(`/factory/sessions?project_id=${access.project}&offset=${offset}`);
 async function create(signal:AbortSignal){const s=await apiRequest<Schema['FactorySession']>('/factory/sessions',{method:'POST',csrf:access.session.csrf,body:{project_id:access.project},signal});action.navigate(`/factory/${s.id}`);}
 return <Page title="工厂助手" description="说出目标，一起打磨助手与流程。先预览，确认后创建并发布。" actions={<a href="#/onboarding">运行已有助手 →</a>}>
  <div className="factory-layout"><section className="factory-history" aria-label="工厂对话记录"><button className="primary" disabled={action.busy||!canEdit(access)} onClick={()=>void action.run(create)}>＋ 新对话</button><ActionNotice {...action}/><QueryState {...history} retry={history.reload}/>
   <nav aria-label="历史对话">{history.data?.items.map(s=><a key={s.id} href={`#/factory/${s.id}`} aria-current={s.id===id?'page':undefined}><strong>{s.title}</strong><small>{timestamp(s.updated_at)}</small></a>)}</nav><Pager offset={offset} next={history.data?.next_offset} change={setOffset}/>
  </section>{id?<Conversation key={id} access={access} id={id}/>:<section className="factory-welcome"><span className="factory-mark">P</span><p className="eyebrow">从想法到可执行的团队</p><h2>你希望助手帮你完成什么？</h2><p>用自然语言描述目标、资料来源和交付物，工厂助手会补齐职责、交接契约及流程。</p><ul><li>创建一个能做竞品分析的产品经理助手</li><li>先调研，再产出产品规划，最后由我审批</li><li>分析失败任务，给现有助手提出修改方案</li></ul><button className="primary" disabled={action.busy||!canEdit(access)} onClick={()=>void action.run(create)}>开始对话</button><small>内置 build-optimize-workflow · 创建前可继续对话调整</small></section>}</div>
 </Page>;
}

function Conversation({access,id}:{access:Access;id:string}) {
 const query=useQuery<Schema['FactorySession']>(`/factory/sessions/${id}`,3000),action=useAction();
 const [message,setMessage]=useState(''),[mode,setMode]=useState<Schema['FactoryInput']['mode']>('build');
 const [connection,setConnection]=useState(''),[target,setTarget]=useState<Schema['CatalogObject']>(),[source,setSource]=useState('');
 const retry=useRef<Schema['FactoryInput']|undefined>(undefined);
 const s=query.data,latest=s?.turns.at(-1),pending=latest?.status==='pending';
 const uncertain=!!retry.current&&action.error instanceof APIError&&(action.error.status===0||action.error.status>=500);
 const owned=canEdit(access)&&(access.session.admin||access.role==='manager'||s?.created_by===access.session.id);
 const disabled=action.busy||pending||uncertain||!owned||!!query.error;
 async function send(signal:AbortSignal){
  if(!s||s.project_id!==access.project)throw Error('请重新读取当前项目的对话');
  const input=uncertain?retry.current!:{id:crypto.randomUUID(),revision:s.revision,message,mode,connection_version:connection,...(target?{target_id:target.id,target_revision:target.revision}:{}),...(source?{source_run_id:source}:{})};
  retry.current=input;
  try{receipt(await apiRequest<Schema['Submission']>(`/factory/sessions/${id}/messages`,{method:'POST',csrf:access.session.csrf,body:input,signal}));if(action.active()){retry.current=undefined;setMessage('');query.reload();}}
  catch(error){if(action.active())query.reload();throw error;}
 }
 async function apply(signal:AbortSignal){
  if(!s||!latest) return;
  confirmed(await apiRequest<Schema['FactoryResult']>(`/factory/sessions/${id}/apply`,{method:'POST',csrf:access.session.csrf,body:{turn_id:latest.id,revision:s.revision},signal}));if(action.active())query.reload();
 }
 return <section className="factory-conversation"><QueryState {...query} retry={query.reload}/>{s&&s.project_id!==access.project?<p role="alert">此对话不属于当前项目，请从左侧选择对话。</p>:s&&<>
  <div className="factory-thread" aria-label="对话内容">{!s.turns.length&&<article className="factory-message assistant"><strong>工厂助手</strong><p>告诉我你想完成的工作。我会先设计候选方案；信息不足时会继续询问。</p><p>例如：创建一个产品规划助手，把业务需求整理成目标、功能和可执行任务。</p></article>}{s.turns.map(turn=><article key={turn.id} className="factory-turn"><div className="factory-message user"><strong>需求</strong><p>{turn.status==='expired'?'对话正文已超过保留期':turn.input.message}</p></div><div className="factory-message assistant"><strong>工厂助手</strong>{turn.error&&<p className="notice error" role="alert">{turn.error}</p>}{turn.status==='pending'?<Generation access={access} turn={turn}/>:turn.reply&&<p className="factory-reply">{turn.reply.message}</p>}{(turn===latest||turn.result)&&<Proposal turn={turn} disabled={disabled||turn!==latest||!canManageCatalog(access)} apply={()=>void action.run(apply,'已创建并发布，尚未执行任务')}/>}</div></article>)}</div>
  <ActionNotice {...action}/>{owned&&!canManageCatalog(access)&&<p className="notice">发布共享角色与工作流需要配置管理权限。</p>}{!owned&&<p className="notice">仅对话发起人或项目管理者可继续修改与确认此方案。</p>}{uncertain&&<p className="notice">消息可能已经送达。输入已锁定，重试会恢复同一次生成。</p>}
  <Composer access={access} mode={mode} setMode={setMode} message={message} setMessage={setMessage} connection={connection} setConnection={setConnection} target={target} setTarget={setTarget} source={source} setSource={setSource} disabled={!!disabled} busy={action.busy||!!pending} uncertain={uncertain} send={()=>void action.run(send,'需求已提交，可继续留在此处查看方案')}/>
 </>}</section>;
}

function Generation({access,turn}:{access:Access;turn:Schema['FactoryTurn']}) {
 const query=useQuery<Schema['Submission']>(turn.request_id?`/requests/${turn.request_id}`:null,3000),action=useAction();
 const run=query.data?.run_id;
 return <div className="factory-generating"><p role="status">正在生成方案… 刷新页面后仍可继续查看。</p><QueryState {...query} retry={query.reload}/>{turn.request_id&&<a href={`#/requests/${turn.request_id}`}>查看生成任务</a>}{run&&canEdit(access)&&<button disabled={action.busy} onClick={()=>void action.run(async signal=>{await apiRequest(`/runs/${run}/cancel`,{method:'POST',csrf:access.session.csrf,signal});},'已请求停止，正在等待执行器确认')}>停止生成</button>}<ActionNotice {...action}/></div>;
}
