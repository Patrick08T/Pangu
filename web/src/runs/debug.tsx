import {useEffect,useLayoutEffect,useRef,useState} from 'react';
import {useQuery} from '../api/hooks';
import {Pager,QueryState,Status,timestamp,type Schema} from '../console/components';
import {AttemptDetail} from './execution';
import {WorkflowGraph,activityText} from './workflow';
import {TracePanel} from './trace';

export function RunDebugger({run}:{run:Schema['RunDetails']}){
  const [selected,select]=useState<string>();
  const [opened,setOpened]=useState(false);
  const panel=useRef<HTMLDivElement>(null);
  const position=useRef({x:0,y:0,left:0,top:0});
  const activity=useQuery<Schema['TraceEvent'][]>(run.logs_expired||run.stop_confirmed?null:`/runs/${run.id}/activity`,2000);
  const active=run.logs_expired||run.stop_confirmed||activity.error?[]:(activity.data??[]).filter(event=>run.nodes.some(node=>node.id===event.node_run_id&&node.attempt===event.attempt&&!['succeeded','failed','cancelled','skipped'].includes(node.status)));
  const node=opened?run.definition?.nodes.find(item=>item.id===selected):undefined,state=run.nodes.find(item=>item.node_id===selected);
  useLayoutEffect(()=>{
    if(!selected)return;
    if(opened){window.scrollTo(0,0);return;}
    const graph=panel.current?.querySelector('.dag');
    if(graph){graph.scrollLeft=position.current.left;graph.scrollTop=position.current.top;}
    panel.current?.querySelector<HTMLButtonElement>('button[aria-pressed="true"]')?.focus({preventScroll:true});
    window.scrollTo(position.current.x,position.current.y);
  },[opened,selected]);
  function open(id:string){
    const graph=panel.current?.querySelector('.dag');
    position.current={x:window.scrollX,y:window.scrollY,left:graph?.scrollLeft??0,top:graph?.scrollTop??0};
    select(id);setOpened(true);
  }
  if(!run.definition)return null;
  return <section className="agent-monitor" aria-label="Agent 执行状态"><h2>Agent 执行状态</h2><p className="muted">每张卡片代表一次工作流节点执行。同一角色参与多个阶段时分别展示；点击查看当前活动和执行记录。</p>
    <p className="agent-progress">共 {run.definition.nodes.length} 个节点 · {run.nodes.filter(n=>n.status==='running').length} 个执行中 · {run.nodes.filter(n=>n.status==='succeeded').length} 个完成 · {run.nodes.filter(n=>n.status==='failed').length} 个失败 · {run.nodes.filter(n=>n.status==='skipped').length} 个跳过</p>
    <QueryState {...activity} retry={activity.reload}/>{activity.error&&<p className="notice">当前活动暂不可用，请勿将旧记录视为仍在执行。</p>}
    <div ref={panel} className="run-debugger"><WorkflowGraph definition={run.definition} nodes={run.nodes} selected={selected} select={open} activities={active} stopped={run.stop_confirmed}/>
      {node&&<NodeDebug key={`${run.id}/${node.id}/${run.logs_expired}`} run={run} node={node} state={state} activity={active.find(event=>event.node_id===node.id)} unavailable={!!activity.error} close={()=>setOpened(false)}/>}
    </div>
  </section>;
}
function NodeDebug({run,node,state,activity,unavailable,close}:{run:Schema['RunDetails'];node:Schema['Node'];state?:Schema['NodeRun'];activity?:Schema['TraceEvent'];unavailable:boolean;close:()=>void}){
  const heading=useRef<HTMLHeadingElement>(null);
  useEffect(()=>heading.current?.focus({preventScroll:true}),[]);
  const input=useQuery<Schema['InputPreview'][]>(run.logs_expired?null:`/runs/${run.id}/inputs`,run.stop_confirmed?0:5000);
  const preview=input.data?.find(item=>item.node_id===node.id);
  const labels={ready:'输入已就绪',pending:'等待上游输出',skipped:'条件未命中或依赖已跳过',invalid:'输入错误'};
  return <aside className="node-debug" aria-label="所选节点详情"><div className="section-heading"><button onClick={close}><span aria-hidden="true">← </span>返回任务</button><h3 tabIndex={-1} ref={heading}>Agent · {node.id} {!state&&run.stop_confirmed?<span className="status">未执行</span>:<Status value={state?.status??'queued'}/>}</h3></div>
    <p>{node.executor==='codex'?'原生 Codex':'Eino 助手'}{node.read_only?' · 只读':''}</p>
    {node.description&&<p>{node.description}</p>}
    {state&&state.status!=='queued'?<p>{timestamp(state.started_at)} 至 {timestamp(state.finished_at)}</p>:<p>尚未开始执行</p>}
    {run.logs_expired?<p className="notice">日志与输入已过保留期，无法查看执行内容。</p>:<>
      {state?.reused_from&&<p className="notice">此节点复用了成功检查点，没有产生新的执行尝试。</p>}
      <CurrentActivity state={state} activity={activity} stopped={run.stop_confirmed} unavailable={unavailable}/>
      <TracePanel key={node.id} run={run} nodeID={node.id}/>
      <details><summary>Agent 职责与任务指令</summary><pre>{node.instruction}</pre></details>
      <h4>节点输入</h4><QueryState {...input} retry={input.reload}/>
      {preview?<><p>{labels[preview.state]}</p>{preview.error&&<p className="error-text">{preview.error}</p>}{preview.state==='ready'&&<pre>{JSON.stringify(preview.parameters,null,2)}</pre>}</>:!input.loading&&!input.error&&<p>尚无可用输入记录。</p>}
      {run.definition?.input&&<details><summary>全局任务输入</summary><pre>{run.definition.input}</pre></details>}
      <h4>输出与错误</h4>{state?.error&&<div className="notice error"><strong>{state.error.category}</strong><pre>{state.error.message}</pre>{state.error.truncated&&<p>诊断已截断。</p>}</div>}
      {state?.output?<pre>{state.output}</pre>:<p className="muted">尚无节点输出。</p>}
      {state&&!state.reused_from&&<NodeAttempts key={state.id} run={run} node={state}/>}
    </>}
  </aside>;
}

