import {useState} from 'react';
import {useAction} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,JSONField,type Access,type Schema} from '../console/components';
import {templates} from './templates';

export function CreateCatalog({access,kind,created}:{access:Access;kind:string;created:()=>void}) {
  const action=useAction(),[name,setName]=useState(''),[spec,setSpec]=useState(JSON.stringify(initialSpec(kind),null,2)),[document,setDocument]=useState('');
  async function create(){
    const item=await apiRequest<Schema['CatalogObject']>('/catalog',{method:'POST',csrf:access.session.csrf,body:{kind,name,spec:JSON.parse(spec),...(kind==='template'?{}:{project_id:access.project})}});
    if(action.active())created();action.navigate(`/catalog/${item.id}`);
  }
  async function importJSON(raw:string){
    const data=JSON.parse(raw) as Schema['CatalogDocument'];
    if(data.kind!=='template'&&data.project_id!==access.project)throw Error('导入对象必须属于当前项目；跨项目复用请绑定共享模板。');
    const item=await apiRequest<Schema['CatalogObject']>('/catalog/import',{method:'POST',csrf:access.session.csrf,body:data});
    if(action.active())created();action.navigate(`/catalog/${item.id}`);
  }
  return <section className="editor"><h2>创建配置</h2><form onSubmit={e=>{e.preventDefault();void action.run(create);}}>
    <label className="field">名称<input required value={name} disabled={action.busy} onChange={e=>setName(e.target.value)}/></label><JSONField label="初始配置 JSON" value={spec} onChange={setSpec} disabled={action.busy}/>
    <button className="primary" disabled={action.busy}>创建草稿</button></form>
    {kind==='template'&&<details><summary>导入内置模板</summary><p>导入后创建新草稿；发布固定版本，再绑定到项目使用。</p><div className="toolbar">{templates.map(([label,raw])=><button disabled={action.busy} key={label} onClick={()=>void action.run(()=>importJSON(raw))}>{label}</button>)}</div></details>}
    <details><summary>导入 pangu.catalog.v1 文档</summary><p>导入始终创建新对象，不覆盖已有配置或改变绑定项目。</p><JSONField label="导入文档 JSON" value={document} onChange={setDocument} disabled={action.busy}/><button disabled={action.busy||!document.trim()} onClick={()=>void action.run(()=>importJSON(document))}>导入为新草稿</button></details>
    <ActionNotice {...action}/>
  </section>;
}

function initialSpec(kind:string):Schema['CatalogSpec'] {
  if(kind==='connection')return {connection:{base_url:'',model:'',secret_ref:''}};
  if(kind==='workflow')return {flow:{nodes:[]}};
  return {agent:{id:'agent',executor:'codex',instruction:''}};
}
