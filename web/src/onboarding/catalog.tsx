import {useEffect,useState} from 'react';
import {useQuery} from '../api/hooks';
import {QueryState,type Schema} from '../console/components';

export function AssistantPicker({value,change,disabled}:{value:string;change:(value:Schema['CatalogVersion']|undefined)=>void;disabled:boolean}) {
  const [offset,setOffset]=useState(0);
  const query=useQuery<Schema['RunnableChoicePage']>(`/runnables?${new URLSearchParams({offset:String(offset)})}`);
  const items=query.data?.items.filter(item=>!item.version.archived)??[];
  function label(item:typeof items[number]){return items.some(other=>other.version.object_id!==item.version.object_id&&other.name===item.name)?`${item.name} · v${item.version.number} · ${item.version.object_id.slice(0,8)}`:item.name;}
  useEffect(()=>{if(!disabled&&!value&&items.length===1&&offset===0&&query.data?.next_offset===undefined)change(items[0].version);},[query.data,disabled,value,offset]);
  function page(next:number){change(undefined);setOffset(next);}
  return <div className="assistant-picker"><QueryState {...query} retry={query.reload}/>
    {!query.loading&&!query.error&&items.length===0?<div className="empty"><h2>还没有已发布的角色或工作流</h2><p>通过对话创建并发布助手，再开始执行任务。</p><a className="button primary" href="#/agent">准备助手</a></div>:
      <label className="field">选择助手<select disabled={disabled||query.loading||!!query.error} value={value} onChange={e=>change(items.find(item=>item.version.id===e.target.value)?.version)}><option value="">请选择助手或工作流</option>{(['workflow','agent'] as const).map(kind=><optgroup key={kind} label={kind==='workflow'?'工作流':'Agent 角色'}>{items.filter(item=>item.version.kind===kind).map(item=><option key={item.version.id} value={item.version.id}>{label(item)}</option>)}</optgroup>)}</select></label>}
    {(offset>0||query.data?.next_offset!==undefined)&&<div className="toolbar"><button type="button" disabled={disabled||!offset} onClick={()=>page(0)}>返回第一页</button><button type="button" disabled={disabled||query.data?.next_offset===undefined} onClick={()=>page(query.data!.next_offset!)}>更多助手</button></div>}
  </div>;
}

const modes:Record<string,string>={planning:'产品规划：根据你的目标产出 PRD、优先级和路线图。',design:'方案设计：产出用户流程、功能方案和可执行任务。',research:'资料调研：请在资料来源中提供至少一个授权搜索词、网址或项目文件。',competition:'竞品分析：填写候选产品、比较维度和资料来源。只有任务目标不能完成调研。'};
export function AssistantScope({version}:{version:Schema['CatalogVersion']}) {
  const agent=version.spec.agent;
  return <div className="assistant-scope"><p>{agent?.role==='product_manager'?modes[agent.mode??'']:version.kind==='workflow'?'按已配置的步骤执行任务。':agent?.read_only?'只读助手：查看项目并提供建议，不修改项目文件。':'按任务目标执行，文件修改保存在独立工作区。'}</p>
    {agent?.role==='product_manager'&&<small>只使用你提供或明确授权的资料，未提供的背景会标记为未知。</small>}
    <details><summary>查看助手说明与版本</summary><p>{agent?.instruction??'工作流的具体步骤可在助手管理中查看。'}</p><p>本次固定使用 v{version.number}。<a href={`#/catalog/${version.object_id}`}>管理此配置</a></p></details>
  </div>;
}

const checks:Record<string,{name:string;help:string}>={worker:{name:'执行服务',help:'执行服务未就绪，请启动当前项目对应的 Worker 后重试。'},eino:{name:'模型账号',help:'模型账号未就绪，请检查模型连接或 Worker 的 Codex 登录。'},codex:{name:'Codex 账号',help:'Codex 未就绪，请检查 Worker 的 Codex 安装和登录。'},project:{name:'项目',help:'项目不可用，请检查 Git 来源、分支和执行服务。'},search:{name:'联网搜索',help:'Codex 联网搜索未就绪，请检查 Worker 的 Codex 安装和登录；也可使用授权网址或项目文件。'},input:{name:'任务输入',help:'任务输入不完整，请检查目标及所选助手需要的信息。'}};
export function PreflightStatus({result}:{result:Schema['Preflight']}) {
  return <div className={`notice ${result.ready?'':'error'}`} role={result.ready?'status':'alert'}><strong>{result.ready?'检查通过':'任务尚未开始'}</strong><ul>{result.checks.map((check,index)=>{
    const item=check.component==='codex_model'?{name:'Codex 模型',help:'所选模型不在 Worker 的模型目录中。请在助手工作台选择下拉框中的模型或默认模型；不要填写显示名称或推理档位。'}:checks[check.component]??{name:check.component,help:'此项尚未就绪，请查看运行环境或联系维护者。'};
    return <li key={index}>{['ready','configured'].includes(check.status)?`${item.name}已就绪`:item.help}</li>;
  })}</ul>{!result.ready&&<a href="#/settings">打开设置与诊断</a>}</div>;
}
