import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';
import {fieldName,fields,typeNames} from './model';

export function AssistantSummary({version}:{version?:Schema['CatalogVersion']}){
  const agent=version?.spec.agent;
  const connection=useQuery<Schema['CatalogVersion']>(agent?.executor==='eino'&&agent.connection_version?`/versions/${encodeURIComponent(agent.connection_version)}`:null);
  const valid=connection.data?.id===agent?.connection_version&&connection.data?.kind==='connection';
  const provider=valid?connection.data?.spec.connection?.provider:undefined;
  if(!agent)return <p className="muted">尚未读取到助手版本，请检查版本选择。</p>;
  const mode={research:'调研',competition:'竞品分析',planning:'产品规划',design:'方案设计'};
  return <section className="flow-assistant" aria-label="助手能力">
    <strong>{agent.executor==='codex'?'原生 Codex':provider==='codex'?'Eino · Codex 账号模型':'Eino 助手'}</strong>
    <p>{agent.executor==='codex'?(agent.read_only?'只读检查项目文件，不修改文件。':'可在项目独立工作区内修改文件。'):'由 Eino 管理工具与编排；模型连接不授予原生 Codex 的项目文件执行能力。'}</p>
    {agent.executor==='eino'&&agent.connection_version&&<QueryState {...connection} retry={connection.reload}/>}
    {agent.role==='product_manager'&&<p>产品经理 · {agent.mode?mode[agent.mode]:'未指定任务方式'}</p>}
    <details><summary>职责与输出</summary><p className="flow-instruction">{agent.instruction}</p>
      {agent.output_schema?<ul>{fields(agent.output_schema).map(field=><li key={field.pointer}>{fieldName(field.pointer)} · {typeNames[field.type]}</li>)}</ul>:<p className="muted">未声明结构化输出；下游字段引用需在执行时检查。</p>}
      <p className="muted">职责和工具固定在助手版本中。修改助手并发布新版本后，在本步骤选择新版本。</p>
      <a href={`#/catalog/${version!.object_id}`}>查看助手配置</a>
    </details>
  </section>;
}
