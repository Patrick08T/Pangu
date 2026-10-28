import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Page,Pager,QueryState,timestamp,canEdit,isManager,type Access,type Schema} from '../console/components';
import {CreateCatalog} from './create';

const labels:Record<string,string>={template:'共享模板',agent:'项目 Agent',workflow:'工作流',connection:'模型连接'};
export function Catalog({access,kind}:{access:Access;kind:string}) {
  const [offset,setOffset]=useState(0),[create,setCreate]=useState(false);
  const params=new URLSearchParams({kind,offset:String(offset)});if(kind!=='template')params.set('project_id',access.project);
  const query=useQuery<Schema['CatalogPage']>(access.project||kind==='template'?`/catalog?${params}`:null);
  const permitted=kind==='template'||kind==='connection'?isManager(access)||access.session.admin:canEdit(access);
  return <Page title={labels[kind]} description={kind==='template'?'共享模板可绑定到多个项目；每次绑定创建独立 Agent，项目随后不可迁移。':'草稿保存与发布分开。运行和计划固定使用明确的发布版本。'} actions={permitted&&<button onClick={()=>setCreate(value=>!value)}>{create?'收起创建':'创建 / 导入'}</button>}>
    {create&&<CreateCatalog key={kind} access={access} kind={kind} created={query.reload}/>}
    <QueryState {...query} retry={query.reload} empty={!query.data?.items.length?'当前页没有配置。管理者可创建或导入模板，成员可继续翻页查看已有对象。':undefined}/>
    <div className="table-scroll"><table><thead><tr><th>名称</th><th>草稿修订</th><th>状态</th><th>创建时间</th></tr></thead><tbody>{query.data?.items.map(item=><tr key={item.id}>
      <td><a href={`#/catalog/${item.id}`}>{item.name}</a><small className="identifier">{item.id}</small></td><td>{item.revision}</td><td>{item.archived?'已归档':'可使用'}</td><td>{timestamp(item.created_at)}</td>
    </tr>)}</tbody></table></div>{query.data&&<Pager offset={offset} next={query.data.next_offset} change={setOffset}/>}
  </Page>;
}
