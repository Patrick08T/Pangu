import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,QueryState,timestamp,type Schema} from '../console/components';
import {receipt as receiptID} from '../runs/submission';

type Intent=Schema['ApprovalRebuildIntent'];
export function RunRebuilds({run,project,csrf}:{run:string;project:string;csrf:string}){
  const query=useQuery<Intent[]>(`/runs/${encodeURIComponent(run)}/rebuild-intents`);
  const items=!query.loading&&!query.error?query.data:undefined;
  return <section><h3>系统失效后的后继重建</h3><p>重建由原创建人请求，服务重新核验权限、旧执行停止确认和干净基线。只读重规划不等于批准；新执行仍须新一代审核，不会撤销旧任务的外部副作用。</p>
    <QueryState {...query} retry={query.reload}/>{items&&!Array.isArray(items)?<p role="alert">重建意图回执无效，不能视为没有失效依赖。</p>:Array.isArray(items)&&<>
      {items.length===0&&<p>当前没有可见重建意图。</p>}{items.map(item=>item.run_id!==run?<p key={item.id} role="alert">重建意图不属于当前运行，已隐藏。</p>:<RebuildCard key={item.id} item={item} run={run} project={project} csrf={csrf}/>)}</>}
    <button disabled={query.loading} onClick={query.reload}>刷新重建意图与权限</button>
  </section>;
}

function RebuildCard({item,run,project,csrf}:{item:Intent;run:string;project:string;csrf:string}){
  const [confirmed,setConfirmed]=useState(false),[blocked,setBlocked]=useState(false),[receipt,setReceipt]=useState<Schema['Submission']>(),action=useAction();
  const invalidSuccessor=item.successor_request_id!==undefined&&(typeof item.successor_request_id!=='string'||!(/^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i).test(item.successor_request_id));
  const allowed='can_rebuild' in item&&item.can_rebuild===true&&!!csrf&&!invalidSuccessor;
  const reason='blocked_reason' in item&&typeof item.blocked_reason==='string'?item.blocked_reason:'';
  const successor=receipt?.request_id||(!invalidSuccessor?item.successor_request_id:undefined);
  async function rebuild(signal:AbortSignal){
    try{
      const value=await apiRequest<Schema['Submission']>(`/runs/${encodeURIComponent(run)}/rebuild-intents/${encodeURIComponent(item.id)}`,{method:'POST',body:{},csrf,signal});
      receiptID(value);
      if(typeof value.task_id!=='string'||!value.task_id.trim()||value.project_id!==project)throw new Error('服务未返回有效后继回执；请刷新意图核对，不会自动重试。');
      if(action.active())setReceipt(value);
    }catch(error){if(action.active())setBlocked(true);throw error;}
  }
  return <article className="panel"><h4>节点：{item.node_id}</h4><p>审批 generation {item.from_generation} → {item.to_generation}</p><p>失效原因：{item.reason}</p>
    <p>{item.requires_stop?'必须先确认旧执行已停止。':'停止要求以服务当前核验为准。'}{item.ready_at?` 意图准备时间：${timestamp(item.ready_at)}；不代替提交时的重新检查。`:''}</p>
    {successor?<><p>{receipt?.status==='pending'?'后继请求已登记，等待服务调度；不能视为执行或审批已通过。':'服务已有后继提交；实际执行与审核状态请查看回执。'}</p><a href={`#/requests/${encodeURIComponent(successor)}`}>查看后继提交回执</a></>:<>
      {invalidSuccessor?<p className="notice error" role="alert">既有后继标识无效；请刷新意图核对，不能再次创建或跳转。</p>:!allowed&&<p className="notice">{reason||'服务未明确授予当前用户重建权限；仅可查看。'}</p>}
      <label className="check"><input type="checkbox" checked={confirmed} disabled={!allowed||action.busy||blocked} onChange={event=>setConfirmed(event.target.checked)}/>我理解这是后继只读重规划，仍须新审批且不撤销旧副作用</label>
      <button disabled={!allowed||!confirmed||action.busy||blocked} onClick={()=>void action.run(rebuild)}>请求重建并重新审批</button>
      {blocked&&<p className="notice error">请求未确认或权限/状态已变化。请刷新意图、核对既有后继并重新确认；不会静默重试。</p>}
    </>}<ActionNotice {...action}/>
  </article>;
}
