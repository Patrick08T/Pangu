import {useState} from 'react';
import {useQuery,useAction} from '../api/hooks';
import {apiRequest,APIError} from '../api/request';
import {ActionNotice,QueryState,timestamp,type Schema} from '../console/components';
import {CapabilityNotice,Lifecycle} from './status';
import {CursorPager} from './entries';
export type EvolutionProps={project:string;csrf:string;rollback?:boolean;available?:boolean};
const prefix=(project:string)=>`/projects/${encodeURIComponent(project)}/evolution/intents`;
const states:Record<string,string>={reserved:'已预留：尚未评测',preparing:'准备中',evaluating:'评测中',passed:'判定阶段已完成：不在此披露结果',publishing:'发布中',switching:'入口切换中',stabilizing:'稳定确认中',reconciling:'对账中',rolling_back:'回退对账中',prepared:'候选已准备',rejected:'已拒绝',insufficient:'样本或证据不足',failed:'执行失败',cancelled:'已取消',pending:'等待处理'};
const rollbackStates=['switching','observing','stabilizing','reconciling','rolling_back'];

export function EvolutionHistory(props:EvolutionProps){
  const [cursor,setCursor]=useState(''),[selected,setSelected]=useState('');
  const query=useQuery<Schema['EvolutionIntentPage']>(`${prefix(props.project)}?limit=20${cursor?`&cursor=${encodeURIComponent(cursor)}`:''}`);
  return <section><h2>实验与晋级历史</h2><CapabilityNotice available={props.available} reason="执行能力关闭不删除历史。状态是协调记录，不等于独立评测判定或真实模型收益。"/>
    <p>角色发布、流程发布、入口采用、观察与稳定是不同阶段。受保护用例、预期答案和逐例结果不向此控制台披露。</p>
    <QueryState {...query} retry={query.reload} empty={!query.data?.items?.length?'暂无可查看的进化记录。':undefined}/>
    {!query.error&&!query.loading&&query.data?.items?.map(item=><article className="panel" key={item.id}><h3>{item.id}</h3><IntentSummary item={item}/><button aria-label={`查看进化 ${item.id}`} onClick={()=>setSelected(item.id)}>查看发布与回退</button></article>)}
    <CursorPager cursor={cursor} next={!query.error?query.data?.next_cursor:undefined} busy={query.loading} change={value=>{setCursor(value);setSelected('');}}/>
    {selected&&<IntentDetail key={`${props.project}:${selected}`} {...props} id={selected} close={()=>setSelected('')} changed={query.reload}/>}
  </section>;
}

function IntentSummary({item}:{item:Schema['EvolutionIntentSummary']}){
  return <><p>{item.kind==='memory_benefit'?'记忆增益实验（不修改角色）':item.kind==='role_change'?'角色指令对照实验':'实验类型未登记'}</p>
    <p>策略修订 {item.policy_revision} · generation {item.generation} · {timestamp(item.started_at)}</p>{Object.hasOwn(states,item.state)?<p className="status">{states[item.state]}</p>:<Lifecycle rollout={item.state}/>}
    {item.experiment_id&&<><dl><dt>真实实验标识</dt><dd>{item.experiment_id}</dd></dl><p>实验标识不是机器回执或通过证明；意图标识不能替代实验依据。</p></>}
    {item.kind==='memory_benefit'?<dl><dt>评测目标经验版本</dt><dd>{item.benefit_memory_version_id||'未返回目标版本'}</dd></dl>:<>
      <dl><dt>角色发布版本</dt><dd>{item.publication?.agent_id||'未返回发布记录'}</dd><dt>流程发布版本</dt><dd>{item.publication?.flow_id||'未返回发布记录'}</dd></dl>
      <p>发布记录不代表入口已切换；未返回入口明细时不能推断采用范围或稳定指针。</p></>}</>;
}

function IntentDetail({id,close,changed,...props}:EvolutionProps&{id:string;close:()=>void;changed:()=>void}){
  const query=useQuery<Schema['EvolutionIntentSummary']>(`${prefix(props.project)}/${encodeURIComponent(id)}`);
  return <section className="panel"><h2>进化详情</h2><button onClick={close}>关闭进化详情</button><QueryState {...query} retry={query.reload}/>
    {!query.error&&!query.loading&&query.data&&<><IntentSummary item={query.data}/>
      <p>此接口尚未提供独立受保护评测判定，不能用生命周期状态代替评测通过。</p>
      {props.rollback===true&&rollbackStates.includes(query.data.state)?<Rollback {...props} item={query.data} done={()=>{query.reload();changed();}}/>:<p>当前权限或状态不允许回退。</p>}
    </>}
  </section>;
}

function Rollback({item,done,...props}:EvolutionProps&{item:Schema['EvolutionIntentSummary'];done:()=>void}){
  const [reason,setReason]=useState(''),[confirmed,setConfirmed]=useState(false),[conflict,setConflict]=useState(false),action=useAction();
  const [generation,setGeneration]=useState(item.generation),[latest,setLatest]=useState<Schema['EvolutionIntentSummary']>();
  const path=`${prefix(props.project)}/${encodeURIComponent(item.id)}`;
  async function submit(signal:AbortSignal){
    try{
      const receipt=await apiRequest<Schema['EvolutionIntentSummary']>(`${path}/rollback`,{method:'POST',csrf:props.csrf,body:{generation,reason:reason.trim()},signal});
      if(receipt?.id!==item.id||receipt.project_id!==props.project||!['rolling_back','rolled_back','insufficient'].includes(receipt.state))throw new Error('回退回执尚未确认，请重新读取状态；不自动重试');
      if(action.active())done();
    }catch(error){if(error instanceof APIError&&error.status===409){setConflict(true);setLatest(undefined);}throw error;}
  }
  async function loadLatest(signal:AbortSignal){
    const value=await apiRequest<Schema['EvolutionIntentSummary']>(path,{signal});
    if(value?.id!==item.id||value.project_id!==props.project||!Number.isSafeInteger(value.generation)||value.generation<generation)throw new Error('最新进化记录无效，保留回退理由');
    if(action.active())setLatest(value);
  }
  return <div><h3>受控回退</h3><p>保留人工修改，只处理仍由该晋级控制的未来绑定。不会恢复已删除来源或撤销业务副作用。当前比较代际 {generation}。</p>
    <label className="field">回退理由<input maxLength={1024} value={reason} disabled={action.busy} onChange={event=>setReason(event.target.value)}/></label>
    <label className="check"><input type="checkbox" checked={confirmed} disabled={action.busy} onChange={event=>setConfirmed(event.target.checked)}/>仅回退仍由本次晋级控制的未来入口</label>
    <button disabled={!props.csrf||!confirmed||!reason.trim()||reason.length>1024||conflict||action.busy} onClick={()=>void action.run(submit,'回退请求已由服务确认；等待入口对账完成')}>提交受控回退</button><ActionNotice {...action}/>
    {conflict&&<div className="notice error"><p>代际已变化，理由已保留；读取最新状态并核对，不会自动重试。</p><button disabled={action.busy} onClick={()=>void action.run(loadLatest,'已读取最新状态，未提交回退')}>读取最新代际（保留理由）</button></div>}
    {latest&&<div><IntentSummary item={latest}/><button disabled={!rollbackStates.includes(latest.state)||action.busy} onClick={()=>{setGeneration(latest.generation);setLatest(undefined);setConflict(false);setConfirmed(false);}}>已核对最新状态，采用 generation {latest.generation}</button><p>终态不可用旧进化记录回退。采用新代际后仍需重新确认并提交。</p></div>}
  </div>;
}
