import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';

type Agent=Schema['Node'];
type Choice=Schema['ModelChoicePage']['items'][number];
type Props={value:Agent;change:(node:Agent)=>void;disabled:boolean;editable:boolean;defaultLabel?:string};

export function CodexModelPicker({value,change,disabled,editable,defaultLabel='Worker 默认模型'}:Props){
  const query=useQuery<Schema['ModelChoicePage']>(editable?'/models/codex':null);
  const items=query.data?.items??[];
  return <><label className="field">Codex 模型<select value={value.model??''} disabled={disabled||!editable||query.loading} onChange={e=>change({...value,model:e.target.value})}>
    <option value="">{defaultLabel}</option>
    {value.model&&!items.some(item=>item.id===value.model)&&<option value={value.model}>当前模型：{value.model}</option>}
    {items.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}
  </select></label><ModelOptions value={value} change={change} model={items.find(item=>item.id===value.model)} disabled={disabled||!editable||query.loading} validate={!!query.data}/>
  {editable&&<><QueryState {...query} retry={query.reload}/><button type="button" disabled={disabled||query.loading} onClick={query.reload}>刷新模型列表</button><p className="muted">候选来自 Worker 当前 Codex 程序的内置模型目录。</p></>}</>;
}

function ModelOptions({value,change,model,disabled,validate}:{value:Agent;change:(node:Agent)=>void;model?:Choice;disabled:boolean;validate:boolean}){
  const levels=model?.reasoning_levels??[],unsupported=!!value.reasoning_effort&&!levels.includes(value.reasoning_effort);
  return <><label className="field">推理强度<select disabled={disabled||!value.model} value={value.reasoning_effort??''} onChange={e=>change({...value,reasoning_effort:e.target.value as Agent['reasoning_effort']||undefined})}>
    <option value="">运行环境默认</option>{unsupported&&<option value={value.reasoning_effort}>当前设置：{value.reasoning_effort}</option>}
    {levels.map(level=><option key={level} value={level}>{level}{level===model?.default_reasoning?'（模型默认）':''}</option>)}
  </select></label>
  <label className="field">Fast 模式<select disabled={disabled||!value.model} value={value.fast===undefined?'':String(value.fast)} onChange={e=>change({...value,fast:e.target.value===''?undefined:e.target.value==='true'})}>
    <option value="">运行环境默认</option><option value="true" disabled={validate&&!model?.fast}>开启</option><option value="false">关闭</option>
  </select></label>
  {!value.model&&<p className="muted">选择具体模型后可设置推理强度与 Fast。</p>}
  {validate&&value.model&&unsupported&&<p role="alert">所选模型不支持当前推理强度，请重新选择。</p>}
  {validate&&value.fast&&!model?.fast&&<p role="alert">所选模型不支持 Fast，请关闭或选择其他模型。</p>}
  {model?.fast&&<p className="muted">Fast 使用优先服务档位，可能增加用量；实际可用性由模型账号决定。</p>}</>;
}

export function EinoModelOptions(props:Props){
  const connection=useQuery<Schema['CatalogVersion']>(props.value.connection_version?`/versions/${encodeURIComponent(props.value.connection_version)}`:null);
  const codex=connection.data?.id===props.value.connection_version&&connection.data?.spec.connection?.provider==='codex';
  const query=useQuery<Schema['ModelChoicePage']>(codex&&props.editable?'/models/codex':null);
  if(!codex)return <><QueryState {...connection} retry={connection.reload}/>{(props.value.reasoning_effort||props.value.fast!==undefined)&&!connection.loading&&<p role="alert">当前连接不支持 Codex 运行选项。<button type="button" disabled={props.disabled} onClick={()=>props.change({...props.value,reasoning_effort:undefined,fast:undefined})}>清除 Codex 运行选项</button></p>}</>;
  return <>{!props.value.model&&<p>当前连接使用默认模型。请在<a href="#/connection">模型连接</a>中指定具体模型并发布，再选择该连接版本，即可设置推理强度和 Fast。</p>}<ModelOptions {...props} disabled={props.disabled||!props.editable||query.loading} model={query.data?.items.find(item=>item.id===props.value.model)} validate={!!query.data}/><QueryState {...query} retry={query.reload}/></>;
}
