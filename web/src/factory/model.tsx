import {CodexModelPicker} from '../agent/codex';
import type {Schema} from '../console/components';

export type FactoryModel=Pick<Schema['FactoryInput'],'connection_version'|'model'|'reasoning_effort'|'fast'>;
type Choice=Schema['ConnectionChoicePage']['items'][number];

export function messageModel(input?:Schema['FactoryInput']):FactoryModel {
 return {connection_version:input?.connection_version??'',model:input?.model,reasoning_effort:input?.reasoning_effort,fast:input?.fast};
}

export function FactoryModelPicker({value,change,connection,disabled}:{value:FactoryModel;change:(value:FactoryModel)=>void;connection?:Choice;disabled:boolean}) {
 if(!connection)return null;
 if(connection.provider!=='codex')return <p className="muted">本次使用连接发布时绑定的模型：{connection.model}。</p>;
 const node:Schema['Node']={id:'factory',executor:'codex',instruction:'生成工厂方案',model:value.model,reasoning_effort:value.reasoning_effort,fast:value.fast};
 function update(next:Schema['Node']) {
  const changed=next.model!==value.model;
  change({connection_version:value.connection_version,model:next.model||undefined,reasoning_effort:changed?undefined:next.reasoning_effort,fast:changed?undefined:next.fast});
 }
 return <div className="factory-model"><CodexModelPicker value={node} change={update} disabled={disabled} editable defaultLabel={connection.model?`连接默认模型：${connection.model}`:'Worker 默认模型'}/><p className="muted">这些设置用于本次方案生成；候选 Agent 的模型在方案中单独配置。</p></div>;
}

export function ModelSummary({input}:{input:Schema['FactoryInput']}) {
 let fast='运行环境默认';
 if(input.fast!==undefined)fast=input.fast?'开启':'关闭';
 return <small>生成配置：{input.model||'连接／Worker 默认'} · 推理：{input.reasoning_effort||'运行环境默认'} · Fast：{fast}</small>;
}
