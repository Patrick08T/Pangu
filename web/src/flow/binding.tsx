import {useState} from 'react';
import type {Schema} from '../console/components';
import {bindingProblem,fieldName,fields,identifier,label,types,typeNames,type Flow,type Step,type Versions} from './model';

export function FieldSource({value,change,flow,step,versions,condition=false}:{value:Schema['FieldBinding'];change:(value:Schema['FieldBinding'])=>void;flow:Flow;step:Step;versions:Versions;condition?:boolean}) {
  const sources=['$input',...(step.depends_on??[]).filter(id=>flow.nodes.some(node=>node.id===id))];
  function schema(from:string){const source=flow.nodes.find(n=>n.id===from);return from==='$input'?flow.parameters:source?(source.output_schema??versions[source.agent_version]?.spec.agent?.output_schema):undefined;}
  function options(from:string){return fields(schema(from)).filter(field=>!condition||['boolean','string'].includes(field.type));}
  const shape=schema(value.from),choices=options(value.from);
  const problem=!sources.includes(value.from)?'来源不是直接上游':bindingProblem(flow,step,value,versions);
  const selected=choices.find(field=>field.pointer===value.pointer);
  function sourceChange(from:string){const first=options(from)[0];change({from,pointer:first?.pointer??'',type:first?.type??(condition?'boolean':'string')});}
  return <div className="flow-source"><label className="field">数据来源<select value={value.from} onChange={e=>sourceChange(e.target.value)}><option value="">选择来源</option>{sources.map(id=><option key={id} value={id}>{id==='$input'?'流程输入':label(flow,id)}</option>)}{!sources.includes(value.from)&&value.from&&<option value={value.from}>失效来源：{value.from}</option>}</select></label>
    {!!choices.length&&<label className="field">选择字段<select value={selected?value.pointer:'__custom'} onChange={e=>{const field=choices.find(f=>f.pointer===e.target.value);if(field)change({...value,...field});}}><option value="__custom" disabled>请选择字段</option>{choices.map(field=><option key={field.pointer} value={field.pointer}>{fieldName(field.pointer)} · {typeNames[field.type]}</option>)}</select></label>}
    {problem&&<p role="alert" className="error-text">引用已失效：{problem}。请重新选择；原值仍然保留。</p>}
    {selected&&!problem&&<small className="muted">{value.from==='$input'?'流程输入':label(flow,value.from)} › {fieldName(value.pointer)} · {typeNames[value.type]}</small>}
    {!shape&&<p className="muted">来源未声明字段结构，可展开高级路径指定；执行时会检查实际值。</p>}
    {shape&&!choices.length&&condition&&<p className="muted">此来源没有文本或布尔字段，请更换来源。</p>}
    <details><summary>高级路径</summary><label className="field">字段路径<input placeholder="/text，留空表示整个值" maxLength={256} value={value.pointer} onChange={e=>change({...value,pointer:e.target.value})}/></label>
      <label className="field">值类型<select value={value.type} onChange={e=>change({...value,type:e.target.value})}>{(condition?['boolean','string']:types).map(type=><option key={type} value={type}>{typeNames[type]}</option>)}</select></label></details>
  </div>;
}
export function Mappings({step,flow,versions,change}:{step:Step;flow:Flow;versions:Versions;change:(patch:Partial<Step>)=>void}) {
  const [name,setName]=useState('');
  function set(key:string,value:Schema['FieldBinding']){change({bindings:{...step.bindings,[key]:value}});}
  return <details><summary>输入字段映射（{Object.keys(step.bindings??{}).length}）</summary><p className="muted">连线决定执行先后。这里选择传给助手的命名参数，只能引用流程输入或直接上游。</p>
    {Object.entries(step.bindings??{}).map(([key,value])=><div className="flow-mapping" key={key}><div className="section-heading"><strong>{key}</strong><button type="button" onClick={()=>change({bindings:Object.fromEntries(Object.entries(step.bindings??{}).filter(([id])=>id!==key))})}>移除映射</button></div><FieldSource value={value} change={next=>set(key,next)} step={step} flow={flow} versions={versions}/></div>)}
    <label className="field">新参数名<input value={name} maxLength={64} onChange={e=>setName(e.target.value)} placeholder="例如 goal"/></label>
    <button type="button" disabled={!identifier.test(name)||Object.hasOwn(step.bindings??{},name)||Object.keys(step.bindings??{}).length>=32} onClick={()=>{set(name,{from:'$input',pointer:'',type:flow.parameters?.type??'string'});setName('');}}>添加映射</button>
  </details>;
}
