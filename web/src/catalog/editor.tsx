import {useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,JSONField,canEdit,isManager,type Access,type Schema} from '../console/components';

export function editable(access:Access,item:Pick<Schema['CatalogObject'],'kind'|'project_id'>) {
  if(item.kind==='template')return isManager(access)||access.session.admin;
  return access.project===item.project_id&&(item.kind==='connection'?isManager(access):canEdit(access));
}

export function CatalogEditor({access,item,changed}:{access:Access;item:Schema['CatalogObject'];changed:()=>void}) {
  const [name,setName]=useState(item.name),[spec,setSpec]=useState(JSON.stringify(item.spec,null,2));
  const action=useAction(),canWrite=editable(access,item),dirty=name!==item.name||spec!==JSON.stringify(item.spec,null,2);
  async function save(){await apiRequest(`/catalog/${item.id}/draft`,{method:'PUT',csrf:access.session.csrf,body:{revision:item.revision,name,spec:JSON.parse(spec)}});changed();}
  async function publish(){await apiRequest(`/catalog/${item.id}/publish`,{method:'POST',csrf:access.session.csrf,body:{revision:item.revision}});changed();}
  async function archive(){await apiRequest(`/catalog/${item.id}/archive`,{method:'POST',csrf:access.session.csrf,body:{archived:!item.archived}});changed();}
  return <form className="editor" onSubmit={event=>{event.preventDefault();void action.run(save,'草稿已保存');}}>
    <div className="section-heading"><h2>草稿 · 修订 {item.revision}</h2><span>{item.archived?'已归档':'可编辑草稿'}</span></div>
    {item.project_id&&<label className="field">绑定项目（不可迁移）<input readOnly value={item.project_id}/></label>}
    {item.spec.template_version&&<p className="identifier">模板来源版本：{item.spec.template_version}</p>}
    <label className="field">名称<input value={name} maxLength={128} required disabled={!canWrite||item.archived||action.busy} onChange={e=>setName(e.target.value)}/></label>
    <JSONField label="配置 JSON" value={spec} onChange={setSpec} disabled={!canWrite||item.archived||action.busy} rows={18}/>
    {item.kind==='connection'&&<p className="muted">只填写服务端凭据引用名，禁止粘贴密钥。项目、端点和模型需匹配管理员的授权登记；新发布版本需显式绑定才生效。</p>}
    <div className="toolbar"><button className="primary" disabled={!canWrite||item.archived||action.busy||!dirty}>保存草稿</button>
      <button type="button" disabled={!canWrite||item.archived||action.busy||dirty} onClick={()=>void action.run(publish,'当前草稿已发布为固定版本')}>发布当前草稿</button>
      <button type="button" disabled={!canWrite||action.busy} onClick={()=>void action.run(archive,item.archived?'已恢复配置':'已归档；历史引用保留')}>{item.archived?'恢复配置':'归档配置'}</button>
      <button type="button" disabled={action.busy} onClick={changed}>丢弃本地编辑，重新读取</button></div>
    {dirty&&<p className="muted">存在未保存修改，保存后才能发布。</p>}<ActionNotice {...action}/>
  </form>;
}
