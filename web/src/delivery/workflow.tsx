import {useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {Download} from '../api/download';
import {VersionPicker} from '../catalog/selection';
import {ActionNotice,Page,Pager,QueryState,canEdit,isManager,timestamp,type Access,type Schema} from '../console/components';
import {RequestReceipt} from '../runs/submission';

export function CreateDelivery({run,report,access}:{run:string;report:Schema['ReportDetail'];access:Access}) {
 const [tasks,setTasks]=useState<string[]>([]),[implementation,setImplementation]=useState<Schema['CatalogVersion']>(),[review,setReview]=useState<Schema['CatalogVersion']>(),action=useAction(),key=useRef(crypto.randomUUID());
 const uncertain=action.error instanceof APIError&&(action.error.status===0||action.error.status>=500||action.error.status===409),disabled=action.busy||uncertain;
 const valid=tasks.length>0&&implementation?.spec.agent?.executor==='codex'&&!implementation.spec.agent.read_only&&review?.spec.agent?.executor==='codex'&&review.spec.agent.read_only;
 async function create(){
  const item=await apiRequest<Schema['Delivery']>(`/runs/${run}/reports/${report.id}/deliveries`,{method:'POST',csrf:access.session.csrf,key:key.current,body:{subject_hash:report.subject_hash,task_ids:tasks,implementation_version:implementation!.id,review_version:review!.id}});
  action.navigate(`/deliveries/${item.id}`);
 }
 return <details><summary>选择设计任务，生成交付草案</summary><form onSubmit={e=>{e.preventDefault();void action.run(create);}}>
  <p>选择任务及其全部前置依赖。项目需要登记真实验证命令；草案固定后，实施前还需人工审批。</p>
  <fieldset disabled={disabled}><legend>任务范围</legend>{report.data?.tasks?.map(task=><label className="field" key={task.id}><span><input type="checkbox" checked={tasks.includes(task.id)} onChange={e=>setTasks(e.target.checked?[...tasks,task.id]:tasks.filter(id=>id!==task.id))}/>{task.id}：{task.title}</span><span>范围：{task.scope} · 依赖：{task.depends_on?.join('、')||'无'}</span><span>交付：{task.deliverable} · 验收：{task.acceptance}</span></label>)}</fieldset>
  <h4>实施 Agent（可写 Codex）</h4><VersionPicker kind="agent" value={implementation?.id??''} change={setImplementation} disabled={disabled}/>
  <h4>审查 Agent（只读 Codex）</h4><VersionPicker kind="agent" value={review?.id??''} change={setReview} disabled={disabled}/>
  {!valid&&<p>请选择任务、可写 Codex 实施版本和只读 Codex 审查版本。</p>}
  <button disabled={!valid||action.busy}>{uncertain?'重试同一草案请求':'保存固定草案'}</button><ActionNotice {...action}/>
 </form></details>;
}

export function Deliveries({access}:{access:Access}) {
 const [offset,setOffset]=useState(0),query=useQuery<Schema['DeliveryPage']>(`/deliveries?project_id=${access.project}&offset=${offset}`);
 return <Page title="方案到交付" description="从设计报告选择任务，固定草案，人工审批后实施、验证与只读审查，最后由人采纳。">
  <p>在成功的产品经理设计运行中打开「报告证据核验」，选择任务生成草案。</p><QueryState {...query} retry={query.reload}/>
  {query.data?.items.map(item=><p key={item.id}><a href={`#/deliveries/${item.id}`}>{item.id}</a> · {item.request_id?'已提交执行':item.definition?'待执行草案':'正文已过期'} · {timestamp(item.created_at)}</p>)}
  <Pager offset={offset} next={query.data?.next_offset} change={setOffset}/>
 </Page>;
}

export function DeliveryDetail({id,access}:{id:string;access:Access}) {
 const query=useQuery<Schema['Delivery']>(`/deliveries/${id}`,5000),item=query.data,action=useAction(),[confirm,setConfirm]=useState(false);
 const allowed=item?.project_id===access.project&&(isManager(access)||(canEdit(access)&&item.actor_id===access.session.id));
 async function execute(){await apiRequest(`/deliveries/${id}/run`,{method:'POST',csrf:access.session.csrf,body:{subject_hash:item!.subject_hash}});if(action.active())query.reload();}
 return <Page title="固定交付草案" actions={<a href="#/deliveries">返回交付列表</a>}><QueryState {...query} retry={query.reload}/>{item&&<>
  <p>来源设计：<a href={`#/runs/${item.source_run_id}`}>{item.source_run_id}</a> · 报告 {item.report_id}</p><p>保留至 {timestamp(item.expires_at)}</p>
  <p className="identifier">草案摘要：{item.subject_hash}</p>
  {item.definition?<><h2>固定任务、版本与审批条件</h2><pre>{item.definition.input}</pre><details><summary>执行定义</summary><pre>{JSON.stringify(item.definition,null,2)}</pre></details></>:<p>草案或来源正文已过期，不能开始新运行。</p>}
  {!item.request_id&&item.definition&&allowed&&<><label><input type="checkbox" checked={confirm} disabled={action.busy} onChange={e=>setConfirm(e.target.checked)}/>确认以上范围，创建等待人工审批的运行</label><button disabled={!confirm||action.busy} onClick={()=>void action.run(execute)}>创建或找回同一运行</button></>}
  <ActionNotice {...action}/>{item.request_id&&<><RequestReceipt id={item.request_id}/><p>在运行中批准实施，检查验证与审查结果后登记人工采纳。检查中的 diff/ZIP 仍可在运行页面查看。</p><Download path={`/deliveries/${id}/artifacts.zip`} filename={`delivery-${id}.zip`}>导出已采纳交付</Download><p>导出会检查成功验证、只读审查及当前产物的采纳记录；不推送源仓库。</p></>}
 </>}</Page>;
}
