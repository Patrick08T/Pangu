import {useState} from 'react';
import {ConnectionPicker} from '../catalog/connection';
import {templates} from '../catalog/templates';
import {ShapeFields} from '../flow/shape';
import {engine,changeEngine,modeNames,policy,type Agent,type Draft} from './model';
import type {Schema} from '../console/components';
import {CodexModelPicker} from './codex';
import {HandoffFields} from './handoff';

const sections=['职责与说明','模型与执行','工具与资料','重试与权限','输出格式','交接契约'];
export function AgentConfiguration({value,change,disabled,editable}:{value:Draft;change:(value:Draft)=>void;disabled:boolean;editable:boolean}){
  const [tab,setTab]=useState(sections[0]),node=value.spec.agent!;
  function update(next:Agent){change({...value,spec:{...value.spec,agent:next}});}
  return <div className="agent-configuration"><nav className="agent-sections" aria-label="助手配置分区">{sections.map(label=><button type="button" key={label} aria-pressed={tab===label} onClick={()=>setTab(label)}>{label}</button>)}</nav>
    <div className="agent-fields"><h2>{tab}</h2><fieldset disabled={disabled} className="agent-fieldset">
      {tab===sections[0]&&<AgentIdentity value={value} change={change}/>}
      {tab===sections[1]&&<AgentEngine node={node} change={update} disabled={disabled} editable={editable}/>}
      {tab===sections[2]&&<AgentCapabilities node={node}/>}
      {tab===sections[3]&&<AgentPolicy node={node} change={update}/>}
      {tab===sections[5]&&<HandoffFields value={node.handoff} change={handoff=>update({...node,handoff})} disabled={disabled}/>}
      {tab===sections[4]&&<><label className="field">输出格式<select value={node.output_schema?'object':'text'} onChange={e=>update({...node,output_schema:e.target.value==='object'?{type:'object',properties:{}}:undefined})}><option value="text">自然语言文本</option><option value="object">结构化对象</option></select></label><p className="muted">声明字段后，执行结果必须符合结构，才能作为成功输出。</p>{node.output_schema&&<ShapeFields value={node.output_schema} title="助手输出字段" change={shape=>update({...node,output_schema:shape})} disabled={disabled}/>}</>}
    </fieldset></div>
  </div>;
}
function AgentIdentity({value,change}:{value:Draft;change:(value:Draft)=>void}){
  const node=value.spec.agent!;
  function update(patch:Partial<Agent>){change({...value,spec:{...value.spec,agent:{...node,...patch}}});}
  function preset(index:string){
    if(!index||!window.confirm('将替换职责、执行方式及限制；保留名称和已有审批要求。确认使用此角色预设？'))return;
    const preset=(JSON.parse(templates[Number(index)-1][1]) as Schema['CatalogDocument']).spec.agent!;
    change({...value,spec:{...value.spec,agent:{...preset,id:node.id,approval:node.approval,...(preset.executor==='eino'&&node.executor==='eino'?{connection_version:node.connection_version,model:node.model}:{})}}});
  }
  return <><label className="field">助手名称<input required maxLength={128} value={value.name} onChange={e=>change({...value,name:e.target.value})}/></label>
    <label className="field">用途说明<input value={node.description??''} maxLength={1024} onChange={e=>update({description:e.target.value})} placeholder="说明适用任务，让团队更容易选择"/></label>
    <label className="field">助手职责<textarea required className="agent-prompt" rows={14} maxLength={16384} value={node.instruction} onChange={e=>update({instruction:e.target.value})} placeholder="角色与目标、工作步骤、边界要求、交付标准"/></label>
    <p className="muted">清楚描述目标、可用资料、不能做的事和验收标准。任务内容在右侧填写。</p>
    <label className="field">套用角色预设<select value="" onChange={e=>preset(e.target.value)}><option value="">选择预设…</option>{templates.map(([name],i)=><option key={name} value={i+1}>{name}</option>)}</select></label>
  </>;
}
function AgentEngine({node,change,disabled,editable}:{node:Agent;change:(node:Agent)=>void;disabled:boolean;editable:boolean}){
  function select(next:string){if(window.confirm('切换执行方式会清除不适用的模型连接、角色和文件权限；职责、输出和审批要求保留。继续？'))change(changeEngine(node,next));}
  return <><label className="field">执行方式<select value={engine(node)} onChange={e=>select(e.target.value)}><option value="product_manager">Eino 产品经理</option><option value="eino">Eino 通用助手</option><option value="codex">原生 Codex</option></select></label>
    {node.role==='product_manager'&&<label className="field">产品能力<select value={node.mode} onChange={e=>change({...node,mode:e.target.value as Agent['mode']})}>{Object.entries(modeNames).map(([key,name])=><option key={key} value={key}>{name}</option>)}</select></label>}
    {node.executor==='eino'?<>{editable?<ConnectionPicker value={node.connection_version??''} change={(id,model)=>change({...node,connection_version:id,model})} disabled={disabled}/>:<p>固定连接：{node.connection_version||'尚未选择'} · 模型：{node.model||'连接默认模型'}</p>}<p className="muted">选择已发布的共享连接。Codex 账号连接共用 Worker 登录；工具与资料权限由 Eino 管理。</p></>:<><CodexModelPicker value={node.model??''} change={model=>change({...node,model})} disabled={disabled} editable={editable}/><p>使用 Worker 专用 Codex 登录；项目文件在独立工作区执行。</p></>}
  </>;
}
function AgentPolicy({node,change}:{node:Agent;change:(node:Agent)=>void}){
  const value=policy(node);
  return <>
    <label className="field">失败最多重试次数<input type="number" min={0} max={2} step={1} value={value.retries} onChange={e=>change({...node,policy:{...value,retries:Number(e.target.value)}})}/></label>
    {node.executor==='codex'&&<label className="field">项目文件权限<select value={node.read_only?'read':'write'} onChange={e=>change({...node,read_only:e.target.value==='read'})}><option value="read">只读项目</option><option value="write">允许在独立项目工作区内读写</option></select></label>}
    <p className="muted">运行没有业务执行时限；可在任务详情暂停后续节点或主动取消。失败重试可能增加模型消耗。文件产物由团队审查采纳。</p>
    <label className="check"><input type="checkbox" checked={!!node.approval} onChange={e=>change({...node,approval:e.target.checked?{timeout_seconds:300}:undefined})}/>执行前需要人工审批</label>
    {node.approval&&<label className="field">审批等待上限（秒）<input type="number" min={1} max={3600} value={node.approval.timeout_seconds} onChange={e=>change({...node,approval:{...node.approval!,timeout_seconds:Number(e.target.value)}})}/></label>}
  </>;
}
function AgentCapabilities({node}:{node:Agent}){
  const tools=node.executor==='codex'?['读取项目文件','受控命令执行',...(node.read_only?[]:['独立工作区内修改文件'])]:node.role==='product_manager'?['搜索已授权的公开搜索词','读取授权网页及搜索来源','读取授权项目资料','登记有来源依据的产品报告']:['根据任务输入生成回答'];
  return <><p>当前助手实际可用的能力：</p><ul className="agent-capabilities">{tools.map(tool=><li key={tool}>{tool}</li>)}</ul><p>资料授权在每次任务输入中指定。账号与服务端权限由项目管理者配置。</p>{node.role==='product_manager'&&<p className="muted">用于工作流时，还可读取直接上游报告；独立试运行没有上游节点。</p>}<p className="notice">每次运行是独立任务。当前不携带前一次任务的会话记忆。</p></>;
}
