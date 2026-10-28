import {useState} from 'react';
import {VersionPicker} from '../catalog/selection';
import {ErrorNotice} from '../console/components';
import {FieldSource,Mappings} from './binding';
import {ShapeFields} from './shape';
import {AssistantSummary} from './assistant';
import {connect,disconnect,display,label,update,type Flow,type Step,type Versions} from './model';

export function StepPanel({step,flow,versions,change,disabled,copy,remove}:{step:Step;flow:Flow;versions:Versions;change:(flow:Flow)=>void;disabled:boolean;copy:()=>void;remove:()=>void}) {
  const [error,setError]=useState<Error>();
  const version=versions[step.agent_version],inherited=version?.spec.agent?.approval;
  const patch=(value:Partial<Step>)=>change(update(flow,step.id,value));
  return <div className="flow-panel"><h2>步骤设置</h2><fieldset disabled={disabled}>
    <label className="field">步骤名称<input value={label(flow,step.id)} maxLength={42} onChange={e=>change(display(flow,step.id,{name:e.target.value}))}/></label>
    <p className="muted">{version?`固定版本 v${version.number}`:'正在读取版本'}</p>
    <AssistantSummary key={step.agent_version} version={version}/>
    <details><summary>更换助手或版本</summary><p className="muted">更换后保留字段映射，请核对新助手的输入输出。</p><VersionPicker kind="agent" agentsOnly value={step.agent_version} disabled={disabled} change={item=>{if(item)patch({agent_version:item.id});}}/></details>
    <label className="field">本步骤任务<textarea rows={7} maxLength={8192} value={step.task??''} placeholder="角色在本步骤需要完成什么、什么情况下停止" onChange={e=>patch({task:e.target.value})}/></label>
    <label className="flow-check"><input type="checkbox" checked={!!(step.read_only||version?.spec.agent?.read_only)} disabled={!!version?.spec.agent?.read_only} onChange={e=>patch({read_only:e.target.checked})}/>本步骤只读</label>
    <p className="muted">只读限制优先于角色的文件写入能力；角色已有的只读限制不能在此关闭。</p>
    <details><summary>步骤输出字段{step.output_schema?' · 已指定':''}</summary><label className="flow-check"><input type="checkbox" checked={!!step.output_schema} onChange={e=>patch({output_schema:e.target.checked?{type:'object',properties:{}}:undefined})}/>为本步骤定义输出字段</label>
      {step.output_schema&&<ShapeFields value={step.output_schema} change={output_schema=>patch({output_schema})} disabled={disabled}/>}<p className="muted">覆盖本步骤的输出结构，不修改角色模板。任务指令需明确按这些字段输出 JSON。</p>
    </details>
    <details><summary>前置步骤（{step.depends_on?.length??0}）</summary><p className="muted">也可拖动画布连接点。所有前置步骤完成后才执行。</p>
      {flow.nodes.filter(n=>n.id!==step.id).map(node=><label className="flow-check" key={node.id}><input type="checkbox" checked={step.depends_on?.includes(node.id)??false} onChange={e=>{try{change(e.target.checked?connect(flow,node.id,step.id):disconnect(flow,node.id,step.id));setError(undefined);}catch(reason){setError(reason as Error);}}}/>{label(flow,node.id)}</label>)}<ErrorNotice error={error}/>
      {flow.nodes.length===1&&<p className="muted">添加其他步骤后可连接。</p>}
    </details>
    <Mappings step={step} flow={flow} versions={versions} change={patch}/>
    <details><summary>执行条件{step.when?' · 已设置':''}</summary><label className="flow-check"><input type="checkbox" checked={!!step.when} onChange={e=>patch({when:e.target.checked?{field:{from:'$input',pointer:'',type:'boolean'},equals:true}:undefined})}/>满足条件才执行</label>
      {step.when&&<><FieldSource condition flow={flow} step={step} versions={versions} value={step.when.field} change={field=>patch({when:{field,equals:field.type===step.when!.field.type?step.when!.equals:field.type==='boolean'?true:''}})}/>
        <label className="field">等于{step.when.field.type==='boolean'?<select value={String(step.when.equals)} onChange={e=>patch({when:{...step.when!,equals:e.target.value==='true'}})}><option value="true">是</option><option value="false">否</option></select>:<input value={String(step.when.equals)} maxLength={300} onChange={e=>patch({when:{...step.when!,equals:e.target.value}})}/>}</label>
        <p className="notice">条件不满足时跳过本步骤；依赖它的步骤也会跳过。连接多个互斥分支不会自动汇合。</p></>}
    </details>
    <details><summary>人工审批{step.approval||inherited?' · 已启用':''}</summary>
      {inherited&&<p className="muted">助手版本自带审批要求，不能在工作流中关闭。</p>}
      <label className="flow-check"><input type="checkbox" checked={!!(step.approval||inherited)} disabled={!!inherited} onChange={e=>patch({approval:e.target.checked?{timeout_seconds:600}:undefined})}/>执行前等待人工批准</label>
      {(step.approval||inherited)&&<label className="field">最长等待（秒）<input type="number" min={1} max={3600} value={(step.approval??inherited)!.timeout_seconds} onChange={e=>patch({approval:{timeout_seconds:Number(e.target.value)}})}/></label>}
    </details>
    <div className="toolbar"><button type="button" onClick={copy} disabled={flow.nodes.length>=32}>复制步骤</button><button type="button" className="danger" onClick={remove}>删除步骤</button></div>
    <small className="identifier">步骤编号：{step.id}</small>
  </fieldset></div>;
}

export function FlowSettings({flow,change,disabled}:{flow:Flow;change:(flow:Flow)=>void;disabled:boolean}) {
  return <div className="flow-panel"><h2>流程设置</h2><p className="muted">点击画布步骤配置助手；连线决定先后，同一层可并行执行。</p><fieldset disabled={disabled}>
    <label className="flow-check"><input type="checkbox" checked={!!flow.parameters} onChange={e=>change({...flow,parameters:e.target.checked?{type:'object',properties:{}}:undefined})}/>使用结构化输入表单</label>
    {flow.parameters?<ShapeFields value={flow.parameters} change={parameters=>change({...flow,parameters})} disabled={disabled}/>:<p className="muted">默认接受文本任务，所有步骤均能读取。产品经理助手接受产品需求表单；字段映射需先定义结构化输入。</p>}
    <details><summary>调用预算{flow.budget?' · 已指定':' · 未设上限'}</summary><label className="flow-check"><input type="checkbox" checked={!!flow.budget} onChange={e=>change({...flow,budget:e.target.checked?{model_calls:1056,search_calls:960,codex_sessions:96}:undefined})}/>指定调用上限</label>
      {flow.budget&&Object.entries({model_calls:'模型调用',search_calls:'搜索调用',codex_sessions:'Codex 会话'}).map(([key,text])=><label className="field" key={key}>{text}<input type="number" min={0} max={4096} value={flow.budget![key as keyof typeof flow.budget]} onChange={e=>change({...flow,budget:{...flow.budget!,[key]:Number(e.target.value)}})}/></label>)}
    </details>
  </fieldset><p className="muted">最多 32 个步骤。Codex 的工作区合并、审批与并行限制以发布检查为准。定时任务在发布版本后设置。</p></div>;
}
