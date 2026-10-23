import {useState} from 'react';
import {useQuery} from '../api/hooks';
import {Page,Pager,QueryState,Status,timestamp,type Access,type Schema} from '../console/components';

export function Audit({access}:{access:Access}) {
  const [offset,setOffset]=useState(0),[all,setAll]=useState(false);
  const query=useQuery<Schema['AuditPage']>(`/audit?offset=${offset}${all&&access.session.admin?'':`&project_id=${access.project}`}`,5000);
  return <Page title="操作审计" description="由服务端记录修改意图和结果。结果未知表示响应证据缺失，应按对象编号核查实际状态。">
    {access.session.admin&&<label className="check"><input type="checkbox" checked={all} onChange={e=>{setAll(e.target.checked);setOffset(0);}}/>查看整个工作空间</label>}
    <QueryState {...query} retry={query.reload} empty={!query.data?.items.length?'当前页没有审计记录。':undefined}/>
    {!!query.data?.items.length&&<div className="table-scroll"><table><thead><tr><th>时间 / 操作者</th><th>操作 / 对象</th><th>结果</th><th>追踪</th></tr></thead><tbody>{query.data.items.map(entry=><tr key={entry.id}><td>{timestamp(entry.created_at)}<small className="identifier">{entry.actor_id}</small></td><td><code>{entry.action}</code><small className="identifier">{entry.object_id??entry.result_id??'—'}</small>{entry.target_id&&<small>目标成员：{entry.target_id} · {entry.role}</small>}{entry.result_version&&<small>版本 / 修订 {entry.result_version}</small>}</td><td><Status value={entry.outcome}/><small>{entry.http_status??'未记录响应'}</small></td><td><details><summary>查看编号</summary><p className="identifier">审计：{entry.id}<br/>请求：{entry.request_id}<br/>项目：{entry.project_id??'工作空间'}</p></details></td></tr>)}</tbody></table></div>}
    <Pager offset={offset} next={query.data?.next_offset} change={setOffset}/>
  </Page>;
}
