import type {Schema} from '../console/components';

export function ApprovalConfiguration({value,change,sources}:{value:Schema['ApprovalPolicy'];change:(value:Schema['ApprovalPolicy'])=>void;sources?:{id:string;label:string}[]}){
  return <div><label className="field">审核报告来源{sources?<select value={value.source_node} onChange={e=>change({...value,source_node:e.target.value})}><option value="">选择直接上游</option>{sources.map(s=><option key={s.id} value={s.id}>{s.label}</option>)}</select>:<input value={value.source_node} onChange={e=>change({...value,source_node:e.target.value})} placeholder="在工作流中的直接上游节点 ID"/>}</label>
    <label className="field">纠偏时重新执行的规划节点<input value={value.revision_nodes.join(',')} onChange={e=>change({...value,revision_nodes:e.target.value.split(',').map(v=>v.trim()).filter(Boolean)})} placeholder="用英文逗号分隔节点 ID"/></label>
    <label><input type="checkbox" checked={!!value.execution_plan} onChange={e=>change({...value,execution_plan:e.target.checked})}/>审批执行图差异，批准后发布新版本并保留成果续跑</label>
    <p className="muted">来源报告需包含结构化 review。修订节点须为只读 Codex 上游，包含汇总节点及所有受影响的规划节点；发布时检查。不自动过期，修订后再次人工审核。</p>
  </div>;
}
