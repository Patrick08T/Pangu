import {useCallback,useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {APIError,apiRequest} from '../api/request';
import {ActionNotice,Page,QueryState,canEdit,canManageCatalog,type Access,type Schema} from '../console/components';
import {useLeaveGuard} from '../flow/history';
import {AgentConfiguration} from './configuration';
import {AgentTrial} from './trial';
import {AgentDifference,AgentVersions} from './version';
import {engineName,problems,type Draft} from './model';
import {handoffProblems} from './handoff';
import './style.css';

export function AgentStudio({item,access}:{item:Schema['CatalogObject'];access:Access}){
  const [base,setBase]=useState(item),[draft,setDraft]=useState<Draft>({name:item.name,spec:item.spec}),[tab,setTab]=useState('配置');
  const [publish,setPublish]=useState(false),[untested,setUntested]=useState(false),[published,setPublished]=useState<Schema['CatalogVersion']>(),[remote,setRemote]=useState(false);
  const [version,setVersion]=useState<Schema['CatalogVersion']>(),[locked,setLocked]=useState(false),[success,setSuccess]=useState(0),[check,setCheck]=useState<{revision:number;ready:boolean}>();
  const [restoring,setRestoring]=useState<Schema['CatalogVersion']>();
  const action=useAction(),dirty=JSON.stringify(draft)!==JSON.stringify({name:base.name,spec:base.spec});
  const editable=canManageCatalog(access),disabled=!editable||base.archived||action.busy||locked,errors=problems(draft);
  useLeaveGuard(dirty||locked,locked?'检查或试运行请求尚未确认。离开将丢失当前重试键，请先查试运行历史以免重复执行。确定离开？':undefined);
  const checked=useCallback((revision:number,ready:boolean)=>setCheck({revision,ready}),[]);
  function accept(next:Schema['CatalogObject']){if(action.active()){setBase(next);setDraft({name:next.name,spec:next.spec});setRemote(false);setPublish(false);setRestoring(undefined);}}
  async function save(spec=draft.spec){accept(await apiRequest<Schema['CatalogObject']>(`/catalog/${base.id}/draft`,{method:'PUT',csrf:access.session.csrf,body:{revision:base.revision,name:draft.name,spec}}));}
  async function release(){const next=await apiRequest<Schema['CatalogVersion']>(`/catalog/${base.id}/publish`,{method:'POST',csrf:access.session.csrf,body:{revision:base.revision}});if(action.active()){setPublished(next);setPublish(false);}}
  async function reload(){accept(await apiRequest<Schema['CatalogObject']>(`/catalog/${base.id}`));}
  async function archive(){await apiRequest(`/catalog/${base.id}/archive`,{method:'POST',csrf:access.session.csrf,body:{archived:!base.archived}});await reload();}
  const contractErrors=handoffProblems(draft.spec.agent?.handoff);
  const blocked=errors.length>0||contractErrors.length>0||draft.spec.agent?.executor==='eino'&&!draft.spec.agent.connection_version;
  return <div className="agent-studio"><Page title="Agent 工作台" description={`${base.name} · ${engineName(draft.spec.agent!)} · 工作空间共享`} actions={<a className="button" href="#/agent">返回助手列表</a>}>
    <div className="agent-topbar"><div role="status"><span className={`agent-save ${dirty?'is-dirty':''}`}>{action.busy?'正在处理…':dirty?'有未保存修改':`已保存 r${base.revision}`}</span>{base.archived&&<span className="status">已归档</span>}</div><div className="toolbar"><button disabled={disabled||!dirty||errors.length>0} onClick={()=>void action.run(()=>save(),'草稿已保存')}>保存草稿</button><button className="primary" disabled={disabled||dirty||!!blocked} onClick={()=>{setUntested(false);setPublish(!publish);}}>发布</button></div></div>
    {!editable&&<p className="notice">当前为只读视图。编辑需要工作空间配置管理权限；试运行使用当前项目。</p>}
    <ActionNotice {...action}/>{errors.length>0&&<ul className="notice error" role="alert">{errors.map(error=><li key={error}>{error}</li>)}</ul>}
    {contractErrors.length>0&&<p className="notice" role="status">交接契约尚未完成：{contractErrors.join('；')}。可暂存草稿，请在“交接契约”分区补齐后发布或试运行。</p>}
    {action.error instanceof APIError&&action.error.status===409&&<div className="notice"><p>服务器草稿可能已变化，本地修改已保留。查看差异后再决定如何处理。</p><button onClick={()=>setRemote(true)}>查看服务器草稿</button></div>}
    {remote&&<ServerDraft id={base.id} draft={draft} reload={()=>{if(window.confirm('丢弃本地编辑并读取服务器最新草稿？'))void action.run(reload);}}/>}
    {restoring&&<section className="agent-publish" role="alert"><h2>恢复 v{restoring.number} 为新草稿</h2><p>将替换当前配置并保存为新草稿，未保存的配置会丢弃；名称和历史版本保持不变。</p><button disabled={disabled} onClick={()=>void action.run(()=>save(restoring.spec),'已恢复为新草稿')}>确认恢复为新草稿</button><button disabled={action.busy} onClick={()=>setRestoring(undefined)}>暂不恢复</button></section>}
    {publish&&<section className="agent-publish"><h2>发布草稿 r{base.revision}</h2><p>结构与引用会由服务器再次校验。发布后生成固定版本，可用于任务、工作流与定时计划。</p><ul><li>本地字段检查：通过</li><li>当前修订环境检查：{check?.revision===base.revision?(check.ready?'通过':'未通过'):'未执行'}</li><li>当前修订试运行：{success===base.revision?'已观察到成功结果；质量仍需人工验收':'尚未确认成功'}</li></ul>{success!==base.revision&&<label className="check"><input type="checkbox" checked={untested} onChange={e=>setUntested(e.target.checked)}/>我确认发布未经成功试运行验证的版本</label>}<button className="primary" disabled={disabled||dirty||!!blocked||success!==base.revision&&!untested} onClick={()=>void action.run(release,'固定版本已发布')}>确认发布</button><button onClick={()=>setPublish(false)}>暂不发布</button></section>}
    {published&&<p className="notice">已发布 v{published.number}。<a href={`#/start/${published.id}`}>使用此版本开始任务 →</a></p>}
    <nav className="agent-tabs" aria-label="工作台视图">{['配置','试运行','版本'].map(value=><button key={value} aria-pressed={tab===value} onClick={()=>setTab(value)}>{value}</button>)}</nav>
    <div className="agent-workspace" data-view={tab}><div className="agent-editor-pane">{tab!=='版本'?<AgentConfiguration value={draft} change={setDraft} disabled={disabled} editable={editable}/>:<AgentVersions key={`${base.revision}/${published?.id??''}`} item={base} draft={draft} disabled={disabled} restore={setRestoring} test={v=>{setVersion(v);setTab('试运行');}}/>}
      <details className="agent-management"><summary>助手管理与原始配置</summary><p>节点标识：{base.spec.agent?.id}</p><label className="field">本地配置副本（可复制）<textarea readOnly className="code-input" rows={6} value={JSON.stringify(draft,null,2)}/></label><button disabled={!editable||action.busy||locked||dirty} onClick={()=>{if(window.confirm(base.archived?'恢复此助手？':'归档后不能新建运行或发布；历史记录保留。继续？'))void action.run(archive);}}>{base.archived?'恢复助手':'归档助手'}</button></details>
    </div><AgentTrial item={base} access={access} disabled={!access.project||!canEdit(access)||base.archived||dirty||action.busy||!version&&!!blocked} version={version} onUnlock={()=>setVersion(undefined)} onSuccess={setSuccess} onCheck={checked} onLock={setLocked}/></div>
  </Page></div>;
}
function ServerDraft({id,draft,reload}:{id:string;draft:Draft;reload:()=>void}){
  const query=useQuery<Schema['CatalogObject']>(`/catalog/${id}`);
  return <section><h2>服务器与本地草稿</h2><QueryState {...query} retry={query.reload}/>{query.data&&<><p>服务器 r{query.data.revision} · {query.data.name}</p><AgentDifference before={query.data.spec} after={draft.spec}/><button onClick={reload}>丢弃本地修改，读取服务器草稿</button></>}</section>;
}
