import {useState} from 'react';
import {JSONField,type Schema} from '../console/components';
import {ConnectionPicker} from './connection';
import {templates} from './templates';
import {ShapeFields} from '../flow/shape';
import {HandoffFields} from '../agent/handoff';
import '../flow/style.css';

export function AgentFields({value,change,disabled=false}:{value:string;change:(value:string)=>void;disabled?:boolean}) {
  const [advanced,setAdvanced]=useState(false),[preset,setPreset]=useState('');
  let spec:Schema['CatalogSpec']|undefined;
  try {const parsed=JSON.parse(value);if(parsed&&typeof parsed==='object'&&!Array.isArray(parsed)&&parsed.agent&&typeof parsed.agent==='object'&&!Array.isArray(parsed.agent))spec=parsed;}catch{ /* 保留无效 JSON，让用户在高级输入中修正。 */ }
  const node=spec?.agent;
  function update(fields:Partial<Schema['Node']>){change(JSON.stringify({...spec,agent:{...node,...fields}},null,2));}
  function choose(index:string){setPreset(index);if(index==='')return;const template=JSON.parse(templates[Number(index)][1]) as Schema['CatalogDocument'];change(JSON.stringify({...spec,agent:template.spec.agent},null,2));}
  return <>{!advanced&&node?<>
    <label className="field">常用角色<select aria-label="常用角色" disabled={disabled} value={preset} onChange={e=>choose(e.target.value)}><option value="">保留当前配置 / 自定义</option>{templates.map(([label],index)=><option key={label} value={index}>{label}</option>)}</select><small>选择角色会填入职责和执行配置，你可以继续修改。</small></label>
    <label className="field">助手职责<textarea required rows={5} maxLength={16384} disabled={disabled} value={node.instruction} onChange={e=>update({instruction:e.target.value})}/></label>
    <details open><summary>交接契约（发布必填）</summary><HandoffFields value={node.handoff} change={handoff=>update({handoff})} disabled={disabled}/></details>
    <p className="muted">{node.executor==='codex'?'使用 Worker 专用 Codex 账号。代码修改只保存在独立工作区。':node.role==='product_manager'?'产品经理通过已发布模型连接执行，生成报告。':'使用已发布模型连接执行任务。'}</p>
    {node.executor==='eino'&&<><ConnectionPicker value={node.connection_version??''} change={(id,model)=>update({connection_version:id,model})} disabled={disabled}/><p className="muted">没有可选连接时，请由配置管理者在<a href="#/connection">模型连接</a>中准备并发布。</p></>}
    {node.executor==='codex'&&<label className="check"><input type="checkbox" checked={node.read_only??false} disabled={disabled} onChange={e=>update({read_only:e.target.checked})}/>只读检查，不修改文件</label>}
    <details><summary>输出字段</summary><p className="muted">声明后，工作流下游可以直接选择这些字段。执行结果必须满足此结构。</p>
      <label className="check"><input type="checkbox" checked={!!node.output_schema} disabled={disabled} onChange={e=>update({output_schema:e.target.checked?{type:'object',properties:{}}:undefined})}/>声明结构化输出</label>
      {node.output_schema&&<ShapeFields title="助手输出字段" value={node.output_schema} change={output_schema=>update({output_schema})} disabled={disabled}/>}
    </details>
  </>:!advanced?<p role="alert">助手配置不是有效对象，请在高级配置中修正。</p>:<JSONField label="配置 JSON" value={value} onChange={change} disabled={disabled}/>}
    <details><summary>高级配置</summary><label className="check"><input type="checkbox" checked={advanced} disabled={disabled} onChange={e=>setAdvanced(e.target.checked)}/>直接编辑 JSON</label></details>
  </>;
}