function CurrentActivity({state,activity,stopped,unavailable}:{state?:Schema['NodeRun'];activity?:Schema['TraceEvent'];stopped:boolean;unavailable:boolean}){
  const terminal=stopped||!!state&&['succeeded','failed','cancelled','skipped'].includes(state.status);
  let message=state?.status==='running'?'节点执行中，等待下一条模型或工具活动记录。':'等待节点开始执行。';
  if(unavailable)message='当前活动读取失败，暂无法确认。';
  if(terminal)message='该节点当前没有正在执行的操作，可查看下方历史记录。';
  return <section className="current-activity" aria-label="当前活动"><h4>当前正在做什么</h4>
    {activity&&!terminal&&!unavailable?<>
      <strong>{activityText(activity)}</strong><p>开始于 {timestamp(activity.time)} · 已持续 {Math.max(0,Math.floor((Date.now()-Date.parse(activity.time))/1000))} 秒 · 第 {activity.attempt} 次尝试</p>
      {activity.input&&<pre>{activity.input}</pre>}{activity.truncated&&<p>当前操作摘要已截断。</p>}
    </>:<p>{message}</p>}
    <small>活动每 2 秒更新，显示最近开始且尚未结束的操作；这是执行记录，不是模型内部思维链。</small>
  </section>;
}
function NodeAttempts({run,node}:{run:Schema['RunDetails'];node:Schema['NodeRun']}){
  const [offset,setOffset]=useState(0),[selected,select]=useState<number>();
  const query=useQuery<Schema['AttemptPage']>(`/runs/${run.id}/attempts?offset=${offset}&limit=100`,run.stop_confirmed?0:5000);
  const items=(query.data?.items??[]).filter(item=>item.node_run_id===node.id).sort((a,b)=>b.attempt-a.attempt);
  const current=items.find(item=>item.attempt===selected)??items[0];
  return <section><h4>执行尝试与证据</h4><QueryState {...query} retry={query.reload}/>
    {current?<><label className="field">执行尝试<select value={current.attempt} onChange={event=>select(Number(event.target.value))}>{items.map(item=><option key={item.attempt} value={item.attempt}>第 {item.attempt} 次</option>)}</select></label>
      <AttemptDetail key={`${current.node_run_id}/${current.attempt}`} run={run.id} item={current} stopped={run.stop_confirmed}/></>:
      !query.loading&&!query.error&&<p>{offset||query.data?.next_offset!==undefined?'本页暂无此步骤记录，请翻页查看。':'尚无实际执行尝试。'}</p>}
    {query.data&&(offset>0||query.data.next_offset!==undefined)&&<Pager offset={offset} next={query.data.next_offset} change={next=>{select(undefined);setOffset(next);}}/>}
  </section>;
}
