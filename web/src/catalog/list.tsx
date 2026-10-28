import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Page,Pager,QueryState,timestamp,canManageCatalog,type Access,type Schema} from '../console/components';
import {CreateCatalog} from './create';

const labels:Record<string,string>={agent:'助手',workflow:'工作流',connection:'模型连接'};
export function Catalog({access,kind}:{access:Access;kind:string}) {
  const [offset,setOffset]=useState(0),[create,setCreate]=useState(false);
  const params=new URLSearchParams({kind,offset:String(offset)});
  const query=useQuery<Schema['CatalogPage']>(`/catalog?${params}`);
  const permitted=canManageCatalog(access);
  return <Page title={labels[kind]} description={kind==='workflow'?'在画布上连接助手，用表单配置步骤；发布后即可运行或设置定时任务。':kind==='agent'?'选择常用角色、填写职责，保存并发布后即可在新建任务中使用。':'保存草稿后发布，任务使用已发布的固定版本。'} actions={permitted&&(kind==='workflow'?<a className="button primary" href="#/workflow/new">创建工作流</a>:<button onClick={()=>setCreate(value=>!value)}>{create?'收起创建':kind==='agent'?'准备新助手':'创建 / 导入'}</button>)}>
    {create&&kind!=='workflow'&&<CreateCatalog key={kind} access={access} kind={kind} created={query.reload}/>}
    <QueryState {...query} retry={query.reload} empty={!query.data?.items.length?(kind==='workflow'?'当前页没有工作流。点击“创建工作流”开始连接共享角色。':'当前页没有配置。管理者可创建或导入模板，成员可继续翻页查看已有对象。'):undefined}/>
    <div className="table-scroll"><table><thead><tr><th>名称</th><th>草稿修订</th><th>状态</th><th>创建时间</th></tr></thead><tbody>{query.data?.items.map(item=><tr key={item.id}>
      <td><a href={`#/catalog/${item.id}`}>{item.name}</a><small className="identifier">{item.id}</small></td><td>{item.revision}</td><td>{item.archived?'已归档':'可使用'}</td><td>{timestamp(item.created_at)}</td>
    </tr>)}</tbody></table></div>{query.data&&<Pager offset={offset} next={query.data.next_offset} change={setOffset}/>}
  </Page>;
}
