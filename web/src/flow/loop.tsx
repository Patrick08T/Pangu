import type {Schema} from '../console/components';
import {dagLayers} from '../runs/workflow';
import {label,type Flow,type Versions,type Issue} from './model';

function gateShape(shape?:Schema['Shape']):Schema['Shape'] {
  return {type:'object',...shape,properties:{...shape?.properties,passed:{type:'boolean'},summary:{type:'string'}},required:[...new Set([...(shape?.required??[]),'passed','summary'])]};
}

export function markGate(flow:Flow,loopID:string,nodeID:string,checked:boolean,versions:Versions={}):Flow {
  let order=flow.nodes.map(n=>n.id);try{order=dagLayers(flow.nodes).flat().map(n=>n.id);}catch{/* 无效依赖由发布检查显示。 */}
  const loops=flow.loops?.map(loop=>loop.id!==loopID?loop:{...loop,gates:order.filter(id=>id===nodeID?checked:loop.gates.includes(id))});
  const nodes=flow.nodes.map(node=>{
    if(node.id!==nodeID||!checked)return node;
    const inherited=versions[node.agent_version]?.spec.agent,report=node.report_schema??inherited?.report_schema;
    return {...node,read_only:true,output_schema:gateShape(node.output_schema??inherited?.output_schema),...(report?{report_schema:gateShape(report)}:{})};
  });
  return {...flow,loops,nodes};
}

export function LoopSettings({flow,change,disabled,versions={}}:{flow:Flow;change:(flow:Flow)=>void;disabled:boolean;versions?:Versions}) {
  function edit(index:number,patch:Partial<Schema['Loop']>){change({...flow,loops:flow.loops?.map((l,i)=>i===index?{...l,...patch}:l)});}
  return <section aria-label="循环块设置"><h3>循环块</h3><p className="muted">同一轮的全部质量门通过后完成；不设轮数上限。未通过时保留补丁和证据进入下一轮，预算和取消仍生效。</p><fieldset disabled={disabled}>
    {flow.loops?.map((loop,index)=><div key={index} className="loop-settings">
      <label className="field">循环标识<input value={loop.id} maxLength={64} onChange={e=>edit(index,{id:e.target.value})}/></label>
      <p>选择成员（至少两个）</p>{flow.nodes.map(node=><label className="flow-check" key={node.id}><input aria-label={`${loop.id} 循环成员：${label(flow,node.id)}`} type="checkbox" checked={loop.nodes.includes(node.id)} onChange={e=>edit(index,{nodes:e.target.checked?[...loop.nodes,node.id]:loop.nodes.filter(id=>id!==node.id),gates:loop.gates.filter(id=>id!==node.id)})}/>{label(flow,node.id)}</label>)}
      <p>选择质量门，按依赖顺序执行</p>{flow.nodes.filter(node=>loop.nodes.includes(node.id)).map(node=><label className="flow-check" key={node.id}><input aria-label={`${loop.id} 质量门：${label(flow,node.id)}`} type="checkbox" checked={loop.gates.includes(node.id)} onChange={e=>change(markGate(flow,loop.id,node.id,e.target.checked,versions))}/>{label(flow,node.id)}</label>)}
      <p className="muted">标记质量门会将该步骤设为只读，并补齐 passed、summary 必填输出。全部修改步骤须先于首个质量门，质量门依次串行；块外从最后质量门接续。</p>
      <button type="button" onClick={()=>change({...flow,loops:flow.loops?.filter((_,i)=>i!==index)})}>移除循环块</button>
    </div>)}
    <button type="button" disabled={disabled||(flow.loops?.length??0)>=16} onClick={()=>change({...flow,loops:[...(flow.loops??[]),{id:`loop_${crypto.randomUUID().slice(0,8)}`,nodes:[],gates:[]}]})}>添加循环块</button>
  </fieldset></section>;
}

export function loopIssues(flow:Flow,versions?:Versions):Issue[] {
  const out:Issue[]=[],members=new Map<string,string>(),ids=new Set<string>();
  const add=(message:string,id?:string)=>out.push({id,message});
  for(const loop of flow.loops??[]){
    if(!/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(loop.id)||ids.has(loop.id))add('循环标识无效或重复');ids.add(loop.id);
    if(loop.nodes.length<2||!loop.gates.length)add(`循环 ${loop.id} 至少需要两个成员和一个质量门`);
    for(const id of loop.nodes){
      if(members.has(id))add(`循环成员 ${id} 重叠`,id);members.set(id,loop.id);
      const n=flow.nodes.find(n=>n.id===id),agent=n&&versions?.[n.agent_version]?.spec.agent;
      if(!n){add(`循环成员 ${id} 不存在`);continue;}
      if(n.when||n.approval||agent?.approval||agent?.role||agent&&agent.executor!=='codex')add(`循环成员 ${id} 需无条件和审批的 Codex 节点`,id);
    }
    for(const [i,id] of loop.gates.entries()){
      const n=flow.nodes.find(n=>n.id===id),agent=n&&versions?.[n.agent_version]?.spec.agent,shape=n?.output_schema??agent?.output_schema;
      if(!loop.nodes.includes(id)||loop.gates.indexOf(id)!==i||!n)add(`循环质量门 ${id} 必须是独立成员`,id);
      if(n&&!(n.read_only||agent?.read_only))add(`质量门 ${id} 必须只读`,id);
      if(!shape||shape.properties?.passed?.type!=='boolean'||shape.properties?.summary?.type!=='string'||!shape.required?.includes('passed')||!shape.required.includes('summary'))add(`质量门 ${id} 缺少必填 passed 和 summary`,id);
      if(i&&!ancestor(flow,loop.gates[i-1],id))add('质量门必须按依赖顺序串行执行',id);
    }
    for(const id of loop.nodes)if(!loop.gates.includes(id)&&loop.gates.length&&!ancestor(flow,id,loop.gates[0]))add(`循环步骤 ${id} 必须先于首个质量门完成`,id);
  }
  for(const node of flow.nodes)for(const id of node.depends_on??[]){
    const loop=flow.loops?.find(l=>l.nodes.includes(id));
    if(loop&&members.get(node.id)!==loop.id&&id!==loop.gates.at(-1))add(`循环 ${loop.id} 仅能从最后质量门连接外部步骤`,node.id);
  }
  return out;
}

function ancestor(flow:Flow,parent:string,id:string,seen=new Set<string>()):boolean {
  if(seen.has(id))return false;seen.add(id);
  return (flow.nodes.find(n=>n.id===id)?.depends_on??[]).some(p=>p===parent||ancestor(flow,parent,p,seen));
}
