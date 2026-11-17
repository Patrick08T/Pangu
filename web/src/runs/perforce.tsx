import {useAction,useQuery} from '../api/hooks';
import {apiRequest} from '../api/request';
import {ActionNotice,QueryState,type Schema} from '../console/components';

export function PerforceDelivery({run,allowed,stopped,csrf}:{run:string;allowed:boolean;stopped:boolean;csrf?:string}) {
  const query=useQuery<Schema['PerforceDelivery']|null>(`/runs/${run}/perforce`,5000),action=useAction();
  const item=query.data;
  async function deliver(){const result=await apiRequest<Schema['PerforceDelivery']>(`/runs/${run}/perforce`,{method:'POST',csrf});query.reload();if(result.status==='failed')throw new Error(result.error||'交付未完成，可重试');}
  if(!item&&!query.error)return null;
  const labels={available:'可生成待审核交付',preparing:'正在生成',shelved:'本轮 changelist 已 shelve，等待人工审查',failed:'交付失败，可重试',empty:'本轮没有待 shelve 的新增打开文件'};
  return <section className="panel"><h2>Perforce 交付</h2><QueryState {...query} retry={query.reload}/>{item&&<><p>已提交参考基线：{item.baseline} · {labels[item.status]}</p>{item.local_directory&&<p>绑定的工作区根目录：<code>{item.local_directory}</code>。运行直接修改本地文件，快照包含执行前已有修改。</p>}{item.changelist&&<p><strong>本轮 changelist：{item.changelist}</strong></p>}<p>Client：<code>{item.client}</code></p>{!!item.existing_changelists?.length&&<p><strong>本轮还修改了原 changelist 中的文件：{item.existing_changelists.join('、')}。这些文件保留原归属，未包含在本轮 shelf 中，请在 P4V 中一并审查。</strong></p>}{item.error&&<p className="error-text">{item.error}</p>}{allowed&&stopped&&!['shelved','empty'].includes(item.status)&&<button disabled={action.busy} onClick={()=>void action.run(deliver)}>生成待审核 changelist</button>}<p>可在 P4V 中按编号查看 shelved files；平台不自动提交或回退本地文件。</p></>}<ActionNotice {...action}/></section>;
}
