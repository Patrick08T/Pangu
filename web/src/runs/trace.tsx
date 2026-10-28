import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {Download} from '../api/download';
import {ActionNotice,QueryState,Status,timestamp,type Access,type Schema} from '../console/components';

export function DebugControls({run,allowed,access}:{run:Schema['RunDetails'];allowed:boolean;access:Access}){
 const query=useQuery<Schema['DebugState']>(run.logs_expired?null:`/runs/${run.id}/debug`,run.stop_confirmed?0:2000),action=useAction();
 const state=query.data,disabled=action.busy||query.loading||!!query.error||run.stop_confirmed||run.cancel_requested;
 async function control(command:'pause'|'resume'|'step',signal:AbortSignal){
  if(!state)return;
  try{await apiRequest(`/runs/${run.id}/debug`,{method:'POST',body:{action:command,revision:state.revision},csrf:access.session.csrf,signal});}
  finally{query.reload();}
 }
 if(run.logs_expired)return null;
 return <section aria-label="运行调试控制"><h2>运行调试</h2><QueryState {...query} retry={query.reload}/>
  {state&&<><p>{run.stop_confirmed?'运行已停止':state.paused?'后续节点已暂停':'正常派发节点'} · 当前活动节点 {state.active} 个{(state.step_requested||state.step_node)&&' · 等待单步节点开始'}</p>
   <p className="muted">暂停在工作流节点开始前生效，已开始的节点继续执行。单步放行一个就绪节点尝试，随后保持暂停。</p>
   {allowed&&!run.stop_confirmed&&<div className="toolbar"><button disabled={disabled} onClick={()=>void action.run(signal=>control(state.paused?'resume':'pause',signal))}>{state.paused?'继续运行':'暂停后续节点'}</button>
    <button disabled={disabled||!state.paused||state.active>0||state.step_requested||!!state.step_node} onClick={()=>void action.run(signal=>control('step',signal))}>单步执行一个节点</button></div>}
  </>}<ActionNotice {...action}/>
 </section>;
}

export function TracePanel({run,nodeID}:{run:Schema['RunDetails'];nodeID?:string}){
 const [node,setNode]=useState(''),[kind,setKind]=useState(''),[status,setStatus]=useState(''),[before,setBefore]=useState<number>();
 const filters=new URLSearchParams();if(nodeID||node)filters.set('node',nodeID||node);if(kind)filters.set('kind',kind);if(status)filters.set('status',status);
 const suffix=filters.toString(),params=new URLSearchParams(filters);if(before)params.set('before',String(before));
 const query=useQuery<Schema['TracePage']>(run.logs_expired?null:`/runs/${run.id}/trace?${params}`,before||run.stop_confirmed?0:2000);
 function filter(change:(value:string)=>void,value:string){change(value);setBefore(undefined);}
 if(run.logs_expired)return null;
 return <section aria-label="行为追踪"><h2>{nodeID?'Agent 执行时间线':'行为追踪'}</h2><p className="muted">按最新到最早排列；运行时每 2 秒更新。记录模型请求摘要、工具参数和结果；敏感凭据脱敏，正文过长时显示截断标记。</p>
  <div className="toolbar">{!nodeID&&<label>追踪节点<select value={node} onChange={e=>filter(setNode,e.target.value)}><option value="">全部节点</option>{run.definition?.nodes.map(n=><option key={n.id} value={n.id}>{n.id}</option>)}</select></label>}
   <label>行为类型<select value={kind} onChange={e=>filter(setKind,e.target.value)}><option value="">全部类型</option><option value="node">节点</option><option value="model">模型请求</option><option value="tool">工具调用</option></select></label>
   <label>行为状态<select value={status} onChange={e=>filter(setStatus,e.target.value)}><option value="">全部状态</option>{Object.entries({running:'开始',succeeded:'成功',failed:'失败',cancelled:'取消'}).map(([id,label])=><option key={id} value={id}>{label}</option>)}</select></label>
   <button onClick={()=>{setBefore(undefined);query.reload();}}>回到最新记录</button>
   <Download path={`/runs/${run.id}/trace.ndjson?${suffix}`} filename={`trace-${run.id}.ndjson`}>导出筛选结果</Download>
  </div><QueryState {...query} retry={query.reload}/>
  {query.data?.items.length===0&&<p>尚无匹配的行为记录。</p>}
  {query.data?.items.map(event=><details key={event.id} className="node-result"><summary>{timestamp(event.time)} · <strong>{event.name}</strong> · {event.node_id} · {event.phase==='start'?<span className="status">开始记录</span>:<><Status value={event.status}/> · {event.duration_ms} ms</>}</summary>
   <p>{timestamp(event.time)} · 第 {event.attempt} 次尝试 · {event.kind}</p><p className="identifier">Span：{event.span_id} · 父 Span：{event.parent_span_id||'无'}</p>
   {event.input&&<><h4>参数 / 请求摘要</h4><pre>{event.input}</pre></>}{event.output&&<><h4>结果</h4><pre>{event.output}</pre></>}{event.error&&<><h4>错误</h4><pre>{event.error}</pre></>}{event.truncated&&<p className="notice">正文已截断，不能据此认定结果完整。</p>}
  </details>)}
  {query.data?.next_before!==undefined&&<button disabled={query.loading} onClick={()=>setBefore(query.data?.next_before)}>更早记录</button>}
 </section>;
}
