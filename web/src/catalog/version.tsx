import {useState} from 'react';
import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,Page,Pager,QueryState,timestamp,canEdit,type Access,type Schema} from '../console/components';
import {CatalogEditor} from './editor';
import {Download} from '../api/download';
import {VersionPreflight} from './preflight';

export function CatalogDetail({id,access}:{id:string;access:Access}) {
  const query=useQuery<Schema['CatalogObject']>(`/catalog/${encodeURIComponent(id)}`);
  return <Page title={query.data?.name??'配置详情'} description="项目与来源版本单独保存，编辑不会迁移项目或改写已发布历史。" actions={<a className="button" href={`#/${query.data?.kind??'agent'}`}>返回配置列表</a>}>
    <QueryState {...query} retry={query.reload}/>{query.data&&<>
      <CatalogEditor key={`${id}/${query.data.revision}/${query.data.archived}`} item={query.data} access={access} changed={query.reload}/>
      <Download path={`/catalog/${id}/export`} filename={`${id}.json`}>导出当前草稿</Download>
      <Versions key={`${id}/${query.data.revision}/${query.data.archived}`} item={query.data} access={access}/>
    </>}
  </Page>;
}

function Versions({item,access}:{item:Schema['CatalogObject'];access:Access}) {
  const [offset,setOffset]=useState(0),[selected,select]=useState<Schema['CatalogVersion']>();
  const query=useQuery<Schema['CatalogVersion'][]>(`/catalog/${item.id}/versions?offset=${offset}`);
  return <section><h2>发布版本</h2><QueryState {...query} retry={query.reload} empty={!query.data?.length?'尚未发布。保存完整草稿后发布第一个版本。':undefined}/>
    <div className="table-scroll"><table><thead><tr><th>版本</th><th>来源修订</th><th>发布时间</th><th>操作</th></tr></thead><tbody>{query.data?.map(version=><tr key={version.id}>
      <td>v{version.number}<small className="identifier">{version.id}</small></td><td>{version.source_revision}</td><td>{timestamp(version.created_at)}</td><td><div className="toolbar">
        <button onClick={()=>select(version)}>查看 / 使用版本</button><Download path={`/versions/${version.id}/export`} filename={`${version.id}.json`}>导出</Download>
      </div></td></tr>)}</tbody></table></div>
    {query.data&&<Pager offset={offset} next={query.data.length===100?offset+100:undefined} change={setOffset}/>}
    {selected&&<VersionDetail key={selected.id} version={selected} access={access}/>}
  </section>;
}

function VersionDetail({version,access}:{version:Schema['CatalogVersion'];access:Access}) {
  const [name,setName]=useState(''),[connection,setConnection]=useState(''),action=useAction();
  async function bind(){
    const item=await apiRequest<Schema['CatalogObject']>(`/versions/${version.id}/bind`,{method:'POST',csrf:access.session.csrf,body:{project_id:access.project,name,connection_version:connection}});
    action.navigate(`/catalog/${item.id}`);
  }
  return <div className="version-detail"><h3>v{version.number} 固定快照</h3><pre>{JSON.stringify(version.spec,null,2)}</pre>
    {version.kind==='template'&&canEdit(access)&&!version.archived&&<form onSubmit={e=>{e.preventDefault();void action.run(bind);}}>
      <p>绑定到当前项目 {access.project}，生成新的项目 Agent。绑定后不可移动；原模板更新不会改变此 Agent。</p>
      <label className="field">项目 Agent 名称<input required value={name} disabled={action.busy} onChange={e=>setName(e.target.value)}/></label>
      {version.spec.agent?.executor==='eino'&&<label className="field">项目连接版本 ID<input required value={connection} disabled={action.busy} onChange={e=>setConnection(e.target.value)}/><small>由项目管理者发布连接版本后提供，绑定使用该版本的模型。</small></label>}
      <button className="primary" disabled={action.busy||!access.project}>绑定到当前项目</button></form>}
    {(version.kind==='agent'||version.kind==='workflow')&&version.project_id===access.project&&canEdit(access)&&!version.archived&&<div className="toolbar"><a className="button primary" href={`#/start/${version.id}`}>使用此版本运行</a><a className="button" href={`#/schedule/new/${version.id}`}>创建定时计划</a></div>}
    {version.kind!=='template'&&version.project_id===access.project&&canEdit(access)&&!version.archived&&<VersionPreflight version={version} access={access}/>}
    <ActionNotice {...action}/>
  </div>;
}
