import {useState} from 'react';
import {ConnectionFields} from './connection';
import {AgentFields} from './agent';
import {handoffProblems} from '../agent/handoff';
import {useAction} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,JSONField,canManageCatalog,type Access,type Schema} from '../console/components';

export function editable(access:Access) { return canManageCatalog(access); }

export function CatalogEditor({access,item,changed}:{access:Access;item:Schema['CatalogObject'];changed:()=>void}) {
  const [name,setName]=useState(item.name),[spec,setSpec]=useState(JSON.stringify(item.spec,null,2));
  const action=useAction(),canWrite=editable(access),dirty=name!==item.name||spec!==JSON.stringify(item.spec,null,2);
  const contractErrors=item.spec.agent?handoffProblems(item.spec.agent.handoff):[];
  async function save(){await apiRequest(`/catalog/${item.id}/draft`,{method:'PUT',csrf:access.session.csrf,body:{revision:item.revision,name,spec:JSON.parse(spec)}});changed();}
  async function publish(){await apiRequest(`/catalog/${item.id}/publish`,{method:'POST',csrf:access.session.csrf,body:{revision:item.revision}});changed();}
  async function archive(){await apiRequest(`/catalog/${item.id}/archive`,{method:'POST',csrf:access.session.csrf,body:{archived:!item.archived}});changed();}
  return <form className="editor" onSubmit={event=>{event.preventDefault();void action.run(save,'草稿已保存');}}>
    <div className="section-heading"><h2>草稿 · 修订 {item.revision}</h2><span>{item.archived?'已归档':'可编辑草稿'}</span></div>
    <label className="field">名称<input value={name} maxLength={128} required disabled={!canWrite||item.archived||action.busy} onChange={e=>setName(e.target.value)}/></label>
    {item.kind==='connection'?<ConnectionFields value={spec} change={setSpec} disabled={!canWrite||item.archived||action.busy}/>:item.kind==='agent'?<AgentFields value={spec} change={setSpec} disabled={!canWrite||item.archived||action.busy}/>:<JSONField label="配置 JSON" value={spec} onChange={setSpec} disabled={!canWrite||item.archived||action.busy} rows={18}/>}
    {item.kind==='connection'&&<p className="muted">账号登录与 API 密钥由 Worker 管理，禁止在配置中粘贴密钥。新发布版本需显式绑定才生效。</p>}
    <div className="toolbar"><button className="primary" disabled={!canWrite||item.archived||action.busy||!dirty}>保存草稿</button>
      <button type="button" disabled={!canWrite||item.archived||action.busy||dirty||contractErrors.length>0} onClick={()=>void action.run(publish,'当前草稿已发布为固定版本')}>发布当前草稿</button>
      <button type="button" disabled={!canWrite||action.busy} onClick={()=>void action.run(archive,item.archived?'已恢复配置':'已归档；历史引用保留')}>{item.archived?'恢复配置':'归档配置'}</button>
      <button type="button" disabled={action.busy} onClick={changed}>丢弃本地编辑，重新读取</button></div>
    {dirty&&<p className="muted">存在未保存修改，保存后才能发布。</p>}{contractErrors.length>0&&<p className="notice">请在交接契约中补齐并保存：{contractErrors.join('；')}。</p>}<ActionNotice {...action}/>
  </form>;
}
