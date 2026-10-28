import {useEffect,useState} from 'react';
import {apiRequest} from '../api/request';
import {useAction} from '../api/hooks';
import {ActionNotice,ErrorNotice,Page,canManageCatalog,canEdit,type Access,type Schema} from '../console/components';
import {AssistantLibrary,useVersions} from './catalog';
import {FlowCanvas} from './canvas';
import {StepPanel,FlowSettings} from './panel';
import {useDraft,useLeaveGuard} from './history';
import {FlowVersions} from './version';
import {productBrief} from '../onboarding/input';
import {arrange,display,duplicate,issues,label,removeNodes,type Flow} from './model';
import './style.css';

export function FlowStudio({access,item}:{access:Access;item?:Schema['CatalogObject']}) {
  const [base,setBase]=useState(item),[name,setName]=useState(item?.name??''),[selected,select]=useState<string>(),[error,setError]=useState<Error>(),[published,setPublished]=useState<Schema['CatalogVersion']>();
  const draft=useDraft(item?.spec.flow??{nodes:[]}),flow=draft.value,action=useAction(),lookup=useVersions(flow);
  const allowed=canManageCatalog(access),writable=allowed&&!base?.archived,disabled=!writable||action.busy;
  const dirty=name!==(base?.name??'')||JSON.stringify(flow)!==JSON.stringify(base?.spec.flow??{nodes:[]});useLeaveGuard(dirty);
  useEffect(()=>{if(!item&&base&&!dirty)window.location.hash=`/catalog/${base.id}`;},[item,base,dirty]);
  const problems=issues(flow,lookup.loading?undefined:lookup.versions),step=flow.nodes.find(n=>n.id===selected);
  function change(next:Flow){if(disabled)return;draft.change(next);setError(undefined);}
  function edit(operation:()=>Flow){try{change(operation());}catch(reason){setError(reason as Error);}}
  function add(choice:Schema['RunnableChoicePage']['items'][number]){
    if(disabled||flow.nodes.length>=32)return;const id=`step_${crypto.randomUUID().slice(0,8)}`;
    const parameters=flow.parameters??(choice.version.spec.agent?.role==='product_manager'?structuredClone(productBrief):undefined);
    change(display({...flow,parameters,nodes:[...flow.nodes,{id,agent_version:choice.version.id}]},id,{name:choice.name,x:60+flow.nodes.length%3*320,y:80+Math.floor(flow.nodes.length/3)*180}));select(id);
  }
  async function save(signal:AbortSignal){
    if(!name.trim())throw Error('请填写工作流名称');
    const body={name,spec:{flow},...(base?{revision:base.revision}:{kind:'workflow'})};
    const result=await apiRequest<Schema['CatalogObject']>(base?`/catalog/${base.id}/draft`:'/catalog',{method:base?'PUT':'POST',csrf:access.session.csrf,body,signal});
    if(action.active()){setBase(result);setName(result.name);draft.reset(result.spec.flow!);setPublished(undefined);}
  }
  async function publish(signal:AbortSignal){
    if(!base)return;const version=await apiRequest<Schema['CatalogVersion']>(`/catalog/${base.id}/publish`,{method:'POST',csrf:access.session.csrf,body:{revision:base.revision},signal});if(action.active())setPublished(version);
  }
  async function archive(signal:AbortSignal){
    if(!base)return;await apiRequest(`/catalog/${base.id}/archive`,{method:'POST',csrf:access.session.csrf,body:{archived:!base.archived},signal});
    const result=await apiRequest<Schema['CatalogObject']>(`/catalog/${base.id}`,{signal});if(action.active()){setBase(result);setName(result.name);draft.reset(result.spec.flow!);select(undefined);setPublished(undefined);}
  }
  return <Page title="工作流编排" description="连接共享 Agent 角色，发布流程；执行任务时选择项目。" actions={<a className="button" href="#/workflow">返回工作流</a>}>
    <div className="flow-studio"><div className="flow-toolbar"><label>工作流名称<input aria-label="工作流名称" value={name} maxLength={42} disabled={disabled} placeholder="例如：产品方案到代码审查" onChange={e=>setName(e.target.value)}/></label>
      <span className="flow-save-state">{base?.archived?'已归档 · 只读':!writable?'只读':dirty?'有未保存修改':base?'已保存':'新草稿'}</span>
      <button type="button" disabled={disabled||(!dirty&&!!base)} onClick={()=>void action.run(save,'草稿已保存')}>保存草稿</button>
      <button type="button" className="primary" disabled={disabled||dirty||!base||!!problems.length||lookup.loading} onClick={()=>void action.run(publish,'已发布，任务将使用此固定版本')}>发布</button>
      {published&&<a className="button primary" href={`#/start/${published.id}`}>运行此版本</a>}
    </div><ActionNotice {...action}/><ErrorNotice error={error}/>
      <div className="flow-workbench">{writable?<AssistantLibrary disabled={disabled||flow.nodes.length>=32} add={add}/>:<div className="flow-library"><h2>流程步骤</h2>{flow.nodes.map(n=><button type="button" key={n.id} onClick={()=>select(n.id)}>{label(flow,n.id)}</button>)}</div>}
        <FlowCanvas flow={flow} versions={lookup.versions} selected={selected} select={select} disabled={disabled} change={change} problems={problems.flatMap(p=>p.id?[p.id]:[])}/>
        {step?<StepPanel key={step.id} step={step} flow={flow} versions={lookup.versions} disabled={disabled} change={next=>edit(()=>next)} copy={()=>edit(()=>duplicate(flow,step.id,`step_${crypto.randomUUID().slice(0,8)}`))} remove={()=>{change(removeNodes(flow,[step.id]));select(undefined);}}/>:<FlowSettings flow={flow} change={change} disabled={disabled}/>}
      </div>
      <div className="flow-bottom"><button type="button" onClick={()=>select(undefined)}>流程输入与预算</button><span>{flow.nodes.length} / 32 步骤</span><div className="toolbar"><button type="button" disabled={disabled||!draft.canUndo} onClick={draft.undo}>撤销</button><button type="button" disabled={disabled||!draft.canRedo} onClick={draft.redo}>重做</button><button type="button" disabled={disabled||!flow.nodes.length} onClick={()=>change(arrange(flow))}>自动排列</button></div></div>
      {problems.length>0&&<div className="flow-issues"><strong>发布前还需处理 {problems.length} 项</strong>{problems.map((problem,i)=><button type="button" key={i} onClick={()=>select(problem.id)}>{problem.message}</button>)}</div>}
    </div>
    {base&&<FlowVersions key={`${base.id}:${base.revision}:${published?.id}`} id={base.id} canRun={canEdit(access)&&!!access.project}/>}
    {base&&allowed&&<p><button type="button" disabled={action.busy||dirty} onClick={()=>void action.run(archive,base.archived?'工作流已恢复':'工作流已归档，历史版本保留')}>{base.archived?'恢复工作流':'归档工作流'}</button></p>}
  </Page>;
}
