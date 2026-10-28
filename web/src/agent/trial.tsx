import {useEffect,useRef,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,Pager,QueryState,Status,timestamp,type Access,type Schema} from '../console/components';
import {RunInput} from '../onboarding/input';
import {PreflightStatus} from '../onboarding/catalog';
import {receipt} from '../runs/submission';
import {Attempts} from '../runs/execution';
import {Usage} from '../usage/workflow';
import {Approvals} from '../approval/workflow';
import {handoffProblems} from './handoff';

type Selection={request:string;revision?:number;version?:number};
type Frozen={body:Schema['AgentTrialInput'];path:string;key:string;version?:number};
export function AgentTrial({item,access,disabled,version,onUnlock,onSuccess,onCheck,onLock}:{item:Schema['CatalogObject'];access:Access;disabled:boolean;version?:Schema['CatalogVersion'];onUnlock:()=>void;onSuccess:(revision:number)=>void;onCheck:(revision:number,ready:boolean)=>void;onLock:(value:boolean)=>void}){
  const missingContract=handoffProblems((version?.spec??item.spec).agent?.handoff).length>0;
  disabled=disabled||missingContract;
  const [input,setInput]=useState(''),[check,setCheck]=useState<{scope:string;result:Schema['Preflight']}>(),[selection,select]=useState<Selection>(),[offset,setOffset]=useState(0);
  const action=useAction(),frozen=useRef<Frozen|undefined>(undefined),history=useQuery<Schema['AgentTrialPage']>(access.project?`/catalog/${item.id}/trials?project_id=${encodeURIComponent(access.project)}&offset=${offset}`:null,10000);
  const uncertain=!!frozen.current&&action.error instanceof APIError&&(action.error.status===0||action.error.status>=500);
  const locked=action.busy||uncertain,scope=version?`v${version.number}`:`r${item.revision}`,inputScope=`${access.project}/${scope}/${input}`;
  useEffect(()=>{onLock(locked);return()=>onLock(false);},[locked,onLock]);
  async function inspect(signal:AbortSignal){
    setCheck(undefined);
    try{const result=await apiRequest<Schema['Preflight']>(version?`/versions/${version.id}/preflight`:`/catalog/${item.id}/draft/preflight`,{method:'POST',csrf:access.session.csrf,body:version?{project_id:access.project,input}:{project_id:access.project,revision:item.revision,input},signal});if(action.active()){setCheck({scope:inputScope,result});if(!version)onCheck(item.revision,result.ready);}}
    catch(error){if(action.active()&&error instanceof APIError&&error.response&&typeof error.response==='object'&&'checks' in error.response){setCheck({scope:inputScope,result:error.response as Schema['Preflight']});if(!version)onCheck(item.revision,false);}throw error;}
  }
  async function submit(signal:AbortSignal){
    const request=frozen.current??{body:{project_id:access.project,revision:item.revision,input},path:version?`/versions/${version.id}/run`:`/catalog/${item.id}/trials`,key:`${Date.now()}.${crypto.randomUUID()}`,version:version?.number};
    frozen.current=request;
    try{
      const value=await apiRequest<Schema['Submission']>(request.path,{method:'POST',csrf:access.session.csrf,key:request.key,body:request.version?{project_id:request.body.project_id,input:request.body.input}:request.body,signal});
      const id=receipt(value);if(action.active()){select({request:id,revision:request.version?undefined:request.body.revision,version:request.version});frozen.current=undefined;history.reload();}
    }catch(error){if(error instanceof APIError&&error.status>=400&&error.status<500)frozen.current=undefined;throw error;}
  }
  return <aside className="agent-trial" aria-label="试运行"><div className="agent-panel-title"><h2>试运行</h2><span className="agent-revision">{version?'发布版本':'已保存草稿'} {scope}</span></div>
    {version&&<button disabled={locked} onClick={onUnlock}>切回当前草稿</button>}
    <p className="muted">先检查环境，再运行独立任务。试运行会调用模型，按当前重试策略执行。</p>
    {missingContract?<p className="notice">此配置缺少完整交接契约；请补齐草稿，历史发布版本需重新发布后使用。</p>:disabled&&!version&&<p className="notice">保存当前修改后即可试运行；查看者和已归档助手不能执行。</p>}
    <RunInput key={version?.id??item.id} version={{spec:version?.spec??item.spec}} value={input} change={setInput} disabled={disabled||locked}/>
    <div className="toolbar"><button disabled={disabled||locked} onClick={()=>void action.run(inspect,'环境检查完成')}>检查环境</button><button className="primary" disabled={action.busy||disabled&&!uncertain} onClick={()=>void action.run(submit,'试运行请求已接受')}>{uncertain?'重试同一试运行':version?`试运行 v${version.number}`:'试运行草稿'}</button></div>
    {uncertain&&<p className="notice">提交结果未确认，已固定原修订、输入和请求键。重试会继续同一请求；请先确认结果。</p>}
    <ActionNotice {...action}/>{check?.scope===inputScope&&<PreflightStatus result={check.result}/>}
    {selection&&<TrialReceipt key={selection.request} selection={selection} access={access} onSuccess={onSuccess}/>}
    <details className="agent-history"><summary>草稿试运行历史</summary><QueryState {...history} retry={history.reload} empty={history.data?.items.length===0?'还没有草稿试运行。':undefined}/>
      {history.data?.items.map(row=><button key={row.request_id} className="agent-trial-row" aria-pressed={selection?.request===row.request_id} onClick={()=>select({request:row.request_id,revision:row.revision})}><span>草稿 r{row.revision}</span><time>{timestamp(row.created_at)}</time><Status value={row.status}/></button>)}
      <Pager offset={offset} next={history.data?.next_offset} change={setOffset}/>
    </details>
  </aside>;
}
function TrialReceipt({selection,access,onSuccess}:{selection:Selection;access:Access;onSuccess:(revision:number)=>void}){
  const query=useQuery<Schema['Submission']>(`/requests/${selection.request}`,5000);
  return <section className="agent-result"><h3>试运行结果 · {selection.version?`v${selection.version}`:`草稿 r${selection.revision}`}</h3><QueryState {...query} retry={query.reload}/>
    <a className="identifier" href={`#/requests/${selection.request}`}>回执 {selection.request}</a>
    {query.data?.run_id?<TrialResult id={query.data.run_id} access={access} revision={selection.revision} onSuccess={onSuccess}/>:<p>提交意图已保存，正在等待调度回执。</p>}
  </section>;
}
function TrialResult({id,access,revision,onSuccess}:{id:string;access:Access;revision?:number;onSuccess:(revision:number)=>void}){
  const query=useQuery<Schema['RunDetails']>(`/runs/${id}`,5000),action=useAction(),run=query.data;
  useEffect(()=>{if(revision&&run?.status==='succeeded')onSuccess(revision);},[revision,run?.status,onSuccess]);
  const cancel=run&&run.project_id===access.project&&(access.role==='manager'||access.role==='operator'&&run.created_by===access.session.id);
  return <><QueryState {...query} retry={query.reload}/>{run&&<><div className="toolbar"><Status value={run.status}/><a href={`#/runs/${id}`}>完整运行详情 →</a>{cancel&&!run.stop_confirmed&&<button disabled={action.busy||run.cancel_requested} onClick={()=>void action.run(async()=>{await apiRequest(`/runs/${id}/cancel`,{method:'POST',csrf:access.session.csrf});query.reload();},'取消请求已记录')}>取消试运行</button>}</div><ActionNotice {...action}/>
    {run.error&&<p role="alert">{run.error.category}：{run.error.message}</p>}
    <Approvals run={id} access={access} stopped={run.stop_confirmed||run.cancel_requested}/>
    {run.logs_expired?<p>结果与日志已过保留期，正文已清除。</p>:<>{run.nodes.map(node=><div key={node.id}>{node.error&&<p role="alert">{node.error.message}</p>}{node.output&&<pre className="agent-output">{output(node.output)}</pre>}</div>)}<details><summary>实际执行尝试与工具记录</summary><Attempts run={id}/></details></>}
    <details><summary>实际用量与费用</summary><Usage run={run}/></details>
  </>}</>;
}
function output(value:string){try{const parsed=JSON.parse(value);return typeof parsed.text==='string'?parsed.text:value;}catch{return value;}}
