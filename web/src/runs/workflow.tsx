import { ErrorNotice, Status, type Schema } from '../console/components';
import {currentNodes,gateResult} from './loop';

type Dependency = { id: string; depends_on?: string[] };
export function dagLayers<T extends Dependency>(nodes: T[]): T[][] {
  const ids=new Set(nodes.map(node=>node.id));
  if(ids.size!==nodes.length || nodes.length>32)throw Error('节点数量无效或标识重复');
  if(nodes.some(node=>node.depends_on?.some(id=>!ids.has(id))))throw Error('依赖节点不存在');
  const done=new Set<string>(),layers:T[][]=[];
  while(done.size<nodes.length){
    const layer=nodes.filter(node=>!done.has(node.id)&&(node.depends_on??[]).every(id=>done.has(id)));
    if(!layer.length)throw Error('节点依赖存在循环');
    layers.push(layer);layer.forEach(node=>done.add(node.id));
  }
  return layers;
}

export function activityText(event?:Schema['TraceEvent']) {
  if(event?.kind==='tool')return `正在调用工具：${event.name}`;
  if(event?.kind==='model')return `正在请求模型：${event.name}`;
  return 'Agent 正在执行节点';
}

export function WorkflowGraph({ definition, nodes, selected, select, activities=[],stopped=false }: { definition: Schema['Workflow']; nodes: Schema['NodeRun'][]; selected?:string; select?:(id:string)=>void;activities?:Schema['TraceEvent'][];stopped?:boolean }) {
  let layers: Schema['Node'][][];
  const current=currentNodes(nodes);
  try { layers=dagLayers(definition.nodes); } catch(error) {return <ErrorNotice error={error as Error}/>;}
  return <section aria-label="只读流程图" className="dag"><div className="dag-layers">
    {layers.map((layer,index)=><div className="dag-layer" key={index}><p>第 {index+1} 层</p>{layer.map(node=>{
      const state=current.find(item=>item.node_id===node.id),loop=definition.loops?.find(l=>l.nodes.includes(node.id)),verdict=loop?.gates.includes(node.id)?gateResult(state):undefined;
      const status=state?.next_retry_at?'waiting_retry':state?.status??'queued';
      const activity=activities.find(item=>item.node_id===node.id);
      return <button key={node.id} className={`dag-node dag-node-${status}`} aria-pressed={selected===node.id} onClick={()=>select?select(node.id):document.getElementById(`node-${node.id}`)?.focus()}>
        <strong>{node.id}</strong><span>{node.executor}{node.read_only?' · 只读':''}</span>{!state&&stopped?<span className="status">未执行</span>:<Status value={status}/>}
        {node.description&&<span className="agent-description">{node.description}</span>}
        {activity&&<span className="agent-activity">{activityText(activity)}</span>}
        {!!state?.attempt&&<small>第 {state.attempt} 次尝试</small>}
        {loop&&<small>循环 {loop.id}{state?.loop_round?` · 第 ${state.loop_round} 轮`:''}{loop.gates.includes(node.id)?' · 质量门':''}</small>}
        {verdict&&<small>{verdict.passed?'质量检查通过':'质量检查未通过，需返修'}</small>}
        {!!state?.model_retry&&<small>模型重试 {state.model_retry} 次</small>}
        {state?.review_round&&<small>第 {state.review_round} 轮计划修订</small>}
        {node.when&&<small>条件：{node.when.field.from}{node.when.field.pointer} = {JSON.stringify(node.when.equals)}</small>}{state?.reused_from&&<small>已复用检查点</small>}
        <small>依赖：{node.depends_on?.join('、')||'无'}</small></button>;
    })}</div>)}
  </div></section>;
}
